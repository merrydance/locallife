-- name: CreateOrderDisplayConfig :one
INSERT INTO order_display_configs (
    merchant_id,
    enable_print,
    print_takeout,
    print_dine_in,
    print_reservation,
    enable_voice,
    voice_takeout,
    voice_dine_in,
    enable_kds,
    kds_url
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: GetOrderDisplayConfig :one
SELECT * FROM order_display_configs
WHERE id = $1 LIMIT 1;

-- name: GetOrderDisplayConfigByMerchant :one
SELECT * FROM order_display_configs
WHERE merchant_id = $1 LIMIT 1;

-- name: UpdateOrderDisplayConfig :one
UPDATE order_display_configs
SET
    enable_print = COALESCE(sqlc.narg(enable_print), enable_print),
    print_takeout = COALESCE(sqlc.narg(print_takeout), print_takeout),
    print_dine_in = COALESCE(sqlc.narg(print_dine_in), print_dine_in),
    print_reservation = COALESCE(sqlc.narg(print_reservation), print_reservation),
    enable_voice = COALESCE(sqlc.narg(enable_voice), enable_voice),
    voice_takeout = COALESCE(sqlc.narg(voice_takeout), voice_takeout),
    voice_dine_in = COALESCE(sqlc.narg(voice_dine_in), voice_dine_in),
    enable_kds = COALESCE(sqlc.narg(enable_kds), enable_kds),
    kds_url = COALESCE(sqlc.narg(kds_url), kds_url),
    updated_at = now()
WHERE merchant_id = $1
RETURNING *;

-- name: UpsertOrderDisplayConfig :one
INSERT INTO order_display_configs (
    merchant_id,
    enable_print,
    print_takeout,
    print_dine_in,
    print_reservation,
    enable_voice,
    voice_takeout,
    voice_dine_in,
    enable_kds,
    kds_url
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (merchant_id) DO UPDATE SET
    enable_print = EXCLUDED.enable_print,
    print_takeout = EXCLUDED.print_takeout,
    print_dine_in = EXCLUDED.print_dine_in,
    print_reservation = EXCLUDED.print_reservation,
    enable_voice = EXCLUDED.enable_voice,
    voice_takeout = EXCLUDED.voice_takeout,
    voice_dine_in = EXCLUDED.voice_dine_in,
    enable_kds = EXCLUDED.enable_kds,
    kds_url = EXCLUDED.kds_url,
    updated_at = now()
RETURNING *;
