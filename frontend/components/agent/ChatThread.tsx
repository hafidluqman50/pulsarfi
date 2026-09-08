'use client';

import { memo, useCallback, useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { sendChatMessage, retryLastMessage, chatStreamTopic, type AgentChatMessage, type ChatStreamEvent, type SubTaskStarted } from '@/http/agent/chatApi';
import type { AgentSubTask } from '@/http/agent/taskApi';
import { useRealtimeTopic } from '@/http/realtime/useRealtimeSocket';
import { NewsBrief } from './NewsBrief';
import { PlanCard } from './PlanCard';
import { PortfolioChart } from './PortfolioChart';
import { agentDisplayName, humanizeKey, statusColor, stepDisplayName, StructuredOrProse } from './SubTaskReasoning';
import { useChatMessages } from '@/http/agent/hooks';

type ChatThreadProps = {
  chatId: string;
};

let placeholderIdCounter = 0;

// A sub_task_started event has no persisted row yet — it becomes a
// synthetic in_progress placeholder in the live list, occupying that
// step's slot the instant it begins rather than only once it's done.
function placeholderSubTask(started: SubTaskStarted, stepOrder: number): AgentSubTask {
  placeholderIdCounter -= 1;
  return {
    id: placeholderIdCounter,
    task_id: 0,
    step_order: stepOrder,
    agent: started.agent,
    step_name: started.step_name,
    label: started.label,
    status: 'in_progress',
    reasoning: '',
    output: null,
    prev_decision_hash: '',
    decision_hash: '',
    recorded_on_chain: false,
    on_chain_tx_hash: null,
    created_at: new Date().toISOString(),
  };
}

// addStartedPlaceholder first marks any existing still-in_progress
// placeholder for the same (agent, step_name) as "retried" instead of
// leaving it stuck — Supervisor calling analyzer_agent/executor_agent a
// second time for the same step (e.g. its first attempt failed silently
// via WrapToolGraceful, no matching "done" ever arrived for it) used to
// leave that first placeholder orphaned in_progress forever, and its own
// client-assigned step_order could collide visually with the real
// step_order of whatever got recorded next. Only ever one in_progress
// placeholder per (agent, step_name) can exist after this, so
// mergeSubTaskDone's "find the matching placeholder" below stays
// unambiguous even across a retry.
function addStartedPlaceholder(prev: AgentSubTask[], started: SubTaskStarted): AgentSubTask[] {
  const superseded = prev.map((row) =>
    row.status === 'in_progress' && row.agent === started.agent && row.step_name === started.step_name ? { ...row, status: 'retried' } : row,
  );
  return [...superseded, placeholderSubTask(started, superseded.length + 1)];
}

// mergeSubTaskDone replaces the still-in_progress placeholder for the same
// (agent, step_name) with the real persisted row — same slot, same
// position, so a step never appears twice (once as "in progress", again as
// "done"). Falls back to appending if no matching placeholder exists (e.g.
// a step that never got a sub_task_started of its own).
function mergeSubTaskDone(prev: AgentSubTask[], done: AgentSubTask): AgentSubTask[] {
  const index = prev.findIndex((row) => row.status === 'in_progress' && row.agent === done.agent && row.step_name === done.step_name);
  if (index === -1) return [...prev, done];
  const next = [...prev];
  next[index] = done;
  return next;
}

// toolActivityByAgent maps agent ("analyzer"/"executor") to a human label for
// the tool it is currently (or was last seen) calling, sourced from tool_call
// events — this is per-agent, not per-(agent, step_name), because only one
// Sub Task per agent is ever in_progress at a time in the current graph
// (docs/plans/agent-orchestration-graph-rebuild.md v2.8). Kept set after a
// "end" phase (not cleared) so the label doesn't flicker back to the generic
// placeholder text between two tool calls in the same step — it only changes
// once the next tool call starts, or the row disappears once the step itself
// is done.
function LiveSubTasks({ subTasks, toolActivityByAgent }: { subTasks: AgentSubTask[]; toolActivityByAgent: Record<string, string> }) {
  const [openRow, setOpenRow] = useState<number | null>(null);
  if (subTasks.length === 0) return null;
  return (
    <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)' }}>
      <div style={{ background: 'var(--ink)', color: 'var(--canvas)', padding: '10px 13px', display: 'flex', alignItems: 'center', gap: 9 }}>
        <span className="pulsar" />
        <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>Working</span>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--hairline-strong)' }}>{subTasks.length} sub tasks so far</span>
      </div>
      {subTasks.map((subTask) => {
        const color = statusColor(subTask.status);
        const isOpen = openRow === subTask.id;
        return (
          <div key={subTask.id} className="rise" style={{ borderBottom: '1px solid var(--hairline)' }}>
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
                    {subTask.status === 'in_progress' ? 'IN PROGRESS' : subTask.status.toUpperCase()}
                  </span>
                </span>
              </span>
            </button>
            {isOpen && (
              <div style={{ margin: '0 13px 12px 35px', borderLeft: '1px solid var(--hairline)', paddingLeft: 12 }}>
                <div style={{ font: '600 8.5px/1.2 var(--font-sans)', letterSpacing: '.12em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 6 }}>
                  Reasoning · {agentDisplayName(subTask.agent)}
                </div>
                <div style={{ marginBottom: subTask.output ? 14 : 0 }}>
                  {subTask.status === 'in_progress' || subTask.status === 'retried' ? (
                    // Nothing to show yet, not a bug — the step's reasoning only
                    // exists once SubTaskRecorder.Record actually persists it, and
                    // that only happens once the whole step (e.g. Analyzer's
                    // search/read/conclude pass) has finished. A blank div here
                    // read as broken; say plainly what's actually going on instead.
                    // "retried" means Supervisor started this same step again
                    // before this attempt ever finished — this one simply never
                    // got a result, not a bug in this render.
                    <div style={{ fontSize: 12.5, lineHeight: 1.5, color: 'var(--ticker)', fontStyle: 'italic' }}>
                      {subTask.status === 'retried'
                        ? `${agentDisplayName(subTask.agent)} mengulang langkah ini sebelum percobaan ini selesai.`
                        : toolActivityByAgent[subTask.agent]
                          ? `${agentDisplayName(subTask.agent)} memanggil ${toolActivityByAgent[subTask.agent]}…`
                          : `${agentDisplayName(subTask.agent)} sedang memproses langkah ini…`}
                    </div>
                  ) : (
                    <StructuredOrProse raw={subTask.reasoning} />
                  )}
                </div>
                {subTask.output && (
                  <>
                    <div style={{ font: '600 8.5px/1.2 var(--font-sans)', letterSpacing: '.12em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 8 }}>
                      Output
                    </div>
                    <StructuredOrProse raw={subTask.output} />
                  </>
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function MessageMarkdown({ content }: { content: string }) {
  return (
    // overflowWrap/wordBreak here, not just on the link itself: a raw URL
    // (no natural break points, unlike prose) otherwise overflows its
    // container's width outright — most visible in a narrow viewport,
    // where a long news link pushed the whole reply bubble past its edge.
    <div style={{ fontSize: 14.5, lineHeight: 1.55, overflowWrap: 'anywhere', wordBreak: 'break-word', minWidth: 0 }}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          p: ({ children }) => <p style={{ margin: '0 0 10px' }}>{children}</p>,
          strong: ({ children }) => <strong style={{ fontWeight: 600, color: 'var(--ink)' }}>{children}</strong>,
          ul: ({ children }) => <ul style={{ margin: '0 0 10px', paddingLeft: 20 }}>{children}</ul>,
          ol: ({ children }) => <ol style={{ margin: '0 0 10px', paddingLeft: 20 }}>{children}</ol>,
          li: ({ children }) => <li style={{ marginBottom: 4 }}>{children}</li>,
          a: ({ children, href }) => (
            <a href={href} target="_blank" rel="noopener noreferrer" style={{ color: 'var(--merah)', overflowWrap: 'anywhere' }}>
              {children}
            </a>
          ),
          table: ({ children }) => (
            <div style={{ overflowX: 'auto', margin: '0 0 10px' }}>
              <table style={{ borderCollapse: 'collapse', width: '100%', fontSize: 13 }}>{children}</table>
            </div>
          ),
          thead: ({ children }) => <thead style={{ borderBottom: '1px solid var(--hairline-strong)' }}>{children}</thead>,
          th: ({ children }) => <th style={{ textAlign: 'left', padding: '6px 10px', fontWeight: 600, color: 'var(--ink)' }}>{children}</th>,
          td: ({ children }) => <td style={{ padding: '6px 10px', borderTop: '1px solid var(--hairline)' }}>{children}</td>,
          code: ({ children }) => (
            <code style={{ fontFamily: 'var(--font-mono)', fontSize: 12.5, background: 'var(--canvas-soft)', padding: '1px 4px' }}>{children}</code>
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}

type ChartUIProps = { lens?: string; ticker?: string; lensNote?: string; data?: unknown };

function SingleChartCard({ props }: { props: ChartUIProps }) {
  if (!props.lens) return null;
  return (
    <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--putih)', padding: '13px 15px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', marginBottom: 10, paddingBottom: 9, borderBottom: '1px solid var(--canvas-soft)' }}>
        <span style={{ font: '700 9px/1.3 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase', color: 'var(--merah)', border: '1px solid var(--merah)', padding: '3px 5px', whiteSpace: 'nowrap' }}>
          {props.lens}
        </span>
        {props.lensNote && <span style={{ fontSize: 11.5, color: 'var(--body)', lineHeight: 1.4 }}>{props.lensNote}</span>}
      </div>
      <PortfolioChart payload={{ lens: props.lens, ticker: props.ticker, lensNote: props.lensNote, data: props.data }} />
    </div>
  );
}

// A compound request ("chart-in portofolio ku, dan chart BRPT") makes
// buildWorkflowCard (task_service.go) send back an array of chart payloads
// instead of a single object — one SingleChartCard per item, not just the
// first, so a second chart Analyzer genuinely fetched never silently
// disappears again.
function ChartCard({ uiProps }: { uiProps: unknown }) {
  if (Array.isArray(uiProps)) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        {(uiProps as ChartUIProps[]).map((props, i) => (
          <SingleChartCard key={props.ticker ?? props.lens ?? i} props={props} />
        ))}
      </div>
    );
  }
  return <SingleChartCard props={(uiProps as ChartUIProps | undefined) ?? {}} />;
}

type MessageListProps = {
  chatId: string;
  messages: AgentChatMessage[];
  isLoading: boolean;
  isStreaming: boolean;
  pendingText: string | null;
  failedMessage: { text: string; description: string } | null;
  liveSubTasks: AgentSubTask[];
  toolActivityByAgent: Record<string, string>;
  streamingReplyText: string;
  onRetry: () => void;
};

// Memoized and pulled out of ChatThread on purpose: draft (the textarea's
// own state) used to live in the same component that renders this whole
// list — ReactMarkdown re-parsing every message, plus a PlanCard per
// supervisor reply doing its own data fetching, on every single keystroke,
// as a chat's history grows. Now this only re-renders when its own props
// (real content) actually change, not when the user is just typing.
const MessageList = memo(function MessageList({ chatId, messages, isLoading, isStreaming, pendingText, failedMessage, liveSubTasks, toolActivityByAgent, streamingReplyText, onRetry }: MessageListProps) {
  const threadRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = threadRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [messages, pendingText, liveSubTasks, streamingReplyText, isStreaming]);

  return (
    <div ref={threadRef} className="thread" style={{ flex: 1, overflowY: 'auto', padding: '16px 14px', display: 'flex', flexDirection: 'column', gap: 14 }}>
      {isLoading && <div className="skeleton" style={{ height: 80, width: '100%' }} />}

      {messages.map((message, index) => {
        const isLast = index === messages.length - 1;
        const needsRetry = isLast && message.sender === 'user' && !isStreaming && !pendingText;
        const retryDescription = needsRetry && failedMessage?.text === message.content ? failedMessage.description : 'No reply received for this message yet.';

        return message.sender === 'user' ? (
          <div key={message.id} className="rise" style={{ background: 'var(--merah-soft)', border: `1px solid ${needsRetry ? 'var(--negative)' : 'var(--merah-line)'}`, borderRight: `2px solid ${needsRetry ? 'var(--negative)' : 'var(--merah)'}`, padding: '13px 15px', marginLeft: 'clamp(18px,6vw,34px)' }}>
            <div style={{ fontSize: 14.5, lineHeight: 1.55, color: 'var(--ink-soft)' }}>{message.content}</div>
            {needsRetry && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 9, paddingTop: 9, borderTop: '1px solid var(--merah-line)' }}>
                <span style={{ fontSize: 11.5, color: 'var(--negative)' }}>{retryDescription}</span>
                <button
                  onClick={onRetry}
                  style={{ marginLeft: 'auto', appearance: 'none', cursor: 'pointer', border: '1px solid var(--negative)', background: 'transparent', color: 'var(--negative)', font: '600 11px/1 var(--font-mono)', padding: '6px 10px', flex: 'none' }}
                >
                  Retry
                </button>
              </div>
            )}
          </div>
        ) : (
          <div key={message.id} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {message.ui_ref_task_id != null && <PlanCard taskId={message.ui_ref_task_id} chatId={chatId} />}
            <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--putih)', padding: '13px 15px' }}>
              <MessageMarkdown content={message.content} />
            </div>
            {message.content_type === 'chart' && <ChartCard uiProps={message.ui_props} />}
            {message.content_type === 'news' && <NewsBrief uiProps={message.ui_props} />}
          </div>
        );
      })}

      {pendingText && (
        <div className="rise" style={{ background: 'var(--merah-soft)', border: '1px solid var(--merah-line)', borderRight: '2px solid var(--merah)', padding: '13px 15px', marginLeft: 'clamp(18px,6vw,34px)', opacity: 0.6 }}>
          <div style={{ fontSize: 14.5, lineHeight: 1.55, color: 'var(--ink-soft)' }}>{pendingText}</div>
        </div>
      )}

      {isStreaming && <LiveSubTasks subTasks={liveSubTasks} toolActivityByAgent={toolActivityByAgent} />}

      {isStreaming && streamingReplyText && (
        <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--putih)', padding: '13px 15px' }}>
          <MessageMarkdown content={streamingReplyText} />
        </div>
      )}

      {isStreaming && liveSubTasks.length === 0 && !streamingReplyText && (
        <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--canvas)', padding: '13px 15px', display: 'flex', alignItems: 'center', gap: 10 }}>
          <span className="pulsar" />
          <span style={{ fontSize: 12.5, color: 'var(--body)', fontFamily: 'var(--font-mono)' }}>Quasar is thinking…</span>
        </div>
      )}
    </div>
  );
});

export function ChatThread({ chatId }: ChatThreadProps) {
  const { data: messages = [], isLoading } = useChatMessages(chatId);
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState('');
  const [pendingText, setPendingText] = useState<string | null>(null);
  const [failedMessage, setFailedMessage] = useState<{ text: string; description: string } | null>(null);
  const [liveSubTasks, setLiveSubTasks] = useState<AgentSubTask[]>([]);
  const [toolActivityByAgent, setToolActivityByAgent] = useState<Record<string, string>>({});
  const [streamingReplyText, setStreamingReplyText] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const isSendingRef = useRef(false);

  // Live progress (sub_task/sub_task_started/reply_delta) arrives over the
  // shared WebSocket now, not the HTTP response body — subscribed here,
  // unconditionally, for as long as this chat is open, independent of
  // whether *this* tab is the one that sent the message
  // (docs/plans/agent-orchestration-graph-rebuild.md v2.6: "no SSE, disini
  // pake socket").
  useRealtimeTopic<ChatStreamEvent>(chatStreamTopic(chatId), (event) => {
    if (event.type === 'sub_task_started') {
      setLiveSubTasks((prev) => addStartedPlaceholder(prev, event.data));
      // A fresh step hasn't called any tool yet — drop the previous step's
      // leftover label so it can't briefly show through this step's own
      // placeholder before its first tool_call (if any) arrives.
      setToolActivityByAgent((prev) => {
        if (!(event.data.agent in prev)) return prev;
        const next = { ...prev };
        delete next[event.data.agent];
        return next;
      });
    }
    if (event.type === 'sub_task') {
      setLiveSubTasks((prev) => mergeSubTaskDone(prev, event.data));
    }
    if (event.type === 'tool_call' && event.data.phase === 'start') {
      setToolActivityByAgent((prev) => ({ ...prev, [event.data.agent]: humanizeKey(event.data.tool) }));
    }
    if (event.type === 'reply_delta') {
      setStreamingReplyText((prev) => prev + event.data.delta);
    }
  });

  // Derived at render time, not via a setState-in-effect: once the real
  // persisted message shows up in `messages`, the optimistic bubble below
  // is redundant and hides itself immediately — no extra render tick spent
  // holding a stale copy, and no effect needed to reconcile the two.
  const pendingBubbleText = pendingText && !messages.some((m) => m.sender === 'user' && m.content === pendingText) ? pendingText : null;

  async function handleSend(overrideText?: string) {
    const trimmed = (overrideText ?? draft).trim();
    if (!trimmed || isSendingRef.current) return;
    isSendingRef.current = true;
    setIsStreaming(true);
    setPendingText(trimmed);
    setFailedMessage(null);
    setLiveSubTasks([]);
    setToolActivityByAgent({});
    setStreamingReplyText('');
    if (!overrideText) setDraft('');
    try {
      await sendChatMessage(chatId, trimmed);
      queryClient.invalidateQueries({ queryKey: ['agent-chat-messages', chatId] });
      queryClient.invalidateQueries({ queryKey: ['agent-tasks'] });
    } catch (error: unknown) {
      const description = error instanceof Error ? error.message : 'Something went wrong';
      setFailedMessage({ text: trimmed, description });
      toast.error('Message failed to send', { description });
    } finally {
      isSendingRef.current = false;
      setPendingText(null);
      setLiveSubTasks([]);
      setStreamingReplyText('');
      setIsStreaming(false);
    }
  }

  const handleRetry = useCallback(async () => {
    if (isSendingRef.current) return;
    isSendingRef.current = true;
    setIsStreaming(true);
    setFailedMessage(null);
    setLiveSubTasks([]);
    setToolActivityByAgent({});
    setStreamingReplyText('');
    try {
      await retryLastMessage(chatId);
      queryClient.invalidateQueries({ queryKey: ['agent-chat-messages', chatId] });
      queryClient.invalidateQueries({ queryKey: ['agent-tasks'] });
    } catch (error: unknown) {
      const description = error instanceof Error ? error.message : 'Something went wrong';
      setFailedMessage({ text: messages[messages.length - 1]?.content ?? '', description });
      toast.error('Message failed to send', { description });
    } finally {
      isSendingRef.current = false;
      setLiveSubTasks([]);
      setStreamingReplyText('');
      setIsStreaming(false);
    }
  }, [chatId, messages, queryClient]);

  return (
    <>
      <MessageList
        chatId={chatId}
        messages={messages}
        isLoading={isLoading}
        isStreaming={isStreaming}
        pendingText={pendingBubbleText}
        failedMessage={failedMessage}
        liveSubTasks={liveSubTasks}
        toolActivityByAgent={toolActivityByAgent}
        streamingReplyText={streamingReplyText}
        onRetry={handleRetry}
      />

      <div style={{ flex: 'none', borderTop: '1px solid var(--hairline)', background: 'var(--canvas)', padding: '10px 12px 12px' }}>
        <div style={{ border: '1px solid var(--ink)', background: 'var(--putih)', display: 'flex', alignItems: 'flex-end', gap: 0 }}>
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                handleSend();
              }
            }}
            rows={2}
            placeholder="Ask or instruct Quasar…"
            style={{ flex: 1, border: 0, background: 'transparent', padding: '11px 12px', font: '400 14px/1.5 var(--font-sans)', color: 'var(--ink)', outline: 'none', resize: 'none' }}
          />
          <button
            onClick={() => handleSend()}
            disabled={!draft.trim() || isStreaming || pendingText != null}
            style={{ appearance: 'none', border: 0, cursor: 'pointer', background: 'var(--merah)', color: 'var(--putih)', font: '600 12px/1 var(--font-sans)', padding: '14px 13px', flex: 'none', alignSelf: 'stretch' }}
          >
            Send
          </button>
        </div>
        <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--ticker)', marginTop: 7 }}>only this box is treated as a command</div>
      </div>
    </>
  );
}
