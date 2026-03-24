ALTER TABLE refund_orders
DROP CONSTRAINT IF EXISTS refund_orders_refund_type_check;

ALTER TABLE refund_orders
ADD CONSTRAINT refund_orders_refund_type_check
CHECK (refund_type IN ('miniprogram', 'profit_sharing', 'rider_deposit'));