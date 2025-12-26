-- name: CreateOrderItem :one
INSERT INTO order_items (
    order_id,
    dish_id,
    combo_id,
    name,
    unit_price,
    quantity,
    subtotal,
    customizations
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: BatchCreateOrderItems :copyfrom
INSERT INTO order_items (
    order_id,
    dish_id,
    combo_id,
    name,
    unit_price,
    quantity,
    subtotal,
    customizations
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: GetOrderItem :one
SELECT * FROM order_items
WHERE id = $1 LIMIT 1;

-- name: ListOrderItemsByOrder :many
SELECT * FROM order_items
WHERE order_id = $1
ORDER BY id;

-- name: ListOrderItemsWithDishByOrder :many
SELECT 
    oi.*,
    d.image_url as dish_image_url
FROM order_items oi
LEFT JOIN dishes d ON oi.dish_id = d.id
WHERE oi.order_id = $1
ORDER BY oi.id;

-- name: DeleteOrderItems :exec
DELETE FROM order_items
WHERE order_id = $1;
