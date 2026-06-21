CREATE TABLE merchant_packaging_settings (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT false,
    required BOOLEAN NOT NULL DEFAULT true,
    applicable_order_types TEXT[] NOT NULL DEFAULT ARRAY['takeout','takeaway']::TEXT[],
    default_option_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    CONSTRAINT merchant_packaging_settings_order_types_check CHECK (
        applicable_order_types <@ ARRAY['takeout','takeaway']::TEXT[]
    )
);

CREATE TABLE merchant_packaging_options (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    legacy_dish_id BIGINT REFERENCES dishes(id),
    name TEXT NOT NULL,
    description TEXT,
    price BIGINT NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    sort_order SMALLINT NOT NULL DEFAULT 0,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    CONSTRAINT merchant_packaging_options_price_check CHECK (price >= 0),
    CONSTRAINT merchant_packaging_options_name_check CHECK (char_length(trim(name)) BETWEEN 1 AND 50)
);

CREATE INDEX idx_merchant_packaging_options_merchant_active
ON merchant_packaging_options(merchant_id, is_enabled, sort_order, id)
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uq_merchant_packaging_options_legacy_dish
ON merchant_packaging_options(legacy_dish_id)
WHERE legacy_dish_id IS NOT NULL;

CREATE UNIQUE INDEX uq_merchant_packaging_options_name_active
ON merchant_packaging_options(merchant_id, lower(name))
WHERE deleted_at IS NULL;

CREATE TABLE cart_packaging_selections (
    cart_id BIGINT PRIMARY KEY REFERENCES carts(id) ON DELETE CASCADE,
    packaging_option_id BIGINT REFERENCES merchant_packaging_options(id),
    selection_version BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE order_packaging_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    packaging_option_id BIGINT REFERENCES merchant_packaging_options(id),
    name TEXT NOT NULL,
    unit_price BIGINT NOT NULL,
    quantity SMALLINT NOT NULL DEFAULT 1,
    subtotal BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT order_packaging_items_price_check CHECK (unit_price >= 0),
    CONSTRAINT order_packaging_items_quantity_check CHECK (quantity > 0),
    CONSTRAINT order_packaging_items_subtotal_check CHECK (subtotal = unit_price * quantity),
    CONSTRAINT order_packaging_items_name_check CHECK (char_length(trim(name)) BETWEEN 1 AND 50)
);

CREATE UNIQUE INDEX uq_order_packaging_items_order
ON order_packaging_items(order_id);

ALTER TABLE orders
ADD COLUMN packaging_fee BIGINT NOT NULL DEFAULT 0;
