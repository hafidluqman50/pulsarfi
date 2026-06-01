-- Extend stock_transactions.side to support redeem tracking
ALTER TABLE stock_transactions ALTER COLUMN side TYPE VARCHAR(20);

ALTER TABLE stock_transactions DROP CONSTRAINT IF EXISTS stock_transactions_side_check;
ALTER TABLE stock_transactions ADD CONSTRAINT stock_transactions_side_check
  CHECK (side IN ('buy', 'sell', 'request-redeem', 'redeemed'));
