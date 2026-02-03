-- Phase1 规则种子导入示例（开发/测试环境）
-- 注意：仅示例，不可直接用于生产

BEGIN;

-- 1) 创建规则
INSERT INTO rules (name, category, status)
VALUES ('takeout_blocklist_deny', 'order', 'draft')
RETURNING id;

-- 2) 创建规则版本（示例，仅展示结构）
-- 请在实际导入时填充 scope/condition/action/gray_config
-- INSERT INTO rule_versions (rule_id, version, status, priority, scope, condition, action, gray_config)
-- VALUES (
--   <rule_id>,
--   1,
--   'published',
--   10,
--   '{"order_type":"takeout"}',
--   '{"behavior_blocklist":true}',
--   '{"type":"deny","reason":"外卖服务已被限制：该账号存在异常索赔记录"}',
--   '{"region_id":[1101]}'
-- );

-- 3) 绑定当前版本
-- UPDATE rules SET status = 'active', current_version_id = <version_id> WHERE id = <rule_id>;

-- 4) 记录审计
-- INSERT INTO rule_audits (rule_id, rule_version_id, action, actor_id, actor_role, detail)
-- VALUES (...);

ROLLBACK; -- 防止误写
