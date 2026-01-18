# 行为追溯裁决系统实施计划（生产级）

> 目标：基于[docs/BEHAVIOR_TRACE_SYSTEM_DESIGN.md](BEHAVIOR_TRACE_SYSTEM_DESIGN.md)落地可上线的生产级实现。
> 约束：遵守[docs/ai_coding_prompt.md](ai_coding_prompt.md)与现有工程规范；所有实现方案需生产级、可回滚、可观测。

## 0. 里程碑与交付物
- M0 设计冻结：确认默认阈值、配置落地方式、回滚策略。
- M1 数据层与配置：表结构/索引/配置表/配置读取。
- M2 采集链路：下单/索赔入口事务性采集与证据链落库。
- M3 规则引擎与裁决：规则执行、裁决记录、动作执行。
- M4 运营与展示：店铺后台提醒、审计与指标。
- M5 灰度与替换：双写/开关/回滚。

## 1. 需求范围确认（生产级）
- 顾客索赔无条件赔付；裁决用于治理。
- 外卖拒绝服务：仅拦截外卖下单；不影响预订/堂食。
- 达到阈值时店铺后台提醒：“该顾客有多次恶意索赔记录，谨慎服务”。
- 设备信号：保留`device_id`并新增`device_fingerprint`（哈希存储，90天留存）。
- 申诉/复核仅面向商户/骑手；自动化再评估。

## 2. 配置与参数治理
### 2.1 配置落地方式（推荐）
- 方案B（优先）：平台配置表，支持按城市/门店灰度。
- 方案A（兜底）：配置文件默认值（保证冷启动可用）。

### 2.2 默认值（可热更新）
- 异常次数阈值：7天≥3；30天≥5。
- 异常率阈值：7天≥30%；30天≥20%。
- 7天仅1单：补算30天窗口（分母下限=30天总单数）。
- 拒绝外卖服务冷却期：14天。
- 恢复规则：冷却期内无新增异常且完成≥3单正常外卖。
- 设备复用：7天同设备≥3用户且索赔≥3。
- 地址聚类：7天同地址≥3用户且索赔≥3。
- 协同索赔：1小时内同商户≥3用户且有关联。
- 店铺后台提醒频率：同一顾客24小时内最多1次提示。
- 申诉再评估冷却：7天。

## 3. 数据层设计与迁移
### 3.1 新增/调整表
- `behavior_decisions`
- `behavior_evidence`
- `behavior_trace_snapshots`
- `behavior_actions`
- `behavior_appeals`
- `behavior_blocklist`
- `platform_config`

### 3.2 索引建议
- `behavior_decisions(order_id, created_at)`
- `behavior_blocklist(entity_type, entity_id, status)`
- `behavior_evidence(decision_id)`
- `behavior_actions(decision_id, status)`
- `behavior_trace_snapshots(decision_id)`

### 3.3 SQLC 与迁移
- 表变更进入`db/migration`；查询进入`db/query`；执行`sqlc generate`与`mockgen`。

## 4. 采集链路（事务性）
### 4.1 采集入口
- 下单与索赔入口：采集`device_id`/`device_fingerprint`/`ip`/`user_agent`/`address_id`。
- 采集动作与业务记录同事务落库。

### 4.2 证据链绑定
- 采集信号写入`behavior_evidence`。
- 形成`behavior_trace_snapshots`，用于规则回放。

## 5. 规则引擎与裁决
### 5.1 规则执行
- 设备复用/地址聚类/协同索赔在索赔提交时自动触发。
- 规则输出：`reason_codes`、`decision_version`、`responsible_party`、`compensation_source`。

### 5.2 裁决记录
- 创建`behavior_decisions`并写入审计日志。
- 生成对应`behavior_actions`。

## 6. 动作执行
- 外卖拒绝服务：写入`behavior_blocklist`并拦截外卖下单。
- 店铺后台提醒：基于`behavior_blocklist`在后台页面展示提示，限频控制。
- 异步通知：使用任务队列，幂等。

## 7. 申诉与复核（自动化）
- 申诉仅限商户/骑手；写入`behavior_appeals`。
- 再评估触发：进入规则引擎自动重算并回写。

## 8. 观测与审计
- 监控指标：异常占比、拦截率、误判率、申诉复评率、行动执行失败率。
- 审计字段：`decision_version`、`reason_codes`、证据链完整性。

## 9. 实施步骤（按迭代）
### Sprint 1
- 新增表结构与索引；配置表与读取逻辑。
- SQLC 与 mockgen 更新。

### Sprint 2
- 下单/索赔入口采集链路与证据落库（事务性）。
- 写入`behavior_trace_snapshots`。

### Sprint 3
- 规则引擎与裁决记录落地；自动触发。
- 生成`behavior_actions`。

### Sprint 4
- 外卖拒绝服务拦截；店铺后台提醒展示与限频。
- 指标与审计。

### Sprint 5
- 压测与稳定性验证。

## 10. 风险与对策
- 误判风险：保留申诉自动复核与灰度策略。
- 性能风险：索引优化与异步任务下沉。
- 隐私风险：`device_fingerprint`哈希与留存策略。

## 11. 验收标准
- 无人工介入；规则自动裁决与审计可追溯。
- 外卖拒绝服务仅影响外卖；预订/堂食不受影响。
- 店铺后台提醒可见且限频。
- 采集链路事务性落库；证据链完整。
