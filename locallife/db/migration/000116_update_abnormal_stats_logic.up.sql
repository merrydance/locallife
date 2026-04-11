-- Phase3: update abnormal stats triggers and behavior window config

-- Update claims abnormal stats trigger to count only approved/auto-approved
CREATE OR REPLACE FUNCTION trg_claims_abnormal_stats() RETURNS TRIGGER AS $$
DECLARE
    v_stat_date DATE;
    v_order_user_id BIGINT;
    v_order_merchant_id BIGINT;
    v_rider_id BIGINT;
BEGIN
    IF NEW.status NOT IN ('auto-approved', 'approved') THEN
        RETURN NEW;
    END IF;

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

CREATE OR REPLACE FUNCTION trg_claims_abnormal_stats_update() RETURNS TRIGGER AS $$
DECLARE
    v_stat_date DATE;
    v_order_user_id BIGINT;
    v_order_merchant_id BIGINT;
    v_rider_id BIGINT;
BEGIN
    IF NEW.status NOT IN ('auto-approved', 'approved') THEN
        RETURN NEW;
    END IF;

    IF OLD.status IS DISTINCT FROM NEW.status THEN
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
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_claims_abnormal_stats_insert ON claims;
CREATE TRIGGER trg_claims_abnormal_stats_insert
AFTER INSERT ON claims
FOR EACH ROW
EXECUTE FUNCTION trg_claims_abnormal_stats();

DROP TRIGGER IF EXISTS trg_claims_abnormal_stats_update ON claims;
CREATE TRIGGER trg_claims_abnormal_stats_update
AFTER UPDATE OF status ON claims
FOR EACH ROW
EXECUTE FUNCTION trg_claims_abnormal_stats_update();

-- Seed behavior window config (upsert)
INSERT INTO platform_configs (config_key, config_value, scope_type, scope_id)
VALUES ('behavior_trace.window_days', '{"window_7d":7,"window_30d":30}', 'global', NULL)
ON CONFLICT (config_key, scope_type, scope_id)
DO UPDATE SET config_value = EXCLUDED.config_value, updated_at = NOW();
