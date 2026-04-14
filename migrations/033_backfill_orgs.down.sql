BEGIN;

-- WARNING: This rollback is destructive — org assignments are lost.
-- The up migration should be considered one-way in production.

ALTER TABLE cases DROP CONSTRAINT IF EXISTS cases_reference_code_org_unique;
ALTER TABLE cases ALTER COLUMN organization_id DROP NOT NULL;

-- Remove backfilled memberships and orgs
DELETE FROM organization_memberships WHERE organization_id IN (
    SELECT id FROM organizations WHERE name = 'Personal Workspace'
);
DELETE FROM organizations WHERE name = 'Personal Workspace';

UPDATE cases SET organization_id = NULL;

COMMIT;
