ALTER TABLE "merchants"
ADD COLUMN "storefront_images" jsonb,
ADD COLUMN "environment_images" jsonb;

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

COMMENT ON COLUMN "merchants"."storefront_images" IS 'Live merchant storefront image URL array JSON,最多3张';
COMMENT ON COLUMN "merchants"."environment_images" IS 'Live merchant environment image URL array JSON,最多5张';

WITH single_merchant_owner AS (
  SELECT owner_user_id AS user_id
  FROM merchants
  WHERE deleted_at IS NULL
  GROUP BY owner_user_id
  HAVING COUNT(*) = 1
),
latest_application AS (
  SELECT DISTINCT ON (user_id)
    user_id,
    CASE
      WHEN jsonb_typeof(storefront_images) = 'array' THEN
        CASE
          WHEN jsonb_array_length(storefront_images) <= 3
            AND NOT jsonb_path_exists(storefront_images, '$[*] ? (@.type() != "string")')
            THEN storefront_images
          ELSE NULL
        END
      ELSE NULL
    END AS storefront_images,
    CASE
      WHEN jsonb_typeof(environment_images) = 'array' THEN
        CASE
          WHEN jsonb_array_length(environment_images) <= 5
            AND NOT jsonb_path_exists(environment_images, '$[*] ? (@.type() != "string")')
            THEN environment_images
          ELSE NULL
        END
      ELSE NULL
    END AS environment_images
  FROM merchant_applications
  WHERE status = 'approved'
  ORDER BY user_id, created_at DESC, id DESC
)
UPDATE merchants m
SET
  storefront_images = COALESCE(m.storefront_images, latest_application.storefront_images),
  environment_images = COALESCE(m.environment_images, latest_application.environment_images),
  updated_at = now()
FROM latest_application
JOIN single_merchant_owner ON single_merchant_owner.user_id = latest_application.user_id
WHERE latest_application.user_id = m.owner_user_id
  AND m.deleted_at IS NULL
  AND (
    (m.storefront_images IS NULL AND latest_application.storefront_images IS NOT NULL)
    OR (m.environment_images IS NULL AND latest_application.environment_images IS NOT NULL)
  );
