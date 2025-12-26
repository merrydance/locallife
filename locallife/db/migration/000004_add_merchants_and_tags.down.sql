-- 删除索引
DROP INDEX IF EXISTS "idx_merchant_applications_business_license_number";
DROP INDEX IF EXISTS "idx_merchant_applications_status";
DROP INDEX IF EXISTS "idx_merchant_applications_user_id";
DROP INDEX IF EXISTS "idx_merchant_business_hours_merchant_id_special_date";
DROP INDEX IF EXISTS "idx_merchant_business_hours_merchant_id_day_of_week";
DROP INDEX IF EXISTS "idx_merchant_business_hours_merchant_id";
DROP INDEX IF EXISTS "idx_merchants_status";
DROP INDEX IF EXISTS "idx_merchants_owner_user_id";
DROP INDEX IF EXISTS "idx_tags_type_name";
DROP INDEX IF EXISTS "idx_tags_type";

-- 删除外键约束（级联删除会自动处理）
-- 删除表
DROP TABLE IF EXISTS "merchant_applications";
DROP TABLE IF EXISTS "merchant_business_hours";
DROP TABLE IF EXISTS "merchant_tags";
DROP TABLE IF EXISTS "merchants";
DROP TABLE IF EXISTS "tags";
