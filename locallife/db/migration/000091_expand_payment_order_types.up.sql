ALTER TABLE payment_orders
DROP CONSTRAINT IF EXISTS payment_orders_payment_type_check;

ALTER TABLE payment_orders
ADD CONSTRAINT payment_orders_payment_type_check
CHECK (payment_type IN ('miniprogram', 'native', 'profit_sharing', 'wechat', 'jsapi', 'mini_program'));

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
  'deposit',
  'recharge'
));
