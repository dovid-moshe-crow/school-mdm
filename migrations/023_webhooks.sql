-- Outbound webhooks for activity events (admin API integrations).

CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id          UUID PRIMARY KEY,
    url         TEXT NOT NULL,
    secret      TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    events      TEXT[] NOT NULL DEFAULT '{}',
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id          UUID PRIMARY KEY,
    endpoint_id UUID NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_id    TEXT NOT NULL DEFAULT '',
    event_name  TEXT NOT NULL,
    status      TEXT NOT NULL,
    attempt     INT NOT NULL DEFAULT 1,
    http_status INT NOT NULL DEFAULT 0,
    error       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS webhook_deliveries_endpoint_at_idx
    ON webhook_deliveries (endpoint_id, created_at DESC);
