-- 添加门头照和环境照字段（使用 jsonb 存储多张图片URL数组）
ALTER TABLE "merchant_applications"
ADD COLUMN "storefront_images" jsonb,
ADD COLUMN "environment_images" jsonb;

COMMENT ON COLUMN "merchant_applications"."storefront_images" IS '门头照片URL数组 JSON，最多3张';
COMMENT ON COLUMN "merchant_applications"."environment_images" IS '店内环境照片URL数组 JSON，最多5张';
