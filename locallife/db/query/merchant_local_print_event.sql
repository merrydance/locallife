-- name: UpsertMerchantLocalPrintEvent :one
INSERT INTO merchant_local_print_events (
    merchant_id,
    order_id,
    event_key,
    source,
    status,
    printer_name,
    error_message,
    printed_at
)
SELECT
    sqlc.arg(merchant_id),
    o.id,
    sqlc.arg(event_key),
    sqlc.arg(source),
    sqlc.arg(status),
    sqlc.narg(printer_name),
    sqlc.narg(error_message),
    CASE WHEN sqlc.arg(status) = 'success' THEN now() ELSE NULL END
FROM orders o
WHERE o.id = sqlc.arg(order_id)
    AND o.merchant_id = sqlc.arg(merchant_id)
ON CONFLICT (merchant_id, event_key) DO UPDATE SET
    status = CASE
        WHEN merchant_local_print_events.status = 'success' THEN merchant_local_print_events.status
        ELSE EXCLUDED.status
    END,
    printer_name = COALESCE(EXCLUDED.printer_name, merchant_local_print_events.printer_name),
    error_message = CASE
        WHEN merchant_local_print_events.status = 'success' THEN merchant_local_print_events.error_message
        ELSE EXCLUDED.error_message
    END,
    printed_at = CASE
        WHEN merchant_local_print_events.status = 'success' THEN merchant_local_print_events.printed_at
        WHEN EXCLUDED.status = 'success' THEN now()
        ELSE merchant_local_print_events.printed_at
    END,
    updated_at = now()
RETURNING id, merchant_id, order_id, event_key, source, status, printer_name, error_message, printed_at, created_at, updated_at;

-- name: GetMerchantLocalPrintEventByKey :one
SELECT id, merchant_id, order_id, event_key, source, status, printer_name, error_message, printed_at, created_at, updated_at
FROM merchant_local_print_events
WHERE merchant_id = $1
    AND event_key = $2
LIMIT 1;
