-- name: CreateBaofuFeeLedger :one
INSERT INTO baofu_fee_ledger (
    fee_type,
    payer_type,
    payer_id,
    business_object_type,
    business_object_id,
    amount,
    fee_rate_bps,
    provider_bill_no,
    status
) VALUES (
    sqlc.arg(fee_type),
    sqlc.arg(payer_type),
    sqlc.narg(payer_id),
    sqlc.arg(business_object_type),
    sqlc.arg(business_object_id),
    sqlc.arg(amount),
    sqlc.narg(fee_rate_bps),
    sqlc.narg(provider_bill_no),
    sqlc.arg(status)
) RETURNING id, fee_type, payer_type, payer_id, business_object_type, business_object_id, amount, fee_rate_bps, provider_bill_no, status, created_at, updated_at;

-- name: GetBaofuFeeLedger :one
SELECT id, fee_type, payer_type, payer_id, business_object_type, business_object_id, amount, fee_rate_bps, provider_bill_no, status, created_at, updated_at
FROM baofu_fee_ledger
WHERE id = $1
LIMIT 1;

-- name: GetBaofuFeeLedgerByBusinessObject :one
SELECT id, fee_type, payer_type, payer_id, business_object_type, business_object_id, amount, fee_rate_bps, provider_bill_no, status, created_at, updated_at
FROM baofu_fee_ledger
WHERE fee_type = sqlc.arg(fee_type)
  AND business_object_type = sqlc.arg(business_object_type)
  AND business_object_id = sqlc.arg(business_object_id)
LIMIT 1;

-- name: ListBaofuFeeLedgerByPayer :many
SELECT id, fee_type, payer_type, payer_id, business_object_type, business_object_id, amount, fee_rate_bps, provider_bill_no, status, created_at, updated_at
FROM baofu_fee_ledger
WHERE payer_type = sqlc.arg(payer_type)
  AND (sqlc.narg(payer_id)::bigint IS NULL OR payer_id = sqlc.narg(payer_id))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);
