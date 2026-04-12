-- Migration 018: Sprint 9 schema hardening
--
-- Two follow-up fixes flagged during the Sprint 9 security review:
--
-- 1. destruction_authority length cap (M5)
--    The column was TEXT with no bound. Cap it at 2000 characters (more
--    than enough for a court order reference + brief legal basis) so a
--    client cannot push multi-megabyte strings into the audit row.
--
-- 2. erasure_requests.evidence_id FK → RESTRICT (M7)
--    The FK was ON DELETE CASCADE, which would silently wipe the GDPR
--    audit trail if any future migration or operator hard-deleted an
--    evidence row. Change to RESTRICT: any attempt to hard-delete an
--    evidence item that still has an associated erasure request will
--    fail loudly, forcing the operator to resolve the request first.

BEGIN;

ALTER TABLE evidence_items
    ADD CONSTRAINT chk_destruction_authority_length
        CHECK (destruction_authority IS NULL
               OR char_length(destruction_authority) <= 2000);

-- The FK added in migration 017 was auto-named by Postgres. For portability
-- across environments, drop by the conventional name pattern; then recreate
-- with an explicit name and RESTRICT semantics.
ALTER TABLE erasure_requests
    DROP CONSTRAINT IF EXISTS erasure_requests_evidence_id_fkey;

ALTER TABLE erasure_requests
    ADD CONSTRAINT erasure_requests_evidence_id_fkey
        FOREIGN KEY (evidence_id)
        REFERENCES evidence_items(id)
        ON DELETE RESTRICT;

COMMIT;
