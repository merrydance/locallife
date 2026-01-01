-- 先添加唯一约束（如果已存在同名标签会失败，但我们先检查无重复）
-- 注意：约束添加前需确保无重复name
ALTER TABLE tags ADD CONSTRAINT tags_name_unique UNIQUE (name);

-- 创建系统自动标签（新部署时插入，已存在时跳过）
-- type='system' 表示系统生成的标签，由定时任务自动管理
INSERT INTO tags (name, type, sort_order, status) VALUES
    ('热卖', 'system', 1, 'active'),
    ('推荐', 'system', 2, 'active')
ON CONFLICT (name) DO UPDATE SET type = 'system';
