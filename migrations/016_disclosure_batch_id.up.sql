-- Migration 016: Add batch_id to disclosures
-- Replaces the fragile (case_id, disclosed_by, disclosed_at) batch identity
-- with a stable UUID that is immune to same-microsecond collisions.
BEGIN;

ALTER TABLE disclosures ADD COLUMN IF NOT EXISTS batch_id UUID DEFAULT gen_random_uuid();

-- Backfill existing rows: each row becomes its own batch.
-- Rows created before this migration have no shared batch context, so
-- assigning each the row's own id (which is already a UUID) preserves
-- the existing semantics without merging unrelated historical records.
UPDATE disclosures SET batch_id = id WHERE batch_id IS NULL;

ALTER TABLE disclosures ALTER COLUMN batch_id SET NOT NULL;

CREATE INDEX idx_disclosures_batch_id ON disclosures (batch_id);

COMMIT;
