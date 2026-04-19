-- Boss 店铺认领查询

-- name: CreateMerchantBoss :one
INSERT INTO merchant_bosses (user_id, merchant_id, status)
VALUES ($1, $2, 'active')
RETURNING *;

-- name: GetMerchantBoss :one
SELECT id, user_id, merchant_id, status, created_at, updated_at FROM merchant_bosses
WHERE user_id = $1 AND merchant_id = $2 AND status = 'active';

-- name: ListMerchantsByBoss :many
-- 获取 Boss 关联的所有店铺
SELECT m.id, m.owner_user_id, m.name, m.description, m.phone, m.address, m.latitude, m.longitude, m.status, m.application_data, m.created_at, m.updated_at, m.version, m.region_id, m.is_open, m.auto_close_at, m.deleted_at, m.pending_owner_bind, m.bind_code, m.bind_code_expires_at, m.group_id, m.brand_id, m.logo_media_asset_id, m.auto_open_by_business_hours FROM merchants m
JOIN merchant_bosses mb ON m.id = mb.merchant_id
WHERE mb.user_id = $1 AND mb.status = 'active' AND m.deleted_at IS NULL
ORDER BY m.created_at;

-- name: ListBossesByMerchant :many
-- 获取店铺的所有 Boss
SELECT 
    mb.id, mb.user_id, mb.merchant_id, mb.status, mb.created_at, mb.updated_at,
    u.full_name,
    u.avatar_url,
    u.phone
FROM merchant_bosses mb
JOIN users u ON mb.user_id = u.id
WHERE mb.merchant_id = $1
ORDER BY mb.created_at;

-- name: UpdateMerchantBossStatus :one
UPDATE merchant_bosses
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteMerchantBoss :exec
DELETE FROM merchant_bosses WHERE id = $1;

-- name: CheckUserIsBoss :one
-- 检查用户是否是某商户的 Boss
SELECT EXISTS(
    SELECT 1 FROM merchant_bosses
    WHERE user_id = $1 AND merchant_id = $2 AND status = 'active'
);

-- name: CountMerchantBosses :one
SELECT COUNT(*) FROM merchant_bosses
WHERE merchant_id = $1 AND status = 'active';
