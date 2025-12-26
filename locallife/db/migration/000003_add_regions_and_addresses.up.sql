-- M2: 地理位置与区域管理
-- 严格遵循架构约束：bigint 自增主键 + text 类型

-- 创建 regions 表（省市区县四级地理数据）
CREATE TABLE "regions" (
  "id" bigserial PRIMARY KEY,
  "code" text UNIQUE NOT NULL,
  "name" text NOT NULL,
  "level" smallint NOT NULL,
  "parent_id" bigint,
  "longitude" decimal(10,7),
  "latitude" decimal(10,7),
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 创建 user_addresses 表（用户收货地址）
CREATE TABLE "user_addresses" (
  "id" bigserial PRIMARY KEY,
  "user_id" bigint NOT NULL,
  "region_id" bigint NOT NULL,
  "detail_address" text NOT NULL,
  "contact_name" text NOT NULL,
  "contact_phone" text NOT NULL,
  "longitude" decimal(10,7) NOT NULL,
  "latitude" decimal(10,7) NOT NULL,
  "is_default" boolean NOT NULL DEFAULT false,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 添加外键约束
ALTER TABLE "regions" ADD FOREIGN KEY ("parent_id") REFERENCES "regions" ("id");
ALTER TABLE "user_addresses" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE;
ALTER TABLE "user_addresses" ADD FOREIGN KEY ("region_id") REFERENCES "regions" ("id");

-- 创建索引
CREATE INDEX ON "regions" ("parent_id");
CREATE UNIQUE INDEX ON "regions" ("code");
CREATE INDEX ON "regions" ("longitude", "latitude");

CREATE INDEX ON "user_addresses" ("user_id");
CREATE INDEX ON "user_addresses" ("user_id", "is_default");

-- 添加注释
COMMENT ON COLUMN "regions"."code" IS '行政区划代码';
COMMENT ON COLUMN "regions"."level" IS '1=省 2=市 3=区 4=县';
