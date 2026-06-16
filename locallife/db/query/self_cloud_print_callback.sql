-- name: CreateSelfCloudPrintCallbackEvent :one
INSERT INTO self_cloud_print_callback_events (
    event_id,
    print_job_id,
    print_log_id,
    status,
    raw_payload,
    processed_at
) VALUES (
    $1, $2, $3, $4, $5, now()
) RETURNING id, event_id, print_job_id, print_log_id, status, raw_payload, received_at, processed_at;

-- name: GetSelfCloudPrintCallbackEventByEventID :one
SELECT id, event_id, print_job_id, print_log_id, status, raw_payload, received_at, processed_at
FROM self_cloud_print_callback_events
WHERE event_id = $1
LIMIT 1;
