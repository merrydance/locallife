-- Add order_type, table_id, and reservation_id to carts
ALTER TABLE "carts" ADD COLUMN "order_type" text NOT NULL DEFAULT 'takeout';
ALTER TABLE "carts" ADD COLUMN "table_id" bigint;
ALTER TABLE "carts" ADD COLUMN "reservation_id" bigint;

-- Update unique index to support different business scenarios
DROP INDEX IF EXISTS "carts_user_id_merchant_id_idx";
CREATE UNIQUE INDEX "carts_user_id_merchant_id_order_type_idx" ON "carts" ("user_id", "merchant_id", "order_type", "table_id", "reservation_id");

COMMENT ON COLUMN "carts"."order_type" IS '订单类型：takeout=外卖, dine_in=堂食, reservation=预订';
COMMENT ON COLUMN "carts"."table_id" IS '桌台ID（仅堂食有效）';
COMMENT ON COLUMN "carts"."reservation_id" IS '预订ID（仅预订有效）';
