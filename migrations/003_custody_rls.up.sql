-- Row-Level Security on custody_log: append-only enforcement
-- No UPDATE or DELETE policies = immutable audit trail

BEGIN;

-- Create application role (used by Go server)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'vaultkeeper_app') THEN
        CREATE ROLE vaultkeeper_app LOGIN;
    END IF;
END
$$;

-- Create read-only role (used for reporting)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'vaultkeeper_readonly') THEN
        CREATE ROLE vaultkeeper_readonly LOGIN;
    END IF;
END
$$;

-- Grant table access to application role
GRANT SELECT, INSERT ON custody_log TO vaultkeeper_app;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO vaultkeeper_app;
GRANT INSERT, UPDATE, DELETE ON cases, case_roles, evidence_items, witnesses, disclosures, notifications, api_keys, backup_log TO vaultkeeper_app;
GRANT USAGE ON SCHEMA public TO vaultkeeper_app;

-- Grant read-only access
GRANT SELECT ON ALL TABLES IN SCHEMA public TO vaultkeeper_readonly;
GRANT USAGE ON SCHEMA public TO vaultkeeper_readonly;

-- Enable RLS on custody_log
ALTER TABLE custody_log ENABLE ROW LEVEL SECURITY;

-- Force RLS for table owner too (defense in depth)
ALTER TABLE custody_log FORCE ROW LEVEL SECURITY;

-- Allow INSERT for application role
CREATE POLICY custody_log_insert ON custody_log
    FOR INSERT
    TO vaultkeeper_app
    WITH CHECK (true);

-- Allow SELECT for application role
CREATE POLICY custody_log_select ON custody_log
    FOR SELECT
    TO vaultkeeper_app
    USING (true);

-- Allow SELECT for read-only role
CREATE POLICY custody_log_select_readonly ON custody_log
    FOR SELECT
    TO vaultkeeper_readonly
    USING (true);

-- NO UPDATE or DELETE policies: any attempt will be denied by RLS

COMMIT;
