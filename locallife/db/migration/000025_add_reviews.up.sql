-- M13: 评价系统

-- 订单评价表
CREATE TABLE IF NOT EXISTS reviews (
  id bigserial PRIMARY KEY,
  order_id bigint NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  merchant_id bigint NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
  
  -- 评价内容
  content text NOT NULL,
  images text[],
  
  -- 可见性控制（低信用用户评价不展示）
  is_visible boolean NOT NULL DEFAULT true,
  
  -- 商户回复
  merchant_reply text,
  replied_at timestamptz,
  
  created_at timestamptz NOT NULL DEFAULT now()
);

-- 索引
CREATE UNIQUE INDEX idx_reviews_order_id ON reviews(order_id);
CREATE INDEX idx_reviews_merchant_id ON reviews(merchant_id);
CREATE INDEX idx_reviews_user_id ON reviews(user_id);
CREATE INDEX idx_reviews_merchant_visible_created ON reviews(merchant_id, is_visible, created_at);

-- 注释
COMMENT ON TABLE reviews IS '订单评价表';
COMMENT ON COLUMN reviews.order_id IS '订单ID，每个订单只能评价一次';
COMMENT ON COLUMN reviews.user_id IS '评价用户ID';
COMMENT ON COLUMN reviews.merchant_id IS '商户ID';
COMMENT ON COLUMN reviews.content IS '评价内容';
COMMENT ON COLUMN reviews.images IS '评价图片URLs，PostgreSQL数组类型';
COMMENT ON COLUMN reviews.is_visible IS '是否可见，低信用用户评价不展示';
COMMENT ON COLUMN reviews.merchant_reply IS '商户回复内容';
COMMENT ON COLUMN reviews.replied_at IS '回复时间';
