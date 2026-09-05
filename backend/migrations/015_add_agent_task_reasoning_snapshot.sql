-- Raw JSON bytes, stored verbatim and never re-serialized after being set —
-- this is the exact payload whose keccak256 is recorded on-chain as
-- reasoningHash (see AgentTaskManager.executeTask/logDecision). The API
-- endpoint serving this back to callers must return these bytes unchanged,
-- not reconstruct them from other columns, or the hash stops matching.
ALTER TABLE agent_tasks ADD COLUMN IF NOT EXISTS reasoning_snapshot TEXT;
