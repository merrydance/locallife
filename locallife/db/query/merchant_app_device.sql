-- name: DeactivateMerchantAppDevicesByPushToken :exec
UPDATE merchant_app_devices
SET status = 'inactive',
    unregistered_at = COALESCE(unregistered_at, now()),
    updated_at = now()
WHERE push_token = sqlc.arg('push_token')
  AND device_id <> sqlc.arg('device_id')
  AND status = 'active';

-- name: DeactivateStaleMerchantAppDevices :execrows
UPDATE merchant_app_devices
SET status = 'inactive',
    unregistered_at = COALESCE(unregistered_at, now()),
    updated_at = now()
WHERE status = 'active'
  AND last_active_at < sqlc.arg('last_active_before');

-- name: RegisterMerchantAppDevice :one
INSERT INTO merchant_app_devices (
    merchant_id,
    user_id,
    device_id,
    platform,
    provider,
    push_token,
    status,
    device_model,
    os_version,
    app_version,
    last_registered_at,
    last_active_at,
    unregistered_at
) VALUES (
    sqlc.arg('merchant_id'),
    sqlc.arg('user_id'),
    sqlc.arg('device_id'),
    sqlc.arg('platform'),
    sqlc.arg('provider'),
    sqlc.arg('push_token'),
    'active',
    sqlc.narg('device_model'),
    sqlc.narg('os_version'),
    sqlc.narg('app_version'),
    now(),
    now(),
    NULL
)
ON CONFLICT (device_id) WHERE status = 'active' DO UPDATE
SET merchant_id = EXCLUDED.merchant_id,
    user_id = EXCLUDED.user_id,
    platform = EXCLUDED.platform,
    provider = EXCLUDED.provider,
    push_token = EXCLUDED.push_token,
    status = 'active',
    device_model = EXCLUDED.device_model,
    os_version = EXCLUDED.os_version,
    app_version = EXCLUDED.app_version,
    last_registered_at = now(),
    last_active_at = now(),
    unregistered_at = NULL,
    updated_at = now()
RETURNING id, merchant_id, user_id, device_id, platform, provider, push_token, status, device_model, os_version, app_version, last_registered_at, last_active_at, unregistered_at, created_at, updated_at;

-- name: GetActiveMerchantAppDevice :one
SELECT id, merchant_id, user_id, device_id, platform, provider, push_token, status, device_model, os_version, app_version, last_registered_at, last_active_at, unregistered_at, created_at, updated_at
FROM merchant_app_devices
WHERE merchant_id = sqlc.arg('merchant_id')
  AND device_id = sqlc.arg('device_id')
  AND status = 'active'
LIMIT 1;

-- name: UpdateMerchantAppDeviceHeartbeat :one
UPDATE merchant_app_devices
SET provider = COALESCE(sqlc.narg('provider'), provider),
    push_token = COALESCE(sqlc.narg('push_token'), push_token),
    device_model = COALESCE(sqlc.narg('device_model'), device_model),
    os_version = COALESCE(sqlc.narg('os_version'), os_version),
    app_version = COALESCE(sqlc.narg('app_version'), app_version),
    last_active_at = now(),
    updated_at = now()
WHERE merchant_id = sqlc.arg('merchant_id')
  AND device_id = sqlc.arg('device_id')
  AND status = 'active'
RETURNING id, merchant_id, user_id, device_id, platform, provider, push_token, status, device_model, os_version, app_version, last_registered_at, last_active_at, unregistered_at, created_at, updated_at;

-- name: UnregisterMerchantAppDevice :execrows
UPDATE merchant_app_devices
SET status = 'inactive',
    unregistered_at = COALESCE(unregistered_at, now()),
    updated_at = now()
WHERE merchant_id = sqlc.arg('merchant_id')
  AND device_id = sqlc.arg('device_id')
  AND status = 'active';

-- name: ListActiveMerchantAppDevicesByMerchant :many
SELECT id, merchant_id, user_id, device_id, platform, provider, push_token, status, device_model, os_version, app_version, last_registered_at, last_active_at, unregistered_at, created_at, updated_at
FROM merchant_app_devices
WHERE merchant_id = sqlc.arg('merchant_id')
  AND status = 'active'
ORDER BY last_active_at DESC, id DESC;

-- name: ListActiveMerchantAppDevicesByMerchantAndProvider :many
SELECT id, merchant_id, user_id, device_id, platform, provider, push_token, status, device_model, os_version, app_version, last_registered_at, last_active_at, unregistered_at, created_at, updated_at
FROM merchant_app_devices
WHERE merchant_id = sqlc.arg('merchant_id')
  AND provider = sqlc.arg('provider')
  AND status = 'active'
ORDER BY last_active_at DESC, id DESC;
