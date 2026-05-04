CREATE TABLE IF NOT EXISTS baofu_merchant_reports (
    id BIGSERIAL PRIMARY KEY,
    owner_type TEXT NOT NULL CHECK (owner_type = 'merchant'),
    owner_id BIGINT NOT NULL,
    report_type TEXT NOT NULL CHECK (report_type IN ('WECHAT','ALIPAY')),
    report_no TEXT NOT NULL UNIQUE,
    bct_mer_id TEXT NOT NULL,
    sub_mch_id TEXT,
    report_state TEXT NOT NULL CHECK (report_state IN ('processing','succeeded','failed','unknown')),
    applet_auth_state TEXT NOT NULL DEFAULT 'pending' CHECK (applet_auth_state IN ('pending','succeeded','failed','not_required')),
    platform_biz_no TEXT,
    failure_code TEXT,
    failure_message TEXT,
    raw_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_type, owner_id, report_type)
);

CREATE INDEX IF NOT EXISTS idx_baofu_merchant_reports_state
    ON baofu_merchant_reports(report_state, updated_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS idx_baofu_merchant_reports_applet_auth_state
    ON baofu_merchant_reports(applet_auth_state, updated_at ASC, id ASC);
