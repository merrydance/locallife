-- 创建运营商区域扩展申请表
-- 允许已入驻运营商申请管理更多区域

CREATE TABLE operator_region_applications (
    id              BIGSERIAL PRIMARY KEY,
    operator_id     BIGINT NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
    region_id       BIGINT NOT NULL REFERENCES regions(id),
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'approved', 'rejected')),
    reject_reason   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ,
    -- 同一运营商对同一区域只能有一条待审核申请
    UNIQUE (operator_id, region_id)
);

CREATE INDEX idx_ora_operator_id ON operator_region_applications(operator_id);
CREATE INDEX idx_ora_region_id   ON operator_region_applications(region_id);
CREATE INDEX idx_ora_status      ON operator_region_applications(status);

COMMENT ON TABLE operator_region_applications IS '运营商区域扩展申请，允许已入驻运营商申请管理更多区县';
