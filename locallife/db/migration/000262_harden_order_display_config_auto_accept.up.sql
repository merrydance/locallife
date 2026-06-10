UPDATE order_display_configs
SET auto_accept_paid_orders = false,
    updated_at = now()
WHERE enable_print = false
  AND auto_accept_paid_orders = true;

ALTER TABLE order_display_configs
    DROP CONSTRAINT IF EXISTS order_display_configs_print_auto_accept_check;

ALTER TABLE order_display_configs
    ADD CONSTRAINT order_display_configs_print_auto_accept_check
    CHECK (enable_print OR NOT auto_accept_paid_orders);
