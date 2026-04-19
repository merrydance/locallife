-- M9: TrustScore信任分系统查询
-- 设计理念：信用驱动，非证据驱动

-- ==========================================
-- merchant_profiles（商户信任画像）
-- ==========================================

-- name: CreateMerchantProfile :one
INSERT INTO merchant_profiles (
  merchant_id
) VALUES (
  $1
) RETURNING id, merchant_id, total_orders, total_sales, completed_orders, total_claims, foreign_object_claims, food_safety_incidents, timeout_count, refuse_order_count, recent_7d_claims, recent_7d_incidents, recent_30d_claims, recent_30d_incidents, recent_30d_timeouts, recent_90d_claims, recent_90d_incidents, is_suspended, suspend_reason, suspended_at, suspend_until, is_takeout_suspended, takeout_suspend_reason, takeout_suspended_at, takeout_suspend_until, updated_at;

-- name: GetMerchantProfile :one
SELECT id, merchant_id, total_orders, total_sales, completed_orders, total_claims, foreign_object_claims, food_safety_incidents, timeout_count, refuse_order_count, recent_7d_claims, recent_7d_incidents, recent_30d_claims, recent_30d_incidents, recent_30d_timeouts, recent_90d_claims, recent_90d_incidents, is_suspended, suspend_reason, suspended_at, suspend_until, is_takeout_suspended, takeout_suspend_reason, takeout_suspended_at, takeout_suspend_until, updated_at
FROM merchant_profiles
WHERE merchant_id = $1
LIMIT 1;

-- name: GetMerchantProfileForUpdate :one
SELECT id, merchant_id, total_orders, total_sales, completed_orders, total_claims, foreign_object_claims, food_safety_incidents, timeout_count, refuse_order_count, recent_7d_claims, recent_7d_incidents, recent_30d_claims, recent_30d_incidents, recent_30d_timeouts, recent_90d_claims, recent_90d_incidents, is_suspended, suspend_reason, suspended_at, suspend_until, is_takeout_suspended, takeout_suspend_reason, takeout_suspended_at, takeout_suspend_until, updated_at
FROM merchant_profiles
WHERE merchant_id = $1
LIMIT 1
FOR UPDATE;

-- name: UpdateMerchantProfile :exec
UPDATE merchant_profiles
SET total_orders = COALESCE(sqlc.narg('total_orders'), total_orders),
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
    is_takeout_suspended = COALESCE(sqlc.narg('is_takeout_suspended'), is_takeout_suspended),
    takeout_suspend_reason = COALESCE(sqlc.narg('takeout_suspend_reason'), takeout_suspend_reason),
    takeout_suspended_at = COALESCE(sqlc.narg('takeout_suspended_at'), takeout_suspended_at),
    takeout_suspend_until = COALESCE(sqlc.narg('takeout_suspend_until'), takeout_suspend_until),
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

-- name: SuspendMerchantTakeout :exec
UPDATE merchant_profiles
SET is_takeout_suspended = true,
    takeout_suspend_reason = $2,
    takeout_suspended_at = NOW(),
    takeout_suspend_until = $3,
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

-- name: UnsuspendMerchantTakeout :exec
UPDATE merchant_profiles
SET is_takeout_suspended = false,
    takeout_suspend_reason = NULL,
    takeout_suspended_at = NULL,
    takeout_suspend_until = NULL,
    updated_at = NOW()
WHERE merchant_id = $1;

-- ==========================================
-- rider_profiles（骑手信任画像）
-- ==========================================

-- name: CreateRiderProfile :one
INSERT INTO rider_profiles (
  rider_id
) VALUES (
  $1
) RETURNING *;

-- name: GetRiderProfile :one
SELECT id, rider_id, total_deliveries, completed_deliveries, on_time_deliveries, delayed_deliveries, cancelled_deliveries, total_damage_incidents, customer_complaints, timeout_incidents, recent_7d_damages, recent_7d_delays, recent_30d_damages, recent_30d_delays, recent_30d_complaints, recent_90d_damages, recent_90d_delays, total_online_hours, is_suspended, suspend_reason, suspended_at, suspend_until, updated_at FROM rider_profiles
WHERE rider_id = $1
LIMIT 1;

-- name: GetRiderProfileForUpdate :one
SELECT id, rider_id, total_deliveries, completed_deliveries, on_time_deliveries, delayed_deliveries, cancelled_deliveries, total_damage_incidents, customer_complaints, timeout_incidents, recent_7d_damages, recent_7d_delays, recent_30d_damages, recent_30d_delays, recent_30d_complaints, recent_90d_damages, recent_90d_delays, total_online_hours, is_suspended, suspend_reason, suspended_at, suspend_until, updated_at FROM rider_profiles
WHERE rider_id = $1
LIMIT 1
FOR UPDATE;

-- name: UpdateRiderProfile :exec
UPDATE rider_profiles
SET total_deliveries = COALESCE(sqlc.narg('total_deliveries'), total_deliveries),
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
    claim_amount,
    approved_amount,
    status,
    approval_type,
    is_malicious,
    lookback_result,
    auto_approval_reason,
    rejection_reason,
    reviewer_id,
    review_notes,
    decision_version,
    decision_reason,
    created_at,
    reviewed_at,
    paid_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
) RETURNING *;

-- name: GetClaim :one
SELECT id, order_id, user_id, claim_type, description, claim_amount, approved_amount, status, approval_type, is_malicious, lookback_result, auto_approval_reason, rejection_reason, reviewer_id, review_notes, created_at, reviewed_at, paid_at, decision_version, decision_reason FROM claims
WHERE id = $1
LIMIT 1;

-- name: GetClaimForUpdate :one
SELECT id, order_id, user_id, claim_type, description, claim_amount, approved_amount, status, approval_type, is_malicious, lookback_result, auto_approval_reason, rejection_reason, reviewer_id, review_notes, created_at, reviewed_at, paid_at, decision_version, decision_reason FROM claims
WHERE id = $1
LIMIT 1
FOR UPDATE;

-- name: ListUserClaims :many
SELECT id, order_id, user_id, claim_type, description, claim_amount, approved_amount, status, approval_type, is_malicious, lookback_result, auto_approval_reason, rejection_reason, reviewer_id, review_notes, created_at, reviewed_at, paid_at, decision_version, decision_reason FROM claims
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListUserClaimsInPeriod :many
SELECT id, order_id, user_id, claim_type, description, claim_amount, approved_amount, status, approval_type, is_malicious, lookback_result, auto_approval_reason, rejection_reason, reviewer_id, review_notes, created_at, reviewed_at, paid_at, decision_version, decision_reason FROM claims
WHERE user_id = $1
  AND created_at >= $2
ORDER BY created_at DESC;

-- name: ListMerchantClaims :many
SELECT c.id, c.order_id, c.user_id, c.claim_type, c.description, c.claim_amount, c.approved_amount, c.status, c.approval_type, c.is_malicious, c.lookback_result, c.auto_approval_reason, c.rejection_reason, c.reviewer_id, c.review_notes, c.created_at, c.reviewed_at, c.paid_at, c.decision_version, c.decision_reason FROM claims c
JOIN orders o ON c.order_id = o.id
WHERE o.merchant_id = $1
  AND c.created_at >= $2
ORDER BY c.created_at DESC;

-- name: ListRiderClaims :many
SELECT c.id, c.order_id, c.user_id, c.claim_type, c.description, c.claim_amount, c.approved_amount, c.status, c.approval_type, c.is_malicious, c.lookback_result, c.auto_approval_reason, c.rejection_reason, c.reviewer_id, c.review_notes, c.created_at, c.reviewed_at, c.paid_at, c.decision_version, c.decision_reason FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN deliveries d ON o.id = d.order_id
WHERE d.rider_id = $1
  AND c.created_at >= $2
ORDER BY c.created_at DESC;

-- name: GetUserRecentClaims :many
SELECT id, order_id, user_id, claim_type, description, claim_amount, approved_amount, status, approval_type, is_malicious, lookback_result, auto_approval_reason, rejection_reason, reviewer_id, review_notes, created_at, reviewed_at, paid_at, decision_version, decision_reason FROM claims
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

-- name: MarkClaimPaid :exec
UPDATE claims
SET paid_at = COALESCE(paid_at, $2)
WHERE id = $1;

-- name: GetPendingClaims :many
SELECT id, order_id, user_id, claim_type, description, claim_amount, approved_amount, status, approval_type, is_malicious, lookback_result, auto_approval_reason, rejection_reason, reviewer_id, review_notes, created_at, reviewed_at, paid_at, decision_version, decision_reason FROM claims
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1;

-- name: GetMaliciousClaims :many
SELECT id, order_id, user_id, claim_type, description, claim_amount, approved_amount, status, approval_type, is_malicious, lookback_result, auto_approval_reason, rejection_reason, reviewer_id, review_notes, created_at, reviewed_at, paid_at, decision_version, decision_reason FROM claims
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
    order_snapshot,
    merchant_snapshot,
    rider_snapshot,
    status,
    investigation_report,
    resolution,
    created_at,
    resolved_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) RETURNING *;

-- name: GetFoodSafetyIncident :one
SELECT id, order_id, merchant_id, user_id, incident_type, description, order_snapshot, merchant_snapshot, rider_snapshot, status, investigation_report, resolution, created_at, resolved_at FROM food_safety_incidents
WHERE id = $1
LIMIT 1;

-- name: ListMerchantFoodSafetyIncidents :many
SELECT id, order_id, merchant_id, user_id, incident_type, description, order_snapshot, merchant_snapshot, rider_snapshot, status, investigation_report, resolution, created_at, resolved_at FROM food_safety_incidents
WHERE merchant_id = $1
  AND created_at >= $2
ORDER BY created_at DESC;

-- name: GetMerchantRecentFoodSafetyReports :many
SELECT id, order_id, merchant_id, user_id, incident_type, description, order_snapshot, merchant_snapshot, rider_snapshot, status, investigation_report, resolution, created_at, resolved_at FROM food_safety_incidents
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
SELECT id, order_id, merchant_id, user_id, incident_type, description, order_snapshot, merchant_snapshot, rider_snapshot, status, investigation_report, resolution, created_at, resolved_at FROM food_safety_incidents
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
SELECT id, pattern_type, related_user_ids, related_order_ids, related_claim_ids, device_fingerprints, address_ids, ip_addresses, pattern_description, match_count, is_confirmed, reviewer_id, review_notes, action_taken, detected_at, reviewed_at, confirmed_at FROM fraud_patterns
WHERE id = $1
LIMIT 1;

-- name: ListFraudPatterns :many
SELECT id, pattern_type, related_user_ids, related_order_ids, related_claim_ids, device_fingerprints, address_ids, ip_addresses, pattern_description, match_count, is_confirmed, reviewer_id, review_notes, action_taken, detected_at, reviewed_at, confirmed_at FROM fraud_patterns
WHERE pattern_type = sqlc.narg('pattern_type')
  AND detected_at >= sqlc.narg('from_date')
ORDER BY detected_at DESC
LIMIT $1 OFFSET $2;

-- name: GetUnconfirmedFraudPatterns :many
SELECT id, pattern_type, related_user_ids, related_order_ids, related_claim_ids, device_fingerprints, address_ids, ip_addresses, pattern_description, match_count, is_confirmed, reviewer_id, review_notes, action_taken, detected_at, reviewed_at, confirmed_at FROM fraud_patterns
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
SELECT id, pattern_type, related_user_ids, related_order_ids, related_claim_ids, device_fingerprints, address_ids, ip_addresses, pattern_description, match_count, is_confirmed, reviewer_id, review_notes, action_taken, detected_at, reviewed_at, confirmed_at FROM fraud_patterns
WHERE related_user_ids && $1::bigint[]
ORDER BY detected_at DESC;

-- name: GetFraudPatternsByDevice :many
SELECT id, pattern_type, related_user_ids, related_order_ids, related_claim_ids, device_fingerprints, address_ids, ip_addresses, pattern_description, match_count, is_confirmed, reviewer_id, review_notes, action_taken, detected_at, reviewed_at, confirmed_at FROM fraud_patterns
WHERE device_fingerprints && $1::text[]
  AND detected_at >= $2
ORDER BY detected_at DESC;

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
  device_fingerprint,
    device_type,
    first_seen,
    last_seen
) VALUES (
  $1, $2, $3, $4, NOW(), NOW()
) ON CONFLICT (user_id, device_id)
DO UPDATE SET
  device_fingerprint = COALESCE(EXCLUDED.device_fingerprint, user_devices.device_fingerprint),
    last_seen = NOW(),
    updated_at = NOW()
RETURNING *;

-- name: GetUsersByDeviceID :many
SELECT DISTINCT user_id
FROM user_devices
WHERE device_id = $1;

-- name: GetUsersByDeviceFingerprint :many
SELECT DISTINCT user_id
FROM user_devices
WHERE device_fingerprint = $1;

-- name: GetDevicesByUserID :many
SELECT id, user_id, device_id, device_type, device_model, os_version, app_version, user_agent, ip_address, last_login_at, created_at, first_seen, last_seen, updated_at, device_fingerprint
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
SELECT c.id, c.order_id, c.user_id, c.claim_type, c.description, c.claim_amount, c.approved_amount, c.status, c.approval_type, c.is_malicious, c.lookback_result, c.auto_approval_reason, c.rejection_reason, c.reviewer_id, c.review_notes, c.created_at, c.reviewed_at, c.paid_at, c.decision_version, c.decision_reason, o.address_id
FROM claims c
JOIN orders o ON c.order_id = o.id
WHERE c.created_at >= sqlc.arg('start_at')
  AND c.created_at <= sqlc.arg('end_at')
  AND c.id != sqlc.arg('exclude_id')  -- 排除当前索赔
ORDER BY c.created_at DESC;

-- name: GetClaimsWithSameAddress :many
-- 查询使用相同配送地址的索赔
SELECT DISTINCT c.id, c.order_id, c.user_id, c.claim_type, c.description, c.claim_amount, c.approved_amount, c.status, c.approval_type, c.is_malicious, c.lookback_result, c.auto_approval_reason, c.rejection_reason, c.reviewer_id, c.review_notes, c.created_at, c.reviewed_at, c.paid_at, c.decision_version, c.decision_reason, c.user_id
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
WHERE created_at >= sqlc.arg('start_at')
  AND created_at <= sqlc.arg('end_at');

-- ==========================================
-- 欺诈处理：损失返还
-- ==========================================

-- name: GetClaimsByFraudPattern :many
-- 获取欺诈模式关联的所有索赔详情
SELECT c.id, c.order_id, c.user_id, c.claim_type, c.description, c.claim_amount, c.approved_amount, c.status, c.approval_type, c.is_malicious, c.lookback_result, c.auto_approval_reason, c.rejection_reason, c.reviewer_id, c.review_notes, c.created_at, c.reviewed_at, c.paid_at, c.decision_version, c.decision_reason, o.merchant_id, d.rider_id
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


-- name: GetClaimWithDetails :one
-- 获取索赔详情（包含订单、商户、用户信息）
SELECT c.id, c.order_id, c.user_id, c.claim_type, c.description, c.claim_amount, c.approved_amount, c.status, c.approval_type, c.is_malicious, c.lookback_result, c.auto_approval_reason, c.rejection_reason, c.reviewer_id, c.review_notes, c.created_at, c.reviewed_at, c.paid_at, c.decision_version, c.decision_reason,
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
SELECT id, user_id, warning_count, last_warning_at, last_warning_reason, requires_evidence, platform_pay_count, created_at, updated_at FROM user_claim_warnings
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
SELECT c.id, c.order_id, c.user_id, c.claim_type, c.description, c.claim_amount, c.approved_amount, c.status, c.approval_type, c.is_malicious, c.lookback_result, c.auto_approval_reason, c.rejection_reason, c.reviewer_id, c.review_notes, c.created_at, c.reviewed_at, c.paid_at, c.decision_version, c.decision_reason
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
ORDER BY created_at DESC, id DESC
LIMIT $2;