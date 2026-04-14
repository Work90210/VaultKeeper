BEGIN;

DROP INDEX IF EXISTS idx_case_roles_user;
DROP INDEX IF EXISTS idx_cases_org;

-- Restore global uniqueness (only safe if no duplicate reference_codes exist)
ALTER TABLE cases ADD CONSTRAINT cases_reference_code_key UNIQUE (reference_code);

ALTER TABLE cases DROP COLUMN IF EXISTS organization_id;

COMMIT;
