CREATE TABLE IF NOT EXISTS agent_checkpoints (
    checkpoint_id VARCHAR(255) PRIMARY KEY,
    data BYTEA NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_checkpoints_updated_at ON agent_checkpoints(updated_at);
