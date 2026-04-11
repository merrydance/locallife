ALTER TABLE profit_sharing_configs
    DROP CONSTRAINT IF EXISTS profit_sharing_configs_platform_rate_check,
    DROP CONSTRAINT IF EXISTS profit_sharing_configs_operator_rate_check,
    DROP CONSTRAINT IF EXISTS profit_sharing_configs_total_rate_check;

ALTER TABLE profit_sharing_configs
    ADD CONSTRAINT profit_sharing_configs_platform_rate_check
        CHECK (platform_rate >= 0 AND platform_rate <= 100),
    ADD CONSTRAINT profit_sharing_configs_operator_rate_check
        CHECK (operator_rate >= 0 AND operator_rate <= 100),
    ADD CONSTRAINT profit_sharing_configs_total_rate_check
        CHECK (platform_rate + operator_rate <= 100);
