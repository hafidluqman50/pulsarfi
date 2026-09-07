import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import { toast } from 'sonner';
import { useRealtimeConnected, useRealtimeTopic } from '../realtime/useRealtimeSocket';
import * as chatApi from './chatApi';
import * as taskApi from './taskApi';

// Every agent mutation below routes its failure through this — without it,
// a failed request (expired session, 500, network drop) fails completely
// silently: no toast, no console output, just a button that appears to do
// nothing when clicked.
function agentErrorMessage(error: unknown): string {
  if (isAxiosError(error)) {
    const backendMessage = (error.response?.data as { message?: string } | undefined)?.message;
    if (backendMessage) return backendMessage;
  }
  return error instanceof Error ? error.message : 'Something went wrong';
}

function toastAgentError(title: string) {
  return (error: unknown) => toast.error(title, { description: agentErrorMessage(error) });
}

export function useAgentChats() {
  return useQuery({ queryKey: ['agent-chats'], queryFn: chatApi.listChats });
}

export function useChatMessages(chatId?: string) {
  return useQuery({
    queryKey: ['agent-chat-messages', chatId],
    queryFn: () => chatApi.getChatMessages(chatId!),
    enabled: !!chatId,
  });
}

export function useSendChatMessage(chatId?: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (message: string) => chatApi.sendChatMessage(chatId!, message),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agent-chat-messages', chatId] });
      queryClient.invalidateQueries({ queryKey: ['agent-tasks'] });
    },
    onError: toastAgentError('Message failed to send'),
  });
}

export function useAgentTasks() {
  return useQuery({ queryKey: ['agent-tasks'], queryFn: taskApi.listTasks });
}

export function useAgentActivity() {
  return useQuery({ queryKey: ['agent-activity'], queryFn: taskApi.getActivity });
}

// upsertById replaces the row matching incoming.id if present, otherwise
// appends it — how a single pushed row (one agent_sub_tasks/agent_trades
// insert) gets folded into the already-cached list, instead of refetching
// the whole list over HTTP for one new row.
function upsertById<T extends { id: number }>(prev: T[] | undefined, incoming: T): T[] {
  if (!prev) return [incoming];
  const index = prev.findIndex((row) => row.id === incoming.id);
  if (index === -1) return [...prev, incoming];
  const next = [...prev];
  next[index] = incoming;
  return next;
}

// useTaskReasoning gets each new Sub Task row pushed the instant
// SubTaskRecorder.Record persists it (docs/plans/realtime-websocket-updates.md
// §3), falling back to the original 5s poll only while the socket is
// disconnected.
export function useTaskReasoning(taskId?: number) {
  const queryClient = useQueryClient();
  const connected = useRealtimeConnected();
  const topic = taskId ? `agent-task-reasoning:${taskId}` : undefined;

  useRealtimeTopic<taskApi.AgentSubTask>(topic, (row) => {
    queryClient.setQueryData<taskApi.AgentSubTask[]>(['agent-task-reasoning', taskId], (prev) => upsertById(prev, row));
  });

  return useQuery({
    queryKey: ['agent-task-reasoning', taskId],
    queryFn: () => taskApi.getTaskReasoning(taskId!),
    enabled: !!taskId,
    refetchInterval: connected ? false : 5_000,
  });
}

// useTaskTrades subscribes to the same topic mechanism as useTaskReasoning,
// though nothing publishes to it yet — AgentTradeRepository.Create has no
// caller anywhere in this codebase yet (a pre-existing gap, not introduced
// here). Wired now so trades start updating live for free the moment that
// gap is closed, instead of needing this hook revisited too.
export function useTaskTrades(taskId?: number) {
  const queryClient = useQueryClient();
  const connected = useRealtimeConnected();
  const topic = taskId ? `agent-task-trades:${taskId}` : undefined;

  useRealtimeTopic<taskApi.AgentTrade>(topic, (row) => {
    queryClient.setQueryData<taskApi.AgentTrade[]>(['agent-task-trades', taskId], (prev) => upsertById(prev, row));
  });

  return useQuery({
    queryKey: ['agent-task-trades', taskId],
    queryFn: () => taskApi.getTaskTrades(taskId!),
    enabled: !!taskId,
    refetchInterval: connected ? false : 15_000,
  });
}

export function useArmTask(taskId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { totalBudget: string; durationSec: number }) =>
      taskApi.armTask(taskId, input.totalBudget, input.durationSec),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-tasks'] }),
    onError: toastAgentError('Could not arm the task'),
  });
}

export function useDisarmTask(taskId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => taskApi.disarmTask(taskId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-tasks'] }),
    onError: toastAgentError('Could not disarm the task'),
  });
}

export function usePauseTask(taskId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => taskApi.pauseTask(taskId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-tasks'] }),
    onError: toastAgentError('Could not pause the task'),
  });
}

export function useResumeTask(taskId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => taskApi.resumeTask(taskId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-tasks'] }),
    onError: toastAgentError('Could not resume the task'),
  });
}
