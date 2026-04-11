-- Revert fine-grained takeout status fields

-- Restore original status constraint
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (
  status IN ('pending', 'paid', 'preparing', 'ready', 'delivering', 'completed', 'cancelled')
);

-- Drop newly added columns
ALTER TABLE orders
  DROP COLUMN IF EXISTS auto_user_delivered_at,
  DROP COLUMN IF EXISTS user_delivered_at,
  DROP COLUMN IF EXISTS rider_delivered_at,
  DROP COLUMN IF EXISTS picked_at,
  DROP COLUMN IF EXISTS courier_accept_at,
  DROP COLUMN IF EXISTS ready_at,
  DROP COLUMN IF EXISTS prep_start_at,
  DROP COLUMN IF EXISTS overtime,
  DROP COLUMN IF EXISTS claim_channel,
  DROP COLUMN IF EXISTS exception_state,
  DROP COLUMN IF EXISTS badges,
  DROP COLUMN IF EXISTS status_hint,
  DROP COLUMN IF EXISTS flow_id,
  DROP COLUMN IF EXISTS dispatch_order_id,
  DROP COLUMN IF EXISTS pickup_code;

ALTER TABLE deliveries
  DROP COLUMN IF EXISTS rider_delivered_at;
