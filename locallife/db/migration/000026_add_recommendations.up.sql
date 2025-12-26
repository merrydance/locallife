-- M11: 千人千面推荐引擎

-- 用户行为埋点表
CREATE TABLE "user_behaviors" (
  "id" bigserial PRIMARY KEY,
  "user_id" bigint NOT NULL,
  "behavior_type" text NOT NULL,
  "dish_id" bigint,
  "combo_id" bigint,
  "merchant_id" bigint,
  "duration" integer,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 用户偏好表
CREATE TABLE "user_preferences" (
  "id" bigserial PRIMARY KEY,
  "user_id" bigint UNIQUE NOT NULL,
  "cuisine_preferences" jsonb,
  "price_range_min" bigint,
  "price_range_max" bigint,
  "avg_order_amount" bigint,
  "favorite_time_slots" integer[],
  "purchase_frequency" smallint NOT NULL DEFAULT 0,
  "last_order_date" date,
  "top_cuisines" jsonb,
  "updated_at" timestamptz NOT NULL DEFAULT (now())
);

-- 推荐结果表
CREATE TABLE "recommendations" (
  "id" bigserial PRIMARY KEY,
  "user_id" bigint NOT NULL,
  "dish_ids" bigint[],
  "combo_ids" bigint[],
  "merchant_ids" bigint[],
  "algorithm" text NOT NULL,
  "score" numeric(5,4),
  "generated_at" timestamptz NOT NULL DEFAULT (now()),
  "expired_at" timestamptz NOT NULL
);

-- 推荐配置表（区域级别）
CREATE TABLE "recommendation_configs" (
  "id" bigserial PRIMARY KEY,
  "region_id" bigint UNIQUE NOT NULL,
  "exploitation_ratio" numeric(3,2) NOT NULL DEFAULT 0.60,
  "exploration_ratio" numeric(3,2) NOT NULL DEFAULT 0.30,
  "random_ratio" numeric(3,2) NOT NULL DEFAULT 0.10,
  "auto_adjust" boolean NOT NULL DEFAULT false,
  "updated_at" timestamptz NOT NULL DEFAULT (now())
);

-- 外键约束
ALTER TABLE "user_behaviors" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id");
ALTER TABLE "user_behaviors" ADD FOREIGN KEY ("dish_id") REFERENCES "dishes" ("id");
ALTER TABLE "user_behaviors" ADD FOREIGN KEY ("combo_id") REFERENCES "combo_sets" ("id");
ALTER TABLE "user_behaviors" ADD FOREIGN KEY ("merchant_id") REFERENCES "merchants" ("id");

ALTER TABLE "user_preferences" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id");

ALTER TABLE "recommendations" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id");

ALTER TABLE "recommendation_configs" ADD FOREIGN KEY ("region_id") REFERENCES "regions" ("id");

-- 索引
CREATE INDEX ON "user_behaviors" ("user_id");
CREATE INDEX ON "user_behaviors" ("created_at");
CREATE INDEX ON "user_behaviors" ("user_id", "behavior_type");
CREATE INDEX ON "user_behaviors" ("user_id", "created_at");

CREATE UNIQUE INDEX ON "user_preferences" ("user_id");

CREATE INDEX ON "recommendations" ("user_id");
CREATE INDEX ON "recommendations" ("generated_at");
CREATE INDEX ON "recommendations" ("user_id", "generated_at");

CREATE UNIQUE INDEX ON "recommendation_configs" ("region_id");

-- 注释
COMMENT ON TABLE "user_behaviors" IS '用户行为埋点表：浏览、详情、加购、购买';
COMMENT ON COLUMN "user_behaviors"."behavior_type" IS 'view/detail/cart/purchase - 浏览列表/查看详情/加购/购买';
COMMENT ON COLUMN "user_behaviors"."duration" IS '停留时长(秒)，仅view/detail行为有值';

COMMENT ON TABLE "user_preferences" IS '用户偏好表：基于行为和消费分析';
COMMENT ON COLUMN "user_preferences"."cuisine_preferences" IS 'JSON格式：{"川菜": 0.8, "粤菜": 0.6} - 菜系偏好得分';
COMMENT ON COLUMN "user_preferences"."top_cuisines" IS 'JSON格式：{"川菜": 15, "粤菜": 8} - 购买次数统计';
COMMENT ON COLUMN "user_preferences"."favorite_time_slots" IS 'PostgreSQL数组：[11,12,18,19] - 常下单时段(小时)';

COMMENT ON TABLE "recommendations" IS '推荐结果表：缓存生成的推荐结果';
COMMENT ON COLUMN "recommendations"."algorithm" IS '使用的算法：collaborative/content-based/hybrid/ee-algorithm';
COMMENT ON COLUMN "recommendations"."expired_at" IS '推荐过期时间（通常5分钟后）';

COMMENT ON TABLE "recommendation_configs" IS '推荐配置表：区域级别EE算法配置';
COMMENT ON COLUMN "recommendation_configs"."exploitation_ratio" IS '喜好推荐比例 0.00-1.00，默认60%';
COMMENT ON COLUMN "recommendation_configs"."exploration_ratio" IS '探索推荐比例 0.00-1.00，默认30%';
COMMENT ON COLUMN "recommendation_configs"."random_ratio" IS '随机推荐比例 0.00-1.00，默认10%';
COMMENT ON COLUMN "recommendation_configs"."auto_adjust" IS '是否启用自动调整比例（基于成交转化率，M12功能）';
