ALTER TABLE merchant_cancel_withdraw_applications
    ADD COLUMN business_license_status_declaration VARCHAR(32),
    ADD CONSTRAINT merchant_cancel_withdraw_applications_license_status_declaration_check
        CHECK (
            business_license_status_declaration IS NULL OR
            business_license_status_declaration IN ('ACTIVE', 'CANCELED', 'REVOKED')
        );

COMMENT ON COLUMN merchant_cancel_withdraw_applications.business_license_status_declaration IS '商户声明的营业执照状态：ACTIVE/CANCELED/REVOKED；仅用于企业主体注销提现材料校验';