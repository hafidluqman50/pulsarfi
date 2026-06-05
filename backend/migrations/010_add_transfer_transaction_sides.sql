ALTER TABLE stock_transactions ALTER COLUMN side TYPE VARCHAR(20);

ALTER TABLE stock_transactions DROP CONSTRAINT IF EXISTS stock_transactions_side_check;
ALTER TABLE stock_transactions ADD CONSTRAINT stock_transactions_side_check
  CHECK (side IN ('buy', 'sell', 'request-redeem', 'redeemed', 'cancel-redeem', 'transfer-in', 'transfer-out'));

ALTER TABLE stock_transactions ADD COLUMN IF NOT EXISTS log_index INTEGER NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS stock_transactions_tx_hash_key;
DROP INDEX IF EXISTS stock_transactions_tx_hash_wallet_side_key;
CREATE UNIQUE INDEX IF NOT EXISTS stock_transactions_tx_log_wallet_side_key
  ON stock_transactions (tx_hash, log_index, wallet_address, side);
