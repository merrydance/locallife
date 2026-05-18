-- name: UpsertBaofuMerchantReportProcessing :one
INSERT INTO baofu_merchant_reports (
    owner_type,
    owner_id,
    report_type,
    report_no,
    bct_mer_id,
    report_state,
    applet_auth_state,
    raw_snapshot
) VALUES (
    sqlc.arg(owner_type),
    sqlc.arg(owner_id),
    sqlc.arg(report_type),
    sqlc.arg(report_no),
    sqlc.arg(bct_mer_id),
    'processing',
    'pending',
    sqlc.arg(raw_snapshot)
)
ON CONFLICT (owner_type, owner_id, report_type)
DO UPDATE SET
    report_no = EXCLUDED.report_no,
    bct_mer_id = EXCLUDED.bct_mer_id,
    report_state = 'processing',
    applet_auth_state = 'pending',
    platform_biz_no = NULL,
    failure_code = NULL,
    failure_message = NULL,
    raw_snapshot = EXCLUDED.raw_snapshot,
    updated_at = now()
RETURNING id, owner_type, owner_id, report_type, report_no, bct_mer_id, sub_mch_id, report_state, applet_auth_state, platform_biz_no, failure_code, failure_message, raw_snapshot, created_at, updated_at;

-- name: GetBaofuMerchantReportByOwner :one
SELECT id, owner_type, owner_id, report_type, report_no, bct_mer_id, sub_mch_id, report_state, applet_auth_state, platform_biz_no, failure_code, failure_message, raw_snapshot, created_at, updated_at
FROM baofu_merchant_reports
WHERE owner_type = $1 AND owner_id = $2 AND report_type = $3
LIMIT 1;

-- name: GetBaofuMerchantReportByReportNo :one
SELECT id, owner_type, owner_id, report_type, report_no, bct_mer_id, sub_mch_id, report_state, applet_auth_state, platform_biz_no, failure_code, failure_message, raw_snapshot, created_at, updated_at
FROM baofu_merchant_reports
WHERE report_no = $1
LIMIT 1;

-- name: ListRecoverableBaofuMerchantReports :many
SELECT id, owner_type, owner_id, report_type, report_no, bct_mer_id, sub_mch_id, report_state, applet_auth_state, platform_biz_no, failure_code, failure_message, raw_snapshot, created_at, updated_at
FROM baofu_merchant_reports
WHERE report_type = 'WECHAT'
  AND updated_at <= sqlc.arg(updated_before)
  AND (
      report_state = 'processing'
      OR (
          report_state = 'succeeded'
          AND sub_mch_id IS NOT NULL
          AND applet_auth_state = 'pending'
      )
  )
ORDER BY updated_at ASC, id ASC
LIMIT sqlc.arg(limit_count);

-- name: MarkBaofuMerchantReportSucceeded :one
UPDATE baofu_merchant_reports
SET report_state = 'succeeded',
    sub_mch_id = sqlc.narg(sub_mch_id),
    platform_biz_no = sqlc.narg(platform_biz_no),
    failure_code = NULL,
    failure_message = NULL,
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, owner_type, owner_id, report_type, report_no, bct_mer_id, sub_mch_id, report_state, applet_auth_state, platform_biz_no, failure_code, failure_message, raw_snapshot, created_at, updated_at;

-- name: MarkBaofuMerchantReportFailed :one
UPDATE baofu_merchant_reports
SET report_state = 'failed',
    failure_code = sqlc.narg(failure_code),
    failure_message = sqlc.narg(failure_message),
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, owner_type, owner_id, report_type, report_no, bct_mer_id, sub_mch_id, report_state, applet_auth_state, platform_biz_no, failure_code, failure_message, raw_snapshot, created_at, updated_at;

-- name: MarkBaofuMerchantReportAppletAuthSucceeded :one
UPDATE baofu_merchant_reports
SET applet_auth_state = 'succeeded',
    updated_at = now()
WHERE id = $1
RETURNING id, owner_type, owner_id, report_type, report_no, bct_mer_id, sub_mch_id, report_state, applet_auth_state, platform_biz_no, failure_code, failure_message, raw_snapshot, created_at, updated_at;

-- name: MarkBaofuMerchantReportAppletAuthFailed :one
UPDATE baofu_merchant_reports
SET applet_auth_state = 'failed',
    failure_code = sqlc.narg(failure_code),
    failure_message = sqlc.narg(failure_message),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, owner_type, owner_id, report_type, report_no, bct_mer_id, sub_mch_id, report_state, applet_auth_state, platform_biz_no, failure_code, failure_message, raw_snapshot, created_at, updated_at;
