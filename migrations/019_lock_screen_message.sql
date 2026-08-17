-- Lock Screen Message (com.apple.shareddeviceconfiguration) school defaults.

ALTER TABLE mdm_settings
    ADD COLUMN IF NOT EXISTS lock_screen_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS lock_screen_footnote TEXT NOT NULL DEFAULT 'מכשיר בית ספר · KFilter';
