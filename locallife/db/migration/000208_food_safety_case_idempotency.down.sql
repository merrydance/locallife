DROP INDEX IF EXISTS uq_food_safety_cases_merchant_open;

DROP INDEX IF EXISTS uq_food_safety_incidents_order_user_open;

ALTER TABLE food_safety_incidents
DROP CONSTRAINT IF EXISTS food_safety_incidents_status_check;

ALTER TABLE food_safety_incidents
ADD CONSTRAINT food_safety_incidents_status_check
CHECK (status IN ('reported', 'investigating', 'merchant-suspended', 'resolved'));