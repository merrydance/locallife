ALTER TABLE reconciliation_reports
    DROP CONSTRAINT IF EXISTS reconciliation_reports_bill_type_check;

ALTER TABLE reconciliation_reports
    ADD CONSTRAINT reconciliation_reports_bill_type_check
    CHECK (bill_type IN ('trade', 'ecommerce_trade', 'refund'));
