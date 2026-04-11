-- Revert operator_type constraint to original values.

ALTER TABLE order_status_logs
    DROP CONSTRAINT IF EXISTS order_status_logs_operator_type_check;

ALTER TABLE order_status_logs
    ADD CONSTRAINT order_status_logs_operator_type_check CHECK (
        operator_type IS NULL OR operator_type IN ('user', 'merchant', 'system')
    );
