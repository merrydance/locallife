DROP INDEX IF EXISTS profit_sharing_orders_baofu_processing_started_idx;

ALTER TABLE profit_sharing_orders
    DROP COLUMN IF EXISTS command_started_at;
