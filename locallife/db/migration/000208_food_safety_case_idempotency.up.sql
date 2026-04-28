ALTER TABLE food_safety_incidents
DROP CONSTRAINT IF EXISTS food_safety_incidents_status_check;

ALTER TABLE food_safety_incidents
ADD CONSTRAINT food_safety_incidents_status_check
CHECK (status IN ('reported', 'rejected', 'investigating', 'merchant-suspended', 'resolved'));

CREATE UNIQUE INDEX IF NOT EXISTS uq_food_safety_incidents_order_user_open
ON food_safety_incidents (order_id, user_id)
WHERE status IN ('reported', 'investigating', 'merchant-suspended');

CREATE UNIQUE INDEX IF NOT EXISTS uq_food_safety_cases_merchant_open
ON food_safety_cases (merchant_id)
WHERE status IN ('merchant-suspended', 'investigating');