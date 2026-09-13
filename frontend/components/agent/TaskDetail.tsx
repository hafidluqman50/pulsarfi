'use client';

import type { AgentTask } from '@/http/agent/taskApi';
import { PlanCard } from './PlanCard';

type TaskDetailProps = {
  task: AgentTask;
};

// Pixel-matched composition of Agent Chat.dc.html's Task detail flow:
// plan card first, then either the pre-arm ArmPanel or the post-arm
// TradeLedger (which itself covers armed/paused/disarmed banners).
export function TaskDetail({ task }: TaskDetailProps) {
  // armed_at, not on_chain_task_id: the latter is set the instant any Task
  // is recognized, so inferring "armed" from it made this true immediately
  // and ArmPanel could never render at all.
  const isArmed = task.armed_at != null;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14, padding: '16px 14px' }}>
      <div>
        <div style={{ fontSize: 15, fontWeight: 600 }}>Task T-{task.id}</div>
        <div style={{ fontSize: 12, color: 'var(--body)' }}>{task.summary ?? task.raw_prompt ?? 'No summary yet'}</div>
        <div style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--body)', marginTop: 4 }}>
          status {task.status} · {task.is_actionable ? 'actionable' : 'informational'}
          {task.paused && ' · paused'}
        </div>
      </div>

      <PlanCard taskId={task.id} />
    </div>
  );
}
