ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_rider_amount_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_rider_amount_check
    CHECK (
        rider_amount >= 0
        AND (
            rider_id IS NULL
            OR rider_amount <= CASE
                WHEN rider_gross_amount > 0 THEN rider_gross_amount
                ELSE delivery_fee
            END
        )
    );
