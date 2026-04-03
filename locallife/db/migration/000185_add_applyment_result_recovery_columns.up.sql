ALTER TABLE ecommerce_applyments
ADD COLUMN result_task_processed_state TEXT,
ADD COLUMN result_task_processed_at TIMESTAMPTZ;

CREATE INDEX idx_ecommerce_applyments_result_recovery
ON ecommerce_applyments (updated_at ASC, id ASC)
WHERE status IN ('submitted', 'auditing', 'to_be_signed', 'to_be_confirmed', 'signing', 'finish', 'rejected');