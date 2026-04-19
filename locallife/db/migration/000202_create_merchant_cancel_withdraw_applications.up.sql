CREATE TABLE merchant_cancel_withdraw_applications (
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
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX merchant_cancel_withdraw_applications_applyment_id_uidx
ON merchant_cancel_withdraw_applications(applyment_id)
WHERE applyment_id IS NOT NULL;

CREATE INDEX merchant_cancel_withdraw_applications_merchant_idx
ON merchant_cancel_withdraw_applications(merchant_id, created_at DESC);

CREATE INDEX merchant_cancel_withdraw_applications_pending_idx
ON merchant_cancel_withdraw_applications(local_sync_state, cancel_state, created_at ASC);

COMMENT ON TABLE merchant_cancel_withdraw_applications IS '微信支付商户注销提现申请单';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.out_request_no IS '平台侧商户注销申请单号';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.applyment_id IS '微信支付注销提现申请单号';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.withdraw IS '是否提取资金：NOT_APPLY_WITHDRAW/APPLY_WITHDRAW';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.local_sync_state IS '本地提交同步状态：created/submit_succeeded/submit_unknown/sync_failed';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.cancel_state IS '微信支付注销状态 cancel_state';
COMMENT ON COLUMN merchant_cancel_withdraw_applications.withdraw_state IS '微信支付提现状态 withdraw_state';