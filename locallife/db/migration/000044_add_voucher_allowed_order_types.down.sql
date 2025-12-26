-- 回滚：删除代金券允许的订单类型字段
ALTER TABLE vouchers
    DROP COLUMN IF EXISTS allowed_order_types;
