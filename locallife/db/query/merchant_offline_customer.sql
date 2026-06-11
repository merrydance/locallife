-- name: UpsertMerchantOfflineCustomer :one
INSERT INTO merchant_offline_customers (
    merchant_id,
    contact_name,
    contact_phone,
    source,
    created_by_user_id,
    updated_by_user_id,
    first_seen_at,
    last_seen_at
) VALUES (
    sqlc.arg(merchant_id), btrim(sqlc.arg(contact_name)), btrim(sqlc.arg(contact_phone)), sqlc.arg(source), sqlc.arg(created_by_user_id), sqlc.arg(created_by_user_id), now(), now()
)
ON CONFLICT (merchant_id, contact_phone) DO UPDATE
SET contact_name = EXCLUDED.contact_name,
    source = EXCLUDED.source,
    updated_by_user_id = EXCLUDED.updated_by_user_id,
    last_seen_at = now(),
    updated_at = now()
RETURNING *;

-- name: GetMerchantOfflineCustomer :one
SELECT id, merchant_id, contact_name, contact_phone, source, created_by_user_id, updated_by_user_id, first_seen_at, last_seen_at, created_at, updated_at
FROM merchant_offline_customers
WHERE id = $1
  AND merchant_id = $2
LIMIT 1;
