ALTER TABLE operators 
ADD COLUMN merchant_deposit BIGINT NOT NULL DEFAULT 500000 CHECK (merchant_deposit >= 0),
ADD COLUMN rider_deposit BIGINT NOT NULL DEFAULT 20000 CHECK (rider_deposit >= 0),
ADD COLUMN weather_coeff_extreme NUMERIC(3,2) NOT NULL DEFAULT 2.00 CHECK (weather_coeff_extreme >= 1.00),
ADD COLUMN weather_coeff_heavy NUMERIC(3,2) NOT NULL DEFAULT 1.80 CHECK (weather_coeff_heavy >= 1.00),
ADD COLUMN weather_coeff_moderate NUMERIC(3,2) NOT NULL DEFAULT 1.30 CHECK (weather_coeff_moderate >= 1.00),
ADD COLUMN weather_coeff_light NUMERIC(3,2) NOT NULL DEFAULT 1.10 CHECK (weather_coeff_light >= 1.00);
