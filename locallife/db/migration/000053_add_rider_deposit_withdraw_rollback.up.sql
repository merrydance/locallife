-- Add withdraw_rollback to rider_deposits type constraint

-- Drop old constraint
ALTER TABLE rider_deposits DROP CONSTRAINT IF EXISTS rider_deposits_type_check;

-- Add new constraint with withdraw_rollback type
ALTER TABLE rider_deposits 
ADD CONSTRAINT rider_deposits_type_check 
CHECK (type IN ('deposit', 'withdraw', 'freeze', 'unfreeze', 'deduct', 'withdraw_rollback'));

-- Add comment
COMMENT ON COLUMN rider_deposits.type IS '流水类型: deposit=充值, withdraw=提现, freeze=冻结, unfreeze=解冻, deduct=扣款, withdraw_rollback=提现回滚';
