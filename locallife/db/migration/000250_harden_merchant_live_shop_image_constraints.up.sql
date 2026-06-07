ALTER TABLE "merchants"
DROP CONSTRAINT IF EXISTS "merchants_storefront_images_array_check",
DROP CONSTRAINT IF EXISTS "merchants_environment_images_array_check";

UPDATE "merchants"
SET
  storefront_images = CASE
    WHEN storefront_images IS NULL THEN NULL
    WHEN jsonb_typeof(storefront_images) = 'array' THEN
      CASE
        WHEN jsonb_array_length(storefront_images) <= 3
          AND NOT jsonb_path_exists(storefront_images, '$[*] ? (@.type() != "string")')
          THEN storefront_images
        ELSE NULL
      END
    ELSE NULL
  END,
  environment_images = CASE
    WHEN environment_images IS NULL THEN NULL
    WHEN jsonb_typeof(environment_images) = 'array' THEN
      CASE
        WHEN jsonb_array_length(environment_images) <= 5
          AND NOT jsonb_path_exists(environment_images, '$[*] ? (@.type() != "string")')
          THEN environment_images
        ELSE NULL
      END
    ELSE NULL
  END,
  updated_at = now()
WHERE
  CASE
    WHEN storefront_images IS NULL THEN false
    WHEN jsonb_typeof(storefront_images) = 'array' THEN
      CASE
        WHEN jsonb_array_length(storefront_images) <= 3
          AND NOT jsonb_path_exists(storefront_images, '$[*] ? (@.type() != "string")')
          THEN false
        ELSE true
      END
    ELSE true
  END
  OR
  CASE
    WHEN environment_images IS NULL THEN false
    WHEN jsonb_typeof(environment_images) = 'array' THEN
      CASE
        WHEN jsonb_array_length(environment_images) <= 5
          AND NOT jsonb_path_exists(environment_images, '$[*] ? (@.type() != "string")')
          THEN false
        ELSE true
      END
    ELSE true
  END;

ALTER TABLE "merchants"
ADD CONSTRAINT "merchants_storefront_images_array_check"
  CHECK (
    CASE
      WHEN storefront_images IS NULL THEN true
      WHEN jsonb_typeof(storefront_images) = 'array' THEN
        jsonb_array_length(storefront_images) <= 3
        AND NOT jsonb_path_exists(storefront_images, '$[*] ? (@.type() != "string")')
      ELSE false
    END
  ),
ADD CONSTRAINT "merchants_environment_images_array_check"
  CHECK (
    CASE
      WHEN environment_images IS NULL THEN true
      WHEN jsonb_typeof(environment_images) = 'array' THEN
        jsonb_array_length(environment_images) <= 5
        AND NOT jsonb_path_exists(environment_images, '$[*] ? (@.type() != "string")')
      ELSE false
    END
  );
