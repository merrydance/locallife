-- 评价图片关联表
-- 取代 reviews.images text[] 字段，支持每张图独立的媒体元数据
CREATE TABLE review_images (
    id             bigserial    PRIMARY KEY,
    review_id      bigint       NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    media_asset_id bigint       NOT NULL REFERENCES media_assets(id),
    sort_order     integer      NOT NULL DEFAULT 0,
    created_at     timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT review_images_unique UNIQUE (review_id, media_asset_id)
);

CREATE INDEX idx_review_images_review_id ON review_images (review_id);

COMMENT ON TABLE review_images IS '评价图片关联表，取代 reviews.images 数组字段';
COMMENT ON COLUMN review_images.sort_order IS '图片排列顺序';
