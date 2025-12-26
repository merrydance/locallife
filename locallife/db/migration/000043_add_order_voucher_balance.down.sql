-- 回滚：移除订单表的优惠券和余额支付字段

ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS orders_voucher_amount_check,
    DROP CONSTRAINT IF EXISTS orders_balance_paid_check;

DROP INDEX IF EXISTS orders_user_voucher_id_idx;
DROP INDEX IF EXISTS orders_membership_id_idx;

ALTER TABLE orders
    DROP COLUMN IF EXISTS user_voucher_id,
    DROP COLUMN IF EXISTS voucher_amount,
    DROP COLUMN IF EXISTS balance_paid,
    DROP COLUMN IF EXISTS membership_id;
