-- Rollback order_type, table_id, and reservation_id from carts
DROP INDEX IF EXISTS "carts_user_id_merchant_id_order_type_idx";
CREATE UNIQUE INDEX "carts_user_id_merchant_id_idx" ON "carts" ("user_id", "merchant_id");

ALTER TABLE "carts" DROP COLUMN IF EXISTS "reservation_id";
ALTER TABLE "carts" DROP COLUMN IF EXISTS "table_id";
ALTER TABLE "carts" DROP COLUMN IF EXISTS "order_type";
