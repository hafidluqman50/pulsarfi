-- When grantTradePermission actually succeeded for this Task, and nothing
-- else. Before this column, the frontend inferred "armed" from
-- on_chain_task_id != null — correct only under the pre-rebuild design,
-- where on_chain_task_id was set by the Arm action itself. Since createTask
-- now fires the instant a Task is recognized (any Task, actionable or not),
-- that inference reads true immediately and ArmPanel could never render at
-- all (docs/plans/fix-comet-trade-execution-blockers.md Defect B).
-- Nullable on purpose: null means "not armed", and a Task that is never
-- armed keeps it null forever.
ALTER TABLE agent_tasks ADD COLUMN armed_at TIMESTAMPTZ;
