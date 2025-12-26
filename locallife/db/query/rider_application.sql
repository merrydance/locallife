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
SELECT * FROM rider_applications
WHERE id = $1;

-- name: GetRiderApplicationByUserID :one
-- 根据用户ID获取骑手申请
SELECT * FROM rider_applications
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
    id_card_front_url = COALESCE(sqlc.narg(id_card_front_url), id_card_front_url),
    id_card_back_url = COALESCE(sqlc.narg(id_card_back_url), id_card_back_url),
    id_card_ocr = COALESCE(sqlc.narg(id_card_ocr), id_card_ocr),
    -- OCR识别出姓名时自动更新real_name
    real_name = COALESCE(sqlc.narg(real_name), real_name),
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateRiderApplicationHealthCert :one
-- 更新健康证信息
UPDATE rider_applications
SET 
    health_cert_url = COALESCE(sqlc.narg(health_cert_url), health_cert_url),
    health_cert_ocr = COALESCE(sqlc.narg(health_cert_ocr), health_cert_ocr),
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

-- name: RejectRiderApplication :one
-- 拒绝骑手申请
UPDATE rider_applications
SET 
    status = 'rejected',
    reject_reason = $2,
    reviewed_by = sqlc.narg(reviewed_by),
    reviewed_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'submitted'
RETURNING *;

-- name: ResetRiderApplicationToDraft :one
-- 重置申请为草稿状态（被拒绝后可重新编辑）
UPDATE rider_applications
SET 
    status = 'draft',
    reject_reason = NULL,
    reviewed_by = NULL,
    reviewed_at = NULL,
    submitted_at = NULL,
    updated_at = now()
WHERE id = $1 AND status = 'rejected'
RETURNING *;

-- name: ListRiderApplications :many
-- 列出骑手申请（管理员用）
SELECT * FROM rider_applications
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
