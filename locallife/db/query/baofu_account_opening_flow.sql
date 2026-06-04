-- name: CreateBaofuAccountOpeningFlow :one
INSERT INTO baofu_account_opening_flows (
    owner_type,
    owner_id,
    account_type,
    opening_mode,
    profile_id,
    state,
    verify_fee_amount,
    verify_fee_payment_order_id,
    open_trans_serial_no,
    login_no,
    provider_request_snapshot,
    raw_snapshot
) VALUES (
    sqlc.arg(owner_type),
    sqlc.arg(owner_id),
    sqlc.arg(account_type),
    sqlc.arg(opening_mode),
    sqlc.narg(profile_id),
    sqlc.arg(state),
    sqlc.arg(verify_fee_amount),
    sqlc.narg(verify_fee_payment_order_id),
    sqlc.narg(open_trans_serial_no),
    sqlc.narg(login_no),
    sqlc.arg(provider_request_snapshot),
    sqlc.arg(raw_snapshot)
)
RETURNING id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode;

-- name: GetBaofuAccountOpeningFlow :one
SELECT id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode
FROM baofu_account_opening_flows
WHERE id = $1
LIMIT 1;

-- name: GetActiveBaofuAccountOpeningFlowByOwner :one
SELECT id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode
FROM baofu_account_opening_flows
WHERE owner_type = $1
  AND owner_id = $2
  AND state IN (
      'profile_pending',
      'verify_fee_pending',
      'verify_fee_processing',
      'opening_processing',
      'merchant_report_processing',
      'applet_auth_pending'
  )
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetLatestBaofuAccountOpeningFlowByOwner :one
SELECT id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode
FROM baofu_account_opening_flows
WHERE owner_type = $1 AND owner_id = $2
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetBaofuAccountOpeningFlowByOpenTransSerialNo :one
SELECT id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode
FROM baofu_account_opening_flows
WHERE open_trans_serial_no = $1
LIMIT 1;

-- name: GetBaofuAccountOpeningFlowByPaymentOrder :one
SELECT id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode
FROM baofu_account_opening_flows
WHERE verify_fee_payment_order_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: SetBaofuAccountOpeningFlowProfilePending :one
UPDATE baofu_account_opening_flows
SET
    profile_id = sqlc.narg(profile_id),
    state = 'profile_pending',
    failure_code = NULL,
    failure_message = NULL,
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND state IN ('profile_pending', 'verify_fee_pending', 'verify_fee_processing', 'failed')
RETURNING id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode;

-- name: MarkBaofuAccountOpeningFlowVerifyFeePending :one
UPDATE baofu_account_opening_flows
SET
    profile_id = sqlc.narg(profile_id),
    state = 'verify_fee_pending',
    verify_fee_amount = sqlc.arg(verify_fee_amount),
    verify_fee_payment_order_id = sqlc.narg(verify_fee_payment_order_id),
    failure_code = NULL,
    failure_message = NULL,
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND state IN ('profile_pending', 'verify_fee_pending', 'verify_fee_processing')
RETURNING id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode;

-- name: MarkBaofuAccountOpeningFlowVerifyFeeProcessing :one
UPDATE baofu_account_opening_flows
SET
    state = 'verify_fee_processing',
    verify_fee_payment_order_id = sqlc.narg(verify_fee_payment_order_id),
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND state IN ('verify_fee_pending', 'verify_fee_processing')
RETURNING id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode;

-- name: MarkBaofuAccountOpeningFlowOpeningProcessing :one
UPDATE baofu_account_opening_flows
SET
    profile_id = sqlc.narg(profile_id),
    state = 'opening_processing',
    verify_fee_payment_order_id = COALESCE(sqlc.narg(verify_fee_payment_order_id), verify_fee_payment_order_id),
    open_trans_serial_no = COALESCE(sqlc.narg(open_trans_serial_no), open_trans_serial_no),
    login_no = COALESCE(sqlc.narg(login_no), login_no),
    failure_code = NULL,
    failure_message = NULL,
    provider_request_snapshot = sqlc.arg(provider_request_snapshot),
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND state IN ('profile_pending', 'verify_fee_pending', 'verify_fee_processing', 'opening_processing')
RETURNING id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode;

-- name: MarkBaofuAccountOpeningFlowMerchantReportProcessing :one
UPDATE baofu_account_opening_flows
SET
    state = 'merchant_report_processing',
    account_binding_id = sqlc.narg(account_binding_id),
    merchant_report_id = sqlc.narg(merchant_report_id),
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND state IN ('opening_processing', 'merchant_report_processing')
RETURNING id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode;

-- name: MarkBaofuAccountOpeningFlowAppletAuthPending :one
UPDATE baofu_account_opening_flows
SET
    state = 'applet_auth_pending',
    merchant_report_id = COALESCE(sqlc.narg(merchant_report_id), merchant_report_id),
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND state IN ('merchant_report_processing', 'applet_auth_pending')
RETURNING id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode;

-- name: MarkBaofuAccountOpeningFlowReady :one
UPDATE baofu_account_opening_flows
SET
    state = 'ready',
    account_binding_id = COALESCE(sqlc.narg(account_binding_id), account_binding_id),
    merchant_report_id = COALESCE(sqlc.narg(merchant_report_id), merchant_report_id),
    failure_code = NULL,
    failure_message = NULL,
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND (
      state IN ('opening_processing', 'merchant_report_processing', 'applet_auth_pending', 'ready')
      OR (state = 'failed' AND failure_code IN ('BF00060', 'EXISTED_LOGIN_NO'))
  )
RETURNING id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode;

-- name: RecoverFailedBaofuAccountOpeningFlowFromActiveBinding :one
UPDATE baofu_account_opening_flows
SET
    state = CASE
        WHEN baofu_account_opening_flows.owner_type = 'merchant'
             AND baofu_account_opening_flows.state = 'failed' THEN 'merchant_report_processing'
        ELSE 'ready'
    END,
    account_binding_id = sqlc.arg(account_binding_id),
    failure_code = NULL,
    failure_message = NULL,
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE baofu_account_opening_flows.id = sqlc.arg(id)
  AND baofu_account_opening_flows.state IN ('failed', 'ready')
  AND baofu_account_opening_flows.open_trans_serial_no = sqlc.arg(open_trans_serial_no)
  AND EXISTS (
      SELECT 1
      FROM baofu_account_bindings AS b
      WHERE b.id = sqlc.arg(account_binding_id)
        AND b.owner_type = baofu_account_opening_flows.owner_type
        AND b.owner_id = baofu_account_opening_flows.owner_id
        AND b.account_type = baofu_account_opening_flows.account_type
        AND b.opening_mode = baofu_account_opening_flows.opening_mode
        AND b.open_state = 'active'
        AND b.last_open_trans_serial_no = baofu_account_opening_flows.open_trans_serial_no
        AND b.contract_no = sqlc.arg(contract_no)
  )
RETURNING id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode;

-- name: MarkBaofuAccountOpeningFlowFailed :one
UPDATE baofu_account_opening_flows
SET
    state = 'failed',
    failure_code = sqlc.narg(failure_code),
    failure_message = sqlc.narg(failure_message),
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND state <> 'ready'
  AND state <> 'voided'
RETURNING id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode;

-- name: VoidBaofuAccountOpeningFlow :one
UPDATE baofu_account_opening_flows
SET
    state = 'voided',
    failure_code = sqlc.narg(failure_code),
    failure_message = sqlc.narg(failure_message),
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND state IN (
      'profile_pending',
      'verify_fee_pending',
      'verify_fee_processing',
      'opening_processing',
      'merchant_report_processing',
      'applet_auth_pending',
      'failed'
  )
RETURNING id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode;

-- name: ListRecoverableBaofuAccountOpeningFlows :many
SELECT id, owner_type, owner_id, account_type, profile_id, state, verify_fee_amount, verify_fee_payment_order_id, open_trans_serial_no, login_no, account_binding_id, merchant_report_id, failure_code, failure_message, provider_request_snapshot, raw_snapshot, created_at, updated_at, opening_mode
FROM baofu_account_opening_flows
WHERE (
    state IN ('opening_processing', 'merchant_report_processing', 'applet_auth_pending')
    OR (
        state = 'failed'
        AND failure_code IN ('BF00060', 'EXISTED_LOGIN_NO')
        AND id = (
            SELECT latest.id
            FROM baofu_account_opening_flows latest
            WHERE latest.owner_type = baofu_account_opening_flows.owner_type
              AND latest.owner_id = baofu_account_opening_flows.owner_id
            ORDER BY latest.created_at DESC, latest.id DESC
            LIMIT 1
        )
    )
)
AND baofu_account_opening_flows.updated_at <= sqlc.arg(before_at)
ORDER BY updated_at ASC, id ASC
LIMIT sqlc.arg(limit_count);
