-- Device registry (P2P WebSocket + signed-pay auth, ADR-0006/0010/0011) and offline
-- spending vouchers (ADR-0012). Only device PUBLIC keys are stored — the private key
-- never leaves the phone. All money columns are BIGINT minor units (kuruş, ADR-0003).

-- No FK to accounts: a phone can enroll its device key at signup, before any deposit
-- creates the account row. Devices are identity, not money.
CREATE TABLE IF NOT EXISTS devices (
    user_id       UUID        NOT NULL,
    device_pubkey BYTEA       NOT NULL,            -- Ed25519 public key (32 bytes)
    status        SMALLINT    NOT NULL DEFAULT 1,  -- 1=active 2=revoked
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at    TIMESTAMPTZ,
    PRIMARY KEY (user_id, device_pubkey)
);

CREATE INDEX IF NOT EXISTS idx_devices_user_active ON devices(user_id) WHERE status = 1;

CREATE TABLE IF NOT EXISTS vouchers (
    id              UUID        PRIMARY KEY,
    user_id         UUID        NOT NULL REFERENCES accounts(user_id),
    amount_minor    BIGINT      NOT NULL,          -- pre-reserved at issue (kuruş)
    status          SMALLINT    NOT NULL DEFAULT 1, -- 1=issued 2=redeemed 3=expired
    nonce           BYTEA       NOT NULL,          -- uniqueness in the signed payload
    sig             BYTEA       NOT NULL,          -- wallet's Ed25519 sig over the voucher
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,
    redeemed_at     TIMESTAMPTZ,
    redeem_trx_code TEXT
);

CREATE INDEX IF NOT EXISTS idx_vouchers_user_id ON vouchers(user_id);
