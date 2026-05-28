ALTER TABLE order_display_configs
    ADD COLUMN auto_accept_paid_orders BOOLEAN NOT NULL DEFAULT false;
