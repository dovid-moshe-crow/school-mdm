-- Device display names (nicknames for enrollment IDs)

CREATE TABLE IF NOT EXISTS devices (
    enrollment_id TEXT PRIMARY KEY,
    name          TEXT NOT NULL DEFAULT '',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
