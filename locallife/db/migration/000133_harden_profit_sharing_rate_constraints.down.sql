ALTER TABLE profit_sharing_configs
    DROP CONSTRAINT IF EXISTS profit_sharing_configs_platform_rate_check,
    DROP CONSTRAINT IF EXISTS profit_sharing_configs_operator_rate_check,
    DROP CONSTRAINT IF EXISTS profit_sharing_configs_total_rate_check;
