-- 回滚：移除商户申请表的区域字段

DROP INDEX IF EXISTS "idx_merchant_applications_region_id";

ALTER TABLE "merchant_applications" 
DROP CONSTRAINT IF EXISTS "fk_merchant_applications_region";

ALTER TABLE "merchant_applications" 
DROP COLUMN IF EXISTS "region_id";
