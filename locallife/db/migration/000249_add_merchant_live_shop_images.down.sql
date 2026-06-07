ALTER TABLE "merchants"
DROP CONSTRAINT IF EXISTS "merchants_storefront_images_array_check",
DROP CONSTRAINT IF EXISTS "merchants_environment_images_array_check",
DROP COLUMN IF EXISTS "storefront_images",
DROP COLUMN IF EXISTS "environment_images";
