-- 回滚：移除 regions 表的 qweather_location_id 字段

ALTER TABLE "regions" DROP COLUMN IF EXISTS "qweather_location_id";
