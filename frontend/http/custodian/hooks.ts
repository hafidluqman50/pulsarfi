import { useQuery } from '@tanstack/react-query';
import { getCustodianRequests, getCustodianStats, getReserves } from './custodianApi';

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

export function useReserves() {
  return useQuery({
    queryKey: ['public', 'reserves'],
    queryFn: getReserves,
    refetchInterval: 30_000,
  });
}
