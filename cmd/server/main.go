package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vaultkeeper/vaultkeeper/internal/audit"
	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/backup"
	"github.com/vaultkeeper/vaultkeeper/internal/cases"
	"github.com/vaultkeeper/vaultkeeper/internal/config"
	"github.com/vaultkeeper/vaultkeeper/internal/custody"
	"github.com/vaultkeeper/vaultkeeper/internal/database"
	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/logging"
	"github.com/vaultkeeper/vaultkeeper/internal/notifications"
	"github.com/vaultkeeper/vaultkeeper/internal/reports"
	"github.com/vaultkeeper/vaultkeeper/internal/search"
	"github.com/vaultkeeper/vaultkeeper/internal/server"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	logger := logging.NewLogger(cfg.AppEnv, cfg.LogLevel, os.Stdout)
	logger.Info("configuration loaded", "config", cfg.String())
	for _, warning := range cfg.Warnings {
		logger.Warn("configuration warning", "warning", warning)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create postgres pool: %w", err)
	}
	defer pool.Close()

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	if err := database.RunMigrations(ctx, pool, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	jwks := auth.NewJWKSFetcher(cfg.KeycloakURL, cfg.KeycloakRealm)
	jwksCtx, jwksCancel := context.WithTimeout(ctx, 15*time.Second)
	defer jwksCancel()
	if err := jwks.Prefetch(jwksCtx); err != nil {
		logger.Warn("JWKS prefetch failed; continuing without warm cache", "error", err)
	} else {
		logger.Info("JWKS keys prefetched successfully")
	}

	auditLogger := audit.NewLogger(pool)

	custodyRepo := custody.NewRepository(pool)
	custodyLogger := custody.NewLogger(custodyRepo)

	caseRepo := cases.NewRepository(pool)
	caseSvc, err := cases.NewService(caseRepo, custodyLogger, cfg.CaseReferenceRegex)
	if err != nil {
		return fmt.Errorf("create case service: %w", err)
	}
	caseHandler := cases.NewHandler(caseSvc, auditLogger)

	roleRepo := cases.NewRoleRepository(pool)
	roleHandler := cases.NewRoleHandler(roleRepo, custodyLogger, auditLogger)

	// Evidence subsystem
	minioStorage, err := evidence.NewMinIOStorage(ctx, cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	if err != nil {
		return fmt.Errorf("create minio storage: %w", err)
	}

	var tsaClient integrity.TimestampAuthority
	if cfg.TSAEnabled {
		tsaClient = integrity.NewRFC3161Client(cfg.TSAURL)
	} else {
		tsaClient = &integrity.NoopTimestampAuthority{}
	}

	var searchIndexer search.SearchIndexer
	if cfg.MeilisearchURL != "" {
		searchIndexer = search.NewMeilisearchClient(cfg.MeilisearchURL, cfg.MeilisearchAPIKey)
	} else {
		searchIndexer = &search.NoopSearchIndexer{}
	}

	evidenceRepo := evidence.NewRepository(pool)
	caseLookup := evidence.NewCaseLookup(pool)
	thumbGen := evidence.NewThumbnailGenerator()

	evidenceSvc := evidence.NewService(
		evidenceRepo, minioStorage, tsaClient, searchIndexer,
		custodyLogger, caseLookup, thumbGen, logger, cfg.MaxUploadSize,
	)
	evidenceHandler := evidence.NewHandler(evidenceSvc, custodyRepo, auditLogger, cfg.MaxUploadSize)

	// Start TSA retry job
	tsaRetryJob := integrity.NewTSARetryJob(tsaClient, evidenceRepo, evidenceRepo, custodyLogger, logger)
	tsaCtx, tsaCancel := context.WithCancel(ctx)
	defer tsaCancel()
	go tsaRetryJob.Start(tsaCtx)

	// Notification subsystem
	notifRepo := notifications.NewRepository(pool)
	emailSender := notifications.NewEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom, logger)
	emailCtx, emailCancel := context.WithCancel(ctx)
	defer emailCancel()
	emailSender.Start(emailCtx)
	defer emailSender.Stop()

	// Email delivery requires a UserEmailResolver (e.g. Keycloak admin API integration).
	// Passing nil disables email; in-app notifications still work.
	notifService := notifications.NewService(notifRepo, emailSender, nil, logger, cfg.AdminUserIDs)
	notifHandler := notifications.NewHandler(notifService)

	// Search subsystem
	caseIDLoader := search.NewCaseIDsLoader(pool)
	var evidenceSearcher search.EvidenceSearcher
	if cfg.MeilisearchURL != "" {
		meiliClient := search.NewMeilisearchClient(cfg.MeilisearchURL, cfg.MeilisearchAPIKey)
		evidenceSearcher = meiliClient
	} else {
		evidenceSearcher = &search.NoopEvidenceSearcher{}
	}
	searchHandler := search.NewHandler(evidenceSearcher, caseIDLoader, caseIDLoader, auditLogger)

	// Integrity verification handler
	fileReaderAdapter := &integrity.StorageFileReader{
		GetFn: minioStorage.GetObject,
	}
	evidenceLoaderAdapter := &integrity.VerifiableItemAdapter[evidence.VerifiableItem]{
		ListFn: evidenceRepo.ListForVerification,
		ConvertFn: func(item evidence.VerifiableItem) integrity.VerifiableItem {
			return integrity.VerifiableItem{
				ID:         item.ID,
				CaseID:     item.CaseID,
				StorageKey: item.StorageKey,
				SHA256Hash: item.SHA256Hash,
				TSAToken:   item.TSAToken,
				TSAStatus:  item.TSAStatus,
				Filename:   item.Filename,
			}
		},
	}
	integrityNotifier := &integrity.NotificationAdapter{
		NotifyFn: func(ctx context.Context, event integrity.NotificationEvent) error {
			return notifService.Notify(ctx, notifications.NotificationEvent{
				Type:   event.Type,
				CaseID: event.CaseID,
				Title:  event.Title,
				Body:   event.Body,
			})
		},
	}
	integrityHandler := integrity.NewHandler(
		evidenceLoaderAdapter, fileReaderAdapter, tsaClient,
		custodyLogger, integrityNotifier, evidenceRepo, logger, auditLogger,
	)

	// Backup subsystem
	backupNotifier := &backup.NotificationBridge{
		NotifyFn: func(ctx context.Context, err error) error {
			return notifService.Notify(ctx, notifications.NotificationEvent{
				Type:  notifications.EventBackupFailed,
				Title: "Backup failed",
				Body:  err.Error(),
			})
		},
	}
	backupRunner := backup.NewBackupRunner(pool, cfg.BackupEncKey, cfg.BackupDestination, logger, backupNotifier, minioStorage)
	backupHandler := backup.NewHandler(backupRunner, logger, auditLogger)

	// Start backup scheduler (daily at 03:00 UTC)
	backupCtx, backupCancel := context.WithCancel(ctx)
	defer backupCancel()
	go backupRunner.StartScheduler(backupCtx, 3, 0)

	// Case export
	exportSvc := cases.NewExportService(evidenceRepo, custodyRepo, caseRepo, minioStorage, custodyLogger)
	exportHandler := cases.NewExportHandler(exportSvc, auditLogger)

	// Custody PDF reports
	caseReportAdapter := &caseReportSourceAdapter{repo: caseRepo}
	reportGen := reports.NewCustodyReportGenerator(custodyRepo, evidenceRepo, caseReportAdapter)
	reportHandler := reports.NewHandler(reportGen, auditLogger)

	healthHandler := server.NewHealthHandler(
		pool, minioStorage, cfg.MinIOBucket,
		cfg.MeilisearchURL, cfg.KeycloakURL, cfg.KeycloakRealm,
		version, auditLogger,
		server.WithBackupChecker(backupRunner),
	)

	httpServer := server.NewHTTPServer(cfg, logger, version, jwks, auditLogger, healthHandler,
		caseHandler, roleHandler, evidenceHandler, notifHandler, searchHandler, integrityHandler,
		backupHandler, exportHandler, reportHandler,
	)

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server starting", "addr", httpServer.Addr, "version", version)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("server stopped cleanly")
	return nil
}

// caseReportSourceAdapter adapts the cases.PGRepository to the reports.CaseReportSource interface.
type caseReportSourceAdapter struct {
	repo *cases.PGRepository
}

func (a *caseReportSourceAdapter) FindByID(ctx context.Context, id uuid.UUID) (reports.CaseRecord, error) {
	c, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return reports.CaseRecord{}, err
	}
	return reports.CaseRecord{
		ID:            c.ID,
		ReferenceCode: c.ReferenceCode,
		Title:         c.Title,
		Jurisdiction:  c.Jurisdiction,
		Status:        c.Status,
	}, nil
}
