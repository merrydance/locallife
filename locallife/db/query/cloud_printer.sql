-- name: CreateCloudPrinter :one
INSERT INTO cloud_printers (
    merchant_id,
    printer_name,
    printer_sn,
    printer_key,
    printer_type,
    printer_role,
    print_takeout,
    print_dine_in,
    print_reservation
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING id, merchant_id, printer_name, printer_sn, printer_key, printer_type, print_takeout, print_dine_in, print_reservation, is_active, created_at, updated_at, printer_role, deleted_at;

-- name: GetCloudPrinter :one
SELECT id, merchant_id, printer_name, printer_sn, printer_key, printer_type, print_takeout, print_dine_in, print_reservation, is_active, created_at, updated_at, printer_role, deleted_at FROM cloud_printers
WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetCloudPrinterIncludingDeleted :one
SELECT id, merchant_id, printer_name, printer_sn, printer_key, printer_type, print_takeout, print_dine_in, print_reservation, is_active, created_at, updated_at, printer_role, deleted_at FROM cloud_printers
WHERE id = $1 LIMIT 1;

-- name: GetCloudPrinterBySN :one
SELECT id, merchant_id, printer_name, printer_sn, printer_key, printer_type, print_takeout, print_dine_in, print_reservation, is_active, created_at, updated_at, printer_role, deleted_at FROM cloud_printers
WHERE printer_sn = $1 AND deleted_at IS NULL LIMIT 1;

-- name: ListCloudPrintersByMerchant :many
SELECT id, merchant_id, printer_name, printer_sn, printer_key, printer_type, print_takeout, print_dine_in, print_reservation, is_active, created_at, updated_at, printer_role, deleted_at FROM cloud_printers
WHERE merchant_id = $1 AND deleted_at IS NULL
ORDER BY created_at;

-- name: ListActiveCloudPrintersByMerchant :many
SELECT id, merchant_id, printer_name, printer_sn, printer_key, printer_type, print_takeout, print_dine_in, print_reservation, is_active, created_at, updated_at, printer_role, deleted_at FROM cloud_printers
WHERE merchant_id = $1 AND is_active = true AND deleted_at IS NULL
ORDER BY created_at;

-- name: UpdateCloudPrinter :one
UPDATE cloud_printers
SET
    printer_name = COALESCE(sqlc.narg(printer_name), printer_name),
    printer_key = COALESCE(sqlc.narg(printer_key), printer_key),
    printer_role = COALESCE(sqlc.narg(printer_role), printer_role),
    print_takeout = COALESCE(sqlc.narg(print_takeout), print_takeout),
    print_dine_in = COALESCE(sqlc.narg(print_dine_in), print_dine_in),
    print_reservation = COALESCE(sqlc.narg(print_reservation), print_reservation),
    is_active = COALESCE(sqlc.narg(is_active), is_active),
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, merchant_id, printer_name, printer_sn, printer_key, printer_type, print_takeout, print_dine_in, print_reservation, is_active, created_at, updated_at, printer_role, deleted_at;

-- name: DeleteCloudPrinter :exec
UPDATE cloud_printers
SET deleted_at = now(), is_active = false, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;
