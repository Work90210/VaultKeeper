CREATE TABLE notification_preferences (
    user_id    UUID PRIMARY KEY,
    preferences JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
