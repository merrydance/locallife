ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_rider_amount_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_rider_amount_check
    CHECK (rider_amount >= 0 AND (rider_id IS NULL OR rider_amount = delivery_fee));
