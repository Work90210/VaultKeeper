BEGIN;

DROP INDEX IF EXISTS idx_disclosures_case_evidence;
DROP INDEX IF EXISTS idx_disclosures_evidence_id;
DROP INDEX IF EXISTS idx_disclosures_case_id;
ALTER TABLE disclosures DROP COLUMN IF EXISTS redacted;

COMMIT;
