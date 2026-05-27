ALTER TABLE profit_sharing_orders
    ADD COLUMN IF NOT EXISTS command_started_at TIMESTAMPTZ;

UPDATE profit_sharing_orders
SET command_started_at = created_at
WHERE provider = 'baofu'
  AND channel = 'baofu_aggregate'
  AND status = 'processing'
  AND command_started_at IS NULL;

CREATE INDEX IF NOT EXISTS profit_sharing_orders_baofu_processing_started_idx
    ON profit_sharing_orders(provider, channel, status, command_started_at, id)
    WHERE provider = 'baofu'
      AND channel = 'baofu_aggregate'
      AND status = 'processing';
