-- name: CreateDiningSession :one
INSERT INTO dining_sessions (
    merchant_id,
    table_id,
    reservation_id,
    user_id,
    active_order_id,
    status
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetDiningSession :one
SELECT id, merchant_id, table_id, reservation_id, user_id, active_order_id, status, opened_at, closed_at, created_at, updated_at FROM dining_sessions WHERE id = $1;

-- name: GetActiveDiningSessionByTable :one
SELECT id, merchant_id, table_id, reservation_id, user_id, active_order_id, status, opened_at, closed_at, created_at, updated_at FROM dining_sessions
WHERE table_id = $1 AND status = 'open'
LIMIT 1;

-- name: GetActiveDiningSessionByReservation :one
SELECT id, merchant_id, table_id, reservation_id, user_id, active_order_id, status, opened_at, closed_at, created_at, updated_at FROM dining_sessions
WHERE reservation_id = $1 AND status = 'open'
LIMIT 1;

-- name: UpdateDiningSessionActiveOrder :one
UPDATE dining_sessions
SET active_order_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CloseDiningSession :one
UPDATE dining_sessions
SET status = 'closed',
    closed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListDiningSessionsByUser :many
SELECT id, merchant_id, table_id, reservation_id, user_id, active_order_id, status, opened_at, closed_at, created_at, updated_at FROM dining_sessions
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: ListOpenDiningSessionsBefore :many
SELECT id, merchant_id, table_id, reservation_id, user_id, active_order_id, status, opened_at, closed_at, created_at, updated_at FROM dining_sessions
WHERE status = sqlc.arg('status')
  AND opened_at < sqlc.arg('opened_at')
ORDER BY opened_at ASC, id ASC
LIMIT sqlc.arg('limit');

-- name: ListPaidOpenDineInSessionsForCheckoutRecovery :many
SELECT ds.id, ds.merchant_id, ds.table_id, ds.reservation_id, ds.user_id, ds.active_order_id, ds.status, ds.opened_at, ds.closed_at, ds.created_at, ds.updated_at
FROM dining_sessions ds
INNER JOIN orders o ON o.id = ds.active_order_id
WHERE ds.status = 'open'
  AND ds.opened_at < sqlc.arg('opened_before')
  AND o.status = 'paid'
  AND o.order_type = 'dine_in'
  AND o.merchant_id = ds.merchant_id
  AND o.user_id = ds.user_id
ORDER BY ds.opened_at ASC, ds.id ASC
LIMIT sqlc.arg('limit');
