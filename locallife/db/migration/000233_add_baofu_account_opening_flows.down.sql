DROP INDEX IF EXISTS baofu_account_opening_flows_payment_order_idx;
DROP INDEX IF EXISTS baofu_account_opening_flows_state_idx;
DROP INDEX IF EXISTS baofu_account_opening_flows_open_trans_uidx;
DROP INDEX IF EXISTS baofu_account_opening_flows_active_owner_uidx;
DROP TABLE IF EXISTS baofu_account_opening_flows;

DROP INDEX IF EXISTS baofu_account_opening_profiles_status_idx;
DROP TABLE IF EXISTS baofu_account_opening_profiles;

DROP INDEX IF EXISTS payment_orders_baofu_verify_fee_active_uidx;

ALTER TABLE payment_orders
DROP CONSTRAINT IF EXISTS payment_orders_business_type_check;

ALTER TABLE payment_orders
ADD CONSTRAINT payment_orders_business_type_check
CHECK (business_type IN (
  'order',
  'reservation',
  'reservation_addon',
  'membership_recharge',
  'rider_deposit',
  'claim_recovery',
  'deposit',
  'recharge'
));
