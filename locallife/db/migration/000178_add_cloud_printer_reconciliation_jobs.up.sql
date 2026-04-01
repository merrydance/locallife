CREATE TABLE cloud_printer_reconciliation_jobs (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    printer_id BIGINT REFERENCES cloud_printers(id) ON DELETE SET NULL,
    printer_name TEXT NOT NULL,
    printer_sn TEXT NOT NULL,
    printer_key TEXT,
    printer_type TEXT NOT NULL,
    desired_action TEXT NOT NULL CHECK (desired_action IN ('register', 'remove')),
    source_action TEXT NOT NULL CHECK (source_action IN ('create', 'delete')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'resolved')),
    failure_reason TEXT NOT NULL,
    last_error TEXT NOT NULL,
    retry_count INT NOT NULL DEFAULT 0,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cloud_printer_reconciliation_jobs_merchant_id
    ON cloud_printer_reconciliation_jobs (merchant_id, created_at DESC);

CREATE UNIQUE INDEX uq_cloud_printer_reconciliation_jobs_pending
    ON cloud_printer_reconciliation_jobs (merchant_id, printer_sn, desired_action)
    WHERE status = 'pending';