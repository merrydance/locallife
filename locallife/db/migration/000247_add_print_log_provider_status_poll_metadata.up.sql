ALTER TABLE print_logs
    ADD COLUMN provider_status_checked_at TIMESTAMPTZ,
    ADD COLUMN provider_status_check_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN provider_status_last_error TEXT;

CREATE INDEX print_logs_pending_provider_status_due_idx
ON print_logs(provider_status_checked_at, created_at, id)
WHERE status = 'pending'
  AND vendor_order_id IS NOT NULL;
