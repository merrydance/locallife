ALTER TABLE riders DROP CONSTRAINT IF EXISTS riders_status_check;
ALTER TABLE riders ALTER COLUMN status SET DEFAULT 'approved';
ALTER TABLE riders ADD CONSTRAINT riders_status_check CHECK (status IN ('approved', 'active', 'suspended'));

ALTER TABLE rider_applications DROP CONSTRAINT IF EXISTS rider_applications_status_check;
ALTER TABLE rider_applications ADD CONSTRAINT rider_applications_status_check CHECK (status IN ('draft', 'submitted', 'approved'));

ALTER TABLE ecommerce_applyments DROP CONSTRAINT IF EXISTS ecommerce_applyments_subject_type_check;
ALTER TABLE ecommerce_applyments ADD CONSTRAINT ecommerce_applyments_subject_type_check CHECK (subject_type IN ('merchant', 'operator'));

DROP INDEX IF EXISTS riders_sub_mch_id_idx;
ALTER TABLE riders DROP COLUMN IF EXISTS sub_mch_id;