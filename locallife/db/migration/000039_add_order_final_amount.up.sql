-- 添加 final_amount 和 platform_commission 到 orders 表
-- final_amount: 用户实际支付金额（扣除折扣后）
-- platform_commission: 平台佣金

ALTER TABLE orders ADD COLUMN IF NOT EXISTS final_amount bigint DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS platform_commission bigint DEFAULT 0;

-- 添加约束
ALTER TABLE orders ADD CONSTRAINT orders_final_amount_check CHECK (final_amount >= 0);
ALTER TABLE orders ADD CONSTRAINT orders_platform_commission_check CHECK (platform_commission >= 0);

-- 更新现有数据：将 final_amount 设置为 total_amount - discount_amount
UPDATE orders SET final_amount = GREATEST(total_amount - discount_amount, 0) WHERE final_amount = 0 OR final_amount IS NULL;

-- 创建索引以提高统计查询性能
CREATE INDEX IF NOT EXISTS idx_orders_final_amount ON orders(final_amount);
