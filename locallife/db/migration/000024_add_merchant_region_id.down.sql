-- Remove region_id from merchants table
DROP INDEX IF EXISTS idx_merchants_region_id;
ALTER TABLE merchants DROP COLUMN IF EXISTS region_id;
