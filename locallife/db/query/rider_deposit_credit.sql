-- name: CreateRiderDepositCredit :one
INSERT INTO rider_deposit_credits (
    rider_id,
    payment_order_id,
    original_amount,
    refundable_amount,
    refunded_amount,
    status,
    paid_at,
    refundable_until,
    last_reminded_at,
    expired_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: GetRiderDepositCredit :one
SELECT id, rider_id, payment_order_id, original_amount, refundable_amount, refunded_amount, status, paid_at, refundable_until, last_reminded_at, expired_at, created_at, updated_at FROM rider_deposit_credits
WHERE id = $1
LIMIT 1;

-- name: GetRiderDepositCreditByPaymentOrderID :one
SELECT id, rider_id, payment_order_id, original_amount, refundable_amount, refunded_amount, status, paid_at, refundable_until, last_reminded_at, expired_at, created_at, updated_at FROM rider_deposit_credits
WHERE payment_order_id = $1
LIMIT 1;

-- name: GetRiderDepositCreditForUpdate :one
SELECT id, rider_id, payment_order_id, original_amount, refundable_amount, refunded_amount, status, paid_at, refundable_until, last_reminded_at, expired_at, created_at, updated_at FROM rider_deposit_credits
WHERE id = $1
LIMIT 1
FOR UPDATE;

-- name: ListActiveRiderDepositCreditsByRiderID :many
SELECT id, rider_id, payment_order_id, original_amount, refundable_amount, refunded_amount, status, paid_at, refundable_until, last_reminded_at, expired_at, created_at, updated_at FROM rider_deposit_credits
WHERE rider_id = $1
  AND status IN ('active', 'partially_refunded')
  AND refundable_amount > 0
ORDER BY refundable_until ASC, id ASC;

-- name: ConsumeRiderDepositCredit :one
UPDATE rider_deposit_credits
SET refundable_amount = refundable_amount - $2,
    refunded_amount = refunded_amount + $2,
    status = CASE
        WHEN refundable_amount - $2 = 0 THEN 'fully_refunded'
        ELSE 'partially_refunded'
    END,
    updated_at = now()
WHERE id = $1
  AND refundable_amount >= $2
RETURNING *;

-- name: RestoreRiderDepositCreditByPaymentOrderID :one
UPDATE rider_deposit_credits
SET refundable_amount = refundable_amount + $2,
    refunded_amount = refunded_amount - $2,
    status = CASE
        WHEN refunded_amount - $2 = 0 THEN 'active'
        ELSE 'partially_refunded'
    END,
    expired_at = NULL,
    updated_at = now()
WHERE payment_order_id = $1
  AND refunded_amount >= $2
RETURNING *;

-- name: MarkRiderDepositCreditExpired :one
UPDATE rider_deposit_credits
SET status = 'expired',
    expired_at = COALESCE($2, now()),
    updated_at = now()
WHERE id = $1
  AND status IN ('active', 'partially_refunded', 'legacy')
RETURNING *;

-- name: TouchRiderDepositCreditReminder :one
UPDATE rider_deposit_credits
SET last_reminded_at = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListRiderDepositCreditsForReminderWindow :many
SELECT id, rider_id, payment_order_id, original_amount, refundable_amount, refunded_amount, status, paid_at, refundable_until, last_reminded_at, expired_at, created_at, updated_at FROM rider_deposit_credits
WHERE status IN ('active', 'partially_refunded')
  AND refundable_amount > 0
  AND refundable_until > $1
  AND refundable_until <= $2
ORDER BY refundable_until ASC, id ASC
LIMIT $3;

-- name: ListExpiredRiderDepositCredits :many
SELECT id, rider_id, payment_order_id, original_amount, refundable_amount, refunded_amount, status, paid_at, refundable_until, last_reminded_at, expired_at, created_at, updated_at FROM rider_deposit_credits
WHERE status IN ('active', 'partially_refunded', 'legacy')
  AND refundable_until <= $1
ORDER BY refundable_until ASC, id ASC
LIMIT $2;