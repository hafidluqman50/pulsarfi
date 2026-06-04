import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createWalletVerification, getCustodianRequests, getCustodianStats, getCustodianStocks, getReserves, getWalletVerifications } from './custodianApi';

export function useCustodianStats() {
  return useQuery({
    queryKey: ['custodian', 'stats'],
    queryFn: getCustodianStats,
    refetchInterval: 30_000,
  });
}

export function useCustodianRequests() {
  return useQuery({
    queryKey: ['custodian', 'requests'],
    queryFn: getCustodianRequests,
    refetchInterval: 15_000,
  });
}

export function useCustodianStocks() {
  return useQuery({
    queryKey: ['custodian', 'stocks'],
    queryFn: getCustodianStocks,
    refetchInterval: 30_000,
  });
}

export function useReserves() {
  return useQuery({
    queryKey: ['public', 'reserves'],
    queryFn: getReserves,
    refetchInterval: 30_000,
  });
}

export function useWalletVerifications() {
  return useQuery({
    queryKey: ['custodian', 'wallet-verifications'],
    queryFn: getWalletVerifications,
    refetchInterval: 30_000,
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
