-- Case role definitions: per-org customizable role templates with granular permissions.
-- Default roles are seeded on org creation; admins can edit permissions or create custom roles.

CREATE TABLE case_role_definitions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    slug             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    permissions      JSONB NOT NULL DEFAULT '{}',
    is_default       BOOLEAN NOT NULL DEFAULT false,
    is_system        BOOLEAN NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, slug)
);

CREATE INDEX idx_case_role_defs_org ON case_role_definitions(organization_id);

-- Add role_definition_id FK to case_roles (nullable initially for backfill).
ALTER TABLE case_roles ADD COLUMN role_definition_id UUID REFERENCES case_role_definitions(id);

-- Drop the CHECK constraint on case_roles.role so we can support custom slugs.
ALTER TABLE case_roles DROP CONSTRAINT IF EXISTS case_roles_role_check;

-- Seed default role definitions for every existing organization.
INSERT INTO case_role_definitions (organization_id, name, slug, description, permissions, is_default, is_system)
SELECT o.id, def.name, def.slug, def.description, def.permissions, true, true
FROM organizations o
CROSS JOIN (VALUES
    ('Lead Investigator', 'lead_investigator', 'Full case access including member and case management',
     '{"view_evidence":true,"upload_evidence":true,"edit_evidence":true,"delete_evidence":true,"view_witnesses":true,"manage_witnesses":true,"view_disclosures":true,"manage_disclosures":true,"manage_case":true,"manage_members":true,"export":true,"manage_investigation":true}'::jsonb),
    ('Investigator', 'investigator', 'Evidence collection and witness management',
     '{"view_evidence":true,"upload_evidence":true,"edit_evidence":true,"delete_evidence":false,"view_witnesses":true,"manage_witnesses":true,"view_disclosures":true,"manage_disclosures":false,"manage_case":false,"manage_members":false,"export":true,"manage_investigation":true}'::jsonb),
    ('Prosecutor', 'prosecutor', 'Prosecution team with disclosure management',
     '{"view_evidence":true,"upload_evidence":false,"edit_evidence":false,"delete_evidence":false,"view_witnesses":true,"manage_witnesses":false,"view_disclosures":true,"manage_disclosures":true,"manage_case":false,"manage_members":false,"export":true,"manage_investigation":false}'::jsonb),
    ('Defence', 'defence', 'Defence counsel with read access to disclosed materials',
     '{"view_evidence":true,"upload_evidence":false,"edit_evidence":false,"delete_evidence":false,"view_witnesses":false,"manage_witnesses":false,"view_disclosures":true,"manage_disclosures":false,"manage_case":false,"manage_members":false,"export":true,"manage_investigation":false}'::jsonb),
    ('Judge', 'judge', 'Judicial oversight with full read access',
     '{"view_evidence":true,"upload_evidence":false,"edit_evidence":false,"delete_evidence":false,"view_witnesses":true,"manage_witnesses":false,"view_disclosures":true,"manage_disclosures":false,"manage_case":false,"manage_members":false,"export":true,"manage_investigation":false}'::jsonb),
    ('Observer', 'observer', 'Read-only access to evidence and disclosures',
     '{"view_evidence":true,"upload_evidence":false,"edit_evidence":false,"delete_evidence":false,"view_witnesses":false,"manage_witnesses":false,"view_disclosures":true,"manage_disclosures":false,"manage_case":false,"manage_members":false,"export":false,"manage_investigation":false}'::jsonb),
    ('Victim Representative', 'victim_representative', 'Victim participation with limited access',
     '{"view_evidence":true,"upload_evidence":false,"edit_evidence":false,"delete_evidence":false,"view_witnesses":false,"manage_witnesses":false,"view_disclosures":true,"manage_disclosures":false,"manage_case":false,"manage_members":false,"export":false,"manage_investigation":false}'::jsonb)
) AS def(name, slug, description, permissions)
ON CONFLICT (organization_id, slug) DO NOTHING;

-- Backfill existing case_roles rows with the matching role definition.
UPDATE case_roles
SET role_definition_id = crd.id
FROM cases c, case_role_definitions crd
WHERE case_roles.case_id = c.id
  AND crd.organization_id = c.organization_id
  AND crd.slug = case_roles.role
  AND case_roles.role_definition_id IS NULL;

-- Clean up orphaned rows (cases without org_id that couldn't be backfilled).
DELETE FROM case_roles WHERE role_definition_id IS NULL AND role NOT IN (
  SELECT DISTINCT slug FROM case_role_definitions
);
