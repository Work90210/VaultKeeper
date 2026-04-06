-- Performance indexes for VaultKeeper core queries

BEGIN;

-- Evidence lookups by case
CREATE INDEX idx_evidence_case_id ON evidence_items (case_id);

-- Current version evidence per case (partial index)
CREATE INDEX idx_evidence_case_current ON evidence_items (case_id)
    WHERE is_current = true;

-- Custody log lookups by evidence
CREATE INDEX idx_custody_evidence_id ON custody_log (evidence_id);

-- Custody log lookups by case
CREATE INDEX idx_custody_case_id ON custody_log (case_id);

-- Case roles by user (which cases can this user see)
CREATE INDEX idx_case_roles_user ON case_roles (user_id);

-- Case roles by case (who is on this case)
CREATE INDEX idx_case_roles_case ON case_roles (case_id);

-- Unread notifications per user (partial index)
CREATE INDEX idx_notifications_user_unread ON notifications (user_id)
    WHERE read = false;

-- API key lookup by hash (partial index, active keys only)
CREATE INDEX idx_api_keys_hash ON api_keys (key_hash)
    WHERE revoked_at IS NULL;

-- Disclosures by case
CREATE INDEX idx_disclosures_case ON disclosures (case_id);

-- Witnesses by case
CREATE INDEX idx_witnesses_case ON witnesses (case_id);

-- Custody log timestamp ordering
CREATE INDEX idx_custody_timestamp ON custody_log (timestamp);

COMMIT;
