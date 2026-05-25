-- Drop all existing tables in dependency order
DROP TABLE IF EXISTS stock_attestations    CASCADE;
DROP TABLE IF EXISTS stock_transactions    CASCADE;
DROP TABLE IF EXISTS redeem_attestations   CASCADE;
DROP TABLE IF EXISTS redeem_proposals      CASCADE;
DROP TABLE IF EXISTS mint_attestations     CASCADE;
DROP TABLE IF EXISTS mint_proposals        CASCADE;
DROP TABLE IF EXISTS wallet_verifications  CASCADE;
DROP TABLE IF EXISTS stocks                CASCADE;
DROP TABLE IF EXISTS custodians            CASCADE;

-- ─── custodians ───────────────────────────────────────────────────────────────
CREATE TABLE custodians (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    wallet_address VARCHAR(42)  NOT NULL UNIQUE,
    name           VARCHAR(100) NOT NULL,
    email          VARCHAR(255),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ─── stocks ───────────────────────────────────────────────────────────────────
CREATE TABLE stocks (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticker           VARCHAR(10)  NOT NULL UNIQUE,
    stock_name       VARCHAR(100) NOT NULL,
    idx_ticker       VARCHAR(10)  NOT NULL,
    sector           VARCHAR(50),
    contract_address VARCHAR(42)  UNIQUE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ─── wallet_verifications (KYC) ───────────────────────────────────────────────
CREATE TABLE wallet_verifications (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    wallet_address VARCHAR(42) NOT NULL UNIQUE,
    type           VARCHAR(15) NOT NULL CHECK (type IN ('retail', 'institution')),
    status         VARCHAR(10) NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'approved', 'rejected')),
    document_url   TEXT,
    submitted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    verified_at    TIMESTAMPTZ,
    verified_by    BIGINT      REFERENCES custodians(id)
);

-- ─── mint_proposals ───────────────────────────────────────────────────────────
-- destination: 'operator_wallet' (institutional) | 'liquidity_pool'
-- approve/reject counts are derived via COUNT on mint_attestations — not stored.
CREATE TABLE mint_proposals (
    id                        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    on_chain_id               BIGINT         NOT NULL UNIQUE,
    stock_id                  BIGINT         NOT NULL REFERENCES stocks(id),
    token_amount              NUMERIC(78, 0) NOT NULL,
    idrx_amount               NUMERIC(78, 0),
    attestation_hash          CHAR(66)       NOT NULL,
    destination               VARCHAR(20)    NOT NULL
                                  CHECK (destination IN ('operator_wallet', 'liquidity_pool')),
    requester_id              BIGINT         REFERENCES custodians(id),
    status                    VARCHAR(10)    NOT NULL DEFAULT 'pending'
                                  CHECK (status IN ('pending', 'executed', 'rejected')),
    request_tx_hash           CHAR(66),
    execute_tx_hash           CHAR(66),
    created_at                TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    executed_at               TIMESTAMPTZ
);

-- ─── mint_attestations ────────────────────────────────────────────────────────
-- Unified approve + reject votes. One row per custodian per proposal.
CREATE TABLE mint_attestations (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    proposal_id  BIGINT      NOT NULL REFERENCES mint_proposals(id),
    custodian_id BIGINT      NOT NULL REFERENCES custodians(id),
    type         VARCHAR(7)  NOT NULL CHECK (type IN ('approve', 'reject')),
    tx_hash      CHAR(66),
    attested_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (proposal_id, custodian_id)
);

-- ─── redeem_proposals ─────────────────────────────────────────────────────────
-- source derived from wallet_verifications.type of the requesting user.
-- approve/reject counts derived via COUNT on redeem_attestations.
CREATE TABLE redeem_proposals (
    id                        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    on_chain_id               BIGINT         NOT NULL UNIQUE,
    stock_id                  BIGINT         NOT NULL REFERENCES stocks(id),
    token_amount              NUMERIC(78, 0) NOT NULL,
    fee_idrx                  NUMERIC(78, 0) NOT NULL DEFAULT 0,
    user_address              VARCHAR(42)    NOT NULL,
    status                    VARCHAR(10)    NOT NULL DEFAULT 'pending'
                                  CHECK (status IN ('pending', 'executed', 'rejected')),
    request_tx_hash           CHAR(66),
    execute_tx_hash           CHAR(66),
    created_at                TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    executed_at               TIMESTAMPTZ
);

-- ─── redeem_attestations ──────────────────────────────────────────────────────
-- Unified approve + reject votes. One row per custodian per proposal.
CREATE TABLE redeem_attestations (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    proposal_id  BIGINT      NOT NULL REFERENCES redeem_proposals(id),
    custodian_id BIGINT      NOT NULL REFERENCES custodians(id),
    type         VARCHAR(7)  NOT NULL CHECK (type IN ('approve', 'reject')),
    tx_hash      CHAR(66),
    attested_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (proposal_id, custodian_id)
);

-- ─── stock_transactions ───────────────────────────────────────────────────────
CREATE TABLE stock_transactions (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    stock_id       BIGINT         NOT NULL REFERENCES stocks(id),
    wallet_address VARCHAR(42)    NOT NULL,
    side           VARCHAR(4)     NOT NULL CHECK (side IN ('buy', 'sell')),
    idrx_amount    NUMERIC(78, 0) NOT NULL,
    stock_amount   NUMERIC(78, 0) NOT NULL,
    tx_hash        CHAR(66)       NOT NULL UNIQUE,
    block_number   BIGINT         NOT NULL,
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- ─── stock_attestations (proof of reserves) ───────────────────────────────────
CREATE TABLE stock_attestations (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    stock_id           BIGINT         NOT NULL REFERENCES stocks(id),
    custodian_holdings NUMERIC(78, 0) NOT NULL,
    on_chain_supply    NUMERIC(78, 0) NOT NULL,
    attestation_hash   CHAR(66),
    attested_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- ─── indexes ──────────────────────────────────────────────────────────────────
CREATE INDEX ON mint_proposals     (stock_id);
CREATE INDEX ON mint_proposals     (requester_id);
CREATE INDEX ON mint_proposals     (status);
CREATE INDEX ON mint_attestations  (proposal_id);
CREATE INDEX ON redeem_proposals   (stock_id);
CREATE INDEX ON redeem_proposals   (user_address);
CREATE INDEX ON redeem_proposals   (status);
CREATE INDEX ON redeem_attestations(proposal_id);
CREATE INDEX ON stock_transactions (stock_id);
CREATE INDEX ON stock_transactions (wallet_address);
CREATE INDEX ON stock_attestations (stock_id);
CREATE INDEX ON stock_attestations (attested_at DESC);
CREATE INDEX ON wallet_verifications(status);
