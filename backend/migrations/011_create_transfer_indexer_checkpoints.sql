CREATE TABLE IF NOT EXISTS transfer_indexer_checkpoints (
  id BIGINT PRIMARY KEY,
  last_indexed_block BIGINT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
