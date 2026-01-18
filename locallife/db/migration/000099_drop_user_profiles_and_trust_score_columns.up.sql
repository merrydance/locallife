-- Remove TrustScore user profiles and trust_score columns

DROP INDEX IF EXISTS idx_merchant_profiles_trust_score;
ALTER TABLE merchant_profiles DROP COLUMN IF EXISTS trust_score;

DROP INDEX IF EXISTS idx_rider_profiles_trust_score;
ALTER TABLE rider_profiles DROP COLUMN IF EXISTS trust_score;

DROP TABLE IF EXISTS user_profiles;
