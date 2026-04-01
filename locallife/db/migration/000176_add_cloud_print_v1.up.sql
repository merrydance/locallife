CREATE TABLE order_pickup_counters (
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    pickup_date DATE NOT NULL,
    last_sequence INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    PRIMARY KEY (merchant_id, pickup_date),
    CONSTRAINT order_pickup_counters_last_sequence_check CHECK (last_sequence >= 0)
);

ALTER TABLE cloud_printers
    ADD COLUMN printer_role TEXT NOT NULL DEFAULT 'front',
    ADD CONSTRAINT cloud_printers_printer_role_check CHECK (printer_role IN ('front', 'kitchen'));

ALTER TABLE order_display_configs
    ADD COLUMN print_dispatch_mode TEXT NOT NULL DEFAULT 'single_full',
    ADD COLUMN print_trigger_mode TEXT NOT NULL DEFAULT 'accepted',
    ADD CONSTRAINT order_display_configs_print_dispatch_mode_check CHECK (print_dispatch_mode IN ('single_full', 'split')),
    ADD CONSTRAINT order_display_configs_print_trigger_mode_check CHECK (print_trigger_mode IN ('accepted', 'ready', 'manual'));

CREATE INDEX order_pickup_counters_pickup_date_idx ON order_pickup_counters(pickup_date);