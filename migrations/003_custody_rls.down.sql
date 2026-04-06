BEGIN;

-- Drop RLS policies
DROP POLICY IF EXISTS custody_log_select_readonly ON custody_log;
DROP POLICY IF EXISTS custody_log_select ON custody_log;
DROP POLICY IF EXISTS custody_log_insert ON custody_log;

-- Disable RLS
ALTER TABLE custody_log DISABLE ROW LEVEL SECURITY;

-- Revoke grants
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM vaultkeeper_readonly;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM vaultkeeper_app;
REVOKE USAGE ON SCHEMA public FROM vaultkeeper_readonly;
REVOKE USAGE ON SCHEMA public FROM vaultkeeper_app;

-- Drop roles
DROP ROLE IF EXISTS vaultkeeper_readonly;
DROP ROLE IF EXISTS vaultkeeper_app;

COMMIT;
