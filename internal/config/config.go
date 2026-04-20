package config

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	DefaultMinIOUseSSL                  = true
	DefaultTSAEnabled                   = true
	DefaultMaxUploadSize          int64 = 10 * 1024 * 1024 * 1024
	DefaultServerPort                   = 8080
	DefaultSMTPPort                     = 587
	DefaultMaxConcurrentSessions        = 3
	DefaultCaseReferenceRegex           = `^[A-Za-z0-9][A-Za-z0-9/._-]{1,98}[A-Za-z0-9]$`
)

type Config struct {
	DatabaseURL           string
	MinIOEndpoint         string
	MinIOAccessKey        string
	MinIOSecretKey        string
	MinIOBucket           string
	MinIOUseSSL           bool
	KeycloakURL           string
	KeycloakRealm         string
	KeycloakClientID      string
	TSAURL                string
	TSAEnabled            bool
	MeilisearchURL        string
	MeilisearchAPIKey     string
	SMTPHost              string
	SMTPPort              int
	SMTPUsername          string
	SMTPPassword          string
	SMTPFrom              string
	AppURL                string
	AppEnv                string
	LogLevel              slog.Level
	MaxUploadSize         int64
	ServerPort            int
	WitnessEncryptionKey      string
	WitnessEncryptionKeyBytes []byte
	MasterEncryptionKey       string
	MasterEncryptionKeyBytes  []byte
	MaxConcurrentSessions int
	CaseReferenceRegex    string
	CORSOrigins           []string
	BackupDestination     string
	BackupEncKey          string
	BackupEncKeyBytes     []byte
	ArchiveStorageBucket  string
	ArchiveStoragePath    string
	AdminUserIDs          []string
	// MigrationStagingRoot is the allowlisted filesystem prefix under
	// which Sprint 10 migration manifests and evidence directories MUST
	// reside. The migration HTTP handler refuses any manifest_path /
	// source_root that does not live under this root. Empty means
	// migration HTTP ingestion is disabled (CLI dry-run still works).
	MigrationStagingRoot string

	// Federation — instance identity for cross-border evidence exchange.
	InstanceID          string
	InstanceDisplayName string

	Warnings []string
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		MinIOUseSSL:           DefaultMinIOUseSSL,
		TSAEnabled:            DefaultTSAEnabled,
		LogLevel:              slog.LevelInfo,
		MaxUploadSize:         DefaultMaxUploadSize,
		ServerPort:            DefaultServerPort,
		SMTPPort:              DefaultSMTPPort,
		MaxConcurrentSessions: DefaultMaxConcurrentSessions,
		CaseReferenceRegex:    DefaultCaseReferenceRegex,
	}

	var errs []string

	cfg.DatabaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if cfg.DatabaseURL == "" {
		errs = append(errs, "DATABASE_URL is required")
	} else if err := validateDatabaseURL(cfg.DatabaseURL); err != nil {
		errs = append(errs, fmt.Sprintf("DATABASE_URL %v", err))
	}

	cfg.MinIOEndpoint = strings.TrimSpace(os.Getenv("MINIO_ENDPOINT"))
	if cfg.MinIOEndpoint == "" {
		errs = append(errs, "MINIO_ENDPOINT is required")
	}

	cfg.MinIOAccessKey = os.Getenv("MINIO_ACCESS_KEY")
	if cfg.MinIOAccessKey == "" {
		errs = append(errs, "MINIO_ACCESS_KEY is required")
	}

	cfg.MinIOSecretKey = os.Getenv("MINIO_SECRET_KEY")
	if cfg.MinIOSecretKey == "" {
		errs = append(errs, "MINIO_SECRET_KEY is required")
	}

	cfg.MinIOBucket = strings.TrimSpace(os.Getenv("MINIO_BUCKET"))
	if cfg.MinIOBucket == "" {
		errs = append(errs, "MINIO_BUCKET is required")
	}

	if value := strings.TrimSpace(os.Getenv("MINIO_USE_SSL")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			errs = append(errs, "MINIO_USE_SSL must be a boolean")
		} else {
			cfg.MinIOUseSSL = parsed
		}
	}

	cfg.KeycloakURL = strings.TrimSpace(os.Getenv("KEYCLOAK_URL"))
	if cfg.KeycloakURL == "" {
		errs = append(errs, "KEYCLOAK_URL is required")
	} else if err := validateURL(cfg.KeycloakURL); err != nil {
		errs = append(errs, fmt.Sprintf("KEYCLOAK_URL %v", err))
	}

	cfg.KeycloakRealm = strings.TrimSpace(os.Getenv("KEYCLOAK_REALM"))
	if cfg.KeycloakRealm == "" {
		errs = append(errs, "KEYCLOAK_REALM is required")
	}

	cfg.KeycloakClientID = strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_ID"))
	if cfg.KeycloakClientID == "" {
		errs = append(errs, "KEYCLOAK_CLIENT_ID is required")
	}

	if value := strings.TrimSpace(os.Getenv("TSA_ENABLED")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			errs = append(errs, "TSA_ENABLED must be a boolean")
		} else {
			cfg.TSAEnabled = parsed
		}
	}

	cfg.TSAURL = strings.TrimSpace(os.Getenv("TSA_URL"))
	if cfg.TSAEnabled {
		if cfg.TSAURL == "" {
			errs = append(errs, "TSA_URL is required when TSA_ENABLED=true")
		} else if err := validateURL(cfg.TSAURL); err != nil {
			errs = append(errs, fmt.Sprintf("TSA_URL %v", err))
		}
	} else if cfg.TSAURL != "" {
		if err := validateURL(cfg.TSAURL); err != nil {
			errs = append(errs, fmt.Sprintf("TSA_URL %v", err))
		}
	}

	cfg.MeilisearchURL = strings.TrimSpace(os.Getenv("MEILISEARCH_URL"))
	if cfg.MeilisearchURL == "" {
		errs = append(errs, "MEILISEARCH_URL is required")
	} else if err := validateURL(cfg.MeilisearchURL); err != nil {
		errs = append(errs, fmt.Sprintf("MEILISEARCH_URL %v", err))
	}

	cfg.MeilisearchAPIKey = os.Getenv("MEILISEARCH_API_KEY")
	if cfg.MeilisearchAPIKey == "" {
		errs = append(errs, "MEILISEARCH_API_KEY is required")
	}

	cfg.SMTPHost = strings.TrimSpace(os.Getenv("SMTP_HOST"))
	cfg.SMTPUsername = strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	cfg.SMTPPassword = os.Getenv("SMTP_PASSWORD")
	cfg.SMTPFrom = strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if value := strings.TrimSpace(os.Getenv("SMTP_PORT")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			errs = append(errs, "SMTP_PORT must be an integer")
		} else if parsed <= 0 {
			errs = append(errs, "SMTP_PORT must be greater than zero")
		} else {
			cfg.SMTPPort = parsed
		}
	}

	smtpFields := []bool{
		cfg.SMTPHost != "",
		cfg.SMTPUsername != "",
		cfg.SMTPPassword != "",
		cfg.SMTPFrom != "",
	}
	smtpConfigured := 0
	for _, configured := range smtpFields {
		if configured {
			smtpConfigured++
		}
	}
	if smtpConfigured > 0 && smtpConfigured < len(smtpFields) {
		cfg.Warnings = append(cfg.Warnings, "SMTP configuration is partial; email delivery remains disabled until all SMTP_* fields are set")
	}

	cfg.AppURL = strings.TrimSpace(os.Getenv("APP_URL"))
	if cfg.AppURL == "" {
		errs = append(errs, "APP_URL is required")
	} else if err := validateURL(cfg.AppURL); err != nil {
		errs = append(errs, fmt.Sprintf("APP_URL %v", err))
	}

	cfg.AppEnv = strings.TrimSpace(os.Getenv("APP_ENV"))
	switch cfg.AppEnv {
	case "":
		errs = append(errs, "APP_ENV is required")
	case "development", "staging", "production":
	default:
		errs = append(errs, "APP_ENV must be one of development|staging|production")
	}

	if value := strings.TrimSpace(os.Getenv("LOG_LEVEL")); value != "" {
		var level slog.Level
		if err := level.UnmarshalText([]byte(value)); err != nil {
			errs = append(errs, "LOG_LEVEL must be one of debug|info|warn|error")
		} else {
			cfg.LogLevel = level
		}
	}

	if value := strings.TrimSpace(os.Getenv("MAX_UPLOAD_SIZE")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 {
			errs = append(errs, "MAX_UPLOAD_SIZE must be a positive integer")
		} else {
			cfg.MaxUploadSize = parsed
		}
	}

	if value := strings.TrimSpace(os.Getenv("SERVER_PORT")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			errs = append(errs, "SERVER_PORT must be an integer")
		} else if parsed <= 0 || parsed > 65535 {
			errs = append(errs, "SERVER_PORT must be between 1 and 65535")
		} else {
			cfg.ServerPort = parsed
		}
	}

	cfg.WitnessEncryptionKey = os.Getenv("WITNESS_ENCRYPTION_KEY")
	cfg.MasterEncryptionKey = os.Getenv("MASTER_ENCRYPTION_KEY")
	if cfg.AppEnv == "production" {
		if cfg.WitnessEncryptionKey == "" {
			errs = append(errs, "WITNESS_ENCRYPTION_KEY is required in production")
		}
		if cfg.MasterEncryptionKey == "" {
			errs = append(errs, "MASTER_ENCRYPTION_KEY is required in production")
		}
	}
	// Enforce minimum key length (32 bytes / 64 hex chars) when a key is supplied,
	// regardless of environment, to catch misconfiguration before runtime failures.
	if cfg.WitnessEncryptionKey != "" {
		if len(cfg.WitnessEncryptionKey) < 64 {
			errs = append(errs, "WITNESS_ENCRYPTION_KEY must be at least 64 hex characters (32 bytes)")
		} else {
			decoded, err := hex.DecodeString(cfg.WitnessEncryptionKey)
			if err != nil {
				errs = append(errs, "WITNESS_ENCRYPTION_KEY must be valid hex")
			} else {
				cfg.WitnessEncryptionKeyBytes = decoded
			}
		}
	}
	if cfg.MasterEncryptionKey != "" {
		if len(cfg.MasterEncryptionKey) < 64 {
			errs = append(errs, "MASTER_ENCRYPTION_KEY must be at least 64 hex characters (32 bytes)")
		} else {
			decoded, err := hex.DecodeString(cfg.MasterEncryptionKey)
			if err != nil {
				errs = append(errs, "MASTER_ENCRYPTION_KEY must be valid hex")
			} else {
				cfg.MasterEncryptionKeyBytes = decoded
			}
		}
	}

	if value := strings.TrimSpace(os.Getenv("MAX_CONCURRENT_SESSIONS")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			errs = append(errs, "MAX_CONCURRENT_SESSIONS must be an integer")
		} else if parsed <= 0 {
			errs = append(errs, "MAX_CONCURRENT_SESSIONS must be greater than zero")
		} else {
			cfg.MaxConcurrentSessions = parsed
		}
	}

	if value := strings.TrimSpace(os.Getenv("CASE_REFERENCE_REGEX")); value != "" {
		if _, err := regexp.Compile(value); err != nil {
			errs = append(errs, "CASE_REFERENCE_REGEX must be a valid regular expression")
		} else {
			cfg.CaseReferenceRegex = value
		}
	}

	cfg.CORSOrigins = parseCSV(os.Getenv("CORS_ORIGINS"))
	cfg.BackupDestination = strings.TrimSpace(os.Getenv("BACKUP_DESTINATION"))
	cfg.BackupEncKey = os.Getenv("BACKUP_ENC_KEY")
	if cfg.BackupDestination != "" && cfg.BackupEncKey == "" {
		errs = append(errs, "BACKUP_ENC_KEY is required when BACKUP_DESTINATION is set")
	}
	if cfg.BackupEncKey != "" && len(cfg.BackupEncKey) < 64 {
		errs = append(errs, "BACKUP_ENC_KEY must be at least 64 hex characters (32 bytes)")
	}
	if cfg.BackupEncKey != "" {
		decoded, err := hex.DecodeString(cfg.BackupEncKey)
		if err != nil || len(decoded) < 32 {
			errs = append(errs, "BACKUP_ENC_KEY must be valid hex encoding 32+ bytes")
		} else {
			cfg.BackupEncKeyBytes = decoded
		}
	}
	cfg.ArchiveStorageBucket = strings.TrimSpace(os.Getenv("ARCHIVE_STORAGE_BUCKET"))
	cfg.ArchiveStoragePath = strings.TrimSpace(os.Getenv("ARCHIVE_STORAGE_PATH"))
	cfg.AdminUserIDs = parseCSV(os.Getenv("ADMIN_USER_IDS"))
	cfg.MigrationStagingRoot = strings.TrimSpace(os.Getenv("MIGRATION_STAGING_ROOT"))

	// Federation identity (optional — defaults allow dev without config).
	cfg.InstanceID = strings.TrimSpace(os.Getenv("INSTANCE_ID"))
	if cfg.InstanceID == "" {
		cfg.InstanceID = "vaultkeeper-dev"
	}
	cfg.InstanceDisplayName = strings.TrimSpace(os.Getenv("INSTANCE_DISPLAY_NAME"))
	if cfg.InstanceDisplayName == "" {
		cfg.InstanceDisplayName = "VaultKeeper (dev)"
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("configuration validation failed: %s", strings.Join(errs, "; "))
	}

	return cfg, nil
}

func (c Config) String() string {
	view := map[string]any{
		"DatabaseURL":           redact(c.DatabaseURL),
		"MinIOEndpoint":         c.MinIOEndpoint,
		"MinIOAccessKey":        redact(c.MinIOAccessKey),
		"MinIOSecretKey":        redact(c.MinIOSecretKey),
		"MinIOBucket":           c.MinIOBucket,
		"MinIOUseSSL":           c.MinIOUseSSL,
		"KeycloakURL":           c.KeycloakURL,
		"KeycloakRealm":         c.KeycloakRealm,
		"KeycloakClientID":      c.KeycloakClientID,
		"TSAURL":                c.TSAURL,
		"TSAEnabled":            c.TSAEnabled,
		"MeilisearchURL":        c.MeilisearchURL,
		"MeilisearchAPIKey":     redact(c.MeilisearchAPIKey),
		"SMTPHost":              c.SMTPHost,
		"SMTPPort":              c.SMTPPort,
		"SMTPUsername":          c.SMTPUsername,
		"SMTPPassword":          redact(c.SMTPPassword),
		"SMTPFrom":              c.SMTPFrom,
		"AppURL":                c.AppURL,
		"AppEnv":                c.AppEnv,
		"LogLevel":              c.LogLevel.String(),
		"MaxUploadSize":         c.MaxUploadSize,
		"ServerPort":            c.ServerPort,
		"WitnessEncryptionKey":  redact(c.WitnessEncryptionKey),
		"MasterEncryptionKey":   redact(c.MasterEncryptionKey),
		"MaxConcurrentSessions": c.MaxConcurrentSessions,
		"CaseReferenceRegex":    c.CaseReferenceRegex,
		"CORSOrigins":           c.CORSOrigins,
		"BackupDestination":     c.BackupDestination,
		"BackupEncKey":          redact(c.BackupEncKey),
		"ArchiveStorageBucket":  c.ArchiveStorageBucket,
		"ArchiveStoragePath":    c.ArchiveStoragePath,
		"AdminUserIDs":          c.AdminUserIDs,
	}

	b, _ := json.Marshal(view)
	return string(b)
}

func validateDatabaseURL(value string) error {
	if !strings.HasPrefix(value, "postgres://") && !strings.HasPrefix(value, "postgresql://") {
		return fmt.Errorf("must start with postgres:// or postgresql://")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("must be a valid URL")
	}
	if parsed.Host == "" {
		return fmt.Errorf("must include a host")
	}
	return nil
}

func validateURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("must be a valid URL")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("must include scheme and host")
	}
	return nil
}

func parseCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func redact(value string) string {
	if value == "" {
		return ""
	}
	return "[REDACTED]"
}
