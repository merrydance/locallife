-- Extend order_status_logs.operator_type to include rider actions.
-- Existing code writes operator_type='rider' from delivery/rider flows.

ALTER TABLE order_status_logs
    DROP CONSTRAINT IF EXISTS order_status_logs_operator_type_check;

ALTER TABLE order_status_logs
    ADD CONSTRAINT order_status_logs_operator_type_check CHECK (
        operator_type IS NULL OR operator_type IN ('user', 'merchant', 'system', 'rider')
    );
