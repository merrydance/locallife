ALTER TABLE external_payment_commands
	DROP CONSTRAINT IF EXISTS external_payment_commands_channel_check;

ALTER TABLE external_payment_commands
	ADD CONSTRAINT external_payment_commands_channel_check
	CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider'));

ALTER TABLE external_payment_facts
	DROP CONSTRAINT IF EXISTS external_payment_facts_channel_check;

ALTER TABLE external_payment_facts
	ADD CONSTRAINT external_payment_facts_channel_check
	CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider'));
