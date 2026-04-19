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
