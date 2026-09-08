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
  sender: 'user' | 'supervisor';
  content_type: 'text' | 'workflow_card' | 'chart' | 'news';
  content: string;
  ui_component: string | null;
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

export interface SubTaskStarted {
  agent: string;
  step_name: string;
  label: string;
}

export interface ToolCallEvent {
  agent: string;
  tool: string;
  phase: 'start' | 'end';
}

export type ChatStreamEvent =
  | { type: 'sub_task'; data: AgentSubTask }
  | { type: 'sub_task_started'; data: SubTaskStarted }
  | { type: 'tool_call'; data: ToolCallEvent }
  | { type: 'reply_delta'; data: { delta: string } }
  | { type: 'final'; data: WorkflowCard }
  | { type: 'error'; data: { message: string } };

// chatStreamTopic must match the backend's own chatStreamTopic()
// (backend/src/http/handlers/agent/chats.go) exactly — subscribe to this
// via useRealtimeTopic *before* calling sendChatMessage/retryLastMessage,
// since live progress (sub_task/sub_task_started/reply_delta) now arrives
// over the shared WebSocket, not in the HTTP response body.
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
export async function sendChatMessage(chatId: string, message: string): Promise<WorkflowCard> {
  const res = await client.post(`/agent/chats/${chatId}/messages`, { message });
  return res.data.data as WorkflowCard;
}

export async function retryLastMessage(chatId: string): Promise<WorkflowCard> {
  const res = await client.post(`/agent/chats/${chatId}/messages/retry`, {});
  return res.data.data as WorkflowCard;
}
