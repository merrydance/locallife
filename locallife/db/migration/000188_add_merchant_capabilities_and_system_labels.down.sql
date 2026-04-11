DELETE FROM merchant_system_labels
WHERE tag_id IN (
  SELECT id FROM tags WHERE name IN ('有明厨亮灶', '无明厨亮灶', '无堂食') AND type = 'system'
);

DROP INDEX IF EXISTS merchant_system_labels_tag_id_idx;
DROP TABLE IF EXISTS merchant_system_labels;
DROP TABLE IF EXISTS merchant_capabilities;

DELETE FROM tags
WHERE name IN ('有明厨亮灶', '无明厨亮灶', '无堂食')
  AND type = 'system';