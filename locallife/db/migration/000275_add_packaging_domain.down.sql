ALTER TABLE orders
DROP COLUMN IF EXISTS packaging_fee;

DROP TABLE IF EXISTS order_packaging_items;
DROP TABLE IF EXISTS cart_packaging_selections;
DROP TABLE IF EXISTS merchant_packaging_options;
DROP TABLE IF EXISTS merchant_packaging_settings;
