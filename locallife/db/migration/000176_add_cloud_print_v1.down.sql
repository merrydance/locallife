DROP INDEX IF EXISTS order_pickup_counters_pickup_date_idx;

ALTER TABLE order_display_configs
    DROP CONSTRAINT IF EXISTS order_display_configs_print_trigger_mode_check,
    DROP CONSTRAINT IF EXISTS order_display_configs_print_dispatch_mode_check,
    DROP COLUMN IF EXISTS print_trigger_mode,
    DROP COLUMN IF EXISTS print_dispatch_mode;

ALTER TABLE cloud_printers
    DROP CONSTRAINT IF EXISTS cloud_printers_printer_role_check,
    DROP COLUMN IF EXISTS printer_role;

DROP TABLE IF EXISTS order_pickup_counters;