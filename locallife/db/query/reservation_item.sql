-- name: CreateReservationItem :one
INSERT INTO reservation_items (
    reservation_id,
    dish_id,
    combo_id,
    quantity,
    unit_price,
    total_price
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: ListReservationItems :many
SELECT 
    ri.*,
    d.name as dish_name,
    d.image_media_asset_id as dish_image_media_asset_id,
    cs.name as combo_name,
    cs.image_media_asset_id as combo_image_media_asset_id
FROM reservation_items ri
LEFT JOIN dishes d ON ri.dish_id = d.id
LEFT JOIN combo_sets cs ON ri.combo_id = cs.id
WHERE ri.reservation_id = $1
ORDER BY ri.id;

-- name: GetReservationItemsByReservation :many
SELECT id, reservation_id, dish_id, combo_id, quantity, unit_price, total_price, created_at FROM reservation_items
WHERE reservation_id = $1
ORDER BY id;

-- name: DeleteReservationItems :exec
DELETE FROM reservation_items
WHERE reservation_id = $1;

-- name: CountReservationItems :one
SELECT COUNT(*) FROM reservation_items
WHERE reservation_id = $1;

-- name: SumReservationItemsTotal :one
SELECT COALESCE(SUM(total_price), 0)::bigint as total
FROM reservation_items
WHERE reservation_id = $1;

-- name: ListReservationDishSummary :many
SELECT dish_id, SUM(quantity)::int4 as quantity
FROM reservation_items
WHERE reservation_id = $1 AND dish_id IS NOT NULL
GROUP BY dish_id
ORDER BY dish_id;
