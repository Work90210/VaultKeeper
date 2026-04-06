BEGIN;

ALTER TABLE custody_log ALTER COLUMN evidence_id DROP NOT NULL;
ALTER TABLE custody_log DROP CONSTRAINT IF EXISTS custody_log_evidence_id_fkey;
ALTER TABLE custody_log ADD CONSTRAINT custody_log_evidence_id_fkey
    FOREIGN KEY (evidence_id) REFERENCES evidence_items(id) ON DELETE RESTRICT;

COMMIT;
