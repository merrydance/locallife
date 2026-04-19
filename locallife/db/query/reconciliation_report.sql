-- name: CreateReconciliationReport :one
INSERT INTO reconciliation_reports (
    bill_date,
    bill_type,
    status
) VALUES (
    $1, $2, 'running'
) ON CONFLICT (bill_date, bill_type)
  DO UPDATE SET status = 'running', updated_at = now()
RETURNING *;

-- name: UpdateReconciliationReport :one
UPDATE reconciliation_reports
SET
    status          = $2,
    wxpay_count     = $3,
    local_count     = $4,
    mismatch_count  = $5,
    missing_local   = $6,
    missing_wxpay   = $7,
    amount_mismatch = $8,
    error_message   = $9,
    updated_at      = now()
WHERE id = $1
RETURNING *;

-- name: GetReconciliationReport :one
SELECT id, bill_date, bill_type, status, wxpay_count, local_count, mismatch_count, missing_local, missing_wxpay, amount_mismatch, error_message, created_at, updated_at FROM reconciliation_reports
WHERE id = $1 LIMIT 1;

-- name: GetReconciliationReportByDateAndType :one
SELECT id, bill_date, bill_type, status, wxpay_count, local_count, mismatch_count, missing_local, missing_wxpay, amount_mismatch, error_message, created_at, updated_at FROM reconciliation_reports
WHERE bill_date = $1 AND bill_type = $2
LIMIT 1;

-- name: ListReconciliationReports :many
SELECT id, bill_date, bill_type, status, wxpay_count, local_count, mismatch_count, missing_local, missing_wxpay, amount_mismatch, error_message, created_at, updated_at FROM reconciliation_reports
ORDER BY bill_date DESC, bill_type
LIMIT $1 OFFSET $2;
