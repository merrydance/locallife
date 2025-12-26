-- =============================================
-- 运营商入驻申请表
-- 设计原则：
-- 1. 参考商户入驻申请表结构，支持草稿保存
-- 2. 需要上传营业执照和法人身份证，通过微信OCR识别回填
-- 3. 区域独占：一个区县只能有一个运营商
-- 4. 人工审核：提交后需要平台管理员审核（与商户/骑手自动审核不同）
-- 5. 审核通过后自动创建 operators 记录
-- =============================================

CREATE TABLE operator_applications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL REFERENCES users(id),
    
    -- 申请区域（独占，一区一运营商）
    region_id BIGINT NOT NULL REFERENCES regions(id),
    
    -- 基础信息
    name TEXT,                                     -- 运营商名称（可从营业执照OCR回填）
    contact_name TEXT,                             -- 联系人姓名
    contact_phone TEXT,                            -- 联系人电话
    
    -- 营业执照信息
    business_license_url TEXT,                     -- 营业执照照片URL
    business_license_number TEXT,                  -- 营业执照注册号（统一社会信用代码）
    business_license_ocr JSONB,                    -- 营业执照OCR识别结果
                                                   -- {
                                                   --   "reg_num": "91110000...",
                                                   --   "enterprise_name": "xxx公司",
                                                   --   "legal_representative": "张三",
                                                   --   "type_of_enterprise": "有限责任公司",
                                                   --   "address": "北京市...",
                                                   --   "business_scope": "餐饮服务...",
                                                   --   "registered_capital": "100万",
                                                   --   "valid_period": "2020年01月01日至2040年01月01日",
                                                   --   "credit_code": "91110000...",
                                                   --   "ocr_at": "2025-01-01T12:00:00Z"
                                                   -- }
    
    -- 法人身份证信息
    legal_person_name TEXT,                        -- 法人姓名（OCR识别或手动填写）
    legal_person_id_number TEXT,                   -- 法人身份证号
    id_card_front_url TEXT,                        -- 身份证正面照片URL
    id_card_back_url TEXT,                         -- 身份证背面照片URL
    id_card_front_ocr JSONB,                       -- 身份证正面OCR识别结果
                                                   -- {
                                                   --   "name": "张三",
                                                   --   "id_number": "110101199001011234",
                                                   --   "gender": "男",
                                                   --   "nation": "汉",
                                                   --   "address": "北京市...",
                                                   --   "ocr_at": "2025-01-01T12:00:00Z"
                                                   -- }
    id_card_back_ocr JSONB,                        -- 身份证背面OCR识别结果
                                                   -- {
                                                   --   "valid_start": "2020-01-01",
                                                   --   "valid_end": "2030-01-01",  -- 或 "长期"
                                                   --   "ocr_at": "2025-01-01T12:00:00Z"
                                                   -- }
    
    -- 申请合同期限（用户选择）
    requested_contract_years INT NOT NULL DEFAULT 1, -- 申请的合同年限（1/2/3年等）
    
    -- 申请状态
    status TEXT NOT NULL DEFAULT 'draft',
    reject_reason TEXT,                            -- 拒绝原因
    reviewed_by BIGINT REFERENCES users(id),       -- 审核人
    reviewed_at TIMESTAMPTZ,                       -- 审核时间
    
    -- 时间戳
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at TIMESTAMPTZ,                      -- 提交审核时间
    
    -- 约束
    CONSTRAINT operator_applications_status_check 
        CHECK (status IN ('draft', 'submitted', 'approved', 'rejected'))
);

-- 索引
CREATE INDEX operator_applications_status_idx ON operator_applications(status);
CREATE INDEX operator_applications_region_id_idx ON operator_applications(region_id);
CREATE INDEX operator_applications_submitted_at_idx ON operator_applications(submitted_at);

-- 注释
COMMENT ON TABLE operator_applications IS '运营商入驻申请表，支持草稿保存和人工审核';
COMMENT ON COLUMN operator_applications.region_id IS '申请运营的区县ID，独占（一区一运营商）';
COMMENT ON COLUMN operator_applications.business_license_ocr IS '营业执照OCR识别结果JSON';
COMMENT ON COLUMN operator_applications.id_card_front_ocr IS '身份证正面OCR识别结果JSON';
COMMENT ON COLUMN operator_applications.id_card_back_ocr IS '身份证背面OCR识别结果JSON';
COMMENT ON COLUMN operator_applications.status IS 'draft=草稿, submitted=已提交待审核, approved=已通过, rejected=已拒绝';
