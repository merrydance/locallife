CREATE TABLE external_payment_commands (
	id BIGSERIAL PRIMARY KEY,
	provider TEXT NOT NULL,
	channel TEXT NOT NULL,
	capability TEXT NOT NULL,
	command_type TEXT NOT NULL,
	business_owner TEXT NOT NULL,
	business_object_type TEXT,
	business_object_id BIGINT,
	external_object_type TEXT NOT NULL,
	external_object_key TEXT NOT NULL,
	external_secondary_key TEXT,
	command_status TEXT NOT NULL,
	submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	accepted_at TIMESTAMPTZ,
	rejected_at TIMESTAMPTZ,
	last_error_code TEXT,
	last_error_message TEXT,
	request_fingerprint TEXT,
	response_snapshot JSONB,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT external_payment_commands_channel_check CHECK (channel IN ('direct', 'ecommerce')),
	CONSTRAINT external_payment_commands_command_status_check CHECK (command_status IN ('submitted', 'accepted', 'rejected', 'unknown')),
	CONSTRAINT external_payment_commands_object_key_not_empty CHECK (length(trim(external_object_key)) > 0),
	CONSTRAINT external_payment_commands_business_object_pair_check CHECK (
		(business_object_type IS NULL AND business_object_id IS NULL) OR
		(business_object_type IS NOT NULL AND business_object_id IS NOT NULL)
	)
);

CREATE UNIQUE INDEX external_payment_commands_external_uidx
ON external_payment_commands(provider, channel, capability, command_type, external_object_type, external_object_key);

CREATE INDEX external_payment_commands_business_idx
ON external_payment_commands(business_owner, business_object_type, business_object_id)
WHERE business_object_type IS NOT NULL AND business_object_id IS NOT NULL;

CREATE INDEX external_payment_commands_status_idx
ON external_payment_commands(command_status, submitted_at ASC, id ASC);

CREATE TABLE external_payment_facts (
	id BIGSERIAL PRIMARY KEY,
	provider TEXT NOT NULL,
	channel TEXT NOT NULL,
	capability TEXT NOT NULL,
	fact_source TEXT NOT NULL,
	source_event_id TEXT,
	source_event_type TEXT,
	external_object_type TEXT NOT NULL,
	external_object_key TEXT NOT NULL,
	external_secondary_key TEXT,
	business_owner TEXT,
	business_object_type TEXT,
	business_object_id BIGINT,
	upstream_state TEXT NOT NULL,
	terminal_status TEXT NOT NULL,
	is_terminal BOOLEAN NOT NULL,
	amount BIGINT,
	currency TEXT NOT NULL DEFAULT 'CNY',
	occurred_at TIMESTAMPTZ,
	upstream_updated_at TIMESTAMPTZ,
	observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	raw_resource JSONB NOT NULL DEFAULT '{}'::jsonb,
	dedupe_key TEXT NOT NULL,
	processing_status TEXT NOT NULL DEFAULT 'received',
	processing_error TEXT,
	processed_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT external_payment_facts_channel_check CHECK (channel IN ('direct', 'ecommerce')),
	CONSTRAINT external_payment_facts_source_check CHECK (fact_source IN ('callback', 'query', 'manual_reconciliation')),
	CONSTRAINT external_payment_facts_terminal_status_check CHECK (terminal_status IN ('success', 'failed', 'closed', 'expired', 'processing', 'unknown')),
	CONSTRAINT external_payment_facts_terminal_consistency_check CHECK (
		is_terminal = (terminal_status IN ('success', 'failed', 'closed', 'expired'))
	),
	CONSTRAINT external_payment_facts_processing_status_check CHECK (processing_status IN ('received', 'terminalized', 'ignored', 'failed')),
	CONSTRAINT external_payment_facts_amount_check CHECK (amount IS NULL OR amount >= 0),
	CONSTRAINT external_payment_facts_object_key_not_empty CHECK (length(trim(external_object_key)) > 0),
	CONSTRAINT external_payment_facts_dedupe_key_not_empty CHECK (length(trim(dedupe_key)) > 0),
	CONSTRAINT external_payment_facts_business_object_pair_check CHECK (
		(business_object_type IS NULL AND business_object_id IS NULL) OR
		(business_object_type IS NOT NULL AND business_object_id IS NOT NULL)
	)
);

CREATE UNIQUE INDEX external_payment_facts_dedupe_uidx
ON external_payment_facts(dedupe_key);

CREATE INDEX external_payment_facts_external_object_idx
ON external_payment_facts(provider, channel, external_object_type, external_object_key, created_at DESC, id DESC);

CREATE INDEX external_payment_facts_business_idx
ON external_payment_facts(business_owner, business_object_type, business_object_id, created_at DESC, id DESC)
WHERE business_owner IS NOT NULL AND business_object_type IS NOT NULL AND business_object_id IS NOT NULL;

CREATE INDEX external_payment_facts_processing_idx
ON external_payment_facts(processing_status, is_terminal, observed_at ASC, id ASC);

CREATE TABLE external_payment_fact_applications (
	id BIGSERIAL PRIMARY KEY,
	fact_id BIGINT NOT NULL REFERENCES external_payment_facts(id) ON DELETE CASCADE,
	consumer TEXT NOT NULL,
	business_object_type TEXT NOT NULL,
	business_object_id BIGINT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	attempt_count INT NOT NULL DEFAULT 0,
	last_error TEXT,
	next_retry_at TIMESTAMPTZ,
	applied_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT external_payment_fact_applications_status_check CHECK (status IN ('pending', 'processing', 'applied', 'skipped', 'failed')),
	CONSTRAINT external_payment_fact_applications_attempt_count_check CHECK (attempt_count >= 0)
);

CREATE UNIQUE INDEX external_payment_fact_applications_consumer_uidx
ON external_payment_fact_applications(fact_id, consumer, business_object_type, business_object_id);

CREATE INDEX external_payment_fact_applications_retry_idx
ON external_payment_fact_applications(status, next_retry_at ASC, id ASC)
WHERE status IN ('pending', 'failed');

CREATE TABLE payment_domain_outbox (
	id BIGSERIAL PRIMARY KEY,
	event_type TEXT NOT NULL,
	aggregate_type TEXT NOT NULL,
	aggregate_id BIGINT NOT NULL,
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	status TEXT NOT NULL DEFAULT 'pending',
	attempt_count INT NOT NULL DEFAULT 0,
	next_retry_at TIMESTAMPTZ,
	last_error TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT payment_domain_outbox_status_check CHECK (status IN ('pending', 'processing', 'published', 'failed')),
	CONSTRAINT payment_domain_outbox_attempt_count_check CHECK (attempt_count >= 0)
);

CREATE INDEX payment_domain_outbox_dispatch_idx
ON payment_domain_outbox(status, next_retry_at ASC, id ASC)
WHERE status IN ('pending', 'failed');

CREATE INDEX payment_domain_outbox_aggregate_idx
ON payment_domain_outbox(aggregate_type, aggregate_id, created_at DESC, id DESC);
