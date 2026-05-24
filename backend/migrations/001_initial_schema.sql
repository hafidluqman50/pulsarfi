-- ─── 001_initial_schema.sql ──────────────────────────────────────────────────

-- Custodians: the 5 multisig participants (auth via SIWE, not password)
CREATE TABLE custodians (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    wallet_address VARCHAR(42)  NOT NULL UNIQUE,
    name           VARCHAR(100) NOT NULL,
    email          VARCHAR(255),                -- for Resend notifications only
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Stocks: one row per listed IDX equity token
CREATE TABLE stocks (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticker           VARCHAR(10)  NOT NULL UNIQUE,  -- e.g. BUMIP
    stock_name       VARCHAR(100) NOT NULL,          -- e.g. Bumi Resources
    idx_ticker       VARCHAR(10)  NOT NULL,          -- e.g. BUMI
    contract_address VARCHAR(42)  UNIQUE,            -- null until first executeMint
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Mint proposals: mirrors on-chain MintProposal struct
CREATE TABLE mint_proposals (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    on_chain_id      BIGINT      NOT NULL UNIQUE,   -- proposalId from contract
    stock_id         BIGINT      NOT NULL REFERENCES stocks(id),
    token_amount     NUMERIC(78, 0) NOT NULL,       -- raw 18-decimal amount
    idrx_amount      NUMERIC(78, 0),                -- null when destination = operator_wallet
    attestation_hash CHAR(66)    NOT NULL,          -- bytes32 as 0x-prefixed hex
    destination      VARCHAR(20) NOT NULL           -- 'operator_wallet' | 'liquidity_pool'
                         CHECK (destination IN ('operator_wallet', 'liquidity_pool')),
    requester_id     BIGINT      REFERENCES custodians(id),
    approval_count   SMALLINT    NOT NULL DEFAULT 1,
    executed         BOOLEAN     NOT NULL DEFAULT FALSE,
    request_tx_hash  CHAR(66),
    execute_tx_hash  CHAR(66),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    executed_at      TIMESTAMPTZ
);

-- Mint approvals: one row per custodian approval event
CREATE TABLE mint_approvals (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    proposal_id  BIGINT      NOT NULL REFERENCES mint_proposals(id),
    custodian_id BIGINT      NOT NULL REFERENCES custodians(id),
    tx_hash      CHAR(66),
    approved_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (proposal_id, custodian_id)
);

-- Wallet verifications: KYC/AML records (retail + institutional)
CREATE TABLE wallet_verifications (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    wallet_address VARCHAR(42)  NOT NULL UNIQUE,
    type           VARCHAR(15)  NOT NULL
                       CHECK (type IN ('retail', 'institution')),
    status         VARCHAR(10)  NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'approved', 'rejected')),
    document_url   TEXT,                             -- Supabase Storage path (PDF)
    submitted_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    verified_at    TIMESTAMPTZ,
    verified_by    BIGINT       REFERENCES custodians(id)
);

-- Stock transactions: mirrors on-chain TokensSwapped events
CREATE TABLE stock_transactions (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    stock_id       BIGINT         NOT NULL REFERENCES stocks(id),
    wallet_address VARCHAR(42)    NOT NULL,
    side           VARCHAR(4)     NOT NULL CHECK (side IN ('buy', 'sell')),
    idrx_amount    NUMERIC(78, 0) NOT NULL,  -- raw 2-decimal IDRX amount
    stock_amount   NUMERIC(78, 0) NOT NULL,  -- raw 18-decimal PulsarStock amount
    tx_hash        CHAR(66)       NOT NULL UNIQUE,
    block_number   BIGINT         NOT NULL,
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Indexes for common query patterns
CREATE INDEX ON mint_proposals (stock_id);
CREATE INDEX ON mint_proposals (requester_id);
CREATE INDEX ON mint_approvals (proposal_id);
CREATE INDEX ON stock_transactions (stock_id);
CREATE INDEX ON stock_transactions (wallet_address);
CREATE INDEX ON wallet_verifications (status);
