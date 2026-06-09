-- 商户员工管理查询

-- name: CreateMerchantStaff :one
INSERT INTO merchant_staff (merchant_id, user_id, role, status, invited_by)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetMerchantStaff :one
SELECT id, merchant_id, user_id, role, status, invited_by, created_at, updated_at FROM merchant_staff
WHERE merchant_id = $1 AND user_id = $2;

-- name: GetMerchantStaffByID :one
SELECT id, merchant_id, user_id, role, status, invited_by, created_at, updated_at FROM merchant_staff
WHERE id = $1;

-- name: ListMerchantStaffByMerchant :many
-- 显示所有员工，包括离职员工（软删除），按状态和角色排序
SELECT 
    ms.id, ms.merchant_id, ms.user_id, ms.role, ms.status, ms.invited_by, ms.created_at, ms.updated_at,
    u.full_name,
    u.avatar_url,
    u.avatar_media_asset_id
FROM merchant_staff ms
JOIN users u ON ms.user_id = u.id
WHERE ms.merchant_id = $1
ORDER BY 
    CASE ms.status 
        WHEN 'active' THEN 1 
        WHEN 'disabled' THEN 2 
    END,
    CASE ms.role 
        WHEN 'owner' THEN 1 
        WHEN 'manager' THEN 2 
        WHEN 'chef' THEN 3 
        WHEN 'cashier' THEN 4 
        WHEN 'pending' THEN 5
    END;

-- name: ListMerchantsByStaff :many
SELECT m.id, m.owner_user_id, m.name, m.description, m.phone, m.address, m.latitude, m.longitude, m.status, m.application_data, m.created_at, m.updated_at, m.version, m.region_id, m.is_open, m.auto_close_at, m.deleted_at, m.pending_owner_bind, m.bind_code, m.bind_code_expires_at, m.group_id, m.brand_id, m.logo_media_asset_id, m.auto_open_by_business_hours, m.storefront_images, m.environment_images FROM merchants m
JOIN merchant_staff ms ON m.id = ms.merchant_id
WHERE ms.user_id = $1 AND ms.status = 'active'
ORDER BY m.created_at;

-- name: GetUserMerchantRole :one
SELECT role FROM merchant_staff 
WHERE merchant_id = $1 AND user_id = $2 AND status = 'active';

-- name: GetMerchantStaffForUpdate :one
SELECT id, merchant_id, user_id, role, status, invited_by, created_at, updated_at FROM merchant_staff
WHERE id = $1
FOR UPDATE;

-- name: CountAssignedActiveMerchantStaffByUser :one
SELECT COUNT(*) FROM merchant_staff
WHERE user_id = $1 AND status = 'active' AND role <> 'pending';

-- name: UpdateMerchantStaffRole :one
-- 更新角色时同时激活员工（从 pending 变为 active）
UPDATE merchant_staff 
SET role = $2, status = 'active', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateMerchantStaffStatus :one
UPDATE merchant_staff 
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SoftDeleteMerchantStaff :one
-- 软删除员工（设置 status='disabled'），保留历史记录
UPDATE merchant_staff 
SET status = 'disabled', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteMerchantStaff :exec
-- 硬删除（仅用于特殊情况）
DELETE FROM merchant_staff WHERE id = $1;

-- name: DeleteMerchantStaffByMerchant :exec
DELETE FROM merchant_staff WHERE merchant_id = $1;

-- name: CountMerchantStaff :one
SELECT COUNT(*) FROM merchant_staff 
WHERE merchant_id = $1 AND status = 'active' AND role <> 'pending';

-- name: CheckUserHasMerchantAccess :one
SELECT EXISTS(
    SELECT 1 FROM merchant_staff 
    WHERE merchant_id = $1 AND user_id = $2 AND status = 'active' AND role <> 'pending'
) AS has_access;
