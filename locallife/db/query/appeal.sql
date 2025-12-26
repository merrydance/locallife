-- =====================================================================
-- Appeal Queries - 申诉相关查询
-- =====================================================================

-- =========================== 商户申诉 ===========================

-- name: CreateAppeal :one
-- 创建申诉
INSERT INTO appeals (
    claim_id,
    appellant_type,
    appellant_id,
    reason,
    evidence_urls,
    region_id
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetAppeal :one
-- 获取申诉详情
SELECT * FROM appeals
WHERE id = $1
LIMIT 1;

-- name: GetAppealByClaim :one
-- 根据索赔ID获取申诉
SELECT * FROM appeals
WHERE claim_id = $1
LIMIT 1;

-- name: GetAppealWithDetails :one
-- 获取申诉详情（包含索赔和订单信息）
SELECT 
    a.*,
    c.claim_type,
    c.claim_amount,
    c.approved_amount AS claim_approved_amount,
    c.description AS claim_description,
    c.evidence_urls AS claim_evidence_urls,
    c.status AS claim_status,
    c.created_at AS claim_created_at,
    o.order_no,
    o.total_amount AS order_amount,
    o.status AS order_status,
    o.created_at AS order_created_at,
    u.phone AS user_phone,
    u.full_name AS user_name
FROM appeals a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
JOIN users u ON c.user_id = u.id
WHERE a.id = $1
LIMIT 1;

-- =========================== 商户视角 ===========================

-- name: ListMerchantAppealsForMerchant :many
-- 商户查询自己的申诉列表
SELECT 
    a.*,
    c.claim_type,
    c.claim_amount,
    c.description AS claim_description,
    o.order_no
FROM appeals a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
WHERE a.appellant_type = 'merchant'
  AND a.appellant_id = $1
ORDER BY a.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMerchantAppealsForMerchant :one
-- 商户申诉计数
SELECT COUNT(*) FROM appeals
WHERE appellant_type = 'merchant'
  AND appellant_id = $1;

-- name: GetMerchantAppealDetail :one
-- 商户查看自己的申诉详情
SELECT 
    a.*,
    c.claim_type,
    c.claim_amount,
    c.approved_amount AS claim_approved_amount,
    c.description AS claim_description,
    c.evidence_urls AS claim_evidence_urls,
    o.order_no,
    o.total_amount AS order_amount,
    u.phone AS user_phone
FROM appeals a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
JOIN users u ON c.user_id = u.id
WHERE a.id = $1
  AND a.appellant_type = 'merchant'
  AND a.appellant_id = $2
LIMIT 1;

-- name: ListMerchantClaimsForMerchant :many
-- 商户查看收到的索赔列表（未申诉的+已申诉的）
SELECT 
    c.*,
    o.order_no,
    o.total_amount AS order_amount,
    u.phone AS user_phone,
    u.full_name AS user_name,
    a.id AS appeal_id,
    a.status AS appeal_status
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN users u ON c.user_id = u.id
LEFT JOIN appeals a ON a.claim_id = c.id AND a.appellant_type = 'merchant'
WHERE o.merchant_id = $1
  AND c.status IN ('approved', 'auto-approved')
ORDER BY c.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMerchantClaimsForMerchant :one
-- 商户收到的索赔计数
SELECT COUNT(*) 
FROM claims c
JOIN orders o ON c.order_id = o.id
WHERE o.merchant_id = $1
  AND c.status IN ('approved', 'auto-approved');

-- name: GetMerchantClaimDetailForMerchant :one
-- 商户查看索赔详情
SELECT 
    c.*,
    o.order_no,
    o.total_amount AS order_amount,
    o.created_at AS order_created_at,
    u.phone AS user_phone,
    u.full_name AS user_name,
    a.id AS appeal_id,
    a.status AS appeal_status,
    a.reason AS appeal_reason,
    a.review_notes AS appeal_review_notes
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN users u ON c.user_id = u.id
LEFT JOIN appeals a ON a.claim_id = c.id AND a.appellant_type = 'merchant'
WHERE c.id = $1
  AND o.merchant_id = $2
LIMIT 1;

-- =========================== 骑手视角 ===========================

-- name: ListRiderAppeals :many
-- 骑手查询自己的申诉列表
SELECT 
    a.*,
    c.claim_type,
    c.claim_amount,
    c.description AS claim_description,
    o.order_no
FROM appeals a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
WHERE a.appellant_type = 'rider'
  AND a.appellant_id = $1
ORDER BY a.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountRiderAppeals :one
-- 骑手申诉计数
SELECT COUNT(*) FROM appeals
WHERE appellant_type = 'rider'
  AND appellant_id = $1;

-- name: GetRiderAppealDetail :one
-- 骑手查看自己的申诉详情
SELECT 
    a.*,
    c.claim_type,
    c.claim_amount,
    c.approved_amount AS claim_approved_amount,
    c.description AS claim_description,
    c.evidence_urls AS claim_evidence_urls,
    o.order_no,
    o.total_amount AS order_amount,
    u.phone AS user_phone
FROM appeals a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
JOIN users u ON c.user_id = u.id
WHERE a.id = $1
  AND a.appellant_type = 'rider'
  AND a.appellant_id = $2
LIMIT 1;

-- name: ListRiderClaimsForRider :many
-- 骑手查看收到的索赔列表（通过配送单关联）
SELECT 
    c.*,
    o.order_no,
    o.total_amount AS order_amount,
    u.phone AS user_phone,
    u.full_name AS user_name,
    a.id AS appeal_id,
    a.status AS appeal_status
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN deliveries d ON d.order_id = o.id
JOIN users u ON c.user_id = u.id
LEFT JOIN appeals a ON a.claim_id = c.id AND a.appellant_type = 'rider'
WHERE d.rider_id = $1
  AND c.status IN ('approved', 'auto-approved')
ORDER BY c.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountRiderClaimsForRider :one
-- 骑手收到的索赔计数
SELECT COUNT(*) 
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN deliveries d ON d.order_id = o.id
WHERE d.rider_id = $1
  AND c.status IN ('approved', 'auto-approved');

-- name: GetRiderClaimDetailForRider :one
-- 骑手查看索赔详情
SELECT 
    c.*,
    o.order_no,
    o.total_amount AS order_amount,
    o.created_at AS order_created_at,
    u.phone AS user_phone,
    u.full_name AS user_name,
    a.id AS appeal_id,
    a.status AS appeal_status,
    a.reason AS appeal_reason,
    a.review_notes AS appeal_review_notes
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN deliveries d ON d.order_id = o.id
JOIN users u ON c.user_id = u.id
LEFT JOIN appeals a ON a.claim_id = c.id AND a.appellant_type = 'rider'
WHERE c.id = $1
  AND d.rider_id = $2
LIMIT 1;

-- =========================== 运营商视角 ===========================

-- name: ListOperatorAppeals :many
-- 运营商查询区域内的申诉列表
SELECT 
    a.*,
    c.claim_type,
    c.claim_amount,
    c.description AS claim_description,
    o.order_no,
    o.merchant_id,
    m.name AS merchant_name,
    CASE 
        WHEN a.appellant_type = 'merchant' THEN m.name
        ELSE (SELECT phone FROM riders WHERE id = a.appellant_id)
    END AS appellant_name
FROM appeals a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
JOIN merchants m ON o.merchant_id = m.id
WHERE a.region_id = $1
  AND (NULLIF($2::TEXT, '') IS NULL OR a.status = $2)
ORDER BY 
    CASE WHEN a.status = 'pending' THEN 0 ELSE 1 END,
    a.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountOperatorAppeals :one
-- 运营商申诉计数
SELECT COUNT(*) FROM appeals
WHERE region_id = $1
  AND (NULLIF($2::TEXT, '') IS NULL OR status = $2);

-- name: GetOperatorAppealDetail :one
-- 运营商查看申诉详情
SELECT 
    a.*,
    c.claim_type,
    c.claim_amount,
    c.approved_amount AS claim_approved_amount,
    c.description AS claim_description,
    c.evidence_urls AS claim_evidence_urls,
    c.status AS claim_status,
    c.trust_score_snapshot AS user_trust_score,
    c.lookback_result,
    c.created_at AS claim_created_at,
    o.order_no,
    o.total_amount AS order_amount,
    o.status AS order_status,
    o.created_at AS order_created_at,
    o.merchant_id,
    m.name AS merchant_name,
    m.phone AS merchant_phone,
    u.phone AS user_phone,
    u.full_name AS user_name,
    d.rider_id
FROM appeals a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
JOIN merchants m ON o.merchant_id = m.id
JOIN users u ON c.user_id = u.id
LEFT JOIN deliveries d ON d.order_id = o.id
WHERE a.id = $1
  AND a.region_id = $2
LIMIT 1;

-- name: ReviewAppeal :one
-- 审核申诉
UPDATE appeals
SET status = @status,
    reviewer_id = @reviewer_id,
    review_notes = @review_notes,
    reviewed_at = NOW(),
    compensation_amount = CASE WHEN @status = 'approved' THEN sqlc.narg(compensation_amount)::bigint ELSE NULL END,
    compensated_at = CASE WHEN @status = 'approved' THEN NOW() ELSE NULL END
WHERE id = @id
  AND status = 'pending'
RETURNING *;

-- =========================== 通用查询 ===========================

-- name: CheckAppealExists :one
-- 检查索赔是否已有申诉
SELECT EXISTS (
    SELECT 1 FROM appeals WHERE claim_id = $1
) AS exists;

-- name: GetAppealForPostProcess :one
-- 获取申诉审核后处理所需信息
SELECT 
    a.id AS appeal_id,
    a.claim_id,
    a.appellant_type,
    a.appellant_id,
    a.status AS appeal_status,
    a.compensation_amount,
    c.user_id AS claimant_user_id,
    c.claim_type,
    c.claim_amount,
    c.approved_amount AS claim_approved_amount,
    c.order_id,
    o.order_no,
    o.merchant_id,
    d.rider_id
FROM appeals a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
LEFT JOIN deliveries d ON d.order_id = o.id
WHERE a.id = $1;

-- name: GetClaimForAppeal :one
-- 获取索赔信息（用于创建申诉时验证）
SELECT 
    c.*,
    o.merchant_id,
    m.region_id,
    d.rider_id
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN merchants m ON o.merchant_id = m.id
LEFT JOIN deliveries d ON d.order_id = o.id
WHERE c.id = $1
  AND c.status IN ('approved', 'auto-approved')
LIMIT 1;
