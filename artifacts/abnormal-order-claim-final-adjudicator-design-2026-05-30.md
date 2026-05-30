# 异常订单索赔终裁评分器设计

## 1. 背景

本文记录异常订单索赔的终裁实现方案，防止后续上下文切换时把该系统误解为人工审核、辅助告警或普通风控提示。

平台业务模式是：

1. 顾客下单后，平台把顾客委托代取任务推入抢单大厅。
2. 骑手接单后，与顾客形成代取委托关系；骑手不是平台雇佣配送员。
3. 骑手到店取餐时有义务确认餐品完整性。
4. 骑手一旦取走餐品，即代表餐品在取走时是完整的。
5. 到达顾客处发生餐损、撒漏、破损，原则上属于骑手代取履约责任。
6. 异物、餐品内部质量、商户出品问题，原则上属于商户销售责任。
7. 平台介入前，用户可以先与骑手或商户沟通；沟通不成后发起平台索赔。
8. 平台介入后，由系统自动终裁，不设置人工裁判。
9. 商户或骑手不服时，只展示系统裁决依据，不进入人工改判。

系统目标不是“判断是否给用户赔”。默认业务口径是：责任方明确时平台先赔用户，再向责任方追偿；如果用户被判为异常索赔用户，先警示用户，用户坚持继续时平台仍赔付，但限制该用户后续服务。

## 2. 当前生产事实

截至 2026-05-30 的生产与代码审查结论：

1. 生产 `RULES_ENGINE_ENABLED=true`。
2. 生产 `rules` 与 `rule_versions` 表为空，DB 规则当前不参与索赔裁定。
3. 生产已有 `behavior_trace.abnormal_thresholds`、`behavior_trace.alert_thresholds`、`behavior_trace.window_days` 等配置。
4. 生产已有 `abnormal_stats_daily` 数据，但当前主要用于统计、摘要、快照、告警和规则引擎输入。
5. 当前 `SubmitClaim` 主链会做订单归属、订单完成、重复索赔、金额上限、行为黑名单等硬校验。
6. 当前 `ClaimAutoApproval.EvaluateClaim` 主要按 `claim_type` 固定归责：超时和餐损归骑手，异物归商户，食安走独立链路。
7. 当前 `CreateClaimWithBehaviorTx` 会写 `claims`、`behavior_decisions`、`behavior_trace_snapshots`，并可基于强用户风险信号提升为 `user_restricted`。
8. 当前 `FraudDetector` 的设备复用、地址聚类、协同索赔只挂在管理员手动接口，不是用户提交索赔时的自动终裁链路。

因此，当前系统已有终裁链路的持久化基础，但裁定逻辑还没有形成完整的三方证据化终裁评分器。

## 3. 设计目标

新增一个确定性、可解释、可复盘的索赔终裁评分器。

评分器必须：

1. 同时读取顾客、骑手、商户三方历史行为。
2. 以业务类型基线责任为优先事实。
3. 使用规则化评分，不使用黑盒模型。
4. 只在强证据下覆盖基线责任。
5. 将本次裁定依据冻结到持久化对象，供后续展示和追偿使用。
6. 支持按区域灰度，例如先在宁晋县启用。
7. 与现有 `claims`、`behavior_decisions`、`behavior_trace_snapshots`、`claim_recoveries`、`behavior_blocklist` 体系衔接。

评分器不负责：

1. 直接发起微信或余额赔付。
2. 直接执行追偿扣款。
3. 人工复议或人工改判。
4. 食安群体事件处置。

## 4. 责任基线

责任基线是评分器的第一层事实，不能被普通弱统计轻易覆盖。

| claim_type | 中文语义 | 基线责任方 | 说明 |
| --- | --- | --- | --- |
| `timeout` | 代取超时 | `rider` | 骑手接受代取委托后承担履约时效责任。 |
| `damage` | 餐损、撒漏、破损 | `rider` | 骑手取餐即代表餐品完整，到达顾客处餐损归骑手。 |
| `foreign_object` | 异物、餐品内部异常 | `merchant` | 骑手无法合理验出餐品内部异物，归商户销售责任。 |
| `food_safety` | 食安问题 | 独立链路 | 不进入普通异常索赔终裁评分器。 |

如果未来新增类型，必须先定义基线责任方，再允许上线。

## 5. 裁定结果

终裁评分器输出的正式 decision mode 建议保持与现有事务层兼容：

1. `rider-recovery`
   - 骑手责任。
   - 平台先赔用户。
   - 用户确认继续后生成骑手追偿单。

2. `merchant-recovery`
   - 商户责任。
   - 平台先赔用户。
   - 用户确认继续后生成商户追偿单。

3. `user-restricted`
   - 用户异常索赔风险成立。
   - 先向用户展示警示。
   - 用户撤回则结束。
   - 用户坚持继续，平台赔付本次索赔，并限制该用户后续使用。

4. `default-recovery`
   - 内部分类，不一定需要作为数据库枚举。
   - 表示三方历史证据不足以覆盖基线责任。
   - 对外仍落到 `rider-recovery` 或 `merchant-recovery`。

## 6. 数据来源与 schema

本设计优先使用现有数据模型，不新建大模型或黑盒评分表。

### 6.1 输入数据来源

| 数据 | 来源 | 用途 |
| --- | --- | --- |
| 当前订单 | `orders` | 获取用户、商户、订单类型、金额、地址、完成时间等。 |
| 当前配送/代取 | `deliveries` | 获取骑手 ID、履约事实。 |
| 当前索赔 | request + `claims` | 获取索赔类型、金额、描述。 |
| 三方异常统计 | `abnormal_stats_daily` | 读取 user/merchant/rider 的 7/30 天订单数、异常索赔数、异常率。 |
| 行为效果统计 | `behavior_effects` 聚合查询 | 读取历史有效索赔、追偿、恶意确认、限制等。 |
| 设备关联 | user device 相关表 | 识别共享设备其他用户数。 |
| 地址关联 | address/order 相关表 | 识别共享地址其他用户数。 |
| 平台配置 | `platform_configs` | 读取窗口、阈值、灰度区域。 |

### 6.2 输出持久化

| 输出 | 表/字段 | 要求 |
| --- | --- | --- |
| 索赔主记录 | `claims` | 保存状态、批准金额、自动裁定原因。 |
| 终裁记录 | `behavior_decisions` | 保存责任方、赔付来源、decision mode、reason codes。 |
| 评分明细 | `behavior_decisions.score_breakdown` | 保存三方分数、命中信号、阈值、算法版本。 |
| 事实快照 | `behavior_decisions.fact_snapshot` | 冻结订单、三方窗口统计、关联信号、责任基线。 |
| 行为快照 | `behavior_trace_snapshots` | 保存 7/30 天窗口快照，供展示和复盘。 |
| 追偿单 | `claim_recoveries` | 用户确认继续后，对骑手或商户生成追偿。 |
| 限制记录 | `behavior_blocklist` | 用户坚持继续且命中 `user-restricted` 后限制后续服务。 |
| 审计 | `audit_logs` / `rule_hits` | 记录灰度、命中、异常和人工查看行为。 |

### 6.3 score_breakdown JSON schema

建议 `behavior_decisions.score_breakdown` 采用如下结构：

```json
{
  "version": "claim_final_adjudicator_v1",
  "region_id": 0,
  "claim_type": "damage",
  "base_responsible_party": "rider",
  "final_decision_mode": "rider-recovery",
  "scores": {
    "user_risk": {
      "score": 0,
      "level": "low",
      "signals": []
    },
    "rider_liability": {
      "score": 75,
      "level": "high",
      "signals": [
        {
          "code": "base_type_damage_rider",
          "weight": 50,
          "message": "餐损由取餐骑手承担基线责任"
        }
      ]
    },
    "merchant_liability": {
      "score": 0,
      "level": "low",
      "signals": []
    },
    "confidence": {
      "score": 80,
      "level": "high",
      "signals": [
        {
          "code": "base_responsibility_matched",
          "weight": 40,
          "message": "裁定符合索赔类型基线责任"
        }
      ]
    }
  },
  "thresholds": {
    "min_user_orders_30d": 5,
    "min_user_claims_30d": 3,
    "user_claim_rate_30d": 0.5,
    "merchant_abnormal_rate_30d": 0.08,
    "rider_abnormal_rate_30d": 0.06
  }
}
```

### 6.4 fact_snapshot JSON schema

建议 `behavior_decisions.fact_snapshot` 扩展为：

```json
{
  "order_id": 0,
  "claim_type": "damage",
  "claim_amount": 0,
  "base_responsible_party": "rider",
  "responsible_party": "rider",
  "decision_mode": "rider-recovery",
  "compensation_source": "rider",
  "windows": {
    "window_7d": {
      "start": "2026-05-23",
      "end": "2026-05-30"
    },
    "window_30d": {
      "start": "2026-04-30",
      "end": "2026-05-30"
    }
  },
  "user": {
    "id": 0,
    "orders_7d": 0,
    "claims_7d": 0,
    "claim_rate_7d": 0,
    "orders_30d": 0,
    "claims_30d": 0,
    "claim_rate_30d": 0,
    "malicious_confirmed_claims": 0,
    "shared_device_other_users": 0,
    "shared_address_other_users": 0
  },
  "rider": {
    "id": 0,
    "orders_7d": 0,
    "abnormal_claims_7d": 0,
    "abnormal_rate_7d": 0,
    "orders_30d": 0,
    "abnormal_claims_30d": 0,
    "abnormal_rate_30d": 0
  },
  "merchant": {
    "id": 0,
    "orders_7d": 0,
    "abnormal_claims_7d": 0,
    "abnormal_rate_7d": 0,
    "orders_30d": 0,
    "abnormal_claims_30d": 0,
    "abnormal_rate_30d": 0
  },
  "reason_codes": [
    "base_type_damage_rider"
  ]
}
```

## 7. 算法

### 7.1 计算方式

采用“基础统计预计算 + 终裁实时计算 + 裁定依据持久化冻结”。

1. `abnormal_stats_daily` 由触发器和每日回填维护。
2. 用户提交索赔时，实时读取三方 7/30 天窗口汇总。
3. 评分器在请求内完成确定性计算。
4. 计算结果与快照随 `CreateClaimWithBehaviorTx` 写入数据库。
5. 后续展示读取冻结快照，不重新计算历史裁定。

### 7.2 基线分

基线分确保业务类型责任优先。

| 条件 | 分数 | 说明 |
| --- | --- | --- |
| `timeout` | rider +60 | 超时为代取履约责任。 |
| `damage` | rider +70 | 餐损为骑手取餐后保管和交付责任。 |
| `foreign_object` | merchant +70 | 异物为商户出品责任。 |

基线分不能把用户判为异常；用户异常必须来自用户自身历史行为或关联风险。

### 7.3 用户风险分

用户风险分只在样本足够时生效。

建议阈值：

1. 30 天订单数 `< 5` 时，不允许仅凭索赔率判定用户异常。
2. 30 天索赔数 `< 3` 时，不允许仅凭索赔率判定用户异常。
3. 30 天订单数 `>= 5` 且索赔数 `>= 3` 且索赔率 `>= 0.5`，用户风险 +70。
4. 7 天索赔数 `>= 3` 且 7 天索赔率 `>= 0.5`，用户风险 +50。
5. 历史恶意确认数 `> 0`，用户风险 +100。
6. 共享设备或共享地址命中，并且净异常索赔数 `>= 3`，用户风险 +80。

用户风险达到强阈值时，允许覆盖基线责任为 `user-restricted`。

### 7.4 骑手责任分

骑手责任分由基线责任与历史异常共同组成。

建议规则：

1. `timeout` 或 `damage` 的基线分已给骑手。
2. 骑手 30 天完成单数 `>= 10` 且异常率 `>= rider_abnormal_rate_30d`，骑手责任 +30。
3. 骑手 7 天完成单数 `>= 3` 且异常索赔数 `>= 2`，骑手责任 +20。
4. 用户历史干净且骑手异常率超过阈值，置信度 +20。

骑手样本不足时，不降低基线责任，只是不增加历史异常分。

### 7.5 商户责任分

商户责任分由基线责任与历史异常共同组成。

建议规则：

1. `foreign_object` 的基线分已给商户。
2. 商户 30 天订单数 `>= 10` 且异常率 `>= merchant_abnormal_rate_30d`，商户责任 +30。
3. 商户 7 天订单数 `>= 3` 且异常索赔数 `>= 2`，商户责任 +20。
4. 用户历史干净且商户异常率超过阈值，置信度 +20。

商户样本不足时，不降低基线责任，只是不增加历史异常分。

### 7.6 终裁选择

终裁顺序：

1. 如果用户风险达到强阈值，输出 `user-restricted`。
2. 否则按 `claim_type` 基线责任输出 `rider-recovery` 或 `merchant-recovery`。
3. 历史责任分用于增强解释和置信度，不轻易推翻基线责任。
4. 只有当未来新增可证明的例外类型时，才允许历史分推翻基线责任。

这意味着当前版本不是“三方自由竞争最高分”。它是“类型基线责任 + 用户强异常覆盖 + 历史行为增强解释”的确定性终裁。

### 7.7 用户异常分支

当输出 `user-restricted`：

1. 提交索赔后返回警示。
2. 用户可以撤回索赔，流程结束。
3. 用户坚持继续时，平台赔付本次索赔。
4. 赔付完成后写入用户限制记录，后续服务拦截由现有行为黑名单承担。

该口径与现有系统原设定一致，不改为本次拒赔。

## 8. 配置

建议新增或复用 `platform_configs`：

```json
{
  "claim_final_adjudicator.enabled": true,
  "claim_final_adjudicator.gray_regions": [0],
  "claim_final_adjudicator.thresholds": {
    "min_user_orders_30d": 5,
    "min_user_claims_30d": 3,
    "user_claim_rate_30d": 0.5,
    "user_claims_7d": 3,
    "user_claim_rate_7d": 0.5,
    "min_rider_orders_30d": 10,
    "rider_abnormal_rate_30d": 0.06,
    "min_merchant_orders_30d": 10,
    "merchant_abnormal_rate_30d": 0.08
  }
}
```

宁晋县灰度时，`gray_regions` 写宁晋县真实 `region_id`。该 ID 必须从生产区域表查询，不能凭名称猜测。

## 9. 接入方案

### 9.1 新增组件

建议新增：

1. `locallife/algorithm/claim_final_adjudicator.go`
2. `locallife/algorithm/claim_final_adjudicator_types.go`
3. `locallife/algorithm/claim_final_adjudicator_test.go`

如果后续希望更清晰地遵守三层架构，也可把读取编排放在 `logic/claim_adjudication_service.go`，把纯算法放在 `algorithm/`。

### 9.2 SubmitClaim 接入

`SubmitClaim` 应保持职责边界：

1. 做请求校验、订单归属、状态、重复索赔、金额上限、行为黑名单。
2. 根据订单区域判断是否启用终裁评分器。
3. 启用时调用终裁评分器生成 `algorithm.Decision`、`score_breakdown`、`fact_snapshot`。
4. 未启用时继续走现有固定判责。
5. 调用 `CreateClaimWithDecisionAndEvidence` 或等价事务入口写入。

### 9.3 事务层接入

`CreateClaimWithBehaviorTx` 需要支持接收评分器输出：

1. `DecisionMode`
2. `ResponsibleParty`
3. `CompensationSource`
4. `ReasonCodes`
5. `TraceSummary`
6. `ScoreBreakdown`
7. `FactSnapshot`

当前事务层已有 `scoreBreakdownJSON := []byte({"version":"claims_rules_v1"})` 的占位行为，后续实现应改为写入真实评分明细。

### 9.4 DB rules 的角色

现有 `rules/rule_versions` 不作为终裁核心。

它适合保留用于：

1. 灰度开关。
2. 区域或商户范围控制。
3. 特殊活动或临时策略。
4. 审计 rule hit。

终裁核心不应依赖“第一条规则命中即返回”，否则三方同时异常时会出现优先级决定责任的偏差。

## 10. 展示与申诉

商户或骑手不服时，系统展示裁定依据，不人工改判。

展示内容应来自冻结数据：

1. 索赔类型与基线责任。
2. 当前订单事实。
3. 责任方 7/30 天异常率。
4. 用户侧是否命中高风险。
5. 命中 reason codes。
6. 追偿金额与追偿状态。

不得在展示时重新计算责任，否则后续数据变化会导致裁定依据漂移。

## 11. 误伤控制

误伤控制原则：

1. 不因小样本高比例直接判用户异常。
2. 用户异常必须满足最小订单数和最小索赔数。
3. 历史恶意确认、共享设备/地址重复索赔属于强信号。
4. 骑手/商户样本不足时，不因缺少历史异常而免除基线责任。
5. 三方证据不足时，按索赔类型基线责任处理。
6. 所有用户限制必须有冻结事实和 reason codes。

## 12. 验证策略

### 12.1 单元测试

至少覆盖：

1. `damage` 且用户正常，输出 `rider-recovery`。
2. `timeout` 且用户正常，输出 `rider-recovery`。
3. `foreign_object` 且用户正常，输出 `merchant-recovery`。
4. 用户 30 天样本不足，即使索赔率高，也不输出 `user-restricted`。
5. 用户 30 天订单数、索赔数、索赔率均超过阈值，输出 `user-restricted`。
6. 历史恶意确认，输出 `user-restricted`。
7. 共享设备/地址 + 重复异常，输出 `user-restricted`。
8. 骑手/商户历史异常只增强解释，不推翻明确的类型基线。

### 12.2 事务测试

至少覆盖：

1. 评分明细写入 `behavior_decisions.score_breakdown`。
2. 事实快照写入 `behavior_decisions.fact_snapshot`。
3. `rider-recovery` 继续能生成骑手追偿。
4. `merchant-recovery` 继续能生成商户追偿。
5. `user-restricted` 在用户确认继续后生成限制动作。

### 12.3 灰度验证

宁晋县灰度前检查：

1. 查询宁晋县真实 `region_id`。
2. 确认该区域最近 30 天订单、索赔、追偿数据可用于回放。
3. 以只读脚本回放历史索赔，比较新评分器与当前固定判责差异。
4. 差异样本人工审阅一次，但不把人工审阅设计为线上裁判。

灰度上线后观察：

1. `behavior_decisions.decision_mode` 分布。
2. `user-restricted` 命中率。
3. 骑手追偿单生成率。
4. 商户追偿单生成率。
5. 用户撤回率和坚持继续率。
6. 行为黑名单新增数。

## 13. 分阶段落地

### 阶段一：纯算法和回放

1. 新增评分器纯函数。
2. 基于现有数据构建输入 DTO。
3. 写单元测试。
4. 用生产历史数据只读回放，不改变线上结果。

### 阶段二：宁晋县灰度双写

1. 宁晋县启用评分器计算。
2. 暂不改变最终裁定，只把评分结果写入审计或 shadow 字段。
3. 比较当前结果与评分器结果。

### 阶段三：宁晋县正式裁定

1. 宁晋县使用评分器结果作为终裁。
2. `score_breakdown` 与 `fact_snapshot` 正式展示。
3. 商户/骑手追偿按评分器责任方生成。

### 阶段四：扩大区域

1. 根据灰度指标扩大到更多区域。
2. 调整阈值只通过配置，不改算法语义。
3. 保持每次裁定可复盘。

## 14. 待确认事项

1. 宁晋县 `region_id` 需要生产库确认。
2. 用户异常索赔限制时长是否永久，还是按配置天数。
3. `damage` 是否还需要细分为撒漏、包装破损、缺失；如果细分，默认仍归骑手。
4. 商户责任类型是否需要新增 `wrong_item`、`missing_item`、`quality_issue`。
5. 商户/骑手展示裁定依据的前端字段是否已经满足。
