ALTER TABLE cloud_printers
    DROP CONSTRAINT IF EXISTS cloud_printers_printer_type_check,
    ADD CONSTRAINT cloud_printers_printer_type_check CHECK (printer_type IN ('feieyun', 'yilianyun', 'other'));

ALTER TABLE print_logs
    DROP CONSTRAINT IF EXISTS print_logs_status_check,
    ADD CONSTRAINT print_logs_status_check CHECK (status IN ('pending', 'success', 'failed'));

DROP INDEX IF EXISTS print_logs_pending_provider_status_idx;
DROP INDEX IF EXISTS uq_print_logs_provider_origin_id;

ALTER TABLE print_logs
    DROP COLUMN IF EXISTS provider_origin_id;
