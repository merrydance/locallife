ALTER TABLE riders DROP CONSTRAINT IF EXISTS riders_status_check;
ALTER TABLE riders ALTER COLUMN status SET DEFAULT 'pending';
ALTER TABLE riders ADD CONSTRAINT riders_status_check CHECK (status IN ('pending', 'approved', 'pending_bindbank', 'bindbank_submitted', 'active', 'suspended', 'rejected'));

ALTER TABLE rider_applications DROP CONSTRAINT IF EXISTS rider_applications_status_check;
ALTER TABLE rider_applications ADD CONSTRAINT rider_applications_status_check CHECK (status IN ('draft', 'submitted', 'approved', 'rejected'));

ALTER TABLE ecommerce_applyments DROP CONSTRAINT IF EXISTS ecommerce_applyments_subject_type_check;
ALTER TABLE ecommerce_applyments ADD CONSTRAINT ecommerce_applyments_subject_type_check CHECK (subject_type IN ('merchant', 'rider', 'operator'));

ALTER TABLE riders ADD COLUMN IF NOT EXISTS sub_mch_id text;
CREATE INDEX IF NOT EXISTS riders_sub_mch_id_idx ON riders(sub_mch_id);