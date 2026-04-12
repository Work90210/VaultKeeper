-- Migration 019: Sprint 10 — data migration tracking + bulk upload jobs
--
-- Two new tables to support Sprint 10:
--
-- 1. evidence_migrations
--    Tracks each cryptographic migration event from an external system
--    (e.g. RelativityOne) into VaultKeeper. Holds the deterministic
--    migration hash, the RFC 3161 timestamp token for the migration event
--    itself, and counts of matched / mismatched items for attestation.
--
-- 2. bulk_upload_jobs
--    Tracks asynchronous bulk ZIP ingestion jobs. Status transitions:
--    extracting → processing → completed | completed_with_errors | failed.
--    processed_files JSONB records per-entry success/failure so a restarted
--    worker can resume without re-processing completed entries.

BEGIN;

CREATE TABLE evidence_migrations (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id            UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    source_system      TEXT NOT NULL CHECK (char_length(source_system) BETWEEN 1 AND 200),
    total_items        INT  NOT NULL CHECK (total_items >= 0),
    matched_items      INT  NOT NULL DEFAULT 0 CHECK (matched_items >= 0),
    mismatched_items   INT  NOT NULL DEFAULT 0 CHECK (mismatched_items >= 0),
    migration_hash     TEXT NOT NULL CHECK (migration_hash ~ '^[0-9a-f]{64}$'),
    manifest_hash      TEXT NOT NULL CHECK (manifest_hash  ~ '^[0-9a-f]{64}$'),
    tsa_token          BYTEA,
    tsa_name           TEXT,
    tsa_timestamp      TIMESTAMPTZ,
    performed_by       TEXT NOT NULL CHECK (char_length(performed_by) BETWEEN 1 AND 200),
    status             TEXT NOT NULL DEFAULT 'in_progress'
                          CHECK (status IN ('in_progress','completed','failed','halted_mismatch')),
    started_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_evidence_migrations_case ON evidence_migrations (case_id, started_at DESC);
CREATE INDEX idx_evidence_migrations_status ON evidence_migrations (status);

-- 3. migration_file_progress
--    Per-file resume state. Survives server restarts so a long migration
--    interrupted by a crash or deployment can resume without re-ingesting
--    completed files (the Sprint 10 DoD requires cross-restart resume).
--    file_path is the sanitised canonical path emitted by the parser.
CREATE TABLE migration_file_progress (
    migration_id UUID NOT NULL REFERENCES evidence_migrations(id) ON DELETE CASCADE,
    file_path    TEXT NOT NULL CHECK (char_length(file_path) BETWEEN 1 AND 4000),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (migration_id, file_path)
);

CREATE INDEX idx_migration_file_progress_migration ON migration_file_progress (migration_id);

CREATE TABLE bulk_upload_jobs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id          UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    archive_key      TEXT NOT NULL,
    archive_sha256   TEXT CHECK (archive_sha256 IS NULL OR archive_sha256 ~ '^[0-9a-f]{64}$'),
    total_files      INT  NOT NULL DEFAULT 0 CHECK (total_files >= 0),
    processed_files  INT  NOT NULL DEFAULT 0 CHECK (processed_files >= 0),
    failed_files     INT  NOT NULL DEFAULT 0 CHECK (failed_files >= 0),
    status           TEXT NOT NULL DEFAULT 'extracting'
                        CHECK (status IN ('extracting','processing','completed','completed_with_errors','failed')),
    progress         JSONB NOT NULL DEFAULT '{}'::jsonb,
    errors           JSONB NOT NULL DEFAULT '[]'::jsonb,
    performed_by     TEXT NOT NULL CHECK (char_length(performed_by) BETWEEN 1 AND 200),
    started_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at     TIMESTAMPTZ
);

CREATE INDEX idx_bulk_upload_jobs_case ON bulk_upload_jobs (case_id, started_at DESC);
CREATE INDEX idx_bulk_upload_jobs_status ON bulk_upload_jobs (status);

COMMIT;
