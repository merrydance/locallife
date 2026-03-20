-- ============================================================
-- 媒体资产查询 (Media Asset Queries)
-- ============================================================

-- name: CreateMediaAsset :one
INSERT INTO media_assets (
    object_key,
    visibility,
    media_category,
    mime_type,
    file_size,
    checksum_sha256,
    upload_status,
    moderation_status,
    uploaded_by,
    source_client
) VALUES (
    $1, $2, $3, $4, $5, $6,
    'pending', 'pending',
    $7, $8
) RETURNING *;

-- name: GetMediaAssetByID :one
SELECT * FROM media_assets
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetMediaAssetByObjectKey :one
SELECT * FROM media_assets
WHERE object_key = $1 AND deleted_at IS NULL;

-- name: ConfirmMediaAssetUploaded :one
UPDATE media_assets
SET
    upload_status = 'confirmed',
    mime_type     = COALESCE($2, mime_type),
    file_size     = COALESCE($3, file_size),
    width         = COALESCE($4, width),
    height        = COALESCE($5, height),
    updated_at    = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SetMediaAssetUploadStatus :one
UPDATE media_assets
SET upload_status = $2, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SetMediaAssetModerationStatus :one
UPDATE media_assets
SET moderation_status = $2, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteMediaAsset :one
UPDATE media_assets
SET
    deleted_at    = now(),
    upload_status = 'deleted',
    updated_at    = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: ListMediaAssetsByUploader :many
SELECT * FROM media_assets
WHERE uploaded_by = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- ============================================================
-- 上传会话查询 (Upload Session Queries)
-- ============================================================

-- name: CreateUploadSession :one
INSERT INTO media_upload_sessions (
    id,
    user_id,
    business_type,
    media_category,
    visibility,
    object_key,
    checksum_sha256,
    content_type,
    content_length,
    status,
    expire_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9,
    'pending',
    $10
) RETURNING *;

-- name: GetUploadSession :one
SELECT * FROM media_upload_sessions WHERE id = $1;

-- name: GetPendingUploadSessionByIdempotencyKey :one
SELECT * FROM media_upload_sessions
WHERE user_id        = $1
  AND media_category = $2
  AND checksum_sha256 = $3
  AND status         = 'pending';

-- name: CompleteUploadSession :one
UPDATE media_upload_sessions
SET
    status         = 'completed',
    media_asset_id = $2
WHERE id = $1
RETURNING *;

-- name: ExpireUploadSession :one
UPDATE media_upload_sessions
SET status = 'expired'
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: ExpireStaleUploadSessions :many
UPDATE media_upload_sessions
SET status = 'expired'
WHERE status = 'pending'
  AND expire_at < now()
RETURNING *;

-- name: ListMediaAssetsByIDs :many
SELECT id, object_key, visibility, media_category, mime_type, file_size,
       width, height, checksum_sha256, upload_status, moderation_status,
       uploaded_by, source_client, created_at, updated_at, deleted_at
FROM media_assets
WHERE id = ANY(@ids::bigint[])
  AND deleted_at IS NULL;
