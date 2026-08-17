-- Cached Apple DEP device list so Admin can show devices without syncing on every open.

CREATE TABLE IF NOT EXISTS abm_device_cache (
    id         SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    devices    JSONB NOT NULL DEFAULT '[]'::jsonb,
    synced_at  TIMESTAMPTZ NULL
);

INSERT INTO abm_device_cache (id, devices)
VALUES (1, '[]'::jsonb)
ON CONFLICT (id) DO NOTHING;
