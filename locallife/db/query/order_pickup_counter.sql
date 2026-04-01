-- name: AllocateDailyPickupSequence :one
INSERT INTO order_pickup_counters (
    merchant_id,
    pickup_date,
    last_sequence
) VALUES (
    $1,
    $2,
    1
)
ON CONFLICT (merchant_id, pickup_date)
DO UPDATE SET
    last_sequence = order_pickup_counters.last_sequence + 1,
    updated_at = now()
RETURNING last_sequence;