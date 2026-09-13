-- agent_task_trade_rebuild: ground-up rebuild of the on-chain-mirroring
-- columns on agent_tasks/agent_sub_tasks, per
-- docs/plans/agent-task-manager-rebuild.md §3a and
-- docs/plans/agent-task-manager-code-implementation.md §3.1. Task itself
-- never carries a money-related number again — ticker/side/budget/expiry
-- all move to on-chain TradePermission (granted separately, later) or the
-- new agent_trades table (one row per fill, never per Task).

ALTER TABLE agent_tasks
    DROP COLUMN IF EXISTS ticker,
    DROP COLUMN IF EXISTS is_buy,
    DROP COLUMN IF EXISTS amount_bps_cap,
    DROP COLUMN IF EXISTS fixed_amount,
    DROP COLUMN IF EXISTS expires_at,
    DROP COLUMN IF EXISTS on_chain_trade_id,
    DROP COLUMN IF EXISTS tx_hash,
    ADD COLUMN IF NOT EXISTS summary TEXT,
    ADD COLUMN IF NOT EXISTS on_chain_task_id BIGINT,
    ADD COLUMN IF NOT EXISTS paused BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS paused_at TIMESTAMPTZ;

ALTER TABLE agent_sub_tasks
    ADD COLUMN IF NOT EXISTS recorded_on_chain BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS on_chain_tx_hash TEXT;

CREATE TABLE IF NOT EXISTS agent_trades (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    task_id           BIGINT NOT NULL REFERENCES agent_tasks (id),
    sub_task_id       BIGINT NOT NULL REFERENCES agent_sub_tasks (id),
    on_chain_trade_id BIGINT,
    tx_hash           TEXT,
    ticker            TEXT NOT NULL,
    side              TEXT NOT NULL CHECK (side IN ('buy', 'sell')),
    amount            NUMERIC(38, 0) NOT NULL,
    summary           TEXT NOT NULL,
    executed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_trades_task_id ON agent_trades (task_id);
