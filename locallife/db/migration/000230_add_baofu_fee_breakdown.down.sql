DROP INDEX IF EXISTS order_payment_fee_ledgers_payment_order_idx;
DROP INDEX IF EXISTS order_payment_fee_ledgers_payer_idx;
DROP INDEX IF EXISTS order_payment_fee_ledgers_once_per_payer_uidx;
DROP TABLE IF EXISTS order_payment_fee_ledgers;

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_fee_breakdown_amounts_check,
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_settlement_mode_check;

ALTER TABLE profit_sharing_orders
    DROP COLUMN IF EXISTS platform_receiver_amount,
    DROP COLUMN IF EXISTS commission_base_amount,
    DROP COLUMN IF EXISTS rider_payment_fee_base_amount,
    DROP COLUMN IF EXISTS rider_payment_fee_rate_bps,
    DROP COLUMN IF EXISTS rider_payment_fee,
    DROP COLUMN IF EXISTS rider_gross_amount,
    DROP COLUMN IF EXISTS merchant_payment_fee_base_amount,
    DROP COLUMN IF EXISTS merchant_payment_fee_rate_bps,
    DROP COLUMN IF EXISTS merchant_payment_fee,
    DROP COLUMN IF EXISTS provider_payment_fee_source,
    DROP COLUMN IF EXISTS provider_payment_fee_base_amount,
    DROP COLUMN IF EXISTS provider_payment_fee_rate_bps,
    DROP COLUMN IF EXISTS provider_payment_fee,
    DROP COLUMN IF EXISTS settlement_mode,
    DROP COLUMN IF EXISTS calculation_version;
