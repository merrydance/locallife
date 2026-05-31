-- Claim recovery queries

-- name: CreateClaimRecovery :one
INSERT INTO claim_recoveries (
  claim_id,
  order_id,
  decision_id,
  responsible_party,
  recovery_target,
  recovery_amount,
  status,
  due_at,
  decision_snapshot,
  recovery_basis
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: CreateClaimRecoveryEvent :one
INSERT INTO claim_recovery_events (
  recovery_id,
  decision_id,
  event_type,
  payload
) VALUES (
  $1, sqlc.narg('decision_id'), $2, $3
)
RETURNING *;

-- name: ListClaimRecoveryEventsByRecovery :many
SELECT id, recovery_id, decision_id, event_type, payload, created_at
FROM claim_recovery_events
WHERE recovery_id = $1
ORDER BY created_at ASC;

-- name: GetClaimRecoveryByClaimID :one
SELECT id, claim_id, order_id, responsible_party, recovery_target, recovery_amount, status, due_at, decision_snapshot, created_at, updated_at, decision_id, recovery_basis
FROM claim_recoveries
WHERE claim_id = $1
ORDER BY id DESC
LIMIT 1;

-- name: GetClaimRecoveryByClaimIDAndTarget :one
SELECT id, claim_id, order_id, responsible_party, recovery_target, recovery_amount, status, due_at, decision_snapshot, created_at, updated_at, decision_id, recovery_basis
FROM claim_recoveries
WHERE claim_id = sqlc.arg('claim_id')
  AND recovery_target = sqlc.arg('recovery_target')
ORDER BY id DESC
LIMIT 1;

-- name: GetClaimRecoveryByID :one
SELECT id, claim_id, order_id, responsible_party, recovery_target, recovery_amount, status, due_at, decision_snapshot, created_at, updated_at, decision_id, recovery_basis
FROM claim_recoveries
WHERE id = $1
LIMIT 1;

-- name: GetClaimRecoveryContextByClaimID :one
SELECT
  cr.id,
  cr.claim_id,
  cr.order_id,
  cr.responsible_party,
  cr.recovery_target,
  cr.recovery_amount,
  cr.status,
  cr.due_at,
  cr.decision_snapshot,
  cr.created_at,
  cr.updated_at,
  cr.decision_id,
  cr.recovery_basis,
  o.merchant_id,
  m.region_id,
  d.rider_id,
  c.paid_at,
  c.created_at AS claim_created_at
FROM claim_recoveries cr
JOIN claims c ON c.id = cr.claim_id
JOIN orders o ON o.id = cr.order_id
JOIN merchants m ON m.id = o.merchant_id
LEFT JOIN deliveries d ON d.order_id = cr.order_id
WHERE cr.claim_id = $1
ORDER BY cr.id DESC
LIMIT 1;

-- name: GetClaimRecoveryContextByClaimIDAndTarget :one
SELECT
  cr.id,
  cr.claim_id,
  cr.order_id,
  cr.responsible_party,
  cr.recovery_target,
  cr.recovery_amount,
  cr.status,
  cr.due_at,
  cr.decision_snapshot,
  cr.created_at,
  cr.updated_at,
  cr.decision_id,
  cr.recovery_basis,
  o.merchant_id,
  m.region_id,
  d.rider_id,
  c.paid_at,
  c.created_at AS claim_created_at
FROM claim_recoveries cr
JOIN claims c ON c.id = cr.claim_id
JOIN orders o ON o.id = cr.order_id
JOIN merchants m ON m.id = o.merchant_id
LEFT JOIN deliveries d ON d.order_id = cr.order_id
WHERE cr.claim_id = sqlc.arg('claim_id')
  AND cr.recovery_target = sqlc.arg('recovery_target')
ORDER BY cr.id DESC
LIMIT 1;

-- name: GetClaimRecoveryContextByID :one
SELECT
  cr.id,
  cr.claim_id,
  cr.order_id,
  cr.responsible_party,
  cr.recovery_target,
  cr.recovery_amount,
  cr.status,
  cr.due_at,
  cr.decision_snapshot,
  cr.created_at,
  cr.updated_at,
  cr.decision_id,
  cr.recovery_basis,
  o.merchant_id,
  m.region_id,
  d.rider_id,
  c.paid_at,
  c.created_at AS claim_created_at
FROM claim_recoveries cr
JOIN claims c ON c.id = cr.claim_id
JOIN orders o ON o.id = cr.order_id
JOIN merchants m ON m.id = o.merchant_id
LEFT JOIN deliveries d ON d.order_id = cr.order_id
WHERE cr.id = $1
LIMIT 1;

-- name: HasBlockingClaimRecoveryForMerchant :one
SELECT EXISTS (
  SELECT 1
  FROM claim_recoveries cr
  JOIN orders o ON o.id = cr.order_id
  WHERE o.merchant_id = $1
    AND cr.recovery_target = 'merchant'
    AND (
      cr.status = 'overdue'
      OR (cr.status = 'disputed' AND cr.due_at <= NOW())
    )
) AS exists;

-- name: HasBlockingClaimRecoveryForRider :one
SELECT EXISTS (
  SELECT 1
  FROM claim_recoveries cr
  JOIN deliveries d ON d.order_id = cr.order_id
  WHERE d.rider_id = $1
    AND cr.recovery_target = 'rider'
    AND (
      cr.status = 'overdue'
      OR (cr.status = 'disputed' AND cr.due_at <= NOW())
    )
) AS exists;

-- name: ListDueClaimRecoveries :many
SELECT id, claim_id, order_id, responsible_party, recovery_target, recovery_amount, status, due_at, decision_snapshot, created_at, updated_at, decision_id, recovery_basis
FROM claim_recoveries
WHERE status = 'pending'
  AND due_at <= $1
ORDER BY due_at ASC
LIMIT $2;

-- name: MarkClaimRecoveryOverdue :one
UPDATE claim_recoveries
SET status = 'overdue',
    updated_at = NOW()
WHERE id = $1
  AND status = 'pending'
RETURNING *;

-- name: MarkClaimRecoveryPaid :one
UPDATE claim_recoveries
SET status = 'paid',
    updated_at = NOW()
WHERE id = $1
  AND status IN ('pending', 'overdue')
RETURNING *;

-- name: MarkClaimRecoveryWaived :one
UPDATE claim_recoveries
SET status = 'waived',
    updated_at = NOW()
WHERE id = $1
  AND status IN ('pending', 'overdue', 'disputed')
RETURNING *;

-- name: MarkClaimRecoveryDisputed :one
UPDATE claim_recoveries
SET status = 'disputed',
    updated_at = NOW()
WHERE id = $1
  AND status IN ('pending', 'overdue')
RETURNING *;

-- name: MarkClaimRecoveryPending :one
UPDATE claim_recoveries
SET status = 'pending',
    updated_at = NOW()
WHERE id = $1
  AND status = 'disputed'
RETURNING *;

-- name: ResumeClaimRecoveryAfterDispute :one
UPDATE claim_recoveries
SET status = CASE WHEN due_at <= NOW() THEN 'overdue' ELSE 'pending' END,
    updated_at = NOW()
WHERE id = $1
  AND status = 'disputed'
RETURNING *;
