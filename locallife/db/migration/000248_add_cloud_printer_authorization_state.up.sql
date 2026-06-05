CREATE TABLE cloud_printer_authorization_sessions (
    id BIGSERIAL PRIMARY KEY,
    state TEXT NOT NULL UNIQUE,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    provider_type TEXT NOT NULL CHECK (provider_type IN ('yilianyun')),
    printer_name TEXT,
    printer_role TEXT CHECK (printer_role IS NULL OR printer_role IN ('front', 'kitchen')),
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cloud_printer_authorization_sessions_merchant_provider
    ON cloud_printer_authorization_sessions (merchant_id, provider_type, created_at DESC);

CREATE INDEX idx_cloud_printer_authorization_sessions_active
    ON cloud_printer_authorization_sessions (state, expires_at)
    WHERE consumed_at IS NULL;

CREATE TABLE cloud_printer_provider_authorizations (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    provider_type TEXT NOT NULL CHECK (provider_type IN ('yilianyun')),
    machine_code TEXT NOT NULL,
    authorized_cloud_printer_id BIGINT REFERENCES cloud_printers(id) ON DELETE SET NULL,
    access_token_ciphertext TEXT NOT NULL,
    refresh_token_ciphertext TEXT NOT NULL,
    access_token_expires_at TIMESTAMPTZ NOT NULL,
    refresh_token_expires_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'refresh_failed', 'revoked')),
    refresh_failure_count INT NOT NULL DEFAULT 0 CHECK (refresh_failure_count >= 0),
    refresh_last_attempted_at TIMESTAMPTZ,
    last_provider_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT cloud_printer_provider_authorizations_unique_machine
        UNIQUE (provider_type, machine_code)
);

CREATE INDEX idx_cloud_printer_provider_authorizations_merchant_provider
    ON cloud_printer_provider_authorizations (merchant_id, provider_type, created_at DESC);

CREATE INDEX idx_cloud_printer_provider_authorizations_refresh_due
    ON cloud_printer_provider_authorizations (status, access_token_expires_at, id)
    WHERE status IN ('active', 'refresh_failed');
