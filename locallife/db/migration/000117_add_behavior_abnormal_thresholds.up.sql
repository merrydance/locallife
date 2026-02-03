-- Phase3: seed abnormal thresholds config

INSERT INTO platform_configs (config_key, config_value, scope_type, scope_id)
VALUES (
  'behavior_trace.abnormal_thresholds',
  '{"user_claim_rate_7d":0.3,"user_claim_rate_30d":0.2,"user_claims_7d":3,"user_claims_30d":5,"merchant_abnormal_rate_30d":0.08,"rider_abnormal_rate_30d":0.06}',
  'global',
  NULL
)
ON CONFLICT (config_key, scope_type, scope_id)
DO UPDATE SET config_value = EXCLUDED.config_value, updated_at = NOW();
