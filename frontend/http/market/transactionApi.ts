import client from '@/http/client';

export interface RecordStockTransactionInput {
  ticker: string;
  tx_hash: string;
  wallet_address: string;
  side: 'buy' | 'sell';
  idrx_amount: string;
  stock_amount: string;
  protocol_fee_idrx?: string;
  block_number: number;
  log_index?: number;
}

export interface RecordStockTransferInput {
  ticker: string;
  tx_hash: string;
  from_address: string;
  to_address: string;
  idrx_amount: string;
  stock_amount: string;
  block_number: number;
  log_index?: number;
}

export type TransactionSide = 'buy' | 'sell' | 'request-redeem' | 'redeemed' | 'cancel-redeem' | 'transfer-in' | 'transfer-out';

export interface StockTransaction {
  id: number;
  stock_id: number;
  ticker: string;
  stock_name: string;
  idx_ticker: string;
  wallet_address: string;
  side: TransactionSide;
  idrx_amount: string;
  stock_amount: string;
  protocol_fee_idrx?: string;
  tx_hash: string;
  block_number: number;
  log_index: number;
  created_at: string;
}

export async function recordStockTransaction(input: RecordStockTransactionInput) {
  await client.post('/public/stock-transactions', input);
}

export async function recordStockTransfer(input: RecordStockTransferInput) {
  await client.post('/public/stock-transactions/transfers', input);
}

export async function getStockTransactions(walletAddress: string): Promise<StockTransaction[]> {
  const res = await client.get('/public/stock-transactions', {
    params: { wallet_address: walletAddress },
  });
  const payload = res.data?.data;
  return Array.isArray(payload) ? payload : [];
}
