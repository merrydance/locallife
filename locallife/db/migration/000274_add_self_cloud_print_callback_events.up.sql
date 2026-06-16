CREATE TABLE self_cloud_print_callback_events (
    id BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL,
    print_job_id TEXT NOT NULL,
    print_log_id BIGINT REFERENCES print_logs(id) ON DELETE SET NULL,
    status TEXT NOT NULL,
    raw_payload JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    CONSTRAINT self_cloud_print_callback_events_event_id_uq UNIQUE (event_id),
    CONSTRAINT self_cloud_print_callback_events_event_id_not_blank CHECK (btrim(event_id) <> ''),
    CONSTRAINT self_cloud_print_callback_events_print_job_id_not_blank CHECK (btrim(print_job_id) <> ''),
    CONSTRAINT self_cloud_print_callback_events_status_check CHECK (status IN ('success', 'failed', 'timeout', 'cancelled'))
);

CREATE INDEX self_cloud_print_callback_events_print_log_id_idx
ON self_cloud_print_callback_events(print_log_id)
WHERE print_log_id IS NOT NULL;

CREATE INDEX self_cloud_print_callback_events_print_job_id_idx
ON self_cloud_print_callback_events(print_job_id);
