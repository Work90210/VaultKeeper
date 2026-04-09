BEGIN;

DROP INDEX IF EXISTS idx_witnesses_protection_status;
DROP INDEX IF EXISTS idx_witnesses_case_id;
ALTER TABLE witnesses DROP CONSTRAINT IF EXISTS witnesses_case_witness_code_unique;
ALTER TABLE witnesses
    DROP COLUMN IF EXISTS judge_identity_visible,
    DROP COLUMN IF EXISTS related_evidence,
    DROP COLUMN IF EXISTS statement_summary,
    DROP COLUMN IF EXISTS witness_code;

COMMIT;
