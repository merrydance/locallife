-- Group applications
-- name: CreateGroupApplicationDraft :one
INSERT INTO merchant_group_applications (
  applicant_user_id, group_name, contact_phone
) VALUES (
  $1, '', ''
) RETURNING *;

-- name: GetLatestGroupApplicationByApplicant :one
SELECT id, applicant_user_id, group_name, contact_phone, license_number, address, region_id, status, reject_reason, reviewed_by, reviewed_at, application_data, created_at, updated_at, license_media_asset_id FROM merchant_group_applications
WHERE applicant_user_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetGroupApplication :one
SELECT id, applicant_user_id, group_name, contact_phone, license_number, address, region_id, status, reject_reason, reviewed_by, reviewed_at, application_data, created_at, updated_at, license_media_asset_id FROM merchant_group_applications
WHERE id = $1;

-- name: UpdateGroupApplicationBasic :one
UPDATE merchant_group_applications
SET group_name = COALESCE($2, group_name),
    contact_phone = COALESCE($3, contact_phone),
    license_number = COALESCE($4, license_number),
    license_media_asset_id = COALESCE($5, license_media_asset_id),
    address = COALESCE($6, address),
    region_id = COALESCE($7, region_id),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateGroupApplicationLicense :one
UPDATE merchant_group_applications
SET license_media_asset_id = COALESCE($2, license_media_asset_id),
    license_number = COALESCE($3, license_number),
    application_data = COALESCE($4, application_data),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ClearGroupApplicationBusinessLicense :one
UPDATE merchant_group_applications
SET license_media_asset_id = NULL,
    license_number = NULL,
    application_data = COALESCE(application_data, '{}'::jsonb) - 'business_license_ocr',
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearGroupApplicationIDCardFront :one
UPDATE merchant_group_applications
SET application_data = COALESCE(application_data, '{}'::jsonb)
      - 'id_card_front_asset_id'
      - 'id_card_front_ocr'
      - 'legal_person_name'
      - 'legal_person_id_number',
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ClearGroupApplicationIDCardBack :one
UPDATE merchant_group_applications
SET application_data = COALESCE(application_data, '{}'::jsonb)
      - 'id_card_back_asset_id'
      - 'id_card_back_ocr',
    updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: ResetGroupApplicationToDraft :one
UPDATE merchant_group_applications
SET status = 'draft',
    reject_reason = NULL,
    reviewed_by = NULL,
    reviewed_at = NULL,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SubmitGroupApplication :one
UPDATE merchant_group_applications
SET status = 'submitted', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ReviewGroupApplication :one
UPDATE merchant_group_applications
SET status = $2,
    reject_reason = $3,
    reviewed_by = $4,
    reviewed_at = $5,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- Groups
-- name: CreateMerchantGroup :one
INSERT INTO merchant_groups (
  name, owner_user_id, contact_phone, license_number, license_media_asset_id, address, region_id, application_data
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetMerchantGroup :one
SELECT id, name, owner_user_id, status, contact_phone, license_number, address, region_id, application_data, created_at, updated_at, license_media_asset_id FROM merchant_groups
WHERE id = $1;

-- name: ListMerchantGroups :many
SELECT id, name, owner_user_id, status, contact_phone, license_number, address, region_id, application_data, created_at, updated_at, license_media_asset_id FROM merchant_groups
WHERE status = 'active'
  AND ($1::text IS NULL OR name ILIKE '%' || $1 || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: UpdateMerchantGroup :one
UPDATE merchant_groups
SET name = COALESCE($2, name),
    contact_phone = COALESCE($3, contact_phone),
    license_number = COALESCE($4, license_number),
    license_media_asset_id = COALESCE($5, license_media_asset_id),
    address = COALESCE($6, address),
    region_id = COALESCE($7, region_id),
    status = COALESCE($8, status),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateMerchantBrand :one
INSERT INTO merchant_brands (
  group_id, name, logo_media_asset_id, description
) VALUES (
  $1, $2, $3, $4
) RETURNING *;

-- name: GetMerchantBrand :one
SELECT id, group_id, name, description, status, created_at, updated_at, logo_media_asset_id FROM merchant_brands WHERE id = $1;

-- name: ListMerchantBrandsByGroup :many
SELECT id, group_id, name, description, status, created_at, updated_at, logo_media_asset_id FROM merchant_brands WHERE group_id = $1 ORDER BY created_at DESC;

-- Group members
-- name: CreateGroupMember :one
INSERT INTO merchant_group_members (
  group_id, user_id, role, invited_by
) VALUES (
  $1, $2, $3, $4
) RETURNING *;

-- name: GetGroupMemberRole :one
SELECT role FROM merchant_group_members
WHERE group_id = $1 AND user_id = $2 AND status = 'active';

-- Join requests
-- name: CreateGroupJoinRequest :one
INSERT INTO merchant_group_join_requests (
  group_id, merchant_id, applicant_user_id, reason
) VALUES (
  $1, $2, $3, $4
) RETURNING *;

-- name: GetGroupJoinRequest :one
SELECT id, group_id, merchant_id, applicant_user_id, status, reason, reviewed_by, reviewed_at, created_at FROM merchant_group_join_requests WHERE id = $1;

-- name: ListGroupJoinRequestsByGroup :many
SELECT id, group_id, merchant_id, applicant_user_id, status, reason, reviewed_by, reviewed_at, created_at FROM merchant_group_join_requests
WHERE group_id = $1
ORDER BY created_at DESC;

-- name: UpdateGroupJoinRequestStatus :one
UPDATE merchant_group_join_requests
SET status = $2,
    reviewed_by = $3,
    reviewed_at = $4
WHERE id = $1
RETURNING *;

-- Merchant affiliation
-- name: UpdateMerchantGroupAffiliation :exec
UPDATE merchants
SET group_id = $2, brand_id = $3, updated_at = now()
WHERE id = $1
;

-- name: GetMerchantGroupAffiliation :one
SELECT group_id, brand_id FROM merchants WHERE id = $1;

-- Group policies
-- name: UpsertGroupPolicies :one
INSERT INTO group_policies (group_id, pricing_mode, menu_mode, inventory_mode, promotion_mode)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (group_id)
DO UPDATE SET
  pricing_mode = EXCLUDED.pricing_mode,
  menu_mode = EXCLUDED.menu_mode,
  inventory_mode = EXCLUDED.inventory_mode,
  promotion_mode = EXCLUDED.promotion_mode
RETURNING *;

-- name: GetGroupPolicies :one
SELECT group_id, pricing_mode, menu_mode, inventory_mode, promotion_mode FROM group_policies WHERE group_id = $1;

-- Templates
-- name: CreateGroupMenuTemplate :one
INSERT INTO group_menu_templates (group_id, payload, version, status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateBrandMenuTemplate :one
INSERT INTO brand_menu_templates (brand_id, payload, version, status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- Audit logs
-- name: CreateGroupAuditLog :one
INSERT INTO merchant_group_audit_logs (
  group_id, actor_user_id, action, target_type, target_id, metadata
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- Group merchants
-- name: ListGroupMerchants :many
SELECT id, name, logo_media_asset_id, address, phone, status FROM merchants WHERE group_id = $1 ORDER BY created_at DESC;