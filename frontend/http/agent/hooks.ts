import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import * as chatApi from './chatApi';
import * as taskApi from './taskApi';

export function useAgentChats() {
  return useQuery({ queryKey: ['agent-chats'], queryFn: chatApi.listChats });
}

export function useCreateChat() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: chatApi.createChat,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-chats'] }),
  });
}

export function useChatMessages(chatId?: number) {
  return useQuery({
    queryKey: ['agent-chat-messages', chatId],
    queryFn: () => chatApi.getChatMessages(chatId!),
    enabled: !!chatId,
  });
}

// useSendChatMessage is also what answers a needs_input question — a
// clarifying answer is just the next chat message, Supervisor resolves it
// from conversation context. There is no separate answer-endpoint.
export function useSendChatMessage(chatId?: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (message: string) => chatApi.sendChatMessage(chatId!, message),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agent-chat-messages', chatId] });
      queryClient.invalidateQueries({ queryKey: ['agent-tasks'] });
    },
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
  });
}

export function useDisarmTask(taskId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => taskApi.disarmTask(taskId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-tasks'] }),
  });
}

export function usePauseTask(taskId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => taskApi.pauseTask(taskId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-tasks'] }),
  });
}

export function useResumeTask(taskId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => taskApi.resumeTask(taskId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-tasks'] }),
  });
}
