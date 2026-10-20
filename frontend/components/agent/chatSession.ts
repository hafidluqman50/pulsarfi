'use client';

import { useCallback, useSyncExternalStore } from 'react';
import { chatStreamTopic, type ChatStreamEvent, type LiveSubTask } from '@/http/agent/chatApi';
import { realtimeSocket } from '@/http/realtime/useRealtimeSocket';
import { humanizeKey } from './SubTaskReasoning';

// Tool names that produce chart-shaped output (matches classifyReply's own
// check on the backend) — used only to raise the chart skeleton the instant a
// tool_call event says one is coming.
const CHART_TOOLS = new Set(['get_portfolio_snapshot', 'get_stock_chart']);

// ChatSession is everything live about one chat turn. It deliberately lives in
// a module-level store, not in ChatThread: closing the panel, navigating to
// another route, or any other remount must not throw away a turn that is still
// running on the server (docs/plans/agent-chat-turn-lifecycle.md).
export type ChatSession = {
  running: boolean;
  liveSubTasks: LiveSubTask[];
  toolActivityByAgent: Record<string, string>;
  thinkingByAgent: Record<string, string>;
  streamingReplyText: string;
  isFinalizing: boolean;
  chartPending: boolean;
  failedMessage: { text: string; description: string } | null;
  // Bumped whenever a turn ends, so a mounted view can refetch the persisted
  // messages before it drops the live bubbles (no blink).
  endedAt: number;
};

const EMPTY_SESSION: ChatSession = {
  running: false,
  liveSubTasks: [],
  toolActivityByAgent: {},
  thinkingByAgent: {},
  streamingReplyText: '',
  isFinalizing: false,
  chartPending: false,
  failedMessage: null,
  endedAt: 0,
};

const sessions = new Map<string, ChatSession>();
const listeners = new Map<string, Set<() => void>>();
const topicUnsubscribers = new Map<string, () => void>();

export function getChatSession(chatId: string): ChatSession {
  return sessions.get(chatId) ?? EMPTY_SESSION;
}

export function setChatSession(chatId: string, patch: Partial<ChatSession>): void {
  sessions.set(chatId, { ...getChatSession(chatId), ...patch });
  listeners.get(chatId)?.forEach((listener) => listener());
}

function ensureSubscribed(chatId: string): void {
  if (topicUnsubscribers.has(chatId)) return;
  realtimeSocket.connect();
  const unsubscribe = realtimeSocket.subscribe(chatStreamTopic(chatId), (data) => {
    handleEvent(chatId, data as ChatStreamEvent);
  });
  topicUnsubscribers.set(chatId, unsubscribe);
}

// The stream stays attached while a view is mounted OR a turn is running, so
// minimising the panel / leaving the route does not lose the live progress.
function maybeUnsubscribe(chatId: string): void {
  if ((listeners.get(chatId)?.size ?? 0) > 0) return;
  if (getChatSession(chatId).running) return;
  topicUnsubscribers.get(chatId)?.();
  topicUnsubscribers.delete(chatId);
}

function appendRecord(
  chatId: string,
  key: 'thinkingByAgent' | 'toolActivityByAgent',
  field: string,
  value: string,
  mode: 'append' | 'replace',
): void {
  const current = getChatSession(chatId)[key];
  const next = { ...current, [field]: mode === 'append' ? (current[field] ?? '') + value : value };
  setChatSession(chatId, key === 'thinkingByAgent' ? { thinkingByAgent: next } : { toolActivityByAgent: next });
}

function handleEvent(chatId: string, event: ChatStreamEvent): void {
  switch (event.type) {
    case 'sub_tasks':
      setChatSession(chatId, { liveSubTasks: event.data });
      break;
    case 'tool_call':
      if (event.data.phase === 'start') {
        appendRecord(chatId, 'toolActivityByAgent', event.data.agent, humanizeKey(event.data.tool), 'replace');
        if (CHART_TOOLS.has(event.data.tool)) setChatSession(chatId, { chartPending: true });
      }
      break;
    case 'thinking':
      appendRecord(chatId, 'thinkingByAgent', event.data.agent, event.data.delta, 'append');
      break;
    case 'reply_delta':
      setChatSession(chatId, { streamingReplyText: getChatSession(chatId).streamingReplyText + event.data.delta });
      break;
    case 'finalizing':
      setChatSession(chatId, { isFinalizing: true });
      break;
    case 'turn':
      if (event.data.status === 'running') markTurnStarted(chatId);
      else markTurnEnded(chatId);
      break;
    case 'final':
    case 'error':
      markTurnEnded(chatId);
      break;
    case 'status':
      // Reserved event; the thinking label is a fixed English constant now.
      break;
  }
}

export function subscribeChatSession(chatId: string, listener: () => void): () => void {
  let set = listeners.get(chatId);
  if (!set) {
    set = new Set();
    listeners.set(chatId, set);
  }
  set.add(listener);
  ensureSubscribed(chatId);

  return () => {
    set!.delete(listener);
    if (set!.size === 0) listeners.delete(chatId);
    maybeUnsubscribe(chatId);
  };
}

// beginTurn is the user-initiated start: it clears the previous turn's live
// data. markTurnStarted is the observed start (probe/WS) and must NOT clear,
// because a view attaching mid-turn needs to keep what the store already has.
export function beginTurn(chatId: string): void {
  setChatSession(chatId, {
    running: true,
    liveSubTasks: [],
    toolActivityByAgent: {},
    thinkingByAgent: {},
    streamingReplyText: '',
    isFinalizing: false,
    chartPending: false,
    failedMessage: null,
    endedAt: 0,
  });
}

export function markTurnStarted(chatId: string): void {
  setChatSession(chatId, { running: true });
}

export function markTurnEnded(chatId: string): void {
  if (!getChatSession(chatId).running) return;
  setChatSession(chatId, { running: false, endedAt: Date.now() });
  maybeUnsubscribe(chatId);
}

export function failTurn(chatId: string, text: string, description: string): void {
  setChatSession(chatId, { failedMessage: { text, description }, running: false, endedAt: Date.now() });
  maybeUnsubscribe(chatId);
}

export function clearTurnLive(chatId: string): void {
  setChatSession(chatId, {
    liveSubTasks: [],
    toolActivityByAgent: {},
    thinkingByAgent: {},
    streamingReplyText: '',
    isFinalizing: false,
    chartPending: false,
  });
}

export function useChatSession(chatId: string): ChatSession {
  const subscribe = useCallback((listener: () => void) => subscribeChatSession(chatId, listener), [chatId]);
  const getSnapshot = useCallback(() => getChatSession(chatId), [chatId]);
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}
