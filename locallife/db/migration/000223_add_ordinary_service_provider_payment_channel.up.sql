ALTER TABLE payment_orders
	DROP CONSTRAINT IF EXISTS payment_orders_payment_channel_check;

ALTER TABLE payment_orders
	ADD CONSTRAINT payment_orders_payment_channel_check
	CHECK (payment_channel IN ('direct', 'ecommerce', 'ordinary_service_provider'));
