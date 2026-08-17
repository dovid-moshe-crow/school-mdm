-- Persist the active Automated Device Enrollment profile UUID so assignment
-- remains possible after an Admin UI refresh or server restart.
ALTER TABLE mdm_settings
    ADD COLUMN IF NOT EXISTS dep_profile_uuid TEXT NOT NULL DEFAULT '';
