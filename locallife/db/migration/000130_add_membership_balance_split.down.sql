ALTER TABLE membership_transactions
DROP COLUMN IF EXISTS principal_amount,
DROP COLUMN IF EXISTS bonus_amount;

ALTER TABLE merchant_memberships
DROP COLUMN IF EXISTS principal_balance,
DROP COLUMN IF EXISTS bonus_balance;
