DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'wallet_verifications'
          AND column_name = 'document_url'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'wallet_verifications'
          AND column_name = 'document_ref'
    ) THEN
        ALTER TABLE wallet_verifications RENAME COLUMN document_url TO document_ref;
    END IF;
END $$;

ALTER TABLE wallet_verifications
    ADD COLUMN IF NOT EXISTS full_name VARCHAR(120),
    ADD COLUMN IF NOT EXISTS email VARCHAR(255),
    ADD COLUMN IF NOT EXISTS approval_tx_hash CHAR(66);
