-- name: CreatePrintLog :one
INSERT INTO print_logs (
    order_id,
    printer_id,
    print_content,
    status
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetPrintLog :one
SELECT * FROM print_logs
WHERE id = $1 LIMIT 1;

-- name: ListPrintLogsByOrder :many
SELECT 
    pl.*,
    cp.printer_name
FROM print_logs pl
INNER JOIN cloud_printers cp ON pl.printer_id = cp.id
WHERE pl.order_id = $1
ORDER BY pl.created_at;

-- name: ListPrintLogsByPrinter :many
SELECT * FROM print_logs
WHERE printer_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdatePrintLogStatus :one
UPDATE print_logs
SET
    status = $2,
    error_message = $3,
    printed_at = CASE WHEN $2 = 'success' THEN now() ELSE printed_at END
WHERE id = $1
RETURNING *;

-- name: CountPendingPrintLogs :one
SELECT COUNT(*) FROM print_logs
WHERE printer_id = $1 AND status = 'pending';
