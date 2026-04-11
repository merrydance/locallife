-- 扩展 reconciliation_reports 的 bill_type 允许值，新增收付通退款账单类型
-- 原约束名由 PostgreSQL 自动生成，格式为 reconciliation_reports_bill_type_check

ALTER TABLE reconciliation_reports
    DROP CONSTRAINT IF EXISTS reconciliation_reports_bill_type_check;

ALTER TABLE reconciliation_reports
    ADD CONSTRAINT reconciliation_reports_bill_type_check
    CHECK (bill_type IN ('trade', 'ecommerce_trade', 'refund', 'ecommerce_refund'));
