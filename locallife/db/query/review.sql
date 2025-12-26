-- name: CreateReview :one
INSERT INTO reviews (
  order_id,
  user_id,
  merchant_id,
  content,
  images,
  is_visible
) VALUES (
  $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetReview :one
SELECT * FROM reviews
WHERE id = $1 LIMIT 1;

-- name: GetReviewByOrderID :one
SELECT * FROM reviews
WHERE order_id = $1 LIMIT 1;

-- name: ListReviewsByMerchant :many
SELECT * FROM reviews
WHERE merchant_id = $1
  AND is_visible = true
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountReviewsByMerchant :one
SELECT COUNT(*) FROM reviews
WHERE merchant_id = $1
  AND is_visible = true;

-- name: ListReviewsByUser :many
SELECT * FROM reviews
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountReviewsByUser :one
SELECT COUNT(*) FROM reviews
WHERE user_id = $1;

-- name: UpdateMerchantReply :one
UPDATE reviews
SET merchant_reply = $2,
    replied_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateReviewVisibility :one
UPDATE reviews
SET is_visible = $2
WHERE id = $1
RETURNING *;

-- name: DeleteReview :exec
DELETE FROM reviews
WHERE id = $1;

-- name: ListAllReviewsByMerchant :many
-- 商户查看所有评价（包含不可见的）
SELECT * FROM reviews
WHERE merchant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAllReviewsByMerchant :one
-- 商户查看所有评价数量（包含不可见的）
SELECT COUNT(*) FROM reviews
WHERE merchant_id = $1;
