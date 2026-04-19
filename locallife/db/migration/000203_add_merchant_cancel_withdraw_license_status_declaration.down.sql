ALTER TABLE merchant_cancel_withdraw_applications
    DROP CONSTRAINT IF EXISTS merchant_cancel_withdraw_applications_license_status_declaration_check,
    DROP COLUMN IF EXISTS business_license_status_declaration;