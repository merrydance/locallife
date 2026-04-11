-- Seed behavior trace platform configs

INSERT INTO platform_configs (config_key, config_value, scope_type, scope_id)
VALUES
  ('behavior_trace.thresholds', '{"claims_7d":3,"claims_30d":5,"rate_7d":0.3,"rate_30d":0.2,"fallback_30d_if_7d_one":true}', 'global', NULL),
  ('behavior_trace.reject_service_cooldown_days', '{"days":14}', 'global', NULL),
  ('behavior_trace.device_reuse', '{"window_days":7,"min_users":3,"min_claims":3}', 'global', NULL),
  ('behavior_trace.address_cluster', '{"window_days":7,"min_users":3,"min_claims":3}', 'global', NULL),
  ('behavior_trace.coordinated_claims', '{"window_minutes":60,"min_users":3}', 'global', NULL),
  ('behavior_trace.backoffice_remind_frequency_hours', '{"hours":24}', 'global', NULL)
ON CONFLICT (config_key, scope_type, scope_id)
DO UPDATE SET config_value = EXCLUDED.config_value, updated_at = NOW();
