-- Recurring credit allotments: rules, per-period grants, and allotment bucket.
--
-- Two-bucket model:
--   device_credits.balance            = permanent (purchases + manual gifts)
--   device_credits.allotment_balance  = current period allotment (resets, never stacks)
-- Spend deducts allotment_balance first, then permanent balance.

ALTER TABLE device_credits
    ADD COLUMN IF NOT EXISTS allotment_balance INT NOT NULL DEFAULT 0
        CHECK (allotment_balance >= 0);

-- Widen ledger reasons for allotment grant / clawback.
ALTER TABLE credit_ledger DROP CONSTRAINT IF EXISTS credit_ledger_reason_check;
ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_reason_check
    CHECK (reason IN (
        'purchase', 'spend', 'refund', 'gift', 'adjust',
        'allotment', 'allotment_expire'
    ));

CREATE TABLE IF NOT EXISTS credit_allotment_rules (
    id           UUID PRIMARY KEY,
    name         TEXT NOT NULL DEFAULT '',
    note         TEXT NOT NULL DEFAULT '',
    amount       INT NOT NULL CHECK (amount > 0),
    interval     TEXT NOT NULL CHECK (interval IN ('daily', 'weekly', 'monthly')),
    target_type  TEXT NOT NULL CHECK (target_type IN ('everyone', 'group', 'individual')),
    target_id    TEXT NOT NULL DEFAULT '',
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS credit_allotment_rules_enabled_idx
    ON credit_allotment_rules (enabled);

-- One grant per rule + enrollment + period (idempotency). remaining tracks unused allotment for clawback.
CREATE TABLE IF NOT EXISTS credit_allotment_grants (
    id            UUID PRIMARY KEY,
    rule_id       UUID NOT NULL REFERENCES credit_allotment_rules (id) ON DELETE CASCADE,
    enrollment_id TEXT NOT NULL,
    period_key    TEXT NOT NULL,
    amount        INT NOT NULL CHECK (amount > 0),
    remaining     INT NOT NULL CHECK (remaining >= 0),
    granted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (rule_id, enrollment_id, period_key)
);

CREATE INDEX IF NOT EXISTS credit_allotment_grants_enrollment_idx
    ON credit_allotment_grants (enrollment_id, granted_at);
