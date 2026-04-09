DELETE FROM tags
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