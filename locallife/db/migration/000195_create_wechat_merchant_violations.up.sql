-- 微信支付商户违规记录表
-- 对应微信支付平台收付通违规通知回调：/v3/merchant-risk-manage/violation-notifications

CREATE TABLE wechat_merchant_violations (
    id                      BIGSERIAL PRIMARY KEY,
    record_id               TEXT NOT NULL UNIQUE,
    sub_mch_id              TEXT NOT NULL,
    merchant_id             BIGINT REFERENCES merchants(id) ON DELETE SET NULL,
    company_name            TEXT NOT NULL DEFAULT '',
    event_type              TEXT NOT NULL
                                CHECK (event_type IN (
                                    'VIOLATION.PUNISH',
                                    'VIOLATION.INTERCEPT',
                                    'VIOLATION.APPEAL'
                                )),
    risk_type               TEXT NOT NULL DEFAULT '',
    risk_description        TEXT NOT NULL DEFAULT '',
    punish_plan             TEXT NOT NULL DEFAULT '',
    punish_time             TIMESTAMPTZ,
    punish_description      TEXT NOT NULL DEFAULT '',
    latest_notification_id  TEXT NOT NULL,
    latest_notify_time      TIMESTAMPTZ NOT NULL,
    last_received_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ
);

CREATE INDEX idx_wechat_merchant_violations_sub_mch_id
    ON wechat_merchant_violations(sub_mch_id);
CREATE INDEX idx_wechat_merchant_violations_merchant_id
    ON wechat_merchant_violations(merchant_id);
CREATE INDEX idx_wechat_merchant_violations_event_type
    ON wechat_merchant_violations(event_type);
CREATE INDEX idx_wechat_merchant_violations_risk_type
    ON wechat_merchant_violations(risk_type);
CREATE INDEX idx_wechat_merchant_violations_latest_notify_time
    ON wechat_merchant_violations(latest_notify_time DESC);
CREATE INDEX idx_wechat_merchant_violations_punish_time
    ON wechat_merchant_violations(punish_time DESC NULLS LAST);

COMMENT ON TABLE wechat_merchant_violations IS '微信支付平台收付通商户违规通知记录；由违规 webhook 持久化，供平台审计与运营处理';