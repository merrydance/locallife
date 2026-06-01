DROP INDEX IF EXISTS wanted_merchant_votes_region_user_idx;
DROP INDEX IF EXISTS wanted_merchant_votes_candidate_idx;
DROP TABLE IF EXISTS wanted_merchant_votes;

DROP INDEX IF EXISTS wanted_merchants_matched_merchant_idx;
DROP INDEX IF EXISTS wanted_merchants_region_rank_idx;
DROP INDEX IF EXISTS wanted_merchants_active_region_name_uidx;
DROP TABLE IF EXISTS wanted_merchants;
