ALTER TABLE daily_inventory
ADD COLUMN reserved_quantity INT4 NOT NULL DEFAULT 0;

CREATE INDEX daily_inventory_reserved_idx ON daily_inventory (merchant_id, dish_id, date, reserved_quantity);
