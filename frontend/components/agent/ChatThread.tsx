'use client';

import { useState } from 'react';
import { Icon } from '@/components/ui/Icon';
import { useChatMessages, useSendChatMessage } from '@/http/agent/hooks';
import { PlanCard } from './PlanCard';

type ChatThreadProps = {
  chatId: number;
};

// Chart rendering (echarts + a deterministic lens -> option mapping) is
// not wired up yet in this pass — this shows the raw lens name and data
// honestly rather than faking a chart component that doesn't exist.
function ChartPlaceholder({ uiProps }: { uiProps: unknown }) {
  const props = uiProps as { lens?: string; lensNote?: string; chartQ?: string } | undefined;
  return (
    <div className="hairline p-[12px] text-[12px] text-[var(--body)]">
      <p>Chart data received (lens: {props?.lens ?? 'unknown'}) — visualization not wired up yet.</p>
      {props?.lensNote && <p className="mt-[4px]">{props.lensNote}</p>}
    </div>
  );
}

export function ChatThread({ chatId }: ChatThreadProps) {
  const { data: messages = [], isLoading } = useChatMessages(chatId);
  const sendMessage = useSendChatMessage(chatId);
  const [draft, setDraft] = useState('');

  function handleSend() {
    const trimmed = draft.trim();
    if (!trimmed || sendMessage.isPending) return;
    sendMessage.mutate(trimmed);
    setDraft('');
  }

  return (
    <div className="flex flex-col gap-[12px] p-[16px]">
      {isLoading && <div className="skeleton h-[80px] w-full" />}

      {messages.map((message) => (
        <div key={message.id} className={message.sender === 'user' ? 'self-end text-right' : 'self-start'}>
          <div className={`inline-block max-w-[320px] px-[12px] py-[8px] text-[13px] ${message.sender === 'user' ? 'bg-[var(--ink)] text-[var(--canvas)]' : 'hairline'}`}>
            {message.content}
          </div>

          {message.sender === 'supervisor' && message.content_type === 'chart' && (
            <div className="mt-[8px]">
              <ChartPlaceholder uiProps={message.ui_props} />
            </div>
          )}

          {message.sender === 'supervisor' && message.ui_ref_task_id != null && (
            <div className="mt-[8px]">
              <PlanCard taskId={message.ui_ref_task_id} chatId={chatId} />
            </div>
          )}
        </div>
      ))}

      <div className="hairline-top mt-[8px] flex gap-[8px] pt-[12px]">
        <input
          className="flex-1 border border-[var(--hairline-strong)] px-[10px] py-[8px] text-[13px]"
          placeholder="Only this box is treated as a command."
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSend()}
        />
        <button className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[8px]" disabled={!draft.trim() || sendMessage.isPending} onClick={handleSend}>
          <Icon name="send" size={16} />
        </button>
      </div>
    </div>
  );
}
