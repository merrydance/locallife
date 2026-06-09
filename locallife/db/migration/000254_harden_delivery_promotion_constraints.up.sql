UPDATE merchant_delivery_promotions
SET
    min_order_amount = GREATEST(min_order_amount, 1),
    discount_amount = LEAST(GREATEST(discount_amount, 1), GREATEST(min_order_amount, 1)),
    valid_until = CASE
        WHEN valid_until <= valid_from THEN valid_from + INTERVAL '1 second'
        ELSE valid_until
    END,
    updated_at = now()
WHERE min_order_amount <= 0
   OR discount_amount <= 0
   OR discount_amount > min_order_amount
   OR valid_until <= valid_from;

ALTER TABLE merchant_delivery_promotions
    ADD CONSTRAINT merchant_delivery_promotions_positive_amounts_check
        CHECK (min_order_amount > 0 AND discount_amount > 0),
    ADD CONSTRAINT merchant_delivery_promotions_discount_threshold_check
        CHECK (discount_amount <= min_order_amount),
    ADD CONSTRAINT merchant_delivery_promotions_valid_period_check
        CHECK (valid_until > valid_from);
