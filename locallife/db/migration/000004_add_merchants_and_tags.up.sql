-- 创建标签表（全站通用）
CREATE TABLE "tags" (
  "id" bigserial PRIMARY KEY,
  "name" text NOT NULL,
  "type" text NOT NULL,
  "sort_order" smallint NOT NULL DEFAULT 0,
  "status" text NOT NULL DEFAULT 'active',
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 创建商户表
CREATE TABLE "merchants" (
  "id" bigserial PRIMARY KEY,
  "owner_user_id" bigint NOT NULL,
  "name" text NOT NULL,
  "description" text,
  "logo_url" text,
  "phone" text NOT NULL,
  "address" text NOT NULL,
  "latitude" decimal(10,7),
  "longitude" decimal(10,7),
  "status" text NOT NULL DEFAULT 'pending',
  "application_data" jsonb,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  "updated_at" timestamptz NOT NULL DEFAULT (now())
);

-- 创建商户标签关联表（多对多）
CREATE TABLE "merchant_tags" (
  "merchant_id" bigint NOT NULL,
  "tag_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  PRIMARY KEY ("merchant_id", "tag_id")
);

-- 创建商户营业时间表
CREATE TABLE "merchant_business_hours" (
  "id" bigserial PRIMARY KEY,
  "merchant_id" bigint NOT NULL,
  "day_of_week" int NOT NULL,
  "open_time" time NOT NULL,
  "close_time" time NOT NULL,
  "is_closed" boolean NOT NULL DEFAULT false,
  "special_date" date,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  "updated_at" timestamptz NOT NULL DEFAULT (now())
);

-- 创建商户入驻申请表
CREATE TABLE "merchant_applications" (
  "id" bigserial PRIMARY KEY,
  "user_id" bigint NOT NULL,
  "merchant_name" text NOT NULL,
  "business_license_number" text NOT NULL,
  "business_license_image_url" text NOT NULL,
  "legal_person_name" text NOT NULL,
  "legal_person_id_number" text NOT NULL,
  "legal_person_id_front_url" text NOT NULL,
  "legal_person_id_back_url" text NOT NULL,
  "contact_phone" text NOT NULL,
  "business_address" text NOT NULL,
  "business_scope" text,
  "status" text NOT NULL DEFAULT 'pending',
  "reject_reason" text,
  "reviewed_by" bigint,
  "reviewed_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  "updated_at" timestamptz NOT NULL DEFAULT (now())
);

-- 添加外键约束
ALTER TABLE "merchants" ADD FOREIGN KEY ("owner_user_id") REFERENCES "users" ("id") ON DELETE RESTRICT;

ALTER TABLE "merchant_tags" ADD FOREIGN KEY ("merchant_id") REFERENCES "merchants" ("id") ON DELETE CASCADE;

ALTER TABLE "merchant_tags" ADD FOREIGN KEY ("tag_id") REFERENCES "tags" ("id") ON DELETE CASCADE;

ALTER TABLE "merchant_business_hours" ADD FOREIGN KEY ("merchant_id") REFERENCES "merchants" ("id") ON DELETE CASCADE;

ALTER TABLE "merchant_applications" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE RESTRICT;

ALTER TABLE "merchant_applications" ADD FOREIGN KEY ("reviewed_by") REFERENCES "users" ("id") ON DELETE SET NULL;

-- 创建索引
CREATE INDEX ON "tags" ("type");

CREATE INDEX ON "tags" ("type", "name");

CREATE INDEX ON "merchants" ("owner_user_id");

CREATE INDEX ON "merchants" ("status");

CREATE UNIQUE INDEX ON "tags" ("name", "type");

CREATE INDEX ON "tags" ("type");

CREATE INDEX ON "tags" ("status");

CREATE INDEX ON "merchant_business_hours" ("merchant_id");

CREATE INDEX ON "merchant_business_hours" ("merchant_id", "day_of_week");

CREATE INDEX ON "merchant_business_hours" ("merchant_id", "special_date");

CREATE INDEX ON "merchant_applications" ("user_id");

CREATE INDEX ON "merchant_applications" ("status");

CREATE INDEX ON "merchant_applications" ("business_license_number");

-- 添加注释
COMMENT ON COLUMN "tags"."type" IS 'merchant, product, service, etc.';

COMMENT ON COLUMN "merchants"."status" IS 'pending, approved, rejected, suspended';

COMMENT ON COLUMN "merchants"."application_data" IS 'JSON存储原始申请数据（营业执照信息等）';

COMMENT ON COLUMN "merchant_business_hours"."day_of_week" IS '0=Sunday, 1=Monday, ..., 6=Saturday';

COMMENT ON COLUMN "merchant_business_hours"."special_date" IS '特殊日期覆盖常规营业时间，如节假日';

COMMENT ON COLUMN "merchant_applications"."status" IS 'pending, approved, rejected';
