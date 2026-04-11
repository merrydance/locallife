-- Phase2: 分账规则配置表（草案）

CREATE TABLE IF NOT EXISTS profit_sharing_configs (
  id BIGSERIAL PRIMARY KEY,
  status TEXT NOT NULL DEFAULT 'active', -- active/disabled
  order_source TEXT NOT NULL DEFAULT 'takeout', -- takeout/dine_in/takeaway/reservation/all
  region_id BIGINT, -- 可空，表示全局
  merchant_id BIGINT, -- 可空，表示全局
  platform_rate INT NOT NULL DEFAULT 2, -- 平台分成百分比
  operator_rate INT NOT NULL DEFAULT 3, -- 运营商分成百分比
  rider_enabled BOOLEAN NOT NULL DEFAULT true, -- 是否启用骑手分成
  priority INT NOT NULL DEFAULT 100,
  effective_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  created_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_profit_sharing_configs_status ON profit_sharing_configs(status);
CREATE INDEX IF NOT EXISTS idx_profit_sharing_configs_order_source ON profit_sharing_configs(order_source);
CREATE INDEX IF NOT EXISTS idx_profit_sharing_configs_region_id ON profit_sharing_configs(region_id);
CREATE INDEX IF NOT EXISTS idx_profit_sharing_configs_merchant_id ON profit_sharing_configs(merchant_id);
CREATE INDEX IF NOT EXISTS idx_profit_sharing_configs_priority ON profit_sharing_configs(priority);

COMMENT ON TABLE profit_sharing_configs IS '分账规则配置表（Phase2 草案）';
