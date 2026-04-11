-- Rollback Phase3 abnormal stats updates

DROP TRIGGER IF EXISTS trg_claims_abnormal_stats_update ON claims;
DROP FUNCTION IF EXISTS trg_claims_abnormal_stats_update;

-- Restore claims abnormal stats trigger to count all claims
CREATE OR REPLACE FUNCTION trg_claims_abnormal_stats() RETURNS TRIGGER AS $$
DECLARE
    v_stat_date DATE;
    v_order_user_id BIGINT;
    v_order_merchant_id BIGINT;
    v_rider_id BIGINT;
BEGIN
    v_stat_date := NEW.created_at::DATE;

    SELECT user_id, merchant_id INTO v_order_user_id, v_order_merchant_id
    FROM orders
    WHERE id = NEW.order_id;

    PERFORM upsert_abnormal_stats_daily(v_stat_date, 'user', v_order_user_id, 0, 1);
    PERFORM upsert_abnormal_stats_daily(v_stat_date, 'merchant', v_order_merchant_id, 0, 1);

    SELECT rider_id INTO v_rider_id
    FROM deliveries
    WHERE order_id = NEW.order_id;

    IF v_rider_id IS NOT NULL THEN
        PERFORM upsert_abnormal_stats_daily(v_stat_date, 'rider', v_rider_id, 0, 1);
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_claims_abnormal_stats_insert ON claims;
CREATE TRIGGER trg_claims_abnormal_stats_insert
AFTER INSERT ON claims
FOR EACH ROW
EXECUTE FUNCTION trg_claims_abnormal_stats();

DELETE FROM platform_configs
WHERE config_key = 'behavior_trace.window_days' AND scope_type = 'global' AND scope_id IS NULL;
