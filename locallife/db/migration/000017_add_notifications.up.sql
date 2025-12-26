-- M14: 通知系统

-- 用户通知表（支持WebSocket实时推送）
CREATE TABLE notifications (
  id bigserial PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  
  -- 通知内容
  type text NOT NULL CHECK (type IN ('order', 'payment', 'delivery', 'system', 'food_safety')),
  title text NOT NULL,
  content text NOT NULL,
  
  -- 关联数据（用于跳转）
  related_type text CHECK (related_type IN ('order', 'payment', 'delivery', 'reservation', 'merchant')),
  related_id bigint,
  
  -- 额外数据（JSON格式，用于前端渲染）
  extra_data jsonb,
  
  -- 状态
  is_read boolean NOT NULL DEFAULT false,
  read_at timestamptz,
  
  -- 推送状态
  is_pushed boolean NOT NULL DEFAULT false,
  pushed_at timestamptz,
  
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz
);

-- 索引
CREATE INDEX idx_notifications_user_id ON notifications(user_id);
CREATE INDEX idx_notifications_user_read ON notifications(user_id, is_read);
CREATE INDEX idx_notifications_user_created ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_type ON notifications(type);
CREATE INDEX idx_notifications_related ON notifications(related_type, related_id) WHERE related_type IS NOT NULL;
CREATE INDEX idx_notifications_created ON notifications(created_at DESC);

-- 用户通知偏好设置表
CREATE TABLE user_notification_preferences (
  id bigserial PRIMARY KEY,
  user_id bigint NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  
  -- 通知类型开关
  enable_order_notifications boolean NOT NULL DEFAULT true,
  enable_payment_notifications boolean NOT NULL DEFAULT true,
  enable_delivery_notifications boolean NOT NULL DEFAULT true,
  enable_system_notifications boolean NOT NULL DEFAULT true,
  enable_food_safety_notifications boolean NOT NULL DEFAULT true,
  
  -- 免打扰时段
  do_not_disturb_start time,
  do_not_disturb_end time,
  
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

-- 索引
CREATE UNIQUE INDEX idx_notification_prefs_user ON user_notification_preferences(user_id);

-- 注释
COMMENT ON TABLE notifications IS '用户通知表，支持WebSocket实时推送';
COMMENT ON COLUMN notifications.type IS '通知类型：order/payment/delivery/system/food_safety';
COMMENT ON COLUMN notifications.related_type IS '关联实体类型，用于跳转';
COMMENT ON COLUMN notifications.extra_data IS '扩展数据JSON，用于前端渲染';
COMMENT ON COLUMN notifications.is_pushed IS '是否已通过WebSocket推送';
COMMENT ON COLUMN notifications.expires_at IS '过期时间，过期后可删除';

COMMENT ON TABLE user_notification_preferences IS '用户通知偏好设置';
COMMENT ON COLUMN user_notification_preferences.do_not_disturb_start IS '免打扰开始时间';
COMMENT ON COLUMN user_notification_preferences.do_not_disturb_end IS '免打扰结束时间';
