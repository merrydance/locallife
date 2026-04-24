# 商户与骑手入驻自动审核重构建议

## 1. 目标

这份文档只处理一件事：

把商户和骑手入驻里的“OCR 识别”和“自动审核判定”重新拆清楚，解决当前线上总出现意外、但又说不清是 OCR 问题还是审核算法问题的现状。

它不是 OCR provider 选型文档，也不是微信进件状态机文档。

本轮只看本地入驻自动审核主链：

- 商户入驻自动审核
- 骑手入驻自动审核
- OCR worker 到审核判定之间的契约
- 失败分类、持久化、观测与人工兜底

不在本轮范围：

- 微信支付商户进件回调与签约状态机
- UI 视觉改造
- OCR provider 切换策略本身

补充说明：

- 本轮新增纳入“证照存续治理”，即证照在入驻通过后的到期提醒、过期停用、换证复审与恢复闭环。

## 2. 结论先说

当前问题的主因，不是单点 OCR provider 精度，而是系统把“OCR job 成功返回”和“证件已经可用于自动审核”混成了一层。

现状里至少有四个结构性问题：

1. OCR worker 只要拿到 provider 成功响应，就会把 OCR JSON 写成 done，但 done 不代表关键字段已经齐全、可解析、可判定。
2. 商户和骑手的自动审核逻辑直接消费业务表上的 OCR JSON，缺少一个独立的“识别可用性契约层”。
3. 审核失败的分类没有被稳定持久化。骑手只保留一条 reject_reason 字符串，商户很多失败甚至只在当次 HTTP 返回里存在。
4. 现有指标和测试主要覆盖 OCR job 成败，没有把“OCR 成功但字段不够”和“规则真的判拒”分开统计。

所以现在讨论“到底是 OCR 问题还是自动审核算法问题”，答案通常会变成：两边都有，但系统没有把边界立起来，导致无法归因。

## 2.1 当前实现进展（2026-04-22）

这份重构建议中，已有一部分被转成了仓库代码：

- review run 持久化与申请 `review_summary` 投影已落地。
- 审核通过后的当前生效证照台账与 `active_credentials` 响应投影已落地。
- activation 已从 handler 零散写库收口到独立的 credential governance service。
- 7 天到期 reminder scheduler 与过期暂停 enforcement scheduler 已接到治理主链。
- restore gate 与 owned release 已接到治理主链：只有矩阵满足且暂停仍由 `document_expired` 占有时才会自动恢复。
- restore notification 已接到治理主链。

但核心拆分目标还没有全部实现：

- OCR readiness 还没有成为独立、稳定的持久化契约。
- submit 仍然同步执行自动审核，并没有真正切成“提交 -> review run -> review worker”。
- OCR readiness 之外的治理闭环主链已接通；当前剩余主要缺口集中在 readiness 持久化与异步 review worker。

## 3. 现状代码证据

### 3.1 骑手自动审核直接吃 OCR JSON

骑手提交时，会在 [locallife/api/rider_application.go](locallife/api/rider_application.go#L551-L605) 里直接调用审批判断；实际规则实现在 [locallife/api/rider_application.go](locallife/api/rider_application.go#L658-L733)。

当前判断口径是：

- 身份证 OCR 是否已有姓名、身份证号、有效期
- 健康证 OCR 是否已有姓名、有效期
- 两者姓名是否一致
- 有效期是否还能覆盖阈值天数

这里没有独立的“识别完成但字段缺失 / 识别歧义 / 归一化失败”层，只有最终的业务 reject_reason。

### 3.2 商户自动审核是一个大而脆的同步函数

商户提交时会在 [locallife/api/merchant_application.go](locallife/api/merchant_application.go#L866-L932) 直接调用自动审核；主要规则都堆在 [locallife/api/merchant_application.go](locallife/api/merchant_application.go#L1022-L1231)。

当前规则把多种性质完全不同的判断混在一次同步提交里：

- 证件有效期
- 餐饮经营范围识别
- 营业执照地址与地图定位地址匹配
- 食品经营许可证主体名与营业执照主体名匹配
- 经营者姓名 / 法人姓名回退判断
- GPS 去重
- 营业执照号和身份证号唯一性检查

这意味着商户自动审核实际上不是“一个算法”，而是“OCR 结果解析 + 模糊比对 + 风控规则 + 去重策略”全塞在一个 handler 入口上。

### 3.3 OCR worker 把 done 当作 provider 成功，不是字段可用

骑手 OCR worker 在 [locallife/worker/task_rider_application_ocr.go](locallife/worker/task_rider_application_ocr.go#L339-L392) 和 [locallife/worker/task_rider_application_ocr.go](locallife/worker/task_rider_application_ocr.go#L479-L525) 中，只要 OCR job 成功，就把状态写成 done。

但同一段代码也会同时记录：

- has_name
- has_id_number
- has_valid_end
- has_health_cert_result

这说明代码自己已经承认：“job 成功”和“关键字段齐备”是两回事，只是这个差别没有进入业务契约。

商户 OCR worker 也是同样模式，见 [locallife/worker/task_merchant_application_ocr.go](locallife/worker/task_merchant_application_ocr.go#L147-L177)、[locallife/worker/task_merchant_application_ocr.go](locallife/worker/task_merchant_application_ocr.go#L228-L247)、[locallife/worker/task_merchant_application_ocr.go](locallife/worker/task_merchant_application_ocr.go#L301-L333)。

### 3.4 失败分类没有形成稳定数据面

骑手提交失败后，只会在 [locallife/api/rider_application.go](locallife/api/rider_application.go#L594-L605) 把申请退回 draft，并在 [locallife/db/query/rider_application.sql](locallife/db/query/rider_application.sql#L145-L154) 里落一个 reject_reason。

商户提交失败更弱：在 [locallife/api/merchant_application.go](locallife/api/merchant_application.go#L907-L917) 里如果 check 不通过，直接返回 BadRequest，根本不会形成一条稳定的审核失败记录。

所以线上即使出现大量误杀，也很难回答：

- 是 OCR provider 网络失败
- 是 OCR 成功但缺关键字段
- 是字段解析失败
- 是规则阈值太硬
- 还是确实存在风险命中

### 3.5 现有测试覆盖点不对焦

现有测试证明了一些局部格式兼容和字符串规则，例如：

- 骑手身份证日期格式兼容，见 [locallife/api/rider_application_test.go](locallife/api/rider_application_test.go#L785-L840)
- 商户食品经营许可证名称回退与不匹配场景，见 [locallife/api/merchant_application_test.go](locallife/api/merchant_application_test.go#L951-L1104)

但没有看到关键的体系化验证：

- OCR succeeded 但关键字段不全时，系统应归类成什么
- OCR succeeded 且字段齐全，但规则判拒时，应归类成什么
- 商户和骑手的自动审核失败分布，能否按 stage 和 reason_code 聚合
- OCR baseline 指标能否和申请最终通过率打通

## 4. 当前根因拆解

## 4.1 不是“一个问题”，而是三层问题被揉在一起

当前入驻自动审核里至少有三层：

1. OCR 执行层
2. OCR 结果归一化与字段可用性判定层
3. 业务审核规则层

现在系统对第 1 层做得相对最好：

- 有 ocr_job
- 有 provider 抽象
- 有 baseline 工具
- 有 worker 失败分类

但第 2 层几乎不存在成型契约，第 3 层又直接消费第 1 层的输出，所以归因被压扁了。

## 4.2 done 语义过宽，导致假成功

当前 OCR JSON 的 status=done 只表示：

- provider 调用了
- normalized_result 能解出来
- worker 成功回写了 JSON

它不表示：

- 关键字段都识别到了
- 日期已经可解析
- 主体名已完成归一化
- 该文档已经达到自动审核判定所需最低字段集

这正是“看起来 OCR 成功了，但自动审核还是失败”的根源之一。

## 4.3 商户规则把“应人工复核”当成“自动拒绝”

商户自动审核里，很多规则本质上不适合直接自动拒绝，而更像“需要人工复核”：

- 营业执照地址与逆地理结果不一致
- 食品经营许可证主体名只能通过 RawText 或经营者姓名回退
- 经营范围没有识别到明确餐饮关键词
- GPS 近邻去重命中但缺少更多上下文

这些场景当前都可能在提交同步路径里直接挡回去。这样做的后果不是更安全，而是把 OCR 模糊性、规则模糊性、真实风险三者全混成同一类拒绝。

## 4.4 骑手规则把“资料未识别完全”当成“用户自己填错”

骑手规则相对简单，但边界同样不清：

- 身份证背面有效期没识别到
- 健康证有效期存在但格式无法解析
- 健康证姓名识别模糊或被噪声污染

这些有些是文档质量问题，有些是 OCR/解析问题，有些才是用户证件真实不满足条件。

当前它们都通过同一条 reject_reason 返回给用户，业务上没有形成“请补传”“进入人工审核”“明确拒绝”的分流。

## 5. 目标设计

目标不是把规则写得更复杂，而是把自动审核重构成一个分层、可归因、可观测的闭环。

目标状态应当是：

1. OCR 执行成功不再直接等于业务可判定。
2. 每份证件都能输出“字段覆盖度”和“可审核 readiness”。
3. 自动审核只消费 readiness 和结构化字段，不再直接猜 raw OCR JSON。
4. 每次提交都能得到结构化失败分类，能明确落到 OCR、normalization、rule、risk、manual 五类之一。
5. 商户和骑手都具备“自动通过 / 自动拒绝 / 进入人工审核 / 等待补传”四种稳定结果，而不是只剩提交成功或报错。
6. 商户和骑手在入驻通过后，证照仍然有独立的生命周期治理，不再把“入驻时审核通过”误当成“后续一直有效”。

## 5.1 证照存续治理也要并进这轮设计

这条需求应该直接并进任务，而且应该被视为入驻闭环的一部分，不是后续附属功能。

原因很直接：

- 自动审核解决的是“证照首次提交是否可通过”。
- 证照到期治理解决的是“证照通过后是否仍持续有效”。

如果这两件事拆成完全无关的项目，最终会再次出现语义漂移：

- 入驻审核认定某张证照有效
- 但生产交易面没有持续校验和提醒
- 或者到期暂停了交易，但换证复审通过后没有稳定恢复路径

所以这条需求不应该只写成提醒任务，而应该纳入“上传 -> OCR -> 审核 -> 生效证照 -> 到期治理 -> 换证复审 -> 恢复”的完整链路。

## 6. 建议的新契约

## 6.1 证件识别结果分成三层

建议把单份证件结果拆成：

1. provider_result
2. normalized_fields
3. readiness

readiness 不是简单布尔值，建议至少包含：

- state: ready | partial | unreadable | invalid | provider_failed
- reason_code
- required_fields
- missing_fields
- unparseable_fields
- provider
- ocr_job_id

示意：

```json
{
  "status": "done",
  "ocr_job_id": 123,
  "provider": "aliyun",
  "fields": {
    "name": {"value": "张三", "state": "recognized"},
    "id_number": {"value": "110101199001011234", "state": "recognized"},
    "valid_end": {"value": "2035.01.01", "state": "recognized", "normalized": "2035-01-01"}
  },
  "readiness": {
    "state": "ready",
    "reason_code": "ok"
  }
}
```

如果 valid_end 原文存在但无法解析，应该是：

- OCR job succeeded
- field.state = unparseable
- readiness.state = partial

而不是简单 done 然后把问题丢给审核层。

## 6.2 自动审核结果需要结构化持久化

建议为商户和骑手都增加统一的审核运行快照。可以先用 JSONB 列低成本落地，而不是一上来建一套复杂中心表。

建议新增一类 review_summary / review_run 字段，至少包含：

- stage: ocr | normalization | review | risk | manual
- outcome: approved | rejected | needs_resubmit | needs_manual
- reason_code
- reason_message
- evidence
- ocr_job_refs
- rule_hits
- created_at

这样才能回答：

- 哪个 stage 最常失败
- 哪个字段最常缺失
- 哪种 reason_code 导致最多人工介入

补充：

- review_summary 只解决“这次审核怎么判”。
- 对于商户与骑手正式生效后的证照，还需要单独的 active_document_summary 或 credential_ledger，记录当前线上生效证照的到期日、来源申请、提醒时间、暂停时间和恢复时间。

## 6.3 提交应触发 review run，而不是同步大函数直接拍板

建议把提交改成：

1. 校验基础必填资料
2. 锁定当前提交快照
3. 创建 review run
4. 由 review worker 执行自动审核
5. 回写最终 outcome

这样做有三个直接收益：

- 不再把一次 HTTP 请求当成完整审核生命周期
- 审核可以重跑、补跑、复盘
- OCR 与规则算法都能在同一个 run 上归因

同时也为后续证照到期治理提供稳定输入：

- review run 通过后，系统把通过的证照快照投影成“当前生效证照”
- 到期提醒和自动暂停只扫描“当前生效证照”，而不是回头扫草稿或历史 OCR JSON

## 6.4 生效证照台账需要独立于申请草稿

建议把“申请草稿里的 OCR/审核结果”和“生产上当前生效的证照台账”拆开。

建议至少有一份统一的 credential ledger，哪怕第一阶段先用 JSONB 也可以。每条生效证照至少包含：

- subject_type: merchant | rider
- subject_id
- document_type
- source_application_id
- media_asset_id
- normalized_expiry_date
- readiness_state
- review_outcome
- effective_at
- expires_at
- last_reminded_at
- suspended_at
- suspension_reason_code
- resumed_at

这个台账是后续定时提醒、自动暂停、换证恢复的唯一可信输入。

## 6.5 到期提醒和自动暂停的目标规则

这条需求建议直接固定成平台规则，不放进可随意漂移的模糊算法里。

建议首版规则：

- 商户：营业执照、食品经营许可证进入当前生效台账后，距 expires_at 7 天时发送换证提醒。
- 骑手：至少健康证进入当前生效台账后，距 expires_at 7 天时发送换证提醒；身份证若业务确认也纳入证照治理，则使用同一机制。
- 证照到期当日后仍未有新的“审核通过并生效”的替代证照时：
  - 商户自动暂停交易功能
  - 骑手自动暂停接单功能
- 恢复条件不是“重新上传了图片”，而是“新证照已提交并重新审核通过，且已替换当前生效证照”。

这里要特别收紧一条边界：

- 提醒基于 expires_at
- 暂停基于当前生效证照是否已过期
- 恢复基于新 review run approved 后的台账切换

不允许直接依据草稿上传状态恢复交易或接单能力。

## 7. 商户与骑手的目标审核策略

## 7.1 骑手：强约束少，但要把“补传”和“拒绝”分开

骑手建议维持较强的自动审核，但分四类结果：

- auto_approved
  - 身份证姓名、证号、有效期齐全且有效
  - 健康证姓名、有效期齐全且有效
  - 姓名一致

- needs_resubmit
  - OCR 明确无法拿到关键字段
  - 图片模糊或证件边界不完整
  - 日期原文缺失

- needs_manual
  - OCR 有结果但存在模糊性，例如姓名轻微噪声、健康证样式特殊、日期格式异常但疑似有效

- auto_rejected
  - 证件明确过期
  - 姓名明确不一致
  - 命中明确业务硬拒规则

关键点：

- “缺字段”不要和“明确不合规”共用一套 reject_reason。
- “OCR succeeded but partial” 不应该被算成算法误杀，也不该直接算用户填错。
- 骑手证照到期治理要复用同一条 credential ledger；提醒和暂停是“生效后治理”，不与首次入驻审核混写在一个 handler 里。

## 7.2 商户：改成硬规则、软规则、人工复核三层

商户不适合再维持当前“一次性全自动拍板”的模型，建议拆成三层：

### 硬规则

命中即可自动拒绝：

- 营业执照过期
- 法人身份证过期
- 食品经营许可证过期且确认无误
- 营业执照号已被占用
- 法人身份证号已被占用
- 身份证姓名与营业执照法人明确不一致

### 软规则

命中后进入人工复核，不应自动拒绝：

- 地址匹配不稳定
- 经营范围没有稳定识别出餐饮关键词
- 食品经营许可证主体名只能依赖 RawText 回退
- 许可证主体名与营业执照名存在近似但非完全一致
- GPS 去重命中但缺少更多上下文

### 补传规则

命中后引导补传：

- 营业执照主体字段缺失
- 食品经营许可证有效期缺失
- 身份证背面有效期字段不可用

商户自动审核的核心目标，不应该是“尽量一次拒绝”，而应该是“把真正明确可判的通过/拒绝自动化，把模糊场景稳定送到人工池”。

补充到期治理口径：

- 商户“自动暂停交易功能”在当前代码面上优先对齐外卖交易入口的暂停能力，而不是重新发明一套状态。
- 现有仓库已经有 merchant_profiles.is_takeout_suspended 与 SuspendMerchantTakeout / UnsuspendMerchantTakeout，可作为首版交易暂停落点；见 [locallife/db/query/trust_score.sql](locallife/db/query/trust_score.sql#L75-L99)。
- 是否同步升级到 merchants.status = suspended，应作为二阶段治理决定；首版先以交易面拦截为准，避免与食安/人工风控的全量停业语义混用。

对骑手则直接复用 rider_profiles.is_suspended 与 SuspendRider / UnsuspendRider；见 [locallife/db/query/trust_score.sql](locallife/db/query/trust_score.sql#L158-L175)。

## 7.3 证照到期治理的运行方式

这部分不建议做成“用户访问时顺便检查”，而应该做成定时扫描任务。

原因：

- 提醒是时间窗口行为，不是请求时行为。
- 暂停必须具备幂等批处理能力。
- 恢复必须依赖 review run 通过后的台账切换，而不是用户刷新页面。

仓库里已有可复用的模式：

- DataCleanupScheduler 的“窗口扫描 + 发通知 + touch reminder ledger”模式，见 [locallife/scheduler/data_cleanup.go](locallife/scheduler/data_cleanup.go#L512-L588)。
- 现有 reminder 还配了去重和平台告警测试，见 [locallife/scheduler/rider_deposit_credit_scheduler_test.go](locallife/scheduler/rider_deposit_credit_scheduler_test.go#L43-L158)。

因此这条能力建议落成两个定时任务：

1. credential reminder scheduler
  - 每日扫描未来 7 天到期的当前生效证照
  - 发商户/骑手通知
  - 回写 last_reminded_at，避免重复提醒

2. credential expiry enforcement scheduler
  - 扫描已过期且仍未替换的当前生效证照
  - 商户调用 SuspendMerchantTakeout
  - 骑手调用 SuspendRider
  - 记录 suspension_reason_code=document_expired

恢复则不由 scheduler 主动猜测，而是在新的 review run 通过后，由 credential activation 流程主动：

- 切换当前生效证照
- 清除 document_expired 导致的暂停
- 发送恢复通知

## 8. 观测与评估改造

## 8.1 需要的指标

当前已有 OCR job 成功/失败观测，但缺申请级观测。建议新增：

- onboarding_review_runs_total{application_type,outcome,stage,reason_code}
- onboarding_review_duration_seconds{application_type}
- onboarding_document_readiness_total{application_type,document_type,readiness_state}
- onboarding_missing_field_total{application_type,document_type,field}
- onboarding_manual_queue_total{application_type,reason_code}

## 8.2 需要的日报口径

建议固定日报口径：

- OCR provider 失败率
- OCR job succeeded 但 readiness != ready 的占比
- auto approved / auto rejected / needs manual / needs resubmit 占比
- Top reason_code
- 商户与骑手各自的字段缺失 Top N

这样才能把“是 OCR 退化了”与“是规则过严了”区分开。

## 8.3 baseline 工具需要和入驻结果打通

仓库里已经有 OCR baseline 能力和样例，但它还停留在 OCR 视角，没有和申请结果打通。

下一步应让 baseline 至少能回答：

- 某份文档 OCR 成功率如何
- 在入驻自动审核里，它实际影响了哪个字段
- 最终是补传、人工复核、自动拒绝还是自动通过

对于证照存续治理，还应补一条台账级分析：

- 当前线上生效证照中，未来 7 天内到期多少
- 已过期未替换多少
- 因 document_expired 被暂停的商户和骑手有多少

否则 baseline 看起来健康，入驻还是会“经常出意外”。

## 9. 分阶段改造建议

## Phase 1：先把归因能力补起来

目标：不先重写算法，先让系统能回答“错在哪一层”。

建议动作：

- 为商户和骑手增加 review_summary JSONB
- OCR worker 回写字段级 state 和 readiness，而不是只有 done / failed
- 提交时把失败分类持久化，不再只返回临时报错
- 增加 application_type + reason_code 级指标

验收标准：

- 可以统计 OCR 成功但 readiness=partial 的比例
- 可以区分 needs_resubmit 与 auto_rejected
- 商户和骑手都能输出稳定 failure_code 分布
- 可以统计未来 7 天到期证照数、已过期未替换数、因证照过期被暂停的主体数

## Phase 2：把审核从同步 handler 抽到 review worker

目标：把审核生命周期从 HTTP 请求里拆出来，形成可重跑的 run。

建议动作：

- 提交时创建 review_run
- 新增 review worker
- worker 读取 OCR readiness + 业务快照后判定 outcome
- 人工复核和自动审核共用一套 outcome 枚举

验收标准：

- 同一申请可以重跑审核且有审计轨迹
- 商户和骑手提交接口不再承载完整审核逻辑
- review run approved 后可以稳定投影出当前生效证照，而不是只停留在申请表草稿字段

## Phase 3：重构商户规则分层

目标：把商户自动审核从大函数拆成硬规则、软规则、人工复核三层。

建议动作：

- 提炼规则 owner
- 给每条规则稳定 reason_code
- 把地址、经营范围、主体名比对从“直接拒绝”收敛成“明确拒绝 / 人工复核 / 补传”三选一

验收标准：

- 商户拒绝率下降不应依赖放水，而应来自正确分流
- 人工复核池有明确输入理由，不是杂糅报错

## Phase 3A：证照到期提醒与暂停闭环

目标：把“首次审核通过”扩成“持续有效治理”。

建议动作：

- 新增当前生效证照台账或等价投影
- 固定 merchant business_license / food_permit、rider health_cert 的 expires_at 来源
- 新增 7 天提醒 scheduler
- 新增过期强制暂停 scheduler
- 新增换证复审通过后的自动恢复逻辑

验收标准：

- 到期前 7 天会只提醒一次或按既定窗口提醒，不重复轰炸
- 到期后未替换的商户交易能力会被自动暂停
- 到期后未替换的骑手接单能力会被自动暂停
- 新证照复审通过后会自动恢复，不需要人工 SQL 修复

## Phase 4：用真实样本回标规则

目标：不要靠主观感觉调阈值。

建议动作：

- 抽取近一段时间商户和骑手入驻失败样本
- 按 OCR 问题、解析问题、规则问题、真实风险问题四类回标
- 用 reason_code 分布反推规则调整优先级
- 同时抽取证照即将到期、已过期暂停、换证恢复样本，校验提醒命中率和恢复闭环正确率

验收标准：

- 能明确列出 Top 10 误杀原因
- 能明确列出 Top 10 OCR 缺字段原因
- 每次规则调整前后有通过率、人工率、误杀率对比

## 10. 本轮最值得先做的事

如果只允许先做一轮最小但高价值改造，我建议顺序是：

1. 给商户和骑手补 review_summary 持久化。
2. 把 OCR done 语义收紧成“provider succeeded”，并新增 readiness。
3. 把商户提交失败从“HTTP 临时报错”改成“结构化审核结果”。
4. 给骑手和商户统一增加 reason_code 指标。
5. 把证照存续治理并进任务，不再留到“以后再补”。

这四步做完后，系统还没完全重写，但已经能准确区分：

- OCR provider 出错
- OCR 成功但字段不够
- 归一化失败
- 规则判拒
- 应进入人工审核

而把第 5 步并进后，系统还能继续回答：

- 当前线上哪些主体是因为证照过期被暂停
- 哪些提醒已经发送但还未换证
- 哪些主体已换证但尚未复审通过

到那时，再谈“重写算法”才会有真实依据，而不是继续靠体感调规则。

## 11. 当前判断

当前这条链路不适合继续做零散补丁式修修补补。

真正该重构的不是某一个正则、某一个日期解析、或者某一个名称回退，而是：

- OCR 成功语义
- 自动审核输入契约
- 审核失败持久化
- 人工兜底分流

只有先把这四个边界立起来，才能稳定回答一句现在最关键的话：

这次失败，到底是 OCR 的锅，还是自动审核算法的锅。