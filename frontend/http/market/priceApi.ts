import client from '@/http/client';

export interface MarketStock {
  ticker: string;
  stock_name: string;
  sector: string | null;
  price: number;
  change_24h: number;
  sparkline_7d: number[];
}

export interface StockPrice {
  price: number;
  change_24h: number;
  currency: string;
  source: string;
  fetched_at: string;
}

export async function getStockPrice(ticker: string): Promise<StockPrice> {
  const res = await client.get(`/public/prices/${ticker}`);
  return res.data.data;
}

export async function getMarketStocks(): Promise<MarketStock[]> {
  const res = await client.get('/public/stocks');
  const payload = res.data?.data;
  if (Array.isArray(payload)) return payload;
  if (Array.isArray(payload?.stocks)) return payload.stocks;
  if (Array.isArray(res.data)) return res.data;
  return [];
}
