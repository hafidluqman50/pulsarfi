import client from '@/http/client';

export interface RecordStockTransactionInput {
  ticker: string;
  tx_hash: string;
  wallet_address: string;
  side: 'buy' | 'sell';
  idrx_amount: string;
  stock_amount: string;
  block_number: number;
}

export interface StockTransaction {
  id: number;
  stock_id: number;
  ticker: string;
  stock_name: string;
  idx_ticker: string;
  wallet_address: string;
  side: 'buy' | 'sell';
  idrx_amount: string;
  stock_amount: string;
  tx_hash: string;
  block_number: number;
  created_at: string;
}

export async function recordStockTransaction(input: RecordStockTransactionInput) {
  await client.post('/public/stock-transactions', input);
}

export async function getStockTransactions(walletAddress: string): Promise<StockTransaction[]> {
  const res = await client.get('/public/stock-transactions', {
    params: { wallet_address: walletAddress },
  });
  const payload = res.data?.data;
  return Array.isArray(payload) ? payload : [];
}
