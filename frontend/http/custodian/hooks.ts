import { useQuery } from '@tanstack/react-query';
import { getCustodianStats } from './custodianApi';

export function useCustodianStats() {
  return useQuery({
    queryKey: ['custodian', 'stats'],
    queryFn: getCustodianStats,
    refetchInterval: 30_000,
  });
}
