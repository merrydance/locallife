-- 包间/桌台图片表
CREATE TABLE table_images (
    id BIGSERIAL PRIMARY KEY,
    table_id BIGINT NOT NULL REFERENCES tables(id) ON DELETE CASCADE,
    image_url TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 索引
CREATE INDEX idx_table_images_table_id ON table_images(table_id);
CREATE INDEX idx_table_images_primary ON table_images(table_id) WHERE is_primary = TRUE;

-- 注释
COMMENT ON TABLE table_images IS '桌台/包间图片表';
COMMENT ON COLUMN table_images.table_id IS '桌台ID';
COMMENT ON COLUMN table_images.image_url IS '图片URL';
COMMENT ON COLUMN table_images.sort_order IS '排序顺序';
COMMENT ON COLUMN table_images.is_primary IS '是否为主图';
