DO $$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM ecommerce_applyments
		WHERE organization_type = '2600'
	) THEN
		RAISE EXCEPTION 'cannot apply ecommerce_applyments organization_type constraint while rows still use legacy value 2600';
	END IF;
END $$;

ALTER TABLE ecommerce_applyments
DROP CONSTRAINT IF EXISTS ecommerce_applyments_org_type_check;

ALTER TABLE ecommerce_applyments
ADD CONSTRAINT ecommerce_applyments_org_type_check
CHECK (organization_type IN ('2401', '2500', '4', '2', '3', '2502', '1708'));

COMMENT ON COLUMN ecommerce_applyments.organization_type IS '微信主体类型: 2401-小微商户, 2500-个人卖家, 4-个体工商户, 2-企业, 3-事业单位, 2502-政府机关, 1708-社会组织';