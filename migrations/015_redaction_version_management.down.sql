-- Migration 015 down: Revert redaction version management

DROP INDEX IF EXISTS idx_evidence_items_evidence_number;
DROP INDEX IF EXISTS idx_evidence_redaction_parent;

ALTER TABLE evidence_items
    DROP COLUMN IF EXISTS redaction_name,
    DROP COLUMN IF EXISTS redaction_purpose,
    DROP COLUMN IF EXISTS redaction_area_count,
    DROP COLUMN IF EXISTS redaction_author_id,
    DROP COLUMN IF EXISTS redaction_finalized_at;

DROP INDEX IF EXISTS idx_redaction_drafts_evidence_status;
DROP INDEX IF EXISTS idx_redaction_drafts_unique_name;

ALTER TABLE redaction_drafts
    DROP COLUMN IF EXISTS name,
    DROP COLUMN IF EXISTS purpose,
    DROP COLUMN IF EXISTS area_count;

-- Restore original single-draft-per-evidence constraint
CREATE UNIQUE INDEX idx_redaction_drafts_evidence_draft
    ON redaction_drafts(evidence_id) WHERE status = 'draft';

DROP TYPE IF EXISTS redaction_purpose;
