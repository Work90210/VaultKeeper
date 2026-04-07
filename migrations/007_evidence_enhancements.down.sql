BEGIN;

DROP INDEX IF EXISTS idx_evidence_tags;
DROP INDEX IF EXISTS idx_evidence_parent;
DROP INDEX IF EXISTS idx_evidence_pending_tsa;
DROP INDEX IF EXISTS idx_evidence_sha256;

ALTER TABLE evidence_items DROP COLUMN IF EXISTS exif_data;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS destroy_reason;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS destroyed_by;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS destroyed_at;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS tsa_last_retry;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS tsa_retry_count;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS tsa_status;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS tsa_timestamp;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS tsa_name;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS parent_id;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS tags;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS description;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS thumbnail_key;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS storage_key;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS evidence_number;

ALTER TABLE cases DROP COLUMN IF EXISTS evidence_counter;

COMMIT;
