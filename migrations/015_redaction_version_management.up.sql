-- Migration 015: Redaction Version Management
-- Adds named multi-draft support and redaction metadata on evidence items.
-- golang-migrate runs each migration in a transaction automatically.

-- 1. Purpose enumeration
CREATE TYPE redaction_purpose AS ENUM (
    'disclosure_defence',
    'disclosure_prosecution',
    'public_release',
    'court_submission',
    'witness_protection',
    'internal_review'
);

-- 2. Enhance redaction_drafts — support multiple named drafts per evidence
ALTER TABLE redaction_drafts
    ADD COLUMN name TEXT,
    ADD COLUMN purpose redaction_purpose,
    ADD COLUMN area_count INTEGER NOT NULL DEFAULT 0 CHECK (area_count >= 0);

-- Backfill existing drafts with sensible defaults
UPDATE redaction_drafts
SET name    = 'Draft ' || to_char(created_at, 'YYYY-MM-DD HH24:MI'),
    purpose = 'internal_review';

ALTER TABLE redaction_drafts
    ALTER COLUMN name SET NOT NULL,
    ALTER COLUMN purpose SET NOT NULL;

-- Replace single-draft-per-evidence constraint with per-name uniqueness.
-- Uses status != 'discarded' (not just status = 'draft') so that applied drafts
-- also block name reuse — prevents audit trail collision between applied and draft.
DROP INDEX IF EXISTS idx_redaction_drafts_evidence_draft;
CREATE UNIQUE INDEX idx_redaction_drafts_unique_name
    ON redaction_drafts (evidence_id, lower(name)) WHERE status != 'discarded';
CREATE INDEX idx_redaction_drafts_evidence_status
    ON redaction_drafts (evidence_id, last_saved_at DESC)
    WHERE status != 'discarded';

-- 3. Add redaction metadata to evidence_items — self-describing finalized copies
ALTER TABLE evidence_items
    ADD COLUMN redaction_name         TEXT,
    ADD COLUMN redaction_purpose      redaction_purpose,
    ADD COLUMN redaction_area_count   INTEGER,
    ADD COLUMN redaction_author_id    UUID,
    ADD COLUMN redaction_finalized_at TIMESTAMPTZ;

-- Optimize queries for listing derivatives of an original.
-- Includes redaction_name IS NOT NULL predicate to match ListFinalizedRedactions query.
CREATE INDEX idx_evidence_redaction_parent
    ON evidence_items (parent_id, created_at DESC)
    WHERE parent_id IS NOT NULL AND redaction_name IS NOT NULL;

-- Deduplicate existing evidence numbers created by the old redaction system
-- (which appended just "-R" without a unique suffix). Append a sequential suffix.
UPDATE evidence_items e
SET evidence_number = e.evidence_number || '-' || row_num::text
FROM (
    SELECT id, evidence_number,
           ROW_NUMBER() OVER (PARTITION BY evidence_number ORDER BY created_at) AS row_num
    FROM evidence_items
    WHERE evidence_number IS NOT NULL AND destroyed_at IS NULL
) sub
WHERE e.id = sub.id AND sub.row_num > 1;

-- Enforce unique evidence numbers to prevent race conditions during finalization.
-- Excludes destroyed items (which retain their number for audit trail).
CREATE UNIQUE INDEX idx_evidence_items_evidence_number
    ON evidence_items (evidence_number)
    WHERE evidence_number IS NOT NULL AND destroyed_at IS NULL;
