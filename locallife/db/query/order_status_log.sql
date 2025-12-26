-- name: CreateOrderStatusLog :one
INSERT INTO order_status_logs (
    order_id,
    from_status,
    to_status,
    operator_id,
    operator_type,
    notes
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: ListOrderStatusLogs :many
SELECT * FROM order_status_logs
WHERE order_id = $1
ORDER BY created_at;

-- name: ListOrderStatusLogsWithOperator :many
SELECT 
    osl.*,
    u.full_name as operator_name
FROM order_status_logs osl
LEFT JOIN users u ON osl.operator_id = u.id
WHERE osl.order_id = $1
ORDER BY osl.created_at;
