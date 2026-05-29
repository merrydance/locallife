CREATE TABLE reservation_adjustments (
    id BIGSERIAL PRIMARY KEY,
    reservation_id BIGINT NOT NULL REFERENCES table_reservations(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    direction TEXT NOT NULL,
    status TEXT NOT NULL,
    current_total BIGINT NOT NULL,
    target_total BIGINT NOT NULL,
    delta_amount BIGINT NOT NULL,
    payment_order_id BIGINT UNIQUE REFERENCES payment_orders(id),
    failure_reason TEXT,
    close_reason TEXT,
    applied_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT reservation_adjustments_direction_check CHECK (direction IN ('positive', 'negative', 'zero')),
    CONSTRAINT reservation_adjustments_status_check CHECK (status IN ('creating_payment', 'pending_payment', 'applying', 'applied', 'closed', 'failed', 'expired')),
    CONSTRAINT reservation_adjustments_totals_check CHECK (current_total >= 0 AND target_total >= 0),
    CONSTRAINT reservation_adjustments_delta_check CHECK (delta_amount = target_total - current_total),
    CONSTRAINT reservation_adjustments_direction_delta_check CHECK (
        (direction = 'positive' AND delta_amount > 0)
        OR (direction = 'negative' AND delta_amount < 0)
        OR (direction = 'zero' AND delta_amount = 0)
    ),
    CONSTRAINT reservation_adjustments_payment_status_check CHECK (
        (direction = 'positive' AND payment_order_id IS NOT NULL)
        OR (direction <> 'positive')
        OR status = 'creating_payment'
    )
);

CREATE TABLE reservation_adjustment_items (
    id BIGSERIAL PRIMARY KEY,
    adjustment_id BIGINT NOT NULL REFERENCES reservation_adjustments(id) ON DELETE CASCADE,
    dish_id BIGINT REFERENCES dishes(id) ON DELETE SET NULL,
    combo_id BIGINT REFERENCES combo_sets(id) ON DELETE SET NULL,
    quantity SMALLINT NOT NULL,
    unit_price BIGINT NOT NULL,
    total_price BIGINT NOT NULL,
    position INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT reservation_adjustment_items_dish_or_combo_check CHECK (
        (dish_id IS NOT NULL AND combo_id IS NULL)
        OR (dish_id IS NULL AND combo_id IS NOT NULL)
    ),
    CONSTRAINT reservation_adjustment_items_quantity_check CHECK (quantity > 0),
    CONSTRAINT reservation_adjustment_items_price_check CHECK (unit_price >= 0 AND total_price >= 0)
);

CREATE TABLE reservation_adjustment_inventory_holds (
    id BIGSERIAL PRIMARY KEY,
    adjustment_id BIGINT NOT NULL REFERENCES reservation_adjustments(id) ON DELETE CASCADE,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    dish_id BIGINT NOT NULL REFERENCES dishes(id),
    reservation_date DATE NOT NULL,
    quantity INT4 NOT NULL,
    status TEXT NOT NULL DEFAULT 'held',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT reservation_adjustment_inventory_holds_status_check CHECK (status IN ('held', 'converted', 'released')),
    CONSTRAINT reservation_adjustment_inventory_holds_quantity_check CHECK (quantity > 0)
);

CREATE UNIQUE INDEX reservation_adjustments_one_active_positive_uidx
ON reservation_adjustments(reservation_id)
WHERE direction = 'positive'
  AND status IN ('creating_payment', 'pending_payment', 'applying');

CREATE INDEX reservation_adjustments_reservation_idx ON reservation_adjustments(reservation_id);
CREATE INDEX reservation_adjustments_payment_order_idx ON reservation_adjustments(payment_order_id) WHERE payment_order_id IS NOT NULL;
CREATE INDEX reservation_adjustments_status_idx ON reservation_adjustments(status);
CREATE INDEX reservation_adjustments_merchant_status_idx ON reservation_adjustments(merchant_id, status);

CREATE INDEX reservation_adjustment_items_adjustment_idx ON reservation_adjustment_items(adjustment_id);
CREATE INDEX reservation_adjustment_items_dish_idx ON reservation_adjustment_items(dish_id) WHERE dish_id IS NOT NULL;
CREATE INDEX reservation_adjustment_items_combo_idx ON reservation_adjustment_items(combo_id) WHERE combo_id IS NOT NULL;

CREATE INDEX reservation_adjustment_inventory_holds_adjustment_idx ON reservation_adjustment_inventory_holds(adjustment_id);
CREATE INDEX reservation_adjustment_inventory_holds_status_expires_idx ON reservation_adjustment_inventory_holds(status, expires_at);
CREATE INDEX reservation_adjustment_inventory_holds_inventory_idx ON reservation_adjustment_inventory_holds(merchant_id, dish_id, reservation_date);
