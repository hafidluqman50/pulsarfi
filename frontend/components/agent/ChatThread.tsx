'use client';

import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { sendChatMessage, retryLastMessage, getTurnStatus, type AgentChatMessage, type LiveSubTask } from '@/http/agent/chatApi';
import { ClarifyingQuestions } from './ClarifyingQuestions';
import { HorizonNoticeCard } from './HorizonNoticeCard';
import { NewsBrief } from './NewsBrief';
import { PlanCard } from './PlanCard';
import { PortfolioChart } from './PortfolioChart';
import { RosterCard } from './RosterCard';
import { agentDisplayName, MessageMarkdown, statusColor, stepDisplayName, StructuredOrProse } from './SubTaskReasoning';
import { beginTurn, clearTurnLive, failTurn, getChatSession, markTurnEnded, markTurnStarted, useChatSession } from './chatSession';
import { useAgentChats, useChatMessages } from '@/http/agent/hooks';

// The thinking indicator is a fixed English UI-chrome label — global for every
// user, regardless of the conversation's language — plus a remembered
// show/hide control for the reasoning trace behind it
// (docs/plans/quasar-thinking-label-and-reasoning-toggle.md; AGENTS.md §3.5).
const QUASAR_THINKING_LABEL = 'Quasar is thinking';
const REASONING_VISIBILITY_KEY = 'pulsar.agent.showReasoning';

function useReasoningVisibility(): [boolean, (value: boolean) => void] {
  const [show, setShow] = useState(() =>
    typeof window === 'undefined' ? true : window.localStorage.getItem(REASONING_VISIBILITY_KEY) !== 'false');
  const update = useCallback((value: boolean) => {
    setShow(value);
    try {
      window.localStorage.setItem(REASONING_VISIBILITY_KEY, value ? 'true' : 'false');
    } catch {
      /* localStorage unavailable (private mode); the toggle still works for this session */
    }
  }, []);
  return [show, update];
}

type ChatThreadProps = {
  // activeChatId is the single source of truth for "which chat is open" (see
  // QuasarPanel) — a brand-new chat is just a fresh client UUID whose row does
  // not exist yet, so it reads as an empty message list until the first send.
  // There is no message hand-off prop: the thread renders its own intro while
  // it has no messages.
  chatId: string;
};

function ChartSkeletonCard() {
  return (
    <div className="rise" style={{ border: '1px solid var(--hairline)', background: 'var(--putih)', padding: '13px 15px', boxShadow: '0 5px 0 -2px #fbfaf7, 0 6px 0 -2px #e3ddd2, 0 18px 22px -14px rgba(22,17,14,.25)' }}>
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
  showReasoning,
  onToggleReasoning,
}: {
  subTasks: LiveSubTask[];
  toolActivityByAgent: Record<string, string>;
  thinkingByAgent: Record<string, string>;
  isFinalizing: boolean;
  showReasoning: boolean;
  onToggleReasoning: () => void;
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
  const label = isFinalizing ? 'Finalizing on-chain' : anyInProgress ? QUASAR_THINKING_LABEL : 'Composing reply';
  return (
    <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)', boxShadow: '0 5px 0 -2px #fbfaf7, 0 6px 0 -2px #e3ddd2, 0 11px 0 -4px #fbfaf7, 0 12px 0 -4px #e3ddd2, 0 20px 24px -14px rgba(22,17,14,.3)' }}>
      <div style={{ background: 'var(--ink)', color: 'var(--canvas)', padding: '10px 13px', display: 'flex', alignItems: 'center', gap: 9 }}>
        <span className="pulsar" />
        <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>{label}</span>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--hairline-strong)' }}>{subTasks.length} sub tasks so far</span>
        <button
          onClick={onToggleReasoning}
          style={{ marginLeft: 'auto', appearance: 'none', border: '1px solid var(--ink-line)', cursor: 'pointer', background: 'transparent', color: 'var(--canvas)', font: '500 10px/1 var(--font-mono)', letterSpacing: '.06em', padding: '5px 7px', flex: 'none', whiteSpace: 'nowrap' }}
        >
          {showReasoning ? 'hide thinking' : 'show thinking'}
        </button>
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
              <div style={{ margin: '10px 13px 12px 22px', background: 'var(--canvas-soft)', border: '1px solid var(--hairline)', padding: '10px 12px', transform: 'rotate(-.25deg)' }}>
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
                    // one arrives. The reasoning markdown itself respects the
                    // user's show/hide preference.
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                      {toolActivityByAgent[subTask.agent] && (
                        <span style={{ font: '600 9px/1.3 var(--font-mono)', letterSpacing: '.08em', textTransform: 'uppercase', color: 'var(--ticker)' }}>
                          {toolActivityByAgent[subTask.agent]}
                        </span>
                      )}
                      {showReasoning && thinkingByAgent[subTask.agent] && (
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
    <div className="rise" style={{ border: '1px solid var(--hairline)', background: 'var(--putih)', padding: '13px 15px', boxShadow: '0 5px 0 -2px #fbfaf7, 0 6px 0 -2px #e3ddd2, 0 18px 22px -14px rgba(22,17,14,.25)' }}>
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
  showReasoning: boolean;
  chartPending: boolean;
  streamingReplyText: string;
  onRetry: () => void;
  onToggleReasoning: () => void;
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

// The thread's own empty state — a brand-new chat (a fresh client UUID with no
// row yet) reads as an empty message list, and this is what it shows instead
// of the old panel-owned "welcome" view.
function EmptyThreadIntro() {
  return (
    <div className="rise" style={{ maxWidth: '92%' }}>
      <div style={{ font: '600 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase', color: 'var(--merah)', marginBottom: 5 }}>Quasar</div>
      <div style={{ fontFamily: 'var(--font-display)', fontSize: 15, lineHeight: 1.5, color: 'var(--ink-soft)', border: '1px solid var(--hairline)', borderLeft: '3px solid var(--merah)', background: 'var(--putih)', padding: '13px 15px', boxShadow: '0 5px 0 -2px #fbfaf7, 0 6px 0 -2px #e3ddd2, 0 18px 22px -14px rgba(22,17,14,.25)' }}>
        I am Quasar. Write the instruction in your own words — a fast trade or a long mandate, I read the horizon out of what you wrote. Standing
        instructions I turn into a rule with named sources, a number and hard caps, and show you every step before anything is armed.
      </div>
    </div>
  );
}

// Memoized and pulled out of ChatThread on purpose: draft (the textarea's
// own state) used to live in the same component that renders this whole
// list — ReactMarkdown re-parsing every message, plus a PlanCard per
// supervisor reply doing its own data fetching, on every single keystroke,
// as a chat's history grows. Now this only re-renders when its own props
// (real content) actually change, not when the user is just typing.
const MessageList = memo(function MessageList({ chatId, messages, isLoading, isStreaming, pendingText, failedMessage, liveSubTasks, toolActivityByAgent, thinkingByAgent, isFinalizing, showReasoning, chartPending, streamingReplyText, onRetry, onToggleReasoning, onSendPrompt }: MessageListProps) {
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

  const showEmptyIntro = messages.length === 0 && !isLoading && !isStreaming && !pendingText;

  return (
    <div ref={threadRef} className="thread" style={{ flex: 1, overflowY: 'auto', padding: '16px 14px', display: 'flex', flexDirection: 'column', gap: 14 }}>
      {isLoading && <div className="skeleton" style={{ height: 80, width: '100%' }} />}

      {showEmptyIntro && (
        <>
          <EmptyThreadIntro />
          <RosterCard />
        </>
      )}

      {messages.map((message, index) => {
        // Real chat history, fed to the LLM as context, but never rendered
        // — the compiled answer a clarifying-questions card sent, not
        // something the user typed by hand.
        if (isHiddenMessage(message)) return null;

        const isLast = index === messages.length - 1;
        const needsRetry = isLast && message.sender === 'user' && !isStreaming && !pendingText;
        const retryDescription = needsRetry && failedMessage?.text === message.content ? failedMessage.description : 'No reply received for this message yet.';

        return message.sender === 'user' ? (
          <div key={message.id} className="rise" style={{ alignSelf: 'flex-end', maxWidth: '84%', background: 'var(--ink)', color: 'var(--canvas)', padding: '10px 12px', boxShadow: '0 4px 0 -2px #fbfaf7, 0 5px 0 -2px #4a423d', outline: needsRetry ? '1px solid var(--negative)' : 'none', outlineOffset: 2 }}>
            <div style={{ fontSize: 13.5, lineHeight: 1.5, overflowWrap: 'anywhere' }}>{message.content}</div>
            {needsRetry && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 9, paddingTop: 9, borderTop: '1px solid var(--ink-line)' }}>
                <span style={{ fontSize: 11.5, color: '#ff8a9a' }}>{retryDescription}</span>
                <button
                  onClick={onRetry}
                  style={{ marginLeft: 'auto', appearance: 'none', cursor: 'pointer', border: '1px solid #ff8a9a', background: 'transparent', color: '#ff8a9a', font: '600 11px/1 var(--font-mono)', padding: '6px 10px', flex: 'none' }}
                >
                  Retry
                </button>
              </div>
            )}
          </div>
        ) : (
          <div key={message.id} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {message.content && (
              <div className="rise" style={{ maxWidth: '92%' }}>
                <div style={{ font: '600 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase', color: 'var(--merah)', marginBottom: 5 }}>Quasar</div>
                <div style={{ fontFamily: 'var(--font-display)', color: 'var(--ink-soft)', border: '1px solid var(--hairline)', borderLeft: '3px solid var(--merah)', background: 'var(--putih)', padding: '13px 15px', boxShadow: '0 5px 0 -2px #fbfaf7, 0 6px 0 -2px #e3ddd2, 0 18px 22px -14px rgba(22,17,14,.25)' }}>
                  <MessageMarkdown content={message.content} fontSize={15} />
                </div>
              </div>
            )}
            {message.ui_ref_task_id != null && message.ui_component !== 'HorizonNoticeCard' && message.content_type !== 'horizon_notice' && lastPlanCardIndexByTaskId.get(message.ui_ref_task_id) === index && <PlanCard taskId={message.ui_ref_task_id} chatId={chatId} />}
            {message.content_type === 'chart' && (
              <>
                <ChartCard uiProps={(message.ui_props as { charts?: unknown })?.charts ?? message.ui_props} />
                {Array.isArray((message.ui_props as { news?: unknown })?.news) && (
                  <NewsBrief uiProps={(message.ui_props as { news?: unknown })?.news} />
                )}
              </>
            )}
            {message.content_type === 'news' && (
              <>
                {(message.ui_props as { charts?: unknown })?.charts != null && (
                  <ChartCard uiProps={(message.ui_props as { charts?: unknown })?.charts} />
                )}
                <NewsBrief uiProps={Array.isArray(message.ui_props) ? message.ui_props : (message.ui_props as { news?: unknown })?.news} />
              </>
            )}
            {message.content_type === 'composite' && (
              <>
                <ChartCard uiProps={(message.ui_props as { charts?: unknown })?.charts ?? message.ui_props} />
                <NewsBrief uiProps={(message.ui_props as { news?: unknown })?.news} />
              </>
            )}
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
        <div className="rise" style={{ alignSelf: 'flex-end', maxWidth: '84%', background: 'var(--ink)', color: 'var(--canvas)', padding: '10px 12px', boxShadow: '0 4px 0 -2px #fbfaf7, 0 5px 0 -2px #4a423d', opacity: 0.6 }}>
          <div style={{ fontSize: 13.5, lineHeight: 1.5, overflowWrap: 'anywhere' }}>{pendingText}</div>
        </div>
      )}

      {isStreaming && <LiveSubTasks subTasks={liveSubTasks} toolActivityByAgent={toolActivityByAgent} thinkingByAgent={thinkingByAgent} isFinalizing={isFinalizing} showReasoning={showReasoning} onToggleReasoning={onToggleReasoning} />}

      {isStreaming && chartPending && <ChartSkeletonCard />}

      {streamingReplyText && (
        <div className="rise" style={{ maxWidth: '92%' }}>
          <div style={{ font: '600 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase', color: 'var(--merah)', marginBottom: 5 }}>Quasar</div>
          <div style={{ fontFamily: 'var(--font-display)', color: 'var(--ink-soft)', border: '1px solid var(--hairline)', borderLeft: '3px solid var(--merah)', background: 'var(--putih)', padding: '13px 15px', boxShadow: '0 5px 0 -2px #fbfaf7, 0 6px 0 -2px #e3ddd2, 0 18px 22px -14px rgba(22,17,14,.25)' }}>
            <MessageMarkdown content={streamingReplyText} fontSize={15} />
          </div>
        </div>
      )}

      {isStreaming && liveSubTasks.length === 0 && !streamingReplyText && (
        <div className="rise" style={{ border: '1px solid var(--hairline)', background: 'var(--putih)', padding: '13px 15px', display: 'flex', alignItems: 'center', gap: 10, boxShadow: '0 5px 0 -2px #fbfaf7, 0 6px 0 -2px #e3ddd2, 0 18px 22px -14px rgba(22,17,14,.25)' }}>
          <span className="pulsar" />
          <span style={{ fontSize: 12.5, color: 'var(--body)', fontFamily: 'var(--font-mono)' }}>{QUASAR_THINKING_LABEL}</span>
        </div>
      )}
    </div>
  );
});

export function ChatThread({ chatId }: ChatThreadProps) {
  // A new chat is a client UUID with no row yet, so its history is fetched
  // only once the chat exists (shared agent-chats cache). Everything the user
  // sends is appended to this cache from the response; the DB is the source
  // only when opening an existing chat or after a reload.
  const { data: chats = [] } = useAgentChats();
  const chatExists = chats.some((chat) => chat.id === chatId);
  const { data: messages = [], isLoading } = useChatMessages(chatId, { enabled: chatExists });
  const queryClient = useQueryClient();
  const session = useChatSession(chatId);
  const [draft, setDraft] = useState('');
  const [pendingText, setPendingText] = useState<string | null>(null);
  const [showReasoning, setShowReasoning] = useReasoningVisibility();
  const isSendingRef = useRef(false);

  // A client that mounts mid-turn (reload, route return, second tab) cannot
  // know locally that a turn is running: ChatThread's own state was just
  // created. The server is the source of truth. Without this, the UI would
  // offer Retry for a turn that is still running and start a second one.
  useEffect(() => {
    let cancelled = false;
    getTurnStatus(chatId)
      .then((status) => {
        if (cancelled) return;
        if (status.running) {
          markTurnStarted(chatId);
        } else if (getChatSession(chatId).running) {
          markTurnEnded(chatId);
        }
      })
      .catch(() => {
        /* status is best effort; the 409 guard is the backstop */
      });
    return () => {
      cancelled = true;
    };
  }, [chatId]);

  // The reply is already in the messages cache (appended from the response),
  // so ending a turn only drops the live chrome and refreshes task state.
  // There is no message refetch here: the DB is consulted on mount, not after
  // every turn.
  const endedAt = session.endedAt;
  useEffect(() => {
    if (!endedAt) return;
    clearTurnLive(chatId);
    void queryClient.invalidateQueries({ queryKey: ['agent-tasks'] });
  }, [endedAt, chatId, queryClient]);

  // Derived at render time, not via a setState-in-effect: once the row
  // appended from the response is in `messages`, the optimistic bubble below
  // is redundant and hides itself immediately.
  const pendingBubbleText = pendingText && !messages.some((m) => m.sender === 'user' && m.content === pendingText) ? pendingText : null;

  // The live bubble and the appended reply are the same text. Hide the live
  // copy as soon as `messages` contains it: that covers the frame between the
  // append and clearTurnLive, and any later fetch that brings the same row.
  const liveReplyText = session.streamingReplyText;
  const liveReplyPersisted =
    liveReplyText !== '' &&
    messages.some((message) => message.sender !== 'user' && !isHiddenMessage(message) && message.content === liveReplyText);

  async function handleSend(overrideText?: string, hidden?: boolean) {
    const trimmed = (overrideText ?? draft).trim();
    if (!trimmed || isSendingRef.current) return;
    isSendingRef.current = true;
    beginTurn(chatId);
    // A hidden send (a clarifying-questions card's compiled answer) never
    // shows the optimistic bubble either — it must never appear in the UI at
    // all, live or persisted.
    if (!hidden) setPendingText(trimmed);
    if (!overrideText) setDraft('');
    try {
      const card = await sendChatMessage(chatId, trimmed, hidden);
      // The response is the chat's state: replace, never refetch.
      if (card.messages) queryClient.setQueryData(['agent-chat-messages', chatId], card.messages);
      if (!chatExists) {
        // The first message created the chat server-side; let the sidebar see
        // it. No messages refetch: the rows above are already on screen.
        void queryClient.invalidateQueries({ queryKey: ['agent-chats'] });
      }
      markTurnEnded(chatId);
    } catch (error: unknown) {
      const description = error instanceof Error ? error.message : 'Something went wrong';
      // Error recovery is the one place a refetch is allowed: pull the
      // persisted user row (and the new chat) so Retry has something to act on.
      if (!chatExists) void queryClient.invalidateQueries({ queryKey: ['agent-chats'] });
      void queryClient.invalidateQueries({ queryKey: ['agent-chat-messages', chatId] });
      failTurn(chatId, trimmed, description);
      toast.error('Message failed to send', { description });
    } finally {
      isSendingRef.current = false;
      setPendingText(null);
    }
  }

  const handleRetry = useCallback(async () => {
    if (isSendingRef.current) return;
    isSendingRef.current = true;
    beginTurn(chatId);
    try {
      const card = await retryLastMessage(chatId);
      if (card.messages) queryClient.setQueryData(['agent-chat-messages', chatId], card.messages);
      markTurnEnded(chatId);
    } catch (error: unknown) {
      const description = error instanceof Error ? error.message : 'Something went wrong';
      failTurn(chatId, messages[messages.length - 1]?.content ?? '', description);
      toast.error('Message failed to send', { description });
    } finally {
      isSendingRef.current = false;
    }
  }, [chatId, queryClient, messages]);

  const isBusy = session.running || pendingText != null;

  return (
    <>
      <MessageList
        chatId={chatId}
        messages={messages}
        isLoading={isLoading}
        isStreaming={session.running}
        pendingText={pendingBubbleText}
        failedMessage={session.failedMessage}
        liveSubTasks={session.liveSubTasks}
        toolActivityByAgent={session.toolActivityByAgent}
        thinkingByAgent={session.thinkingByAgent}
        isFinalizing={session.isFinalizing}
        showReasoning={showReasoning}
        chartPending={session.chartPending}
        streamingReplyText={liveReplyPersisted ? '' : liveReplyText}
        onRetry={handleRetry}
        onToggleReasoning={() => setShowReasoning(!showReasoning)}
        onSendPrompt={handleSend}
      />

      <div style={{ flex: 'none', borderTop: '1px solid var(--ink)', background: 'var(--putih)', padding: 10 }}>
        <div style={{ display: 'flex', alignItems: 'stretch', gap: 8 }}>
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
            style={{ flex: 1, border: '1px solid var(--hairline-strong)', background: 'var(--canvas)', padding: '10px 12px', font: '400 13.5px/1.5 var(--font-sans)', color: 'var(--ink)', outline: 'none', resize: 'none' }}
          />
          <button
            onClick={() => handleSend()}
            disabled={!draft.trim() || isBusy}
            style={{ appearance: 'none', border: 0, cursor: !draft.trim() || isBusy ? 'not-allowed' : 'pointer', background: 'var(--ink)', color: 'var(--putih)', font: '600 12px/1 var(--font-sans)', letterSpacing: '.06em', padding: '0 16px', flex: 'none', opacity: !draft.trim() || isBusy ? 0.5 : 1 }}
          >
            Send
          </button>
        </div>
        <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--ticker)', marginTop: 7 }}>only this box is treated as a command</div>
      </div>
    </>
  );
}
