CREATE TABLE IF NOT EXISTS agent_tasks (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    wallet_address       VARCHAR(42) NOT NULL,
    role                 VARCHAR(20) NOT NULL CHECK (role IN ('trader', 'fund_manager')),
    ticker               VARCHAR(10) NOT NULL,
    amount_bps_cap       SMALLINT NOT NULL CHECK (amount_bps_cap > 0 AND amount_bps_cap <= 10000),
    trigger_description  TEXT NOT NULL,
    status               VARCHAR(15) NOT NULL DEFAULT 'pending'
                             CHECK (status IN ('pending', 'executed', 'skipped', 'cancelled', 'failed')),
    reasoning            TEXT,
    tx_hash              VARCHAR(66),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    executed_at          TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_agent_tasks_wallet_address ON agent_tasks (wallet_address);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_status ON agent_tasks (status);
