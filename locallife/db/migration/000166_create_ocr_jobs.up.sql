CREATE TABLE ocr_jobs (
    id               bigserial PRIMARY KEY,
    idempotency_key  text        NOT NULL,
    document_type    text        NOT NULL,
    provider         text        NOT NULL,
    provider_task_id text,
    media_asset_id   bigint      NOT NULL REFERENCES media_assets(id),
    owner_type       text        NOT NULL,
    owner_id         bigint      NOT NULL,
    side             text        NOT NULL DEFAULT '',
    status           text        NOT NULL DEFAULT 'pending',
    attempt_count    integer     NOT NULL DEFAULT 0,
    max_attempts     integer     NOT NULL DEFAULT 3,
    next_retry_at    timestamptz,
    leased_at        timestamptz,
    lease_owner      text,
    error_code       text,
    error_message    text,
    raw_result       jsonb,
    normalized_result jsonb,
    result_version   integer     NOT NULL DEFAULT 1,
    retention_until  timestamptz,
    requested_by     bigint      NOT NULL REFERENCES users(id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    started_at       timestamptz,
    finished_at      timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ocr_jobs_document_type_check CHECK (document_type IN ('business_license', 'id_card', 'food_permit', 'health_cert')),
    CONSTRAINT ocr_jobs_owner_type_check CHECK (owner_type IN ('merchant_application', 'operator_application', 'rider_application', 'group_application')),
    CONSTRAINT ocr_jobs_side_check CHECK (side IN ('', 'front', 'back')),
    CONSTRAINT ocr_jobs_status_check CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT ocr_jobs_attempt_count_check CHECK (attempt_count >= 0),
    CONSTRAINT ocr_jobs_max_attempts_check CHECK (max_attempts > 0),
    CONSTRAINT ocr_jobs_result_version_check CHECK (result_version > 0)
);

CREATE UNIQUE INDEX idx_ocr_jobs_idempotency_key ON ocr_jobs (idempotency_key);
CREATE INDEX idx_ocr_jobs_owner_created_at ON ocr_jobs (owner_type, owner_id, created_at DESC);
CREATE INDEX idx_ocr_jobs_status_next_retry_at ON ocr_jobs (status, next_retry_at, created_at);
CREATE INDEX idx_ocr_jobs_media_asset_id ON ocr_jobs (media_asset_id);

COMMENT ON TABLE ocr_jobs IS '统一 OCR 任务表';
COMMENT ON COLUMN ocr_jobs.idempotency_key IS '由 media_asset_id + document_type + owner_type + owner_id + side 组成的幂等键';
COMMENT ON COLUMN ocr_jobs.document_type IS '证件类型：business_license | id_card | food_permit | health_cert';
COMMENT ON COLUMN ocr_jobs.owner_type IS '业务主体类型：merchant_application | operator_application | rider_application | group_application';
COMMENT ON COLUMN ocr_jobs.side IS '证件面：front | back，非双面证件为空字符串';
COMMENT ON COLUMN ocr_jobs.raw_result IS '供应商原始 OCR 结果';
COMMENT ON COLUMN ocr_jobs.normalized_result IS '统一归一化后的 OCR 结果';