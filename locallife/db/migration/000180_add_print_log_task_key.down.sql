DROP INDEX IF EXISTS uq_print_logs_task_key_printer;

ALTER TABLE print_logs
DROP COLUMN IF EXISTS task_key;