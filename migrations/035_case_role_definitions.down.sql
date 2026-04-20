ALTER TABLE case_roles DROP COLUMN IF EXISTS role_definition_id;

-- Restore CHECK constraint on role column.
ALTER TABLE case_roles ADD CONSTRAINT case_roles_role_check
    CHECK (role IN ('investigator', 'prosecutor', 'defence', 'judge', 'observer', 'victim_representative'));

DROP TABLE IF EXISTS case_role_definitions;
