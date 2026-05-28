-- name: UpsertOCRJob :one
INSERT INTO ocr_jobs (
    idempotency_key,
    document_type,
    provider,
    media_asset_id,
    owner_type,
    owner_id,
    side,
    max_attempts,
    retention_until,
    requested_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (idempotency_key) DO UPDATE
SET updated_at = ocr_jobs.updated_at
WHERE ocr_jobs.document_type = EXCLUDED.document_type
  AND ocr_jobs.provider = EXCLUDED.provider
  AND ocr_jobs.media_asset_id = EXCLUDED.media_asset_id
  AND ocr_jobs.owner_type = EXCLUDED.owner_type
  AND ocr_jobs.owner_id = EXCLUDED.owner_id
  AND ocr_jobs.side = EXCLUDED.side
  AND ocr_jobs.requested_by = EXCLUDED.requested_by
RETURNING *;

-- name: GetOCRJob :one
SELECT id, idempotency_key, document_type, provider, provider_task_id, media_asset_id, owner_type, owner_id, side, status, attempt_count, max_attempts, next_retry_at, leased_at, lease_owner, error_code, error_message, raw_result, normalized_result, result_version, retention_until, requested_by, created_at, started_at, finished_at, updated_at FROM ocr_jobs
WHERE id = $1;

-- name: ListOCRJobsByOwner :many
SELECT id, idempotency_key, document_type, provider, provider_task_id, media_asset_id, owner_type, owner_id, side, status, attempt_count, max_attempts, next_retry_at, leased_at, lease_owner, error_code, error_message, raw_result, normalized_result, result_version, retention_until, requested_by, created_at, started_at, finished_at, updated_at FROM ocr_jobs
WHERE owner_type = $1
  AND owner_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: ListPendingOCRJobsByMediaAsset :many
SELECT id, idempotency_key, document_type, provider, provider_task_id, media_asset_id, owner_type, owner_id, side, status, attempt_count, max_attempts, next_retry_at, leased_at, lease_owner, error_code, error_message, raw_result, normalized_result, result_version, retention_until, requested_by, created_at, started_at, finished_at, updated_at FROM ocr_jobs
WHERE media_asset_id = $1
  AND status = 'pending'
ORDER BY created_at ASC, id ASC;

-- name: ListOCRDeadLetterJobs :many
SELECT id, idempotency_key, document_type, provider, provider_task_id, media_asset_id, owner_type, owner_id, side, status, attempt_count, max_attempts, next_retry_at, leased_at, lease_owner, error_code, error_message, raw_result, normalized_result, result_version, retention_until, requested_by, created_at, started_at, finished_at, updated_at FROM ocr_jobs
WHERE status IN ('failed', 'cancelled')
  AND next_retry_at IS NULL
  AND (
    status = 'cancelled'
    OR attempt_count >= max_attempts
    OR error_code IN (
      'ocr_provider_unauthorized',
      'ocr_provider_forbidden',
      'ocr_bad_request',
      'ocr_media_not_found',
      'ocr_execution_failed'
    )
  )
  AND ((sqlc.arg(owner_type))::text = '' OR owner_type = (sqlc.arg(owner_type))::text)
  AND ((sqlc.arg(document_type))::text = '' OR document_type = (sqlc.arg(document_type))::text)
ORDER BY COALESCE(finished_at, updated_at, created_at) DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: MarkOCRJobProcessing :one
UPDATE ocr_jobs
SET status = 'processing',
    attempt_count = attempt_count + 1,
    next_retry_at = NULL,
    lease_owner = sqlc.arg(lease_owner),
    leased_at = now(),
    started_at = COALESCE(started_at, now()),
    error_code = NULL,
    error_message = NULL,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND (
    status = 'pending'
    OR (
      status = 'processing'
      AND leased_at IS NOT NULL
      AND leased_at < sqlc.arg(lease_expires_before)
    )
  )
RETURNING *;

-- name: CompleteOCRJob :one
UPDATE ocr_jobs
SET status = 'succeeded',
    provider_task_id = COALESCE($2, provider_task_id),
    raw_result = $3,
    normalized_result = $4,
    result_version = $5,
    finished_at = now(),
    next_retry_at = NULL,
    leased_at = NULL,
    lease_owner = NULL,
    error_code = NULL,
    error_message = NULL,
    updated_at = now()
WHERE id = $1
  AND status = 'processing'
RETURNING *;

-- name: FailOCRJob :one
UPDATE ocr_jobs
SET status = $2,
    error_code = $3,
    error_message = $4,
    raw_result = COALESCE($5, raw_result),
    next_retry_at = $6,
    leased_at = NULL,
    lease_owner = NULL,
    finished_at = CASE WHEN $2 = 'failed' OR $2 = 'cancelled' THEN now() ELSE finished_at END,
    updated_at = now()
WHERE id = $1
  AND status = 'processing'
RETURNING *;

-- name: FailPendingOCRJob :one
UPDATE ocr_jobs
SET status = 'failed',
    error_code = $2,
    error_message = $3,
    next_retry_at = NULL,
    leased_at = NULL,
    lease_owner = NULL,
    finished_at = now(),
    updated_at = now()
WHERE id = $1
  AND status = 'pending'
RETURNING *;
