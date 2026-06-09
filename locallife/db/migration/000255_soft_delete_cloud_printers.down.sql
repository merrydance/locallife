DROP INDEX IF EXISTS cloud_printers_printer_sn_active_idx;

UPDATE cloud_printers cp
SET printer_sn = cp.printer_sn || '__deleted_' || cp.id
WHERE cp.deleted_at IS NOT NULL
  AND EXISTS (
      SELECT 1
      FROM cloud_printers duplicate
      WHERE duplicate.printer_sn = cp.printer_sn
        AND duplicate.id <> cp.id
  );

CREATE UNIQUE INDEX IF NOT EXISTS cloud_printers_printer_sn_idx
ON cloud_printers(printer_sn);

ALTER TABLE cloud_printers
    DROP COLUMN IF EXISTS deleted_at;
