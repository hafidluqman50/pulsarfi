import { useQuery } from '@tanstack/react-query';
import { getMarketStocks, getStockPrice } from './priceApi';
import { getStockTransactions } from './transactionApi';
import { getProtocolStats } from './statsApi';

export function useStockPrice(ticker: string, source?: 'idx') {
  return useQuery({
    queryKey: ['prices', ticker, source ?? 'default'],
    queryFn: () => getStockPrice(ticker, source),
    refetchInterval: 15_000,
    enabled: !!ticker,
  });
}

export function useMarketStocks() {
  return useQuery({
    queryKey: ['market-stocks'],
    queryFn: getMarketStocks,
    refetchInterval: 15_000,
  });
}

export function useProtocolStats() {
  return useQuery({
    queryKey: ['protocol-stats'],
    queryFn:  getProtocolStats,
    refetchInterval: 30_000,
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
