'use client';

import type { AgentTask } from '@/http/agent/taskApi';
import { PlanCard } from './PlanCard';
import { ArmPanel } from './ArmPanel';
import { TradeLedger } from './TradeLedger';

type TaskDetailProps = {
  task: AgentTask;
};

// Pixel-matched composition of Agent Chat.dc.html's Task detail flow:
// plan card first, then either the pre-arm ArmPanel or the post-arm
// TradeLedger (which itself covers armed/paused/disarmed banners).
export function TaskDetail({ task }: TaskDetailProps) {
  const isArmed = task.on_chain_task_id != null;

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

      {task.is_actionable && !isArmed && <ArmPanel taskId={task.id} isActionable={task.is_actionable} durationSec={24 * 60 * 60} />}

      {isArmed && <TradeLedger task={task} />}
    </div>
  );
}
