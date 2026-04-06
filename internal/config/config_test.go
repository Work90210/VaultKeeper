package config

import (
	"strings"
	"testing"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/vaultkeeper")
	t.Setenv("MINIO_ENDPOINT", "minio.internal:9000")
	t.Setenv("MINIO_ACCESS_KEY", "minio-access")
	t.Setenv("MINIO_SECRET_KEY", "minio-secret")
	t.Setenv("MINIO_BUCKET", "vaultkeeper-evidence")
	t.Setenv("KEYCLOAK_URL", "https://auth.example.com")
	t.Setenv("KEYCLOAK_REALM", "vaultkeeper")
	t.Setenv("KEYCLOAK_CLIENT_ID", "vaultkeeper-api")
	t.Setenv("TSA_URL", "https://tsa.example.com")
	t.Setenv("MEILISEARCH_URL", "https://search.example.com")
	t.Setenv("MEILISEARCH_API_KEY", "meili-secret")
	t.Setenv("APP_URL", "https://app.example.com")
	t.Setenv("APP_ENV", "development")
	t.Setenv("BACKUP_ENC_KEY", "backup-secret")
	t.Setenv("WITNESS_ENCRYPTION_KEY", "witness-secret")
	t.Setenv("MASTER_ENCRYPTION_KEY", "master-secret")
}

func TestLoadFromEnv_AllValid(t *testing.T) {
	setBaseEnv(t)

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://user:password@localhost:5432/vaultkeeper" {
		t.Fatalf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}
	if cfg.AppEnv != "development" {
		t.Fatalf("unexpected AppEnv: %s", cfg.AppEnv)
	}
}

func TestLoadFromEnv_MissingRequiredVars(t *testing.T) {
	required := []string{
		"DATABASE_URL",
		"MINIO_ENDPOINT",
		"MINIO_ACCESS_KEY",
		"MINIO_SECRET_KEY",
		"MINIO_BUCKET",
		"KEYCLOAK_URL",
		"KEYCLOAK_REALM",
		"KEYCLOAK_CLIENT_ID",
		"TSA_URL",
		"MEILISEARCH_URL",
		"MEILISEARCH_API_KEY",
		"APP_URL",
		"APP_ENV",
	}

	for _, key := range required {
		t.Run(key, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv(key, "")

			_, err := LoadFromEnv()
			if err == nil {
				t.Fatalf("expected error for missing %s", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Fatalf("expected error to mention %s, got %v", key, err)
			}
		})
	}
}

func TestLoadFromEnv_DefaultValues(t *testing.T) {
	setBaseEnv(t)

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.MinIOUseSSL {
		t.Fatal("expected default MinIOUseSSL=true")
	}
	if !cfg.TSAEnabled {
		t.Fatal("expected default TSAEnabled=true")
	}
	if cfg.LogLevel.String() != "INFO" {
		t.Fatalf("expected default log level INFO, got %s", cfg.LogLevel.String())
	}
	if cfg.MaxUploadSize != DefaultMaxUploadSize {
		t.Fatalf("unexpected MaxUploadSize: %d", cfg.MaxUploadSize)
	}
	if cfg.ServerPort != DefaultServerPort {
		t.Fatalf("unexpected ServerPort: %d", cfg.ServerPort)
	}
	if cfg.MaxConcurrentSessions != DefaultMaxConcurrentSessions {
		t.Fatalf("unexpected MaxConcurrentSessions: %d", cfg.MaxConcurrentSessions)
	}
	if cfg.CaseReferenceRegex != DefaultCaseReferenceRegex {
		t.Fatalf("unexpected CaseReferenceRegex: %s", cfg.CaseReferenceRegex)
	}
}

func TestLoadFromEnv_ParsesOptionalValues(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("MINIO_USE_SSL", "false")
	t.Setenv("TSA_ENABLED", "false")
	t.Setenv("TSA_URL", "")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("MAX_UPLOAD_SIZE", "5368709120")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("MAX_CONCURRENT_SESSIONS", "9")
	t.Setenv("CASE_REFERENCE_REGEX", "^[0-9]+$")
	t.Setenv("CORS_ORIGINS", "https://a.example, https://b.example")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MinIOUseSSL {
		t.Fatal("expected MinIOUseSSL=false")
	}
	if cfg.TSAEnabled {
		t.Fatal("expected TSAEnabled=false")
	}
	if cfg.LogLevel.String() != "DEBUG" {
		t.Fatalf("expected DEBUG level, got %s", cfg.LogLevel.String())
	}
	if cfg.MaxUploadSize != 5368709120 {
		t.Fatalf("unexpected MaxUploadSize: %d", cfg.MaxUploadSize)
	}
	if cfg.ServerPort != 9090 {
		t.Fatalf("unexpected ServerPort: %d", cfg.ServerPort)
	}
	if cfg.MaxConcurrentSessions != 9 {
		t.Fatalf("unexpected MaxConcurrentSessions: %d", cfg.MaxConcurrentSessions)
	}
	if cfg.CaseReferenceRegex != "^[0-9]+$" {
		t.Fatalf("unexpected CaseReferenceRegex: %s", cfg.CaseReferenceRegex)
	}
	if len(cfg.CORSOrigins) != 2 {
		t.Fatalf("expected 2 CORS origins, got %d", len(cfg.CORSOrigins))
	}
}

func TestLoadFromEnv_InvalidValues(t *testing.T) {
	cases := []struct {
		name     string
		key      string
		value    string
		contains string
	}{
		{"invalid database scheme", "DATABASE_URL", "mysql://db", "DATABASE_URL"},
		{"invalid keycloak url", "KEYCLOAK_URL", "://bad", "KEYCLOAK_URL"},
		{"invalid tsa url", "TSA_URL", "://bad", "TSA_URL"},
		{"invalid meili url", "MEILISEARCH_URL", "://bad", "MEILISEARCH_URL"},
		{"invalid app url", "APP_URL", "://bad", "APP_URL"},
		{"invalid minio bool", "MINIO_USE_SSL", "banana", "MINIO_USE_SSL"},
		{"invalid tsa bool", "TSA_ENABLED", "banana", "TSA_ENABLED"},
		{"invalid smtp port", "SMTP_PORT", "banana", "SMTP_PORT"},
		{"negative smtp port", "SMTP_PORT", "-1", "SMTP_PORT"},
		{"invalid server port", "SERVER_PORT", "banana", "SERVER_PORT"},
		{"zero server port", "SERVER_PORT", "0", "SERVER_PORT"},
		{"server port too high", "SERVER_PORT", "70000", "SERVER_PORT"},
		{"invalid upload size", "MAX_UPLOAD_SIZE", "banana", "MAX_UPLOAD_SIZE"},
		{"negative upload size", "MAX_UPLOAD_SIZE", "-1", "MAX_UPLOAD_SIZE"},
		{"invalid max sessions", "MAX_CONCURRENT_SESSIONS", "banana", "MAX_CONCURRENT_SESSIONS"},
		{"zero max sessions", "MAX_CONCURRENT_SESSIONS", "0", "MAX_CONCURRENT_SESSIONS"},
		{"invalid app env", "APP_ENV", "qa", "APP_ENV"},
		{"invalid log level", "LOG_LEVEL", "trace", "LOG_LEVEL"},
		{"invalid regex", "CASE_REFERENCE_REGEX", "[", "CASE_REFERENCE_REGEX"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv(tc.key, tc.value)

			_, err := LoadFromEnv()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("expected error to mention %s, got %v", tc.contains, err)
			}
		})
	}
}

func TestLoadFromEnv_SMTPPartialWarning(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("SMTP_HOST", "smtp.example.com")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Warnings) != 1 {
		t.Fatalf("expected one warning, got %d", len(cfg.Warnings))
	}
	if !strings.Contains(cfg.Warnings[0], "SMTP configuration is partial") {
		t.Fatalf("unexpected warning: %v", cfg.Warnings[0])
	}
}

func TestLoadFromEnv_ProductionRequiresEncryptionKeys(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("WITNESS_ENCRYPTION_KEY", "")
	t.Setenv("MASTER_ENCRYPTION_KEY", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected production validation error")
	}
	if !strings.Contains(err.Error(), "WITNESS_ENCRYPTION_KEY") {
		t.Fatalf("expected WITNESS_ENCRYPTION_KEY error, got %v", err)
	}
	if !strings.Contains(err.Error(), "MASTER_ENCRYPTION_KEY") {
		t.Fatalf("expected MASTER_ENCRYPTION_KEY error, got %v", err)
	}
}

func TestLoadFromEnv_DevelopmentDoesNotRequireEncryptionKeys(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("WITNESS_ENCRYPTION_KEY", "")
	t.Setenv("MASTER_ENCRYPTION_KEY", "")

	_, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error in development: %v", err)
	}
}

func TestConfigString_RedactsSecrets(t *testing.T) {
	setBaseEnv(t)
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	value := cfg.String()
	secrets := []string{
		"postgres://user:password@localhost:5432/vaultkeeper",
		"minio-access",
		"minio-secret",
		"meili-secret",
		"backup-secret",
		"witness-secret",
		"master-secret",
	}
	for _, secret := range secrets {
		if strings.Contains(value, secret) {
			t.Fatalf("redacted config leaked secret %q", secret)
		}
	}
	if !strings.Contains(value, "[REDACTED]") {
		t.Fatal("expected redacted marker in config string")
	}
}

func TestLoadFromEnv_DatabaseURLWithPostgresqlScheme(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("DATABASE_URL", "postgresql://user:password@localhost:5432/vaultkeeper")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgresql://user:password@localhost:5432/vaultkeeper" {
		t.Fatalf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}
}

func TestLoadFromEnv_BooleanParsing(t *testing.T) {
	boolValues := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
	}

	for _, tc := range boolValues {
		t.Run("MINIO_USE_SSL="+tc.value, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("MINIO_USE_SSL", tc.value)

			cfg, err := LoadFromEnv()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.MinIOUseSSL != tc.expected {
				t.Fatalf("expected MinIOUseSSL=%v for input %q", tc.expected, tc.value)
			}
		})
	}
}

func TestLoadFromEnv_BackupEncKeyRequiredWithDestination(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("BACKUP_DESTINATION", "/backups")
	t.Setenv("BACKUP_ENC_KEY", "some-key")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BackupDestination != "/backups" {
		t.Fatalf("unexpected BackupDestination: %s", cfg.BackupDestination)
	}
}
