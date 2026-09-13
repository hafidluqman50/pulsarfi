-- The real, contract-assigned Sub Task id from recordSubTasks's own
-- SubTaskRecorded event — previously discarded entirely (only a tx hash was
-- kept). Needed so executeTrade's own subTaskId argument has something real
-- to reference, and so a row can never exist without the on-chain id that
-- proves it was actually confirmed (docs/plans/agent-orchestration-graph-rebuild.md v2.14).
ALTER TABLE agent_sub_tasks ADD COLUMN on_chain_sub_task_id BIGINT;
