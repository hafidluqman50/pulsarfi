import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import { toast } from 'sonner';
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

// useTaskReasoning polls while a Task is being worked — Sub Task rows can
// append mid-run (analyzer_agent/executor_agent tool calls happen inside
// the same HandleChatMessage turn, but this lets the Plan card catch up
// even if the frontend polls independently of the chat response).
export function useTaskReasoning(taskId?: number) {
  return useQuery({
    queryKey: ['agent-task-reasoning', taskId],
    queryFn: () => taskApi.getTaskReasoning(taskId!),
    enabled: !!taskId,
    refetchInterval: 5_000,
  });
}

export function useTaskTrades(taskId?: number) {
  return useQuery({
    queryKey: ['agent-task-trades', taskId],
    queryFn: () => taskApi.getTaskTrades(taskId!),
    enabled: !!taskId,
    refetchInterval: 15_000,
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
