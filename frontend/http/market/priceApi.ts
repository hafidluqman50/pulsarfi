import client from '@/http/client';

export interface MarketStock {
  ticker: string;
  stock_name: string;
  idx_ticker: string;
  sector: string | null;
  contract_address: string | null;
  price: number;
  pool_price: number;
  change_24h: number;
  sparkline_7d: number[];
}

export interface StockPrice {
  price: number;
  change_24h: number;
  currency: string;
  source: string;
  fetched_at: string;
  sparkline_1d?: number[];
}

export interface PriceHistoryPoint {
  timestamp: number;
  value: number;
}

export async function getStockPrice(ticker: string, source?: 'idx'): Promise<StockPrice> {
  const res = await client.get(`/public/prices/${ticker}`, {
    params: source ? { source } : undefined,
  });
  return res.data.data;
}

export async function getStockHistory(ticker: string, range: string, source?: 'idx'): Promise<PriceHistoryPoint[]> {
  const res = await client.get(`/public/prices/${ticker}/history`, {
    params: { range, ...(source ? { source } : {}) },
  });
  return Array.isArray(res.data?.data) ? res.data.data : [];
}

export async function getMarketStocks(): Promise<MarketStock[]> {
  const res = await client.get('/public/stocks');
  const payload = res.data?.data;
  if (Array.isArray(payload)) return payload;
  if (Array.isArray(payload?.stocks)) return payload.stocks;
  if (Array.isArray(res.data)) return res.data;
  return [];
}
