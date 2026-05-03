-- name: CreateMerchantPaymentConfig :one
INSERT INTO merchant_payment_configs (
    merchant_id,
    sub_mch_id,
    status
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: UpsertMerchantPaymentConfig :one
INSERT INTO merchant_payment_configs (
    merchant_id,
    sub_mch_id,
    status
) VALUES (
    $1, $2, $3
)
ON CONFLICT (merchant_id) DO UPDATE
SET
    sub_mch_id = EXCLUDED.sub_mch_id,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING *;

-- name: GetMerchantPaymentConfig :one
SELECT id, merchant_id, sub_mch_id, status, created_at, updated_at, latest_settlement_application_no, latest_settlement_application_submitted_at FROM merchant_payment_configs
WHERE merchant_id = $1 LIMIT 1;

-- name: GetMerchantPaymentConfigBySubMchID :one
SELECT id, merchant_id, sub_mch_id, status, created_at, updated_at, latest_settlement_application_no, latest_settlement_application_submitted_at FROM merchant_payment_configs
WHERE sub_mch_id = $1 LIMIT 1;

-- name: UpdateMerchantPaymentConfig :one
UPDATE merchant_payment_configs
SET
    sub_mch_id = COALESCE(sqlc.narg(sub_mch_id), sub_mch_id),
    status = COALESCE(sqlc.narg(status), status),
    updated_at = now()
WHERE merchant_id = $1
RETURNING *;

-- name: UpdateMerchantPaymentConfigSettlementApplication :one
UPDATE merchant_payment_configs
SET
    latest_settlement_application_no = COALESCE(sqlc.narg(latest_settlement_application_no), latest_settlement_application_no),
    latest_settlement_application_submitted_at = COALESCE(sqlc.narg(latest_settlement_application_submitted_at), latest_settlement_application_submitted_at),
    updated_at = now()
WHERE merchant_id = $1
RETURNING *;

-- name: DeleteMerchantPaymentConfig :exec
DELETE FROM merchant_payment_configs
WHERE merchant_id = $1;
