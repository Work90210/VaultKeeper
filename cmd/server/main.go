package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vaultkeeper/vaultkeeper/internal/apikeys"
	"github.com/vaultkeeper/vaultkeeper/internal/app"
	"github.com/vaultkeeper/vaultkeeper/internal/audit"
	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/backup"
	"github.com/vaultkeeper/vaultkeeper/internal/cases"
	"github.com/vaultkeeper/vaultkeeper/internal/organization"
	"github.com/vaultkeeper/vaultkeeper/internal/profile"
	"github.com/vaultkeeper/vaultkeeper/internal/collaboration"
	"github.com/vaultkeeper/vaultkeeper/internal/config"
	"github.com/vaultkeeper/vaultkeeper/internal/custody"
	"github.com/vaultkeeper/vaultkeeper/internal/database"
	"github.com/vaultkeeper/vaultkeeper/internal/disclosures"
	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
	"github.com/vaultkeeper/vaultkeeper/internal/investigation"
	"github.com/vaultkeeper/vaultkeeper/internal/evidence/cleanup"
	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/logging"
	"github.com/vaultkeeper/vaultkeeper/internal/migration"
	"github.com/vaultkeeper/vaultkeeper/internal/notifications"
	"github.com/vaultkeeper/vaultkeeper/internal/reports"
	"github.com/vaultkeeper/vaultkeeper/internal/search"
	"github.com/vaultkeeper/vaultkeeper/internal/server"
	"github.com/vaultkeeper/vaultkeeper/internal/witnesses"
)

var version = "dev"

// evidenceOwnerAdapter bridges evidence.Repository to investigation.EvidenceOwnerChecker.
type evidenceOwnerAdapter struct {
	repo interface {
		FindByID(ctx context.Context, id uuid.UUID) (evidence.EvidenceItem, error)
	}
}

func (a *evidenceOwnerAdapter) GetUploadedBy(ctx context.Context, evidenceID uuid.UUID) (string, error) {
	item, err := a.repo.FindByID(ctx, evidenceID)
	if err != nil {
		return "", err
	}
	return item.UploadedBy, nil
}

// caseMembershipAdapter bridges cases.RoleRepository to investigation.CaseMembershipChecker.
// System admins bypass case-role lookup — they have cross-case access by design,
// matching the evidence handler pattern (loadCallerCaseRole returns RoleJudge for system admins).
type caseMembershipAdapter struct {
	roleRepo auth.CaseRoleLoader
}

func (a *caseMembershipAdapter) HasRoleOnCase(ctx context.Context, caseID uuid.UUID, userID string) (bool, error) {
	// Check if the caller is a system admin via the auth context on the request.
	// System admins don't have case-level roles but have cross-case access.
	if ac, ok := auth.GetAuthContext(ctx); ok && ac.SystemRole == auth.RoleSystemAdmin {
		return true, nil
	}
	_, err := a.roleRepo.LoadCaseRole(ctx, caseID.String(), userID)
	if err != nil {
		return false, nil // no role = no membership (not an error)
	}
	return true, nil
}

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
	roleRepo := cases.NewRoleRepository(pool)
	roleHandler := cases.NewRoleHandler(roleRepo, custodyLogger, auditLogger)

	// Organization subsystem
	orgRepo := organization.NewOrgRepository(pool)
	memberRepo := organization.NewMembershipRepository(pool)

	orgMemberAdapter := &organization.OrgMemberAdapter{MemberRepo: memberRepo}
	caseHandler := cases.NewHandler(caseSvc, auditLogger, orgMemberAdapter, roleRepo)
	inviteRepo := organization.NewInvitationRepository(pool)
	orgAuthz := organization.NewOrgAuthzService(memberRepo)
	orgSvc := organization.NewService(orgRepo, memberRepo, inviteRepo, orgAuthz, logger).
		WithCaseStatusChecker(caseRepo)
	orgHandler := organization.NewHandler(orgSvc, auditLogger)

	// Profile subsystem
	profileRepo := profile.NewRepository(pool)
	profileHandler := profile.NewHandler(profileRepo, auditLogger)

	// API Keys subsystem
	apiKeysRepo := apikeys.NewRepository(pool)
	apiKeysHandler := apikeys.NewHandler(apiKeysRepo, auditLogger)

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

	// Sprint 9 wiring: inject the legal-hold adapter (bridges cases.Service),
	// a logging retention notifier stub, and the GDPR erasure repository
	// (satisfied by *evidence.PGRepository itself). All three are required
	// for destruction, retention notification, and GDPR erasure to function
	// in production — startup aborts below if any binding is missing.
	legalHoldAdapter := &app.LegalHoldAdapter{Svc: caseSvc}
	retentionNotifier := &app.LoggingRetentionNotifier{Logger: logger}
	uploadAttemptRepo := evidence.NewUploadAttemptRepository(pool)
	captureMetadataRepo := evidence.NewCaptureMetadataRepository(pool)
	evidenceSvc = evidenceSvc.
		WithLegalHoldChecker(legalHoldAdapter).
		WithRetentionNotifier(retentionNotifier).
		WithErasureRepo(evidenceRepo).
		WithUploadAttemptRepository(uploadAttemptRepo).
		WithOutboxPool(pool).
		WithCaptureMetadataRepository(captureMetadataRepo)
	if legalHoldAdapter == nil || retentionNotifier == nil || evidenceRepo == nil {
		logger.Error("sprint 9 wiring incomplete",
			"legal_hold_checker", legalHoldAdapter != nil,
			"retention_notifier", retentionNotifier != nil,
			"erasure_repo", evidenceRepo != nil)
		os.Exit(1)
	}

	evidenceHandler := evidence.NewHandler(evidenceSvc, custodyRepo, auditLogger, cfg.MaxUploadSize, uploadAttemptRepo)

	// Sprint 10 — bulk ZIP upload subsystem.
	bulkJobRepo := evidence.NewBulkJobRepository(pool)
	bulkSvc := evidence.NewBulkService(bulkJobRepo, evidenceSvc, logger, cfg.MaxUploadSize)
	bulkHandler := evidence.NewBulkHandler(bulkSvc, auditLogger, logger, cfg.MaxUploadSize)

	// Sprint 10 — unified archive import endpoint. Accepts a ZIP via
	// multipart upload, auto-detects manifest.csv (verified migration)
	// vs bulk, and extracts to a server-owned temp dir. Operators
	// never supply or see server paths.
	importRunnerAdapter := &importMigrationAdapter{} // set after migrationSvc is built
	importHandler := evidence.NewImportHandler(bulkSvc, importRunnerAdapter, auditLogger, logger, cfg.MaxUploadSize)

	// Sprint 10 — migration subsystem. Signer loads from INSTANCE_ED25519_KEY;
	// in production that env var is required (startup logs a warning when
	// missing and falls back to an ephemeral key suitable only for dev).
	// Sprint 9 wiring pattern: required secrets fail startup. Migration
	// attestation certificates signed with an ephemeral key have zero
	// verification value once the process restarts, so refuse to boot in
	// production without the key. The dev override
	// VAULTKEEPER_ALLOW_EPHEMERAL_SIGNING is flatly rejected in
	// production (AppEnv=production) — the only way to run in prod is
	// with a real key.
	if err := migration.RequireConfiguredKey(); err != nil {
		if cfg.AppEnv == "production" {
			return fmt.Errorf("migration signing key: %w (set INSTANCE_ED25519_KEY; see `vaultkeeper-migrate genkey`)", err)
		}
		if os.Getenv("VAULTKEEPER_ALLOW_EPHEMERAL_SIGNING") != "1" {
			return fmt.Errorf("migration signing key: %w (set INSTANCE_ED25519_KEY; see `vaultkeeper-migrate genkey`)", err)
		}
		logger.Error("MIGRATION SIGNING KEY NOT CONFIGURED — using ephemeral key (NON-PRODUCTION ONLY)",
			"override", "VAULTKEEPER_ALLOW_EPHEMERAL_SIGNING=1",
			"impact", "attestation certificates are not verifiable after server restart")
	}
	migrationSigner, err := migration.LoadOrGenerate()
	if err != nil {
		return fmt.Errorf("load migration signer: %w", err)
	}
	migrationRepo := migration.NewRepository(pool)
	migrationWriter := &migrationEvidenceWriter{svc: evidenceSvc}
	migrationIngester := migration.NewIngester(migrationWriter, migrationRepo)
	migrationSvc := migration.NewService(migration.NewParser(), migrationIngester, migrationRepo, tsaClient, logger)
	importRunnerAdapter.svc = migrationSvc
	migrationHandler := migration.NewHandler(
		migrationSvc,
		&migrationCaseLookup{svc: caseSvc},
		migrationSigner,
		auditLogger,
		version,
		cfg.MigrationStagingRoot,
		logger,
	)
	// Sprint 9: wire the case-role loader so the classification access
	// matrix is enforced on list / get / download / thumbnail / update /
	// destroy paths. Without this, reads bypass the matrix entirely.
	evidenceHandler.SetCaseRoleLoader(roleRepo)

	// Shared org-membership checker and case → org lookup used across handlers.
	orgAdapter := &organization.OrgMemberAdapter{MemberRepo: memberRepo}
	caseLookupOrgFn := func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error) {
		c, err := caseRepo.FindByID(ctx, caseID)
		if err != nil {
			return uuid.Nil, err
		}
		return c.OrganizationID, nil
	}
	evidenceLookupCaseFn := func(ctx context.Context, evidenceID uuid.UUID) (uuid.UUID, error) {
		item, err := evidenceRepo.FindByID(ctx, evidenceID)
		if err != nil {
			return uuid.Nil, err
		}
		return item.CaseID, nil
	}

	evidenceHandler.SetOrgMembershipChecker(orgAdapter, caseLookupOrgFn)
	gdprRegistrar := &evidence.GDPRRouteRegistrar{Handler: evidenceHandler, Audit: auditLogger}

	// Redaction service
	redactionSvc := evidence.NewRedactionService(evidenceSvc, minioStorage, tsaClient, custodyLogger, logger)
	evidenceHandler.SetRedactionService(redactionSvc)

	// Witness subsystem
	witnessEncKey := []byte(cfg.WitnessEncryptionKey)
	if len(witnessEncKey) == 0 {
		return fmt.Errorf("WITNESS_ENCRYPTION_KEY is required")
	}
	witnessEncryptor, err := witnesses.NewEncryptor(witnesses.EncryptionKey{Version: 1, Key: witnessEncKey})
	if err != nil {
		return fmt.Errorf("create witness encryptor: %w", err)
	}
	witnessRepo := witnesses.NewRepository(pool)
	witnessSvc := witnesses.NewService(witnessRepo, witnessEncryptor, custodyLogger, logger)
	witnessHandler := witnesses.NewHandler(witnessSvc, roleRepo, auditLogger)
	witnessHandler.SetOrgMembershipChecker(
		&organization.OrgMemberAdapter{MemberRepo: memberRepo},
		func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error) {
			c, err := caseRepo.FindByID(ctx, caseID)
			if err != nil {
				return uuid.Nil, err
			}
			return c.OrganizationID, nil
		},
	)

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
	notifPrefsRepo := notifications.NewPGPreferencesRepository(pool)
	notifHandler.SetPreferencesRepo(notifPrefsRepo)

	// Disclosure subsystem (depends on notification service)
	disclosureRepo := disclosures.NewRepository(pool)
	disclosureNotifier := &disclosureNotificationAdapter{notifService: notifService}
	disclosureSvc := disclosures.NewService(disclosureRepo, custodyLogger, disclosureNotifier, logger)
	disclosureHandler := disclosures.NewHandler(disclosureSvc, roleRepo, auditLogger)
	disclosureHandler.SetOrgMembershipChecker(
		&organization.OrgMemberAdapter{MemberRepo: memberRepo},
		func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error) {
			c, err := caseRepo.FindByID(ctx, caseID)
			if err != nil {
				return uuid.Nil, err
			}
			return c.OrganizationID, nil
		},
	)

	// Search subsystem
	caseIDLoader := search.NewCaseIDsLoader(pool)
	var evidenceSearcher search.EvidenceSearcher
	if cfg.MeilisearchURL != "" {
		meiliClient := search.NewMeilisearchClient(cfg.MeilisearchURL, cfg.MeilisearchAPIKey)
		// Ensure searchable/filterable/sortable attributes and typo tolerance
		// are configured. Without this, faceted search fails with
		// "invalid_search_facets" and the handler returns 503. Log & continue
		// on error so Meilisearch outages don't block the rest of startup.
		if err := meiliClient.ConfigureEvidenceIndex(ctx); err != nil {
			logger.Warn("failed to configure evidence search index", "error", err)
		}
		evidenceSearcher = meiliClient
	} else {
		evidenceSearcher = &search.NoopEvidenceSearcher{}
	}
	searchHandler := search.NewHandler(evidenceSearcher, caseIDLoader, caseIDLoader, auditLogger)
	searchHandler.SetOrgIDLoader(caseIDLoader)

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

	// Start retention-expiry notification scheduler (daily). Fires
	// NotifyExpiringRetention for items whose effective retention expires
	// within the next 30 days so case admins have runway to review.
	retentionCtx, retentionCancel := context.WithCancel(ctx)
	defer retentionCancel()
	go runRetentionScheduler(retentionCtx, evidenceSvc, logger)

	// Case export
	exportSvc := cases.NewExportService(evidenceRepo, custodyRepo, caseRepo, minioStorage, custodyLogger).
		WithCaptureMetadataExporter(captureMetadataRepo)
	exportHandler := cases.NewExportHandler(exportSvc, auditLogger)
	exportHandler.SetOrgMembershipChecker(orgAdapter, caseLookupOrgFn)

	// Custody PDF reports
	caseReportAdapter := &caseReportSourceAdapter{repo: caseRepo}
	reportGen := reports.NewCustodyReportGenerator(custodyRepo, evidenceRepo, caseReportAdapter)
	reportHandler := reports.NewHandler(reportGen, auditLogger)
	reportHandler.SetOrgMembershipChecker(orgAdapter, caseLookupOrgFn, evidenceLookupCaseFn)

	// PDF page renderer
	pagesHandler := evidence.NewPagesHandler(pool, minioStorage, roleRepo, logger)

	// Redaction draft CRUD (multi-draft)
	draftHandler := evidence.NewDraftHandler(pool, roleRepo, custodyLogger, logger)
	draftHandler.SetRedactionService(redactionSvc)

	// Collaboration subsystem (WebSocket hub for real-time redaction editing)
	draftStore := collaboration.NewPostgresDraftStore(pool)
	collabHub := collaboration.NewHub(draftStore, logger)
	collabCtx, collabCancel := context.WithCancel(ctx)
	defer collabCancel()
	go collabHub.Run(collabCtx)
	wsTokenValidator := auth.NewMiddleware(jwks, cfg.KeycloakURL, cfg.KeycloakRealm, cfg.KeycloakClientID, logger, auditLogger)
	collabHandler := collaboration.NewHandler(collabHub, pool, wsTokenValidator, roleRepo, auditLogger, logger, cfg.CORSOrigins)
	collabHandler.SetOrgMembershipChecker(
		&organization.OrgMemberAdapter{MemberRepo: memberRepo},
		func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error) {
			c, err := caseRepo.FindByID(ctx, caseID)
			if err != nil {
				return uuid.Nil, err
			}
			return c.OrganizationID, nil
		},
	)

	// Sprint 11.5 — cleanup worker for notification_outbox processing.
	cleanupWorker := cleanup.NewWorker(pool, nil, nil, logger, 30*time.Second)
	cleanupCtx, cleanupCancel := context.WithCancel(ctx)
	defer cleanupCancel()
	go cleanupWorker.Run(cleanupCtx)

	healthHandler := server.NewHealthHandler(
		pool, minioStorage, cfg.MinIOBucket,
		cfg.MeilisearchURL, cfg.KeycloakURL, cfg.KeycloakRealm,
		version, auditLogger,
		server.WithBackupChecker(backupRunner),
	)

	// Investigation subsystem (Berkeley Protocol v2+v3)
	investigationRepo := investigation.NewPGRepository(pool)
	investigationSvc := investigation.NewService(investigationRepo, custodyLogger, logger).
		WithEvidenceOwnerChecker(&evidenceOwnerAdapter{repo: evidenceRepo}).
		WithCaseMembershipChecker(&caseMembershipAdapter{roleRepo: roleRepo})
	investigationHandler := investigation.NewHandler(investigationSvc, auditLogger)
	investigationHandler.SetOrgMembershipChecker(
		&organization.OrgMemberAdapter{MemberRepo: memberRepo},
		func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error) {
			c, err := caseRepo.FindByID(ctx, caseID)
			if err != nil {
				return uuid.Nil, err
			}
			return c.OrganizationID, nil
		},
	)

	httpServer := server.NewHTTPServer(cfg, logger, version, jwks, auditLogger, healthHandler,
		caseHandler, roleHandler, evidenceHandler, gdprRegistrar, notifHandler, searchHandler, integrityHandler,
		backupHandler, exportHandler, reportHandler, witnessHandler, disclosureHandler,
		pagesHandler, draftHandler, collabHandler, migrationHandler, bulkHandler, importHandler,
		investigationHandler, orgHandler, profileHandler, apiKeysHandler,
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

// runRetentionScheduler runs NotifyExpiringRetention on a 24h ticker until
// the provided context is cancelled. It fires once on startup so operators
// see immediate output in the logs, then every 24h thereafter. Errors are
// logged but do not terminate the loop.
func runRetentionScheduler(ctx context.Context, svc *evidence.Service, logger *slog.Logger) {
	const window = 30 * 24 * time.Hour
	run := func() {
		runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		count, err := svc.NotifyExpiringRetention(runCtx, window)
		if err != nil {
			logger.Error("retention scheduler run failed", "error", err)
			return
		}
		logger.Info("retention scheduler run complete", "notified", count)
	}

	// Fire once on startup.
	run()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

// migrationEvidenceWriter adapts evidence.Service to migration.EvidenceWriter,
// translating between the two packages' StoreInput/StoreResult types. The
// migration package defines its own narrow interface so it does not depend
// on the evidence package's full surface.
type migrationEvidenceWriter struct {
	svc *evidence.Service
}

func (w *migrationEvidenceWriter) StoreMigratedFile(ctx context.Context, in migration.StoreInput) (migration.StoreResult, error) {
	res, err := w.svc.StoreMigratedFile(ctx, evidence.MigrationStoreInput{
		CaseID:         in.CaseID,
		Filename:       in.Filename,
		OriginalName:   in.OriginalName,
		Reader:         in.Reader,
		SizeBytes:      in.SizeBytes,
		ComputedHash:   in.ComputedHash,
		SourceHash:     in.SourceHash,
		Classification: in.Classification,
		Description:    in.Description,
		Tags:           in.Tags,
		Source:         in.Source,
		SourceDate:     in.SourceDate,
		UploadedBy:     in.UploadedBy,
		CustodyDetail:  in.CustodyDetail,
	})
	if err != nil {
		return migration.StoreResult{}, err
	}
	return migration.StoreResult{EvidenceID: res.EvidenceID, SizeBytes: res.SizeBytes}, nil
}

// importMigrationAdapter bridges evidence.ImportRunner to migration.Service.
// It lives in cmd/server so the evidence package never takes a
// compile-time dependency on the migration package.
type importMigrationAdapter struct {
	svc *migration.Service
}

func (a *importMigrationAdapter) Run(ctx context.Context, in evidence.ImportRunInput) (evidence.ImportRunResult, error) {
	mf, err := os.Open(in.ManifestPath) // #nosec G304 — path is inside the server-owned import temp dir
	if err != nil {
		return evidence.ImportRunResult{}, fmt.Errorf("open manifest: %w", err)
	}
	defer mf.Close()

	res, err := a.svc.Run(ctx, migration.RunInput{
		CaseID:         in.CaseID,
		SourceSystem:   in.SourceSystem,
		PerformedBy:    in.PerformedBy,
		ManifestSource: mf,
		ManifestFormat: migration.FormatCSV,
		SourceRoot:     in.SourceRoot,
		Options: migration.BatchOptions{
			HaltOnMismatch: in.HaltOnMismatch,
			DryRun:         in.DryRun,
		},
	})
	if err != nil {
		return evidence.ImportRunResult{}, err
	}
	return evidence.ImportRunResult{
		MigrationID:     res.Record.ID,
		TotalItems:      res.Record.TotalItems,
		MatchedItems:    res.Record.MatchedItems,
		MismatchedItems: res.Record.MismatchedItems,
		Status:          string(res.Record.Status),
		TSAName:         res.Record.TSAName,
		TSATimestamp:    res.Record.TSATimestamp,
	}, nil
}

// migrationCaseLookup adapts cases.Service to migration.CaseLookup. The
// single-call interface avoids the dual DB round-trip pattern the
// earlier two-method shape produced.
type migrationCaseLookup struct {
	svc *cases.Service
}

func (l *migrationCaseLookup) GetCaseInfo(ctx context.Context, id uuid.UUID) (migration.CaseInfo, error) {
	c, err := l.svc.GetCase(ctx, id)
	if err != nil {
		return migration.CaseInfo{}, err
	}
	return migration.CaseInfo{
		ReferenceCode: c.ReferenceCode,
		Title:         c.Title,
	}, nil
}

// disclosureNotificationAdapter bridges disclosure notifications to the notification service.
type disclosureNotificationAdapter struct {
	notifService *notifications.Service
}

func (a *disclosureNotificationAdapter) NotifyDisclosure(ctx context.Context, caseID uuid.UUID, disclosedTo, title, body string) error {
	return a.notifService.Notify(ctx, notifications.NotificationEvent{
		Type:   "evidence_disclosed",
		CaseID: caseID,
		Title:  title,
		Body:   body,
	})
}
