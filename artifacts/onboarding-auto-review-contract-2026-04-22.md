# 入驻自动审核与证照治理正式契约稿

## 目标

固定商户与骑手入驻自动审核、证照持续治理、暂停与恢复的基础契约，作为后续 migration、SQL、logic、worker、scheduler 和 API 投影的唯一设计输入。

这份契约稿优先冻结：

- subject/document 治理矩阵
- readiness / review outcome / suspension reason_code
- 暂停归属释放规则
- 恢复门禁规则

## 当前落地注记（2026-04-22）

以下契约项已按当前仓库实现落地：

- `review_summary` 已进入商户与骑手申请响应。
- OCR worker 已开始在现有 OCR JSON payload 中回写 `readiness.state`、`reason_code`、`required_fields`、`missing_fields`，submit 前置校验会优先消费这些状态。
- rider submit 已改为“create review run -> enqueue onboarding review worker -> enqueue failed 时 sync fallback”主链，worker 与同步回退共用同一个 rider review owner。
- merchant submit 已改为“create review run -> enqueue onboarding review worker -> enqueue failed 时 sync fallback”主链，worker 与同步回退共用同一个 merchant review owner。
- 审核通过后的持续治理证照已投影到 `credential_ledgers`，并通过 `active_credentials` 摘要对外暴露。
- active ledger 切换使用事务 helper，旧 active 会先失活，再写入新的 active 行。
- 7 天到期 reminder scheduler 已落地，会发送用户通知并回写 `last_reminded_at`。
- expired enforcement scheduler 已落地，会按 `document_expired` 抢占暂停位并回写 ledger `suspended_at`。
- restore gate 已按 required credential matrix 落地；仅当当前 active ledger 满足矩阵且暂停仍由 `document_expired` 占有时，才会尝试自动恢复。
- restore notification 已落地，会在 owned release 成功后发送 subject/document/outcome 元数据齐全的系统通知。
- api 层旧的 rider 同步拍板 helper 已移除，审核规则 owner 现已收口到 logic 层 shared service，不再保留第二套生产实现。
- api 层旧的 merchant 同步拍板 helper 已移除，商户审批主路径现已收口到 logic 层 shared service，不再保留第二套生产实现。

以下契约项仍是“已冻结但尚未完整实现”的约束，不应被误读为已交付：

- OCR readiness 还未形成独立 read model；当前先持久化在现有 OCR JSON payload 内。
- merchant submit gate 的纯文档规则判定已下沉到 logic shared service；当前仍由 API 层持有的是原始 OCR JSON 解码、food permit repair，以及地图反查地址匹配和库查询去重，因此还不能视为 `D3` 完成。
- review 仍会从现有 OCR JSON 读取结构化字段，尚未切成完全独立的 readiness snapshot 输入。

## 当前事实锚点

当前商户自动审核的 submit gate 已由 [locallife/api/merchant_application.go](locallife/api/merchant_application.go#L1042) 负责原始 OCR 解码、repair、地址匹配和去重，再调用 [locallife/logic/merchant_document_review.go](locallife/logic/merchant_document_review.go#L48) 执行纯文档规则判定。

当前骑手自动审核直接依赖身份证 OCR 与健康证 OCR，见 [locallife/api/rider_application.go](locallife/api/rider_application.go#L653)。

因此本契约把“首审所需证件”和“持续治理所需证件”拆开定义，避免再次混淆“首次通过”和“持续有效”。

## 一、职责边界

### 1. 上传层

- 只负责媒体资产归属、草稿绑定、删除与替换。
- 不负责 OCR 成败语义。
- 不负责自动审核拍板。

### 2. OCR 层

- 只负责 provider 调用、结构化字段归一化、字段级状态和 readiness 回写。
- `status=done` 仅表示 provider 调用成功，不表示业务可审核。

### 3. Review 层

- 只消费结构化字段、readiness、业务快照和风险检查结果。
- 输出结构化 `review_run` 或 `review_summary`。
- 不直接读取原始 provider payload 做业务猜测。

### 4. Credential Governance 层

- 只负责当前生效证照台账、到期提醒、过期暂停、换证复审后的恢复。
- 不负责草稿编辑。
- 不直接修改首次审核规则。

## 二、治理矩阵

### 2.1 文档类型枚举

- `business_license`
- `food_permit`
- `id_card_front`
- `id_card_back`
- `health_cert`

### 2.2 主体类型枚举

- `merchant`
- `rider`

### 2.3 首审与持续治理矩阵

| subject_type | document_type | 首次提交必需 | 首次自动审核必需 | 持续治理必需 | 说明 |
| --- | --- | --- | --- | --- | --- |
| merchant | business_license | 是 | 是 | 是 | 商户持续经营资质 |
| merchant | food_permit | 是 | 是 | 是 | 商户持续经营资质 |
| merchant | id_card_front | 是 | 是 | 否 | 首次准入校验使用，首版不进入持续治理 |
| merchant | id_card_back | 是 | 是 | 否 | 首次准入校验使用，首版不进入持续治理 |
| rider | id_card_front | 是 | 是 | 否 | 首次准入校验使用，首版不进入持续治理 |
| rider | id_card_back | 是 | 是 | 否 | 首次准入校验使用，首版不进入持续治理 |
| rider | health_cert | 是 | 是 | 是 | 骑手持续履约资质 |

### 2.4 恢复门禁矩阵

| subject_type | 自动恢复前必须满足的当前生效证照 |
| --- | --- |
| merchant | `business_license` 与 `food_permit` 均存在 active 生效行，且均未过期 |
| rider | `health_cert` 存在 active 生效行，且未过期 |

约束：

- 首版不把 `id_card_front` / `id_card_back` 纳入持续治理恢复门禁。
- 后续若要纳入，必须修改本契约和测试矩阵，不能隐式扩张。

## 三、OCR 与 Readiness 契约

### 3.1 字段状态枚举 `field_state`

- `recognized`: 原文识别成功，且归一化成功
- `missing`: 应有字段未识别到值
- `unparseable`: 原文存在，但归一化失败
- `raw_only`: 仅保留原文，当前不能安全作为规则输入

### 3.2 可审核状态枚举 `readiness_state`

- `ready`: 规则所需字段完整且可安全消费
- `partial`: OCR 成功，但部分关键字段缺失或无法解析
- `unreadable`: 图片质量或文本质量过差，关键字段大面积不可读
- `invalid`: 上传内容不是目标证照，或结构明显不合法
- `provider_failed`: provider 调用失败或结果不可用

### 3.3 Readiness 原因码枚举 `readiness_reason_code`

- `ok`
- `required_field_missing`
- `field_unparseable`
- `document_unreadable`
- `unsupported_document`
- `provider_timeout`
- `provider_error`
- `normalization_failed`

约束：

- `readiness_state=ready` 时，`readiness_reason_code` 必须为 `ok`。
- `readiness_state!=ready` 时，`readiness_reason_code` 不允许为 `ok`。
- 自动审核只读 `readiness_state`、`readiness_reason_code` 和结构化字段，不直接消费原始 OCR JSON。

### 3.4 单证照 OCR 结果结构

```json
{
  "status": "done",
  "ocr_job_id": 123,
  "provider": "aliyun",
  "fields": {
    "valid_end": {
      "value": "2035.01.01",
      "normalized": "2035-01-01",
      "state": "recognized"
    }
  },
  "readiness": {
    "state": "ready",
    "reason_code": "ok",
    "required_fields": ["valid_end"],
    "missing_fields": [],
    "unparseable_fields": []
  }
}
```

## 四、Review Run 契约

### 4.1 审核阶段枚举 `review_stage`

- `ocr`
- `normalization`
- `review`
- `risk`
- `manual`

### 4.2 审核结果枚举 `review_outcome`

- `approved`: 自动通过，可进入生效证照投影
- `rejected`: 明确不满足准入规则
- `needs_resubmit`: 资料问题，用户需补传或更换资料
- `needs_manual`: 不适合自动拍板，进入人工复核

### 4.3 审核原因码命名规则 `review_reason_code`

命名采用 snake_case，并按责任来源分组。第一阶段至少冻结以下前缀：

- `readiness_*`: OCR/归一化未达到可审核状态
- `rule_*`: 明确业务规则不满足
- `risk_*`: 风险或去重类命中
- `manual_*`: 需要人工复核的边界场景

第一阶段最低必备代码集合：

- `readiness_required_field_missing`
- `readiness_field_unparseable`
- `readiness_document_unreadable`
- `readiness_provider_failed`
- `rule_document_expired`
- `rule_name_mismatch`
- `rule_address_mismatch`
- `rule_non_catering_scope`
- `rule_health_cert_too_soon`
- `rule_id_card_expired`
- `risk_duplicate_location`
- `manual_food_permit_name_ambiguous`
- `manual_address_ambiguous`

约束：

- `rejected` 只能搭配 `rule_*` 或明确的 `risk_*`。
- `needs_resubmit` 只能搭配 `readiness_*`。
- `needs_manual` 只能搭配 `manual_*`，以及经评审允许进入人工队列的模糊 `risk_*`。

### 4.4 审核运行最小结构

```json
{
  "stage": "review",
  "outcome": "needs_manual",
  "reason_code": "manual_food_permit_name_ambiguous",
  "reason_message": "食品经营许可证主体名无法稳定自动判定",
  "ocr_job_refs": [101, 102],
  "rule_hits": ["merchant.food_permit.name_match"],
  "created_at": "2026-04-22T10:00:00Z"
}
```

## 五、Credential Governance 契约

### 5.1 生效证照台账唯一语义

对同一主体、同一证照类型，在任一时间最多只允许一条 active 生效记录。

唯一键语义：

- `(subject_type, subject_id, document_type, active=true)` 唯一

### 5.2 到期提醒语义

- 只针对 active 生效证照执行
- 默认窗口为到期前 7 天
- 同一窗口内不重复提醒同一条 active 记录

### 5.3 过期暂停原因码 `suspension_reason_code`

本闭环第一阶段只冻结一个可自动释放的机器码：

- `document_expired`

约束：

- 证照治理触发暂停时，必须把治理侧 reason_code 记录为 `document_expired`。
- 第一阶段如果复用 `merchant_profiles.takeout_suspend_reason` / `rider_profiles.suspend_reason` 作为占位字段，则写入值也必须是 `document_expired`。
- 任何非 `document_expired` 的暂停值，一律视为外部流程占有，不允许由证照治理自动释放。

### 5.4 Owned Release 规则

自动恢复必须同时满足：

1. 当前暂停位仍由 `document_expired` 占有。
2. 当前主体 required credential matrix 已全部满足。
3. 本次恢复依赖的 active 生效证照来自最新已通过的 review run 投影。

任一条件不满足，则禁止自动恢复。

### 5.5 恢复结果约束

- 商户仅恢复证照治理自己占有的外卖交易暂停位。
- 骑手仅恢复证照治理自己占有的接单暂停位。
- 不得清除食安、人工合规、追偿、天气或其他来源的暂停。

## 六、状态映射规则

### 6.1 Readiness 到 Review Outcome

| readiness_state | 默认 review_outcome | 说明 |
| --- | --- | --- |
| ready | 继续执行业务规则 | 不直接等于 approved |
| partial | needs_resubmit | 首版默认要求补传 |
| unreadable | needs_resubmit | 图片或文本不可读 |
| invalid | needs_resubmit | 上传内容错误 |
| provider_failed | needs_resubmit | 首版先回到补传/重试闭环 |

### 6.2 规则命中到 Review Outcome

| 命中类型 | review_outcome |
| --- | --- |
| 明确过期、明确主体不一致、明确不满足经营范围 | rejected |
| OCR 不完整、字段无法解析、资料模糊 | needs_resubmit |
| 模糊主体名、模糊地址、需要人工上下文判断 | needs_manual |

## 七、API 投影约束

- 草稿申请响应可暴露 OCR readiness 和最近一次 `review_summary`。
- 当前线上治理状态必须从生效证照台账投影，不得从草稿 OCR 字段直接推断。
- 前端可同时看到“草稿证照识别结果”和“当前线上生效证照状态”，但两者必须分区展示、语义分离。

## 八、非目标

- 本契约不直接定义 UI 视觉形态。
- 本契约不覆盖微信进件、运营商合同或其他非入驻证照治理域。
- 本契约不在第一阶段重构全量商户状态机。

## 九、冻结结论

本契约冻结以下四项，后续实现不得隐式改义：

- subject/document 治理矩阵
- `readiness_state` / `readiness_reason_code`
- `review_outcome` / `review_reason_code` 前缀规则
- `suspension_reason_code=document_expired` 与 owned release 恢复门禁