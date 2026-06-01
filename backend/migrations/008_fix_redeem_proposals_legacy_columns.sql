ALTER TABLE redeem_proposals DROP COLUMN IF EXISTS source;
ALTER TABLE redeem_proposals ALTER COLUMN approval_count SET DEFAULT 0;
ALTER TABLE redeem_proposals ALTER COLUMN executed SET DEFAULT false;
