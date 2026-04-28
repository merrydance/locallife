-- name: UpsertProfitSharingReceiverTarget :one
INSERT INTO profit_sharing_receiver_targets (
    provider,
    channel,
    owner_type,
    owner_id,
    receiver_type,
    appid,
    account_hash,
    display_name_hash,
    desired_state,
    sync_status
) VALUES (
    sqlc.arg(provider),
    sqlc.arg(channel),
    sqlc.arg(owner_type),
    sqlc.arg(owner_id),
    sqlc.arg(receiver_type),
    sqlc.arg(appid),
    sqlc.arg(account_hash),
    sqlc.narg(display_name_hash),
    sqlc.arg(desired_state),
    'pending'
)
ON CONFLICT (provider, channel, owner_type, owner_id, receiver_type, appid, account_hash)
DO UPDATE SET desired_state = EXCLUDED.desired_state,
    display_name_hash = EXCLUDED.display_name_hash,
    sync_status = 'pending',
    next_retry_at = NULL,
    last_error_code = NULL,
    last_error_message = NULL,
    synced_at = NULL,
    skipped_at = NULL,
    updated_at = now()
RETURNING id, provider, channel, owner_type, owner_id, receiver_type, appid, account_hash, display_name_hash, desired_state, sync_status, attempt_count, next_retry_at, last_error_code, last_error_message, last_attempt_at, synced_at, skipped_at, created_at, updated_at;

-- name: GetProfitSharingReceiverTarget :one
SELECT id, provider, channel, owner_type, owner_id, receiver_type, appid, account_hash, display_name_hash, desired_state, sync_status, attempt_count, next_retry_at, last_error_code, last_error_message, last_attempt_at, synced_at, skipped_at, created_at, updated_at
FROM profit_sharing_receiver_targets
WHERE id = $1
LIMIT 1;

-- name: GetProfitSharingReceiverTargetByKey :one
SELECT id, provider, channel, owner_type, owner_id, receiver_type, appid, account_hash, display_name_hash, desired_state, sync_status, attempt_count, next_retry_at, last_error_code, last_error_message, last_attempt_at, synced_at, skipped_at, created_at, updated_at
FROM profit_sharing_receiver_targets
WHERE provider = sqlc.arg(provider)
    AND channel = sqlc.arg(channel)
    AND owner_type = sqlc.arg(owner_type)
    AND owner_id = sqlc.arg(owner_id)
    AND receiver_type = sqlc.arg(receiver_type)
    AND appid = sqlc.arg(appid)
    AND account_hash = sqlc.arg(account_hash)
LIMIT 1;

-- name: ListProfitSharingReceiverTargetsByOwner :many
SELECT id, provider, channel, owner_type, owner_id, receiver_type, appid, account_hash, display_name_hash, desired_state, sync_status, attempt_count, next_retry_at, last_error_code, last_error_message, last_attempt_at, synced_at, skipped_at, created_at, updated_at
FROM profit_sharing_receiver_targets
WHERE owner_type = sqlc.arg(owner_type)
    AND owner_id = sqlc.arg(owner_id)
ORDER BY id ASC;

-- name: ListProfitSharingReceiverTargets :many
SELECT id, provider, channel, owner_type, owner_id, receiver_type, appid, account_hash, display_name_hash, desired_state, sync_status, attempt_count, next_retry_at, last_error_code, last_error_message, last_attempt_at, synced_at, skipped_at, created_at, updated_at
FROM profit_sharing_receiver_targets
WHERE (sqlc.narg(owner_type)::text IS NULL OR owner_type = sqlc.narg(owner_type))
    AND (sqlc.narg(owner_id)::bigint IS NULL OR owner_id = sqlc.narg(owner_id))
    AND (sqlc.narg(sync_status)::text IS NULL OR sync_status = sqlc.narg(sync_status))
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountProfitSharingReceiverTargets :one
SELECT count(*)::bigint
FROM profit_sharing_receiver_targets
WHERE (sqlc.narg(owner_type)::text IS NULL OR owner_type = sqlc.narg(owner_type))
    AND (sqlc.narg(owner_id)::bigint IS NULL OR owner_id = sqlc.narg(owner_id))
    AND (sqlc.narg(sync_status)::text IS NULL OR sync_status = sqlc.narg(sync_status));

-- name: ListRetryableProfitSharingReceiverTargetsByOwnerType :many
SELECT id, provider, channel, owner_type, owner_id, receiver_type, appid, account_hash, display_name_hash, desired_state, sync_status, attempt_count, next_retry_at, last_error_code, last_error_message, last_attempt_at, synced_at, skipped_at, created_at, updated_at
FROM profit_sharing_receiver_targets
WHERE owner_type = sqlc.arg(owner_type)
    AND sync_status IN ('pending', 'failed')
    AND (next_retry_at IS NULL OR next_retry_at <= sqlc.arg(now_at))
ORDER BY next_retry_at ASC NULLS FIRST, id ASC
LIMIT sqlc.arg(limit_count);

-- name: ClaimPendingProfitSharingReceiverTargets :many
WITH candidates AS (
    SELECT target.id, target.next_retry_at
    FROM profit_sharing_receiver_targets AS target
    WHERE target.sync_status IN ('pending', 'failed')
        AND (target.next_retry_at IS NULL OR target.next_retry_at <= sqlc.arg(now_at))
    ORDER BY target.next_retry_at ASC NULLS FIRST, target.id ASC
    LIMIT sqlc.arg(limit_count)
    FOR UPDATE SKIP LOCKED
), claimed AS (
    UPDATE profit_sharing_receiver_targets AS target
    SET sync_status = 'processing',
        attempt_count = target.attempt_count + 1,
        last_attempt_at = sqlc.arg(now_at),
        updated_at = now()
    FROM candidates
    WHERE target.id = candidates.id
    RETURNING target.id, target.provider, target.channel, target.owner_type, target.owner_id, target.receiver_type, target.appid, target.account_hash, target.display_name_hash, target.desired_state, target.sync_status, target.attempt_count, target.next_retry_at, target.last_error_code, target.last_error_message, target.last_attempt_at, target.synced_at, target.skipped_at, target.created_at, target.updated_at
)
SELECT claimed.id, claimed.provider, claimed.channel, claimed.owner_type, claimed.owner_id, claimed.receiver_type, claimed.appid, claimed.account_hash, claimed.display_name_hash, claimed.desired_state, claimed.sync_status, claimed.attempt_count, claimed.next_retry_at, claimed.last_error_code, claimed.last_error_message, claimed.last_attempt_at, claimed.synced_at, claimed.skipped_at, claimed.created_at, claimed.updated_at
FROM claimed
JOIN candidates ON candidates.id = claimed.id
ORDER BY candidates.next_retry_at ASC NULLS FIRST, claimed.id ASC;

-- name: ClaimProfitSharingReceiverTarget :one
UPDATE profit_sharing_receiver_targets
SET sync_status = 'processing',
    attempt_count = attempt_count + 1,
    last_attempt_at = sqlc.arg(now_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND sync_status IN ('pending', 'failed')
    AND (next_retry_at IS NULL OR next_retry_at <= sqlc.arg(now_at))
RETURNING id, provider, channel, owner_type, owner_id, receiver_type, appid, account_hash, display_name_hash, desired_state, sync_status, attempt_count, next_retry_at, last_error_code, last_error_message, last_attempt_at, synced_at, skipped_at, created_at, updated_at;

-- name: MarkProfitSharingReceiverTargetSynced :one
UPDATE profit_sharing_receiver_targets
SET sync_status = 'synced',
    last_error_code = NULL,
    last_error_message = NULL,
    next_retry_at = NULL,
    synced_at = sqlc.arg(synced_at),
    skipped_at = NULL,
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND sync_status = 'processing'
RETURNING id, provider, channel, owner_type, owner_id, receiver_type, appid, account_hash, display_name_hash, desired_state, sync_status, attempt_count, next_retry_at, last_error_code, last_error_message, last_attempt_at, synced_at, skipped_at, created_at, updated_at;

-- name: MarkProfitSharingReceiverTargetFailed :one
UPDATE profit_sharing_receiver_targets
SET sync_status = 'failed',
    last_error_code = sqlc.narg(last_error_code),
    last_error_message = sqlc.narg(last_error_message),
    next_retry_at = sqlc.narg(next_retry_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND sync_status = 'processing'
RETURNING id, provider, channel, owner_type, owner_id, receiver_type, appid, account_hash, display_name_hash, desired_state, sync_status, attempt_count, next_retry_at, last_error_code, last_error_message, last_attempt_at, synced_at, skipped_at, created_at, updated_at;

-- name: MarkProfitSharingReceiverTargetSkipped :one
UPDATE profit_sharing_receiver_targets
SET sync_status = 'skipped',
    last_error_code = NULL,
    last_error_message = NULL,
    next_retry_at = NULL,
    synced_at = NULL,
    skipped_at = sqlc.arg(skipped_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND sync_status = 'processing'
RETURNING id, provider, channel, owner_type, owner_id, receiver_type, appid, account_hash, display_name_hash, desired_state, sync_status, attempt_count, next_retry_at, last_error_code, last_error_message, last_attempt_at, synced_at, skipped_at, created_at, updated_at;

-- name: CreateProfitSharingReceiverAttempt :one
INSERT INTO profit_sharing_receiver_attempts (
    target_id,
    action,
    status,
    started_at
) VALUES (
    sqlc.arg(target_id),
    sqlc.arg(action),
    sqlc.arg(status),
    sqlc.arg(started_at)
)
RETURNING id, target_id, action, status, idempotent_success, error_code, error_message, started_at, finished_at, created_at;

-- name: MarkProfitSharingReceiverAttemptSucceeded :one
UPDATE profit_sharing_receiver_attempts
SET status = 'succeeded',
    idempotent_success = sqlc.arg(idempotent_success),
    error_code = NULL,
    error_message = NULL,
    finished_at = sqlc.arg(finished_at)
WHERE id = sqlc.arg(id)
    AND status = 'processing'
RETURNING id, target_id, action, status, idempotent_success, error_code, error_message, started_at, finished_at, created_at;

-- name: MarkProfitSharingReceiverAttemptFailed :one
UPDATE profit_sharing_receiver_attempts
SET status = 'failed',
    error_code = sqlc.narg(error_code),
    error_message = sqlc.narg(error_message),
    finished_at = sqlc.arg(finished_at)
WHERE id = sqlc.arg(id)
    AND status = 'processing'
RETURNING id, target_id, action, status, idempotent_success, error_code, error_message, started_at, finished_at, created_at;

-- name: MarkProfitSharingReceiverAttemptSkipped :one
UPDATE profit_sharing_receiver_attempts
SET status = 'skipped',
    error_code = NULL,
    error_message = NULL,
    finished_at = sqlc.arg(finished_at)
WHERE id = sqlc.arg(id)
    AND status = 'processing'
RETURNING id, target_id, action, status, idempotent_success, error_code, error_message, started_at, finished_at, created_at;

-- name: ListProfitSharingReceiverAttemptsByTarget :many
SELECT id, target_id, action, status, idempotent_success, error_code, error_message, started_at, finished_at, created_at
FROM profit_sharing_receiver_attempts
WHERE target_id = $1
ORDER BY id DESC;

-- name: ListProfitSharingReceiverAttemptsByTargetPaginated :many
SELECT id, target_id, action, status, idempotent_success, error_code, error_message, started_at, finished_at, created_at
FROM profit_sharing_receiver_attempts
WHERE target_id = sqlc.arg(target_id)
ORDER BY id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountProfitSharingReceiverAttemptsByTarget :one
SELECT count(*)::bigint
FROM profit_sharing_receiver_attempts
WHERE target_id = sqlc.arg(target_id);
