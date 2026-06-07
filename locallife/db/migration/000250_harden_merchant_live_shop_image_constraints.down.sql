ALTER TABLE "merchants"
DROP CONSTRAINT IF EXISTS "merchants_storefront_images_array_check",
DROP CONSTRAINT IF EXISTS "merchants_environment_images_array_check";

ALTER TABLE "merchants"
ADD CONSTRAINT "merchants_storefront_images_array_check"
  CHECK (
    storefront_images IS NULL
    OR (
      jsonb_typeof(storefront_images) = 'array'
      AND jsonb_array_length(storefront_images) <= 3
    )
  ),
ADD CONSTRAINT "merchants_environment_images_array_check"
  CHECK (
    environment_images IS NULL
    OR (
      jsonb_typeof(environment_images) = 'array'
      AND jsonb_array_length(environment_images) <= 5
    )
  );
