-- name: CreateReview :one
INSERT INTO reviews (
  order_id,
  user_id,
  merchant_id,
  content,
  is_visible
) VALUES (
  $1, $2, $3, $4, $5
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
SELECT r.*, m.name as merchant_name, m.logo_media_asset_id as merchant_logo_media_asset_id
FROM reviews r
JOIN merchants m ON r.merchant_id = m.id
WHERE r.user_id = $1
ORDER BY r.created_at DESC
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

-- name: AddReviewImage :one
INSERT INTO review_images (review_id, media_asset_id, sort_order)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListReviewImages :many
SELECT * FROM review_images
WHERE review_id = $1
ORDER BY sort_order ASC;

-- name: ListReviewImagesByReviews :many
SELECT * FROM review_images
WHERE review_id = ANY($1::bigint[])
ORDER BY review_id, sort_order ASC;

-- name: DeleteReviewImages :exec
DELETE FROM review_images
WHERE review_id = $1;

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
