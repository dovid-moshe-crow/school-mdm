-- Named allowlist packs (e.g. Games) assignable to global / device-groups / devices.

CREATE TABLE IF NOT EXISTS whitelist_packs (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS whitelist_pack_items (
    pack_id    UUID NOT NULL REFERENCES whitelist_packs (id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('app', 'url')),
    value      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (pack_id, kind, value)
);

CREATE INDEX IF NOT EXISTS whitelist_pack_items_kind_value_idx
    ON whitelist_pack_items (kind, value);

CREATE TABLE IF NOT EXISTS whitelist_pack_assignments (
    pack_id     UUID NOT NULL REFERENCES whitelist_packs (id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('global', 'group', 'device')),
    target_id   TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (pack_id, target_type, target_id)
);

CREATE INDEX IF NOT EXISTS whitelist_pack_assignments_target_idx
    ON whitelist_pack_assignments (target_type, target_id);
