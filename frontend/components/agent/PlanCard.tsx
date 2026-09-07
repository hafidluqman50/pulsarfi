'use client';

import { useRef, useState } from 'react';
import { useTaskReasoning, useSendChatMessage } from '@/http/agent/hooks';
import { agentDisplayName, statusColor, stepDisplayName, StructuredOrProse } from './SubTaskReasoning';

type PlanCardProps = {
  taskId: number;
  chatId?: string;
};

export function PlanCard({ taskId, chatId }: PlanCardProps) {
  const { data: subTasks = [] } = useTaskReasoning(taskId);
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

  return (
    <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)' }}>
      <div style={{ background: 'var(--ink)', color: 'var(--canvas)', padding: '10px 13px', display: 'flex', alignItems: 'center', gap: 9 }}>
        <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>Task T-{taskId}</span>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--hairline-strong)' }}>{subTasks.length} sub tasks</span>
      </div>
      <div style={{ borderBottom: '1px solid var(--hairline)', padding: '10px 13px', fontSize: 11.5, color: 'var(--body)', lineHeight: 1.5 }}>
        This whole card is one <span style={{ fontWeight: 600, color: 'var(--ink)' }}>Task</span> — your request. Each numbered row is a{' '}
        <span style={{ fontWeight: 600, color: 'var(--ink)' }}>Sub Task</span>: one step, run by one agent, with its own reasoning and hash. Open a row to
        see what that step output.
      </div>
      {genesisHash && (
        <div style={{ borderBottom: '1px solid var(--hairline)', padding: '9px 13px', fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)', lineHeight: 1.5, overflowWrap: 'anywhere' }}>
          genesis {genesisHash} · keccak256(task_id, trigger, owner)
        </div>
      )}

      {subTasks.map((subTask) => {
        const isOpen = openRow === subTask.id;
        const needsInput = subTask.status === 'needs_input';
        const color = statusColor(subTask.status);
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
                <span style={{ fontSize: 14, fontWeight: 600, lineHeight: 1.3, flex: '1 1 120px', minWidth: 0 }}>{stepDisplayName(subTask.label, subTask.step_name)}</span>
                <span style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 7, flex: 'none' }}>
                  <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)' }}>{agentDisplayName(subTask.agent)}</span>
                  <span style={{ font: '600 9px/1.3 var(--font-sans)', letterSpacing: '.1em', textTransform: 'uppercase', color, border: `1px solid ${color}`, padding: '3px 4px', whiteSpace: 'nowrap' }}>
                    {needsInput ? 'NEEDS YOU' : subTask.status.toUpperCase()}
                  </span>
                </span>
              </span>
            </button>

            {isOpen && !needsInput && (
              <div style={{ margin: '0 13px 12px 35px', borderLeft: '1px solid var(--hairline)', paddingLeft: 12 }}>
                <div style={{ font: '600 8.5px/1.2 var(--font-sans)', letterSpacing: '.12em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 6 }}>
                  Reasoning · {agentDisplayName(subTask.agent)}
                </div>
                <div style={{ marginBottom: 14 }}>
                  <StructuredOrProse raw={subTask.reasoning} />
                </div>
                {subTask.output && (
                  <>
                    <div style={{ font: '600 8.5px/1.2 var(--font-sans)', letterSpacing: '.12em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 8 }}>
                      Output
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
                  This Sub Task is <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--merah)' }}>needs_input</span>. I will not create the Task
                  on a guessed number — nothing arms while an answer is missing.
                </div>
                {chatId != null && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
                    <input
                      style={{ appearance: 'none', border: '1px solid var(--hairline-strong)', background: 'var(--putih)', color: 'var(--ink)', font: '500 12.5px/1.35 var(--font-sans)', padding: '9px 10px' }}
                      placeholder="Your answer"
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
                      Send answer
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
