-- VaultKeeper Initial Schema
-- 9 core tables for evidence management platform

BEGIN;

-- Cases: top-level organizational unit
CREATE TABLE cases (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reference_code TEXT NOT NULL UNIQUE,
    title          TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    jurisdiction   TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'closed', 'archived')),
    created_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Case roles: maps users to cases with specific roles
CREATE TABLE case_roles (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id   UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL,
    role      TEXT NOT NULL
        CHECK (role IN ('investigator', 'prosecutor', 'defence', 'judge', 'observer', 'victim_representative')),
    granted_by UUID NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (case_id, user_id, role)
);

-- Evidence items: metadata for files stored in MinIO
CREATE TABLE evidence_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id         UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    filename        TEXT NOT NULL,
    original_name   TEXT NOT NULL,
    mime_type       TEXT NOT NULL,
    size_bytes      BIGINT NOT NULL,
    sha256_hash     TEXT NOT NULL,
    classification  TEXT NOT NULL DEFAULT 'restricted'
        CHECK (classification IN ('public', 'restricted', 'confidential', 'ex_parte')),
    uploaded_by     UUID NOT NULL,
    is_current      BOOLEAN NOT NULL DEFAULT true,
    version         INTEGER NOT NULL DEFAULT 1,
    tsa_token       BYTEA,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Custody log: immutable, append-only chain of custody
CREATE TABLE custody_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id         UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    evidence_id     UUID NOT NULL REFERENCES evidence_items(id) ON DELETE RESTRICT,
    action          TEXT NOT NULL,
    actor_user_id   UUID NOT NULL,
    detail          TEXT NOT NULL DEFAULT '',
    hash_value      TEXT NOT NULL,
    previous_hash   TEXT NOT NULL DEFAULT '',
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Witnesses: protected identity information
CREATE TABLE witnesses (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id            UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    pseudonym          TEXT NOT NULL,
    full_name_encrypted BYTEA,
    contact_info_encrypted BYTEA,
    location_encrypted  BYTEA,
    protection_status  TEXT NOT NULL DEFAULT 'standard'
        CHECK (protection_status IN ('standard', 'protected', 'high_risk')),
    created_by         UUID NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Disclosures: tracking evidence shared between parties
CREATE TABLE disclosures (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id       UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    evidence_id   UUID NOT NULL REFERENCES evidence_items(id) ON DELETE RESTRICT,
    disclosed_to  UUID NOT NULL,
    disclosed_by  UUID NOT NULL,
    disclosed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    notes         TEXT NOT NULL DEFAULT ''
);

-- Notifications: in-app notification system
CREATE TABLE notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id    UUID REFERENCES cases(id) ON DELETE SET NULL,
    user_id    UUID NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    read       BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- API keys: external system integration
CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    name         TEXT NOT NULL,
    key_hash     TEXT NOT NULL,
    permissions  TEXT NOT NULL DEFAULT 'read'
        CHECK (permissions IN ('read', 'read_write')),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Backup log: tracking backup operations
CREATE TABLE backup_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ,
    status        TEXT NOT NULL DEFAULT 'started'
        CHECK (status IN ('started', 'completed', 'failed')),
    size_bytes    BIGINT,
    destination   TEXT NOT NULL,
    error_message TEXT
);

COMMIT;
