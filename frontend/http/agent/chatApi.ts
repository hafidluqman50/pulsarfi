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
  content_type: 'text' | 'workflow_card' | 'chart';
  content: string;
  ui_component: string | null;
  ui_props: unknown;
  ui_ref_task_id: number | null;
  created_at: string;
}

export interface WorkflowCard {
  task_id?: number;
  reply: string;
  content_type: 'text' | 'chart';
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

export type ChatStreamEvent =
  | { type: 'sub_task'; data: AgentSubTask }
  | { type: 'final'; data: WorkflowCard }
  | { type: 'error'; data: { message: string } };

async function readChatStream(path: string, body: unknown, onEvent?: (event: ChatStreamEvent) => void): Promise<WorkflowCard> {
  const token = typeof window !== 'undefined' ? localStorage.getItem('access_token') : null;
  const res = await fetch(`${process.env.NEXT_PUBLIC_BACKEND_URL}/api/v1${path}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(body),
  });
  if (!res.body) {
    throw new Error(`stream request failed with status ${res.status}`);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let final: WorkflowCard | null = null;

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const frames = buffer.split('\n\n');
    buffer = frames.pop() ?? '';
    for (const frame of frames) {
      const line = frame.trim();
      if (!line.startsWith('data:')) continue;
      const jsonStr = line.slice(5).trim();
      if (!jsonStr) continue;
      let event: ChatStreamEvent;
      try {
        event = JSON.parse(jsonStr);
      } catch {
        continue;
      }
      onEvent?.(event);
      if (event.type === 'error') throw new Error(event.data.message);
      if (event.type === 'final') final = event.data;
    }
  }

  if (!final) throw new Error('stream ended without a final event');
  return final;
}

export async function sendChatMessage(chatId: string, message: string): Promise<WorkflowCard> {
  return readChatStream(`/agent/chats/${chatId}/messages`, { message });
}

export async function streamChatMessage(chatId: string, message: string, onEvent: (event: ChatStreamEvent) => void): Promise<WorkflowCard> {
  return readChatStream(`/agent/chats/${chatId}/messages`, { message }, onEvent);
}

export async function retryLastMessage(chatId: string, onEvent: (event: ChatStreamEvent) => void): Promise<WorkflowCard> {
  return readChatStream(`/agent/chats/${chatId}/messages/retry`, {}, onEvent);
}
