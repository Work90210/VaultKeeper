BEGIN;

DROP INDEX IF EXISTS idx_custody_timestamp;
DROP INDEX IF EXISTS idx_witnesses_case;
DROP INDEX IF EXISTS idx_disclosures_case;
DROP INDEX IF EXISTS idx_api_keys_hash;
DROP INDEX IF EXISTS idx_notifications_user_unread;
DROP INDEX IF EXISTS idx_case_roles_case;
DROP INDEX IF EXISTS idx_case_roles_user;
DROP INDEX IF EXISTS idx_custody_case_id;
DROP INDEX IF EXISTS idx_custody_evidence_id;
DROP INDEX IF EXISTS idx_evidence_case_current;
DROP INDEX IF EXISTS idx_evidence_case_id;

COMMIT;
