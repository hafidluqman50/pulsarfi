UPDATE stock_transactions
SET wallet_address = LOWER(TRIM(wallet_address))
WHERE wallet_address <> LOWER(TRIM(wallet_address));

CREATE INDEX IF NOT EXISTS stock_transactions_wallet_address_idx
  ON stock_transactions (wallet_address);
