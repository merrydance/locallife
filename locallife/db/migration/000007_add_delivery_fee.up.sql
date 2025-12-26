-- M6: 运费计算服务
-- 设计说明：
-- - 区域级别：region_id 关联到区县级别，一个区县一个运营商
-- - 高峰配置：同一区域的时段不允许重叠，代码层面校验
-- - 满返促销：门槛式，阶梯取最优（满足条件的取减免金额最大的那条）
-- - 深夜配送：在 peak_hour_configs 中配置，支持跨天时段

-- 运费配置表（按区县配置基础运费规则）
CREATE TABLE "delivery_fee_configs" (
  "id" bigserial PRIMARY KEY,
  "region_id" bigint NOT NULL,
  "base_fee" bigint NOT NULL,
  "base_distance" int NOT NULL,
  "extra_fee_per_km" bigint NOT NULL,
  "value_ratio" decimal(5,4) NOT NULL DEFAULT 0.0100,
  "max_fee" bigint,
  "min_fee" bigint NOT NULL DEFAULT 0,
  "is_active" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  "updated_at" timestamptz
);

-- 天气系数记录表（定时任务抓取和风天气数据）
CREATE TABLE "weather_coefficients" (
  "id" bigserial PRIMARY KEY,
  "region_id" bigint NOT NULL,
  "recorded_at" timestamptz NOT NULL,
  "weather_data" jsonb,
  "warning_data" jsonb,
  "weather_type" text NOT NULL,
  "weather_code" text,
  "temperature" smallint,
  "feels_like" smallint,
  "humidity" smallint,
  "wind_speed" smallint,
  "wind_scale" text,
  "precip" decimal(5,2),
  "visibility" smallint,
  "has_warning" boolean NOT NULL DEFAULT false,
  "warning_type" text,
  "warning_level" text,
  "warning_severity" text,
  "warning_text" text,
  "weather_coefficient" decimal(3,2) NOT NULL DEFAULT 1.00,
  "warning_coefficient" decimal(3,2) NOT NULL DEFAULT 1.00,
  "final_coefficient" decimal(3,2) NOT NULL DEFAULT 1.00,
  "delivery_suspended" boolean NOT NULL DEFAULT false,
  "suspend_reason" text,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 高峰/特殊时段配置表
CREATE TABLE "peak_hour_configs" (
  "id" bigserial PRIMARY KEY,
  "region_id" bigint NOT NULL,
  "name" text NOT NULL,
  "start_time" time NOT NULL,
  "end_time" time NOT NULL,
  "coefficient" decimal(3,2) NOT NULL,
  "days_of_week" smallint[] NOT NULL,
  "is_active" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  "updated_at" timestamptz
);

-- 商户运费满返促销表
CREATE TABLE "merchant_delivery_promotions" (
  "id" bigserial PRIMARY KEY,
  "merchant_id" bigint NOT NULL,
  "name" text NOT NULL,
  "min_order_amount" bigint NOT NULL,
  "discount_amount" bigint NOT NULL,
  "valid_from" timestamptz NOT NULL,
  "valid_until" timestamptz NOT NULL,
  "is_active" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  "updated_at" timestamptz
);

-- 添加外键约束
ALTER TABLE "delivery_fee_configs" ADD FOREIGN KEY ("region_id") REFERENCES "regions" ("id") ON DELETE CASCADE;
ALTER TABLE "weather_coefficients" ADD FOREIGN KEY ("region_id") REFERENCES "regions" ("id") ON DELETE CASCADE;
ALTER TABLE "peak_hour_configs" ADD FOREIGN KEY ("region_id") REFERENCES "regions" ("id") ON DELETE CASCADE;
ALTER TABLE "merchant_delivery_promotions" ADD FOREIGN KEY ("merchant_id") REFERENCES "merchants" ("id") ON DELETE CASCADE;

-- 创建索引
CREATE UNIQUE INDEX ON "delivery_fee_configs" ("region_id");
CREATE INDEX ON "delivery_fee_configs" ("is_active");

CREATE INDEX ON "weather_coefficients" ("region_id");
CREATE INDEX ON "weather_coefficients" ("recorded_at");
CREATE INDEX ON "weather_coefficients" ("region_id", "recorded_at");
CREATE INDEX ON "weather_coefficients" ("has_warning");

CREATE INDEX ON "peak_hour_configs" ("region_id");
CREATE INDEX ON "peak_hour_configs" ("is_active");
CREATE INDEX ON "peak_hour_configs" ("region_id", "is_active");

CREATE INDEX ON "merchant_delivery_promotions" ("merchant_id");
CREATE INDEX ON "merchant_delivery_promotions" ("merchant_id", "is_active");
CREATE INDEX ON "merchant_delivery_promotions" ("valid_from", "valid_until");

-- 添加注释
COMMENT ON TABLE "delivery_fee_configs" IS '运费配置表，按区县配置基础运费规则';
COMMENT ON COLUMN "delivery_fee_configs"."region_id" IS '区县级别的region_id';
COMMENT ON COLUMN "delivery_fee_configs"."base_fee" IS '基础运费（分）';
COMMENT ON COLUMN "delivery_fee_configs"."base_distance" IS '基础距离（米），在此范围内收取基础运费';
COMMENT ON COLUMN "delivery_fee_configs"."extra_fee_per_km" IS '超出基础距离后每公里加价（分）';
COMMENT ON COLUMN "delivery_fee_configs"."value_ratio" IS '货值系数，如0.01表示1%';
COMMENT ON COLUMN "delivery_fee_configs"."max_fee" IS '最高运费上限（分），NULL表示不限';
COMMENT ON COLUMN "delivery_fee_configs"."min_fee" IS '最低运费（分）';

COMMENT ON TABLE "weather_coefficients" IS '天气系数记录表，定时抓取和风天气数据';
COMMENT ON COLUMN "weather_coefficients"."weather_type" IS 'sunny/cloudy/rainy/heavy_rain/snowy/extreme';
COMMENT ON COLUMN "weather_coefficients"."weather_code" IS '和风天气图标代码';
COMMENT ON COLUMN "weather_coefficients"."wind_speed" IS '风速（km/h）';
COMMENT ON COLUMN "weather_coefficients"."precip" IS '降水量（mm）';
COMMENT ON COLUMN "weather_coefficients"."visibility" IS '能见度（km）';
COMMENT ON COLUMN "weather_coefficients"."has_warning" IS '是否有预警';
COMMENT ON COLUMN "weather_coefficients"."warning_type" IS '预警类型代码，如1001=台风, 1003=暴雨';
COMMENT ON COLUMN "weather_coefficients"."warning_level" IS '预警等级: blue/yellow/orange/red';
COMMENT ON COLUMN "weather_coefficients"."warning_severity" IS '严重程度: minor/moderate/severe/extreme';
COMMENT ON COLUMN "weather_coefficients"."weather_coefficient" IS '天气系数';
COMMENT ON COLUMN "weather_coefficients"."warning_coefficient" IS '预警系数';
COMMENT ON COLUMN "weather_coefficients"."final_coefficient" IS '最终系数 = max(天气系数, 预警系数)';
COMMENT ON COLUMN "weather_coefficients"."delivery_suspended" IS '是否暂停配送（极端天气）';

COMMENT ON TABLE "peak_hour_configs" IS '高峰/特殊时段配置表（午高峰、晚高峰、深夜配送等）';
COMMENT ON COLUMN "peak_hour_configs"."name" IS '配置名称，如：午高峰、晚高峰、深夜配送';
COMMENT ON COLUMN "peak_hour_configs"."end_time" IS '结束时间，可小于start_time表示跨天（如22:00-06:00）';
COMMENT ON COLUMN "peak_hour_configs"."coefficient" IS '运费系数，如1.20表示加价20%';
COMMENT ON COLUMN "peak_hour_configs"."days_of_week" IS '生效的星期：1=周一...7=周日';

COMMENT ON TABLE "merchant_delivery_promotions" IS '商户运费满返促销表，门槛式阶梯取最优';
COMMENT ON COLUMN "merchant_delivery_promotions"."min_order_amount" IS '最低订单金额（分）';
COMMENT ON COLUMN "merchant_delivery_promotions"."discount_amount" IS '减免金额（分）';
