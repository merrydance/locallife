SELECT m.id,
       m.name,
       m.address,
       m.region_id,
       m.latitude,
       m.longitude,
       m.status,
       m.is_open
FROM merchants m
WHERE m.name IN ('宁晋县饺好小吃店', '宁晋县周鹏饭店', '宁晋县奇岩饭店')
  AND m.deleted_at IS NULL
ORDER BY m.id;