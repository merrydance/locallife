-- 回滚：移除骑手表的区域ID字段

DROP INDEX IF EXISTS idx_riders_region_id;
ALTER TABLE riders DROP COLUMN IF EXISTS region_id;
