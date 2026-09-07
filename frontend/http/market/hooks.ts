import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useRealtimeConnected, useRealtimeTopic } from '../realtime/useRealtimeSocket';
import { getMarketStocks, getStockHistory, getStockPrice, type MarketStock } from './priceApi';
import { getStockTransactions } from './transactionApi';
import { getProtocolStats, type ProtocolStats } from './statsApi';

export function useStockPrice(ticker: string, source?: 'idx') {
  return useQuery({
    queryKey: ['prices', ticker, source ?? 'default'],
    queryFn: () => getStockPrice(ticker, source),
    refetchInterval: 15_000,
    enabled: !!ticker,
  });
}

export function useStockHistory(ticker: string, range: string, source?: 'idx') {
  return useQuery({
    queryKey: ['price-history', ticker, range, source ?? 'default'],
    queryFn: () => getStockHistory(ticker, range, source),
    refetchInterval: 60_000,
    enabled: !!ticker && !!range,
  });
}

export function useMarketStocks() {
  const queryClient = useQueryClient();
  const connected = useRealtimeConnected();
  useRealtimeTopic<MarketStock[]>('market-stocks', (data) => {
    queryClient.setQueryData(['market-stocks'], data);
  });
  return useQuery({
    queryKey: ['market-stocks'],
    queryFn: getMarketStocks,
    refetchInterval: connected ? false : 15_000,
  });
}

export function useProtocolStats() {
  const queryClient = useQueryClient();
  const connected = useRealtimeConnected();
  useRealtimeTopic<ProtocolStats>('protocol-stats', (data) => {
    queryClient.setQueryData(['protocol-stats'], data);
  });
  return useQuery({
    queryKey: ['protocol-stats'],
    queryFn: getProtocolStats,
    refetchInterval: connected ? false : 30_000,
  });
}

export function useStockTransactions(walletAddress?: string) {
  return useQuery({
    queryKey: ['stock-transactions', walletAddress],
    queryFn: () => getStockTransactions(walletAddress!),
    enabled: Boolean(walletAddress),
    refetchInterval: 15_000,
  });
}
