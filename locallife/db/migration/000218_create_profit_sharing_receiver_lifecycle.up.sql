CREATE TABLE profit_sharing_receiver_targets (
	id BIGSERIAL PRIMARY KEY,
	provider TEXT NOT NULL,
	channel TEXT NOT NULL,
	owner_type TEXT NOT NULL,
	owner_id BIGINT NOT NULL,
	receiver_type TEXT NOT NULL,
	appid TEXT NOT NULL,
	account_hash TEXT NOT NULL,
	display_name_hash TEXT,
	desired_state TEXT NOT NULL,
	sync_status TEXT NOT NULL DEFAULT 'pending',
	attempt_count INTEGER NOT NULL DEFAULT 0,
	next_retry_at TIMESTAMPTZ,
	last_error_code TEXT,
	last_error_message TEXT,
	last_attempt_at TIMESTAMPTZ,
	synced_at TIMESTAMPTZ,
	skipped_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT profit_sharing_receiver_targets_provider_check CHECK (provider IN ('wechat')),
	CONSTRAINT profit_sharing_receiver_targets_channel_check CHECK (channel IN ('ecommerce')),
	CONSTRAINT profit_sharing_receiver_targets_owner_type_check CHECK (owner_type IN ('rider', 'operator', 'manual')),
	CONSTRAINT profit_sharing_receiver_targets_receiver_type_check CHECK (receiver_type IN ('PERSONAL_OPENID', 'MERCHANT_ID')),
	CONSTRAINT profit_sharing_receiver_targets_desired_state_check CHECK (desired_state IN ('present', 'absent')),
	CONSTRAINT profit_sharing_receiver_targets_sync_status_check CHECK (sync_status IN ('pending', 'processing', 'synced', 'failed', 'skipped')),
	CONSTRAINT profit_sharing_receiver_targets_attempt_count_check CHECK (attempt_count >= 0),
	CONSTRAINT profit_sharing_receiver_targets_account_hash_check CHECK (length(trim(account_hash)) > 0),
	CONSTRAINT profit_sharing_receiver_targets_appid_check CHECK (length(trim(appid)) > 0),
	CONSTRAINT profit_sharing_receiver_targets_unique UNIQUE (provider, channel, owner_type, owner_id, receiver_type, appid, account_hash)
);

CREATE INDEX idx_profit_sharing_receiver_targets_retry
	ON profit_sharing_receiver_targets (next_retry_at ASC NULLS FIRST, id ASC)
	WHERE sync_status IN ('pending', 'failed');

CREATE INDEX idx_profit_sharing_receiver_targets_owner
	ON profit_sharing_receiver_targets (owner_type, owner_id, id);

CREATE TABLE profit_sharing_receiver_attempts (
	id BIGSERIAL PRIMARY KEY,
	target_id BIGINT NOT NULL REFERENCES profit_sharing_receiver_targets(id) ON DELETE CASCADE,
	action TEXT NOT NULL,
	status TEXT NOT NULL,
	idempotent_success BOOLEAN NOT NULL DEFAULT false,
	error_code TEXT,
	error_message TEXT,
	started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	finished_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT profit_sharing_receiver_attempts_action_check CHECK (action IN ('ensure', 'delete')),
	CONSTRAINT profit_sharing_receiver_attempts_status_check CHECK (status IN ('processing', 'succeeded', 'failed', 'skipped'))
);

CREATE INDEX idx_profit_sharing_receiver_attempts_target
	ON profit_sharing_receiver_attempts (target_id, id DESC);
