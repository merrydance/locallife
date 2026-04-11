-- Reservation inventory tracking

-- name: ListReservationInventoryByReservation :many
SELECT * FROM reservation_inventory
WHERE reservation_id = $1
ORDER BY dish_id;

-- name: UpsertReservationInventory :one
INSERT INTO reservation_inventory (
  reservation_id,
  dish_id,
  quantity
) VALUES ($1, $2, $3)
ON CONFLICT (reservation_id, dish_id)
DO UPDATE SET quantity = EXCLUDED.quantity,
              updated_at = now()
RETURNING *;

-- name: DeleteReservationInventoryByDish :exec
DELETE FROM reservation_inventory
WHERE reservation_id = $1 AND dish_id = $2;
