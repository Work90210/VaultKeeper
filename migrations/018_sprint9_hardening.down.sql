-- Migration 018 (down): revert schema hardening.

BEGIN;

ALTER TABLE evidence_items
    DROP CONSTRAINT IF EXISTS chk_destruction_authority_length;

ALTER TABLE erasure_requests
    DROP CONSTRAINT IF EXISTS erasure_requests_evidence_id_fkey;

ALTER TABLE erasure_requests
    ADD CONSTRAINT erasure_requests_evidence_id_fkey
        FOREIGN KEY (evidence_id)
        REFERENCES evidence_items(id)
        ON DELETE CASCADE;

COMMIT;
