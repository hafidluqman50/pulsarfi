ALTER TABLE stock_transactions
  ADD COLUMN IF NOT EXISTS protocol_fee_idrx NUMERIC(78,0) NOT NULL DEFAULT 0;
