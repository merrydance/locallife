-- name: CreateMerchantCredentialLedger :one
INSERT INTO credential_ledgers (
    subject_type,
    merchant_id,
    document_type,
    merchant_application_id,
    review_run_id,
    media_asset_id,
    normalized_payload,
    expires_at,
    activated_at
) VALUES (
    'merchant',
    sqlc.arg(merchant_id),
    sqlc.arg(document_type),
    sqlc.arg(merchant_application_id),
    sqlc.arg(review_run_id),
    sqlc.arg(media_asset_id),
    sqlc.arg(normalized_payload),
    sqlc.narg(expires_at),
    COALESCE(sqlc.narg(activated_at), NOW())
)
RETURNING id, subject_type, merchant_id, rider_id, document_type, merchant_application_id, rider_application_id, review_run_id, media_asset_id, normalized_payload, expires_at, active, activated_at, deactivated_at, last_reminded_at, suspended_at, resumed_at, suspension_reason_code, created_at, updated_at;

-- name: CreateRiderCredentialLedger :one
INSERT INTO credential_ledgers (
    subject_type,
    rider_id,
    document_type,
    rider_application_id,
    review_run_id,
    media_asset_id,
    normalized_payload,
    expires_at,
    activated_at
) VALUES (
    'rider',
    sqlc.arg(rider_id),
    sqlc.arg(document_type),
    sqlc.arg(rider_application_id),
    sqlc.arg(review_run_id),
    sqlc.arg(media_asset_id),
    sqlc.arg(normalized_payload),
    sqlc.narg(expires_at),
    COALESCE(sqlc.narg(activated_at), NOW())
)
RETURNING id, subject_type, merchant_id, rider_id, document_type, merchant_application_id, rider_application_id, review_run_id, media_asset_id, normalized_payload, expires_at, active, activated_at, deactivated_at, last_reminded_at, suspended_at, resumed_at, suspension_reason_code, created_at, updated_at;

-- name: GetCredentialLedger :one
SELECT id, subject_type, merchant_id, rider_id, document_type, merchant_application_id, rider_application_id, review_run_id, media_asset_id, normalized_payload, expires_at, active, activated_at, deactivated_at, last_reminded_at, suspended_at, resumed_at, suspension_reason_code, created_at, updated_at
FROM credential_ledgers
WHERE id = $1;

-- name: DeactivateMerchantActiveCredentialLedger :execrows
UPDATE credential_ledgers
SET active = false,
    deactivated_at = COALESCE(sqlc.narg(deactivated_at), NOW()),
    updated_at = NOW()
WHERE merchant_id = sqlc.arg(merchant_id)
  AND document_type = sqlc.arg(document_type)
  AND active = true;

-- name: DeactivateRiderActiveCredentialLedger :execrows
UPDATE credential_ledgers
SET active = false,
    deactivated_at = COALESCE(sqlc.narg(deactivated_at), NOW()),
    updated_at = NOW()
WHERE rider_id = sqlc.arg(rider_id)
  AND document_type = sqlc.arg(document_type)
  AND active = true;

-- name: GetActiveMerchantCredentialLedgers :many
SELECT id, subject_type, merchant_id, rider_id, document_type, merchant_application_id, rider_application_id, review_run_id, media_asset_id, normalized_payload, expires_at, active, activated_at, deactivated_at, last_reminded_at, suspended_at, resumed_at, suspension_reason_code, created_at, updated_at
FROM credential_ledgers
WHERE merchant_id = $1
  AND active = true
ORDER BY document_type ASC, activated_at DESC, id DESC;

-- name: GetActiveRiderCredentialLedgers :many
SELECT id, subject_type, merchant_id, rider_id, document_type, merchant_application_id, rider_application_id, review_run_id, media_asset_id, normalized_payload, expires_at, active, activated_at, deactivated_at, last_reminded_at, suspended_at, resumed_at, suspension_reason_code, created_at, updated_at
FROM credential_ledgers
WHERE rider_id = $1
  AND active = true
ORDER BY document_type ASC, activated_at DESC, id DESC;

-- name: ListCredentialsForReminderWindow :many
SELECT id,
       subject_type,
       merchant_id,
       rider_id,
       document_type,
       merchant_application_id,
       rider_application_id,
       review_run_id,
       media_asset_id,
       normalized_payload,
       expires_at,
       active,
       activated_at,
       deactivated_at,
       last_reminded_at,
       suspended_at,
       resumed_at,
       suspension_reason_code,
       created_at,
       updated_at
FROM credential_ledgers
WHERE active = true
  AND expires_at IS NOT NULL
  AND expires_at > sqlc.arg(window_start)
  AND expires_at <= sqlc.arg(window_end)
  AND (last_reminded_at IS NULL OR last_reminded_at < sqlc.arg(window_start))
  AND (
        sqlc.narg(last_expires_at)::timestamptz IS NULL
        OR expires_at > sqlc.narg(last_expires_at)::timestamptz
        OR (expires_at = sqlc.narg(last_expires_at)::timestamptz AND id > sqlc.arg(last_id))
      )
ORDER BY expires_at ASC, id ASC
LIMIT sqlc.arg(page_limit);

-- name: ListExpiredActiveCredentialLedgers :many
SELECT id,
       subject_type,
       merchant_id,
       rider_id,
       document_type,
       merchant_application_id,
       rider_application_id,
       review_run_id,
       media_asset_id,
       normalized_payload,
       expires_at,
       active,
       activated_at,
       deactivated_at,
       last_reminded_at,
       suspended_at,
       resumed_at,
       suspension_reason_code,
       created_at,
       updated_at
FROM credential_ledgers
WHERE active = true
  AND expires_at IS NOT NULL
  AND expires_at <= sqlc.arg(expired_before)
  AND (
        sqlc.narg(last_expires_at)::timestamptz IS NULL
        OR expires_at > sqlc.narg(last_expires_at)::timestamptz
        OR (expires_at = sqlc.narg(last_expires_at)::timestamptz AND id > sqlc.arg(last_id))
      )
ORDER BY expires_at ASC, id ASC
LIMIT sqlc.arg(page_limit);

-- name: MarkCredentialLedgerReminderSent :one
UPDATE credential_ledgers
SET last_reminded_at = sqlc.arg(last_reminded_at),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING id, subject_type, merchant_id, rider_id, document_type, merchant_application_id, rider_application_id, review_run_id, media_asset_id, normalized_payload, expires_at, active, activated_at, deactivated_at, last_reminded_at, suspended_at, resumed_at, suspension_reason_code, created_at, updated_at;

-- name: MarkCredentialLedgerSuspended :one
UPDATE credential_ledgers
SET suspended_at = COALESCE(suspended_at, sqlc.arg(suspended_at)),
    suspension_reason_code = sqlc.arg(suspension_reason_code),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING id, subject_type, merchant_id, rider_id, document_type, merchant_application_id, rider_application_id, review_run_id, media_asset_id, normalized_payload, expires_at, active, activated_at, deactivated_at, last_reminded_at, suspended_at, resumed_at, suspension_reason_code, created_at, updated_at;

-- name: MarkCredentialLedgerResumed :one
UPDATE credential_ledgers
SET resumed_at = sqlc.arg(resumed_at),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING id, subject_type, merchant_id, rider_id, document_type, merchant_application_id, rider_application_id, review_run_id, media_asset_id, normalized_payload, expires_at, active, activated_at, deactivated_at, last_reminded_at, suspended_at, resumed_at, suspension_reason_code, created_at, updated_at;