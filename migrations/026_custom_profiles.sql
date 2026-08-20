-- Custom uploaded Apple configuration profiles (.mobileconfig), assignable like packs.

CREATE TABLE IF NOT EXISTS custom_profiles (
    id                   UUID PRIMARY KEY,
    name                 TEXT NOT NULL,
    description          TEXT NOT NULL DEFAULT '',
    filename             TEXT NOT NULL DEFAULT '',
    payload_identifier   TEXT NOT NULL UNIQUE,
    payload_uuid         TEXT NOT NULL DEFAULT '',
    payload_display_name TEXT NOT NULL DEFAULT '',
    payload_type         TEXT NOT NULL DEFAULT 'Configuration',
    payload              BYTEA NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS custom_profile_assignments (
    profile_id  UUID NOT NULL REFERENCES custom_profiles (id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('global', 'group', 'device')),
    target_id   TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (profile_id, target_type, target_id)
);

CREATE INDEX IF NOT EXISTS custom_profile_assignments_target_idx
    ON custom_profile_assignments (target_type, target_id);

ALTER TABLE policy_timers
    ADD COLUMN IF NOT EXISTS profile_ids TEXT[] NOT NULL DEFAULT '{}';
