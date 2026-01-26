ALTER TABLE delivery_pool ADD COLUMN expected_delivery_at TIMESTAMPTZ;
COMMENT ON COLUMN delivery_pool.expected_delivery_at IS '预计送达时间';
