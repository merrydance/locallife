ALTER TABLE operators 
DROP COLUMN IF EXISTS merchant_deposit,
DROP COLUMN IF EXISTS rider_deposit,
DROP COLUMN IF EXISTS weather_coeff_extreme,
DROP COLUMN IF EXISTS weather_coeff_heavy,
DROP COLUMN IF EXISTS weather_coeff_moderate,
DROP COLUMN IF EXISTS weather_coeff_light;
