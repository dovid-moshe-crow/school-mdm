-- Groups + scoped allowlists/grants (global | group | device)

CREATE TABLE IF NOT EXISTS groups (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS group_members (
    group_id      UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    enrollment_id TEXT NOT NULL,
    PRIMARY KEY (group_id, enrollment_id)
);

CREATE INDEX IF NOT EXISTS group_members_enrollment_idx ON group_members (enrollment_id);

-- Rebuild allowlist_entries with target_type / target_id
CREATE TABLE IF NOT EXISTS allowlist_entries_v2 (
    id          UUID PRIMARY KEY,
    kind        TEXT NOT NULL CHECK (kind IN ('app', 'url')),
    value       TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT 'global' CHECK (target_type IN ('global', 'group', 'device')),
    target_id   TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (kind, value, target_type, target_id)
);

INSERT INTO allowlist_entries_v2 (id, kind, value, target_type, target_id, created_at)
SELECT id, kind, value,
       CASE WHEN scope = 'global' OR scope = '' OR scope IS NULL THEN 'global' ELSE 'global' END,
       '',
       COALESCE(created_at, now())
FROM allowlist_entries
ON CONFLICT DO NOTHING;

DROP TABLE IF EXISTS allowlist_entries;
ALTER TABLE allowlist_entries_v2 RENAME TO allowlist_entries;

-- Rebuild grants with target_type / target_id
CREATE TABLE IF NOT EXISTS grants_v2 (
    id          UUID PRIMARY KEY,
    kind        TEXT NOT NULL CHECK (kind IN ('app', 'url')),
    value       TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT 'device' CHECK (target_type IN ('global', 'group', 'device')),
    target_id   TEXT NOT NULL DEFAULT '',
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO grants_v2 (id, kind, value, target_type, target_id, expires_at, created_at)
SELECT id, kind, value,
       CASE WHEN enrollment_id = '' OR enrollment_id IS NULL THEN 'global' ELSE 'device' END,
       COALESCE(enrollment_id, ''),
       expires_at,
       COALESCE(created_at, now())
FROM grants;

DROP TABLE IF EXISTS grants;
ALTER TABLE grants_v2 RENAME TO grants;

CREATE INDEX IF NOT EXISTS grants_target_idx ON grants (target_type, target_id);
CREATE INDEX IF NOT EXISTS grants_expires_idx ON grants (expires_at);
CREATE INDEX IF NOT EXISTS allowlist_target_idx ON allowlist_entries (target_type, target_id);
