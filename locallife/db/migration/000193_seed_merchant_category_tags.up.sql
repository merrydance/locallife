INSERT INTO tags (name, type, sort_order, status) VALUES
  ('面条', 'merchant', 100, 'active'),
  ('炒饼', 'merchant', 101, 'active'),
  ('家常菜', 'merchant', 102, 'active'),
  ('川菜', 'merchant', 103, 'active'),
  ('湘菜', 'merchant', 104, 'active'),
  ('海鲜', 'merchant', 105, 'active'),
  ('炸鸡', 'merchant', 106, 'active'),
  ('汉堡', 'merchant', 107, 'active'),
  ('披萨', 'merchant', 108, 'active'),
  ('减脂', 'merchant', 109, 'active'),
  ('火锅', 'merchant', 110, 'active'),
  ('粥', 'merchant', 111, 'active'),
  ('奶茶', 'merchant', 112, 'active'),
  ('鱼', 'merchant', 113, 'active')
ON CONFLICT (name) DO UPDATE SET
  type = 'merchant',
  sort_order = EXCLUDED.sort_order,
  status = 'active';