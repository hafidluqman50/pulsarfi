-- agent_chats: a conversation thread with Supervisor (docs/plans/ai-agent-role-dispatcher.md).
CREATE TABLE IF NOT EXISTS agent_chats (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_wallet VARCHAR(42) NOT NULL,
    description  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_chats_owner_wallet ON agent_chats (owner_wallet);

-- agent_chat_messages: the full conversation transcript, including turns
-- that never become a Task (plain back-and-forth, clarifying questions).
CREATE TABLE IF NOT EXISTS agent_chat_messages (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    chat_id        BIGINT NOT NULL REFERENCES agent_chats (id),
    sender         VARCHAR(10) NOT NULL CHECK (sender IN ('user', 'supervisor')),
    content_type   VARCHAR(15) NOT NULL DEFAULT 'text'
                       CHECK (content_type IN ('text', 'workflow_card', 'chart')),
    content        TEXT NOT NULL,
    ui_component   VARCHAR(20) CHECK (ui_component IN ('plan_tracker', 'clarifying_questions', 'compiled_rule')),
    ui_props       JSONB,
    ui_ref_task_id BIGINT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_chat_messages_chat_id ON agent_chat_messages (chat_id, created_at);

-- agent_tasks: drop the retired trader/fund_manager role split and the flat
-- reasoning blob (agent_sub_tasks below replaces it as the audit trail),
-- add the chat-intake and Trade-escalation columns from
-- docs/plans/agent-role-architecture.md §3a.
ALTER TABLE agent_tasks DROP COLUMN IF EXISTS role;
ALTER TABLE agent_tasks DROP COLUMN IF EXISTS reasoning;
ALTER TABLE agent_tasks DROP COLUMN IF EXISTS reasoning_snapshot;

ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS source_message_id BIGINT REFERENCES agent_chat_messages (id);
ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS raw_prompt TEXT;
ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS is_actionable BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS is_buy BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS fixed_amount NUMERIC(38, 0);
ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS on_chain_trade_id BIGINT;

ALTER TABLE agent_tasks ALTER COLUMN ticker DROP NOT NULL;
ALTER TABLE agent_tasks ALTER COLUMN amount_bps_cap DROP NOT NULL;
ALTER TABLE agent_tasks ALTER COLUMN trigger_description DROP NOT NULL;

ALTER TABLE agent_tasks DROP CONSTRAINT IF EXISTS agent_tasks_status_check;
ALTER TABLE agent_tasks ADD CONSTRAINT agent_tasks_status_check
    CHECK (status IN ('answered', 'pending', 'executed', 'skipped', 'cancelled', 'failed'));

CREATE INDEX IF NOT EXISTS idx_agent_tasks_source_message_id ON agent_tasks (source_message_id);

-- wallet_address is kept as a denormalized owner column rather than dropped
-- in favor of joining source_message_id -> agent_chats.owner_wallet: the
-- structured POST /agent/tasks path leaves source_message_id NULL and has
-- no chat to join through, so it would otherwise have no owner at all.
COMMENT ON COLUMN agent_tasks.wallet_address IS
    'Denormalized owner. Not solely derived via source_message_id -> agent_chats.owner_wallet because the structured POST /agent/tasks path leaves source_message_id NULL.';

-- agent_sub_tasks: the per-Task hash-chained decision trail, replacing the
-- old flat reasoning_snapshot. See docs/plans/agent-role-architecture.md §3a/§7.
CREATE TABLE IF NOT EXISTS agent_sub_tasks (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    task_id            BIGINT NOT NULL REFERENCES agent_tasks (id),
    step_order         INT NOT NULL,
    agent              VARCHAR(15) NOT NULL CHECK (agent IN ('supervisor', 'analyzer', 'executor')),
    step_name          VARCHAR(50) NOT NULL,
    status             VARCHAR(15) NOT NULL CHECK (status IN ('pending', 'done', 'failed', 'needs_input')),
    reasoning          TEXT NOT NULL,
    -- TEXT, not JSONB: the exact bytes stored here must be byte-identical
    -- to what was keccak256-hashed into decision_hash at write time.
    -- Postgres's JSONB type reformats stored JSON (whitespace, key order),
    -- which would silently break independent hash-chain verification —
    -- same reasoning the old flat reasoning_snapshot column used TEXT.
    output             TEXT,
    prev_decision_hash CHAR(66) NOT NULL,
    decision_hash      CHAR(66) NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_sub_tasks_task_id ON agent_sub_tasks (task_id, step_order);
