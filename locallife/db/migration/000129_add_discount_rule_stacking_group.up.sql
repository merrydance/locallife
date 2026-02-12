ALTER TABLE discount_rules
ADD COLUMN stacking_group TEXT;

CREATE INDEX idx_discount_rules_stacking_group
ON discount_rules(stacking_group)
WHERE stacking_group IS NOT NULL;
