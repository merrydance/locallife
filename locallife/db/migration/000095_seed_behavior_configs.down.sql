DELETE FROM platform_configs
WHERE config_key IN (
  'behavior_trace.thresholds',
  'behavior_trace.reject_service_cooldown_days',
  'behavior_trace.device_reuse',
  'behavior_trace.address_cluster',
  'behavior_trace.coordinated_claims',
  'behavior_trace.backoffice_remind_frequency_hours'
);