CREATE TABLE merchant_capabilities (
  merchant_id BIGINT PRIMARY KEY REFERENCES merchants(id) ON DELETE CASCADE,
  open_kitchen_status TEXT NOT NULL DEFAULT 'unknown' CHECK (open_kitchen_status IN ('unknown', 'yes', 'no')),
  dine_in_status TEXT NOT NULL DEFAULT 'unknown' CHECK (dine_in_status IN ('unknown', 'yes', 'no')),
  source TEXT NOT NULL DEFAULT 'system_default' CHECK (source IN ('system_default', 'manual_review', 'merchant_claim', 'migration_backfill')),
  note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE merchant_system_labels (
  merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
  tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  source TEXT NOT NULL DEFAULT 'capability_reconciler' CHECK (source IN ('capability_reconciler', 'manual_override', 'migration_backfill')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (merchant_id, tag_id)
);

CREATE INDEX merchant_system_labels_tag_id_idx ON merchant_system_labels(tag_id);

COMMENT ON TABLE merchant_capabilities IS '商户能力真值表；系统标签由此表派生，不与经营类目混用';
COMMENT ON COLUMN merchant_capabilities.open_kitchen_status IS '明厨亮灶状态：unknown/yes/no';
COMMENT ON COLUMN merchant_capabilities.dine_in_status IS '堂食能力状态：unknown/yes/no';
COMMENT ON COLUMN merchant_capabilities.source IS '最后一次能力确认来源';
COMMENT ON TABLE merchant_system_labels IS '商户系统展示标签关联表；仅存储派生或运营维护的系统标签';

INSERT INTO tags (name, type, sort_order, status) VALUES
  ('有明厨亮灶', 'system', 10, 'active'),
  ('无明厨亮灶', 'system', 11, 'active'),
  ('无堂食', 'system', 12, 'active')
ON CONFLICT (name) DO UPDATE SET
  type = 'system',
  sort_order = EXCLUDED.sort_order,
  status = 'active';

INSERT INTO merchant_capabilities (merchant_id, open_kitchen_status, dine_in_status, source)
SELECT id, 'unknown', 'unknown', 'migration_backfill'
FROM merchants
WHERE deleted_at IS NULL
  AND status IN ('approved', 'active', 'suspended')
ON CONFLICT (merchant_id) DO NOTHING;

INSERT INTO merchant_system_labels (merchant_id, tag_id, source)
SELECT mc.merchant_id, t.id, 'migration_backfill'
FROM merchant_capabilities mc
JOIN tags t ON t.name = '无明厨亮灶' AND t.type = 'system'
ON CONFLICT (merchant_id, tag_id) DO NOTHING;