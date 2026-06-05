-- name: CreateCloudPrinterAuthorizationSession :one
INSERT INTO cloud_printer_authorization_sessions (
    state,
    merchant_id,
    provider_type,
    printer_name,
    printer_role,
    created_by,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING id, state, merchant_id, provider_type, printer_name, printer_role, created_by, expires_at, consumed_at, created_at, updated_at;

-- name: GetActiveCloudPrinterAuthorizationSessionForUpdate :one
SELECT id, state, merchant_id, provider_type, printer_name, printer_role, created_by, expires_at, consumed_at, created_at, updated_at
FROM cloud_printer_authorization_sessions
WHERE state = $1
  AND consumed_at IS NULL
  AND expires_at > now()
LIMIT 1
FOR UPDATE;

-- name: ConsumeCloudPrinterAuthorizationSession :one
UPDATE cloud_printer_authorization_sessions
SET
    consumed_at = $2,
    updated_at = now()
WHERE id = $1
  AND consumed_at IS NULL
  AND expires_at > now()
RETURNING id, state, merchant_id, provider_type, printer_name, printer_role, created_by, expires_at, consumed_at, created_at, updated_at;

-- name: UpsertCloudPrinterProviderAuthorization :one
INSERT INTO cloud_printer_provider_authorizations (
    merchant_id,
    provider_type,
    machine_code,
    authorized_cloud_printer_id,
    access_token_ciphertext,
    refresh_token_ciphertext,
    access_token_expires_at,
    refresh_token_expires_at,
    status,
    refresh_failure_count,
    refresh_last_attempted_at,
    last_provider_error
)
SELECT
    sqlc.arg(merchant_id),
    sqlc.arg(provider_type),
    sqlc.arg(machine_code),
    sqlc.narg(authorized_cloud_printer_id),
    sqlc.arg(access_token_ciphertext),
    sqlc.arg(refresh_token_ciphertext),
    sqlc.arg(access_token_expires_at),
    sqlc.arg(refresh_token_expires_at),
    sqlc.arg(status),
    sqlc.arg(refresh_failure_count),
    sqlc.narg(refresh_last_attempted_at),
    sqlc.narg(last_provider_error)
WHERE sqlc.narg(authorized_cloud_printer_id)::bigint IS NULL
   OR EXISTS (
       SELECT 1
       FROM cloud_printers
       WHERE id = sqlc.narg(authorized_cloud_printer_id)
         AND merchant_id = sqlc.arg(merchant_id)
   )
ON CONFLICT (provider_type, machine_code) DO UPDATE
SET
    merchant_id = EXCLUDED.merchant_id,
    authorized_cloud_printer_id = COALESCE(EXCLUDED.authorized_cloud_printer_id, cloud_printer_provider_authorizations.authorized_cloud_printer_id),
    access_token_ciphertext = EXCLUDED.access_token_ciphertext,
    refresh_token_ciphertext = EXCLUDED.refresh_token_ciphertext,
    access_token_expires_at = EXCLUDED.access_token_expires_at,
    refresh_token_expires_at = EXCLUDED.refresh_token_expires_at,
    status = EXCLUDED.status,
    refresh_failure_count = EXCLUDED.refresh_failure_count,
    refresh_last_attempted_at = EXCLUDED.refresh_last_attempted_at,
    last_provider_error = EXCLUDED.last_provider_error,
    updated_at = now()
WHERE cloud_printer_provider_authorizations.merchant_id = EXCLUDED.merchant_id
  AND (
      EXCLUDED.authorized_cloud_printer_id IS NULL
      OR EXISTS (
          SELECT 1
          FROM cloud_printers
          WHERE id = EXCLUDED.authorized_cloud_printer_id
            AND merchant_id = EXCLUDED.merchant_id
      )
  )
RETURNING id, merchant_id, provider_type, machine_code, authorized_cloud_printer_id, access_token_ciphertext, refresh_token_ciphertext, access_token_expires_at, refresh_token_expires_at, status, refresh_failure_count, refresh_last_attempted_at, last_provider_error, created_at, updated_at;

-- name: GetCloudPrinterProviderAuthorizationByMerchantAndMachineCode :one
SELECT id, merchant_id, provider_type, machine_code, authorized_cloud_printer_id, access_token_ciphertext, refresh_token_ciphertext, access_token_expires_at, refresh_token_expires_at, status, refresh_failure_count, refresh_last_attempted_at, last_provider_error, created_at, updated_at
FROM cloud_printer_provider_authorizations
WHERE merchant_id = $1
  AND provider_type = $2
  AND machine_code = $3
LIMIT 1;

-- name: ListCloudPrinterProviderAuthorizationsByMerchant :many
SELECT id, merchant_id, provider_type, machine_code, authorized_cloud_printer_id, access_token_ciphertext, refresh_token_ciphertext, access_token_expires_at, refresh_token_expires_at, status, refresh_failure_count, refresh_last_attempted_at, last_provider_error, created_at, updated_at
FROM cloud_printer_provider_authorizations
WHERE merchant_id = $1
  AND provider_type = $2
ORDER BY created_at DESC, id DESC;

-- name: AttachCloudPrinterProviderAuthorizationToPrinter :one
UPDATE cloud_printer_provider_authorizations
SET
    authorized_cloud_printer_id = $4,
    updated_at = now()
WHERE cloud_printer_provider_authorizations.merchant_id = $1
  AND cloud_printer_provider_authorizations.provider_type = $2
  AND cloud_printer_provider_authorizations.machine_code = $3
  AND EXISTS (
      SELECT 1
      FROM cloud_printers
      WHERE cloud_printers.id = $4
        AND cloud_printers.merchant_id = $1
        AND cloud_printers.printer_type = $2
        AND cloud_printers.printer_sn = $3
        AND ($2 <> 'yilianyun' OR cloud_printers.printer_key = '')
  )
  AND (
      cloud_printer_provider_authorizations.authorized_cloud_printer_id IS NULL
      OR cloud_printer_provider_authorizations.authorized_cloud_printer_id = $4
  )
RETURNING id, merchant_id, provider_type, machine_code, authorized_cloud_printer_id, access_token_ciphertext, refresh_token_ciphertext, access_token_expires_at, refresh_token_expires_at, status, refresh_failure_count, refresh_last_attempted_at, last_provider_error, created_at, updated_at;
