'use client';

import { useState } from 'react';
import { Icon } from '@/components/ui/Icon';
import { useAgentChats, useAgentTasks, useCreateChat } from '@/http/agent/hooks';
import { RosterCard } from './RosterCard';
import { MenuPanel, type QuasarDestination } from './MenuPanel';
import { ChatThread } from './ChatThread';
import { TaskDetail } from './TaskDetail';

export function QuasarPanel() {
  const [expanded, setExpanded] = useState(false);
  const [destination, setDestination] = useState<QuasarDestination>('chat');
  const [activeChatId, setActiveChatId] = useState<number | null>(null);
  const [activeTaskId, setActiveTaskId] = useState<number | null>(null);

  const { data: chats = [] } = useAgentChats();
  const { data: tasks = [] } = useAgentTasks();
  const createChat = useCreateChat();

  async function handleNewChat() {
    const chat = await createChat.mutateAsync();
    setActiveChatId(chat.id);
    setDestination('chat');
  }

  if (!expanded) {
    return (
      <button
        className="fixed bottom-[24px] right-[24px] flex items-center gap-[8px] border border-[var(--ink)] bg-[var(--canvas)] px-[16px] py-[10px] text-[13px] font-semibold"
        onClick={() => setExpanded(true)}
      >
        <span className="h-[8px] w-[8px] rounded-full bg-[var(--positive)]" />
        Quasar
      </button>
    );
  }

  const activeTask = tasks.find((t) => t.id === activeTaskId) ?? null;

  return (
    <div className="fixed bottom-[24px] right-[24px] flex h-[560px] w-[400px] flex-col border border-[var(--ink)] bg-[var(--canvas)]">
      <header className="hairline flex items-center justify-between px-[16px] py-[12px]">
        <div className="flex items-center gap-[8px]">
          <span className="h-[8px] w-[8px] rounded-full bg-[var(--positive)]" />
          <span className="font-semibold">Quasar</span>
        </div>
        <div className="flex items-center gap-[12px]">
          <button className="text-[12px]" onClick={handleNewChat} disabled={createChat.isPending}>
            + new chat
          </button>
          <button onClick={() => setDestination(destination === 'chat' ? 'tasks' : 'chat')}>
            <Icon name="menu" size={16} />
          </button>
          <button onClick={() => setExpanded(false)}>
            <Icon name="x" size={16} />
          </button>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto">
        {destination !== 'chat' && !activeTask && (
          <MenuPanel active={destination} onSelect={setDestination} />
        )}

        {destination === 'tasks' && !activeTask && (
          <div className="flex flex-col">
            {tasks.map((task) => (
              <button
                key={task.id}
                className="hairline-top flex items-center justify-between px-[16px] py-[12px] text-left text-[13px]"
                onClick={() => setActiveTaskId(task.id)}
              >
                <span>T-{task.id} · {task.summary ?? task.raw_prompt ?? '—'}</span>
                <span className="text-[11px] text-[var(--body)]">{task.status}</span>
              </button>
            ))}
            {tasks.length === 0 && <div className="p-[16px] text-[13px] text-[var(--body)]">No tasks yet.</div>}
          </div>
        )}

        {destination === 'tasks' && activeTask && (
          <div>
            <button className="px-[16px] pt-[12px] text-[12px] text-[var(--body)]" onClick={() => setActiveTaskId(null)}>
              &lt; back to tasks
            </button>
            <TaskDetail task={activeTask} />
          </div>
        )}

        {destination === 'history' && (
          <div className="flex flex-col">
            {chats.map((chat) => (
              <button
                key={chat.id}
                className="hairline-top px-[16px] py-[12px] text-left text-[13px]"
                onClick={() => {
                  setActiveChatId(chat.id);
                  setDestination('chat');
                }}
              >
                {chat.description ?? `Chat #${chat.id}`}
              </button>
            ))}
            {chats.length === 0 && <div className="p-[16px] text-[13px] text-[var(--body)]">No chat history yet.</div>}
          </div>
        )}

        {destination === 'chat' && (
          <>
            {!activeChatId && (
              <div className="p-[16px]">
                <RosterCard />
                <p className="mt-[12px] text-[13px] text-[var(--body)]">Start a new chat to talk to Quasar.</p>
                <button
                  className="btn btn-ghost !mt-[12px] !border !border-[var(--ink)] !px-[16px] !py-[8px] !text-[13px]"
                  onClick={handleNewChat}
                  disabled={createChat.isPending}
                >
                  + new chat
                </button>
              </div>
            )}
            {activeChatId && <ChatThread chatId={activeChatId} />}
          </>
        )}
      </div>
    </div>
  );
}
