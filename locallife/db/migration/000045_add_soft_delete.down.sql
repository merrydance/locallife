-- 回滚软删除字段

DROP INDEX IF EXISTS idx_discount_rules_deleted_at;
ALTER TABLE discount_rules DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_vouchers_deleted_at;
ALTER TABLE vouchers DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_dish_categories_deleted_at;
ALTER TABLE dish_categories DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_merchants_deleted_at;
ALTER TABLE merchants DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_combo_sets_deleted_at;
ALTER TABLE combo_sets DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_dishes_deleted_at;
ALTER TABLE dishes DROP COLUMN IF EXISTS deleted_at;
