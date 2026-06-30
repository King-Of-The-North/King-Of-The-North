-- Wallet ledger schema. All money columns are BIGINT minor units (kuruş, ADR-0003).
-- See docs/plans/day-zero-wallet.md §2.

CREATE TABLE IF NOT EXISTS accounts (
    user_id           UUID PRIMARY KEY,
    principal_balance BIGINT      NOT NULL DEFAULT 0, -- locked deposit, 1:1 tokenized fiat
    projected_yield   BIGINT      NOT NULL DEFAULT 0, -- Yp at deposit time
    credit_limit      BIGINT      NOT NULL DEFAULT 0, -- L0
    available_credit  BIGINT      NOT NULL DEFAULT 0, -- L0 − sum(spent)
    ltv_ratio         NUMERIC(6,4) NOT NULL DEFAULT 0,
    lockup_end_date   TIMESTAMPTZ,
    pool_type         TEXT        NOT NULL DEFAULT 'fixed', -- only 'fixed' (ADR-0001)
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transactions (
    id              UUID PRIMARY KEY,
    user_id         UUID        NOT NULL REFERENCES accounts(user_id),
    other_trx_code  TEXT        NOT NULL UNIQUE, -- our internal id sent to Moka
    moka_payment_id TEXT,                         -- Moka's id, filled on settlement
    amount          BIGINT      NOT NULL,         -- kuruş
    payment_status  SMALLINT    NOT NULL DEFAULT 0, -- 0=Standby 1=Pre-Provision 2=Payment 3=Cancel 4=Full-Refund
    trx_status      SMALLINT    NOT NULL DEFAULT 0, -- 0=Standby 1=Successful 2=Failed
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions(user_id);
