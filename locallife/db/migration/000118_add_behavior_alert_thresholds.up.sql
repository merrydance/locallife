-- Phase3: seed alert thresholds for abnormal stats

INSERT INTO platform_configs (config_key, config_value, scope_type, scope_id)
VALUES (
  'behavior_trace.alert_thresholds',
  '{"user_rate_30d":0.35,"merchant_rate_30d":0.12,"rider_rate_30d":0.10,"min_claims_30d":5,"limit":100}',
  'global',
  NULL
)
ON CONFLICT (config_key, scope_type, scope_id)
DO UPDATE SET config_value = EXCLUDED.config_value, updated_at = NOW();
