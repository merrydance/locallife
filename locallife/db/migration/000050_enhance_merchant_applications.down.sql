-- 恢复商户申请表到原始状态

-- 恢复pending状态的记录
UPDATE "merchant_applications" SET status = 'pending' WHERE status = 'submitted';

-- 删除新增的列
ALTER TABLE "merchant_applications"
DROP COLUMN IF EXISTS "food_permit_url",
DROP COLUMN IF EXISTS "food_permit_ocr",
DROP COLUMN IF EXISTS "business_license_ocr",
DROP COLUMN IF EXISTS "id_card_front_ocr",
DROP COLUMN IF EXISTS "id_card_back_ocr";

-- 删除新的状态约束
ALTER TABLE "merchant_applications"
DROP CONSTRAINT IF EXISTS "merchant_applications_status_check";

-- 恢复默认状态为pending
ALTER TABLE "merchant_applications"
ALTER COLUMN "status" SET DEFAULT 'pending';
