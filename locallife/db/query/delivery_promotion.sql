-- name: CreateDeliveryPromotion :one
INSERT INTO merchant_delivery_promotions (
  merchant_id,
  name,
  min_order_amount,
  discount_amount,
  valid_from,
  valid_until,
  is_active
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetDeliveryPromotion :one
SELECT * FROM merchant_delivery_promotions
WHERE id = $1 LIMIT 1;

-- name: ListDeliveryPromotionsByMerchant :many
SELECT * FROM merchant_delivery_promotions
WHERE merchant_id = $1
ORDER BY min_order_amount;

-- name: ListActiveDeliveryPromotionsByMerchant :many
SELECT * FROM merchant_delivery_promotions
WHERE merchant_id = $1 
  AND is_active = true
  AND valid_from <= now()
  AND valid_until >= now()
ORDER BY min_order_amount;

-- name: GetBestDeliveryPromotion :one
-- 获取满足订单金额条件的最优促销（减免金额最大的那条）
SELECT * FROM merchant_delivery_promotions
WHERE merchant_id = $1
  AND is_active = true
  AND valid_from <= now()
  AND valid_until >= now()
  AND min_order_amount <= $2
ORDER BY discount_amount DESC
LIMIT 1;

-- name: UpdateDeliveryPromotion :one
UPDATE merchant_delivery_promotions
SET
  name = COALESCE(sqlc.narg(name), name),
  min_order_amount = COALESCE(sqlc.narg(min_order_amount), min_order_amount),
  discount_amount = COALESCE(sqlc.narg(discount_amount), discount_amount),
  valid_from = COALESCE(sqlc.narg(valid_from), valid_from),
  valid_until = COALESCE(sqlc.narg(valid_until), valid_until),
  is_active = COALESCE(sqlc.narg(is_active), is_active),
  updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteDeliveryPromotion :exec
DELETE FROM merchant_delivery_promotions
WHERE id = $1;

-- name: DeleteDeliveryPromotionsByMerchant :exec
DELETE FROM merchant_delivery_promotions
WHERE merchant_id = $1;
