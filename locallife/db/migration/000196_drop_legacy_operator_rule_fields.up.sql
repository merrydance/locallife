ALTER TABLE operators
DROP COLUMN IF EXISTS commission_rate,
DROP COLUMN IF EXISTS merchant_deposit;

ALTER TABLE region_rule_configs
DROP COLUMN IF EXISTS commission_rate,
DROP COLUMN IF EXISTS merchant_deposit;