-- name: CreateTableReservation :one
INSERT INTO table_reservations (
    table_id,
    user_id,
    merchant_id,
    reservation_date,
    reservation_time,
    guest_count,
    contact_name,
    contact_phone,
    payment_mode,
    deposit_amount,
    prepaid_amount,
    refund_deadline,
    payment_deadline,
    notes,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
) RETURNING *;

-- name: GetTableReservation :one
SELECT * FROM table_reservations
WHERE id = $1 LIMIT 1;

-- name: GetTableReservationForUpdate :one
SELECT * FROM table_reservations
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetTableReservationWithTable :one
SELECT 
    tr.*,
    t.table_no,
    t.table_type,
    t.capacity
FROM table_reservations tr
INNER JOIN tables t ON tr.table_id = t.id
WHERE tr.id = $1;

-- name: ListReservationsByUser :many
SELECT 
    tr.*,
    t.table_no,
    t.table_type
FROM table_reservations tr
INNER JOIN tables t ON tr.table_id = t.id
WHERE tr.user_id = $1
ORDER BY tr.reservation_date DESC, tr.reservation_time DESC
LIMIT $2 OFFSET $3;

-- name: ListReservationsByMerchant :many
SELECT 
    tr.*,
    t.table_no,
    t.table_type
FROM table_reservations tr
INNER JOIN tables t ON tr.table_id = t.id
WHERE tr.merchant_id = $1
ORDER BY tr.reservation_date DESC, tr.reservation_time DESC
LIMIT $2 OFFSET $3;

-- name: ListReservationsByMerchantAndDate :many
SELECT 
    tr.*,
    t.table_no,
    t.table_type
FROM table_reservations tr
INNER JOIN tables t ON tr.table_id = t.id
WHERE tr.merchant_id = $1 
  AND tr.reservation_date = $2
ORDER BY tr.reservation_time, t.table_no;

-- name: ListReservationsByMerchantAndStatus :many
SELECT 
    tr.*,
    t.table_no,
    t.table_type
FROM table_reservations tr
INNER JOIN tables t ON tr.table_id = t.id
WHERE tr.merchant_id = $1 
  AND tr.status = $2
ORDER BY tr.reservation_date, tr.reservation_time
LIMIT $3 OFFSET $4;

-- name: ListReservationsByTable :many
SELECT * FROM table_reservations
WHERE table_id = $1
ORDER BY reservation_date DESC, reservation_time DESC
LIMIT $2 OFFSET $3;

-- name: ListReservationsByTableAndDate :many
SELECT * FROM table_reservations
WHERE table_id = $1 
  AND reservation_date = $2
ORDER BY reservation_time;

-- name: CheckTableAvailability :one
-- Check if table has any active reservation for given date and time
SELECT COUNT(*) FROM table_reservations
WHERE table_id = $1 
  AND reservation_date = $2
  AND reservation_time = $3
  AND status IN ('pending', 'paid', 'confirmed');

-- name: UpdateReservationStatus :one
UPDATE table_reservations
SET status = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateReservationToPaid :one
UPDATE table_reservations
SET status = 'paid',
    paid_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateReservationToConfirmed :one
UPDATE table_reservations
SET status = 'confirmed',
    confirmed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateReservationToCompleted :one
UPDATE table_reservations
SET status = 'completed',
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateReservationToCancelled :one
UPDATE table_reservations
SET status = 'cancelled',
    cancelled_at = now(),
    cancel_reason = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateReservationToExpired :one
UPDATE table_reservations
SET status = 'expired',
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateReservationToNoShow :one
UPDATE table_reservations
SET status = 'no_show',
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListExpiredPendingReservations :many
-- Find pending reservations that have passed their payment deadline
SELECT * FROM table_reservations
WHERE status = 'pending'
  AND payment_deadline < now()
ORDER BY payment_deadline;

-- name: ListPendingReservationsNearDeadline :many
-- Find pending reservations within N minutes of payment deadline (for reminder notifications)
SELECT * FROM table_reservations
WHERE status = 'pending'
  AND payment_deadline > now()
  AND payment_deadline < now() + sqlc.arg(minutes_before)::interval
ORDER BY payment_deadline;

-- name: CountReservationsByUserAndStatus :one
SELECT COUNT(*) FROM table_reservations
WHERE user_id = $1 AND status = $2;

-- name: CountReservationsByMerchant :one
SELECT COUNT(*) FROM table_reservations
WHERE merchant_id = $1;

-- name: CountReservationsByMerchantAndStatus :one
SELECT COUNT(*) FROM table_reservations
WHERE merchant_id = $1 AND status = $2;

-- name: CountReservationsByMerchantAndDate :one
SELECT COUNT(*) FROM table_reservations
WHERE merchant_id = $1 AND reservation_date = $2;

-- name: GetReservationStats :one
-- Get reservation statistics for a merchant
SELECT 
    COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
    COUNT(*) FILTER (WHERE status = 'paid') as paid_count,
    COUNT(*) FILTER (WHERE status = 'confirmed') as confirmed_count,
    COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
    COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled_count,
    COUNT(*) FILTER (WHERE status = 'expired') as expired_count,
    COUNT(*) FILTER (WHERE status = 'no_show') as no_show_count
FROM table_reservations
WHERE merchant_id = $1;

-- name: GetReservationStatsByDateRange :one
-- Get reservation statistics for a merchant within date range
SELECT 
    COUNT(*) as total_count,
    COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
    COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled_count,
    COALESCE(SUM(deposit_amount) FILTER (WHERE status = 'completed'), 0) as total_deposit,
    COALESCE(SUM(prepaid_amount) FILTER (WHERE status = 'completed'), 0) as total_prepaid
FROM table_reservations
WHERE merchant_id = $1 
  AND reservation_date >= $2 
  AND reservation_date <= $3;

-- name: CancelMerchantFutureReservations :execrows
-- 商户熔断时自动取消所有未来的预订
UPDATE table_reservations
SET 
    status = 'cancelled',
    cancel_reason = $2,
    updated_at = now()
WHERE merchant_id = $1 
  AND reservation_date >= CURRENT_DATE
  AND status IN ('pending', 'paid', 'confirmed');

-- name: ListMerchantFutureReservationsForRefund :many
-- 获取商户未来预订列表（用于熔断后退款处理）
SELECT * FROM table_reservations
WHERE merchant_id = $1 
  AND reservation_date >= CURRENT_DATE
  AND status IN ('pending', 'paid', 'confirmed')
  AND (deposit_amount > 0 OR prepaid_amount > 0)
ORDER BY reservation_date, reservation_time;

-- name: CountFutureReservationsByTable :one
-- 检查某桌台是否有未来的有效预定（用于删除桌台前检查）
SELECT COUNT(*) FROM table_reservations
WHERE table_id = $1 
  AND reservation_date >= CURRENT_DATE
  AND status IN ('pending', 'paid', 'confirmed');
