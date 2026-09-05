'use client';

import type { AgentTask } from '@/http/agent/taskApi';
import { useDisarmTask, usePauseTask, useResumeTask } from '@/http/agent/hooks';
import { PlanCard } from './PlanCard';
import { ArmPanel } from './ArmPanel';
import { TradeLedger } from './TradeLedger';

type TaskDetailProps = {
  task: AgentTask;
};

export function TaskDetail({ task }: TaskDetailProps) {
  const disarmTask = useDisarmTask(task.id);
  const pauseTask = usePauseTask(task.id);
  const resumeTask = useResumeTask(task.id);

  const isArmed = task.on_chain_task_id != null;

  return (
    <div className="flex flex-col gap-[12px] p-[16px]">
      <div>
        <div className="text-[15px] font-semibold">Task T-{task.id}</div>
        <div className="text-[12px] text-[var(--body)]">{task.summary ?? task.raw_prompt ?? 'No summary yet'}</div>
        <div className="mono mt-[4px] text-[11px] text-[var(--body)]">
          status {task.status} · {task.is_actionable ? 'actionable' : 'informational'}
          {task.paused && ' · paused'}
        </div>
      </div>

      <PlanCard taskId={task.id} />

      {task.is_actionable && !isArmed && (
        <ArmPanel taskId={task.id} isActionable={task.is_actionable} durationSec={24 * 60 * 60} />
      )}

      {isArmed && (
        <>
          <TradeLedger taskId={task.id} onChainTaskId={task.on_chain_task_id!} />
          <div className="flex gap-[8px]">
            {task.paused ? (
              <button className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px] !text-[13px]" onClick={() => resumeTask.mutate()}>
                Resume
              </button>
            ) : (
              <button className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px] !text-[13px]" onClick={() => pauseTask.mutate()}>
                Pause
              </button>
            )}
            <button className="btn btn-ghost !border !border-[var(--negative)] !px-[12px] !py-[6px] !text-[13px] !text-[var(--negative)]" onClick={() => disarmTask.mutate()}>
              Disarm
            </button>
          </div>
        </>
      )}
    </div>
  );
}
