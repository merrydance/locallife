ALTER TABLE merchant_memberships
ADD COLUMN principal_balance BIGINT NOT NULL DEFAULT 0 CHECK (principal_balance >= 0),
ADD COLUMN bonus_balance BIGINT NOT NULL DEFAULT 0 CHECK (bonus_balance >= 0);

UPDATE merchant_memberships
SET principal_balance = balance,
    bonus_balance = 0
WHERE principal_balance = 0 AND bonus_balance = 0;

ALTER TABLE membership_transactions
ADD COLUMN principal_amount BIGINT NOT NULL DEFAULT 0,
ADD COLUMN bonus_amount BIGINT NOT NULL DEFAULT 0;

