-- 为菜品添加预估制作时间字段（单位：分钟）
-- 用于厨房显示系统计算订单预估出餐时间

ALTER TABLE dishes ADD COLUMN prepare_time SMALLINT NOT NULL DEFAULT 10;

COMMENT ON COLUMN dishes.prepare_time IS '预估制作时间（分钟），默认10分钟';
