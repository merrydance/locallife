# 异常订单索赔裁决与追偿闭环执行方案（无证据、行为驱动）

> 目标：所有索赔基于行为规则自动裁决，平台先行赔付并生成追偿单；对商户/骑手的追偿逾期自动限制接单（商户不影响堂食），并保留审计旁路与资金回滚能力。

## 1. 设计原则
- **不采信证据**：索赔不要求证据输入，证据字段不参与裁决。
- **系统裁决唯一来源**：规则引擎输出责任方与赔付策略。
- **平台先赔付**：只要用户被判定非碰瓷，平台先行赔付。
- **追偿闭环**：对责任方生成追偿单，逾期触发限制接单。
- **可审计可回滚**：裁决依据、规则命中、行为指标可追溯；审计复核仅用于纠错回滚。

## 1.1 关键改造要点（与既有方案融合后的必要项）
1) 裁决策略总原则
- 不采信证据：索赔不要求证据，证据字段与相关规则条件全部废弃。
- 系统裁决唯一来源：所有索赔走规则引擎，不进入人工复核。
- 平台兜底仅最后触发：仅在用户/商户/骑手均无明显异常时由平台兜底（兜底指追偿归属，不影响先赔付）。
- 责任方明确即自动执行：责任方裁定用于追偿执行（生成追偿单与限制接单），赔付仍由平台先行承担。

2) 规则引擎改造（核心）
- 移除证据规则：删除 has_evidence、evidence_count、evidence-required 相关条件和动作。
- 新增“责任方裁决”动作类型：responsible_party=customer|merchant|rider|platform_fallback。
- 新增“平台兜底触发条件”：三方行为均低风险才允许 platform_fallback。

3) 行为指标统一与扩展
- 用户：7/30 天异常索赔数、索赔率、同设备/同 openid 多账号、频次陡增。
- 商户：异物/食安占比、短期集中投诉、近 30 天异常率。
- 骑手：餐损/超时占比、近 7/30 天高频、超时集中。
- 统一写入规则 metadata，避免算法逻辑与规则引擎重复分裂。

4) 自动执行闭环
- 判定商户责任：自动商户退款或结算扣款。
- 判定骑手责任：自动扣押金并退款给用户。
- 判定用户碰瓷：拒赔/平台兜底拒绝触发，并加入限制名单。
- 平台兜底：自动退款由平台承担，不影响商户/骑手结算。

5) 申诉与资金回滚
- 申诉仅作为裁决结果复核的审计旁路，不参与主裁决流程。
- 申诉成功必须回滚资金（撤销原赔付/恢复扣款/调整结算）。
- penalizeClaimant 必须落地（恶意索赔用户扣分/封禁）。

6) 人工复核彻底移除
- ReviewClaim 入口停止使用或改为仅审计记录，不改变赔付结果。
- 所有索赔结果都由规则引擎决策。

7) 运营与淘汰策略
- 商户：异物/食安高发进入降权/熔断/淘汰。
- 骑手：餐损/超时高发进入暂停接单/淘汰。
- 通过规则引擎配置化，支持区域化阈值与灰度。

## 2. 裁决与追偿总流程
1. 用户提交索赔。
2. 规则引擎评估：产出责任方与赔付策略。
3. 平台先行赔付（自动退款）。
4. 生成追偿单（面向商户/骑手）。
5. 责任方支付或申辩；逾期 24 小时未支付且无说明 -> 自动限制接单。
6. 审计复核（旁路）：若纠错，执行资金回滚与恢复权限。

## 3. 规则引擎输出（必含）
- `responsible_party`: `merchant` | `rider` | `platform_fallback` | `unknown`
- `payout_strategy`: `platform_payout`（固定先赔付）
- `recovery_required`: `true|false`
- `recovery_target`: `merchant|rider`（当 `recovery_required=true`）
- `recovery_amount`（可由规则决定或与索赔金额一致）
- `decision_reason`（规则命中理由与行为指标摘要）

## 4. 行为指标（仅行为驱动）
### 用户维度
- 7/30 天异常索赔数与索赔率
- 高频索赔（短时窗口）
- 设备指纹/OPENID 关联风险

### 商户维度
- 异物/食安索赔率
- 短期集中投诉
- 近 30 天异常率

### 骑手维度
- 餐损/超时索赔率
- 近 7/30 天高频
- 超时集中

## 5. 追偿单设计
### 必要字段
- 追偿单ID、状态（pending/paid/overdue/waived/appealed）
- 关联订单ID与订单明细快照
- 责任方（商户/骑手）
- 索赔原因、索赔金额、追偿金额
- 裁决依据摘要（规则命中 + 行为指标）
- 到期时间（默认 24h）

### 追偿执行
- 商户：逾期 -> **暂停外卖接单**，堂食不受影响
- 骑手：逾期 -> **暂停接单**

## 6. 审计复核（旁路）
- 审计复核仅用于纠错
- 复核成立 -> 回滚追偿单与恢复权限
- 复核不成立 -> 追偿继续执行


## 7. 风险控制与淘汰策略
- 商户：食安/异物高发 -> 降权/熔断/淘汰
- 骑手：餐损/超时高发 -> 警示/暂停/淘汰
- 用户：高风险碰瓷 -> 限制服务或拒赔

## 8. 可落地改造清单（摘要）
- 移除证据字段与规则条件
- 规则引擎新增责任方/追偿输出
- 索赔流程改为平台先赔付
- 新增追偿单表与处理任务
- 逾期限制接单逻辑
- 审计复核与资金回滚逻辑

---

## 10. 执行计划（落地到业务链）

### 10.1 表结构调整（新增/复用/变更）
**新增表**
- `claim_recoveries` 追偿单（新增）
	- 关键字段：`id`, `claim_id`, `order_id`, `responsible_party`, `recovery_target`, `recovery_amount`, `status(pending/paid/overdue/waived/appealed)`, `due_at`, `decision_snapshot`, `created_at`, `updated_at`

**复用表**
- `claims`：继续作为索赔主表，补充 `decision_version`、`decision_reason`（如缺失）。
- `behavior_decisions` / `behavior_actions`：用于裁决记录与动作执行日志。

**可废弃用途**
- `behavior_evidence`、`claims.evidence_urls`：非审计必要，直接移除，避免误导后续维护。

### 10.2 规则与裁决逻辑改造
1. 规则引擎输出增加：`responsible_party`、`recovery_required`、`recovery_amount`、`decision_reason`。
2. 规则命中后：
	 - 若用户非碰瓷 -> **平台先赔付**（退款触发）。
	 - 若责任方为商户/骑手 -> **生成追偿单**。
	 - 若责任方为 `platform_fallback` -> 不生成追偿单。
3. “平台兜底”定义为**追偿归属为空**，不影响先赔付。

#### 10.2.1 默认规则模板（示例，可由运营调整）
> 说明：通过规则管理 API 写入；`action.meta` 会透传到裁决流程，用于生成追偿单与决策原因。

**规则 1：平台兜底（仅当三方均低风险）**
```json
{
	"scope": {"domain": "claim"},
	"condition": {
		"user_claims_7d_exceeded": false,
		"user_claims_30d_exceeded": false,
		"user_claim_rate_7d_exceeded": false,
		"user_claim_rate_30d_exceeded": false,
		"merchant_abnormal_rate_7d_exceeded": false,
		"merchant_abnormal_rate_30d_exceeded": false,
		"rider_abnormal_rate_7d_exceeded": false,
		"rider_abnormal_rate_30d_exceeded": false
	},
	"action": {
		"type": "platform-pay",
		"reason": "低风险平台兜底",
		"meta": {
			"responsible_party": "platform_fallback",
			"recovery_required": false,
			"decision_reason": "三方均低风险，平台兜底"
		}
	}
}
```

**规则 2：异物/食安 → 商户责任**
```json
{
	"scope": {"domain": "claim"},
	"condition": {"claim_type_in": ["foreign-object", "food-safety"]},
	"action": {
		"type": "auto",
		"reason": "商户责任索赔",
		"meta": {
			"responsible_party": "merchant",
			"recovery_required": true,
			"recovery_target": "merchant",
			"recovery_amount": 0,
			"decision_reason": "商户责任：异物/食安"
		}
	}
}
```

**规则 3：餐损/超时 → 骑手责任**
```json
{
	"scope": {"domain": "claim"},
	"condition": {"claim_type_in": ["damage", "timeout"]},
	"action": {
		"type": "auto",
		"reason": "骑手责任索赔",
		"meta": {
			"responsible_party": "rider",
			"recovery_required": true,
			"recovery_target": "rider",
			"recovery_amount": 0,
			"decision_reason": "骑手责任：餐损/超时"
		}
	}
}
```

### 10.3 业务链路嵌入点（避免摆设）
**索赔提交**（现有 `SubmitClaim`）：
- 在生成索赔后立即触发赔付（平台先行）。
- 根据规则输出写入 `claim_recoveries`（责任方为商户/骑手时）。

**追偿执行**（新增任务/定时器）：
- 追偿单 `due_at` 到期未支付 -> 自动限制接单。
- 商户：限制外卖接单，不影响堂食。
- 骑手：限制接单。

**申诉旁路**：
- 申诉成立 -> 回滚追偿单、恢复接单、必要时撤销赔付/追偿。

### 10.4 现有逻辑修改点（明确落地）
- `SubmitClaim`：改为固定平台先赔付 + 生成追偿单。
- `ReviewClaim`：不再作为主裁决入口，仅保留审计用途或下线。
- `ClaimAutoApproval`：从“直接扣款/押金扣款”改为“生成追偿单”。
- `task_process_appeal_result`：补齐追偿回滚/权限恢复逻辑。

### 10.5 迁移与灰度
1. 先新增追偿表与任务，不改动旧逻辑（旁路写入）。
2. 开启规则引擎输出与追偿单生成（灰度开关）。
3. 切换扣款/押金逻辑为追偿逻辑。
4. 下线人工复核入口。

## 9. 验收标准
- 95%+ 自动裁决覆盖率
- 追偿单生成成功率 ≥ 99%
- 逾期限制接单触发正确率 ≥ 99%
- 审计复核可回滚且全链路可追溯

---

## 附录：现有表/接口/算法清单（异常订单与索赔）

### A. 数据表（已存在，避免重复建表）
- `claims` 索赔记录表（含状态、裁决与回溯字段）[locallife/db/migration/000018_add_trust_score_system.up.sql](locallife/db/migration/000018_add_trust_score_system.up.sql)
- `appeals` 申诉表（商户/骑手对索赔申诉）[locallife/db/migration/000028_add_appeals.up.sql](locallife/db/migration/000028_add_appeals.up.sql)
- `food_safety_incidents` 食安事件表 [locallife/db/migration/000018_add_trust_score_system.up.sql](locallife/db/migration/000018_add_trust_score_system.up.sql)
- `fraud_patterns` 欺诈模式表（设备复用/地址聚类/协同索赔）[locallife/db/migration/000018_add_trust_score_system.up.sql](locallife/db/migration/000018_add_trust_score_system.up.sql)
- `behavior_decisions` 行为裁决记录 [locallife/db/migration/000094_add_behavior_trace_system.up.sql](locallife/db/migration/000094_add_behavior_trace_system.up.sql)
- `behavior_trace_snapshots` 行为追溯统计快照 [locallife/db/migration/000094_add_behavior_trace_system.up.sql](locallife/db/migration/000094_add_behavior_trace_system.up.sql)
- `behavior_actions` 行为动作执行记录 [locallife/db/migration/000094_add_behavior_trace_system.up.sql](locallife/db/migration/000094_add_behavior_trace_system.up.sql)
- `behavior_appeals` 行为裁决申诉记录 [locallife/db/migration/000094_add_behavior_trace_system.up.sql](locallife/db/migration/000094_add_behavior_trace_system.up.sql)
- `behavior_blocklist` 行为黑名单 [locallife/db/migration/000094_add_behavior_trace_system.up.sql](locallife/db/migration/000094_add_behavior_trace_system.up.sql)
- `user_claim_warnings` 用户索赔警告与平台垫付计数 [locallife/db/migration/000055_add_user_claim_warnings.up.sql](locallife/db/migration/000055_add_user_claim_warnings.up.sql)
- `abnormal_stats_daily` 异常索赔/订单统计（日聚合）[locallife/db/migration/000115_add_abnormal_stats_daily.up.sql](locallife/db/migration/000115_add_abnormal_stats_daily.up.sql)
- `platform_configs` 行为阈值配置 [locallife/db/migration/000094_add_behavior_trace_system.up.sql](locallife/db/migration/000094_add_behavior_trace_system.up.sql)

### B. 关键接口（已有路由）
**用户索赔**
- `POST /v1/claims` -> `SubmitClaim` [locallife/api/risk_management.go](locallife/api/risk_management.go#L128-L462)
- `GET /v1/claims` -> `ListUserClaims` [locallife/api/risk_management.go](locallife/api/risk_management.go#L1045-L1109)
- `GET /v1/claims/:id` -> `GetClaimDetail` [locallife/api/risk_management.go](locallife/api/risk_management.go#L1113-L1139)
- `PATCH /v1/claims/:id/review` -> `ReviewClaim`（拟废弃为裁决入口）[locallife/api/risk_management.go](locallife/api/risk_management.go#L487-L582)

**商户侧索赔/申诉**
- `GET /v1/merchant/claims` -> `listMerchantClaims` [locallife/api/appeal.go](locallife/api/appeal.go#L142-L220)
- `GET /v1/merchant/claims/:id` -> `getMerchantClaimDetail` [locallife/api/appeal.go](locallife/api/appeal.go#L228-L299)
- `GET /v1/merchant/claims/behavior-summary` -> `getMerchantClaimBehaviorSummary` [locallife/api/behavior_summary.go](locallife/api/behavior_summary.go#L93-L163)
- `POST /v1/merchant/appeals` -> `createMerchantAppeal` [locallife/api/appeal.go](locallife/api/appeal.go#L304-L392)
- `GET /v1/merchant/appeals` -> `listMerchantAppeals` [locallife/api/appeal.go](locallife/api/appeal.go#L393-L479)
- `GET /v1/merchant/appeals/:id` -> `getMerchantAppealDetail` [locallife/api/appeal.go](locallife/api/appeal.go#L480-L559)

**骑手侧索赔/申诉**
- `GET /v1/rider/claims` -> `listRiderClaims` [locallife/api/appeal.go](locallife/api/appeal.go#L560-L644)
- `GET /v1/rider/claims/:id` -> `getRiderClaimDetail` [locallife/api/appeal.go](locallife/api/appeal.go#L646-L729)
- `GET /v1/rider/claims/behavior-summary` -> `getRiderClaimBehaviorSummary` [locallife/api/behavior_summary.go](locallife/api/behavior_summary.go#L165-L235)
- `POST /v1/rider/appeals` -> `createRiderAppeal` [locallife/api/appeal.go](locallife/api/appeal.go#L730-L818)
- `GET /v1/rider/appeals` -> `listRiderAppeals` [locallife/api/appeal.go](locallife/api/appeal.go#L819-L899)
- `GET /v1/rider/appeals/:id` -> `getRiderAppealDetail` [locallife/api/appeal.go](locallife/api/appeal.go#L900-L979)

**运营商申诉处理**
- `GET /v1/operator/appeals` -> `listOperatorAppeals` [locallife/api/appeal.go](locallife/api/appeal.go#L992-L1088)
- `GET /v1/operator/appeals/:id` -> `getOperatorAppealDetail` [locallife/api/appeal.go](locallife/api/appeal.go#L1090-L1183)
- `POST /v1/operator/appeals/:id/review` -> `reviewAppeal` [locallife/api/appeal.go](locallife/api/appeal.go#L1191-L1328)

**食安与欺诈检测**
- `POST /v1/food-safety/report` -> `ReportFoodSafety` [locallife/api/risk_management.go](locallife/api/risk_management.go#L604-L688)
- `POST /v1/fraud/detect` -> `TriggerFraudDetection` [locallife/api/risk_management.go](locallife/api/risk_management.go#L691-L748)

### C. 核心算法/策略模块
- `ClaimAutoApproval` 自动裁决与行为分层 [locallife/algorithm/claim_auto_approval.go](locallife/algorithm/claim_auto_approval.go#L24-L548)
- `LookbackChecker` 行为回溯与相关性检查 [locallife/algorithm/lookback_checker.go](locallife/algorithm/lookback_checker.go#L1-L215)
- `FraudDetector` 团伙/设备/地址欺诈检测 [locallife/algorithm/fraud_detector.go](locallife/algorithm/fraud_detector.go#L1-L240)
- `FoodSafetyHandler` 食安举报评估与熔断 [locallife/algorithm/food_safety_handler.go](locallife/algorithm/food_safety_handler.go#L1-L214)
- `MerchantForeignObjectTracker` 商户异物追踪 [locallife/algorithm/merchant_foreign_object_tracker.go](locallife/algorithm/merchant_foreign_object_tracker.go#L1-L94)

### D. 异步任务与后处理
- 申诉结果处理与通知：`task_process_appeal_result` [locallife/worker/task_process_appeal_result.go](locallife/worker/task_process_appeal_result.go#L1-L188)
- 商户异物追踪/骑手餐损检查：`task_risk_management` [locallife/worker/task_risk_management.go](locallife/worker/task_risk_management.go#L1-L200)
