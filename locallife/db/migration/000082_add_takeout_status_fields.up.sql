-- Add fine-grained takeout statuses and related fields

ALTER TABLE orders
  ADD COLUMN pickup_code VARCHAR(32),
  ADD COLUMN dispatch_order_id BIGINT,
  ADD COLUMN flow_id BIGINT,
  ADD COLUMN status_hint TEXT,
  ADD COLUMN badges JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN exception_state TEXT,
  ADD COLUMN claim_channel TEXT,
  ADD COLUMN overtime BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN prep_start_at TIMESTAMPTZ,
  ADD COLUMN ready_at TIMESTAMPTZ,
  ADD COLUMN courier_accept_at TIMESTAMPTZ,
  ADD COLUMN picked_at TIMESTAMPTZ,
  ADD COLUMN rider_delivered_at TIMESTAMPTZ,
  ADD COLUMN user_delivered_at TIMESTAMPTZ,
  ADD COLUMN auto_user_delivered_at TIMESTAMPTZ;

-- Extend status enum to include courier/delivery granular states
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (
  status IN (
    'pending',
    'paid',
    'preparing',
    'ready',
    'courier_accepted',
    'picked',
    'delivering',
    'rider_delivered',
    'user_delivered',
    'completed',
    'cancelled'
  )
);

-- Delivery table alignment: rider-delivered timestamp
ALTER TABLE deliveries
  ADD COLUMN rider_delivered_at TIMESTAMPTZ;
