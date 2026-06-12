CREATE TABLE merchant_local_print_events (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    event_key TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'merchant_app_ble',
    status TEXT NOT NULL,
    printer_name TEXT,
    error_message TEXT,
    printed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    CONSTRAINT merchant_local_print_events_source_check
        CHECK (source IN ('merchant_app_ble')),
    CONSTRAINT merchant_local_print_events_status_check
        CHECK (status IN ('started', 'success', 'failed')),
    CONSTRAINT merchant_local_print_events_event_key_not_blank
        CHECK (btrim(event_key) <> '')
);

CREATE UNIQUE INDEX merchant_local_print_events_merchant_event_key_uq
    ON merchant_local_print_events(merchant_id, event_key);

CREATE INDEX merchant_local_print_events_order_id_idx
    ON merchant_local_print_events(order_id);

CREATE INDEX merchant_local_print_events_merchant_status_idx
    ON merchant_local_print_events(merchant_id, status, created_at DESC);
