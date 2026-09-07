import client from '@/http/client';

export interface AgentTask {
  id: number;
  wallet_address: string;
  source_message_id: number | null;
  raw_prompt: string | null;
  is_actionable: boolean;
  status: string;
  summary: string | null;
  trigger_description: string | null;
  on_chain_task_id: number | null;
  paused: boolean;
  paused_at: string | null;
  created_at: string;
  updated_at: string;
  executed_at: string | null;
}

export interface AgentSubTask {
  id: number;
  task_id: number;
  step_order: number;
  agent: string;
  step_name: string;
  label: string | null;
  status: string;
  reasoning: string;
  output: string | null;
  prev_decision_hash: string;
  decision_hash: string;
  recorded_on_chain: boolean;
  on_chain_tx_hash: string | null;
  created_at: string;
}

export interface AgentTrade {
  id: number;
  task_id: number;
  sub_task_id: number;
  on_chain_trade_id: number | null;
  tx_hash: string | null;
  ticker: string;
  side: 'buy' | 'sell';
  amount: string;
  summary: string;
  executed_at: string;
}

export interface ArmTaskResult {
  on_chain_task_id: number;
  token_address?: string;
  total_budget?: string;
}

export async function listTasks(): Promise<AgentTask[]> {
  const res = await client.get('/agent/tasks');
  return Array.isArray(res.data?.data) ? res.data.data : [];
}

export async function getActivity(): Promise<AgentSubTask[]> {
  const res = await client.get('/agent/activity');
  return Array.isArray(res.data?.data) ? res.data.data : [];
}

export async function getTaskReasoning(taskId: number): Promise<AgentSubTask[]> {
  const res = await client.get(`/agent/tasks/${taskId}/reasoning`);
  return Array.isArray(res.data?.data) ? res.data.data : [];
}

export async function getTaskTrades(taskId: number): Promise<AgentTrade[]> {
  const res = await client.get(`/agent/tasks/${taskId}/trades`);
  return Array.isArray(res.data?.data) ? res.data.data : [];
}

export async function armTask(taskId: number, totalBudget: string, durationSec: number): Promise<ArmTaskResult> {
  const res = await client.post(`/agent/tasks/${taskId}/arm`, { total_budget: totalBudget, duration_sec: durationSec });
  return res.data.data;
}

export async function disarmTask(taskId: number): Promise<void> {
  await client.post(`/agent/tasks/${taskId}/disarm`);
}

export async function pauseTask(taskId: number): Promise<void> {
  await client.post(`/agent/tasks/${taskId}/pause`);
}

export async function resumeTask(taskId: number): Promise<void> {
  await client.post(`/agent/tasks/${taskId}/resume`);
}
