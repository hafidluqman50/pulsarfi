import client from '@/http/client';
import type { AgentSubTask } from './taskApi';

export interface AgentChat {
  id: string;
  owner_wallet: string;
  description: string | null;
  created_at: string;
}

export interface AgentChatMessage {
  id: number;
  chat_id: string;
  sender: 'user' | 'supervisor' | 'assistant';
  content_type: 'text' | 'workflow_card' | 'chart' | 'news' | 'horizon_notice' | string;
  content: string;
  ui_component: string | null;
  // { hidden: true } marks a real, persisted chat message (full history,
  // fed to the LLM as context) that must never render as a bubble — the
  // compiled answer a clarifying-questions card sends, not something the
  // user typed. content_type is a strict DB enum with no room for a
  // "hidden" variant, so this lives in the one unconstrained JSON column.
  ui_props: unknown;
  ui_ref_task_id: number | null;
  created_at: string;
}

export interface WorkflowCard {
  task_id?: number;
  reply: string;
  content_type: 'text' | 'chart' | 'news';
  ui_component?: string | null;
  ui_props?: unknown;
}

export async function getPortfolioChart(lens: string, range: string): Promise<unknown> {
  const res = await client.get('/agent/portfolio/chart', { params: { lens, range } });
  return res.data?.data;
}

export async function listChats(): Promise<AgentChat[]> {
  const res = await client.get('/agent/chats');
  return Array.isArray(res.data?.data) ? res.data.data : [];
}

export async function getChatMessages(chatId: string): Promise<AgentChatMessage[]> {
  const res = await client.get(`/agent/chats/${chatId}/messages`);
  return Array.isArray(res.data?.data) ? res.data.data : [];
}

// LiveSubTask is one row of the turn's own sub-task list, exactly as the
// backend decided it (backend/src/http/handlers/agent/chats.go's
// liveSubTaskEntry) — the frontend never receives a fragment for a single
// step, only this full list on every 'sub_tasks' push, so it never matches,
// merges, or infers anything about what happened to a step; it only ever
// replaces its whole local list with what it is given.
export interface LiveSubTask {
  agent: string;
  step_name: string;
  label?: string;
  status: 'in_progress' | 'done' | 'failed';
  reason?: string;
  row?: AgentSubTask;
}

export interface ToolCallEvent {
  agent: string;
  tool: string;
  phase: 'start' | 'end';
}

export interface ThinkingEvent {
  agent: string;
  delta: string;
}

export type ChatStreamEvent =
  | { type: 'sub_tasks'; data: LiveSubTask[] }
  | { type: 'tool_call'; data: ToolCallEvent }
  | { type: 'thinking'; data: ThinkingEvent }
  | { type: 'finalizing'; data: Record<string, never> }
  | { type: 'reply_delta'; data: { delta: string } }
  | { type: 'final'; data: WorkflowCard }
  | { type: 'error'; data: { message: string } };

// chatStreamTopic must match the backend's own chatStreamTopic()
// (backend/src/http/handlers/agent/chats.go) exactly — subscribe to this
// via useRealtimeTopic *before* calling sendChatMessage/retryLastMessage,
// since live progress (sub_tasks/reply_delta) now arrives over the shared
// WebSocket, not in the HTTP response body.
// docs/plans/agent-orchestration-graph-rebuild.md v2.6: "no SSE, disini
// pake socket."
export function chatStreamTopic(chatId: string): string {
  return `agent-chat-stream:${chatId}`;
}

// sendChatMessage/retryLastMessage are now plain, blocking POST requests —
// the response body carries only the final WorkflowCard (or throws on
// error), matching every other endpoint in this API. A caller that never
// subscribed to chatStreamTopic(chatId) still gets a correct, complete
// result, it just misses the live play-by-play.
export async function sendChatMessage(chatId: string, message: string, hidden?: boolean): Promise<WorkflowCard> {
  const res = await client.post(`/agent/chats/${chatId}/messages`, { message, hidden: hidden ?? false });
  return res.data.data as WorkflowCard;
}

export async function retryLastMessage(chatId: string): Promise<WorkflowCard> {
  const res = await client.post(`/agent/chats/${chatId}/messages/retry`, {});
  return res.data.data as WorkflowCard;
}
