-- M9: TrustScore信任分系统查询
-- 设计理念：信用驱动，非证据驱动

-- ==========================================
-- user_profiles（顾客信任画像）
-- ==========================================

-- name: CreateUserProfile :one
INSERT INTO user_profiles (
    user_id,
    role,
    trust_score,
    total_orders,
    completed_orders,
    cancelled_orders,
    total_claims,
    malicious_claims,
    food_safety_reports,
    verified_violations,
    recent_7d_claims,
    recent_7d_orders,
    recent_30d_claims,
    recent_30d_orders,
    recent_30d_cancels,
    recent_90d_claims,
    recent_90d_orders,
    is_blacklisted,
    blacklist_reason,
    blacklisted_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
) RETURNING *;

-- name: GetUserProfile :one
SELECT * FROM user_profiles
WHERE user_id = $1 AND role = $2
LIMIT 1;

-- name: GetUserProfileForUpdate :one
SELECT * FROM user_profiles
WHERE user_id = $1 AND role = $2
LIMIT 1
FOR UPDATE;

-- name: UpdateUserTrustScore :exec
UPDATE user_profiles
SET trust_score = $3,
    updated_at = NOW()
WHERE user_id = $1 AND role = $2;

-- name: UpdateUserProfile :exec
UPDATE user_profiles
SET trust_score = COALESCE(sqlc.narg('trust_score'), trust_score),
    total_orders = COALESCE(sqlc.narg('total_orders'), total_orders),
    completed_orders = COALESCE(sqlc.narg('completed_orders'), completed_orders),
    cancelled_orders = COALESCE(sqlc.narg('cancelled_orders'), cancelled_orders),
    total_claims = COALESCE(sqlc.narg('total_claims'), total_claims),
    malicious_claims = COALESCE(sqlc.narg('malicious_claims'), malicious_claims),
    food_safety_reports = COALESCE(sqlc.narg('food_safety_reports'), food_safety_reports),
    verified_violations = COALESCE(sqlc.narg('verified_violations'), verified_violations),
    recent_7d_claims = COALESCE(sqlc.narg('recent_7d_claims'), recent_7d_claims),
    recent_7d_orders = COALESCE(sqlc.narg('recent_7d_orders'), recent_7d_orders),
    recent_30d_claims = COALESCE(sqlc.narg('recent_30d_claims'), recent_30d_claims),
    recent_30d_orders = COALESCE(sqlc.narg('recent_30d_orders'), recent_30d_orders),
    recent_30d_cancels = COALESCE(sqlc.narg('recent_30d_cancels'), recent_30d_cancels),
    recent_90d_claims = COALESCE(sqlc.narg('recent_90d_claims'), recent_90d_claims),
    recent_90d_orders = COALESCE(sqlc.narg('recent_90d_orders'), recent_90d_orders),
    is_blacklisted = COALESCE(sqlc.narg('is_blacklisted'), is_blacklisted),
    blacklist_reason = COALESCE(sqlc.narg('blacklist_reason'), blacklist_reason),
    blacklisted_at = COALESCE(sqlc.narg('blacklisted_at'), blacklisted_at),
    updated_at = NOW()
WHERE user_id = $1 AND role = $2;

-- name: IncrementUserClaimCount :exec
UPDATE user_profiles
SET total_claims = total_claims + 1,
    recent_7d_claims = recent_7d_claims + 1,
    recent_30d_claims = recent_30d_claims + 1,
    recent_90d_claims = recent_90d_claims + 1,
    updated_at = NOW()
WHERE user_id = $1 AND role = $2;

-- name: BlacklistUser :exec
UPDATE user_profiles
SET is_blacklisted = true,
    blacklist_reason = $3,
    blacklisted_at = NOW(),
    updated_at = NOW()
WHERE user_id = $1 AND role = $2;

-- name: UnblacklistUser :exec
UPDATE user_profiles
SET is_blacklisted = false,
    blacklist_reason = NULL,
    blacklisted_at = NULL,
    updated_at = NOW()
WHERE user_id = $1 AND role = $2;

-- ==========================================
-- merchant_profiles（商户信任画像）
-- ==========================================

-- name: CreateMerchantProfile :one
INSERT INTO merchant_profiles (
    merchant_id,
    trust_score
) VALUES (
    $1, $2
) RETURNING *;

-- name: GetMerchantProfile :one
SELECT * FROM merchant_profiles
WHERE merchant_id = $1
LIMIT 1;

-- name: GetMerchantProfileForUpdate :one
SELECT * FROM merchant_profiles
WHERE merchant_id = $1
LIMIT 1
FOR UPDATE;

-- name: UpdateMerchantTrustScore :exec
UPDATE merchant_profiles
SET trust_score = $2,
    updated_at = NOW()
WHERE merchant_id = $1;

-- name: UpdateMerchantProfile :exec
UPDATE merchant_profiles
SET trust_score = COALESCE(sqlc.narg('trust_score'), trust_score),
    total_orders = COALESCE(sqlc.narg('total_orders'), total_orders),
    total_sales = COALESCE(sqlc.narg('total_sales'), total_sales),
    completed_orders = COALESCE(sqlc.narg('completed_orders'), completed_orders),
    total_claims = COALESCE(sqlc.narg('total_claims'), total_claims),
    foreign_object_claims = COALESCE(sqlc.narg('foreign_object_claims'), foreign_object_claims),
    food_safety_incidents = COALESCE(sqlc.narg('food_safety_incidents'), food_safety_incidents),
    timeout_count = COALESCE(sqlc.narg('timeout_count'), timeout_count),
    refuse_order_count = COALESCE(sqlc.narg('refuse_order_count'), refuse_order_count),
    recent_7d_claims = COALESCE(sqlc.narg('recent_7d_claims'), recent_7d_claims),
    recent_7d_incidents = COALESCE(sqlc.narg('recent_7d_incidents'), recent_7d_incidents),
    recent_30d_claims = COALESCE(sqlc.narg('recent_30d_claims'), recent_30d_claims),
    recent_30d_incidents = COALESCE(sqlc.narg('recent_30d_incidents'), recent_30d_incidents),
    recent_30d_timeouts = COALESCE(sqlc.narg('recent_30d_timeouts'), recent_30d_timeouts),
    recent_90d_claims = COALESCE(sqlc.narg('recent_90d_claims'), recent_90d_claims),
    recent_90d_incidents = COALESCE(sqlc.narg('recent_90d_incidents'), recent_90d_incidents),
    is_suspended = COALESCE(sqlc.narg('is_suspended'), is_suspended),
    suspend_reason = COALESCE(sqlc.narg('suspend_reason'), suspend_reason),
    suspended_at = COALESCE(sqlc.narg('suspended_at'), suspended_at),
    suspend_until = COALESCE(sqlc.narg('suspend_until'), suspend_until),
    updated_at = NOW()
WHERE merchant_id = $1;

-- name: IncrementMerchantForeignObjectClaim :exec
UPDATE merchant_profiles
SET foreign_object_claims = foreign_object_claims + 1,
    total_claims = total_claims + 1,
    recent_7d_claims = recent_7d_claims + 1,
    recent_30d_claims = recent_30d_claims + 1,
    recent_90d_claims = recent_90d_claims + 1,
    updated_at = NOW()
WHERE merchant_id = $1;

-- name: SuspendMerchant :exec
UPDATE merchant_profiles
SET is_suspended = true,
    suspend_reason = $2,
    suspended_at = NOW(),
    suspend_until = $3,
    updated_at = NOW()
WHERE merchant_id = $1;

-- name: UnsuspendMerchant :exec
UPDATE merchant_profiles
SET is_suspended = false,
    suspend_reason = NULL,
    suspended_at = NULL,
    suspend_until = NULL,
    updated_at = NOW()
WHERE merchant_id = $1;

-- ==========================================
-- rider_profiles（骑手信任画像）
-- ==========================================

-- name: CreateRiderProfile :one
INSERT INTO rider_profiles (
    rider_id,
    trust_score
) VALUES (
    $1, $2
) RETURNING *;

-- name: GetRiderProfile :one
SELECT * FROM rider_profiles
WHERE rider_id = $1
LIMIT 1;

-- name: GetRiderProfileForUpdate :one
SELECT * FROM rider_profiles
WHERE rider_id = $1
LIMIT 1
FOR UPDATE;

-- name: UpdateRiderTrustScore :exec
UPDATE rider_profiles
SET trust_score = $2,
    updated_at = NOW()
WHERE rider_id = $1;

-- name: UpdateRiderProfile :exec
UPDATE rider_profiles
SET trust_score = COALESCE(sqlc.narg('trust_score'), trust_score),
    total_deliveries = COALESCE(sqlc.narg('total_deliveries'), total_deliveries),
    completed_deliveries = COALESCE(sqlc.narg('completed_deliveries'), completed_deliveries),
    on_time_deliveries = COALESCE(sqlc.narg('on_time_deliveries'), on_time_deliveries),
    delayed_deliveries = COALESCE(sqlc.narg('delayed_deliveries'), delayed_deliveries),
    cancelled_deliveries = COALESCE(sqlc.narg('cancelled_deliveries'), cancelled_deliveries),
    total_damage_incidents = COALESCE(sqlc.narg('total_damage_incidents'), total_damage_incidents),
    customer_complaints = COALESCE(sqlc.narg('customer_complaints'), customer_complaints),
    timeout_incidents = COALESCE(sqlc.narg('timeout_incidents'), timeout_incidents),
    recent_7d_damages = COALESCE(sqlc.narg('recent_7d_damages'), recent_7d_damages),
    recent_7d_delays = COALESCE(sqlc.narg('recent_7d_delays'), recent_7d_delays),
    recent_30d_damages = COALESCE(sqlc.narg('recent_30d_damages'), recent_30d_damages),
    recent_30d_delays = COALESCE(sqlc.narg('recent_30d_delays'), recent_30d_delays),
    recent_30d_complaints = COALESCE(sqlc.narg('recent_30d_complaints'), recent_30d_complaints),
    recent_90d_damages = COALESCE(sqlc.narg('recent_90d_damages'), recent_90d_damages),
    recent_90d_delays = COALESCE(sqlc.narg('recent_90d_delays'), recent_90d_delays),
    total_online_hours = COALESCE(sqlc.narg('total_online_hours'), total_online_hours),
    is_suspended = COALESCE(sqlc.narg('is_suspended'), is_suspended),
    suspend_reason = COALESCE(sqlc.narg('suspend_reason'), suspend_reason),
    suspended_at = COALESCE(sqlc.narg('suspended_at'), suspended_at),
    suspend_until = COALESCE(sqlc.narg('suspend_until'), suspend_until),
    updated_at = NOW()
WHERE rider_id = $1;

-- name: IncrementRiderDamageIncident :exec
UPDATE rider_profiles
SET total_damage_incidents = total_damage_incidents + 1,
    recent_7d_damages = recent_7d_damages + 1,
    recent_30d_damages = recent_30d_damages + 1,
    recent_90d_damages = recent_90d_damages + 1,
    updated_at = NOW()
WHERE rider_id = $1;

-- name: SuspendRider :exec
UPDATE rider_profiles
SET is_suspended = true,
    suspend_reason = $2,
    suspended_at = NOW(),
    suspend_until = $3,
    updated_at = NOW()
WHERE rider_id = $1;

-- name: UnsuspendRider :exec
UPDATE rider_profiles
SET is_suspended = false,
    suspend_reason = NULL,
    suspended_at = NULL,
    suspend_until = NULL,
    updated_at = NOW()
WHERE rider_id = $1;

-- ==========================================
-- claims（索赔记录）
-- ==========================================

-- name: CreateClaim :one
INSERT INTO claims (
    order_id,
    user_id,
    claim_type,
    description,
    evidence_urls,
    claim_amount,
    approved_amount,
    status,
    approval_type,
    trust_score_snapshot,
    is_malicious,
    lookback_result,
    auto_approval_reason,
    rejection_reason,
    reviewer_id,
    review_notes,
    created_at,
    reviewed_at,
    paid_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
) RETURNING *;

-- name: GetClaim :one
SELECT * FROM claims
WHERE id = $1
LIMIT 1;

-- name: GetClaimForUpdate :one
SELECT * FROM claims
WHERE id = $1
LIMIT 1
FOR UPDATE;

-- name: ListUserClaims :many
SELECT * FROM claims
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListUserClaimsInPeriod :many
SELECT * FROM claims
WHERE user_id = $1
  AND created_at >= $2
ORDER BY created_at DESC;

-- name: ListMerchantClaims :many
SELECT c.* FROM claims c
JOIN orders o ON c.order_id = o.id
WHERE o.merchant_id = $1
  AND c.created_at >= $2
ORDER BY c.created_at DESC;

-- name: ListRiderClaims :many
SELECT c.* FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN deliveries d ON o.id = d.order_id
WHERE d.rider_id = $1
  AND c.created_at >= $2
ORDER BY c.created_at DESC;

-- name: GetUserRecentClaims :many
SELECT * FROM claims
WHERE user_id = $1
  AND created_at >= NOW() - INTERVAL '30 days'
ORDER BY created_at DESC
LIMIT 5;

-- name: CountUserClaimsInPeriod :one
SELECT COUNT(*) FROM claims
WHERE user_id = $1
  AND created_at >= $2;

-- name: UpdateClaimStatus :exec
UPDATE claims
SET status = $2,
    approval_type = COALESCE(sqlc.narg('approval_type'), approval_type),
    approved_amount = COALESCE(sqlc.narg('approved_amount'), approved_amount),
    is_malicious = COALESCE(sqlc.narg('is_malicious'), is_malicious),
    auto_approval_reason = COALESCE(sqlc.narg('auto_approval_reason'), auto_approval_reason),
    rejection_reason = COALESCE(sqlc.narg('rejection_reason'), rejection_reason),
    reviewer_id = COALESCE(sqlc.narg('reviewer_id'), reviewer_id),
    review_notes = COALESCE(sqlc.narg('review_notes'), review_notes),
    reviewed_at = COALESCE(sqlc.narg('reviewed_at'), reviewed_at),
    paid_at = COALESCE(sqlc.narg('paid_at'), paid_at)
WHERE id = $1;

-- name: UpdateClaimLookbackResult :exec
UPDATE claims
SET lookback_result = $2
WHERE id = $1;

-- name: GetPendingClaims :many
SELECT * FROM claims
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1;

-- name: GetMaliciousClaims :many
SELECT * FROM claims
WHERE is_malicious = true
  AND created_at >= $1
ORDER BY created_at DESC;

-- ==========================================
-- food_safety_incidents（食品安全事件）
-- ==========================================

-- name: CreateFoodSafetyIncident :one
INSERT INTO food_safety_incidents (
    order_id,
    merchant_id,
    user_id,
    incident_type,
    description,
    evidence_urls,
    order_snapshot,
    merchant_snapshot,
    rider_snapshot,
    status,
    investigation_report,
    resolution,
    created_at,
    resolved_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
) RETURNING *;

-- name: GetFoodSafetyIncident :one
SELECT * FROM food_safety_incidents
WHERE id = $1
LIMIT 1;

-- name: ListMerchantFoodSafetyIncidents :many
SELECT * FROM food_safety_incidents
WHERE merchant_id = $1
  AND created_at >= $2
ORDER BY created_at DESC;

-- name: GetMerchantRecentFoodSafetyReports :many
SELECT * FROM food_safety_incidents
WHERE merchant_id = $1
  AND created_at >= NOW() - INTERVAL '1 hour'
ORDER BY created_at DESC;

-- name: UpdateFoodSafetyIncidentStatus :exec
UPDATE food_safety_incidents
SET status = $2,
    investigation_report = COALESCE(sqlc.narg('investigation_report'), investigation_report),
    resolution = COALESCE(sqlc.narg('resolution'), resolution),
    resolved_at = COALESCE(sqlc.narg('resolved_at'), resolved_at)
WHERE id = $1;

-- name: GetActiveFoodSafetyIncidents :many
SELECT * FROM food_safety_incidents
WHERE status IN ('reported', 'investigating', 'merchant-suspended')
ORDER BY created_at DESC
LIMIT $1;

-- ==========================================
-- fraud_patterns（欺诈模式检测）
-- ==========================================

-- name: CreateFraudPattern :one
INSERT INTO fraud_patterns (
    pattern_type,
    related_user_ids,
    related_order_ids,
    related_claim_ids,
    device_fingerprints,
    address_ids,
    ip_addresses,
    pattern_description,
    match_count,
    is_confirmed,
    reviewer_id,
    review_notes,
    action_taken,
    detected_at,
    reviewed_at,
    confirmed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
) RETURNING *;

-- name: GetFraudPattern :one
SELECT * FROM fraud_patterns
WHERE id = $1
LIMIT 1;

-- name: ListFraudPatterns :many
SELECT * FROM fraud_patterns
WHERE pattern_type = sqlc.narg('pattern_type')
  AND detected_at >= sqlc.narg('from_date')
ORDER BY detected_at DESC
LIMIT $1 OFFSET $2;

-- name: GetUnconfirmedFraudPatterns :many
SELECT * FROM fraud_patterns
WHERE is_confirmed = false
ORDER BY match_count DESC, detected_at DESC
LIMIT $1;

-- name: ConfirmFraudPattern :exec
UPDATE fraud_patterns
SET is_confirmed = true,
    action_taken = $2,
    confirmed_at = NOW()
WHERE id = $1;

-- name: UpdateFraudPatternReview :exec
UPDATE fraud_patterns
SET reviewer_id = $2,
    review_notes = $3,
    reviewed_at = NOW()
WHERE id = $1;

-- name: GetFraudPatternsByUsers :many
SELECT * FROM fraud_patterns
WHERE related_user_ids && $1::bigint[]
ORDER BY detected_at DESC;

-- name: GetFraudPatternsByDevice :many
SELECT * FROM fraud_patterns
WHERE device_fingerprints && $1::text[]
  AND detected_at >= $2
ORDER BY detected_at DESC;

-- ==========================================
-- trust_score_changes（信任分变更日志）
-- ==========================================

-- name: CreateTrustScoreChange :one
INSERT INTO trust_score_changes (
    entity_type,
    entity_id,
    old_score,
    new_score,
    score_change,
    reason_type,
    reason_description,
    related_type,
    related_id,
    is_auto,
    operator_id,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
) RETURNING *;

-- name: GetTrustScoreChange :one
SELECT * FROM trust_score_changes
WHERE id = $1
LIMIT 1;

-- name: ListEntityTrustScoreChanges :many
SELECT * FROM trust_score_changes
WHERE entity_type = $1 AND entity_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetRecentTrustScoreChanges :many
SELECT * FROM trust_score_changes
WHERE entity_type = $1 AND entity_id = $2
  AND created_at >= $3
ORDER BY created_at DESC;

-- name: GetTrustScoreChangesByReason :many
SELECT * FROM trust_score_changes
WHERE entity_type = $1 AND entity_id = $2
  AND reason_type = $3
ORDER BY created_at DESC
LIMIT $4;

-- name: GetTotalScoreChangeByReason :one
SELECT COALESCE(SUM(score_change), 0) as total_change
FROM trust_score_changes
WHERE entity_type = $1 AND entity_id = $2
  AND reason_type = $3;

-- ==========================================
-- 回溯检查专用查询
-- ==========================================

-- name: GetOrdersMerchantAndRider :many
SELECT o.id as order_id, o.merchant_id, d.rider_id
FROM orders o
LEFT JOIN deliveries d ON o.id = d.order_id
WHERE o.id = ANY($1::bigint[]);

-- ==========================================
-- 设备指纹查询（M9欺诈检测）
-- ==========================================

-- name: UpsertUserDevice :one
INSERT INTO user_devices (
    user_id,
    device_id,
    device_type,
    first_seen,
    last_seen
) VALUES (
    $1, $2, $3, NOW(), NOW()
) ON CONFLICT (user_id, device_id)
DO UPDATE SET
    last_seen = NOW(),
    updated_at = NOW()
RETURNING *;

-- name: GetUsersByDeviceID :many
SELECT DISTINCT user_id
FROM user_devices
WHERE device_id = $1
ORDER BY last_seen DESC;

-- name: GetDevicesByUserID :many
SELECT *
FROM user_devices
WHERE user_id = $1
ORDER BY last_seen DESC;

-- name: GetUsersWithRecentClaims :many
-- 查询最近N天内有索赔记录的用户（设备欺诈检测）
SELECT DISTINCT c.user_id
FROM claims c
WHERE c.user_id = ANY($1::bigint[])
  AND c.created_at >= NOW() - INTERVAL '1 day' * $2
ORDER BY c.created_at DESC;

-- name: CountRecentClaimsByUsers :one
-- 统计多个用户最近N天的索赔总数
SELECT COUNT(*) as claim_count
FROM claims
WHERE user_id = ANY($1::bigint[])
  AND created_at >= NOW() - INTERVAL '1 day' * $2;

-- ==========================================
-- 地址聚类查询（M9欺诈检测）
-- ==========================================

-- name: GetUsersByAddressID :many
-- 查询使用相同地址ID的用户
SELECT DISTINCT o.user_id, COUNT(DISTINCT o.id) as order_count
FROM orders o
INNER JOIN user_addresses ua ON o.address_id = ua.id
WHERE ua.id = $1
GROUP BY o.user_id
ORDER BY order_count DESC;

-- name: GetUsersBySimilarAddress :many
-- 查询地址高度相似的用户（基于region_id和详细地址模糊匹配）
SELECT DISTINCT ua.user_id, ua.id as address_id, ua.detail_address, COUNT(o.id) as order_count
FROM user_addresses ua
LEFT JOIN orders o ON o.address_id = ua.id
WHERE ua.region_id = $1
  AND ua.detail_address LIKE '%' || $2 || '%'
  AND ua.user_id != $3  -- 排除当前用户
GROUP BY ua.user_id, ua.id, ua.detail_address
HAVING COUNT(o.id) > 0
ORDER BY order_count DESC
LIMIT 50;

-- ==========================================
-- 欺诈检测：时间窗口查询
-- ==========================================

-- name: ListClaimsByTimeWindow :many
-- 查询指定时间窗口内的索赔（用于协同欺诈检测）
SELECT c.*, o.address_id
FROM claims c
JOIN orders o ON c.order_id = o.id
WHERE c.created_at >= $1
  AND c.created_at <= $2
  AND c.id != $3  -- 排除当前索赔
ORDER BY c.created_at DESC;

-- name: GetClaimsWithSameAddress :many
-- 查询使用相同配送地址的索赔
SELECT DISTINCT c.*, c.user_id
FROM claims c
JOIN orders o ON c.order_id = o.id
WHERE o.address_id = $1
  AND c.created_at >= $2
  AND c.id != $3
ORDER BY c.created_at DESC;

-- name: CountDistinctUsersInClaimWindow :one
-- 统计时间窗口内发起索赔的不同用户数
SELECT COUNT(DISTINCT user_id) as user_count
FROM claims
WHERE created_at >= $1
  AND created_at <= $2;

-- ==========================================
-- 欺诈处理：损失返还
-- ==========================================

-- name: GetClaimsByFraudPattern :many
-- 获取欺诈模式关联的所有索赔详情
SELECT c.*, o.merchant_id, d.rider_id
FROM claims c
JOIN orders o ON c.order_id = o.id
LEFT JOIN deliveries d ON o.id = d.order_id
WHERE c.id = ANY($1::bigint[])
ORDER BY c.created_at;

-- name: SumClaimAmountsByMerchant :many
-- 按商户统计索赔损失金额
SELECT o.merchant_id, SUM(c.approved_amount)::bigint as total_loss
FROM claims c
JOIN orders o ON c.order_id = o.id
WHERE c.id = ANY($1::bigint[])
  AND c.approved_amount IS NOT NULL
GROUP BY o.merchant_id;

-- name: SumClaimAmountsByRider :many
-- 按骑手统计索赔损失金额（餐损类型）
SELECT d.rider_id, SUM(c.approved_amount)::bigint as total_loss
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN deliveries d ON o.id = d.order_id
WHERE c.id = ANY($1::bigint[])
  AND c.claim_type = 'damage'
  AND c.approved_amount IS NOT NULL
  AND d.rider_id IS NOT NULL
GROUP BY d.rider_id;

-- ==========================================
-- 运营商索赔管理（区域内待审核索赔）
-- ==========================================

-- name: ListRegionPendingClaims :many
-- 获取运营商区域内待人工审核的索赔列表
SELECT c.*, 
       o.order_no,
       o.merchant_id,
       m.name as merchant_name,
       u.phone as user_phone,
       m.region_id
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN merchants m ON o.merchant_id = m.id
JOIN users u ON c.user_id = u.id
WHERE m.region_id = $1
  AND c.status = 'manual-review'
ORDER BY c.created_at ASC
LIMIT $2 OFFSET $3;

-- name: CountRegionPendingClaims :one
-- 统计运营商区域内待人工审核的索赔数量
SELECT COUNT(*) as total
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN merchants m ON o.merchant_id = m.id
WHERE m.region_id = $1
  AND c.status = 'manual-review';

-- name: GetClaimWithDetails :one
-- 获取索赔详情（包含订单、商户、用户信息）
SELECT c.*,
       o.order_no,
       o.merchant_id,
       o.total_amount as order_amount,
       o.status as order_status,
       o.created_at as order_created_at,
       m.name as merchant_name,
       m.region_id,
       u.phone as user_phone,
       u.full_name as user_name
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN merchants m ON o.merchant_id = m.id
JOIN users u ON c.user_id = u.id
WHERE c.id = $1
LIMIT 1;

-- name: ReviewClaim :exec
-- 运营商审核索赔
UPDATE claims
SET status = $2,
    approved_amount = $3,
    reviewer_id = $4,
    review_notes = $5,
    reviewed_at = NOW()
WHERE id = $1;
-- ==========================================
-- 用户索赔行为回溯查询（新设计）
-- ==========================================

-- name: CountUserRecentTakeoutOrders :one
-- 统计用户近N天的外卖订单数（用于行为回溯）
SELECT COUNT(*) as total
FROM orders
WHERE user_id = $1
  AND order_type = 'takeout'
  AND created_at >= NOW() - ($2 || ' days')::INTERVAL;

-- name: CountUserRecentClaims :one
-- 统计用户近N天的索赔次数（用于行为回溯）
SELECT COUNT(*) as total
FROM claims
WHERE user_id = $1
  AND created_at >= NOW() - ($2 || ' days')::INTERVAL;

-- name: GetUserClaimWarningStatus :one
-- 获取用户索赔警告状态
SELECT * FROM user_claim_warnings
WHERE user_id = $1
LIMIT 1;

-- name: CreateUserClaimWarning :one
-- 创建用户索赔警告记录
INSERT INTO user_claim_warnings (
    user_id,
    warning_count,
    last_warning_at,
    last_warning_reason,
    requires_evidence,
    platform_pay_count,
    created_at,
    updated_at
) VALUES (
    $1, 1, NOW(), $2, $3, 0, NOW(), NOW()
) RETURNING *;

-- name: IncrementUserClaimWarning :exec
-- 增加用户警告次数
UPDATE user_claim_warnings
SET warning_count = warning_count + 1,
    last_warning_at = NOW(),
    last_warning_reason = $2,
    requires_evidence = $3,
    updated_at = NOW()
WHERE user_id = $1;

-- name: SetUserRequiresEvidence :exec
-- 设置用户需要提交证据
UPDATE user_claim_warnings
SET requires_evidence = true,
    last_warning_reason = $2,
    updated_at = NOW()
WHERE user_id = $1;

-- name: IncrementUserPlatformPayCount :exec
-- 增加平台垫付次数
UPDATE user_claim_warnings
SET platform_pay_count = platform_pay_count + 1,
    last_warning_at = NOW(),
    last_warning_reason = $2,
    updated_at = NOW()
WHERE user_id = $1;

-- name: GetUserBehaviorStats :one
-- 获取用户行为统计（用于索赔判定）
SELECT 
    (SELECT COUNT(*) FROM orders o WHERE o.user_id = $1 AND o.order_type = 'takeout' 
     AND o.created_at >= NOW() - INTERVAL '90 days')::INT as takeout_orders_90d,
    (SELECT COUNT(*) FROM claims c WHERE c.user_id = $1 
     AND c.created_at >= NOW() - INTERVAL '90 days')::INT as claims_90d,
    COALESCE(w.warning_count, 0)::INT as warning_count,
    COALESCE(w.requires_evidence, false) as requires_evidence,
    COALESCE(w.platform_pay_count, 0)::INT as platform_pay_count
FROM (SELECT 1) as dummy
LEFT JOIN user_claim_warnings w ON w.user_id = $1;

-- ==========================================
-- 商户异物索赔追踪查询
-- ==========================================

-- name: CountMerchantClaimsByType :one
-- 统计商户在指定时间窗口内特定类型的索赔数量
SELECT COUNT(*) as total
FROM claims c
JOIN orders o ON c.order_id = o.id
WHERE o.merchant_id = $1
  AND c.claim_type = $2
  AND c.created_at >= $3;

-- name: ListMerchantClaimsByTypeInPeriod :many
-- 获取商户在指定时间窗口内特定类型的索赔列表
SELECT c.*
FROM claims c
JOIN orders o ON c.order_id = o.id
WHERE o.merchant_id = $1
  AND c.claim_type = $2
  AND c.created_at >= $3
ORDER BY c.created_at DESC;

-- ==========================================
-- 骑手统计查询（保留用于统计展示）
-- ==========================================

-- name: CountRiderDamageClaims :one
-- 统计骑手在指定时间窗口内的餐损索赔次数
SELECT COUNT(*) as total
FROM claims c
JOIN deliveries d ON c.order_id = d.order_id
WHERE d.rider_id = $1
  AND c.claim_type = $2
  AND c.created_at >= $3;

-- name: GetRiderDeliveryStats :one
-- 获取骑手在指定时间窗口内的配送统计
SELECT 
    COUNT(*) as total_orders,
    COUNT(*) FILTER (WHERE status = 'delivered') as completed_orders,
    COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled_orders,
    COUNT(*) FILTER (WHERE is_delayed = true) as timeout_orders
FROM deliveries
WHERE rider_id = $1
  AND created_at >= $2;

-- name: ListUserRecentOrders :many
-- 获取用户最近的订单（用于食安恶作剧检测）
SELECT id, address_id
FROM orders
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;