-- App Store metadata cache (iTunes Search / Lookup)

CREATE TABLE IF NOT EXISTS app_metadata (
    bundle_id    TEXT PRIMARY KEY,
    track_id     BIGINT NOT NULL DEFAULT 0,
    name         TEXT NOT NULL DEFAULT '',
    artist       TEXT NOT NULL DEFAULT '',
    artwork_url  TEXT NOT NULL DEFAULT '',
    store_url    TEXT NOT NULL DEFAULT '',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS app_metadata_name_idx ON app_metadata (lower(name));
