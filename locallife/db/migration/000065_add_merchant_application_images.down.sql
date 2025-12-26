-- 移除门头照和环境照字段
ALTER TABLE "merchant_applications"
DROP COLUMN IF EXISTS "storefront_images",
DROP COLUMN IF EXISTS "environment_images";
