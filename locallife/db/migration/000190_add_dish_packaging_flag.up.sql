ALTER TABLE dishes
ADD COLUMN is_packaging boolean NOT NULL DEFAULT false;

CREATE INDEX idx_dishes_merchant_packaging_active
ON dishes (merchant_id, is_packaging, is_online, is_available)
WHERE deleted_at IS NULL;

COMMENT ON COLUMN dishes.is_packaging IS '是否为包装菜品；包装菜品仅用于外卖与自取订单的包装方式选择';