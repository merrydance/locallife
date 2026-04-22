ALTER TABLE merchant_applications
ADD COLUMN review_summary jsonb;

ALTER TABLE rider_applications
ADD COLUMN review_summary jsonb;

COMMENT ON COLUMN merchant_applications.review_summary IS '最近一次商户入驻审核摘要 JSON';
COMMENT ON COLUMN rider_applications.review_summary IS '最近一次骑手入驻审核摘要 JSON';

CREATE TABLE onboarding_review_runs (
    id BIGSERIAL PRIMARY KEY,
    application_type TEXT NOT NULL,
    merchant_application_id BIGINT REFERENCES merchant_applications(id) ON DELETE CASCADE,
    rider_application_id BIGINT REFERENCES rider_applications(id) ON DELETE CASCADE,
    run_status TEXT NOT NULL DEFAULT 'queued',
    stage TEXT NOT NULL DEFAULT 'review',
    outcome TEXT,
    reason_code TEXT,
    reason_message TEXT,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    rule_hits TEXT[] NOT NULL DEFAULT '{}',
    ocr_job_refs BIGINT[] NOT NULL DEFAULT '{}',
    snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    requested_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    queued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT onboarding_review_runs_application_scope_check CHECK (
        (application_type = 'merchant' AND merchant_application_id IS NOT NULL AND rider_application_id IS NULL)
        OR
        (application_type = 'rider' AND rider_application_id IS NOT NULL AND merchant_application_id IS NULL)
    ),
    CONSTRAINT onboarding_review_runs_status_check CHECK (run_status IN ('queued', 'processing', 'completed', 'cancelled')),
    CONSTRAINT onboarding_review_runs_stage_check CHECK (stage IN ('ocr', 'normalization', 'review', 'risk', 'manual')),
    CONSTRAINT onboarding_review_runs_outcome_check CHECK (outcome IS NULL OR outcome IN ('approved', 'rejected', 'needs_resubmit', 'needs_manual')),
    CONSTRAINT onboarding_review_runs_completed_outcome_required CHECK (run_status <> 'completed' OR outcome IS NOT NULL),
    CONSTRAINT onboarding_review_runs_completed_reason_code_required CHECK (run_status <> 'completed' OR reason_code IS NOT NULL)
);

CREATE INDEX idx_onboarding_review_runs_status_queued_at
    ON onboarding_review_runs (run_status, queued_at, id);

CREATE INDEX idx_onboarding_review_runs_merchant_application_created_at
    ON onboarding_review_runs (merchant_application_id, created_at DESC, id DESC)
    WHERE merchant_application_id IS NOT NULL;

CREATE INDEX idx_onboarding_review_runs_rider_application_created_at
    ON onboarding_review_runs (rider_application_id, created_at DESC, id DESC)
    WHERE rider_application_id IS NOT NULL;

COMMENT ON TABLE onboarding_review_runs IS '商户与骑手入驻自动审核运行快照表';
COMMENT ON COLUMN onboarding_review_runs.application_type IS '申请类型：merchant | rider';
COMMENT ON COLUMN onboarding_review_runs.run_status IS 'queued=待处理, processing=处理中, completed=已完成, cancelled=已取消';
COMMENT ON COLUMN onboarding_review_runs.stage IS '完成或停留阶段：ocr | normalization | review | risk | manual';
COMMENT ON COLUMN onboarding_review_runs.outcome IS '审核结果：approved | rejected | needs_resubmit | needs_manual';
COMMENT ON COLUMN onboarding_review_runs.evidence IS '结构化证据摘要 JSON';
COMMENT ON COLUMN onboarding_review_runs.snapshot IS '执行审核时使用的业务快照 JSON';

CREATE TABLE credential_ledgers (
    id BIGSERIAL PRIMARY KEY,
    subject_type TEXT NOT NULL,
    merchant_id BIGINT REFERENCES merchants(id) ON DELETE CASCADE,
    rider_id BIGINT REFERENCES riders(id) ON DELETE CASCADE,
    document_type TEXT NOT NULL,
    merchant_application_id BIGINT REFERENCES merchant_applications(id) ON DELETE SET NULL,
    rider_application_id BIGINT REFERENCES rider_applications(id) ON DELETE SET NULL,
    review_run_id BIGINT REFERENCES onboarding_review_runs(id) ON DELETE SET NULL,
    media_asset_id BIGINT NOT NULL REFERENCES media_assets(id) ON DELETE RESTRICT,
    normalized_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ,
    active BOOLEAN NOT NULL DEFAULT true,
    activated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deactivated_at TIMESTAMPTZ,
    last_reminded_at TIMESTAMPTZ,
    suspended_at TIMESTAMPTZ,
    resumed_at TIMESTAMPTZ,
    suspension_reason_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT credential_ledgers_subject_scope_check CHECK (
        (subject_type = 'merchant' AND merchant_id IS NOT NULL AND rider_id IS NULL)
        OR
        (subject_type = 'rider' AND rider_id IS NOT NULL AND merchant_id IS NULL)
    ),
    CONSTRAINT credential_ledgers_document_type_check CHECK (document_type IN ('business_license', 'food_permit', 'health_cert')),
    CONSTRAINT credential_ledgers_application_scope_check CHECK (
        (subject_type = 'merchant' AND merchant_application_id IS NOT NULL AND rider_application_id IS NULL)
        OR
        (subject_type = 'rider' AND rider_application_id IS NOT NULL AND merchant_application_id IS NULL)
    ),
    CONSTRAINT credential_ledgers_suspension_reason_code_check CHECK (
        suspension_reason_code IS NULL OR suspension_reason_code = 'document_expired'
    ),
    CONSTRAINT credential_ledgers_deactivated_at_requires_inactive CHECK (
        deactivated_at IS NULL OR active = false
    )
);

CREATE UNIQUE INDEX idx_credential_ledgers_unique_active_merchant_document
    ON credential_ledgers (merchant_id, document_type)
    WHERE active = true AND merchant_id IS NOT NULL;

CREATE UNIQUE INDEX idx_credential_ledgers_unique_active_rider_document
    ON credential_ledgers (rider_id, document_type)
    WHERE active = true AND rider_id IS NOT NULL;

CREATE INDEX idx_credential_ledgers_active_expires_at
    ON credential_ledgers (active, expires_at, id)
    WHERE active = true AND expires_at IS NOT NULL;

CREATE INDEX idx_credential_ledgers_merchant_active_document
    ON credential_ledgers (merchant_id, active, document_type)
    WHERE merchant_id IS NOT NULL;

CREATE INDEX idx_credential_ledgers_rider_active_document
    ON credential_ledgers (rider_id, active, document_type)
    WHERE rider_id IS NOT NULL;

COMMENT ON TABLE credential_ledgers IS '入驻通过后的线上生效证照台账';
COMMENT ON COLUMN credential_ledgers.subject_type IS '主体类型：merchant | rider';
COMMENT ON COLUMN credential_ledgers.document_type IS '持续治理证照类型：business_license | food_permit | health_cert';
COMMENT ON COLUMN credential_ledgers.normalized_payload IS '证照归一化摘要 JSON，用于展示与审计';
COMMENT ON COLUMN credential_ledgers.expires_at IS '证照到期时间，长期有效证照允许为空';
COMMENT ON COLUMN credential_ledgers.suspension_reason_code IS '当前仅允许 document_expired，用于 owned release';