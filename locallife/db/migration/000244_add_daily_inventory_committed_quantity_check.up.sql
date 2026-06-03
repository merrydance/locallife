UPDATE daily_inventory
SET
  total_quantity = sold_quantity + reserved_quantity,
  updated_at = now()
WHERE total_quantity <> -1
  AND total_quantity < sold_quantity + reserved_quantity;

ALTER TABLE daily_inventory
ADD CONSTRAINT daily_inventory_total_committed_quantity_check
CHECK (
  total_quantity = -1
  OR total_quantity >= sold_quantity + reserved_quantity
);
