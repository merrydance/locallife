-- Rollback recreates legacy schema surfaces and loosens channel constraints.
-- Data rewritten or dropped by the up migration is not recoverable from this down migration.

CREATE TABLE IF NOT EXISTS ecommerce_applyments (
    id BIGSERIAL PRIMARY KEY,
    subject_type TEXT NOT NULL,
    subject_id BIGINT NOT NULL,
    out_request_no TEXT NOT NULL UNIQUE,
    applyment_id BIGINT,
    organization_type TEXT NOT NULL,
    business_license_number TEXT,
    business_license_copy TEXT,
    merchant_name TEXT NOT NULL,
    legal_person TEXT NOT NULL,
    id_card_number TEXT NOT NULL,
    id_card_name TEXT NOT NULL,
    id_card_valid_time TEXT NOT NULL,
    id_card_front_copy TEXT NOT NULL,
    id_card_back_copy TEXT NOT NULL,
    account_type TEXT NOT NULL,
    account_bank TEXT NOT NULL,
    bank_address_code TEXT NOT NULL,
    bank_name TEXT,
    account_number TEXT NOT NULL,
    account_name TEXT NOT NULL,
    contact_name TEXT NOT NULL,
    contact_id_card_number TEXT,
    mobile_phone TEXT NOT NULL,
    contact_email TEXT,
    merchant_shortname TEXT NOT NULL,
    qualifications JSONB,
    business_addition_pics TEXT[],
    business_addition_desc TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    sign_url TEXT,
    sign_state TEXT,
    reject_reason TEXT,
    sub_mch_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at TIMESTAMPTZ,
    audited_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    result_task_processed_state TEXT,
    result_task_processed_at TIMESTAMPTZ,
    account_bank_code BIGINT,
    bank_alias TEXT,
    bank_alias_code TEXT,
    bank_branch_id TEXT,
    settlement_verify_first_trade_at TIMESTAMPTZ,
    settlement_verify_last_checked_at TIMESTAMPTZ,
    settlement_verify_check_count INT NOT NULL DEFAULT 0,
    settlement_verify_status TEXT,
    settlement_verify_fail_reason TEXT,
    settlement_verify_failed_notified_at TIMESTAMPTZ,
    legal_validation_url TEXT,
    account_validation JSONB,
    account_willingness_business_code TEXT,
    account_willingness_applyment_id BIGINT,
    account_willingness_state TEXT,
    account_willingness_qrcode_data TEXT,
    account_willingness_reject_reason TEXT,
    account_authorize_state TEXT,
    account_authorize_state_checked_at TIMESTAMPTZ,
    CONSTRAINT ecommerce_applyments_subject_type_check CHECK (subject_type IN ('merchant', 'operator')),
    CONSTRAINT ecommerce_applyments_org_type_check CHECK (organization_type IN ('2401', '2500', '4', '2', '3', '2502', '1708')),
    CONSTRAINT ecommerce_applyments_account_type_check CHECK (account_type IN ('ACCOUNT_TYPE_BUSINESS', 'ACCOUNT_TYPE_PRIVATE')),
    CONSTRAINT ecommerce_applyments_status_check CHECK (status IN (
        'pending',
        'submitted',
        'checking',
        'auditing',
        'account_need_verify',
        'to_be_confirmed',
        'rejected',
        'frozen',
        'to_be_signed',
        'signing',
        'rejected_sign',
        'finish',
        'canceled'
    )),
    CONSTRAINT ecommerce_applyments_settlement_verify_status_check CHECK (
        settlement_verify_status IS NULL OR
        settlement_verify_status IN ('verifying', 'success', 'fail')
    )
);

CREATE INDEX IF NOT EXISTS ecommerce_applyments_subject_idx
    ON ecommerce_applyments(subject_type, subject_id);

CREATE INDEX IF NOT EXISTS ecommerce_applyments_status_idx
    ON ecommerce_applyments(status);

CREATE INDEX IF NOT EXISTS ecommerce_applyments_applyment_id_idx
    ON ecommerce_applyments(applyment_id);

CREATE INDEX IF NOT EXISTS ecommerce_applyments_sub_mch_id_idx
    ON ecommerce_applyments(sub_mch_id);

CREATE INDEX IF NOT EXISTS idx_ecommerce_applyments_result_recovery
    ON ecommerce_applyments (updated_at ASC, id ASC)
    WHERE status IN ('submitted', 'checking', 'auditing', 'account_need_verify', 'to_be_signed', 'to_be_confirmed', 'signing', 'finish', 'rejected');

CREATE INDEX IF NOT EXISTS idx_ecommerce_applyments_settlement_verify
    ON ecommerce_applyments (
        subject_type,
        status,
        settlement_verify_status,
        settlement_verify_last_checked_at,
        id
    )
    WHERE subject_type = 'merchant';

CREATE INDEX IF NOT EXISTS idx_ecommerce_applyments_account_authorize_state
    ON ecommerce_applyments (account_authorize_state, account_authorize_state_checked_at)
    WHERE account_authorize_state IS NOT NULL;

COMMENT ON TABLE ecommerce_applyments IS '微信平台收付通二级商户进件申请';
COMMENT ON COLUMN ecommerce_applyments.subject_type IS '主体类型: merchant-商户, operator-运营商';
COMMENT ON COLUMN ecommerce_applyments.organization_type IS '微信主体类型: 2401-小微商户, 2500-个人卖家, 4-个体工商户, 2-企业, 3-事业单位, 2502-政府机关, 1708-社会组织';
COMMENT ON COLUMN ecommerce_applyments.status IS '进件状态: pending-待提交, submitted-已提交, checking-资料校验中, auditing-审核中, account_need_verify-待账户验证, to_be_confirmed-待确认, rejected-已驳回, frozen-冻结, to_be_signed-待签约, signing-签约中, rejected_sign-签约失败, finish-完成, canceled-已作废';
COMMENT ON COLUMN ecommerce_applyments.account_bank_code IS '微信收付通开户银行编码';
COMMENT ON COLUMN ecommerce_applyments.bank_alias IS '微信收付通银行别名名称';
COMMENT ON COLUMN ecommerce_applyments.bank_alias_code IS '微信收付通银行别名编码';
COMMENT ON COLUMN ecommerce_applyments.bank_branch_id IS '微信收付通支行联行号';
COMMENT ON COLUMN ecommerce_applyments.settlement_verify_first_trade_at IS '商户首笔支付成功时间，用于 0.01 结算卡验卡三日巡检';
COMMENT ON COLUMN ecommerce_applyments.settlement_verify_last_checked_at IS '最近一次查询微信结算卡验卡结果的时间';
COMMENT ON COLUMN ecommerce_applyments.settlement_verify_check_count IS '已执行的每日验卡巡检次数，最多三次';
COMMENT ON COLUMN ecommerce_applyments.settlement_verify_status IS '结算卡验卡巡检状态: verifying-巡检中, success-巡检结束且未发现失败, fail-巡检发现失败';
COMMENT ON COLUMN ecommerce_applyments.settlement_verify_fail_reason IS '微信返回的结算卡验卡失败原因';
COMMENT ON COLUMN ecommerce_applyments.settlement_verify_failed_notified_at IS '运营商已收到结算卡失败通知的时间';
COMMENT ON COLUMN ecommerce_applyments.legal_validation_url IS '微信进件返回的法人扫码验证链接';
COMMENT ON COLUMN ecommerce_applyments.account_validation IS '微信进件返回的汇款账户验证原始信息，敏感字段保持微信返回密文';
COMMENT ON COLUMN ecommerce_applyments.account_willingness_business_code IS '普通服务商开户意愿确认业务申请编号';
COMMENT ON COLUMN ecommerce_applyments.account_willingness_applyment_id IS '普通服务商开户意愿确认微信申请单号';
COMMENT ON COLUMN ecommerce_applyments.account_willingness_state IS '普通服务商开户意愿确认审核状态';
COMMENT ON COLUMN ecommerce_applyments.account_willingness_qrcode_data IS '普通服务商开户意愿确认二维码数据';
COMMENT ON COLUMN ecommerce_applyments.account_willingness_reject_reason IS '普通服务商开户意愿确认驳回原因';
COMMENT ON COLUMN ecommerce_applyments.account_authorize_state IS '普通服务商特约商户开户意愿授权状态';
COMMENT ON COLUMN ecommerce_applyments.account_authorize_state_checked_at IS '最近一次查询普通服务商开户意愿授权状态的时间';

CREATE TABLE IF NOT EXISTS profit_sharing_receiver_targets (
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

CREATE INDEX IF NOT EXISTS idx_profit_sharing_receiver_targets_retry
    ON profit_sharing_receiver_targets (next_retry_at ASC NULLS FIRST, id ASC)
    WHERE sync_status IN ('pending', 'failed');

CREATE INDEX IF NOT EXISTS idx_profit_sharing_receiver_targets_owner
    ON profit_sharing_receiver_targets (owner_type, owner_id, id);

CREATE TABLE IF NOT EXISTS profit_sharing_receiver_attempts (
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

CREATE INDEX IF NOT EXISTS idx_profit_sharing_receiver_attempts_target
    ON profit_sharing_receiver_attempts (target_id, id DESC);

CREATE TABLE IF NOT EXISTS wechat_complaints (
    id BIGSERIAL PRIMARY KEY,
    complaint_id TEXT NOT NULL UNIQUE,
    complaint_time TIMESTAMPTZ NOT NULL,
    payer_openid TEXT,
    complaint_detail TEXT NOT NULL DEFAULT '',
    complaint_state TEXT NOT NULL DEFAULT 'PENDING_RESPONSE'
        CHECK (complaint_state IN ('PENDING_RESPONSE', 'PROCESSING', 'PROCESSED')),
    transaction_id TEXT,
    out_trade_no TEXT,
    sub_mch_id TEXT,
    merchant_id BIGINT REFERENCES merchants(id) ON DELETE SET NULL,
    payer_complaint_full_info BOOLEAN NOT NULL DEFAULT false,
    amount BIGINT NOT NULL DEFAULT 0,
    response_content TEXT,
    responded_at TIMESTAMPTZ,
    media_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    completed_at TIMESTAMPTZ,
    last_synced_at TIMESTAMPTZ,
    wxpay_update_time TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_wechat_complaints_state
    ON wechat_complaints(complaint_state);
CREATE INDEX IF NOT EXISTS idx_wechat_complaints_merchant
    ON wechat_complaints(merchant_id);
CREATE INDEX IF NOT EXISTS idx_wechat_complaints_sub_mch_id
    ON wechat_complaints(sub_mch_id);
CREATE INDEX IF NOT EXISTS idx_wechat_complaints_time
    ON wechat_complaints(complaint_time DESC);
CREATE INDEX IF NOT EXISTS idx_wechat_complaints_out_trade
    ON wechat_complaints(out_trade_no)
    WHERE out_trade_no IS NOT NULL;

COMMENT ON TABLE wechat_complaints IS '微信支付用户投诉记录；由每日同步任务从微信拉取，商户可通过平台回复并完结';

CREATE TABLE IF NOT EXISTS subsidy_orders (
    id BIGSERIAL PRIMARY KEY,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id),
    sub_mch_id TEXT NOT NULL,
    transaction_id TEXT,
    out_subsidy_no TEXT NOT NULL UNIQUE,
    payer_amount BIGINT NOT NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'success', 'failed', 'canceled')),
    wxpay_subsidy_id TEXT,
    fail_reason TEXT,
    out_return_no TEXT UNIQUE,
    return_amount BIGINT,
    return_status TEXT CHECK (return_status IN ('pending_return', 'return_success', 'return_failed')),
    return_wxpay_id TEXT,
    return_fail_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_subsidy_orders_payment_order
    ON subsidy_orders(payment_order_id);
CREATE INDEX IF NOT EXISTS idx_subsidy_orders_status
    ON subsidy_orders(status);
CREATE INDEX IF NOT EXISTS idx_subsidy_orders_sub_mch_id
    ON subsidy_orders(sub_mch_id);
CREATE INDEX IF NOT EXISTS idx_subsidy_orders_created_at
    ON subsidy_orders(created_at DESC);

COMMENT ON TABLE subsidy_orders IS '平台收付通补差订单；平台出资补贴给二级商户，对应微信支付 /v3/ecommerce/subsidies/create';

CREATE TABLE IF NOT EXISTS wechat_merchant_violations (
    id BIGSERIAL PRIMARY KEY,
    record_id TEXT NOT NULL UNIQUE,
    sub_mch_id TEXT NOT NULL,
    merchant_id BIGINT REFERENCES merchants(id) ON DELETE SET NULL,
    company_name TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL
        CHECK (event_type IN ('VIOLATION.PUNISH', 'VIOLATION.INTERCEPT', 'VIOLATION.APPEAL')),
    risk_type TEXT NOT NULL DEFAULT '',
    risk_description TEXT NOT NULL DEFAULT '',
    punish_plan TEXT NOT NULL DEFAULT '',
    punish_time TIMESTAMPTZ,
    punish_description TEXT NOT NULL DEFAULT '',
    latest_notification_id TEXT NOT NULL,
    latest_notify_time TIMESTAMPTZ NOT NULL,
    last_received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_wechat_merchant_violations_sub_mch_id
    ON wechat_merchant_violations(sub_mch_id);
CREATE INDEX IF NOT EXISTS idx_wechat_merchant_violations_merchant_id
    ON wechat_merchant_violations(merchant_id);
CREATE INDEX IF NOT EXISTS idx_wechat_merchant_violations_event_type
    ON wechat_merchant_violations(event_type);
CREATE INDEX IF NOT EXISTS idx_wechat_merchant_violations_risk_type
    ON wechat_merchant_violations(risk_type);
CREATE INDEX IF NOT EXISTS idx_wechat_merchant_violations_latest_notify_time
    ON wechat_merchant_violations(latest_notify_time DESC);
CREATE INDEX IF NOT EXISTS idx_wechat_merchant_violations_punish_time
    ON wechat_merchant_violations(punish_time DESC NULLS LAST);

COMMENT ON TABLE wechat_merchant_violations IS '微信支付商户处置通知记录；平台收付通与普通服务商 webhook 共用，按微信 record_id 幂等持久化，供平台审计与运营处理';

CREATE TABLE IF NOT EXISTS merchant_cancel_withdraw_applications (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    sub_mch_id VARCHAR(32) NOT NULL,
    out_request_no VARCHAR(32) NOT NULL UNIQUE,
    applyment_id VARCHAR(32),
    withdraw VARCHAR(32) NOT NULL,
    proof_media_asset_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    additional_material_asset_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    remark VARCHAR(32),
    local_sync_state VARCHAR(32) NOT NULL,
    cancel_state VARCHAR(64),
    cancel_state_description TEXT,
    withdraw_state VARCHAR(64),
    withdraw_state_description TEXT,
    confirm_cancel_url TEXT,
    account_info JSONB,
    account_withdraw_result JSONB,
    latest_query_response JSONB,
    last_error TEXT,
    modify_time TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,
    last_query_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    business_license_status_declaration VARCHAR(32),
    CONSTRAINT merchant_cancel_withdraw_applications_license_status_declaration_check
        CHECK (
            business_license_status_declaration IS NULL OR
            business_license_status_declaration IN ('ACTIVE', 'CANCELED', 'REVOKED')
        )
);

CREATE UNIQUE INDEX IF NOT EXISTS merchant_cancel_withdraw_applications_applyment_id_uidx
    ON merchant_cancel_withdraw_applications(applyment_id)
    WHERE applyment_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS merchant_cancel_withdraw_applications_merchant_idx
    ON merchant_cancel_withdraw_applications(merchant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS merchant_cancel_withdraw_applications_pending_idx
    ON merchant_cancel_withdraw_applications(local_sync_state, cancel_state, created_at ASC);

COMMENT ON TABLE merchant_cancel_withdraw_applications IS '微信支付商户注销提现申请单';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.out_request_no IS '平台侧商户注销申请单号';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.applyment_id IS '微信支付注销提现申请单号';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.withdraw IS '是否提取资金：NOT_APPLY_WITHDRAW/APPLY_WITHDRAW';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.local_sync_state IS '本地提交同步状态：created/submit_succeeded/submit_unknown/sync_failed';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.cancel_state IS '微信支付注销状态 cancel_state';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.withdraw_state IS '微信支付提现状态 withdraw_state';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.business_license_status_declaration IS '商户声明的营业执照状态：ACTIVE/CANCELED/REVOKED；仅用于企业主体注销提现材料校验';

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_channel_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_channel_check CHECK (channel IN ('ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

ALTER TABLE external_payment_facts
    DROP CONSTRAINT IF EXISTS external_payment_facts_channel_check;

ALTER TABLE external_payment_facts
    ADD CONSTRAINT external_payment_facts_channel_check
    CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

ALTER TABLE external_payment_commands
    DROP CONSTRAINT IF EXISTS external_payment_commands_channel_check;

ALTER TABLE external_payment_commands
    ADD CONSTRAINT external_payment_commands_channel_check
    CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

ALTER TABLE payment_orders
    DROP CONSTRAINT IF EXISTS payment_orders_payment_channel_check;

ALTER TABLE payment_orders
    ADD CONSTRAINT payment_orders_payment_channel_check
    CHECK (payment_channel IN ('direct', 'ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));
