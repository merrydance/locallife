-- Rollback dish categories refactor

-- 1. Re-add columns to dish_categories (this is tricky as data is now shared)
ALTER TABLE "dish_categories" DROP CONSTRAINT "dish_categories_name_key";
ALTER TABLE "dish_categories" ADD COLUMN "merchant_id" bigint;
ALTER TABLE "dish_categories" ADD COLUMN "sort_order" smallint DEFAULT 0;

-- 2. Restore individual records per merchant
-- This won't perfectly restore old IDs, but it restores the structure/logic
INSERT INTO "dish_categories" ("merchant_id", "name", "sort_order")
SELECT mdc."merchant_id", c."name", mdc."sort_order"
FROM "merchant_dish_categories" mdc
JOIN "dish_categories" c ON mdc."category_id" = c."id";

-- 3. Update dishes is hard here because we've generated new records.
-- In a real rollback, we'd need to map them back.
-- For simplicity in this demo environment:
UPDATE "dishes" d
SET "category_id" = c2."id"
FROM "merchant_dish_categories" mdc
JOIN "dish_categories" c1 ON mdc."category_id" = c1."id"
JOIN "dish_categories" c2 ON mdc."merchant_id" = c2."merchant_id" AND c1."name" = c2."name"
WHERE d."category_id" = c1."id" AND d."merchant_id" = mdc."merchant_id";

-- 4. Clean up shared records (those with merchant_id NULL)
DELETE FROM "dish_categories" WHERE "merchant_id" IS NULL;

-- 5. Restore Foreign Key and Not Null
ALTER TABLE "dish_categories" ALTER COLUMN "merchant_id" SET NOT NULL;
ALTER TABLE "dish_categories" ADD FOREIGN KEY ("merchant_id") REFERENCES "merchants" ("id") ON DELETE CASCADE;

-- 6. Drop junction table
DROP TABLE IF EXISTS "merchant_dish_categories";
