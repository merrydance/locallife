-- name: CreateOperator :one
INSERT INTO operators (
    user_id,
    region_id,
    name,
    contact_name,
    contact_phone,
    wechat_mch_id,
    status,
    contract_start_date,
    contract_end_date,
    contract_years
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: GetOperator :one
SELECT id, user_id, region_id, name, contact_name, contact_phone, wechat_mch_id, status, created_at, updated_at, contract_start_date, contract_end_date, contract_years, sub_mch_id, balance, wallet_account, rider_deposit, weather_coeff_extreme, weather_coeff_heavy, weather_coeff_moderate, weather_coeff_light, latest_settlement_application_no, latest_settlement_application_submitted_at FROM operators
WHERE id = $1 LIMIT 1;

-- name: GetOperatorByUser :one
SELECT id, user_id, region_id, name, contact_name, contact_phone, wechat_mch_id, status, created_at, updated_at, contract_start_date, contract_end_date, contract_years, sub_mch_id, balance, wallet_account, rider_deposit, weather_coeff_extreme, weather_coeff_heavy, weather_coeff_moderate, weather_coeff_light, latest_settlement_application_no, latest_settlement_application_submitted_at FROM operators
WHERE user_id = $1 LIMIT 1;

-- name: ListOperators :many
SELECT 
    o.*,
    r.name as region_name
FROM operators o
INNER JOIN regions r ON o.region_id = r.id
ORDER BY o.created_at
LIMIT $1 OFFSET $2;

-- name: UpdateOperator :one
UPDATE operators
SET
    name = COALESCE(sqlc.narg(name), name),
    contact_name = COALESCE(sqlc.narg(contact_name), contact_name),
    contact_phone = COALESCE(sqlc.narg(contact_phone), contact_phone),
    wechat_mch_id = COALESCE(sqlc.narg(wechat_mch_id), wechat_mch_id),
    status = COALESCE(sqlc.narg(status), status),
    contract_start_date = COALESCE(sqlc.narg(contract_start_date), contract_start_date),
    contract_end_date = COALESCE(sqlc.narg(contract_end_date), contract_end_date),
    contract_years = COALESCE(sqlc.narg(contract_years), contract_years),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CountOperators :one
SELECT COUNT(*) FROM operators;

-- name: ListExpiringOperators :many
-- 列出即将到期的运营商（用于提前通知续约）
SELECT 
    o.*,
    r.name as region_name
FROM operators o
INNER JOIN regions r ON o.region_id = r.id
WHERE o.status = 'active' 
  AND o.contract_end_date IS NOT NULL
  AND o.contract_end_date <= CURRENT_DATE + $1::int  -- 参数为天数，如30天内到期
ORDER BY o.contract_end_date ASC;

-- name: ListExpiredOperators :many
-- 列出已过期的运营商
SELECT 
    o.*,
    r.name as region_name
FROM operators o
INNER JOIN regions r ON o.region_id = r.id
WHERE o.contract_end_date IS NOT NULL
  AND o.contract_end_date < CURRENT_DATE
  AND o.status = 'active';

-- name: UpdateOperatorStatus :one
-- 更新运营商状态（用于过期处理等）
UPDATE operators
SET
    status = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RenewOperatorContract :one
-- 续约运营商合同
UPDATE operators
SET
    contract_start_date = $2,
    contract_end_date = $3,
    contract_years = $4,
    status = 'active',
    updated_at = now()
WHERE id = $1
RETURNING *;
