-- =====================================================================
-- Recovery Dispute Queries - 追偿争议相关查询
-- =====================================================================

-- =========================== 追偿争议基础查询 ===========================

-- name: CreateRecoveryDispute :one
-- 创建追偿争议
INSERT INTO recovery_disputes (
    claim_id,
    appellant_type,
    appellant_id,
  reason,
    region_id
) VALUES (
  $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetRecoveryDispute :one
-- 获取追偿争议详情
SELECT id, claim_id, appellant_type, appellant_id, reason, status, reviewer_id, review_notes, reviewed_at, compensation_amount, compensated_at, region_id, created_at FROM recovery_disputes
WHERE id = $1
LIMIT 1;

-- name: GetRecoveryDisputeByClaim :one
-- 根据索赔ID与争议方类型获取追偿争议
SELECT id, claim_id, appellant_type, appellant_id, reason, status, reviewer_id, review_notes, reviewed_at, compensation_amount, compensated_at, region_id, created_at FROM recovery_disputes
WHERE claim_id = $1
  AND appellant_type = $2
LIMIT 1;

-- name: GetRecoveryDisputeWithDetails :one
-- 获取追偿争议详情（包含索赔和订单信息）
SELECT 
  a.id, a.claim_id, a.appellant_type, a.appellant_id, a.reason, a.status, a.reviewer_id, a.review_notes, a.reviewed_at, a.compensation_amount, a.compensated_at, a.region_id, a.created_at,
    c.claim_type,
    c.claim_amount,
    c.approved_amount AS claim_approved_amount,
    c.description AS claim_description,
    c.status AS claim_status,
    c.created_at AS claim_created_at,
    o.order_no,
    o.total_amount AS order_amount,
    o.status AS order_status,
    o.created_at AS order_created_at,
    u.phone AS user_phone,
    u.full_name AS user_name
FROM recovery_disputes a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
JOIN users u ON c.user_id = u.id
WHERE a.id = $1
LIMIT 1;

-- =========================== 商户视角 ===========================

-- name: ListMerchantRecoveryDisputesForMerchant :many
-- 商户查询自己的追偿争议列表
SELECT 
  a.id, a.claim_id, a.appellant_type, a.appellant_id, a.reason, a.status, a.reviewer_id, a.review_notes, a.reviewed_at, a.compensation_amount, a.compensated_at, a.region_id, a.created_at,
    c.claim_type,
    c.claim_amount,
    c.description AS claim_description,
    o.order_no
FROM recovery_disputes a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
WHERE a.appellant_type = 'merchant'
  AND a.appellant_id = sqlc.arg('appellant_id')
  AND (sqlc.narg('status')::text IS NULL OR a.status = sqlc.narg('status')::text)
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMerchantRecoveryDisputesForMerchant :one
-- 商户追偿争议计数
SELECT COUNT(*) FROM recovery_disputes
WHERE appellant_type = 'merchant'
  AND appellant_id = sqlc.arg('appellant_id')
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text);

-- name: GetMerchantRecoveryDisputeDetail :one
-- 商户查看自己的追偿争议详情
SELECT 
  a.id, a.claim_id, a.appellant_type, a.appellant_id, a.reason, a.status, a.reviewer_id, a.review_notes, a.reviewed_at, a.compensation_amount, a.compensated_at, a.region_id, a.created_at,
    c.claim_type,
    c.claim_amount,
    c.approved_amount AS claim_approved_amount,
    c.description AS claim_description,
    o.order_no,
    o.total_amount AS order_amount,
    u.phone AS user_phone
FROM recovery_disputes a
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
  c.id, c.order_id, c.user_id, c.claim_type, c.description, c.claim_amount, c.approved_amount, c.status, c.approval_type, c.is_malicious, c.lookback_result, c.auto_approval_reason, c.rejection_reason, c.reviewer_id, c.review_notes, c.created_at, c.reviewed_at, c.paid_at, c.decision_version, c.decision_reason,
    o.order_no,
    o.total_amount AS order_amount,
    u.phone AS user_phone,
    u.full_name AS user_name,
    a.id AS recovery_dispute_id,
    a.status AS recovery_dispute_status,
    cr.status AS recovery_status
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN users u ON c.user_id = u.id
LEFT JOIN recovery_disputes a ON a.claim_id = c.id AND a.appellant_type = 'merchant'
LEFT JOIN LATERAL (
    SELECT status
    FROM claim_recoveries
    WHERE claim_id = c.id
    ORDER BY id DESC
    LIMIT 1
) cr ON TRUE
WHERE o.merchant_id = sqlc.arg('merchant_id')
  AND c.status IN ('approved', 'auto-approved')
  AND (
    sqlc.narg('bucket')::text IS NULL
    OR (
      sqlc.narg('bucket')::text = 'pending_action'
      AND (
        cr.status IN ('pending', 'overdue')
        OR (a.status = 'rejected' AND COALESCE(cr.status, '') NOT IN ('paid', 'waived'))
      )
    )
    OR (
      sqlc.narg('bucket')::text = 'disputed'
      AND (a.status = 'submitted' OR cr.status = 'disputed')
    )
    OR (
      sqlc.narg('bucket')::text = 'closed'
      AND (cr.status IN ('paid', 'waived') OR a.status = 'approved')
    )
  )
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMerchantClaimsForMerchant :one
-- 商户收到的索赔计数
SELECT COUNT(*) 
FROM claims c
JOIN orders o ON c.order_id = o.id
LEFT JOIN recovery_disputes a ON a.claim_id = c.id AND a.appellant_type = 'merchant'
LEFT JOIN LATERAL (
    SELECT status
    FROM claim_recoveries
    WHERE claim_id = c.id
    ORDER BY id DESC
    LIMIT 1
) cr ON TRUE
WHERE o.merchant_id = sqlc.arg('merchant_id')
  AND c.status IN ('approved', 'auto-approved')
  AND (
    sqlc.narg('bucket')::text IS NULL
    OR (
      sqlc.narg('bucket')::text = 'pending_action'
      AND (
        cr.status IN ('pending', 'overdue')
        OR (a.status = 'rejected' AND COALESCE(cr.status, '') NOT IN ('paid', 'waived'))
      )
    )
    OR (
      sqlc.narg('bucket')::text = 'disputed'
      AND (a.status = 'submitted' OR cr.status = 'disputed')
    )
    OR (
      sqlc.narg('bucket')::text = 'closed'
      AND (cr.status IN ('paid', 'waived') OR a.status = 'approved')
    )
  );

-- name: GetMerchantClaimDetailForMerchant :one
-- 商户查看索赔详情
SELECT 
  c.id, c.order_id, c.user_id, c.claim_type, c.description, c.claim_amount, c.approved_amount, c.status, c.approval_type, c.is_malicious, c.lookback_result, c.auto_approval_reason, c.rejection_reason, c.reviewer_id, c.review_notes, c.created_at, c.reviewed_at, c.paid_at, c.decision_version, c.decision_reason,
    o.order_no,
    o.total_amount AS order_amount,
    o.created_at AS order_created_at,
    u.phone AS user_phone,
    u.full_name AS user_name,
    a.id AS recovery_dispute_id,
    a.status AS recovery_dispute_status,
    a.reason AS recovery_dispute_reason,
    a.review_notes AS recovery_dispute_review_notes
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN users u ON c.user_id = u.id
LEFT JOIN recovery_disputes a ON a.claim_id = c.id AND a.appellant_type = 'merchant'
WHERE c.id = $1
  AND o.merchant_id = $2
LIMIT 1;

-- =========================== 骑手视角 ===========================

-- name: ListRiderRecoveryDisputes :many
-- 骑手查询自己的追偿争议列表
SELECT 
  a.id, a.claim_id, a.appellant_type, a.appellant_id, a.reason, a.status, a.reviewer_id, a.review_notes, a.reviewed_at, a.compensation_amount, a.compensated_at, a.region_id, a.created_at,
    c.claim_type,
    c.claim_amount,
    c.description AS claim_description,
    o.order_no
FROM recovery_disputes a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
WHERE a.appellant_type = 'rider'
  AND a.appellant_id = sqlc.arg('appellant_id')
  AND (sqlc.narg('status')::text IS NULL OR a.status = sqlc.narg('status')::text)
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountRiderRecoveryDisputes :one
-- 骑手追偿争议计数
SELECT COUNT(*) FROM recovery_disputes
WHERE appellant_type = 'rider'
  AND appellant_id = sqlc.arg('appellant_id')
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text);

-- name: GetRiderRecoveryDisputeDetail :one
-- 骑手查看自己的追偿争议详情
SELECT 
  a.id, a.claim_id, a.appellant_type, a.appellant_id, a.reason, a.status, a.reviewer_id, a.review_notes, a.reviewed_at, a.compensation_amount, a.compensated_at, a.region_id, a.created_at,
    c.claim_type,
    c.claim_amount,
    c.approved_amount AS claim_approved_amount,
    c.description AS claim_description,
    o.order_no,
    o.total_amount AS order_amount,
    u.phone AS user_phone
FROM recovery_disputes a
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
  c.id, c.order_id, c.user_id, c.claim_type, c.description, c.claim_amount, c.approved_amount, c.status, c.approval_type, c.is_malicious, c.lookback_result, c.auto_approval_reason, c.rejection_reason, c.reviewer_id, c.review_notes, c.created_at, c.reviewed_at, c.paid_at, c.decision_version, c.decision_reason,
    o.order_no,
    o.total_amount AS order_amount,
    u.phone AS user_phone,
    u.full_name AS user_name,
    a.id AS recovery_dispute_id,
  a.status AS recovery_dispute_status,
  cr.status AS recovery_status
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN deliveries d ON d.order_id = o.id
JOIN users u ON c.user_id = u.id
LEFT JOIN recovery_disputes a ON a.claim_id = c.id AND a.appellant_type = 'rider'
LEFT JOIN LATERAL (
  SELECT status
  FROM claim_recoveries
  WHERE claim_id = c.id
  ORDER BY id DESC
  LIMIT 1
) cr ON TRUE
WHERE d.rider_id = $1
  AND c.status IN ('approved', 'auto-approved')
  AND (
    sqlc.narg('bucket')::text IS NULL
    OR (
      sqlc.narg('bucket')::text = 'pending_action'
      AND (
        cr.status IN ('pending', 'overdue')
        OR (a.status = 'rejected' AND COALESCE(cr.status, '') NOT IN ('paid', 'waived'))
      )
    )
    OR (
      sqlc.narg('bucket')::text = 'disputed'
      AND (a.status = 'submitted' OR cr.status = 'disputed')
    )
    OR (
      sqlc.narg('bucket')::text = 'closed'
      AND (cr.status IN ('paid', 'waived') OR a.status = 'approved')
    )
  )
ORDER BY c.created_at DESC, c.id DESC
LIMIT $2 OFFSET $3;

-- name: CountRiderClaimsForRider :one
-- 骑手收到的索赔计数
SELECT COUNT(*) 
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN deliveries d ON d.order_id = o.id
LEFT JOIN recovery_disputes a ON a.claim_id = c.id AND a.appellant_type = 'rider'
LEFT JOIN LATERAL (
  SELECT status
  FROM claim_recoveries
  WHERE claim_id = c.id
  ORDER BY id DESC
  LIMIT 1
) cr ON TRUE
WHERE d.rider_id = $1
  AND c.status IN ('approved', 'auto-approved')
  AND (
    sqlc.narg('bucket')::text IS NULL
    OR (
      sqlc.narg('bucket')::text = 'pending_action'
      AND (
        cr.status IN ('pending', 'overdue')
        OR (a.status = 'rejected' AND COALESCE(cr.status, '') NOT IN ('paid', 'waived'))
      )
    )
    OR (
      sqlc.narg('bucket')::text = 'disputed'
      AND (a.status = 'submitted' OR cr.status = 'disputed')
    )
    OR (
      sqlc.narg('bucket')::text = 'closed'
      AND (cr.status IN ('paid', 'waived') OR a.status = 'approved')
    )
  );

-- name: GetRiderClaimDetailForRider :one
-- 骑手查看索赔详情
SELECT 
  c.id, c.order_id, c.user_id, c.claim_type, c.description, c.claim_amount, c.approved_amount, c.status, c.approval_type, c.is_malicious, c.lookback_result, c.auto_approval_reason, c.rejection_reason, c.reviewer_id, c.review_notes, c.created_at, c.reviewed_at, c.paid_at, c.decision_version, c.decision_reason,
    o.order_no,
    o.total_amount AS order_amount,
    o.created_at AS order_created_at,
    u.phone AS user_phone,
    u.full_name AS user_name,
    a.id AS recovery_dispute_id,
    a.status AS recovery_dispute_status,
    a.reason AS recovery_dispute_reason,
  a.review_notes AS recovery_dispute_review_notes,
  cr.status AS recovery_status
FROM claims c
JOIN orders o ON c.order_id = o.id
JOIN deliveries d ON d.order_id = o.id
JOIN users u ON c.user_id = u.id
LEFT JOIN recovery_disputes a ON a.claim_id = c.id AND a.appellant_type = 'rider'
LEFT JOIN LATERAL (
  SELECT status
  FROM claim_recoveries
  WHERE claim_id = c.id
  ORDER BY id DESC
  LIMIT 1
) cr ON TRUE
WHERE c.id = $1
  AND d.rider_id = $2
LIMIT 1;

-- =========================== 运营商视角 ===========================

-- name: ListOperatorRecoveryDisputes :many
-- 运营商查询区域内的追偿争议列表
SELECT 
  a.id, a.claim_id, a.appellant_type, a.appellant_id, a.reason, a.status, a.reviewer_id, a.review_notes, a.reviewed_at, a.compensation_amount, a.compensated_at, a.region_id, a.created_at,
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
FROM recovery_disputes a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
JOIN merchants m ON o.merchant_id = m.id
WHERE a.region_id = $1
  AND (NULLIF($2::TEXT, '') IS NULL OR a.status = $2)
ORDER BY 
    CASE WHEN a.status = 'submitted' THEN 0 ELSE 1 END,
    a.created_at DESC,
    a.id DESC
LIMIT $3 OFFSET $4;

-- name: CountOperatorRecoveryDisputes :one
-- 运营商追偿争议计数
SELECT COUNT(*) FROM recovery_disputes
WHERE region_id = $1
  AND (NULLIF($2::TEXT, '') IS NULL OR status = $2);

-- name: GetOperatorRecoveryDisputeDetail :one
-- 运营商查看追偿争议详情
SELECT 
  a.id, a.claim_id, a.appellant_type, a.appellant_id, a.reason, a.status, a.reviewer_id, a.review_notes, a.reviewed_at, a.compensation_amount, a.compensated_at, a.region_id, a.created_at,
    c.claim_type,
    c.claim_amount,
    c.approved_amount AS claim_approved_amount,
    c.description AS claim_description,
    c.status AS claim_status,
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
FROM recovery_disputes a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
JOIN merchants m ON o.merchant_id = m.id
JOIN users u ON c.user_id = u.id
LEFT JOIN deliveries d ON d.order_id = o.id
WHERE a.id = $1
  AND a.region_id = $2
LIMIT 1;

-- name: ReviewRecoveryDispute :one
-- 审核追偿争议
UPDATE recovery_disputes
SET status = @status,
    reviewer_id = @reviewer_id,
    review_notes = @review_notes,
    reviewed_at = NOW(),
    compensation_amount = CASE WHEN @status = 'approved' THEN sqlc.narg(compensation_amount)::bigint ELSE NULL END,
    compensated_at = NULL
WHERE id = @id
  AND status = 'submitted'
RETURNING *;

-- name: MarkRecoveryDisputeCompensated :exec
UPDATE recovery_disputes
SET compensated_at = COALESCE(compensated_at, $2)
WHERE id = $1;

-- =========================== 通用查询 ===========================

-- name: CheckRecoveryDisputeExists :one
-- 检查索赔是否已有指定争议方类型的追偿争议
SELECT EXISTS (
  SELECT 1 FROM recovery_disputes WHERE claim_id = $1 AND appellant_type = $2
) AS exists;

-- name: GetRecoveryDisputeForPostProcess :one
-- 获取追偿争议审核后处理所需信息
SELECT 
    a.id AS recovery_dispute_id,
    a.claim_id,
    a.appellant_type,
    a.appellant_id,
    a.status AS recovery_dispute_status,
    a.compensation_amount,
    c.user_id AS claimant_user_id,
    c.claim_type,
    c.claim_amount,
    c.approved_amount AS claim_approved_amount,
    c.order_id,
    o.order_no,
    o.merchant_id,
    d.rider_id
FROM recovery_disputes a
JOIN claims c ON a.claim_id = c.id
JOIN orders o ON c.order_id = o.id
LEFT JOIN deliveries d ON d.order_id = o.id
WHERE a.id = $1;
