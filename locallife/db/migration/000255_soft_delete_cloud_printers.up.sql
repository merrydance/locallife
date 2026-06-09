ALTER TABLE cloud_printers
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE cloud_printers
    DROP CONSTRAINT IF EXISTS cloud_printers_printer_sn_key;

DROP INDEX IF EXISTS cloud_printers_printer_sn_idx;
DROP INDEX IF EXISTS cloud_printers_printer_sn_active_idx;

CREATE UNIQUE INDEX cloud_printers_printer_sn_active_idx
ON cloud_printers(printer_sn)
WHERE deleted_at IS NULL;
