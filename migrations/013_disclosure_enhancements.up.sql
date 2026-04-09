-- Sprint 8: Disclosure enhancements
BEGIN;

-- Change disclosed_to from UUID to TEXT (supports role names like 'defence' per sprint plan)
ALTER TABLE disclosures ALTER COLUMN disclosed_to TYPE TEXT;

-- Add redacted flag to disclosures
ALTER TABLE disclosures
    ADD COLUMN IF NOT EXISTS redacted BOOLEAN NOT NULL DEFAULT false;

-- Index for case-level disclosure queries
CREATE INDEX IF NOT EXISTS idx_disclosures_case_id ON disclosures (case_id);

-- Index for evidence-level disclosure lookups (defence visibility)
CREATE INDEX IF NOT EXISTS idx_disclosures_evidence_id ON disclosures (evidence_id);

-- Composite index for defence evidence filtering
CREATE INDEX IF NOT EXISTS idx_disclosures_case_evidence ON disclosures (case_id, evidence_id);

COMMIT;
