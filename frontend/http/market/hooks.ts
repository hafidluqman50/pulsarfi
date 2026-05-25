import { useQuery } from '@tanstack/react-query';
import { getMarketStocks, getStockPrice } from './priceApi';

export function useStockPrice(ticker: string) {
  return useQuery({
    queryKey: ['prices', ticker],
    queryFn: () => getStockPrice(ticker),
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
