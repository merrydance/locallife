-- ==================== 商户入驻申请（草稿模式+自动审核） ====================

-- name: CreateMerchantApplicationDraft :one
-- 创建商户申请草稿（仅需用户ID）
INSERT INTO merchant_applications (
  user_id,
  merchant_name,
  business_license_number,
  legal_person_name,
  legal_person_id_number,
  contact_phone,
  business_address,
  status
) VALUES (
  $1, '', '', '', '', '', '', 'draft'
)
RETURNING *;

-- name: GetMerchantApplicationDraft :one
-- 获取用户的草稿或可编辑申请（包含所有状态，以便随时编辑）
SELECT id, user_id, merchant_name, business_license_number, legal_person_name, legal_person_id_number, contact_phone, business_address, business_scope, status, reject_reason, reviewed_by, reviewed_at, created_at, updated_at, longitude, latitude, region_id, food_permit_ocr, business_license_ocr, id_card_front_ocr, id_card_back_ocr, storefront_images, environment_images, business_license_media_asset_id, food_permit_media_asset_id, id_card_front_media_asset_id, id_card_back_media_asset_id, review_summary FROM merchant_applications
WHERE user_id = $1 AND status IN ('draft', 'submitted', 'rejected', 'approved')
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateMerchantApplicationBasicInfo :one
-- 更新基础信息（商户名、联系电话、地址、经纬度、区域、人工修正字段）
UPDATE merchant_applications
SET
  merchant_name = COALESCE(sqlc.narg(merchant_name), merchant_name),
  contact_phone = COALESCE(sqlc.narg(contact_phone), contact_phone),
  business_address = COALESCE(sqlc.narg(business_address), business_address),
  business_license_number = COALESCE(sqlc.narg(business_license_number), business_license_number),
  business_scope = COALESCE(sqlc.narg(business_scope), business_scope),
  legal_person_name = COALESCE(sqlc.narg(legal_person_name), legal_person_name),
  legal_person_id_number = COALESCE(sqlc.narg(legal_person_id_number), legal_person_id_number),
  longitude = COALESCE(sqlc.narg(longitude), longitude),
  latitude = COALESCE(sqlc.narg(latitude), latitude),
  region_id = COALESCE(sqlc.narg(region_id), region_id),
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateMerchantApplicationBusinessLicense :one
-- 更新营业执照信息（图片URL和OCR结果）
UPDATE merchant_applications
SET
  business_license_media_asset_id = COALESCE(sqlc.narg(business_license_media_asset_id), business_license_media_asset_id),
  business_license_number = COALESCE(sqlc.narg(business_license_number), business_license_number),
  business_scope = COALESCE(sqlc.narg(business_scope), business_scope),
  merchant_name = CASE
    WHEN COALESCE(NULLIF(btrim(merchant_name), ''), '') = '' THEN COALESCE(sqlc.narg(merchant_name), merchant_name)
    ELSE merchant_name
  END,
  business_license_ocr = COALESCE(sqlc.narg(business_license_ocr), business_license_ocr),
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearMerchantApplicationBusinessLicense :one
-- 清空营业执照关联和 OCR 结果
UPDATE merchant_applications
SET
  business_license_media_asset_id = NULL,
  business_license_ocr = NULL,
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateMerchantApplicationFoodPermit :one
-- 更新食品经营许可证信息（图片URL和OCR结果）
UPDATE merchant_applications
SET
  food_permit_media_asset_id = COALESCE(sqlc.narg(food_permit_media_asset_id), food_permit_media_asset_id),
  food_permit_ocr = COALESCE(sqlc.narg(food_permit_ocr), food_permit_ocr),
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearMerchantApplicationFoodPermit :one
-- 清空食品经营许可证关联和 OCR 结果
UPDATE merchant_applications
SET
  food_permit_media_asset_id = NULL,
  food_permit_ocr = NULL,
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateMerchantApplicationIDCardFront :one
-- 更新身份证正面信息（图片URL和OCR结果）
UPDATE merchant_applications
SET
  id_card_front_media_asset_id = COALESCE(sqlc.narg(id_card_front_media_asset_id), id_card_front_media_asset_id),
  legal_person_name = COALESCE(sqlc.narg(legal_person_name), legal_person_name),
  legal_person_id_number = COALESCE(sqlc.narg(legal_person_id_number), legal_person_id_number),
  id_card_front_ocr = COALESCE(sqlc.narg(id_card_front_ocr), id_card_front_ocr),
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearMerchantApplicationIDCardFront :one
-- 清空身份证正面关联和 OCR 结果
UPDATE merchant_applications
SET
  id_card_front_media_asset_id = NULL,
  id_card_front_ocr = NULL,
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateMerchantApplicationIDCardBack :one
-- 更新身份证背面信息（图片URL和OCR结果）
UPDATE merchant_applications
SET
  id_card_back_media_asset_id = COALESCE(sqlc.narg(id_card_back_media_asset_id), id_card_back_media_asset_id),
  id_card_back_ocr = COALESCE(sqlc.narg(id_card_back_ocr), id_card_back_ocr),
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearMerchantApplicationIDCardBack :one
-- 清空身份证背面关联和 OCR 结果
UPDATE merchant_applications
SET
  id_card_back_media_asset_id = NULL,
  id_card_back_ocr = NULL,
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: SubmitMerchantApplication :one
-- 提交商户申请（从草稿、被拒绝或已通过状态变为已提交）
UPDATE merchant_applications
SET
  status = 'submitted',
  reject_reason = NULL,
  reviewed_at = NULL,
  reviewed_by = NULL,
  updated_at = now()
WHERE id = $1 AND status IN ('draft', 'rejected', 'approved')
RETURNING *;

-- name: ApproveMerchantApplication :one
-- 审核通过商户申请
UPDATE merchant_applications
SET
  status = 'approved',
  reviewed_at = now(),
  updated_at = now()
WHERE id = $1 AND status = 'submitted'
RETURNING *;

-- name: RejectMerchantApplication :one
-- 拒绝商户申请
UPDATE merchant_applications
SET
  status = 'rejected',
  reject_reason = $2,
  reviewed_at = now(),
  updated_at = now()
WHERE id = $1 AND status = 'submitted'
RETURNING *;

-- name: ResetMerchantApplicationToDraft :one
-- 重置申请为草稿状态（允许用户重新编辑，支持从待审核、被拒绝或已通过状态重置）
UPDATE merchant_applications
SET
  status = 'draft',
  reject_reason = NULL,
  reviewed_at = NULL,
  reviewed_by = NULL,
  updated_at = now()
WHERE id = $1 AND status IN ('submitted', 'rejected', 'approved')
RETURNING *;

-- name: CountMerchantApplicationsByStatus :one
-- 统计各状态的申请数量
SELECT COUNT(*) FROM merchant_applications
WHERE status = $1;

-- name: CheckMerchantAddressExists :one
-- 检查地址是否已被其他商户占用（排除指定用户自己的商户）
SELECT EXISTS(
  SELECT 1 FROM merchants 
  WHERE address = $1 AND owner_user_id != $2 AND deleted_at IS NULL
) AS exists;

-- name: ListMerchantAddressesByRegion :many
SELECT address FROM merchants
WHERE region_id = $1 AND deleted_at IS NULL;

-- name: ListMerchantLocationsInRegion :many
-- 获取区域内所有在营商户的坐标和地址，用于 GPS 距离去重检测
SELECT owner_user_id, address, latitude, longitude FROM merchants
WHERE region_id = $1 AND deleted_at IS NULL;

-- name: UpdateMerchantApplicationImages :one
-- 更新门头照和环境照（jsonb数组）
UPDATE merchant_applications
SET
  storefront_images = COALESCE(sqlc.narg(storefront_images), storefront_images),
  environment_images = COALESCE(sqlc.narg(environment_images), environment_images),
  updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: UpdateMerchantApplicationShopImages :one
-- 更新门头照和环境照（商户已审核通过后也可以更新）
UPDATE merchant_applications
SET
  storefront_images = COALESCE(sqlc.narg(storefront_images), storefront_images),
  environment_images = COALESCE(sqlc.narg(environment_images), environment_images),
  updated_at = now()
WHERE user_id = $1
RETURNING *;

-- name: ResetStaleMerchantOCRStatus :exec
UPDATE merchant_applications
SET 
  business_license_ocr = CASE WHEN business_license_ocr->>'status' = 'processing' THEN jsonb_set(business_license_ocr, '{status}', '"failed"') ELSE business_license_ocr END,
  food_permit_ocr = CASE WHEN food_permit_ocr->>'status' = 'processing' THEN jsonb_set(food_permit_ocr, '{status}', '"failed"') ELSE food_permit_ocr END,
  id_card_front_ocr = CASE WHEN id_card_front_ocr->>'status' = 'processing' THEN jsonb_set(id_card_front_ocr, '{status}', '"failed"') ELSE id_card_front_ocr END,
  id_card_back_ocr = CASE WHEN id_card_back_ocr->>'status' = 'processing' THEN jsonb_set(id_card_back_ocr, '{status}', '"failed"') ELSE id_card_back_ocr END
WHERE 
  (business_license_ocr->>'status' = 'processing' OR
   food_permit_ocr->>'status' = 'processing' OR
   id_card_front_ocr->>'status' = 'processing' OR
   id_card_back_ocr->>'status' = 'processing')
  AND updated_at < $1;

-- name: UpdateMerchantApplicationReviewSummary :one
UPDATE merchant_applications
SET review_summary = sqlc.narg(review_summary),
    updated_at = now()
WHERE id = $1
RETURNING *;
