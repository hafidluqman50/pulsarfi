'use client';

import { useRef, useState } from 'react';
import { useTaskReasoning, useSendChatMessage, useAgentTasks } from '@/http/agent/hooks';
import { agentDisplayName, statusColor, stepDisplayName, StructuredOrProse } from './SubTaskReasoning';
import { ArmPanel } from './ArmPanel';
import { TradeLedger } from './TradeLedger';
import { parseCardContract } from './cardContract';

type PlanCardProps = {
  taskId: number;
  chatId?: string;
};

function extractTaskBudget(task?: { trigger_description?: string | null; summary?: string | null; raw_prompt?: string | null }): string | undefined {
  if (!task) return undefined;
  if (task.trigger_description) {
    try {
      const parsed = JSON.parse(task.trigger_description);
      if (parsed.budget_idrx && Number(parsed.budget_idrx) > 0) return String(parsed.budget_idrx);
      if (parsed.budget && Number(parsed.budget) > 0) return String(parsed.budget);
    } catch {
      const match = task.trigger_description.match(/\b(\d+)\b/);
      if (match && Number(match[1]) > 0) return match[1];
    }
  }
  if (task.summary) {
    const clean = task.summary.replace(/\./g, '');
    const match = clean.match(/(?:budget|senilai|sebesar)\s*(\d+)/i) || clean.match(/(\d+)\s*idrx/i);
    if (match && Number(match[1]) > 0) return match[1];
  }
  if (task.raw_prompt) {
    const clean = task.raw_prompt.replace(/\./g, '');
    const match = clean.match(/(?:budget|idrx_cap|batas.*budget).*?:\s*(\d+)/i);
    if (match && Number(match[1]) > 0) return match[1];
  }
  return undefined;
}

export function PlanCard({ taskId, chatId }: PlanCardProps) {
  const { data: subTasks = [] } = useTaskReasoning(taskId);
  const { data: tasks = [] } = useAgentTasks();
  const task = tasks.find((t) => t.id === taskId);
  const confirmedBudget = extractTaskBudget(task);
  const contract = parseCardContract(task);
  const [openRow, setOpenRow] = useState<number | null>(null);
  const [answerDraft, setAnswerDraft] = useState('');
  const sendMessage = useSendChatMessage(chatId);
  const isSendingAnswerRef = useRef(false);

  function sendAnswer(answer: string) {
    const trimmed = answer.trim();
    if (!trimmed || isSendingAnswerRef.current) return;
    isSendingAnswerRef.current = true;
    sendMessage.mutate(trimmed, {
      onSettled: () => {
        isSendingAnswerRef.current = false;
      },
    });
    setAnswerDraft('');
  }

  const genesisHash = subTasks[0]?.prev_decision_hash;
  const canArm = Boolean(task?.is_actionable && task.status !== 'failed' && task.status !== 'cancelled');

  return (
    <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)' }}>
      <div style={{ background: 'var(--ink)', color: 'var(--canvas)', padding: '10px 13px', display: 'flex', alignItems: 'center', gap: 9 }}>
        <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>
          {contract.task_badge.replace('{id}', String(taskId))}
        </span>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--hairline-strong)' }}>
          {subTasks.length} {contract.subtask_unit}
        </span>
      </div>
      <div style={{ borderBottom: '1px solid var(--hairline)', padding: '10px 13px', fontSize: 11.5, color: 'var(--body)', lineHeight: 1.5 }}>
        {contract.header_description}
      </div>
      {genesisHash && (
        <div style={{ borderBottom: '1px solid var(--hairline)', padding: '9px 13px', fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)', lineHeight: 1.5, overflowWrap: 'anywhere' }}>
          {contract.genesis_label.replace('{hash}', genesisHash)}
        </div>
      )}

      {subTasks.map((subTask) => {
        const isOpen = openRow === subTask.id;
        const needsInput = subTask.status === 'needs_input';
        const color = statusColor(subTask.status);
        const statusLabel = contract.status_labels[subTask.status] || subTask.status.toUpperCase();
        return (
          <div key={subTask.id} style={{ borderBottom: '1px solid var(--hairline)' }}>
            <button
              onClick={() => setOpenRow(isOpen ? null : subTask.id)}
              style={{ width: '100%', appearance: 'none', border: 0, cursor: 'pointer', background: 'transparent', padding: '11px 13px', textAlign: 'left', display: 'block' }}
            >
              <span style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}>
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--ticker)', flex: 'none' }}>
                  {String(subTask.step_order).padStart(2, '0')}
                </span>
                <span style={{ fontSize: 14, fontWeight: 600, lineHeight: 1.3, flex: '1 1 120px', minWidth: 0 }}>
                  {stepDisplayName(subTask.label, subTask.step_name)}
                </span>
                <span style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 7, flex: 'none' }}>
                  <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)' }}>{agentDisplayName(subTask.agent)}</span>
                  <span style={{ font: '600 9px/1.3 var(--font-sans)', letterSpacing: '.1em', textTransform: 'uppercase', color, border: `1px solid ${color}`, padding: '3px 4px', whiteSpace: 'nowrap' }}>
                    {statusLabel}
                  </span>
                </span>
              </span>
            </button>

            {isOpen && !needsInput && (
              <div style={{ margin: '0 13px 12px 35px', borderLeft: '1px solid var(--hairline)', paddingLeft: 12 }}>
                <div style={{ font: '600 8.5px/1.2 var(--font-sans)', letterSpacing: '.12em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 6 }}>
                  {agentDisplayName(subTask.agent)}
                </div>
                <div style={{ marginBottom: 14 }}>
                  <StructuredOrProse raw={subTask.reasoning} />
                </div>
                {subTask.output && (
                  <>
                    <div style={{ font: '600 8.5px/1.2 var(--font-sans)', letterSpacing: '.12em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 8 }}>
                      {agentDisplayName(subTask.agent)}
                    </div>
                    <StructuredOrProse raw={subTask.output} />
                  </>
                )}
                <div style={{ display: 'block', fontFamily: 'var(--font-mono)', fontSize: 9.5, color: 'var(--ticker)', marginTop: 10, lineHeight: 1.5, overflowWrap: 'anywhere' }}>
                  prev {subTask.prev_decision_hash} -&gt; hash {subTask.decision_hash}
                </div>
              </div>
            )}

            {isOpen && needsInput && (
              <div style={{ margin: '0 13px 13px 35px', borderLeft: '2px solid var(--merah)', paddingLeft: 12 }}>
                <div style={{ fontSize: 12, color: 'var(--body)', lineHeight: 1.5, marginBottom: 12 }}>
                  {contract.needs_input.notice}
                </div>
                {chatId != null && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
                    <input
                      style={{ appearance: 'none', border: '1px solid var(--hairline-strong)', background: 'var(--putih)', color: 'var(--ink)', font: '500 12.5px/1.35 var(--font-sans)', padding: '9px 10px' }}
                      placeholder={contract.needs_input.placeholder}
                      value={answerDraft}
                      onChange={(e) => setAnswerDraft(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') sendAnswer(answerDraft);
                      }}
                    />
                    <button
                      disabled={!answerDraft.trim() || sendMessage.isPending}
                      onClick={() => sendAnswer(answerDraft)}
                      style={{ appearance: 'none', cursor: 'pointer', border: '1px solid var(--ink)', background: 'var(--ink)', color: 'var(--canvas)', font: '600 12px/1 var(--font-sans)', padding: '9px 10px', alignSelf: 'flex-start' }}
                    >
                      {contract.needs_input.button}
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>
        );
      })}

      {canArm && task && task.status !== 'executed' && (
        <div style={{ borderTop: '1px solid var(--ink)' }}>
          <ArmPanel
            key={task.id}
            taskId={task.id}
            isActionable={task.is_actionable}
            durationSec={24 * 60 * 60}
            initialArmed={Boolean(task.armed_at)}
            totalBudget={confirmedBudget}
            contract={contract}
          />
        </div>
      )}

      {task?.is_actionable && task.armed_at && task.status === 'executed' && (
        <div style={{ borderTop: '1px solid var(--ink)' }}>
          <TradeLedger task={task} contract={contract} />
        </div>
      )}
    </div>
  );
}
