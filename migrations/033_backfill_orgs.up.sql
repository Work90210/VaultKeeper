BEGIN;

-- Step 1: Create a "Personal Workspace" org for each distinct case creator
INSERT INTO organizations (name, slug, created_by)
SELECT
    'Personal Workspace',
    'personal-' || creator_id::text,
    creator_id
FROM (SELECT DISTINCT created_by AS creator_id FROM cases) AS creators;

-- Step 2: Add each creator as owner of their org
INSERT INTO organization_memberships (organization_id, user_id, role, status, joined_at)
SELECT o.id, o.created_by, 'owner', 'active', now()
FROM organizations o
WHERE o.name = 'Personal Workspace';

-- Step 3: Assign each case to its creator's org
UPDATE cases c
SET organization_id = o.id
FROM organizations o
WHERE o.created_by = c.created_by
  AND o.name = 'Personal Workspace';

-- Step 4: Add case_roles users as org members where not already present
INSERT INTO organization_memberships (organization_id, user_id, role, status, joined_at)
SELECT DISTINCT c.organization_id, cr.user_id, 'member', 'active', now()
FROM case_roles cr
JOIN cases c ON c.id = cr.case_id
WHERE NOT EXISTS (
    SELECT 1 FROM organization_memberships om
    WHERE om.organization_id = c.organization_id
      AND om.user_id = cr.user_id
);

-- Step 5: Enforce NOT NULL now that all cases have an org
ALTER TABLE cases ALTER COLUMN organization_id SET NOT NULL;

-- Step 6: Add per-org reference code uniqueness
ALTER TABLE cases ADD CONSTRAINT cases_reference_code_org_unique
    UNIQUE (organization_id, reference_code);

COMMIT;
