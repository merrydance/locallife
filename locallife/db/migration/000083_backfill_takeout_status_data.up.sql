-- Backfill new takeout status timestamps and defaults

-- If a delivery has delivered_at, set orders.rider_delivered_at accordingly
UPDATE orders o
SET rider_delivered_at = d.delivered_at
FROM deliveries d
WHERE o.id = d.order_id
  AND d.delivered_at IS NOT NULL
  AND o.rider_delivered_at IS NULL;

-- If an order is completed (or already in rider/user delivered) and missing user_delivered_at, set it from completed_at
UPDATE orders
SET user_delivered_at = completed_at
WHERE status IN ('completed', 'user_delivered', 'rider_delivered')
  AND completed_at IS NOT NULL
  AND user_delivered_at IS NULL;

-- deliveries rider_delivered_at from delivered_at
UPDATE deliveries
SET rider_delivered_at = delivered_at
WHERE delivered_at IS NOT NULL
  AND rider_delivered_at IS NULL;
