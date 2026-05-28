-- name: CreatePrintLog :one
INSERT INTO print_logs (
    order_id,
    printer_id,
    print_content,
    status,
    task_key
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetPrintLog :one
SELECT id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key FROM print_logs
WHERE id = $1 LIMIT 1;

-- name: GetPrintLogByTaskKeyAndPrinter :one
SELECT id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key FROM print_logs
WHERE task_key = $1
    AND printer_id = $2
LIMIT 1;

-- name: GetPrintLogByVendorOrderID :one
SELECT id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key FROM print_logs
WHERE vendor_order_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetLatestPrintLogByOrderAndPrinter :one
SELECT id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key FROM print_logs
WHERE order_id = $1
    AND printer_id = $2
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListPrintLogsByOrder :many
SELECT 
    pl.id, pl.order_id, pl.printer_id, pl.print_content, pl.status, pl.error_message, pl.printed_at, pl.created_at, pl.vendor_order_id, pl.task_key,
    cp.printer_name
FROM print_logs pl
INNER JOIN cloud_printers cp ON pl.printer_id = cp.id
WHERE pl.order_id = $1
ORDER BY pl.created_at;

-- name: ListPrintLogsByPrinter :many
SELECT id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key FROM print_logs
WHERE printer_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListMerchantPrintAnomalies :many
WITH latest AS (
        SELECT DISTINCT ON (pl.order_id, pl.printer_id)
                pl.id,
                pl.order_id,
                o.order_no,
                o.order_type,
                pl.printer_id,
                cp.printer_name,
                cp.printer_type,
                cp.is_active,
                pl.status,
                pl.error_message,
                pl.vendor_order_id,
                pl.created_at,
                pl.printed_at
        FROM print_logs pl
        INNER JOIN orders o ON pl.order_id = o.id
        INNER JOIN cloud_printers cp ON pl.printer_id = cp.id
        WHERE o.merchant_id = $1
        ORDER BY pl.order_id, pl.printer_id, pl.created_at DESC
)
SELECT id, order_id, order_no, order_type, printer_id, printer_name, printer_type, is_active, status, error_message, vendor_order_id, created_at, printed_at FROM latest
WHERE status <> 'success'
    AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMerchantPrintAnomalies :one
WITH latest AS (
        SELECT DISTINCT ON (pl.order_id, pl.printer_id)
                pl.status
        FROM print_logs pl
        INNER JOIN orders o ON pl.order_id = o.id
        WHERE o.merchant_id = $1
        ORDER BY pl.order_id, pl.printer_id, pl.created_at DESC
)
SELECT COUNT(*) FROM latest
WHERE status <> 'success'
    AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status));

-- name: ListTimedOutPrintAnomalies :many
        SELECT
    pl.id,
    o.merchant_id,
    m.name AS merchant_name,
    pl.order_id,
    o.order_no,
    pl.printer_id,
    cp.printer_name,
    pl.status,
    pl.error_message,
    pl.vendor_order_id,
    pl.created_at AS anomaly_created_at
FROM print_logs pl
INNER JOIN orders o ON pl.order_id = o.id
INNER JOIN merchants m ON o.merchant_id = m.id
INNER JOIN cloud_printers cp ON pl.printer_id = cp.id
WHERE pl.status <> 'success'
  AND pl.created_at <= $1
  AND NOT EXISTS (
    SELECT 1
    FROM print_logs newer
    WHERE newer.order_id = pl.order_id
      AND newer.printer_id = pl.printer_id
      AND (
        newer.created_at > pl.created_at
        OR (newer.created_at = pl.created_at AND newer.id > pl.id)
      )
  )
ORDER BY pl.created_at ASC
LIMIT $2;

-- name: UpdatePrintLogStatus :one
UPDATE print_logs
SET
    status = $2,
    error_message = $3,
    vendor_order_id = COALESCE($4, vendor_order_id),
    printed_at = CASE WHEN $2 = 'success' THEN now() ELSE printed_at END
WHERE id = $1
RETURNING *;

-- name: CountPendingPrintLogs :one
SELECT COUNT(*) FROM print_logs
WHERE printer_id = $1 AND status = 'pending';
