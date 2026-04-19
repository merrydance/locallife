DROP INDEX IF EXISTS idx_food_safety_incidents_merchant_product_created_at;
DROP INDEX IF EXISTS idx_food_safety_incidents_case_id;

ALTER TABLE food_safety_incidents
    DROP COLUMN IF EXISTS case_id,
    DROP COLUMN IF EXISTS primary_product_label,
    DROP COLUMN IF EXISTS primary_product_key;

DROP INDEX IF EXISTS idx_food_safety_cases_merchant_product_status;
DROP INDEX IF EXISTS idx_food_safety_cases_merchant_status;
DROP INDEX IF EXISTS idx_food_safety_cases_region_status;

DROP TABLE IF EXISTS food_safety_cases;