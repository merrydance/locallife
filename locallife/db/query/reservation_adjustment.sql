-- name: CreateReservationAdjustment :one
INSERT INTO reservation_adjustments (
    reservation_id,
    user_id,
    merchant_id,
    direction,
    status,
    current_total,
    target_total,
    delta_amount,
    payment_order_id,
    failure_reason,
    close_reason
) VALUES (
    sqlc.arg(reservation_id),
    sqlc.arg(user_id),
    sqlc.arg(merchant_id),
    sqlc.arg(direction),
    sqlc.arg(status),
    sqlc.arg(current_total),
    sqlc.arg(target_total),
    sqlc.arg(delta_amount),
    sqlc.narg(payment_order_id),
    sqlc.narg(failure_reason),
    sqlc.narg(close_reason)
) RETURNING *;

-- name: CreateReservationAdjustmentItem :one
INSERT INTO reservation_adjustment_items (
    adjustment_id,
    dish_id,
    combo_id,
    quantity,
    unit_price,
    total_price,
    position
) VALUES (
    sqlc.arg(adjustment_id),
    sqlc.narg(dish_id),
    sqlc.narg(combo_id),
    sqlc.arg(quantity),
    sqlc.arg(unit_price),
    sqlc.arg(total_price),
    sqlc.arg(position)
) RETURNING *;

-- name: CreateReservationAdjustmentInventoryHold :one
INSERT INTO reservation_adjustment_inventory_holds (
    adjustment_id,
    merchant_id,
    dish_id,
    reservation_date,
    quantity,
    expires_at
) VALUES (
    sqlc.arg(adjustment_id),
    sqlc.arg(merchant_id),
    sqlc.arg(dish_id),
    sqlc.arg(reservation_date),
    sqlc.arg(quantity),
    sqlc.arg(expires_at)
) RETURNING *;

-- name: GetReservationAdjustment :one
SELECT id, reservation_id, user_id, merchant_id, direction, status, current_total, target_total, delta_amount, payment_order_id, failure_reason, close_reason, applied_at, closed_at, created_at, updated_at FROM reservation_adjustments
WHERE id = $1 LIMIT 1;

-- name: GetReservationAdjustmentForUpdate :one
SELECT id, reservation_id, user_id, merchant_id, direction, status, current_total, target_total, delta_amount, payment_order_id, failure_reason, close_reason, applied_at, closed_at, created_at, updated_at FROM reservation_adjustments
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetReservationAdjustmentByPaymentOrderForUpdate :one
SELECT id, reservation_id, user_id, merchant_id, direction, status, current_total, target_total, delta_amount, payment_order_id, failure_reason, close_reason, applied_at, closed_at, created_at, updated_at FROM reservation_adjustments
WHERE payment_order_id = $1 LIMIT 1
FOR UPDATE;

-- name: GetActiveReservationAdjustmentByReservation :one
SELECT id, reservation_id, user_id, merchant_id, direction, status, current_total, target_total, delta_amount, payment_order_id, failure_reason, close_reason, applied_at, closed_at, created_at, updated_at FROM reservation_adjustments
WHERE reservation_id = $1
  AND status IN ('creating_payment', 'pending_payment', 'applying')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListReservationAdjustmentItems :many
SELECT id, adjustment_id, dish_id, combo_id, quantity, unit_price, total_price, position, created_at FROM reservation_adjustment_items
WHERE adjustment_id = $1
ORDER BY position, id;

-- name: ListReservationAdjustmentInventoryHoldsForUpdate :many
SELECT id, adjustment_id, merchant_id, dish_id, reservation_date, quantity, status, expires_at, created_at, updated_at FROM reservation_adjustment_inventory_holds
WHERE adjustment_id = $1
ORDER BY id
FOR UPDATE;

-- name: ListActiveReservationAdjustments :many
SELECT id, reservation_id, user_id, merchant_id, direction, status, current_total, target_total, delta_amount, payment_order_id, failure_reason, close_reason, applied_at, closed_at, created_at, updated_at FROM reservation_adjustments
WHERE status IN ('creating_payment', 'pending_payment', 'applying')
ORDER BY created_at, id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: MarkReservationAdjustmentPendingPayment :one
UPDATE reservation_adjustments
SET status = 'pending_payment',
    updated_at = now()
WHERE id = $1
  AND status = 'creating_payment'
RETURNING *;

-- name: MarkReservationAdjustmentApplying :one
UPDATE reservation_adjustments
SET status = 'applying',
    updated_at = now()
WHERE id = $1
  AND status IN ('creating_payment', 'pending_payment', 'applying')
RETURNING *;

-- name: MarkReservationAdjustmentApplied :one
UPDATE reservation_adjustments
SET status = 'applied',
    applied_at = now(),
    updated_at = now()
WHERE id = $1
  AND status IN ('creating_payment', 'pending_payment', 'applying')
RETURNING *;

-- name: MarkReservationAdjustmentClosed :one
UPDATE reservation_adjustments
SET status = 'closed',
    close_reason = sqlc.narg(close_reason),
    closed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status IN ('creating_payment', 'pending_payment', 'applying')
RETURNING *;

-- name: MarkReservationAdjustmentFailed :one
UPDATE reservation_adjustments
SET status = 'failed',
    failure_reason = sqlc.narg(failure_reason),
    closed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status IN ('creating_payment', 'pending_payment', 'applying')
RETURNING *;

-- name: MarkReservationAdjustmentExpired :one
UPDATE reservation_adjustments
SET status = 'expired',
    close_reason = sqlc.narg(close_reason),
    closed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status IN ('creating_payment', 'pending_payment', 'applying')
RETURNING *;

-- name: LinkReservationAdjustmentPaymentOrder :one
UPDATE reservation_adjustments
SET payment_order_id = sqlc.arg(payment_order_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status = 'creating_payment'
  AND payment_order_id IS NULL
RETURNING *;

-- name: MarkReservationAdjustmentHoldConverted :one
UPDATE reservation_adjustment_inventory_holds
SET status = 'converted',
    updated_at = now()
WHERE id = $1
  AND status = 'held'
RETURNING *;

-- name: MarkReservationAdjustmentHoldReleased :one
UPDATE reservation_adjustment_inventory_holds
SET status = 'released',
    updated_at = now()
WHERE id = $1
  AND status = 'held'
RETURNING *;
