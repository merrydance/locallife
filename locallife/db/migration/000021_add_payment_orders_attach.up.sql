-- 添加attach字段用于存储业务自定义数据
ALTER TABLE payment_orders ADD COLUMN attach TEXT;

-- 添加索引
CREATE INDEX payment_orders_attach_idx ON payment_orders USING gin (to_tsvector('simple', attach));
