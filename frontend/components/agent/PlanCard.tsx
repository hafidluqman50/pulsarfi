'use client';

import { useState } from 'react';
import { useTaskReasoning, useSendChatMessage } from '@/http/agent/hooks';

type PlanCardProps = {
  taskId: number;
  chatId?: number;
};

// Renders one Task as a numbered Sub Task list — the on-screen mirror of
// agent_sub_tasks in step order. Answering a needs_input row is just the
// next chat message (useSendChatMessage) — there is no separate
// answer-endpoint; Supervisor resolves it from conversation context.
export function PlanCard({ taskId, chatId }: PlanCardProps) {
  const { data: subTasks = [] } = useTaskReasoning(taskId);
  const [openRow, setOpenRow] = useState<number | null>(null);
  const [answerDraft, setAnswerDraft] = useState('');
  const sendMessage = useSendChatMessage(chatId);

  const genesisHash = subTasks[0]?.prev_decision_hash;

  return (
    <div className="plan-card hairline p-[16px]">
      <div className="flex items-center justify-between">
        <span className="font-semibold">Task T-{taskId}</span>
        <span className="text-[12px] text-[var(--body)]">{subTasks.length} sub tasks</span>
      </div>
      <p className="mt-[4px] text-[12px] text-[var(--body)]">
        This whole card is one Task — your request. Each numbered row is a Sub Task: one step, run by one agent, with
        its own reasoning and hash.
      </p>
      {genesisHash && <div className="mono mt-[8px] text-[11px] text-[var(--body)]">genesis {genesisHash}</div>}

      {subTasks.map((subTask) => {
        const isOpen = openRow === subTask.id;
        const needsInput = subTask.status === 'needs_input';
        return (
          <div key={subTask.id} className="hairline-top py-[10px]">
            <button className="flex w-full items-center justify-between text-left" onClick={() => setOpenRow(isOpen ? null : subTask.id)}>
              <span className="text-[13px]">
                {String(subTask.step_order).padStart(2, '0')} {subTask.step_name}
              </span>
              <span className="text-[11px] text-[var(--body)]">{subTask.agent}</span>
              <span className={`text-[11px] font-semibold ${needsInput ? 'text-[var(--negative)]' : ''}`}>
                {needsInput ? 'NEEDS YOU' : subTask.status.toUpperCase()}
              </span>
            </button>

            {isOpen && (
              <div className="mt-[8px] pl-[16px]">
                <p className="text-[12px]">Reasoning: {subTask.reasoning}</p>
                {subTask.output && <p className="mt-[4px] text-[12px] text-[var(--body)]">Output: {subTask.output}</p>}
                <p className="mono mt-[6px] text-[11px] text-[var(--body)]">
                  prev {subTask.prev_decision_hash} -&gt; hash {subTask.decision_hash}
                </p>

                {needsInput && chatId != null && (
                  <div className="mt-[8px] flex gap-[8px]">
                    <input
                      className="flex-1 border border-[var(--hairline-strong)] px-[8px] py-[6px] text-[13px]"
                      placeholder="Your answer"
                      value={answerDraft}
                      onChange={(e) => setAnswerDraft(e.target.value)}
                    />
                    <button
                      className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px] !text-[13px]"
                      disabled={!answerDraft.trim() || sendMessage.isPending}
                      onClick={() => {
                        sendMessage.mutate(answerDraft);
                        setAnswerDraft('');
                      }}
                    >
                      Send
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
