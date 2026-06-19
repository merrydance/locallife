-- name: GetMerchantPackagingSettings :one
SELECT
  id,
  merchant_id,
  enabled,
  required,
  applicable_order_types,
  default_option_id,
  created_at,
  updated_at
FROM merchant_packaging_settings
WHERE merchant_id = $1
LIMIT 1;

-- name: UpsertMerchantPackagingSettings :one
INSERT INTO merchant_packaging_settings (
  merchant_id,
  enabled,
  required,
  applicable_order_types,
  default_option_id,
  updated_at
) VALUES (
  sqlc.arg('merchant_id'),
  sqlc.arg('enabled'),
  sqlc.arg('required'),
  sqlc.arg('applicable_order_types'),
  sqlc.narg('default_option_id'),
  now()
)
ON CONFLICT (merchant_id) DO UPDATE
SET
  enabled = EXCLUDED.enabled,
  required = EXCLUDED.required,
  applicable_order_types = EXCLUDED.applicable_order_types,
  default_option_id = EXCLUDED.default_option_id,
  updated_at = now()
RETURNING
  id,
  merchant_id,
  enabled,
  required,
  applicable_order_types,
  default_option_id,
  created_at,
  updated_at;

-- name: CreateMerchantPackagingOption :one
INSERT INTO merchant_packaging_options (
  merchant_id,
  legacy_dish_id,
  name,
  description,
  price,
  is_enabled,
  sort_order
) VALUES (
  sqlc.arg('merchant_id'),
  sqlc.narg('legacy_dish_id'),
  sqlc.arg('name'),
  sqlc.narg('description'),
  sqlc.arg('price'),
  sqlc.arg('is_enabled'),
  sqlc.arg('sort_order')
)
RETURNING
  id,
  merchant_id,
  legacy_dish_id,
  name,
  description,
  price,
  is_enabled,
  sort_order,
  deleted_at,
  created_at,
  updated_at;

-- name: GetMerchantPackagingOption :one
SELECT
  id,
  merchant_id,
  legacy_dish_id,
  name,
  description,
  price,
  is_enabled,
  sort_order,
  deleted_at,
  created_at,
  updated_at
FROM merchant_packaging_options
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id')
  AND deleted_at IS NULL
LIMIT 1;

-- name: GetMerchantPackagingOptionForUpdate :one
SELECT
  id,
  merchant_id,
  legacy_dish_id,
  name,
  description,
  price,
  is_enabled,
  sort_order,
  deleted_at,
  created_at,
  updated_at
FROM merchant_packaging_options
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id')
  AND deleted_at IS NULL
LIMIT 1
FOR UPDATE;

-- name: ClearMerchantPackagingDefaultOptionIfMatches :exec
UPDATE merchant_packaging_settings
SET
  default_option_id = NULL,
  updated_at = now()
WHERE merchant_id = sqlc.arg('merchant_id')
  AND default_option_id = sqlc.arg('default_option_id');

-- name: ListMerchantPackagingOptions :many
SELECT
  id,
  merchant_id,
  legacy_dish_id,
  name,
  description,
  price,
  is_enabled,
  sort_order,
  deleted_at,
  created_at,
  updated_at
FROM merchant_packaging_options
WHERE merchant_id = $1
  AND deleted_at IS NULL
ORDER BY sort_order ASC, id ASC;

-- name: ListEnabledMerchantPackagingOptions :many
SELECT
  id,
  merchant_id,
  legacy_dish_id,
  name,
  description,
  price,
  is_enabled,
  sort_order,
  deleted_at,
  created_at,
  updated_at
FROM merchant_packaging_options
WHERE merchant_id = $1
  AND is_enabled = true
  AND deleted_at IS NULL
ORDER BY sort_order ASC, id ASC;

-- name: UpdateMerchantPackagingOption :one
UPDATE merchant_packaging_options
SET
  name = sqlc.arg('name'),
  description = sqlc.narg('description'),
  price = sqlc.arg('price'),
  is_enabled = sqlc.arg('is_enabled'),
  sort_order = sqlc.arg('sort_order'),
  updated_at = now()
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id')
  AND deleted_at IS NULL
RETURNING
  id,
  merchant_id,
  legacy_dish_id,
  name,
  description,
  price,
  is_enabled,
  sort_order,
  deleted_at,
  created_at,
  updated_at;

-- name: SoftDeleteMerchantPackagingOption :one
UPDATE merchant_packaging_options
SET
  is_enabled = false,
  deleted_at = COALESCE(deleted_at, now()),
  updated_at = CASE
    WHEN deleted_at IS NULL OR is_enabled IS DISTINCT FROM false
      THEN now()
    ELSE updated_at
  END
WHERE id = sqlc.arg('id')
  AND merchant_id = sqlc.arg('merchant_id')
RETURNING
  id,
  merchant_id,
  legacy_dish_id,
  name,
  description,
  price,
  is_enabled,
  sort_order,
  deleted_at,
  created_at,
  updated_at;

-- name: GetCartPackagingSelection :one
SELECT
  cart_id,
  packaging_option_id,
  selection_version,
  updated_at
FROM cart_packaging_selections
WHERE cart_id = $1
LIMIT 1;

-- name: UpsertCartPackagingSelection :one
INSERT INTO cart_packaging_selections (
  cart_id,
  packaging_option_id,
  selection_version,
  updated_at
) VALUES (
  sqlc.arg('cart_id'),
  sqlc.narg('packaging_option_id'),
  1,
  now()
)
ON CONFLICT (cart_id) DO UPDATE
SET
  packaging_option_id = EXCLUDED.packaging_option_id,
  selection_version = CASE
    WHEN cart_packaging_selections.packaging_option_id IS NOT DISTINCT FROM EXCLUDED.packaging_option_id
      THEN cart_packaging_selections.selection_version
    ELSE cart_packaging_selections.selection_version + 1
  END,
  updated_at = CASE
    WHEN cart_packaging_selections.packaging_option_id IS NOT DISTINCT FROM EXCLUDED.packaging_option_id
      THEN cart_packaging_selections.updated_at
    ELSE now()
  END
RETURNING
  cart_id,
  packaging_option_id,
  selection_version,
  updated_at;

-- name: ClearCartPackagingSelection :one
INSERT INTO cart_packaging_selections (
  cart_id,
  packaging_option_id,
  selection_version,
  updated_at
) VALUES (
  sqlc.arg('cart_id'),
  NULL,
  0,
  now()
)
ON CONFLICT (cart_id) DO UPDATE
SET
  packaging_option_id = NULL,
  selection_version = CASE
    WHEN cart_packaging_selections.packaging_option_id IS NULL
      THEN cart_packaging_selections.selection_version
    ELSE cart_packaging_selections.selection_version + 1
  END,
  updated_at = CASE
    WHEN cart_packaging_selections.packaging_option_id IS NULL
      THEN cart_packaging_selections.updated_at
    ELSE now()
  END
RETURNING
  cart_id,
  packaging_option_id,
  selection_version,
  updated_at;

-- name: UpdateCartPackagingSelectionIfChanged :one
UPDATE cart_packaging_selections
SET
  packaging_option_id = sqlc.narg('packaging_option_id'),
  selection_version = selection_version + 1,
  updated_at = now()
WHERE cart_id = sqlc.arg('cart_id')
  AND packaging_option_id IS DISTINCT FROM sqlc.narg('packaging_option_id')
RETURNING
  cart_id,
  packaging_option_id,
  selection_version,
  updated_at;

-- name: CreateOrderPackagingItem :one
INSERT INTO order_packaging_items (
  order_id,
  packaging_option_id,
  name,
  unit_price,
  quantity,
  subtotal
) VALUES (
  sqlc.arg('order_id'),
  sqlc.narg('packaging_option_id'),
  sqlc.arg('name'),
  sqlc.arg('unit_price'),
  sqlc.arg('quantity'),
  sqlc.arg('subtotal')
)
RETURNING
  id,
  order_id,
  packaging_option_id,
  name,
  unit_price,
  quantity,
  subtotal,
  created_at;

-- name: ListOrderPackagingItems :many
SELECT
  id,
  order_id,
  packaging_option_id,
  name,
  unit_price,
  quantity,
  subtotal,
  created_at
FROM order_packaging_items
WHERE order_id = $1
ORDER BY id ASC;

-- name: ListOrderPackagingItemsByOrderIDs :many
SELECT
  id,
  order_id,
  packaging_option_id,
  name,
  unit_price,
  quantity,
  subtotal,
  created_at
FROM order_packaging_items
WHERE order_id = ANY(sqlc.arg('order_ids')::bigint[])
ORDER BY order_id ASC, id ASC;
