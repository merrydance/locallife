DROP INDEX IF EXISTS daily_inventory_reserved_idx;
ALTER TABLE daily_inventory DROP COLUMN IF EXISTS reserved_quantity;
