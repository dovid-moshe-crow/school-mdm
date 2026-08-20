-- Scheduled add/remove of whitelist packs on devices and groups.

CREATE TABLE IF NOT EXISTS policy_timers (
    id            UUID PRIMARY KEY,
    name          TEXT NOT NULL,
    action        TEXT NOT NULL,
    pack_ids      TEXT[] NOT NULL DEFAULT '{}',
    device_ids    TEXT[] NOT NULL DEFAULT '{}',
    group_ids     TEXT[] NOT NULL DEFAULT '{}',
    schedule      TEXT NOT NULL,
    run_at        TIMESTAMPTZ,
    weekdays      INT[] NOT NULL DEFAULT '{}',
    time_of_day   TEXT NOT NULL DEFAULT '',
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at   TIMESTAMPTZ,
    last_run_key  TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS policy_timers_enabled_idx ON policy_timers (enabled) WHERE enabled;
