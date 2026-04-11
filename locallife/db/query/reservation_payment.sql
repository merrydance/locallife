-- name: CreateReservationPayment :one
INSERT INTO reservation_payments (
    reservation_id,
    payment_order_id,
    amount,
    type
) VALUES (
    $1, $2, $3, $4
) ON CONFLICT (payment_order_id) DO NOTHING
RETURNING *;

-- name: GetReservationPaymentByPaymentOrderID :one
SELECT * FROM reservation_payments
WHERE payment_order_id = $1 LIMIT 1;
