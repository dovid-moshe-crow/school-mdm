-- Global credit settings (singleton row) and optional ledger notes.

CREATE TABLE IF NOT EXISTS credit_settings (
    id                   SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    access_request_cost  INT NOT NULL DEFAULT 1 CHECK (access_request_cost >= 1),
    enabled              BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO credit_settings (id, access_request_cost, enabled)
VALUES (1, 1, TRUE)
ON CONFLICT (id) DO NOTHING;

ALTER TABLE credit_ledger
    ADD COLUMN IF NOT EXISTS note TEXT NOT NULL DEFAULT '';
