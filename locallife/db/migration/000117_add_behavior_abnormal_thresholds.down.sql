DELETE FROM platform_configs
WHERE config_key = 'behavior_trace.abnormal_thresholds'
  AND scope_type = 'global'
  AND scope_id IS NULL;
