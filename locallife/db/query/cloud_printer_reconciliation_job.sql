-- name: UpsertCloudPrinterReconciliationJob :one
INSERT INTO cloud_printer_reconciliation_jobs (
    merchant_id,
    printer_id,
    printer_name,
    printer_sn,
    printer_key,
    printer_type,
    desired_action,
    source_action,
    failure_reason,
    last_error
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (merchant_id, printer_sn, desired_action)
WHERE status = 'pending'
DO UPDATE SET
    printer_id = EXCLUDED.printer_id,
    printer_name = EXCLUDED.printer_name,
    printer_key = EXCLUDED.printer_key,
    printer_type = EXCLUDED.printer_type,
    source_action = EXCLUDED.source_action,
    failure_reason = EXCLUDED.failure_reason,
    last_error = EXCLUDED.last_error,
    updated_at = now()
RETURNING *;

-- name: GetCloudPrinterReconciliationJob :one
SELECT * FROM cloud_printer_reconciliation_jobs
WHERE id = $1
LIMIT 1;

-- name: ListCloudPrinterReconciliationJobsByMerchant :many
SELECT * FROM cloud_printer_reconciliation_jobs
WHERE merchant_id = $1
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
ORDER BY created_at DESC;

-- name: ResolveCloudPrinterReconciliationJob :one
UPDATE cloud_printer_reconciliation_jobs
SET
    status = 'resolved',
    retry_count = retry_count + 1,
    resolved_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: FailCloudPrinterReconciliationJobRetry :one
UPDATE cloud_printer_reconciliation_jobs
SET
    retry_count = retry_count + 1,
    last_error = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;