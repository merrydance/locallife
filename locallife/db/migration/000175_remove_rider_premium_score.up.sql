DROP TABLE IF EXISTS rider_premium_score_logs;

ALTER TABLE rider_profiles
DROP COLUMN IF EXISTS premium_score;