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
SELECT id, order_id, from_status, to_status, operator_id, operator_type, notes, created_at FROM order_status_logs
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

-- name: CountRecentOrderStatusLogs :one
-- 统计指定订单在指定时间后的特定类型日志数量（用于速率限制）
SELECT COUNT(*)::bigint FROM order_status_logs
WHERE order_id = $1
  AND notes = $2
  AND created_at >= $3;
