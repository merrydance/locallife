-- Track reserved inventory per reservation and dish
CREATE TABLE reservation_inventory (
    id BIGSERIAL PRIMARY KEY,
    reservation_id BIGINT NOT NULL REFERENCES table_reservations(id) ON DELETE CASCADE,
    dish_id BIGINT NOT NULL REFERENCES dishes(id),
    quantity INT4 NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    UNIQUE (reservation_id, dish_id)
);

CREATE INDEX reservation_inventory_reservation_idx ON reservation_inventory(reservation_id);
CREATE INDEX reservation_inventory_dish_idx ON reservation_inventory(dish_id);
