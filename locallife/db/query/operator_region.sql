-- name: AddOperatorRegion :one
-- 为运营商添加管理区域
INSERT INTO operator_regions (
    operator_id,
    region_id,
    status
) VALUES (
    $1, $2, 'active'
) RETURNING *;

-- name: GetOperatorRegion :one
SELECT * FROM operator_regions
WHERE operator_id = $1 AND region_id = $2 LIMIT 1;

-- name: ListOperatorRegions :many
-- 列出运营商管理的所有区域
SELECT or_t.*, r.name as region_name, r.code as region_code, r.level as region_level
FROM operator_regions or_t
JOIN regions r ON or_t.region_id = r.id
WHERE or_t.operator_id = $1 AND or_t.status = 'active'
ORDER BY r.code;

-- name: ListRegionOperators :many
-- 列出管理某区域的所有运营商
SELECT or_t.*, o.name as operator_name, o.contact_name, o.contact_phone
FROM operator_regions or_t
JOIN operators o ON or_t.operator_id = o.id
WHERE or_t.region_id = $1 AND or_t.status = 'active' AND o.status = 'active';

-- name: RemoveOperatorRegion :exec
-- 移除运营商的管理区域
DELETE FROM operator_regions
WHERE operator_id = $1 AND region_id = $2;

-- name: UpdateOperatorRegionStatus :one
-- 更新运营商区域状态（暂停/恢复）
UPDATE operator_regions
SET status = $3
WHERE operator_id = $1 AND region_id = $2
RETURNING *;

-- name: CountOperatorRegions :one
-- 统计运营商管理的区域数量
SELECT COUNT(*) FROM operator_regions
WHERE operator_id = $1 AND status = 'active';

-- name: CheckOperatorManagesRegion :one
-- 检查运营商是否管理某区域
SELECT EXISTS(
    SELECT 1 FROM operator_regions
    WHERE operator_id = $1 AND region_id = $2 AND status = 'active'
) as manages;

-- name: GetActiveOperatorByRegion :one
-- 根据区域获取运营商（通过operator_regions表，支持多区域）
SELECT o.* FROM operators o
JOIN operator_regions or_t ON o.id = or_t.operator_id
WHERE or_t.region_id = $1 
    AND or_t.status = 'active' 
    AND o.status = 'active'
LIMIT 1;

-- name: ListAllOperatorRegions :many
-- 列出所有运营商区域关系（管理后台用）
SELECT or_t.*, 
    o.name as operator_name, 
    r.name as region_name, 
    r.code as region_code
FROM operator_regions or_t
JOIN operators o ON or_t.operator_id = o.id
JOIN regions r ON or_t.region_id = r.id
ORDER BY o.id, r.code
LIMIT $1 OFFSET $2;
