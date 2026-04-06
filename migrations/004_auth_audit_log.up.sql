-- Auth audit log for authentication and authorization events
-- Separate from custody_log which requires case_id/evidence_id

BEGIN;

CREATE TABLE auth_audit_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    action          TEXT NOT NULL,
    actor_user_id   TEXT NOT NULL DEFAULT '',
    ip_address      TEXT NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    detail          JSONB NOT NULL DEFAULT '{}',
    hash_value      TEXT NOT NULL,
    previous_hash   TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_auth_audit_log_actor ON auth_audit_log (actor_user_id);
CREATE INDEX idx_auth_audit_log_action ON auth_audit_log (action);
CREATE INDEX idx_auth_audit_log_created_at ON auth_audit_log (created_at);

COMMIT;
