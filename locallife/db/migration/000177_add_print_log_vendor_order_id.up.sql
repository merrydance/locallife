ALTER TABLE print_logs
ADD COLUMN vendor_order_id TEXT;

CREATE INDEX print_logs_vendor_order_id_idx ON print_logs(vendor_order_id)
WHERE vendor_order_id IS NOT NULL;