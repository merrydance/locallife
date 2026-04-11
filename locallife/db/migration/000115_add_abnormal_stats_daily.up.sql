-- Phase3: Abnormal stats daily aggregation for claims and orders

CREATE TABLE IF NOT EXISTS abnormal_stats_daily (
    id BIGSERIAL PRIMARY KEY,
    stat_date DATE NOT NULL,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('user', 'merchant', 'rider')),
    entity_id BIGINT NOT NULL,
    total_orders INTEGER NOT NULL DEFAULT 0,
    abnormal_claims INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_abnormal_stats_daily UNIQUE (stat_date, entity_type, entity_id)
);

CREATE INDEX idx_abnormal_stats_daily_entity ON abnormal_stats_daily(entity_type, entity_id, stat_date);

-- helper: upsert daily stats
CREATE OR REPLACE FUNCTION upsert_abnormal_stats_daily(
    p_stat_date DATE,
    p_entity_type TEXT,
    p_entity_id BIGINT,
    p_total_orders_delta INTEGER,
    p_abnormal_claims_delta INTEGER
) RETURNS VOID AS $$
BEGIN
    IF p_entity_id IS NULL OR p_entity_id <= 0 THEN
        RETURN;
    END IF;

    INSERT INTO abnormal_stats_daily (
        stat_date,
        entity_type,
        entity_id,
        total_orders,
        abnormal_claims,
        updated_at
    ) VALUES (
        p_stat_date,
        p_entity_type,
        p_entity_id,
        GREATEST(p_total_orders_delta, 0),
        GREATEST(p_abnormal_claims_delta, 0),
        NOW()
    )
    ON CONFLICT (stat_date, entity_type, entity_id)
    DO UPDATE SET
        total_orders = abnormal_stats_daily.total_orders + EXCLUDED.total_orders,
        abnormal_claims = abnormal_stats_daily.abnormal_claims + EXCLUDED.abnormal_claims,
        updated_at = NOW();
END;
$$ LANGUAGE plpgsql;

-- Orders: count completed orders for user/merchant
CREATE OR REPLACE FUNCTION trg_orders_abnormal_stats() RETURNS TRIGGER AS $$
DECLARE
    v_stat_date DATE;
BEGIN
    IF NEW.status = 'completed' THEN
        v_stat_date := COALESCE(NEW.completed_at, NEW.created_at)::DATE;
        PERFORM upsert_abnormal_stats_daily(v_stat_date, 'user', NEW.user_id, 1, 0);
        PERFORM upsert_abnormal_stats_daily(v_stat_date, 'merchant', NEW.merchant_id, 1, 0);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_orders_abnormal_stats_insert ON orders;
CREATE TRIGGER trg_orders_abnormal_stats_insert
AFTER INSERT ON orders
FOR EACH ROW
WHEN (NEW.status = 'completed')
EXECUTE FUNCTION trg_orders_abnormal_stats();

DROP TRIGGER IF EXISTS trg_orders_abnormal_stats_update ON orders;
CREATE TRIGGER trg_orders_abnormal_stats_update
AFTER UPDATE OF status ON orders
FOR EACH ROW
WHEN (NEW.status = 'completed' AND OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION trg_orders_abnormal_stats();

-- Deliveries: count completed deliveries for rider
CREATE OR REPLACE FUNCTION trg_deliveries_abnormal_stats() RETURNS TRIGGER AS $$
DECLARE
    v_stat_date DATE;
BEGIN
    IF NEW.status = 'completed' THEN
        v_stat_date := COALESCE(NEW.completed_at, NEW.delivered_at, NEW.created_at)::DATE;
        PERFORM upsert_abnormal_stats_daily(v_stat_date, 'rider', NEW.rider_id, 1, 0);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_deliveries_abnormal_stats_update ON deliveries;
CREATE TRIGGER trg_deliveries_abnormal_stats_update
AFTER UPDATE OF status ON deliveries
FOR EACH ROW
WHEN (NEW.status = 'completed' AND NEW.rider_id IS NOT NULL AND OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION trg_deliveries_abnormal_stats();

-- Claims: count abnormal claims for user/merchant/rider
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

DROP TRIGGER IF EXISTS trg_claims_abnormal_stats_insert ON claims;
CREATE TRIGGER trg_claims_abnormal_stats_insert
AFTER INSERT ON claims
FOR EACH ROW
EXECUTE FUNCTION trg_claims_abnormal_stats();

-- When manual review becomes approved, count it
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

DROP TRIGGER IF EXISTS trg_claims_abnormal_stats_update ON claims;
CREATE TRIGGER trg_claims_abnormal_stats_update
AFTER UPDATE OF status ON claims
FOR EACH ROW
EXECUTE FUNCTION trg_claims_abnormal_stats_update();
