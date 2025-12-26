-- 为 regions 表添加和风天气 LocationID 字段
-- 用于缓存和风天气 API 返回的城市 ID，避免重复查询 GeoAPI

ALTER TABLE "regions" ADD COLUMN "qweather_location_id" text;

-- 添加注释
COMMENT ON COLUMN "regions"."qweather_location_id" IS '和风天气城市ID，首次查询后缓存';
