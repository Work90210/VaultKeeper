BEGIN;

CREATE TABLE user_profiles (
    user_id      UUID PRIMARY KEY,
    display_name TEXT,
    avatar_url   TEXT,
    bio          TEXT NOT NULL DEFAULT '',
    timezone     TEXT NOT NULL DEFAULT 'UTC',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMIT;
