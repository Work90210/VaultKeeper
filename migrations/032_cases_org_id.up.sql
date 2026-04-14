BEGIN;

-- Add organization_id to cases (nullable initially for migration compatibility)
ALTER TABLE cases ADD COLUMN organization_id UUID REFERENCES organizations(id);

CREATE INDEX idx_cases_org ON cases(organization_id, status, created_at);

-- Drop global uniqueness on reference_code to allow per-org uniqueness
ALTER TABLE cases DROP CONSTRAINT cases_reference_code_key;

-- Index for case_roles lookups by user
CREATE INDEX IF NOT EXISTS idx_case_roles_user ON case_roles(user_id, case_id);

COMMIT;
