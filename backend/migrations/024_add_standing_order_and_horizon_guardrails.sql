-- Migration 024: Add standing order (DCA) and horizon guardrails to agent_tasks
ALTER TABLE agent_tasks
    ADD COLUMN IF NOT EXISTS is_recurring BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS cooldown_sec INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_per_trade BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS horizon_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS horizon_notified_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS exit_policy VARCHAR(32) NOT NULL DEFAULT 'undecided';

CREATE INDEX IF NOT EXISTS idx_agent_tasks_recurring_due 
    ON agent_tasks (is_recurring, status, next_run_at) 
    WHERE is_recurring = TRUE AND status = 'armed';

CREATE INDEX IF NOT EXISTS idx_agent_tasks_horizon_pending 
    ON agent_tasks (horizon_expires_at, horizon_notified_at) 
    WHERE horizon_expires_at IS NOT NULL AND horizon_notified_at IS NULL;

-- Update status check constraint to include 'armed' and 'settled_held'
ALTER TABLE agent_tasks DROP CONSTRAINT IF EXISTS agent_tasks_status_check;
ALTER TABLE agent_tasks ADD CONSTRAINT agent_tasks_status_check
    CHECK (status IN ('answered', 'pending', 'armed', 'executed', 'skipped', 'cancelled', 'failed', 'settled_held'));

