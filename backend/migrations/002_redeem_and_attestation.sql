-- ─── 002_redeem_and_attestation.sql ───────────────────────────────────────────

-- Add sector to stocks (nullable, retroactive)
ALTER TABLE stocks ADD COLUMN sector VARCHAR(50);

-- Add source to mint_proposals (retail|institutional, nullable for old rows)
ALTER TABLE mint_proposals
    ADD COLUMN source VARCHAR(15) CHECK (source IN ('retail', 'institutional'));

-- Redeem proposals: mirrors mint_proposals for the burn/redemption flow
CREATE TABLE redeem_proposals (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    on_chain_id      BIGINT         NOT NULL UNIQUE,
    stock_id         BIGINT         NOT NULL REFERENCES stocks(id),
    token_amount     NUMERIC(78, 0) NOT NULL,
    idrx_amount      NUMERIC(78, 0),
    attestation_hash CHAR(66)       NOT NULL,
    source           VARCHAR(15)    NOT NULL
                         CHECK (source IN ('retail', 'institutional')),
    requester_id     BIGINT         REFERENCES custodians(id),
    approval_count   SMALLINT       NOT NULL DEFAULT 1,
    executed         BOOLEAN        NOT NULL DEFAULT FALSE,
    request_tx_hash  CHAR(66),
    execute_tx_hash  CHAR(66),
    created_at       TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    executed_at      TIMESTAMPTZ
);

CREATE INDEX ON redeem_proposals (stock_id);
CREATE INDEX ON redeem_proposals (requester_id);
CREATE INDEX ON redeem_proposals (executed);

-- Redeem approvals: one row per custodian signature (mirrors mint_approvals)
CREATE TABLE redeem_approvals (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    proposal_id  BIGINT      NOT NULL REFERENCES redeem_proposals(id),
    custodian_id BIGINT      NOT NULL REFERENCES custodians(id),
    tx_hash      CHAR(66),
    approved_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (proposal_id, custodian_id)
);

CREATE INDEX ON redeem_approvals (proposal_id);

-- Stock attestations: proof of reserves submitted by custodians
CREATE TABLE stock_attestations (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    stock_id           BIGINT         NOT NULL REFERENCES stocks(id),
    custodian_id       BIGINT         NOT NULL REFERENCES custodians(id),
    custodian_holdings NUMERIC(78, 0) NOT NULL,  -- physical shares held
    on_chain_supply    NUMERIC(78, 0) NOT NULL,  -- tokens minted on-chain
    attestation_hash   CHAR(66),                 -- optional on-chain proof
    attested_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX ON stock_attestations (stock_id);
CREATE INDEX ON stock_attestations (attested_at DESC);
