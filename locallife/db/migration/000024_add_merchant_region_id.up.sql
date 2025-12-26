-- Add region_id to merchants table
ALTER TABLE merchants ADD COLUMN region_id bigint REFERENCES regions(id);

-- Create index for region queries
CREATE INDEX idx_merchants_region_id ON merchants(region_id);

-- Update existing merchants based on closest region (assuming regions table has geographic data)
-- This is a placeholder - actual implementation depends on your region matching logic
-- For now, we'll leave region_id as NULL for existing merchants

-- Add comment
COMMENT ON COLUMN merchants.region_id IS '商户所属区域ID';
