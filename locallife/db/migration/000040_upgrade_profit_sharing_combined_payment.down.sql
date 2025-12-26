-- 回滚：移除合单支付和分账升级

-- 移除 payment_orders 的合单关联
ALTER TABLE payment_orders DROP COLUMN IF EXISTS combined_payment_id;

-- 删除合单支付子订单表
DROP TABLE IF EXISTS combined_payment_sub_orders;

-- 删除合单支付主表
DROP TABLE IF EXISTS combined_payment_orders;

-- 移除 profit_sharing_orders 的新字段
ALTER TABLE profit_sharing_orders 
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_rider_amount_check;

DROP INDEX IF EXISTS profit_sharing_orders_rider_id_idx;

ALTER TABLE profit_sharing_orders 
    DROP COLUMN IF EXISTS delivery_fee,
    DROP COLUMN IF EXISTS rider_id,
    DROP COLUMN IF EXISTS rider_amount,
    DROP COLUMN IF EXISTS distributable_amount,
    DROP COLUMN IF EXISTS platform_rate,
    DROP COLUMN IF EXISTS operator_rate;
