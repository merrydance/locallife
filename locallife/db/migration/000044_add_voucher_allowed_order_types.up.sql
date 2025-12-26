-- 为代金券模板添加允许的订单类型字段
-- 商家可以选择代金券适用于哪些订单场景：
--   - takeout: 外卖
--   - dine_in: 堂食
--   - takeaway: 外带
--   - reservation: 预定

ALTER TABLE vouchers
    ADD COLUMN allowed_order_types TEXT[] NOT NULL DEFAULT ARRAY['takeout', 'dine_in', 'takeaway', 'reservation'];

-- 添加注释说明
COMMENT ON COLUMN vouchers.allowed_order_types IS '允许使用的订单类型: takeout(外卖), dine_in(堂食), takeaway(外带), reservation(预定)';
