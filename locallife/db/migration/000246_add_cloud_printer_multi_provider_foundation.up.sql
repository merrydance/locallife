ALTER TABLE print_logs
    ADD COLUMN provider_origin_id TEXT;

CREATE UNIQUE INDEX uq_print_logs_provider_origin_id
ON print_logs(provider_origin_id)
WHERE provider_origin_id IS NOT NULL;

CREATE INDEX print_logs_pending_provider_status_idx
ON print_logs(created_at, id)
WHERE status = 'pending'
  AND (
    vendor_order_id IS NOT NULL
    OR provider_origin_id IS NOT NULL
  );

ALTER TABLE print_logs
    DROP CONSTRAINT IF EXISTS print_logs_status_check,
    ADD CONSTRAINT print_logs_status_check CHECK (status IN ('pending', 'success', 'failed', 'cancelled'));

ALTER TABLE cloud_printers
    DROP CONSTRAINT IF EXISTS cloud_printers_printer_type_check,
    ADD CONSTRAINT cloud_printers_printer_type_check CHECK (printer_type IN ('feieyun', 'yilianyun', 'shangpeng', 'other'));
