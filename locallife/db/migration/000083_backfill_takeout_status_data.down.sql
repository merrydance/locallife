-- Revert backfill changes (best-effort)

-- Clear orders.rider_delivered_at when it matches deliveries.delivered_at
UPDATE orders o
SET rider_delivered_at = NULL
FROM deliveries d
WHERE o.id = d.order_id
  AND o.rider_delivered_at IS NOT NULL
  AND d.delivered_at IS NOT NULL
  AND o.rider_delivered_at = d.delivered_at;

-- Clear user_delivered_at set from completed_at
UPDATE orders
SET user_delivered_at = NULL
WHERE user_delivered_at IS NOT NULL
  AND completed_at IS NOT NULL
  AND user_delivered_at = completed_at;

-- Clear deliveries rider_delivered_at when it equals delivered_at
UPDATE deliveries
SET rider_delivered_at = NULL
WHERE rider_delivered_at IS NOT NULL
  AND delivered_at IS NOT NULL
  AND rider_delivered_at = delivered_at;
