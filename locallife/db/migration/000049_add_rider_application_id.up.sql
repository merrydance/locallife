-- riders表添加application_id关联申请表
-- 审核通过后，从rider_applications创建rider记录

ALTER TABLE riders 
ADD COLUMN application_id BIGINT REFERENCES rider_applications(id);

-- 添加唯一约束，确保一个申请只能创建一个骑手
CREATE UNIQUE INDEX riders_application_id_idx ON riders(application_id) WHERE application_id IS NOT NULL;

COMMENT ON COLUMN riders.application_id IS '关联的入驻申请ID，审核通过后填充';
