-- KFilter companion app: push tokens + MDM/VPP settings extensions.

CREATE TABLE IF NOT EXISTS device_push_tokens (
    enrollment_id TEXT NOT NULL,
    token         TEXT NOT NULL,
    platform      TEXT NOT NULL DEFAULT 'ios',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (token)
);

CREATE INDEX IF NOT EXISTS device_push_tokens_enrollment_idx
    ON device_push_tokens (enrollment_id);

ALTER TABLE mdm_settings
    ADD COLUMN IF NOT EXISTS companion_bundle_id TEXT NOT NULL DEFAULT 'com.kfilter.portal',
    ADD COLUMN IF NOT EXISTS companion_itunes_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS companion_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS vpp_token BYTEA NULL,
    ADD COLUMN IF NOT EXISTS vpp_token_filename TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS vpp_token_updated_at TIMESTAMPTZ NULL;
