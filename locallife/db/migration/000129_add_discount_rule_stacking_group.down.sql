DROP INDEX IF EXISTS idx_discount_rules_stacking_group;
ALTER TABLE discount_rules DROP COLUMN IF EXISTS stacking_group;
