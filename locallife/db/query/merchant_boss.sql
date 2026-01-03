-- Boss 店铺认领查询

-- name: CreateMerchantBoss :one
INSERT INTO merchant_bosses (user_id, merchant_id, status)
VALUES ($1, $2, 'active')
RETURNING *;

-- name: GetMerchantBoss :one
SELECT * FROM merchant_bosses
WHERE user_id = $1 AND merchant_id = $2 AND status = 'active';

-- name: ListMerchantsByBoss :many
-- 获取 Boss 关联的所有店铺
SELECT m.* FROM merchants m
JOIN merchant_bosses mb ON m.id = mb.merchant_id
WHERE mb.user_id = $1 AND mb.status = 'active' AND m.deleted_at IS NULL
ORDER BY m.created_at;

-- name: ListBossesByMerchant :many
-- 获取店铺的所有 Boss
SELECT 
    mb.*,
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
