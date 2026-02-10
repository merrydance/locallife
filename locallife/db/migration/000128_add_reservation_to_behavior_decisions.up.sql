ALTER TABLE behavior_decisions
    ADD COLUMN reservation_id BIGINT REFERENCES table_reservations(id) ON DELETE SET NULL,
    ALTER COLUMN order_id DROP NOT NULL;

CREATE INDEX idx_behavior_decisions_reservation_id ON behavior_decisions(reservation_id);

-- Ensure either order_id or reservation_id is set
ALTER TABLE behavior_decisions
    ADD CONSTRAINT check_behavior_decisions_source CHECK (
        (order_id IS NOT NULL AND reservation_id IS NULL) OR
        (order_id IS NULL AND reservation_id IS NOT NULL)
    );
