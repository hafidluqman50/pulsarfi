'use client';

import { useEffect, useState } from 'react';
import { useAgentActivity, useAgentChats, useAgentTasks } from '@/http/agent/hooks';
import { RosterCard } from './RosterCard';
import { MenuPanel, type QuasarDestination } from './MenuPanel';
import { ChatThread } from './ChatThread';
import { TaskDetail } from './TaskDetail';
import { agentDisplayName, statusColor, stepDisplayName } from './SubTaskReasoning';

const ACTIVITY_KINDS = ['all', 'supervisor', 'analyzer', 'executor'] as const;
type ActivityKind = (typeof ACTIVITY_KINDS)[number];

export function QuasarPanel() {
  const [expanded, setExpanded] = useState(false);
  const [destination, setDestination] = useState<QuasarDestination>('chat');
  const [activeChatId, setActiveChatId] = useState<string | null>(null);
  const [activeTaskId, setActiveTaskId] = useState<number | null>(null);
  const [isSheet, setIsSheet] = useState(false);
  const [activityKind, setActivityKind] = useState<ActivityKind>('all');

  useEffect(() => {
    const check = () => setIsSheet(window.innerWidth < 700);
    check();
    window.addEventListener('resize', check);
    return () => window.removeEventListener('resize', check);
  }, []);

  const { data: chats = [] } = useAgentChats();
  const { data: tasks = [] } = useAgentTasks();
  const { data: activity = [] } = useAgentActivity();

  useEffect(() => {
    if (activeChatId || chats.length === 0) return;
    setActiveChatId(chats[0].id);
  }, [chats, activeChatId]);

  function handleNewChat() {
    setActiveChatId(crypto.randomUUID());
    setActiveTaskId(null);
    setDestination('chat');
  }

  if (!expanded) {
    return (
      <button
        onClick={() => setExpanded(true)}
        style={{
          position: 'fixed',
          right: 'clamp(12px,2vw,22px)',
          bottom: 'clamp(12px,2vw,22px)',
          zIndex: 300,
          appearance: 'none',
          border: '1px solid var(--ink)',
          cursor: 'pointer',
          background: 'var(--ink)',
          color: 'var(--canvas)',
          padding: '14px 18px',
          display: 'flex',
          alignItems: 'center',
          gap: 11,
          textAlign: 'left',
        }}
        className="quasar-launcher"
      >
        <span className="pulsar" />
        <span style={{ font: '600 12px/1 var(--font-sans)', letterSpacing: '.1em', textTransform: 'uppercase' }}>Quasar</span>
      </button>
    );
  }

  const activeTask = tasks.find((t) => t.id === activeTaskId) ?? null;

  return (
    <div
      className="panel"
      style={{
        position: 'fixed',
        right: isSheet ? 0 : 22,
        bottom: isSheet ? 0 : 22,
        zIndex: 300,
        width: isSheet ? '100vw' : 452,
        height: isSheet ? '100dvh' : 'min(760px, calc(100vh - 146px))',
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--canvas)',
        border: isSheet ? '0' : '1px solid var(--ink)',
      }}
    >
      <div style={{ background: 'var(--ink)', color: 'var(--canvas)', padding: '11px 12px', display: 'flex', alignItems: 'center', gap: 9, flex: 'none' }}>
        <span className="pulsar" />
        <span style={{ fontFamily: 'var(--font-display)', fontSize: 17, letterSpacing: '-.01em', whiteSpace: 'nowrap' }}>Quasar</span>
        <button
          onClick={handleNewChat}
          style={{ marginLeft: 'auto', appearance: 'none', border: '1px solid var(--ink-line)', cursor: 'pointer', background: 'transparent', color: 'var(--canvas)', font: '500 11px/1 var(--font-mono)', padding: '7px 9px', flex: 'none', whiteSpace: 'nowrap' }}
        >
          + new chat
        </button>
        <button
          onClick={() => setDestination(destination === 'chat' ? 'tasks' : 'chat')}
          style={{ marginLeft: 0, appearance: 'none', border: '1px solid var(--ink-line)', cursor: 'pointer', background: 'transparent', color: 'var(--canvas)', font: '500 11px/1 var(--font-mono)', padding: '7px 9px', flex: 'none' }}
        >
          menu
        </button>
        <button
          onClick={() => setExpanded(false)}
          style={{ appearance: 'none', border: '1px solid var(--ink-line)', cursor: 'pointer', background: 'transparent', color: 'var(--canvas)', font: '400 14px/1 var(--font-sans)', padding: '5px 9px', flex: 'none' }}
        >
          &minus;
        </button>
      </div>

      {destination !== 'chat' && !activeTask && (
        <MenuPanel active={destination} onSelect={setDestination} />
      )}

      {destination === 'tasks' && !activeTask && (
        <div className="rise" style={{ flex: 1, overflowY: 'auto' }}>
          {tasks.map((task) => (
            <button
              key={task.id}
              onClick={() => setActiveTaskId(task.id)}
              style={{ width: '100%', appearance: 'none', border: 0, borderBottom: '1px solid var(--hairline)', cursor: 'pointer', background: 'var(--canvas)', padding: 13, textAlign: 'left' }}
            >
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 9, marginBottom: 6 }}>
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--ticker)' }}>T-{task.id}</span>
                <span style={{ font: '600 9px/1.3 var(--font-sans)', letterSpacing: '.1em', textTransform: 'uppercase', color: 'var(--body)', border: '1px solid var(--body)', padding: '3px 5px', whiteSpace: 'nowrap' }}>
                  {task.status}
                </span>
              </div>
              <div style={{ fontSize: 14, fontWeight: 600, lineHeight: 1.3 }}>{task.summary ?? task.raw_prompt ?? '—'}</div>
            </button>
          ))}
          {tasks.length === 0 && <div style={{ padding: 13, fontSize: 13, color: 'var(--body)' }}>No tasks yet.</div>}
        </div>
      )}

      {destination === 'tasks' && activeTask && (
        <div className="rise" style={{ flex: 1, overflowY: 'auto' }}>
          <div style={{ padding: '11px 13px', borderBottom: '1px solid var(--hairline)', display: 'flex', alignItems: 'center', gap: 10 }}>
            <button
              onClick={() => setActiveTaskId(null)}
              style={{ appearance: 'none', border: 0, background: 'transparent', cursor: 'pointer', padding: 0, font: '600 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase', color: 'var(--body)' }}
            >
              &larr; Tasks
            </button>
          </div>
          <TaskDetail task={activeTask} />
        </div>
      )}

      {destination === 'history' && (
        <div className="rise" style={{ flex: 1, overflowY: 'auto' }}>
          {chats.map((chat) => (
            <button
              key={chat.id}
              onClick={() => {
                setActiveChatId(chat.id);
                setDestination('chat');
              }}
              style={{ width: '100%', appearance: 'none', border: 0, borderBottom: '1px solid var(--hairline)', cursor: 'pointer', background: 'var(--canvas)', padding: 13, textAlign: 'left' }}
            >
              <div style={{ fontSize: 14, fontWeight: 600, lineHeight: 1.3 }}>{chat.description ?? `Chat #${chat.id}`}</div>
            </button>
          ))}
          {chats.length === 0 && <div style={{ padding: 13, fontSize: 13, color: 'var(--body)' }}>No chat history yet.</div>}
        </div>
      )}

      {destination === 'activity' && (
        <div className="rise" style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column' }}>
          <div style={{ display: 'flex', gap: 4, padding: '10px 13px', borderBottom: '1px solid var(--hairline)', flex: 'none' }}>
            {ACTIVITY_KINDS.map((kind) => (
              <button
                key={kind}
                onClick={() => setActivityKind(kind)}
                style={{
                  appearance: 'none',
                  cursor: 'pointer',
                  border: `1px solid ${activityKind === kind ? 'var(--ink)' : 'var(--hairline)'}`,
                  background: activityKind === kind ? 'var(--ink)' : 'transparent',
                  color: activityKind === kind ? 'var(--canvas)' : 'var(--body)',
                  font: '600 10.5px/1 var(--font-mono)',
                  padding: '5px 9px',
                }}
              >
                {kind === 'all' ? 'ALL' : agentDisplayName(kind)}
              </button>
            ))}
          </div>
          <div style={{ flex: 1, overflowY: 'auto' }}>
            {activity
              .filter((row) => activityKind === 'all' || row.agent === activityKind)
              .map((row) => {
                const color = statusColor(row.status);
                return (
                  <button
                    key={row.id}
                    onClick={() => {
                      setActiveTaskId(row.task_id);
                      setDestination('tasks');
                    }}
                    style={{ width: '100%', appearance: 'none', border: 0, borderBottom: '1px solid var(--hairline)', cursor: 'pointer', background: 'var(--canvas)', padding: 13, textAlign: 'left' }}
                  >
                    <div style={{ display: 'flex', alignItems: 'baseline', gap: 9, marginBottom: 6 }}>
                      <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--ticker)' }}>T-{row.task_id}</span>
                      <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)' }}>{agentDisplayName(row.agent)}</span>
                      <span style={{ marginLeft: 'auto', font: '600 9px/1.3 var(--font-sans)', letterSpacing: '.1em', textTransform: 'uppercase', color, border: `1px solid ${color}`, padding: '3px 4px', whiteSpace: 'nowrap' }}>
                        {row.status.toUpperCase()}
                      </span>
                    </div>
                    <div style={{ fontSize: 14, fontWeight: 600, lineHeight: 1.3 }}>{stepDisplayName(row.label, row.step_name)}</div>
                  </button>
                );
              })}
            {activity.length === 0 && <div style={{ padding: 13, fontSize: 13, color: 'var(--body)' }}>No activity yet.</div>}
          </div>
        </div>
      )}

      {destination === 'chat' && (
        <>
          {!activeChatId && (
            <div className="thread" style={{ flex: 1, overflowY: 'auto', padding: '16px 14px', display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--putih)', padding: '13px 15px' }}>
                <div style={{ fontSize: 14.5, lineHeight: 1.55 }}>
                  I am Quasar. Write the instruction in your own words — a fast trade or a long mandate, I read the horizon out of what you wrote. Standing
                  instructions I turn into a rule with named sources, a number and hard caps, and show you every step before anything is armed.
                </div>
              </div>
              <RosterCard />
              <button
                onClick={handleNewChat}
                style={{ appearance: 'none', cursor: 'pointer', border: '1px solid var(--hairline-strong)', background: 'transparent', color: 'var(--ink)', font: '400 13.5px/1.45 var(--font-sans)', padding: '10px 12px', textAlign: 'left', display: 'flex', gap: 9 }}
              >
                <span style={{ color: 'var(--merah)', fontFamily: 'var(--font-mono)', fontSize: 12, flex: 'none' }}>&rarr;</span>
                <span>Start a new chat to talk to Quasar</span>
              </button>
            </div>
          )}
          {activeChatId && <ChatThread chatId={activeChatId} />}
        </>
      )}
    </div>
  );
}
