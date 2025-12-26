-- Remove withdraw_rollback from rider_deposits type constraint

-- Drop new constraint
ALTER TABLE rider_deposits DROP CONSTRAINT IF EXISTS rider_deposits_type_check;

-- Restore original constraint (without withdraw_rollback)
ALTER TABLE rider_deposits 
ADD CONSTRAINT rider_deposits_type_check 
CHECK (type IN ('deposit', 'withdraw', 'freeze', 'unfreeze', 'deduct'));
