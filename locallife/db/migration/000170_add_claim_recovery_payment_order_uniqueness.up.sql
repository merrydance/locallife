CREATE UNIQUE INDEX IF NOT EXISTS payment_orders_claim_recovery_attach_active_uq
    ON payment_orders (business_type, attach)
    WHERE business_type = 'claim_recovery'
      AND attach IS NOT NULL
      AND status IN ('pending', 'paid');