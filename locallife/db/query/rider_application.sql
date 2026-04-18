-- name: CreateRiderApplication :one
-- 创建骑手申请草稿
INSERT INTO rider_applications (
    user_id,
    status
) VALUES (
    $1,
    'draft'
) RETURNING *;

-- name: GetRiderApplication :one
-- 获取骑手申请
SELECT id, user_id, real_name, phone, id_card_ocr, health_cert_ocr, status, reject_reason, reviewed_by, reviewed_at, created_at, updated_at, submitted_at, id_card_front_media_asset_id, id_card_back_media_asset_id, health_cert_media_asset_id FROM rider_applications
WHERE id = $1;

-- name: GetRiderApplicationByUserID :one
-- 根据用户ID获取骑手申请
SELECT id, user_id, real_name, phone, id_card_ocr, health_cert_ocr, status, reject_reason, reviewed_by, reviewed_at, created_at, updated_at, submitted_at, id_card_front_media_asset_id, id_card_back_media_asset_id, health_cert_media_asset_id FROM rider_applications
WHERE user_id = $1;

-- name: UpdateRiderApplicationBasicInfo :one
-- 更新基础信息（姓名、手机号）
UPDATE rider_applications
SET 
    real_name = COALESCE(sqlc.narg(real_name), real_name),
    phone = COALESCE(sqlc.narg(phone), phone),
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateRiderApplicationIDCard :one
-- 更新身份证信息
UPDATE rider_applications
SET 
    id_card_front_media_asset_id = COALESCE(sqlc.narg(id_card_front_media_asset_id), id_card_front_media_asset_id),
    id_card_back_media_asset_id = COALESCE(sqlc.narg(id_card_back_media_asset_id), id_card_back_media_asset_id),
    id_card_ocr = CASE
        WHEN sqlc.narg(id_card_ocr)::jsonb IS NULL THEN id_card_ocr
        WHEN id_card_ocr IS NULL THEN sqlc.narg(id_card_ocr)::jsonb
        ELSE id_card_ocr || sqlc.narg(id_card_ocr)::jsonb
    END,
    -- OCR识别出姓名时自动更新real_name
    real_name = COALESCE(sqlc.narg(real_name), real_name),
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateRiderApplicationHealthCert :one
-- 更新健康证信息
UPDATE rider_applications
SET 
    health_cert_media_asset_id = COALESCE(sqlc.narg(health_cert_media_asset_id), health_cert_media_asset_id),
    health_cert_ocr = COALESCE(sqlc.narg(health_cert_ocr), health_cert_ocr),
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearRiderApplicationIDCardFront :one
-- 清空身份证正面媒体与对应 OCR 字段，保留背面有效期信息
UPDATE rider_applications
SET
    id_card_front_media_asset_id = NULL,
    id_card_ocr = CASE
        WHEN id_card_ocr IS NULL THEN NULL
        ELSE NULLIF(
            id_card_ocr
                - 'status'
                - 'error'
                - 'error_code'
                - 'alert_emitted_at'
                - 'queued_at'
                - 'started_at'
                - 'ocr_job_id'
                - 'ocr_at'
                - 'name'
                - 'id_number'
                - 'gender'
                - 'nation'
                - 'address',
            '{}'::jsonb
        )
    END,
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearRiderApplicationIDCardBack :one
-- 清空身份证背面媒体与对应 OCR 字段，保留正面实名信息
UPDATE rider_applications
SET
    id_card_back_media_asset_id = NULL,
    id_card_ocr = CASE
        WHEN id_card_ocr IS NULL THEN NULL
        ELSE NULLIF(
            id_card_ocr
                - 'status'
                - 'error'
                - 'error_code'
                - 'alert_emitted_at'
                - 'queued_at'
                - 'started_at'
                - 'ocr_job_id'
                - 'ocr_at'
                - 'valid_start'
                - 'valid_end',
            '{}'::jsonb
        )
    END,
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearRiderApplicationHealthCert :one
-- 清空健康证媒体与 OCR 结果
UPDATE rider_applications
SET
    health_cert_media_asset_id = NULL,
    health_cert_ocr = NULL,
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: SubmitRiderApplication :one
-- 提交骑手申请
UPDATE rider_applications
SET 
    status = 'submitted',
    submitted_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ApproveRiderApplication :one
-- 审核通过骑手申请
UPDATE rider_applications
SET 
    status = 'approved',
    reviewed_by = sqlc.narg(reviewed_by),
    reviewed_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'submitted'
RETURNING *;

-- name: ReturnRiderApplicationToDraft :one
-- 审核未通过后退回草稿，保留失败原因
UPDATE rider_applications
SET 
    status = 'draft',
    reject_reason = $2,
    reviewed_by = sqlc.narg(reviewed_by),
    reviewed_at = now(),
    submitted_at = NULL,
    updated_at = now()
WHERE id = $1 AND status = 'submitted'
RETURNING *;

-- name: ResetRiderApplicationToDraft :one
-- 手动重置申请为草稿状态，并清空审核痕迹
UPDATE rider_applications
SET 
    status = 'draft',
    reject_reason = NULL,
    reviewed_by = NULL,
    reviewed_at = NULL,
    submitted_at = NULL,
    updated_at = now()
WHERE id = $1 AND status = 'submitted'
RETURNING *;

-- name: ListRiderApplications :many
-- 列出骑手申请（管理员用）
SELECT id, user_id, real_name, phone, id_card_ocr, health_cert_ocr, status, reject_reason, reviewed_by, reviewed_at, created_at, updated_at, submitted_at, id_card_front_media_asset_id, id_card_back_media_asset_id, health_cert_media_asset_id FROM rider_applications
WHERE 
    (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
ORDER BY 
    CASE WHEN status = 'submitted' THEN 0 ELSE 1 END,
    submitted_at DESC NULLS LAST,
    created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountRiderApplicationsByStatus :one
-- 统计各状态的申请数量
SELECT COUNT(*) FROM rider_applications
WHERE status = $1;
