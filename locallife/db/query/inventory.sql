-- ============================================
-- 库存管理查询 (Inventory Queries)
-- ============================================

-- name: CreateDailyInventory :one
INSERT INTO daily_inventory (
  merchant_id,
  dish_id,
  date,
  total_quantity,
  sold_quantity
) VALUES (
  $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetDailyInventory :one
SELECT id, merchant_id, dish_id, date, total_quantity, sold_quantity, created_at, updated_at, reserved_quantity FROM daily_inventory
WHERE merchant_id = $1 AND dish_id = $2 AND date = $3
LIMIT 1;

-- name: GetDailyInventoryForUpdate :one
SELECT id, merchant_id, dish_id, date, total_quantity, sold_quantity, created_at, updated_at, reserved_quantity FROM daily_inventory
WHERE merchant_id = $1 AND dish_id = $2 AND date = $3
LIMIT 1
FOR UPDATE;

-- name: ListDailyInventoryByMerchant :many
SELECT 
  COALESCE(di.id, 0) as id,
  d.merchant_id as merchant_id,
  d.id as dish_id,
  sqlc.arg('date')::date as date,
  COALESCE(di.total_quantity, -1) as total_quantity,
  COALESCE(di.sold_quantity, 0) as sold_quantity,
  COALESCE(di.created_at, now()) as created_at,
  COALESCE(di.updated_at, now()) as updated_at,
  COALESCE(di.reserved_quantity, 0) as reserved_quantity,
  d.name as dish_name,
  d.price as dish_price
FROM dishes d
LEFT JOIN daily_inventory di ON di.dish_id = d.id
  AND di.merchant_id = d.merchant_id
  AND di.date = sqlc.arg('date')
WHERE 
  d.merchant_id = $1
  AND d.is_online = true
  AND d.deleted_at IS NULL
ORDER BY d.name ASC;

-- name: ListDailyInventoryByDate :many
SELECT id, merchant_id, dish_id, date, total_quantity, sold_quantity, created_at, updated_at, reserved_quantity FROM daily_inventory
WHERE date = $1
ORDER BY merchant_id ASC, dish_id ASC;

-- name: UpdateDailyInventory :one
UPDATE daily_inventory
SET
  total_quantity = COALESCE(sqlc.narg('total_quantity'), total_quantity),
  sold_quantity = COALESCE(sqlc.narg('sold_quantity'), sold_quantity),
  reserved_quantity = COALESCE(sqlc.narg('reserved_quantity'), reserved_quantity),
  updated_at = now()
WHERE merchant_id = sqlc.arg('merchant_id')
  AND dish_id = sqlc.arg('dish_id')
  AND date = sqlc.arg('date')
RETURNING *;

-- name: IncrementSoldQuantity :one
UPDATE daily_inventory
SET
  sold_quantity = sold_quantity + $4,
  updated_at = now()
WHERE merchant_id = $1
  AND dish_id = $2
  AND date = $3
RETURNING *;

-- name: CheckAndDecrementInventory :one
UPDATE daily_inventory
SET
  sold_quantity = sold_quantity + $4,
  updated_at = now()
WHERE merchant_id = $1
  AND dish_id = $2
  AND date = $3
  AND (total_quantity = -1 OR sold_quantity + reserved_quantity + $4 <= total_quantity)
RETURNING *;

-- name: ReserveInventory :one
UPDATE daily_inventory
SET
  reserved_quantity = reserved_quantity + $4,
  updated_at = now()
WHERE merchant_id = $1
  AND dish_id = $2
  AND date = $3
  AND (total_quantity = -1 OR sold_quantity + reserved_quantity + $4 <= total_quantity)
RETURNING *;

-- name: ReleaseReservedInventory :one
UPDATE daily_inventory
SET
  reserved_quantity = GREATEST(reserved_quantity - $4, 0),
  updated_at = now()
WHERE merchant_id = $1
  AND dish_id = $2
  AND date = $3
RETURNING *;

-- name: DeleteDailyInventory :exec
DELETE FROM daily_inventory
WHERE merchant_id = $1 AND dish_id = $2 AND date = $3;

-- name: DeleteOldInventory :exec
DELETE FROM daily_inventory
WHERE date < $1;

-- name: GetInventoryStats :one
SELECT 
  COUNT(d.id) as total_dishes,
  COUNT(d.id) FILTER (WHERE di.total_quantity = -1 OR di.id IS NULL) as unlimited_dishes,
  COUNT(d.id) FILTER (WHERE di.total_quantity = 0 OR (di.total_quantity > 0 AND di.sold_quantity + di.reserved_quantity >= di.total_quantity)) as sold_out_dishes,
  COUNT(d.id) FILTER (
    WHERE (di.total_quantity = -1 OR di.id IS NULL) -- 无限库存
    OR (di.total_quantity > 0 AND di.sold_quantity + di.reserved_quantity < di.total_quantity) -- 有限库存且仍有未提交库存
  ) as available_dishes
FROM dishes d
LEFT JOIN daily_inventory di ON d.id = di.dish_id AND di.date = $2
WHERE d.merchant_id = $1 AND d.is_online = true;

-- name: BatchCreateDailyInventory :copyfrom
INSERT INTO daily_inventory (
  merchant_id,
  dish_id,
  date,
  total_quantity,
  sold_quantity
) VALUES (
  $1, $2, $3, $4, $5
);
