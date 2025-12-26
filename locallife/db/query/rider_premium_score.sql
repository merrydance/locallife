-- =============================================
-- 高值单资格积分查询
-- =============================================

-- name: GetRiderPremiumScore :one
-- 获取骑手高值单资格积分
SELECT premium_score FROM rider_profiles
WHERE rider_id = $1
LIMIT 1;

-- name: UpdateRiderPremiumScore :one
-- 更新骑手高值单资格积分（原子操作）
UPDATE rider_profiles
SET premium_score = premium_score + $2,
    updated_at = NOW()
WHERE rider_id = $1
RETURNING premium_score;

-- name: CreateRiderPremiumScoreLog :one
-- 创建高值单资格积分变更日志
INSERT INTO rider_premium_score_logs (
    rider_id,
    change_amount,
    old_score,
    new_score,
    change_type,
    related_order_id,
    related_delivery_id,
    remark
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: ListRiderPremiumScoreLogs :many
-- 查询骑手高值单资格积分变更历史
SELECT * FROM rider_premium_score_logs
WHERE rider_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountRiderPremiumScoreLogs :one
-- 统计骑手高值单资格积分变更历史数量
SELECT COUNT(*) FROM rider_premium_score_logs
WHERE rider_id = $1;

-- name: GetRiderPremiumScoreWithProfile :one
-- 获取骑手高值单资格积分及基本信息（用于API返回）
SELECT 
    r.id AS rider_id,
    r.real_name,
    rp.premium_score,
    CASE WHEN rp.premium_score >= 0 THEN TRUE ELSE FALSE END AS can_accept_premium_order
FROM riders r
JOIN rider_profiles rp ON r.id = rp.rider_id
WHERE r.id = $1
LIMIT 1;
