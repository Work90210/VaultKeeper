-- Migration 017: Retention policies, audited destruction, and GDPR erasure requests.
--
-- Adds:
--   * evidence_items.retention_until          — soonest date the item may be destroyed
--   * evidence_items.destruction_authority    — free-text legal authority recorded at destruction
--   * cases.retention_until                   — case-level retention floor (max of item + case wins)
--   * erasure_requests                        — GDPR erasure workflow with conflict resolution

ALTER TABLE evidence_items
    ADD COLUMN retention_until TIMESTAMPTZ,
    ADD COLUMN destruction_authority TEXT;

ALTER TABLE cases
    ADD COLUMN retention_until TIMESTAMPTZ;

CREATE INDEX idx_evidence_items_retention_until
    ON evidence_items (retention_until)
    WHERE retention_until IS NOT NULL;

CREATE TABLE erasure_requests (
    id             UUID PRIMARY KEY,
    evidence_id    UUID NOT NULL REFERENCES evidence_items(id) ON DELETE CASCADE,
    requested_by   TEXT NOT NULL,
    rationale      TEXT NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('ready', 'conflict_pending', 'resolved_preserve', 'resolved_erase')),
    decision       TEXT,
    decided_by     TEXT,
    decided_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_erasure_requests_evidence ON erasure_requests(evidence_id);
CREATE INDEX idx_erasure_requests_status   ON erasure_requests(status);
