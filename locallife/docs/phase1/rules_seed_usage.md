# Phase1 规则写入接口使用示例（草案）

> 说明：以下仅描述 SQLC 接口调用顺序，避免误写生产数据。

## 写入步骤（建议）
1. CreateRule（status=draft）
2. CreateRuleVersion（status=published/draft）
3. UpdateRuleStatus（status=active + current_version_id）
4. CreateRuleAudit（create/create_version/publish）

## 回滚/禁用步骤（建议）
- 禁用：UpdateRuleStatus（status=disabled, current_version_id=NULL）+ CreateRuleAudit（disable）
- 回滚：UpdateRuleStatus（status=active, current_version_id=target）+ CreateRuleAudit（rollback）

## 注意事项
- 规则版本发布后才会被引擎读取
- 建议记录 actor_id/actor_role
- 回滚未指定版本时应选择最近的已发布版本（排除当前版本）
