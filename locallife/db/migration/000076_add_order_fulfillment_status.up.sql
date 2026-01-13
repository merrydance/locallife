ALTER TABLE orders
ADD COLUMN fulfillment_status text NOT NULL DEFAULT 'scheduled';
