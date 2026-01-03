-- 商户员工管理查询

-- name: CreateMerchantStaff :one
INSERT INTO merchant_staff (merchant_id, user_id, role, invited_by)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetMerchantStaff :one
SELECT * FROM merchant_staff
WHERE merchant_id = $1 AND user_id = $2;

-- name: GetMerchantStaffByID :one
SELECT * FROM merchant_staff
WHERE id = $1;

-- name: ListMerchantStaffByMerchant :many
SELECT 
    ms.*,
    u.full_name,
    u.avatar_url
FROM merchant_staff ms
JOIN users u ON ms.user_id = u.id
WHERE ms.merchant_id = $1 AND ms.status = 'active'
ORDER BY 
    CASE ms.role 
        WHEN 'owner' THEN 1 
        WHEN 'manager' THEN 2 
        WHEN 'chef' THEN 3 
        WHEN 'cashier' THEN 4 
    END;

-- name: ListMerchantsByStaff :many
SELECT m.* FROM merchants m
JOIN merchant_staff ms ON m.id = ms.merchant_id
WHERE ms.user_id = $1 AND ms.status = 'active'
ORDER BY m.created_at;

-- name: GetUserMerchantRole :one
SELECT role FROM merchant_staff 
WHERE merchant_id = $1 AND user_id = $2 AND status = 'active';

-- name: UpdateMerchantStaffRole :one
UPDATE merchant_staff 
SET role = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateMerchantStaffStatus :one
UPDATE merchant_staff 
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteMerchantStaff :exec
DELETE FROM merchant_staff WHERE id = $1;

-- name: DeleteMerchantStaffByMerchant :exec
DELETE FROM merchant_staff WHERE merchant_id = $1;

-- name: CountMerchantStaff :one
SELECT COUNT(*) FROM merchant_staff 
WHERE merchant_id = $1 AND status = 'active';

-- name: CheckUserHasMerchantAccess :one
SELECT EXISTS(
    SELECT 1 FROM merchant_staff 
    WHERE merchant_id = $1 AND user_id = $2 AND status = 'active'
) AS has_access;
