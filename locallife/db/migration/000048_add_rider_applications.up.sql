-- =============================================
-- 骑手入驻申请表
-- 设计原则：
-- 1. 每种证件独立JSONB字段，便于单独更新和查询
-- 2. 支持草稿保存，用户可分步填写
-- 3. 自动审核：身份证有效期内 + 健康证已上传 → 自动通过
-- =============================================

CREATE TABLE rider_applications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL REFERENCES users(id),
    
    -- 基础信息
    real_name TEXT,                    -- 真实姓名（OCR识别或手动填写）
    phone TEXT,                        -- 联系手机号
    
    -- 身份证信息
    id_card_front_url TEXT,            -- 身份证正面照片URL
    id_card_back_url TEXT,             -- 身份证背面照片URL
    id_card_ocr JSONB,                 -- 身份证OCR识别结果
                                       -- {
                                       --   "name": "张三",
                                       --   "id_number": "110101199001011234",
                                       --   "gender": "男",
                                       --   "nation": "汉",
                                       --   "address": "北京市...",
                                       --   "valid_start": "2020-01-01",
                                       --   "valid_end": "2030-01-01",  -- 或 "长期"
                                       --   "ocr_at": "2025-01-01T12:00:00Z"
                                       -- }
    
    -- 健康证信息
    health_cert_url TEXT,              -- 健康证照片URL
    health_cert_ocr JSONB,             -- 健康证OCR识别结果（如果支持）
                                       -- {
                                       --   "cert_number": "...",
                                       --   "valid_start": "2024-01-01",
                                       --   "valid_end": "2025-01-01",
                                       --   "ocr_at": "2025-01-01T12:00:00Z"
                                       -- }
    
    -- 申请状态
    status TEXT NOT NULL DEFAULT 'draft',
    reject_reason TEXT,                -- 拒绝原因
    reviewed_by BIGINT REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    
    -- 时间戳
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,          -- 提交审核时间
    
    -- 约束
    CONSTRAINT rider_applications_status_check 
        CHECK (status IN ('draft', 'submitted', 'approved', 'rejected'))
);

-- 索引
CREATE INDEX rider_applications_status_idx ON rider_applications(status);
CREATE INDEX rider_applications_submitted_at_idx ON rider_applications(submitted_at);

-- 注释
COMMENT ON TABLE rider_applications IS '骑手入驻申请表，支持草稿保存和自动审核';
COMMENT ON COLUMN rider_applications.id_card_ocr IS '身份证OCR识别结果JSON';
COMMENT ON COLUMN rider_applications.health_cert_ocr IS '健康证OCR识别结果JSON';
COMMENT ON COLUMN rider_applications.status IS 'draft=草稿, submitted=已提交待审核, approved=已通过, rejected=已拒绝';
