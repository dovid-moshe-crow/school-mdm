-- Structured audit / activity trail for Admin "לוגים".

CREATE TABLE IF NOT EXISTS activity_events (
    id             UUID PRIMARY KEY,
    at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    category       TEXT NOT NULL,
    action         TEXT NOT NULL,
    actor_type     TEXT NOT NULL DEFAULT 'system',
    actor          TEXT NOT NULL DEFAULT '',
    enrollment_id  TEXT NULL,
    group_id       TEXT NULL,
    request_id     TEXT NULL,
    command_uuid   TEXT NULL,
    result         TEXT NOT NULL DEFAULT 'ok',
    summary        TEXT NOT NULL DEFAULT '',
    detail         JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS activity_events_at_idx ON activity_events (at DESC);
CREATE INDEX IF NOT EXISTS activity_events_category_at_idx ON activity_events (category, at DESC);
CREATE INDEX IF NOT EXISTS activity_events_action_at_idx ON activity_events (action, at DESC);
CREATE INDEX IF NOT EXISTS activity_events_enrollment_at_idx ON activity_events (enrollment_id, at DESC)
    WHERE enrollment_id IS NOT NULL AND enrollment_id <> '';
CREATE INDEX IF NOT EXISTS activity_events_result_at_idx ON activity_events (result, at DESC);
