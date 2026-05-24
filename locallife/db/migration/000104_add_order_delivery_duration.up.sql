ALTER TABLE orders ADD COLUMN delivery_duration INTEGER;
COMMENT ON COLUMN orders.delivery_duration IS '代取预计在途时间（秒），由 LBS 真实路径计算得出';
