ALTER TABLE cloud_printers
    DROP CONSTRAINT IF EXISTS cloud_printers_printer_type_check,
    ADD CONSTRAINT cloud_printers_printer_type_check CHECK (printer_type IN ('feieyun', 'yilianyun', 'shangpeng', 'self_cloud', 'other'));
