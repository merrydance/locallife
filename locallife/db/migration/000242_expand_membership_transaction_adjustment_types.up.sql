ALTER TABLE membership_transactions
    DROP CONSTRAINT IF EXISTS membership_transactions_type_check;

ALTER TABLE membership_transactions
    ADD CONSTRAINT membership_transactions_type_check
        CHECK (type IN ('recharge', 'consume', 'refund', 'bonus', 'adjustment_credit', 'adjustment_debit'));
