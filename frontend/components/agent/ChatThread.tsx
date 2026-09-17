'use client';

import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { sendChatMessage, retryLastMessage, chatStreamTopic, type AgentChatMessage, type ChatStreamEvent, type LiveSubTask } from '@/http/agent/chatApi';
import { useRealtimeTopic } from '@/http/realtime/useRealtimeSocket';
import { ClarifyingQuestions } from './ClarifyingQuestions';
import { HorizonNoticeCard } from './HorizonNoticeCard';
import { NewsBrief } from './NewsBrief';
import { PlanCard } from './PlanCard';
import { PortfolioChart } from './PortfolioChart';
import { agentDisplayName, humanizeKey, MessageMarkdown, statusColor, stepDisplayName, StructuredOrProse } from './SubTaskReasoning';
import { useChatMessages } from '@/http/agent/hooks';

type ChatThreadProps = {
  chatId: string;
  // Set only when ChatThread is mounted for the very first message of a
  // brand-new chat (QuasarPanel's lazy-mount flow) — the chat row and its
  // first message do not exist in the DB yet, so this is dispatched via the
  // normal handleSend path on mount, rather than requiring the user to
  // retype what they already typed into the pending-chat textarea.
  initialMessage?: string;
};

// Tool names that produce chart-shaped output (matches classifyReply's own
// check, orchestrator_service.go) — used here purely to know, the instant
// the tool_call event fires, that a chart is on its way, so a skeleton can
// appear immediately instead of only once the whole turn finishes and the
// real chart shows up in the persisted message.
const CHART_TOOLS = new Set(['get_portfolio_snapshot', 'get_stock_chart']);

function ChartSkeletonCard() {
  return (
    <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--putih)', padding: '13px 15px' }}>
      <div className="skeleton" style={{ height: 220, width: '100%' }} />
    </div>
  );
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
function LiveSubTasks({
  subTasks,
  toolActivityByAgent,
  thinkingByAgent,
  isFinalizing,
}: {
  subTasks: LiveSubTask[];
  toolActivityByAgent: Record<string, string>;
  thinkingByAgent: Record<string, string>;
  isFinalizing: boolean;
}) {
  const [openRow, setOpenRow] = useState<number | null>(null);
  if (subTasks.length === 0) return null;
  // Every listed Sub Task can already say DONE while the panel's own header
  // still says "Working" with nothing left to point at — confusing, flagged
  // live ("apa yang masih working? gak jelas"). Once nothing is in_progress
  // anymore, the remaining work is either Quasar composing the final reply,
  // or (once that's also done) the turn's on-chain Sub Task batch write —
  // the one real, possibly multi-second stretch that previously had no
  // signal reaching the UI at all ("semua done, tapi no info").
  const anyInProgress = subTasks.some((subTask) => subTask.status === 'in_progress');
  const label = isFinalizing ? 'Finalizing on-chain' : anyInProgress ? 'Working' : 'Composing reply';
  return (
    <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)' }}>
      <div style={{ background: 'var(--ink)', color: 'var(--canvas)', padding: '10px 13px', display: 'flex', alignItems: 'center', gap: 9 }}>
        <span className="pulsar" />
        <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>{label}</span>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--hairline-strong)' }}>{subTasks.length} sub tasks so far</span>
      </div>
      {subTasks.map((subTask, i) => {
        const color = statusColor(subTask.status);
        const isOpen = openRow === i;
        const output = subTask.row?.output;
        return (
          <div key={`${subTask.agent}-${subTask.step_name}-${i}`} className="rise" style={{ borderBottom: '1px solid var(--hairline)' }}>
            <button
              onClick={() => setOpenRow(isOpen ? null : i)}
              style={{ width: '100%', appearance: 'none', border: 0, cursor: 'pointer', background: 'transparent', padding: '11px 13px', textAlign: 'left', display: 'block' }}
            >
              <span style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}>
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--ticker)', flex: 'none' }}>
                  {String(i + 1).padStart(2, '0')}
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
                <div style={{ marginBottom: output ? 14 : 0 }}>
                  {subTask.status === 'failed' ? (
                    // A real failure the backend caught and reported directly
                    // (WrapToolGraceful, see OnSubTaskFailed) — shown as-is,
                    // never relabeled or hidden.
                    <div style={{ fontSize: 12.5, lineHeight: 1.5, color: 'var(--negative)' }}>
                      {subTask.reason || `${agentDisplayName(subTask.agent)} could not complete this step.`}
                    </div>
                  ) : subTask.status === 'in_progress' ? (
                    // Real, live content only — Nova's/Comet's own streamed
                    // reasoning (thinking events) and the tool it's currently
                    // calling, both sourced from actual events, never a
                    // stand-in sentence invented for the gap before the first
                    // one arrives. Deliberately blank in that brief gap rather
                    // than a fake "sedang memproses…" placeholder.
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                      {toolActivityByAgent[subTask.agent] && (
                        <span style={{ font: '600 9px/1.3 var(--font-mono)', letterSpacing: '.08em', textTransform: 'uppercase', color: 'var(--ticker)' }}>
                          {toolActivityByAgent[subTask.agent]}
                        </span>
                      )}
                      {thinkingByAgent[subTask.agent] && (
                        <div style={{ color: 'var(--ink-soft)' }}>
                          <MessageMarkdown content={thinkingByAgent[subTask.agent]} fontSize={12.5} />
                        </div>
                      )}
                    </div>
                  ) : (
                    <StructuredOrProse raw={subTask.row?.reasoning ?? ''} />
                  )}
                </div>
                {output && (
                  <>
                    <div style={{ font: '600 8.5px/1.2 var(--font-sans)', letterSpacing: '.12em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 8 }}>
                      Output
                    </div>
                    <StructuredOrProse raw={output} />
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
          <SingleChartCard key={`${props.ticker ?? props.lens ?? 'chart'}-${i}`} props={props} />
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
  liveSubTasks: LiveSubTask[];
  toolActivityByAgent: Record<string, string>;
  thinkingByAgent: Record<string, string>;
  isFinalizing: boolean;
  chartPending: boolean;
  streamingReplyText: string;
  onRetry: () => void;
  onSendPrompt?: (text: string, hidden?: boolean) => void | Promise<void>;
};

// A hidden intake-answer message carries { hidden: true } in ui_props
// (content_type is a strict DB enum with no room for a "hidden" variant —
// see docs/plans/fix-hide-intake-answer-message.md).
function isHiddenMessage(message: AgentChatMessage): boolean {
  return Boolean((message.ui_props as { hidden?: boolean } | null | undefined)?.hidden);
}

// Parses a compiled "key: value" answer message's own content back into the
// same shape ClarifyingQuestions built it from — the format is fixed and
// self-authored, so this is reading back known data, not inferring anything.
function parseIntakeAnswer(content: string): Record<string, string> {
  const answers: Record<string, string> = {};
  for (const line of content.split('\n')) {
    const separatorIndex = line.indexOf(': ');
    if (separatorIndex === -1) continue;
    answers[line.slice(0, separatorIndex)] = line.slice(separatorIndex + 2);
  }
  return answers;
}

// Memoized and pulled out of ChatThread on purpose: draft (the textarea's
// own state) used to live in the same component that renders this whole
// list — ReactMarkdown re-parsing every message, plus a PlanCard per
// supervisor reply doing its own data fetching, on every single keystroke,
// as a chat's history grows. Now this only re-renders when its own props
// (real content) actually change, not when the user is just typing.
const MessageList = memo(function MessageList({ chatId, messages, isLoading, isStreaming, pendingText, failedMessage, liveSubTasks, toolActivityByAgent, thinkingByAgent, isFinalizing, chartPending, streamingReplyText, onRetry, onSendPrompt }: MessageListProps) {
  const threadRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = threadRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [messages, pendingText, liveSubTasks, streamingReplyText, isStreaming, isFinalizing, chartPending]);

  // A Task can be referenced by several messages across a conversation (an
  // intake reply while still needs_input, then a later "ready to arm"
  // reply once actionable, etc). PlanCard always reads the Task's own
  // *current* live state, not this message's own snapshot — rendering it
  // once per referencing message duplicated the same on-chain step list
  // (or an ArmPanel the task has since moved past) once per message. Only
  // the *last* message referencing a given task renders it
  // (docs/plans/fix-plancard-clarifying-questions-duplication.md v1.2 —
  // corrects the earlier per-ui_component exclusion, which was too broad
  // and hid the legitimate Sub Task list during an active intake).
  const lastPlanCardIndexByTaskId = useMemo(() => {
    const map = new Map<number, number>();
    messages.forEach((message, index) => {
      if (message.ui_ref_task_id == null) return;
      if (message.ui_component === 'HorizonNoticeCard' || message.content_type === 'horizon_notice') return;
      map.set(message.ui_ref_task_id, index);
    });
    return map;
  }, [messages]);

  return (
    <div ref={threadRef} className="thread" style={{ flex: 1, overflowY: 'auto', padding: '16px 14px', display: 'flex', flexDirection: 'column', gap: 14 }}>
      {isLoading && <div className="skeleton" style={{ height: 80, width: '100%' }} />}

      {messages.map((message, index) => {
        // Real chat history, fed to the LLM as context, but never rendered
        // — the compiled answer a clarifying-questions card sent, not
        // something the user typed by hand.
        if (isHiddenMessage(message)) return null;

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
            {message.content && (
              <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--putih)', padding: '13px 15px' }}>
                <MessageMarkdown content={message.content} />
              </div>
            )}
            {message.ui_ref_task_id != null && message.ui_component !== 'HorizonNoticeCard' && message.content_type !== 'horizon_notice' && lastPlanCardIndexByTaskId.get(message.ui_ref_task_id) === index && <PlanCard taskId={message.ui_ref_task_id} chatId={chatId} />}
            {message.content_type === 'chart' && <ChartCard uiProps={message.ui_props} />}
            {message.content_type === 'news' && <NewsBrief uiProps={message.ui_props} />}
            {message.ui_component === 'clarifying_questions' && (
              <ClarifyingQuestions
                uiProps={message.ui_props}
                chatId={chatId}
                onSendPrompt={onSendPrompt}
                initialAnswers={
                  messages[index + 1] != null && isHiddenMessage(messages[index + 1])
                    ? parseIntakeAnswer(messages[index + 1].content)
                    : undefined
                }
              />
            )}
            {(message.ui_component === 'HorizonNoticeCard' || message.content_type === 'horizon_notice') && (
              <HorizonNoticeCard
                uiProps={message.ui_props}
                taskId={message.ui_ref_task_id ?? undefined}
                chatId={chatId}
                onSendPrompt={onSendPrompt}
              />
            )}
          </div>
        );
      })}

      {pendingText && (
        <div className="rise" style={{ background: 'var(--merah-soft)', border: '1px solid var(--merah-line)', borderRight: '2px solid var(--merah)', padding: '13px 15px', marginLeft: 'clamp(18px,6vw,34px)', opacity: 0.6 }}>
          <div style={{ fontSize: 14.5, lineHeight: 1.55, color: 'var(--ink-soft)' }}>{pendingText}</div>
        </div>
      )}

      {isStreaming && <LiveSubTasks subTasks={liveSubTasks} toolActivityByAgent={toolActivityByAgent} thinkingByAgent={thinkingByAgent} isFinalizing={isFinalizing} />}

      {isStreaming && chartPending && <ChartSkeletonCard />}

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

export function ChatThread({ chatId, initialMessage }: ChatThreadProps) {
  // A freshly-promoted chat (initialMessage set) has no row in the database
  // yet — GET /agent/chats/:id/messages would 404/return empty and briefly
  // flash the loading skeleton for nothing. Disabled until the first send
  // settles (docs/plans/fix-new-chat-first-message-loading-flash.md); an
  // existing chat (no initialMessage) is unaffected, enabled from mount.
  const [messagesQueryEnabled, setMessagesQueryEnabled] = useState(!initialMessage);
  const { data: messages = [], isLoading } = useChatMessages(chatId, { enabled: messagesQueryEnabled });
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState('');
  const [pendingText, setPendingText] = useState<string | null>(null);
  const [failedMessage, setFailedMessage] = useState<{ text: string; description: string } | null>(null);
  const [liveSubTasks, setLiveSubTasks] = useState<LiveSubTask[]>([]);
  const [toolActivityByAgent, setToolActivityByAgent] = useState<Record<string, string>>({});
  const [thinkingByAgent, setThinkingByAgent] = useState<Record<string, string>>({});
  const [isFinalizing, setIsFinalizing] = useState(false);
  const [chartPending, setChartPending] = useState(false);
  const [streamingReplyText, setStreamingReplyText] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const isSendingRef = useRef(false);

  // Live progress (sub_tasks/reply_delta) arrives over the shared WebSocket
  // now, not the HTTP response body — subscribed here, unconditionally, for
  // as long as this chat is open, independent of whether *this* tab is the
  // one that sent the message (docs/plans/agent-orchestration-graph-rebuild.md
  // v2.6: "no SSE, disini pake socket"). sub_tasks always carries the
  // turn's full, current list (backend/src/http/handlers/agent/chats.go's
  // liveSubTasks) — this is a plain state replace, no matching/merging of
  // any kind (docs/plans/fix-sub-task-attempt-lifecycle-events.md).
  useRealtimeTopic<ChatStreamEvent>(chatStreamTopic(chatId), (event) => {
    if (event.type === 'sub_tasks') {
      setLiveSubTasks(event.data);
    }
    if (event.type === 'tool_call' && event.data.phase === 'start') {
      setToolActivityByAgent((prev) => ({ ...prev, [event.data.agent]: humanizeKey(event.data.tool) }));
      if (CHART_TOOLS.has(event.data.tool)) setChartPending(true);
    }
    if (event.type === 'thinking') {
      setThinkingByAgent((prev) => ({ ...prev, [event.data.agent]: (prev[event.data.agent] ?? '') + event.data.delta }));
    }
    if (event.type === 'finalizing') {
      setIsFinalizing(true);
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

  async function handleSend(overrideText?: string, hidden?: boolean) {
    const trimmed = (overrideText ?? draft).trim();
    if (!trimmed || isSendingRef.current) return;
    isSendingRef.current = true;
    setIsStreaming(true);
    // A hidden send (a clarifying-questions card's compiled answer) never
    // shows the optimistic bubble either — it must never appear in the UI
    // at all, live or persisted.
    if (!hidden) setPendingText(trimmed);
    setFailedMessage(null);
    setLiveSubTasks([]);
    setToolActivityByAgent({});
    setThinkingByAgent({});
    setIsFinalizing(false);
    setChartPending(false);
    setStreamingReplyText('');
    if (!overrideText) setDraft('');
    try {
      await sendChatMessage(chatId, trimmed, hidden);
      // The chat row is guaranteed to exist now (FindOrCreate ran inside the
      // send above) — safe to enable the messages query from here on, a
      // no-op if it was already enabled.
      setMessagesQueryEnabled(true);
      // Awaited on purpose: the streaming bubble below is cleared in
      // `finally` right after this. If the persisted message list hasn't
      // actually refetched yet by then, there's a gap where neither the
      // streaming bubble nor the real message is on screen — the reply
      // visibly disappears for a beat before popping back in once the
      // refetch lands. Awaiting here makes the swap atomic instead.
      await queryClient.invalidateQueries({ queryKey: ['agent-chat-messages', chatId] });
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

  // Fires exactly once, on mount, only for a freshly-promoted new chat that
  // was typed into QuasarPanel's pending-chat textarea before ChatThread
  // (and its WebSocket subscription) existed to send it directly.
  const hasSentInitialMessageRef = useRef(false);
  useEffect(() => {
    if (initialMessage && !hasSentInitialMessageRef.current) {
      hasSentInitialMessageRef.current = true;
      handleSend(initialMessage);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleRetry = useCallback(async () => {
    if (isSendingRef.current) return;
    isSendingRef.current = true;
    setIsStreaming(true);
    setFailedMessage(null);
    setLiveSubTasks([]);
    setToolActivityByAgent({});
    setThinkingByAgent({});
    setIsFinalizing(false);
    setChartPending(false);
    setStreamingReplyText('');
    try {
      await retryLastMessage(chatId);
      // Same reasoning as handleSend: await so the message list already has
      // the real reply before the streaming bubble is cleared below.
      await queryClient.invalidateQueries({ queryKey: ['agent-chat-messages', chatId] });
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
        thinkingByAgent={thinkingByAgent}
        isFinalizing={isFinalizing}
        chartPending={chartPending}
        streamingReplyText={streamingReplyText}
        onRetry={handleRetry}
        onSendPrompt={handleSend}
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
