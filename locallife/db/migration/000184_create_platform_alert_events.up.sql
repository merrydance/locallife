CREATE TABLE platform_alert_events (
  id BIGSERIAL PRIMARY KEY,
  alert_type TEXT NOT NULL,
  level TEXT NOT NULL,
  title TEXT NOT NULL,
  message TEXT NOT NULL,
  related_id BIGINT NOT NULL DEFAULT 0,
  related_type TEXT NOT NULL,
  extra JSONB NOT NULL DEFAULT '{}'::jsonb,
  emitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_platform_alert_events_emitted_at ON platform_alert_events (emitted_at DESC, id DESC);
CREATE INDEX idx_platform_alert_events_alert_type ON platform_alert_events (alert_type);
CREATE INDEX idx_platform_alert_events_related ON platform_alert_events (related_type, related_id);

COMMENT ON TABLE platform_alert_events IS '平台运营告警历史，用于离线回看与控制台首屏恢复';
COMMENT ON COLUMN platform_alert_events.extra IS '告警扩展字段，前端按需渲染';
