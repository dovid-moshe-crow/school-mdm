-- Per-device Allow-all (unrestricted) flag + admin undo log.

ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS unrestricted BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS admin_actions (
    id         UUID PRIMARY KEY,
    at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    kind       TEXT NOT NULL,
    payload    JSONB NOT NULL DEFAULT '{}'::jsonb,
    undone_at  TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS admin_actions_at_idx ON admin_actions (at DESC);
CREATE INDEX IF NOT EXISTS admin_actions_undone_idx ON admin_actions (undone_at);
