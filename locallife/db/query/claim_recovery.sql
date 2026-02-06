-- Claim recovery queries

-- name: CreateClaimRecovery :one
INSERT INTO claim_recoveries (
  claim_id,
  order_id,
  responsible_party,
  recovery_target,
  recovery_amount,
  status,
  due_at,
  decision_snapshot
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetClaimRecoveryByClaimID :one
SELECT *
FROM claim_recoveries
WHERE claim_id = $1
ORDER BY id DESC
LIMIT 1;

-- name: ListDueClaimRecoveries :many
SELECT *
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
  AND status IN ('pending', 'overdue', 'appealed')
RETURNING *;

-- name: MarkClaimRecoveryAppealed :one
UPDATE claim_recoveries
SET status = 'appealed',
    updated_at = NOW()
WHERE id = $1
  AND status IN ('pending', 'overdue')
RETURNING *;

-- name: MarkClaimRecoveryPending :one
UPDATE claim_recoveries
SET status = 'pending',
    updated_at = NOW()
WHERE id = $1
  AND status = 'appealed'
RETURNING *;
