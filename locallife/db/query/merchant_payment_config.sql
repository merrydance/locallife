-- name: CreateMerchantPaymentConfig :one
INSERT INTO merchant_payment_configs (
    merchant_id,
    sub_mch_id,
    status
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetMerchantPaymentConfig :one
SELECT * FROM merchant_payment_configs
WHERE merchant_id = $1 LIMIT 1;

-- name: GetMerchantPaymentConfigBySubMchID :one
SELECT * FROM merchant_payment_configs
WHERE sub_mch_id = $1 LIMIT 1;

-- name: UpdateMerchantPaymentConfig :one
UPDATE merchant_payment_configs
SET
    sub_mch_id = COALESCE(sqlc.narg(sub_mch_id), sub_mch_id),
    status = COALESCE(sqlc.narg(status), status),
    updated_at = now()
WHERE merchant_id = $1
RETURNING *;

-- name: DeleteMerchantPaymentConfig :exec
DELETE FROM merchant_payment_configs
WHERE merchant_id = $1;
