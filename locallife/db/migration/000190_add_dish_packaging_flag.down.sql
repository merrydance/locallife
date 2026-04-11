DROP INDEX IF EXISTS idx_dishes_merchant_packaging_active;

ALTER TABLE dishes
DROP COLUMN IF EXISTS is_packaging;