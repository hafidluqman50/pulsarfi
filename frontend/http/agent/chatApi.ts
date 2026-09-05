import client from '@/http/client';

export interface AgentChat {
  id: number;
  owner_wallet: string;
  description: string | null;
  created_at: string;
}

export interface AgentChatMessage {
  id: number;
  chat_id: number;
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

export async function createChat(): Promise<AgentChat> {
  const res = await client.post('/agent/chats');
  return res.data.data;
}

export async function listChats(): Promise<AgentChat[]> {
  const res = await client.get('/agent/chats');
  return Array.isArray(res.data?.data) ? res.data.data : [];
}

export async function getChatMessages(chatId: number): Promise<AgentChatMessage[]> {
  const res = await client.get(`/agent/chats/${chatId}/messages`);
  return Array.isArray(res.data?.data) ? res.data.data : [];
}

export async function sendChatMessage(chatId: number, message: string): Promise<WorkflowCard> {
  const res = await client.post(`/agent/chats/${chatId}/messages`, { message });
  return res.data.data;
}
