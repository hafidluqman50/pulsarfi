import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRealtimeConnected, useRealtimeTopic } from '../realtime/useRealtimeSocket';
import {
  createWalletVerification,
  getCustodianRequests,
  getCustodianStats,
  getCustodianStocks,
  getReserves,
  getWalletVerifications,
  type CustodianRequests,
  type CustodianStats,
  type CustodianStock,
  type ReserveEntry,
  type WalletVerification,
} from './custodianApi';

export function useCustodianStats() {
  const queryClient = useQueryClient();
  const connected = useRealtimeConnected();
  useRealtimeTopic<CustodianStats>('custodian-stats', (data) => {
    queryClient.setQueryData(['custodian', 'stats'], data);
  });
  return useQuery({
    queryKey: ['custodian', 'stats'],
    queryFn: getCustodianStats,
    refetchInterval: connected ? false : 30_000,
  });
}

export function useCustodianRequests() {
  const queryClient = useQueryClient();
  const connected = useRealtimeConnected();
  useRealtimeTopic<CustodianRequests>('custodian-requests', (data) => {
    queryClient.setQueryData(['custodian', 'requests'], data);
  });
  return useQuery({
    queryKey: ['custodian', 'requests'],
    queryFn: getCustodianRequests,
    refetchInterval: connected ? false : 15_000,
  });
}

export function useCustodianStocks() {
  const queryClient = useQueryClient();
  const connected = useRealtimeConnected();
  useRealtimeTopic<CustodianStock[]>('custodian-stocks', (data) => {
    queryClient.setQueryData(['custodian', 'stocks'], data);
  });
  return useQuery({
    queryKey: ['custodian', 'stocks'],
    queryFn: getCustodianStocks,
    refetchInterval: connected ? false : 30_000,
  });
}

export function useReserves() {
  const queryClient = useQueryClient();
  const connected = useRealtimeConnected();
  useRealtimeTopic<ReserveEntry[]>('reserves', (data) => {
    queryClient.setQueryData(['public', 'reserves'], data);
  });
  return useQuery({
    queryKey: ['public', 'reserves'],
    queryFn: getReserves,
    refetchInterval: connected ? false : 30_000,
  });
}

export function useWalletVerifications() {
  const queryClient = useQueryClient();
  const connected = useRealtimeConnected();
  useRealtimeTopic<WalletVerification[]>('custodian-wallet-verifications', (data) => {
    queryClient.setQueryData(['custodian', 'wallet-verifications'], data);
  });
  return useQuery({
    queryKey: ['custodian', 'wallet-verifications'],
    queryFn: getWalletVerifications,
    refetchInterval: connected ? false : 30_000,
  });
}

export function useCreateWalletVerification() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createWalletVerification,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
    },
  });
}
