DROP INDEX IF EXISTS print_logs_vendor_order_id_idx;

ALTER TABLE print_logs
DROP COLUMN IF EXISTS vendor_order_id;