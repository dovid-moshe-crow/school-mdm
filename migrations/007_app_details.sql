-- Rich App Store details from iTunes Lookup (JSON blob)

ALTER TABLE app_metadata
    ADD COLUMN IF NOT EXISTS details JSONB NOT NULL DEFAULT '{}'::jsonb;
