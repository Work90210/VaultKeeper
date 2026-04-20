BEGIN;

DROP INDEX IF EXISTS idx_disclosures_batch_id;
ALTER TABLE disclosures DROP COLUMN IF EXISTS batch_id;

COMMIT;
