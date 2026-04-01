ALTER TABLE print_logs
ADD COLUMN task_key TEXT;

CREATE UNIQUE INDEX uq_print_logs_task_key_printer
ON print_logs(task_key, printer_id)
WHERE task_key IS NOT NULL;