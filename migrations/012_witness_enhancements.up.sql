-- Sprint 7: Witness management enhancements
BEGIN;

-- Add witness_code (public identifier), statement_summary, related_evidence to witnesses
ALTER TABLE witnesses
    ADD COLUMN IF NOT EXISTS witness_code TEXT,
    ADD COLUMN IF NOT EXISTS statement_summary TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS related_evidence UUID[] NOT NULL DEFAULT '{}';

-- Backfill witness_code from pseudonym for existing rows
UPDATE witnesses SET witness_code = pseudonym WHERE witness_code IS NULL;

-- Make witness_code NOT NULL after backfill
ALTER TABLE witnesses ALTER COLUMN witness_code SET NOT NULL;

-- Unique witness code per case
ALTER TABLE witnesses
    ADD CONSTRAINT witnesses_case_witness_code_unique UNIQUE (case_id, witness_code);

-- Add judge_identity_visible flag for case-by-case judge access
ALTER TABLE witnesses
    ADD COLUMN IF NOT EXISTS judge_identity_visible BOOLEAN NOT NULL DEFAULT false;

-- Index for case lookups
CREATE INDEX IF NOT EXISTS idx_witnesses_case_id ON witnesses (case_id);

-- Index for protection status filtering
CREATE INDEX IF NOT EXISTS idx_witnesses_protection_status ON witnesses (protection_status);

COMMIT;
