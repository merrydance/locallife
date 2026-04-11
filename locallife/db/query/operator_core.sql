-- name: GetOperatorForUpdate :one
SELECT * FROM operators
WHERE id = $1 LIMIT 1 FOR NO KEY UPDATE;

-- name: UpdateOperatorBalance :one
UPDATE operators
SET balance = balance + sqlc.arg(amount)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetOperatorWallet :exec
UPDATE operators
SET wallet_account = $2
WHERE id = $1;

-- name: UpdateOperatorRules :one
UPDATE operators
SET 
    merchant_deposit = COALESCE(sqlc.narg(merchant_deposit), merchant_deposit),
    rider_deposit = COALESCE(sqlc.narg(rider_deposit), rider_deposit),
    weather_coeff_extreme = COALESCE(sqlc.narg(weather_coeff_extreme), weather_coeff_extreme),
    weather_coeff_heavy = COALESCE(sqlc.narg(weather_coeff_heavy), weather_coeff_heavy),
    weather_coeff_moderate = COALESCE(sqlc.narg(weather_coeff_moderate), weather_coeff_moderate),
    weather_coeff_light = COALESCE(sqlc.narg(weather_coeff_light), weather_coeff_light),
    commission_rate = COALESCE(sqlc.narg(commission_rate), commission_rate),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;
