CREATE TABLE delivery_timeout_alerts (
  id BIGSERIAL PRIMARY KEY,
  delivery_id BIGINT NOT NULL REFERENCES deliveries(id) ON DELETE CASCADE,
  alert_key TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (delivery_id, alert_key)
);

CREATE INDEX idx_delivery_timeout_alerts_created_at ON delivery_timeout_alerts (created_at DESC, id DESC);

COMMENT ON TABLE delivery_timeout_alerts IS '配送超时提醒去重真值，避免调度器重复下发同一配送单的同一阈值提醒';
COMMENT ON COLUMN delivery_timeout_alerts.alert_key IS '提醒类型键，例如 pending_dispatch_3m';