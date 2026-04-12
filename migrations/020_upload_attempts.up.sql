BEGIN;

-- Every attempted upload, including rejections. The row is inserted
-- before the file is processed and never updated. State changes are
-- event-sourced via upload_attempt_events so the header stays immutable.
CREATE TABLE upload_attempts_v1 (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id      UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    user_id      UUID NOT NULL
        CHECK (user_id <> '00000000-0000-0000-0000-000000000000'::uuid),
    client_hash  TEXT NOT NULL CHECK (client_hash ~ '^[0-9a-f]{64}$'),
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_upload_attempts_case_started
    ON upload_attempts_v1(case_id, started_at DESC);

CREATE TABLE upload_attempt_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id  UUID NOT NULL REFERENCES upload_attempts_v1(id) ON DELETE RESTRICT,
    event_type  TEXT NOT NULL
        CHECK (event_type IN (
            'bytes_received', 'hash_verified', 'hash_mismatch',
            'stored', 'rejected', 'aborted')),
    payload     JSONB NOT NULL DEFAULT '{}',
    at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_upload_attempt_events_attempt ON upload_attempt_events(attempt_id);

-- notification_outbox: durable compensation actions for MinIO/notification
-- side effects that cannot be rolled back with PG. Consumed by the cleanup
-- worker in internal/evidence/cleanup/.
CREATE TABLE notification_outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    action          TEXT NOT NULL
        CHECK (action IN ('minio_delete_object', 'notification_send')),
    payload         JSONB NOT NULL,
    attempt_count   INT NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts    INT NOT NULL DEFAULT 10 CHECK (max_attempts > 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    dead_letter_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notification_outbox_pending
    ON notification_outbox(next_attempt_at ASC, created_at ASC)
    WHERE completed_at IS NULL AND dead_letter_at IS NULL;

COMMIT;
