-- 扩展商户申请表，支持OCR识别和草稿功能
-- 添加食品经营许可证
ALTER TABLE "merchant_applications"
ADD COLUMN "food_permit_url" text,
ADD COLUMN "food_permit_ocr" jsonb;

-- 添加OCR识别结果存储
ALTER TABLE "merchant_applications"
ADD COLUMN "business_license_ocr" jsonb,
ADD COLUMN "id_card_front_ocr" jsonb,
ADD COLUMN "id_card_back_ocr" jsonb;

-- 将现有pending状态的记录更新为submitted（在添加约束之前）
UPDATE "merchant_applications" SET status = 'submitted' WHERE status = 'pending';

-- 修改状态约束，支持草稿模式
-- 先删除旧的约束（如果存在）
ALTER TABLE "merchant_applications"
DROP CONSTRAINT IF EXISTS "merchant_applications_status_check";

-- 添加新的状态约束: draft, submitted, approved, rejected
ALTER TABLE "merchant_applications"
ADD CONSTRAINT "merchant_applications_status_check"
CHECK (status IN ('draft', 'submitted', 'approved', 'rejected'));

-- 修改默认状态为draft
ALTER TABLE "merchant_applications"
ALTER COLUMN "status" SET DEFAULT 'draft';

-- 添加注释
COMMENT ON COLUMN "merchant_applications"."food_permit_url" IS '食品经营许可证图片URL';
COMMENT ON COLUMN "merchant_applications"."food_permit_ocr" IS '食品经营许可证OCR识别结果JSON';
COMMENT ON COLUMN "merchant_applications"."business_license_ocr" IS '营业执照OCR识别结果JSON';
COMMENT ON COLUMN "merchant_applications"."id_card_front_ocr" IS '身份证正面OCR识别结果JSON';
COMMENT ON COLUMN "merchant_applications"."id_card_back_ocr" IS '身份证背面OCR识别结果JSON';
COMMENT ON COLUMN "merchant_applications"."status" IS 'draft=草稿, submitted=已提交(待审核), approved=已通过, rejected=已拒绝';
