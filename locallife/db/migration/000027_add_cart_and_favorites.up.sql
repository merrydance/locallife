-- 购物车表（每个用户每个商户一个购物车）
CREATE TABLE IF NOT EXISTS "carts" (
  "id" bigserial PRIMARY KEY,
  "user_id" bigint NOT NULL REFERENCES "users"("id"),
  "merchant_id" bigint NOT NULL REFERENCES "merchants"("id"),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ON "carts" ("user_id", "merchant_id");
CREATE INDEX ON "carts" ("user_id");
CREATE INDEX ON "carts" ("merchant_id");

COMMENT ON TABLE "carts" IS '购物车主表，每个用户每个商户一个购物车';
COMMENT ON COLUMN "carts"."user_id" IS '用户ID';
COMMENT ON COLUMN "carts"."merchant_id" IS '商户ID';

-- 购物车商品表
CREATE TABLE IF NOT EXISTS "cart_items" (
  "id" bigserial PRIMARY KEY,
  "cart_id" bigint NOT NULL REFERENCES "carts"("id") ON DELETE CASCADE,
  "dish_id" bigint REFERENCES "dishes"("id") ON DELETE CASCADE,
  "combo_id" bigint REFERENCES "combo_sets"("id") ON DELETE CASCADE,
  "quantity" smallint NOT NULL DEFAULT 1,
  "customizations" jsonb,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "cart_items_dish_or_combo_check" CHECK (
    (dish_id IS NOT NULL AND combo_id IS NULL) OR
    (dish_id IS NULL AND combo_id IS NOT NULL)
  )
);

CREATE INDEX ON "cart_items" ("cart_id");
CREATE INDEX ON "cart_items" ("dish_id");
CREATE INDEX ON "cart_items" ("combo_id");

COMMENT ON TABLE "cart_items" IS '购物车商品表';
COMMENT ON COLUMN "cart_items"."cart_id" IS '购物车ID';
COMMENT ON COLUMN "cart_items"."dish_id" IS '菜品ID，与combo_id二选一';
COMMENT ON COLUMN "cart_items"."combo_id" IS '套餐ID，与dish_id二选一';
COMMENT ON COLUMN "cart_items"."quantity" IS '数量';
COMMENT ON COLUMN "cart_items"."customizations" IS '定制选项，如 [{"name":"辣度","value":"微辣","extra_price":0}]';

-- 用户收藏表（支持收藏商户和菜品）
CREATE TABLE IF NOT EXISTS "favorites" (
  "id" bigserial PRIMARY KEY,
  "user_id" bigint NOT NULL REFERENCES "users"("id"),
  "favorite_type" text NOT NULL,
  "merchant_id" bigint REFERENCES "merchants"("id") ON DELETE CASCADE,
  "dish_id" bigint REFERENCES "dishes"("id") ON DELETE CASCADE,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "favorites_type_check" CHECK (
    favorite_type IN ('merchant', 'dish')
  ),
  CONSTRAINT "favorites_target_check" CHECK (
    (favorite_type = 'merchant' AND merchant_id IS NOT NULL AND dish_id IS NULL) OR
    (favorite_type = 'dish' AND dish_id IS NOT NULL AND merchant_id IS NULL)
  )
);

CREATE UNIQUE INDEX ON "favorites" ("user_id", "favorite_type", "merchant_id") WHERE merchant_id IS NOT NULL;
CREATE UNIQUE INDEX ON "favorites" ("user_id", "favorite_type", "dish_id") WHERE dish_id IS NOT NULL;
CREATE INDEX ON "favorites" ("user_id", "favorite_type");
CREATE INDEX ON "favorites" ("merchant_id") WHERE merchant_id IS NOT NULL;
CREATE INDEX ON "favorites" ("dish_id") WHERE dish_id IS NOT NULL;

COMMENT ON TABLE "favorites" IS '用户收藏表';
COMMENT ON COLUMN "favorites"."user_id" IS '用户ID';
COMMENT ON COLUMN "favorites"."favorite_type" IS '收藏类型：merchant=商户, dish=菜品';
COMMENT ON COLUMN "favorites"."merchant_id" IS '收藏的商户ID';
COMMENT ON COLUMN "favorites"."dish_id" IS '收藏的菜品ID';
