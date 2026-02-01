ALTER TABLE regions ADD COLUMN status varchar(20) NOT NULL DEFAULT 'active';
ALTER TABLE operators ADD COLUMN balance bigint NOT NULL DEFAULT 0 CHECK (balance >= 0);
ALTER TABLE operators ADD COLUMN wallet_account jsonb NOT NULL DEFAULT '{}';

CREATE TABLE withdrawal_records (
    id bigserial PRIMARY KEY,
    user_id bigint NOT NULL,
    amount bigint NOT NULL, -- 单位：分
    status varchar(20) NOT NULL DEFAULT 'pending', -- pending, approved, rejected, completed
    channel varchar(50) NOT NULL DEFAULT 'wechat', -- 提现渠道
    account_info jsonb NOT NULL DEFAULT '{}'::jsonb, -- 提现账号信息
    reason text, -- 拒绝原因等
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_withdrawal_records_user_id ON withdrawal_records(user_id);
CREATE INDEX idx_withdrawal_records_status ON withdrawal_records(status);

CREATE TABLE safety_reports (
    id bigserial PRIMARY KEY,
    reporter_id bigint NOT NULL, -- 上报人ID (运营商操作员)
    region_id bigint NOT NULL, -- 关联区域
    title varchar(255) NOT NULL,
    description text NOT NULL,
    level varchar(20) NOT NULL, -- low, medium, high, critical
    merchant_ids bigint[] DEFAULT NULL, -- 涉及商户ID列表
    images text[] DEFAULT NULL,
    status varchar(20) NOT NULL DEFAULT 'pending', -- pending, processing, resolved, dismissed
    resolution_notes text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_safety_reports_region_id ON safety_reports(region_id);
CREATE INDEX idx_safety_reports_status ON safety_reports(status);
