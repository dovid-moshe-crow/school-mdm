-- Product schema for school-mdm (Neon / Postgres)

CREATE TABLE IF NOT EXISTS schema_migrations (
    filename TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS allowlist_entries (
    id            UUID PRIMARY KEY,
    kind          TEXT NOT NULL CHECK (kind IN ('app', 'url')),
    value         TEXT NOT NULL,
    scope         TEXT NOT NULL DEFAULT 'global',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (kind, value, scope)
);

CREATE TABLE IF NOT EXISTS grants (
    id            UUID PRIMARY KEY,
    kind          TEXT NOT NULL CHECK (kind IN ('app', 'url')),
    value         TEXT NOT NULL,
    enrollment_id TEXT NOT NULL DEFAULT '',
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS grants_enrollment_idx ON grants (enrollment_id);
CREATE INDEX IF NOT EXISTS grants_expires_idx ON grants (expires_at);

CREATE TABLE IF NOT EXISTS requests (
    id            UUID PRIMARY KEY,
    type          TEXT NOT NULL CHECK (type IN ('access', 'general', 'bug')),
    target_kind   TEXT NOT NULL DEFAULT '' CHECK (target_kind IN ('', 'app', 'url')),
    value         TEXT NOT NULL,
    enrollment_id TEXT NOT NULL DEFAULT '',
    reason        TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'denied', 'resolved')),
    duration      TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS requests_status_idx ON requests (status);
CREATE INDEX IF NOT EXISTS requests_type_idx ON requests (type);
