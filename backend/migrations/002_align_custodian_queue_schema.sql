BEGIN;

ALTER TABLE mint_proposals
    ADD COLUMN IF NOT EXISTS status VARCHAR(10);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'mint_proposals'
          AND column_name = 'executed'
    ) THEN
        EXECUTE 'UPDATE mint_proposals SET status = CASE WHEN COALESCE(executed, false) THEN ''executed'' ELSE ''pending'' END WHERE status IS NULL';
    ELSE
        UPDATE mint_proposals SET status = 'pending' WHERE status IS NULL;
    END IF;
END $$;

ALTER TABLE mint_proposals
    ALTER COLUMN status SET DEFAULT 'pending',
    ALTER COLUMN status SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'mint_proposals_status_check'
    ) THEN
        ALTER TABLE mint_proposals
            ADD CONSTRAINT mint_proposals_status_check
            CHECK (status IN ('pending', 'executed', 'rejected'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS mint_attestations (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    proposal_id  BIGINT      NOT NULL REFERENCES mint_proposals(id) ON DELETE CASCADE,
    custodian_id BIGINT      NOT NULL REFERENCES custodians(id),
    type         VARCHAR(10) NOT NULL CHECK (type IN ('approve', 'reject')),
    tx_hash      CHAR(66),
    attested_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (proposal_id, custodian_id)
);

DO $$
BEGIN
    IF to_regclass('public.mint_approvals') IS NOT NULL THEN
        INSERT INTO mint_attestations (proposal_id, custodian_id, type, tx_hash, attested_at)
        SELECT proposal_id, custodian_id, 'approve', tx_hash, COALESCE(approved_at, NOW())
        FROM mint_approvals
        ON CONFLICT (proposal_id, custodian_id) DO NOTHING;
    END IF;
END $$;

ALTER TABLE redeem_proposals
    ADD COLUMN IF NOT EXISTS status VARCHAR(10),
    ADD COLUMN IF NOT EXISTS fee_idrx NUMERIC(78, 0),
    ADD COLUMN IF NOT EXISTS user_address VARCHAR(42);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'redeem_proposals'
          AND column_name = 'idrx_amount'
    ) THEN
        EXECUTE 'UPDATE redeem_proposals SET fee_idrx = COALESCE(fee_idrx, idrx_amount, 0) WHERE fee_idrx IS NULL';
    ELSE
        UPDATE redeem_proposals SET fee_idrx = COALESCE(fee_idrx, 0) WHERE fee_idrx IS NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'redeem_proposals'
          AND column_name = 'requester_id'
    ) THEN
        EXECUTE '
            UPDATE redeem_proposals rp
            SET user_address = COALESCE(rp.user_address, c.wallet_address, '''')
            FROM custodians c
            WHERE rp.requester_id = c.id
              AND rp.user_address IS NULL
        ';
    END IF;
END $$;

UPDATE redeem_proposals
SET user_address = COALESCE(user_address, '')
WHERE user_address IS NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'redeem_proposals'
          AND column_name = 'executed'
    ) THEN
        EXECUTE 'UPDATE redeem_proposals SET status = CASE WHEN COALESCE(executed, false) THEN ''executed'' ELSE ''pending'' END WHERE status IS NULL';
    ELSE
        UPDATE redeem_proposals SET status = 'pending' WHERE status IS NULL;
    END IF;
END $$;

ALTER TABLE redeem_proposals
    ALTER COLUMN status SET DEFAULT 'pending',
    ALTER COLUMN status SET NOT NULL,
    ALTER COLUMN fee_idrx SET NOT NULL,
    ALTER COLUMN user_address SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'redeem_proposals_status_check'
    ) THEN
        ALTER TABLE redeem_proposals
            ADD CONSTRAINT redeem_proposals_status_check
            CHECK (status IN ('pending', 'executed', 'rejected'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS redeem_attestations (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    proposal_id  BIGINT      NOT NULL REFERENCES redeem_proposals(id) ON DELETE CASCADE,
    custodian_id BIGINT      NOT NULL REFERENCES custodians(id),
    type         VARCHAR(10) NOT NULL CHECK (type IN ('approve', 'reject')),
    tx_hash      CHAR(66),
    attested_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (proposal_id, custodian_id)
);

DO $$
BEGIN
    IF to_regclass('public.redeem_approvals') IS NOT NULL THEN
        INSERT INTO redeem_attestations (proposal_id, custodian_id, type, tx_hash, attested_at)
        SELECT proposal_id, custodian_id, 'approve', tx_hash, COALESCE(approved_at, NOW())
        FROM redeem_approvals
        ON CONFLICT (proposal_id, custodian_id) DO NOTHING;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_mint_proposals_status ON mint_proposals(status);
CREATE INDEX IF NOT EXISTS idx_redeem_proposals_status ON redeem_proposals(status);
CREATE INDEX IF NOT EXISTS idx_mint_attestations_proposal_id ON mint_attestations(proposal_id);
CREATE INDEX IF NOT EXISTS idx_redeem_attestations_proposal_id ON redeem_attestations(proposal_id);

COMMIT;
