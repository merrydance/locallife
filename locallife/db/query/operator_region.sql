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
SELECT id, operator_id, region_id, status, created_at FROM operator_regions
WHERE operator_id = $1 AND region_id = $2 LIMIT 1;

-- name: ListOperatorRegions :many
-- 列出运营商管理的所有区域
SELECT or_t.id, or_t.operator_id, or_t.region_id, or_t.status, or_t.created_at, r.name as region_name, r.code as region_code, r.level as region_level
FROM operator_regions or_t
JOIN regions r ON or_t.region_id = r.id
WHERE or_t.operator_id = $1 AND or_t.status = 'active'
ORDER BY r.code;

-- name: ListOperatorRegionRelations :many
-- 列出运营商区域关系用于展示，保留暂停关系状态；不可用于权限判断
SELECT or_t.id, or_t.operator_id, or_t.region_id, or_t.status, or_t.created_at, r.name as region_name, r.code as region_code, r.level as region_level
FROM operator_regions or_t
JOIN regions r ON or_t.region_id = r.id
WHERE or_t.operator_id = $1 AND or_t.status IN ('active', 'suspended')
ORDER BY r.code;

-- name: ListRegionOperators :many
-- 列出管理某区域的所有运营商
SELECT or_t.id, or_t.operator_id, or_t.region_id, or_t.status, or_t.created_at, o.name as operator_name, o.contact_name, o.contact_phone
FROM operator_regions or_t
JOIN operators o ON or_t.operator_id = o.id
WHERE or_t.region_id = $1 AND or_t.status = 'active' AND o.status = 'active';

-- name: ListActiveOperatorNotificationRecipientsByRegion :many
-- 列出区域内可接收提醒的运营商用户
SELECT
        o.id AS operator_id,
        o.user_id,
        or_t.region_id,
        r.name AS region_name,
        o.name AS operator_name,
        o.contact_name,
        o.contact_phone
FROM operator_regions or_t
JOIN operators o ON or_t.operator_id = o.id
JOIN regions r ON r.id = or_t.region_id
WHERE or_t.region_id = $1
    AND or_t.status = 'active'
    AND o.status = 'active'
ORDER BY o.id ASC;

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
SELECT o.id, o.user_id, o.region_id, o.name, o.contact_name, o.contact_phone, o.wechat_mch_id, o.status, o.created_at, o.updated_at, o.contract_start_date, o.contract_end_date, o.contract_years, o.sub_mch_id, o.balance, o.wallet_account, o.rider_deposit, o.weather_coeff_extreme, o.weather_coeff_heavy, o.weather_coeff_moderate, o.weather_coeff_light, o.latest_settlement_application_no, o.latest_settlement_application_submitted_at FROM operators o
JOIN operator_regions or_t ON o.id = or_t.operator_id
WHERE or_t.region_id = $1 
    AND or_t.status = 'active' 
    AND o.status = 'active'
LIMIT 1;

-- name: ListAllOperatorRegions :many
-- 列出所有运营商区域关系（管理后台用）
SELECT or_t.id, or_t.operator_id, or_t.region_id, or_t.status, or_t.created_at,
    o.name as operator_name, 
    r.name as region_name, 
    r.code as region_code
FROM operator_regions or_t
JOIN operators o ON or_t.operator_id = o.id
JOIN regions r ON or_t.region_id = r.id
ORDER BY o.id, r.code
LIMIT $1 OFFSET $2;
