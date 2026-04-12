-- Migration 017 (down): revert retention columns and erasure_requests table.

DROP TABLE IF EXISTS erasure_requests;

DROP INDEX IF EXISTS idx_evidence_items_retention_until;

ALTER TABLE cases
    DROP COLUMN IF EXISTS retention_until;

ALTER TABLE evidence_items
    DROP COLUMN IF EXISTS destruction_authority,
    DROP COLUMN IF EXISTS retention_until;
