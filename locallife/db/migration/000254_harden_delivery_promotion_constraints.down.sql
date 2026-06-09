ALTER TABLE merchant_delivery_promotions
    DROP CONSTRAINT IF EXISTS merchant_delivery_promotions_positive_amounts_check,
    DROP CONSTRAINT IF EXISTS merchant_delivery_promotions_discount_threshold_check,
    DROP CONSTRAINT IF EXISTS merchant_delivery_promotions_valid_period_check;
