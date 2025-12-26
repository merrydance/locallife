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
SELECT * FROM daily_inventory
WHERE merchant_id = $1 AND dish_id = $2 AND date = $3
LIMIT 1;

-- name: GetDailyInventoryForUpdate :one
SELECT * FROM daily_inventory
WHERE merchant_id = $1 AND dish_id = $2 AND date = $3
LIMIT 1
FOR UPDATE;

-- name: ListDailyInventoryByMerchant :many
SELECT 
  di.*,
  d.name as dish_name,
  d.price as dish_price
FROM daily_inventory di
JOIN dishes d ON di.dish_id = d.id
WHERE 
  di.merchant_id = $1
  AND di.date = $2
ORDER BY d.name ASC;

-- name: ListDailyInventoryByDate :many
SELECT * FROM daily_inventory
WHERE date = $1
ORDER BY merchant_id ASC, dish_id ASC;

-- name: UpdateDailyInventory :one
UPDATE daily_inventory
SET
  total_quantity = COALESCE(sqlc.narg('total_quantity'), total_quantity),
  sold_quantity = COALESCE(sqlc.narg('sold_quantity'), sold_quantity),
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
  AND (total_quantity = -1 OR sold_quantity + $4 <= total_quantity)
RETURNING *;

-- name: DeleteDailyInventory :exec
DELETE FROM daily_inventory
WHERE merchant_id = $1 AND dish_id = $2 AND date = $3;

-- name: DeleteOldInventory :exec
DELETE FROM daily_inventory
WHERE date < $1;

-- name: GetInventoryStats :one
SELECT 
  COUNT(*) as total_dishes,
  COUNT(*) FILTER (WHERE total_quantity = -1) as unlimited_dishes,
  COUNT(*) FILTER (WHERE total_quantity > 0 AND sold_quantity >= total_quantity) as sold_out_dishes,
  COUNT(*) FILTER (WHERE total_quantity > 0 AND sold_quantity < total_quantity) as available_dishes
FROM daily_inventory
WHERE merchant_id = $1 AND date = $2;

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
