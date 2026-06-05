-- name: CreatePrintLog :one
INSERT INTO print_logs (
    order_id,
    printer_id,
    print_content,
    status,
    task_key,
    provider_origin_id
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key, provider_origin_id, provider_status_checked_at, provider_status_check_attempts, provider_status_last_error;

-- name: GetPrintLog :one
SELECT id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key, provider_origin_id, provider_status_checked_at, provider_status_check_attempts, provider_status_last_error FROM print_logs
WHERE id = $1 LIMIT 1;

-- name: GetPrintLogByTaskKeyAndPrinter :one
SELECT id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key, provider_origin_id, provider_status_checked_at, provider_status_check_attempts, provider_status_last_error FROM print_logs
WHERE task_key = $1
    AND printer_id = $2
LIMIT 1;

-- name: GetPrintLogByVendorOrderID :one
SELECT id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key, provider_origin_id, provider_status_checked_at, provider_status_check_attempts, provider_status_last_error FROM print_logs
WHERE vendor_order_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetPrintLogByProviderAndVendorOrderID :one
SELECT
    pl.id,
    pl.order_id,
    pl.printer_id,
    pl.print_content,
    pl.status,
    pl.error_message,
    pl.printed_at,
    pl.created_at,
    pl.vendor_order_id,
    pl.task_key,
    pl.provider_origin_id,
    pl.provider_status_checked_at,
    pl.provider_status_check_attempts,
    pl.provider_status_last_error
FROM print_logs pl
INNER JOIN cloud_printers cp ON pl.printer_id = cp.id
WHERE cp.printer_type = $1
    AND pl.vendor_order_id = $2
ORDER BY pl.created_at DESC, pl.id DESC
LIMIT 1;

-- name: GetPrintLogByProviderAndOriginID :one
SELECT
    pl.id,
    pl.order_id,
    pl.printer_id,
    pl.print_content,
    pl.status,
    pl.error_message,
    pl.printed_at,
    pl.created_at,
    pl.vendor_order_id,
    pl.task_key,
    pl.provider_origin_id,
    pl.provider_status_checked_at,
    pl.provider_status_check_attempts,
    pl.provider_status_last_error
FROM print_logs pl
INNER JOIN cloud_printers cp ON pl.printer_id = cp.id
WHERE cp.printer_type = $1
    AND pl.provider_origin_id = $2
ORDER BY pl.created_at DESC, pl.id DESC
LIMIT 1;

-- name: GetLatestPrintLogByOrderAndPrinter :one
SELECT id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key, provider_origin_id, provider_status_checked_at, provider_status_check_attempts, provider_status_last_error FROM print_logs
WHERE order_id = $1
    AND printer_id = $2
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListPrintLogsByOrder :many
SELECT 
    pl.id, pl.order_id, pl.printer_id, pl.print_content, pl.status, pl.error_message, pl.printed_at, pl.created_at, pl.vendor_order_id, pl.task_key, pl.provider_origin_id, pl.provider_status_checked_at, pl.provider_status_check_attempts, pl.provider_status_last_error,
    cp.printer_name,
    cp.printer_type
FROM print_logs pl
INNER JOIN cloud_printers cp ON pl.printer_id = cp.id
WHERE pl.order_id = $1
ORDER BY pl.created_at;

-- name: ListPrintLogsByPrinter :many
SELECT id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key, provider_origin_id, provider_status_checked_at, provider_status_check_attempts, provider_status_last_error FROM print_logs
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
                pl.provider_origin_id,
                pl.created_at,
                pl.printed_at
        FROM print_logs pl
        INNER JOIN orders o ON pl.order_id = o.id
        INNER JOIN cloud_printers cp ON pl.printer_id = cp.id
        WHERE o.merchant_id = $1
        ORDER BY pl.order_id, pl.printer_id, pl.created_at DESC
)
SELECT id, order_id, order_no, order_type, printer_id, printer_name, printer_type, is_active, status, error_message, vendor_order_id, provider_origin_id, created_at, printed_at FROM latest
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
    pl.provider_origin_id,
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
    provider_origin_id = COALESCE($5, provider_origin_id),
    printed_at = CASE WHEN $2 = 'success' THEN now() ELSE printed_at END
WHERE id = $1
RETURNING id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key, provider_origin_id, provider_status_checked_at, provider_status_check_attempts, provider_status_last_error;

-- name: ClaimPendingProviderStatusPrintLogs :many
UPDATE print_logs pl
SET
    provider_status_checked_at = now(),
    provider_status_check_attempts = provider_status_check_attempts + 1,
    provider_status_last_error = NULL
FROM (
    SELECT pl_inner.id
    FROM print_logs pl_inner
    INNER JOIN cloud_printers cp ON pl_inner.printer_id = cp.id
    WHERE pl_inner.status = 'pending'
        AND pl_inner.vendor_order_id IS NOT NULL
        AND cp.printer_type = ANY(sqlc.arg(printer_types)::text[])
        AND pl_inner.created_at <= sqlc.arg(ready_before)
        AND pl_inner.created_at >= sqlc.arg(created_after)
        AND (
            pl_inner.provider_status_checked_at IS NULL
            OR pl_inner.provider_status_checked_at <= sqlc.arg(checked_before)
        )
    ORDER BY pl_inner.created_at ASC, pl_inner.id ASC
    LIMIT sqlc.arg(limit_count)
    FOR UPDATE OF pl_inner SKIP LOCKED
) claim,
cloud_printers cp
WHERE pl.id = claim.id
    AND cp.id = pl.printer_id
RETURNING
    pl.id,
    pl.order_id,
    pl.printer_id,
    pl.print_content,
    pl.status,
    pl.error_message,
    pl.printed_at,
    pl.created_at,
    pl.vendor_order_id,
    pl.task_key,
    pl.provider_origin_id,
    pl.provider_status_checked_at,
    pl.provider_status_check_attempts,
    pl.provider_status_last_error,
    cp.printer_sn,
    cp.printer_type;

-- name: MarkProviderStatusPrintLogTerminal :one
UPDATE print_logs
SET
    status = sqlc.arg(status),
    error_message = sqlc.arg(error_message),
    provider_status_checked_at = now(),
    provider_status_last_error = NULL,
    printed_at = CASE WHEN sqlc.arg(status) = 'success' THEN now() ELSE printed_at END
WHERE id = sqlc.arg(id)
    AND status = 'pending'
RETURNING id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key, provider_origin_id, provider_status_checked_at, provider_status_check_attempts, provider_status_last_error;

-- name: RecordProviderStatusPollError :one
UPDATE print_logs
SET
    provider_status_checked_at = now(),
    provider_status_last_error = sqlc.arg(provider_status_last_error)
WHERE id = sqlc.arg(id)
    AND status = 'pending'
RETURNING id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key, provider_origin_id, provider_status_checked_at, provider_status_check_attempts, provider_status_last_error;

-- name: ExpireProviderStatusPrintLogs :many
UPDATE print_logs pl
SET
    status = 'failed',
    error_message = sqlc.arg(error_message),
    provider_status_checked_at = now(),
    provider_status_last_error = NULL
FROM (
    SELECT pl_inner.id
    FROM print_logs pl_inner
    INNER JOIN cloud_printers cp ON pl_inner.printer_id = cp.id
    WHERE pl_inner.status = 'pending'
        AND pl_inner.vendor_order_id IS NOT NULL
        AND cp.printer_type = ANY(sqlc.arg(printer_types)::text[])
        AND pl_inner.created_at < sqlc.arg(expired_before)
    ORDER BY pl_inner.created_at ASC, pl_inner.id ASC
    LIMIT sqlc.arg(limit_count)
    FOR UPDATE OF pl_inner SKIP LOCKED
) expired
WHERE pl.id = expired.id
RETURNING pl.id, pl.order_id, pl.printer_id, pl.print_content, pl.status, pl.error_message, pl.printed_at, pl.created_at, pl.vendor_order_id, pl.task_key, pl.provider_origin_id, pl.provider_status_checked_at, pl.provider_status_check_attempts, pl.provider_status_last_error;

-- name: CountPendingPrintLogs :one
SELECT COUNT(*) FROM print_logs
WHERE printer_id = $1 AND status = 'pending';
