-- 移除打包费字段
-- 业务决策：打包费由商户自行决定，平台不内置此逻辑
-- 商户如需收取打包费，可将包装作为独立商品出售

ALTER TABLE orders DROP COLUMN IF EXISTS packing_fee;
