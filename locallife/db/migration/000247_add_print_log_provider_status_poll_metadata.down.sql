DROP INDEX IF EXISTS print_logs_pending_provider_status_due_idx;

ALTER TABLE print_logs
    DROP COLUMN IF EXISTS provider_status_last_error,
    DROP COLUMN IF EXISTS provider_status_check_attempts,
    DROP COLUMN IF EXISTS provider_status_checked_at;
