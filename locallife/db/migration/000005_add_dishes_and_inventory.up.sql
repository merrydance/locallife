-- M4: 菜品与库存管理

-- 食材表（全站共享，系统内置 + 商户添加）
CREATE TABLE "ingredients" (
  "id" bigserial PRIMARY KEY,
  "name" text NOT NULL,
  "is_system" boolean NOT NULL DEFAULT false,
  "category" text,
  "is_allergen" boolean NOT NULL DEFAULT false,
  "created_by" bigint,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE UNIQUE INDEX ON "ingredients" ("name");
CREATE INDEX ON "ingredients" ("category");
CREATE INDEX ON "ingredients" ("is_system");

-- 菜品分类表
CREATE TABLE "dish_categories" (
  "id" bigserial PRIMARY KEY,
  "merchant_id" bigint NOT NULL,
  "name" text NOT NULL,
  "sort_order" smallint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX ON "dish_categories" ("merchant_id");
CREATE INDEX ON "dish_categories" ("merchant_id", "sort_order");

-- 菜品表
CREATE TABLE "dishes" (
  "id" bigserial PRIMARY KEY,
  "merchant_id" bigint NOT NULL,
  "category_id" bigint,
  "name" text NOT NULL,
  "description" text,
  "image_url" text,
  "price" bigint NOT NULL,
  "member_price" bigint,
  "is_available" boolean NOT NULL DEFAULT true,
  "is_online" boolean NOT NULL DEFAULT true,
  "sort_order" smallint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  "updated_at" timestamptz
);

CREATE INDEX ON "dishes" ("merchant_id");
CREATE INDEX ON "dishes" ("category_id");
CREATE INDEX ON "dishes" ("merchant_id", "is_online", "is_available");
CREATE INDEX ON "dishes" ("name");

-- 菜品食材关联表（多对多）
CREATE TABLE "dish_ingredients" (
  "id" bigserial PRIMARY KEY,
  "dish_id" bigint NOT NULL,
  "ingredient_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX ON "dish_ingredients" ("dish_id");
CREATE INDEX ON "dish_ingredients" ("ingredient_id");
CREATE UNIQUE INDEX ON "dish_ingredients" ("dish_id", "ingredient_id");

-- 菜品标签关联表（多对多）
CREATE TABLE "dish_tags" (
  "id" bigserial PRIMARY KEY,
  "dish_id" bigint NOT NULL,
  "tag_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX ON "dish_tags" ("dish_id");
CREATE INDEX ON "dish_tags" ("tag_id");
CREATE UNIQUE INDEX ON "dish_tags" ("dish_id", "tag_id");

-- 菜品定制选项组表
CREATE TABLE "dish_customization_groups" (
  "id" bigserial PRIMARY KEY,
  "dish_id" bigint NOT NULL,
  "name" text NOT NULL,
  "is_required" boolean NOT NULL DEFAULT false,
  "sort_order" smallint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX ON "dish_customization_groups" ("dish_id");
CREATE INDEX ON "dish_customization_groups" ("dish_id", "sort_order");

-- 菜品定制选项表
CREATE TABLE "dish_customization_options" (
  "id" bigserial PRIMARY KEY,
  "group_id" bigint NOT NULL,
  "tag_id" bigint NOT NULL,
  "extra_price" bigint NOT NULL DEFAULT 0,
  "sort_order" smallint NOT NULL DEFAULT 0
);

CREATE INDEX ON "dish_customization_options" ("group_id");
CREATE UNIQUE INDEX ON "dish_customization_options" ("group_id", "tag_id");
CREATE INDEX ON "dish_customization_options" ("group_id", "sort_order");

-- 套餐主表
CREATE TABLE "combo_sets" (
  "id" bigserial PRIMARY KEY,
  "merchant_id" bigint NOT NULL,
  "name" text NOT NULL,
  "description" text,
  "image_url" text,
  "original_price" bigint NOT NULL,
  "combo_price" bigint NOT NULL,
  "is_online" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  "updated_at" timestamptz
);

CREATE INDEX ON "combo_sets" ("merchant_id");
CREATE INDEX ON "combo_sets" ("merchant_id", "is_online");

-- 套餐标签关联表（多对多）
CREATE TABLE "combo_tags" (
  "id" bigserial PRIMARY KEY,
  "combo_id" bigint NOT NULL,
  "tag_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX ON "combo_tags" ("combo_id");
CREATE INDEX ON "combo_tags" ("tag_id");
CREATE UNIQUE INDEX ON "combo_tags" ("combo_id", "tag_id");

-- 套餐包含的菜品（多对多）
CREATE TABLE "combo_dishes" (
  "id" bigserial PRIMARY KEY,
  "combo_id" bigint NOT NULL,
  "dish_id" bigint NOT NULL,
  "quantity" smallint NOT NULL DEFAULT 1
);

CREATE INDEX ON "combo_dishes" ("combo_id");
CREATE UNIQUE INDEX ON "combo_dishes" ("combo_id", "dish_id");

-- 每日库存表
CREATE TABLE "daily_inventory" (
  "id" bigserial PRIMARY KEY,
  "merchant_id" bigint NOT NULL,
  "dish_id" bigint NOT NULL,
  "date" date NOT NULL,
  "total_quantity" int NOT NULL DEFAULT -1,
  "sold_quantity" int NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  "updated_at" timestamptz
);

CREATE UNIQUE INDEX ON "daily_inventory" ("merchant_id", "dish_id", "date");
CREATE INDEX ON "daily_inventory" ("date");
CREATE INDEX ON "daily_inventory" ("merchant_id");

-- 外键约束
ALTER TABLE "ingredients" ADD FOREIGN KEY ("created_by") REFERENCES "users" ("id");

ALTER TABLE "dish_categories" ADD FOREIGN KEY ("merchant_id") REFERENCES "merchants" ("id") ON DELETE CASCADE;

ALTER TABLE "dishes" ADD FOREIGN KEY ("merchant_id") REFERENCES "merchants" ("id") ON DELETE CASCADE;
ALTER TABLE "dishes" ADD FOREIGN KEY ("category_id") REFERENCES "dish_categories" ("id") ON DELETE SET NULL;

ALTER TABLE "dish_ingredients" ADD FOREIGN KEY ("dish_id") REFERENCES "dishes" ("id") ON DELETE CASCADE;
ALTER TABLE "dish_ingredients" ADD FOREIGN KEY ("ingredient_id") REFERENCES "ingredients" ("id") ON DELETE CASCADE;

ALTER TABLE "dish_tags" ADD FOREIGN KEY ("dish_id") REFERENCES "dishes" ("id") ON DELETE CASCADE;
ALTER TABLE "dish_tags" ADD FOREIGN KEY ("tag_id") REFERENCES "tags" ("id") ON DELETE CASCADE;

ALTER TABLE "dish_customization_groups" ADD FOREIGN KEY ("dish_id") REFERENCES "dishes" ("id") ON DELETE CASCADE;

ALTER TABLE "dish_customization_options" ADD FOREIGN KEY ("group_id") REFERENCES "dish_customization_groups" ("id") ON DELETE CASCADE;
ALTER TABLE "dish_customization_options" ADD FOREIGN KEY ("tag_id") REFERENCES "tags" ("id") ON DELETE CASCADE;

ALTER TABLE "combo_sets" ADD FOREIGN KEY ("merchant_id") REFERENCES "merchants" ("id") ON DELETE CASCADE;

ALTER TABLE "combo_tags" ADD FOREIGN KEY ("combo_id") REFERENCES "combo_sets" ("id") ON DELETE CASCADE;
ALTER TABLE "combo_tags" ADD FOREIGN KEY ("tag_id") REFERENCES "tags" ("id") ON DELETE CASCADE;

ALTER TABLE "combo_dishes" ADD FOREIGN KEY ("combo_id") REFERENCES "combo_sets" ("id") ON DELETE CASCADE;
ALTER TABLE "combo_dishes" ADD FOREIGN KEY ("dish_id") REFERENCES "dishes" ("id") ON DELETE CASCADE;

ALTER TABLE "daily_inventory" ADD FOREIGN KEY ("merchant_id") REFERENCES "merchants" ("id") ON DELETE CASCADE;
ALTER TABLE "daily_inventory" ADD FOREIGN KEY ("dish_id") REFERENCES "dishes" ("id") ON DELETE CASCADE;
