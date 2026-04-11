-- ==================== 运营商入驻申请（草稿模式+人工审核） ====================

-- name: CreateOperatorApplicationDraft :one
-- 创建运营商申请草稿（需要用户ID和区域ID）
INSERT INTO operator_applications (
  user_id,
  region_id,
  status
) VALUES (
  $1, $2, 'draft'
)
RETURNING *;

-- name: GetOperatorApplicationDraft :one
-- 获取用户的草稿或可编辑申请（排除已通过的）
SELECT * FROM operator_applications
WHERE user_id = $1 AND status IN ('draft', 'rejected')
ORDER BY created_at DESC
LIMIT 1;

-- name: GetOperatorApplicationByID :one
-- 通过ID获取申请
SELECT * FROM operator_applications
WHERE id = $1;

-- name: GetOperatorApplicationByUserID :one
-- 获取用户的任意状态申请
SELECT * FROM operator_applications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetPendingOperatorApplicationByRegion :one
-- 检查区域是否有待审核或已通过的申请（用于区域独占检查）
SELECT * FROM operator_applications
WHERE region_id = $1 AND status IN ('submitted', 'approved')
LIMIT 1;

-- name: UpdateOperatorApplicationRegion :one
-- 更新申请的区域（仅草稿状态可修改）
UPDATE operator_applications
SET
  region_id = $2,
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateOperatorApplicationBasicInfo :one
-- 更新基础信息（名称、联系人、联系电话、合同年限）
UPDATE operator_applications
SET
  name = COALESCE(sqlc.narg(name), name),
  contact_name = COALESCE(sqlc.narg(contact_name), contact_name),
  contact_phone = COALESCE(sqlc.narg(contact_phone), contact_phone),
  requested_contract_years = COALESCE(sqlc.narg(requested_contract_years), requested_contract_years),
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateOperatorApplicationBusinessLicense :one
-- 更新营业执照信息（图片URL、执照号和OCR结果）
UPDATE operator_applications
SET
  business_license_media_asset_id = COALESCE(sqlc.narg(business_license_media_asset_id), business_license_media_asset_id),
  business_license_number = COALESCE(sqlc.narg(business_license_number), business_license_number),
  business_license_ocr = COALESCE(sqlc.narg(business_license_ocr), business_license_ocr),
  name = COALESCE(sqlc.narg(name), name),
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateOperatorApplicationIDCardFront :one
-- 更新身份证正面信息（图片URL、姓名、身份证号和OCR结果）
UPDATE operator_applications
SET
  id_card_front_media_asset_id = COALESCE(sqlc.narg(id_card_front_media_asset_id), id_card_front_media_asset_id),
  legal_person_name = COALESCE(sqlc.narg(legal_person_name), legal_person_name),
  legal_person_id_number = COALESCE(sqlc.narg(legal_person_id_number), legal_person_id_number),
  id_card_front_ocr = COALESCE(sqlc.narg(id_card_front_ocr), id_card_front_ocr),
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateOperatorApplicationIDCardBack :one
-- 更新身份证背面信息（图片URL和OCR结果）
UPDATE operator_applications
SET
  id_card_back_media_asset_id = COALESCE(sqlc.narg(id_card_back_media_asset_id), id_card_back_media_asset_id),
  id_card_back_ocr = COALESCE(sqlc.narg(id_card_back_ocr), id_card_back_ocr),
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearOperatorApplicationBusinessLicense :one
-- 清空营业执照媒体与 OCR 结果
UPDATE operator_applications
SET
  business_license_media_asset_id = NULL,
  business_license_number = NULL,
  business_license_ocr = NULL,
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearOperatorApplicationIDCardFront :one
-- 清空身份证正面媒体与对应 OCR 字段
UPDATE operator_applications
SET
  id_card_front_media_asset_id = NULL,
  legal_person_name = NULL,
  legal_person_id_number = NULL,
  id_card_front_ocr = NULL,
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearOperatorApplicationIDCardBack :one
-- 清空身份证背面媒体与 OCR 结果
UPDATE operator_applications
SET
  id_card_back_media_asset_id = NULL,
  id_card_back_ocr = NULL,
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: SubmitOperatorApplication :one
-- 提交运营商申请（从草稿变为已提交待审核）
UPDATE operator_applications
SET
  status = 'submitted',
  submitted_at = now(),
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ApproveOperatorApplication :one
-- 审核通过运营商申请（平台管理员操作）
UPDATE operator_applications
SET
  status = 'approved',
  reviewed_by = $2,
  reviewed_at = now(),
  updated_at = now()
WHERE id = $1 AND status = 'submitted'
RETURNING *;

-- name: RejectOperatorApplication :one
-- 拒绝运营商申请（平台管理员操作）
UPDATE operator_applications
SET
  status = 'rejected',
  reject_reason = $2,
  reviewed_by = $3,
  reviewed_at = now(),
  updated_at = now()
WHERE id = $1 AND status = 'submitted'
RETURNING *;

-- name: ResetOperatorApplicationToDraft :one
-- 重置被拒绝的申请为草稿（允许重新编辑提交）
UPDATE operator_applications
SET
  status = 'draft',
  reject_reason = NULL,
  updated_at = now()
WHERE id = $1 AND status = 'rejected'
RETURNING *;

-- name: ListPendingOperatorApplications :many
-- 列出申请（平台管理员用，包含 submitted/approved/rejected）
SELECT 
  oa.*,
  u.full_name as applicant_name,
  u.phone as applicant_phone,
  r.name as region_name,
  r.code as region_code
FROM operator_applications oa
LEFT JOIN users u ON u.id = oa.user_id
JOIN regions r ON r.id = oa.region_id
WHERE oa.status IN ('submitted', 'approved', 'rejected')
ORDER BY COALESCE(oa.submitted_at, oa.updated_at, oa.created_at) DESC
LIMIT $1 OFFSET $2;

-- name: CountPendingOperatorApplications :one
-- 统计申请数量（包含 submitted/approved/rejected）
SELECT COUNT(*) FROM operator_applications
WHERE status IN ('submitted', 'approved', 'rejected');

-- name: ListOperatorApplications :many
-- 列出所有申请（支持状态筛选）
SELECT 
  oa.*,
  r.name as region_name,
  r.code as region_code
FROM operator_applications oa
JOIN regions r ON r.id = oa.region_id
WHERE (sqlc.narg(status)::text IS NULL OR oa.status = sqlc.narg(status))
ORDER BY oa.created_at DESC
LIMIT $1 OFFSET $2;
