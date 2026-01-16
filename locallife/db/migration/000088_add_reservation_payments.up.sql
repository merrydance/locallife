CREATE TABLE IF NOT EXISTS reservation_payments (
    id BIGSERIAL PRIMARY KEY,
    reservation_id BIGINT NOT NULL REFERENCES table_reservations(id) ON DELETE CASCADE,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,
    type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT reservation_payments_type_check CHECK (type IN ('reservation', 'addon')),
    CONSTRAINT reservation_payments_amount_check CHECK (amount > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS reservation_payments_payment_order_id_idx
    ON reservation_payments(payment_order_id);

CREATE INDEX IF NOT EXISTS reservation_payments_reservation_id_idx
    ON reservation_payments(reservation_id);
