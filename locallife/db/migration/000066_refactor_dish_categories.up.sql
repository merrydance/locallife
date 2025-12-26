-- Refactor dish categories to be global and unique

-- 1. Create a temporary table to store current category data
CREATE TABLE "dish_categories_temp" AS SELECT * FROM "dish_categories";

-- 2. Modify "dish_categories" to be global
-- Remove merchant-specific columns and add unique constraint
-- We keep the "id" to avoid breaking "dishes" references if possible, 
-- but we need to deduplicate names first.

-- First, empty the old table but keep the structure for a moment
DELETE FROM "dish_categories";

-- Add unique constraint on name
ALTER TABLE "dish_categories" DROP COLUMN "merchant_id";
ALTER TABLE "dish_categories" DROP COLUMN "sort_order";
ALTER TABLE "dish_categories" ADD CONSTRAINT "dish_categories_name_key" UNIQUE ("name");

-- 3. Populate global "dish_categories" with unique names
INSERT INTO "dish_categories" ("name")
SELECT DISTINCT "name" FROM "dish_categories_temp"
ON CONFLICT (name) DO NOTHING;

-- 4. Create junction table for merchant associations
CREATE TABLE "merchant_dish_categories" (
  "merchant_id" bigint NOT NULL,
  "category_id" bigint NOT NULL,
  "sort_order" smallint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  PRIMARY KEY ("merchant_id", "category_id")
);

ALTER TABLE "merchant_dish_categories" ADD FOREIGN KEY ("merchant_id") REFERENCES "merchants" ("id") ON DELETE CASCADE;
ALTER TABLE "merchant_dish_categories" ADD FOREIGN KEY ("category_id") REFERENCES "dish_categories" ("id") ON DELETE CASCADE;

CREATE INDEX ON "merchant_dish_categories" ("merchant_id", "sort_order");

-- 5. Migrate existing merchant-category associations
INSERT INTO "merchant_dish_categories" ("merchant_id", "category_id", "sort_order", "created_at")
SELECT t."merchant_id", c."id", t."sort_order", t."created_at"
FROM "dish_categories_temp" t
JOIN "dish_categories" c ON t."name" = c."name";

-- 6. Update "dishes" to point to new global IDs
-- Since we might have different old IDs for same name, we must update dishes
UPDATE "dishes" d
SET "category_id" = c."id"
FROM "dish_categories_temp" t
JOIN "dish_categories" c ON t."name" = c."name"
WHERE d."category_id" = t."id";

-- 7. Drop temp table
DROP TABLE "dish_categories_temp";
