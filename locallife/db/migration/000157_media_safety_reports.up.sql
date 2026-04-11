-- 补充 safety_reports 的媒体资产关联表（000106 遗漏）
-- 用关联表取代 images text[]，与 review_images 模式一致
CREATE TABLE safety_report_images (
    id              bigserial PRIMARY KEY,
    report_id       bigint NOT NULL REFERENCES safety_reports(id) ON DELETE CASCADE,
    media_asset_id  bigint NOT NULL REFERENCES media_assets(id),
    sort_order      smallint NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (report_id, media_asset_id)
);

CREATE INDEX idx_safety_report_images_report ON safety_report_images(report_id);

COMMENT ON TABLE safety_report_images IS '安全事件上报图片关联表，取代 safety_reports.images text[] 字段';
