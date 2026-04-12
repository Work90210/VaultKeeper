BEGIN;

-- Drop child tables before parent to respect FK constraints.
DROP TABLE IF EXISTS upload_attempt_events;
DROP TABLE IF EXISTS notification_outbox;
DROP TABLE IF EXISTS upload_attempts_v1;

COMMIT;
