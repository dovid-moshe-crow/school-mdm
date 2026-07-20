-- Device credits, ledger, packages, and Nedarim purchases.

CREATE TABLE IF NOT EXISTS device_credits (
    enrollment_id TEXT PRIMARY KEY,
    balance       INT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS credit_ledger (
    id            UUID PRIMARY KEY,
    enrollment_id TEXT NOT NULL,
    delta         INT NOT NULL,
    balance_after INT NOT NULL,
    reason        TEXT NOT NULL CHECK (reason IN ('purchase', 'spend', 'refund', 'gift', 'adjust')),
    ref_type      TEXT NOT NULL DEFAULT '',
    ref_id        TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS credit_ledger_idempotency_idx
    ON credit_ledger (reason, ref_type, ref_id)
    WHERE ref_type <> '' AND ref_id <> '';

CREATE INDEX IF NOT EXISTS credit_ledger_enrollment_idx
    ON credit_ledger (enrollment_id, created_at DESC);

CREATE TABLE IF NOT EXISTS credit_packages (
    id           UUID PRIMARY KEY,
    name_he      TEXT NOT NULL,
    credits      INT NOT NULL CHECK (credits > 0),
    price_agorot INT NOT NULL CHECK (price_agorot > 0),
    active       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order   INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS credit_purchases (
    id                UUID PRIMARY KEY,
    enrollment_id     TEXT NOT NULL,
    package_id        UUID NOT NULL REFERENCES credit_packages (id),
    credits           INT NOT NULL CHECK (credits > 0),
    amount_agorot     INT NOT NULL CHECK (amount_agorot > 0),
    status            TEXT NOT NULL CHECK (status IN ('pending', 'paid', 'failed', 'expired')),
    provider          TEXT NOT NULL CHECK (provider IN ('nedarim', 'fake')),
    provider_tx_id    TEXT NOT NULL DEFAULT '',
    client_unique_id  TEXT NOT NULL UNIQUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at           TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS credit_purchases_enrollment_idx
    ON credit_purchases (enrollment_id, created_at DESC);

-- Seed default packages (₪1 / credit with volume discounts).
INSERT INTO credit_packages (id, name_he, credits, price_agorot, active, sort_order)
VALUES
    ('a0000000-0000-4000-8000-000000000010', '10 קרדיטים', 10, 1000, TRUE, 10),
    ('a0000000-0000-4000-8000-000000000050', '50 קרדיטים', 50, 4500, TRUE, 20),
    ('a0000000-0000-4000-8000-000000000100', '100 קרדיטים', 100, 8000, TRUE, 30)
ON CONFLICT (id) DO NOTHING;
