DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ecommerce_applyments
        WHERE organization_type IN ('4', '2', '3', '2502', '1708')
    ) THEN
        RAISE EXCEPTION 'cannot downgrade ecommerce_applyments organization_type constraint while rows use 4/2/3/2502/1708';
    END IF;
END $$;

ALTER TABLE ecommerce_applyments
DROP CONSTRAINT IF EXISTS ecommerce_applyments_org_type_check;

ALTER TABLE ecommerce_applyments
ADD CONSTRAINT ecommerce_applyments_org_type_check
CHECK (organization_type IN ('2401', '2500', '2600'));

COMMENT ON COLUMN ecommerce_applyments.organization_type IS '微信主体类型(旧约束): 2401-小微商户, 2500-个人卖家, 2600-企业';