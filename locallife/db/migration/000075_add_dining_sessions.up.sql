-- Dining sessions to track reservation/dine-in table usage and active order
CREATE TABLE dining_sessions (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    table_id BIGINT NOT NULL REFERENCES tables(id),
    reservation_id BIGINT REFERENCES table_reservations(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    active_order_id BIGINT REFERENCES orders(id),
    status TEXT NOT NULL DEFAULT 'open',
    opened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    CONSTRAINT dining_sessions_status_check CHECK (status IN ('open', 'closed'))
);

-- Only one open session per table
CREATE UNIQUE INDEX dining_sessions_table_open_idx ON dining_sessions(table_id) WHERE status = 'open';
-- Only one open session per reservation
CREATE UNIQUE INDEX dining_sessions_reservation_open_idx ON dining_sessions(reservation_id) WHERE reservation_id IS NOT NULL AND status = 'open';
-- Finder indexes
CREATE INDEX dining_sessions_table_idx ON dining_sessions(table_id);
CREATE INDEX dining_sessions_reservation_idx ON dining_sessions(reservation_id);
CREATE INDEX dining_sessions_user_idx ON dining_sessions(user_id);
CREATE INDEX dining_sessions_status_idx ON dining_sessions(status);
