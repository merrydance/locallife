ALTER TABLE "tags"
ADD COLUMN IF NOT EXISTS "icon" text;

UPDATE "tags"
SET icon = CASE name
  WHEN '面条' THEN '🍜'
  WHEN '炒饼' THEN '🥞'
  WHEN '家常菜' THEN '🥘'
  WHEN '川菜' THEN '🌶️'
  WHEN '湘菜' THEN '🌶️'
  WHEN '海鲜' THEN '🦐'
  WHEN '炸鸡' THEN '🍗'
  WHEN '汉堡' THEN '🍔'
  WHEN '披萨' THEN '🍕'
  WHEN '减脂' THEN '🥗'
  WHEN '火锅' THEN '🍲'
  WHEN '粥' THEN '🥣'
  WHEN '奶茶' THEN '🧋'
  WHEN '鱼' THEN '🐟'
  ELSE icon
END
WHERE type = 'merchant'
  AND name IN (
    '面条',
    '炒饼',
    '家常菜',
    '川菜',
    '湘菜',
    '海鲜',
    '炸鸡',
    '汉堡',
    '披萨',
    '减脂',
    '火锅',
    '粥',
    '奶茶',
    '鱼'
  );
