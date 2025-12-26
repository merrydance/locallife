-- =============================================
-- 微信平台收付通二级商户进件
-- 用于商户、骑手、运营商的微信支付账户开户
-- =============================================

-- 二级商户进件申请表
CREATE TABLE "ecommerce_applyments" (
    "id" bigserial PRIMARY KEY,
    
    -- 关联主体（多态关联）
    "subject_type" text NOT NULL,                        -- merchant, rider, operator
    "subject_id" bigint NOT NULL,                        -- merchants.id, riders.id, operators.id
    
    -- 进件信息
    "out_request_no" text NOT NULL UNIQUE,               -- 业务申请编号（唯一）
    "applyment_id" bigint,                               -- 微信返回的申请单号
    
    -- 主体资质信息
    "organization_type" text NOT NULL,                   -- 主体类型: 2401-小微商户, 2500-个体工商户, 2600-企业
    "business_license_number" text,                      -- 营业执照号（个体户/企业必填）
    "business_license_copy" text,                        -- 营业执照照片MediaID
    "merchant_name" text NOT NULL,                       -- 商户名称
    "legal_person" text NOT NULL,                        -- 法人姓名
    
    -- 法人身份信息（加密存储）
    "id_card_number" text NOT NULL,                      -- 身份证号（敏感信息已加密）
    "id_card_name" text NOT NULL,                        -- 身份证姓名
    "id_card_valid_time" text NOT NULL,                  -- 身份证有效期 YYYY-MM-DD 或 长期
    "id_card_front_copy" text NOT NULL,                  -- 身份证正面MediaID
    "id_card_back_copy" text NOT NULL,                   -- 身份证背面MediaID
    
    -- 结算银行账户信息（加密存储）
    "account_type" text NOT NULL,                        -- 账户类型: ACCOUNT_TYPE_BUSINESS-对公, ACCOUNT_TYPE_PRIVATE-对私
    "account_bank" text NOT NULL,                        -- 开户银行
    "bank_address_code" text NOT NULL,                   -- 开户银行省市编码
    "bank_name" text,                                    -- 开户银行全称（支行）
    "account_number" text NOT NULL,                      -- 银行账号（敏感信息已加密）
    "account_name" text NOT NULL,                        -- 开户名称
    
    -- 联系信息
    "contact_name" text NOT NULL,                        -- 超级管理员姓名
    "contact_id_card_number" text,                       -- 超级管理员身份证号（可选）
    "mobile_phone" text NOT NULL,                        -- 联系手机号
    "contact_email" text,                                -- 联系邮箱
    
    -- 经营信息
    "merchant_shortname" text NOT NULL,                  -- 商户简称（展示给用户）
    "qualifications" jsonb,                              -- 特殊资质（如食品经营许可证）
    "business_addition_pics" text[],                     -- 补充材料图片MediaID列表
    "business_addition_desc" text,                       -- 补充说明
    
    -- 状态
    "status" text NOT NULL DEFAULT 'pending',            -- pending, submitted, auditing, rejected, frozen, to_be_signed, signing, rejected_sign, finish
    "sign_url" text,                                     -- 签约链接（状态为to_be_signed时返回）
    "sign_state" text,                                   -- 签约状态
    "reject_reason" text,                                -- 拒绝原因
    "sub_mch_id" text,                                   -- 开户成功后返回的特约商户号
    
    -- 时间
    "created_at" timestamptz NOT NULL DEFAULT now(),
    "submitted_at" timestamptz,                          -- 提交微信时间
    "audited_at" timestamptz,                            -- 审核完成时间
    "updated_at" timestamptz NOT NULL DEFAULT now(),
    
    -- 约束
    CONSTRAINT ecommerce_applyments_subject_type_check CHECK (subject_type IN ('merchant', 'rider', 'operator')),
    CONSTRAINT ecommerce_applyments_org_type_check CHECK (organization_type IN ('2401', '2500', '2600')),
    CONSTRAINT ecommerce_applyments_account_type_check CHECK (account_type IN ('ACCOUNT_TYPE_BUSINESS', 'ACCOUNT_TYPE_PRIVATE')),
    CONSTRAINT ecommerce_applyments_status_check CHECK (status IN ('pending', 'submitted', 'auditing', 'rejected', 'frozen', 'to_be_signed', 'signing', 'rejected_sign', 'finish'))
);

-- 索引
CREATE INDEX ecommerce_applyments_subject_idx ON ecommerce_applyments(subject_type, subject_id);
CREATE INDEX ecommerce_applyments_status_idx ON ecommerce_applyments(status);
CREATE INDEX ecommerce_applyments_applyment_id_idx ON ecommerce_applyments(applyment_id);
CREATE INDEX ecommerce_applyments_sub_mch_id_idx ON ecommerce_applyments(sub_mch_id);

-- 注释
COMMENT ON TABLE ecommerce_applyments IS '微信平台收付通二级商户进件申请';
COMMENT ON COLUMN ecommerce_applyments.subject_type IS '主体类型: merchant-商户, rider-骑手, operator-运营商';
COMMENT ON COLUMN ecommerce_applyments.organization_type IS '微信主体类型: 2401-小微商户(个人), 2500-个体工商户, 2600-企业';
COMMENT ON COLUMN ecommerce_applyments.status IS '进件状态: pending-待提交, submitted-已提交, auditing-审核中, rejected-已驳回, frozen-冻结, to_be_signed-待签约, signing-签约中, rejected_sign-签约失败, finish-完成';

-- 为骑手添加微信支付商户号字段
ALTER TABLE "riders" ADD COLUMN IF NOT EXISTS "sub_mch_id" text;
CREATE INDEX IF NOT EXISTS riders_sub_mch_id_idx ON riders(sub_mch_id);
COMMENT ON COLUMN riders.sub_mch_id IS '微信平台收付通二级商户号';

-- 修改商户状态约束，增加 pending_bindbank 和 bindbank_submitted 状态
-- 先删除旧约束
ALTER TABLE "merchants" DROP CONSTRAINT IF EXISTS merchants_status_check;
-- 添加新约束
ALTER TABLE "merchants" ADD CONSTRAINT merchants_status_check CHECK (status IN ('pending', 'approved', 'pending_bindbank', 'bindbank_submitted', 'active', 'suspended', 'rejected'));

-- 修改骑手状态约束，增加 pending_bindbank 和 bindbank_submitted 状态  
ALTER TABLE "riders" DROP CONSTRAINT IF EXISTS riders_status_check;
ALTER TABLE "riders" ADD CONSTRAINT riders_status_check CHECK (status IN ('pending', 'approved', 'pending_bindbank', 'bindbank_submitted', 'active', 'suspended', 'rejected'));

-- 为运营商添加状态约束（如果没有的话）
ALTER TABLE "operators" DROP CONSTRAINT IF EXISTS operators_status_check;
ALTER TABLE "operators" ADD CONSTRAINT operators_status_check CHECK (status IN ('pending', 'approved', 'pending_bindbank', 'bindbank_submitted', 'active', 'suspended'));
