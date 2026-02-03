# Phase1 规则种子导入草案

> 先以文档形式描述导入流程，避免误写生产数据。

## 导入流程（草案）
1. 读取 rules_seed_draft.json
2. 基础校验（name/category/version/status/priority/condition/action）
3. 对每条规则创建 rules + rule_versions
4. 将版本状态设为 published 或 draft（按导入环境）
5. 发布规则并设置 current_version_id（仅灰度/测试环境）
6. 记录 rule_audits
7. 输出导入结果（成功数/失败数/原因）

## 说明
- 仅建议在开发/测试环境导入
- 生产环境需走审批与灰度流程
- 导入需在单事务中执行，失败需回滚
- 重复导入需具备幂等策略（name+category 唯一，或先查询再更新）
- 建议提供 dry-run 模式，仅做校验与统计
- 导入后建议先旁路观察，再逐步扩大灰度
