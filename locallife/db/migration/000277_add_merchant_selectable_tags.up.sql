CREATE TABLE merchant_selectable_tags (
  merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
  tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  sort_order SMALLINT NOT NULL DEFAULT 0,
  created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (merchant_id, tag_id)
);

CREATE INDEX merchant_selectable_tags_merchant_order_idx
  ON merchant_selectable_tags (merchant_id, sort_order, tag_id);

CREATE INDEX merchant_selectable_tags_tag_id_idx
  ON merchant_selectable_tags (tag_id);

COMMENT ON TABLE merchant_selectable_tags IS '商户可选业务标签集合；tags 仍为全局字典，商户通过本表选择可用标签';
COMMENT ON COLUMN merchant_selectable_tags.created_by_user_id IS '首次将标签加入商户可选集合的用户';

INSERT INTO merchant_selectable_tags (merchant_id, tag_id, sort_order)
SELECT used_tags.merchant_id, used_tags.tag_id, MIN(used_tags.sort_order)::SMALLINT
FROM (
  SELECT d.merchant_id, dt.tag_id, t.sort_order
  FROM dish_tags dt
  INNER JOIN dishes d ON d.id = dt.dish_id
  INNER JOIN tags t ON t.id = dt.tag_id
  WHERE d.deleted_at IS NULL
    AND t.status = 'active'
    AND t.type = 'dish'

  UNION ALL

  SELECT tb.merchant_id, tt.tag_id, t.sort_order
  FROM table_tags tt
  INNER JOIN tables tb ON tb.id = tt.table_id
  INNER JOIN tags t ON t.id = tt.tag_id
  WHERE t.status = 'active'
    AND t.type = 'table'

  UNION ALL

  SELECT cs.merchant_id, ct.tag_id, t.sort_order
  FROM combo_tags ct
  INNER JOIN combo_sets cs ON cs.id = ct.combo_id
  INNER JOIN tags t ON t.id = ct.tag_id
  WHERE cs.deleted_at IS NULL
    AND t.status = 'active'
    AND t.type = 'combo'

  UNION ALL

  SELECT d.merchant_id, dco.tag_id, t.sort_order
  FROM dish_customization_options dco
  INNER JOIN dish_customization_groups dcg ON dcg.id = dco.group_id
  INNER JOIN dishes d ON d.id = dcg.dish_id
  INNER JOIN tags t ON t.id = dco.tag_id
  WHERE d.deleted_at IS NULL
    AND t.status = 'active'
    AND t.type = 'customization'
) AS used_tags
GROUP BY used_tags.merchant_id, used_tags.tag_id
ON CONFLICT (merchant_id, tag_id) DO NOTHING;
