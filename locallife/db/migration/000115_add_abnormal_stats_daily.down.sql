DROP TRIGGER IF EXISTS trg_claims_abnormal_stats_insert ON claims;
DROP FUNCTION IF EXISTS trg_claims_abnormal_stats;

DROP TRIGGER IF EXISTS trg_deliveries_abnormal_stats_update ON deliveries;
DROP FUNCTION IF EXISTS trg_deliveries_abnormal_stats;

DROP TRIGGER IF EXISTS trg_orders_abnormal_stats_update ON orders;
DROP TRIGGER IF EXISTS trg_orders_abnormal_stats_insert ON orders;
DROP FUNCTION IF EXISTS trg_orders_abnormal_stats;

DROP FUNCTION IF EXISTS upsert_abnormal_stats_daily;

DROP INDEX IF EXISTS idx_abnormal_stats_daily_entity;
DROP TABLE IF EXISTS abnormal_stats_daily;
