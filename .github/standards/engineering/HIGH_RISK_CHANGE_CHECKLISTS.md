# 高风险变更专项清单

> 作用：为最容易造成高影响事故的变更提供可直接执行的专项检查表。

本文件不是对 `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md` 的替代，而是其在高风险场景下的具体抓手。当前覆盖七类：payment、authz、media、OCR、callback、async recovery，以及 order / fulfillment / reservation / inventory 状态机。

## 1. 使用方式

当变更落入以下任一类时，把对应清单当作实现、review、验证和发布前检查的额外必查项：

1. 支付、退款、分账、提现、投诉补贴、回调、支付 worker、对账。
2. 认证、授权、对象级授权、租户边界、角色映射、Casbin policy、权限敏感 UI 操作。
3. 媒体上传、complete、访问控制、public/private bucket、证照可见性、媒体删除、业务绑定。
4. OCR job 创建、provider 路由、媒体读取、异步 worker 回写、重试、告警、人工介入。
5. 第三方回调、Webhook、结果通知、回调驱动入队、callback ack 语义。
6. recovery scheduler、outbox、补偿任务、结果轮询、dead-letter、人工作业兜底。
7. 订单、履约、配送池、预订、库存、超时取消、占用/释放与相关状态机。

## 2. Payment 变更清单

优先参考：

- `.github/standards/domains/wechat-payment/README.md`
- `.github/standards/domains/wechat-payment/WECHAT_PAYMENT_OPERATIONS_RUNBOOK_2026-03-24.md`
- `/memories/repo/payment-create-concurrency.md`

### 2.1 信任边界与输入

- 回调或下游结果是否经过签名校验、来源校验、必要字段校验。
- 是否有任何资金状态或支付结果只基于客户端传参决定。
- 是否有把 provider 原始错误或敏感字段直接返回给调用方或前端。

### 2.2 幂等与并发

- 重复下单、重复回调、重复退款申请、重复 worker 执行会发生什么。
- 并发创建支付时，是否可能把旧 pending 单据“复活”或覆盖掉新的预支付信息。
- 幂等键、唯一约束、状态条件更新是否真实生效，而不是只靠内存判断。

### 2.3 状态与账务

- 是否明确区分 pending、accepted、succeeded、failed、closed、refunded 等状态语义。
- 资金状态变化是否有持久化记录、审计记录和后续恢复依据。
- 部分成功时，补偿或人工修复路径是否明确。

### 2.4 失败模式

- 下单成功但本地写 prepay_id 失败时怎么办。
- provider 申请成功但本地事务失败时怎么办。
- provider 返回未知态、超时、重试或延迟回调时怎么办。
- 对账、补单、手工补偿入口是否可用。

### 2.5 运行与发布

- 监控是否能看见回调失败、重试耗尽、对账异常、补偿失败。
- 是否需要同步 payment runbook 或 domain docs。
- 回滚是否优先配置、代码、前端版本，而不是直接回滚 schema。

### 2.6 最低验证

- 至少验证一个正常路径。
- 至少验证一个重复执行或并发路径。
- 至少说明一个失败或未知态路径的剩余风险。

## 3. Authz 变更清单

优先参考：

- `.github/standards/backend/SYSTEM_PROMPT.md`
- `.github/standards/backend/API_CONTRACT_STANDARDS.md`
- `/memories/repo/audit-actor-role-taxonomy.md`

### 3.1 身份与授权边界

- 认证成功是否被错误地当成“有权限”。
- 授权是否依赖客户端传入 role、merchant_id、owner_id、operator_id、status 或其他可伪造字段。
- 对象级授权是否真正验证了“这个用户对这个对象有权操作”。

### 3.2 角色与策略来源

- 角色、租户和资源关系是否来自服务端可信数据源。
- Casbin policy、路由中间件、业务层二次校验是否语义一致。
- 审计日志里的 actor_role 是否被错误地当成 RBAC 真值来源。

### 3.3 多租户与越权

- 是否存在跨 merchant、跨 rider、跨 operator、跨用户对象访问。
- 批量接口、搜索接口、导出接口、下载接口是否遗漏对象级过滤。
- 内部 ID、私有材料、敏感状态是否被前端或小程序错误暴露。

### 3.4 客户端与展示层

- Web/Weapp 的权限禁用态是否仅作体验优化，而非唯一防线。
- 危险操作是否在 UI 上做了确认与禁用，但服务端仍保留最终授权校验。
- 权限失败时的用户可见文案是否符合 401/403 语义，而不是伪装成“暂无数据”。

### 3.5 失败模式与运行

- policy 漂移、角色映射错误、缓存未刷新时会怎样。
- 新路由、新 Swagger、新 policy 是否同步更新。
- 是否有足够日志帮助区分认证失败、授权失败、对象不存在、对象无权访问。

### 3.6 最低验证

- 至少验证一个有权限路径。
- 至少验证一个无权限或跨对象越权路径。
- 若变更涉及 Casbin、角色映射或租户边界，必须说明未验证的具体风险。

## 4. Media 变更清单

优先参考：

- `.github/standards/domains/media/README.md`
- `.github/standards/domains/media/MEDIA_API_CONTRACT_DESIGN_2026-03-18.md`
- `.github/standards/domains/media/MEDIA_TEST_AND_ACCEPTANCE_CHECKLIST_2026-03-18.md`
- `/memories/repo/media-document-visibility.md`

### 4.1 可见性与信任边界

- public 与 private bucket 的分类是否仍符合业务语义，而不是为了前端方便临时放宽。
- 营业执照、食品经营许可证等对外展示证照，是否仍受 approved 后才能暴露公开 URL 的门禁约束。
- 身份证正反面等敏感证件是否仍只允许 private bucket + 授权访问，不会通过 publicImageURL、batchPublicImageURLs 或兼容字段泄露。
- 终端、前端或小程序是否能仅凭 object_key、media_asset_id 或旧 URL 越过服务端授权直接访问私有材料。

### 4.2 上传、complete 与绑定一致性

- upload session、complete、resource_bind 是否形成完整闭环，而不是只创建对象不落业务绑定。
- complete 的幂等语义是否明确，重复 complete 不会重复创建媒体或重复绑定业务对象。
- 是否仍以 media_asset_id 作为业务主链，而不是重新引入旧 URL、旧 image_path 或业务表裸 object_key。
- 媒体 ownership、tenant、subject 校验是否来自服务端可信关系，而不是客户端自报分类或归属。

### 4.3 访问控制与生命周期

- GET、private-access、删除、批量读取、缩略图 URL 生成是否都做了对象级权限校验。
- 软删除、物理删除、被业务引用中、moderation 未通过、upload 未完成等状态语义是否清晰且一致。
- CDN 公共图、私有签名访问、规格图派生是否仍遵守统一 contract，而不是某个调用面绕过了标准路径。
- 列表页、卡片页、缩略图位是否仍避免默认读取 original 图，防止性能或隐私回退。

### 4.4 失败模式与运行

- OSS/CDN/对象元数据读取失败时，错误语义是否稳定，是否会把存储层波动伪装成“无数据”。
- complete 成功但业务绑定失败、业务删除成功但异步删对象失败、moderation 状态回写失败时，恢复路径是否明确。
- 审计日志是否足以追踪谁上传、谁访问、谁删除、谁签发了私有访问地址。
- 是否需要同步 media domain docs、验收清单或迁移说明，避免文档与当前 contract 漂移。

### 4.5 最低验证

- 至少验证一个 public 资源成功访问路径。
- 至少验证一个 private 资源无权限访问被拒路径。
- 至少验证一个 complete 幂等或重复绑定路径。
- 若涉及证照可见性或 bucket 分类调整，必须说明未验证的暴露风险。

## 5. OCR 变更清单

优先参考：

- `.github/standards/domains/ocr/README.md`
- `.github/standards/domains/ocr/OCR_OPERATIONS_RUNBOOK_2026-03-25.md`
- `.github/standards/domains/ocr/OCR_ALIYUN_RAM_STS_MIN_PERMISSION_2026-03-25.md`
- `.github/standards/domains/ocr/OCR_BASELINE_EVALUATION_2026-03-25.md`
- `/memories/repo/ocr-refactor-foundation.md`

### 5.1 输入契约与媒体读取

- OCR 主链是否仍以 media_asset_id / ocr_job_id 为准，不会回退到旧 image_path、URL 回读或 handler 直连 provider。
- document_type、owner_type、side、media_asset_id 的组合是否合法且完整，避免把错误任务投递进统一 worker。
- 服务端媒体读取是否仍走 `ReadMediaAsset` 等受控抽象，而不是重新开放终端可见的签名 URL 回读链路。
- 私有证件读取是否仍限定在服务端内部字节流路径，不把敏感证件暴露给外部 provider 调试链路或日志。

### 5.2 Provider、凭证与路由

- Provider 路由是否与当前配置和 document capability 一致，不会把不支持的证件错误送到某 provider。
- 阿里云凭证是否仍遵守最小权限原则，且未把主账号 AK/SK、共享高权限凭证或未实现的 STS 模式误带入生产。
- provider 原始响应、错误详情、证件敏感字段是否被过度记录到日志、告警或 API 响应。
- provider 切换或能力调整时，是否同步了基线评估、runbook 或告警口径。

### 5.3 作业状态、重试与回写

- `ocr_jobs` 的 queued、processing、succeeded、failed、cancelled、next_retry_at、attempt_count 语义是否保持一致。
- stale lease 回收、重复 worker 领取、重复回写、人工重试等并发场景是否仍安全。
- 业务 OCR JSON 的 status、error_code、ocr_job_id、alert_emitted_at 等投影字段是否与主表一致，而不是出现“主表失败、业务表看似成功”。
- 不同证件 front/back 或多次上传场景下，是否会错误覆盖另一面的 OCR 结果或把旧结果写回新资产。

### 5.4 失败模式、告警与人工介入

- 可重试与不可重试错误分类是否仍准确，避免把权限问题当临时抖动重试风暴化。
- 达到 `max_attempts`、provider 限流、媒体不存在、权限不足、坏图等场景，告警是否符合 runbook 定义。
- 人工介入路径是否明确，包括 dead-letter 查询、按 job 重试、重新上传、修配置后的补发策略。
- 是否有任何代码路径通过伪造业务 OCR JSON“补成功”而绕过统一 OCR job 状态机。

### 5.5 最低验证

- 至少验证一个 OCR 正常成功闭环。
- 至少验证一个可重试失败或 lease 恢复路径。
- 至少验证一个不可重试失败或重试耗尽告警路径。
- 若涉及 provider 路由、凭证、任务状态机或业务回写语义，必须说明未验证的具体剩余风险。

## 6. Callback 变更清单

优先参考：

- `.github/standards/domains/wechat-payment/WECHAT_PAYMENT_OPERATIONS_RUNBOOK_2026-03-24.md`
- `/memories/repo/payment-create-concurrency.md`
- `/memories/repo/profit-sharing-recovery-closure.md`

### 6.1 真值来源与信任边界

- 回调是否经过签名校验、来源校验、事件类型校验与必要字段校验。
- 当回调载荷字段不完整、含糊或带兼容旧字段时，是否明确了真正的 source of truth，而不是直接信任回调体。
- 是否存在“本地查不到对象但仍返回 success ack”从而导致上游停止重试的危险分支。
- provider 查询接口与回调字段冲突时，是否优先使用当前权威查询结果，而不是沿用遗留 fallback。

### 6.2 入队、事务与 ack 语义

- callback 收到后，关键状态变化、审计锚点或补偿锚点是否先持久化，再返回 ack。
- 若 callback 只负责入队异步任务，入队失败时是否有指标、平台告警与 recovery 覆盖，而不是静默丢失。
- 是否存在“上游收到 success，但本地事务或任务没有任何 durable 记录”的空洞窗口。
- 回调处理中的外部查询、DB 更新、任务投递是否定义了清晰顺序，避免部分成功后不可恢复。

### 6.3 幂等、乱序与重复投递

- 重复回调、延迟回调、先查询后回调、先 recovery 后回调、不同事件乱序到达时会发生什么。
- 终态是否不可逆，非终态是否允许基于权威查询继续收敛。
- 回调驱动的入队或状态更新是否有幂等键、唯一约束或条件更新保护。
- 是否错误地把旧回调重新写回新状态，导致已关闭、已 supersede 或已完成记录被“复活”。

### 6.4 可观测性与人工介入

- 告警、日志和审计是否携带足够标识定位具体主记录，同时避免泄露 provider 敏感字段。
- runbook 是否说明了 callback 入队失败、结果不一致、上游反复重试时的人工处理步骤。
- 是否保留了可重复执行的补发任务、重查上游、人工 reconciliation 路径，而不是要求直接改数据库终态。
- 若引入新 callback 路径、新事件类型或新 ack 规则，是否同步 runbook 与 review 关注点。

### 6.5 最低验证

- 至少验证一个正常 callback 闭环。
- 至少验证一个重复投递或乱序到达路径。
- 至少验证一个入队失败、查询失败或需要返回 retry 的失败路径。
- 若变更涉及 ack 语义、上游真值来源或幂等收敛，必须说明未验证的具体剩余风险。

## 7. Async Recovery 变更清单

优先参考：

- `.github/standards/domains/wechat-payment/WECHAT_PAYMENT_OPERATIONS_RUNBOOK_2026-03-24.md`
- `/memories/repo/claim-refund-outbox.md`
- `/memories/repo/profit-sharing-recovery-closure.md`

### 7.1 扫描范围与恢复锚点

- recovery scheduler、outbox 或补偿任务扫描的对象，是否来自 durable 状态锚点，而不是运行时推断或易漂移的派生条件。
- 恢复 payload 是否基于真正的主记录，例如 payment_order、refund_order、behavior_action、ocr_job，而不是偷懒只带 order_id。
- processing、running、failed、created、pending 等中间态语义是否能支撑继续收敛，而不是让任务永远卡住或被错误跳过。
- 对需要先 query 再 reconcile 的对象，是否避免盲目重入主处理函数造成 no-op 或错误终态。

### 7.2 并发、租约与重复执行

- 多实例 scheduler 并发扫描、重复领取、stale lease 回收、人工补发与自动重试并发发生时是否安全。
- recovery 执行是否有幂等条件、claim/lease、唯一键或条件更新，而不是靠“应该只跑一次”的假设。
- 当上次执行已经部分成功时，重新恢复是否会重复退款、重复分账、重复补贴或重复通知。
- scheduler 启动顺序、注册状态、暂停恢复后重启等场景是否会造成积压或跳过窗口。

### 7.3 失败分类与收敛策略

- 模糊错误、超时、上游处理中等未知态，是否标记为待轮询或待恢复，而不是直接 final-fail。
- enqueue 失败、query 失败、补偿失败时，是否写入审计、告警或下一次扫描依据。
- 终态关闭、人工放弃、永久失败是否有清晰退出条件，避免无限重试风暴。
- 人工修复后，自动 recovery 是否还能正确识别已修复状态，而不是再次覆盖人工结果。

### 7.4 运行与发布

- 关键 recovery scheduler 是否在启动、部署和发布检查中被显式确认已注册并在正常扫描。
- 指标、日志、平台告警是否能看见 backlog、重试耗尽、恢复命中率、长期 processing 堆积。
- runbook 是否说明“等一个调度周期再复查”与“超过几个周期必须人工介入”的边界。
- 新增 recovery 路径时，是否同步了调度频率、运行手册、发布检查表和 review 关注点。

### 7.5 最低验证

- 至少验证一个 recovery 命中并推动状态收敛的路径。
- 至少验证一个重复扫描、重复执行或 stale lease 恢复路径。
- 至少验证一个 query 失败、enqueue 失败或未知态继续轮询的失败路径。
- 若变更涉及补偿锚点、扫描范围、租约或终态退出条件，必须说明未验证的具体剩余风险。

## 8. Order / Fulfillment / Reservation / Inventory 变更清单

优先参考：

- `locallife/docs/order-money-chains-review.md`
- `locallife/docs/production-robustness-review-report.md`
- `.github/standards/backend/BACKEND_RISK_MAP.md`

### 8.1 状态机与前置条件

- 订单、配送、预订、库存、delivery pool 等状态迁移是否显式检查当前状态，而不是只按 `id` 直接覆盖。
- 商户履约状态、配送状态和订单主状态是否保持一致，不会出现局部成功、全局矛盾的半状态。
- 终态是否不可逆；若允许回退，是否有明确补偿语义和审计依据。

### 8.2 排他性与并发

- “单预订唯一活跃订单”“单配送池项唯一抢单成功者”“库存不能超卖”“同一资源不能被重复占用”等约束是否落实在事务、条件更新或数据库约束层。
- 并发双击、重复提交、多端同时操作、worker/recovery/callback 乱序到达时，最多会成功几次，失败方如何被稳定识别。
- 若条件更新或 claim 失败，是否返回明确 conflict 语义，而不是静默吞掉。

### 8.3 资源与资金联动

- 下单、预订、配送或取消是否会联动库存、优惠券、余额、押金、delivery pool、打印、通知等下游状态；这些联动是否有同事务保护或明确补偿边界。
- 超时取消、商户拒单、用户取消、替单、配送失败、退款成功后，库存/押金/占券/预订资源是否会正确释放或回收。
- 如果某条链路当前依赖 recovery 或补偿任务兜底，是否明确写出主路径失败后的收敛方式。

### 8.4 扫描、列表与运营热路径

- 商户订单列表、骑手待抢单列表、预订列表、库存页、恢复扫描是否评估过排序稳定性、分页策略和索引支撑，而不是默认用高 offset 或全表聚合。
- delivery pool、timeout/recovery 扫描、库存过期清理等 query 是否带有清晰 scope、时间锚点或状态过滤，而不是全表粗扫。

### 8.5 最低验证

- 至少验证一个合法状态推进路径。
- 至少验证一个并发冲突、重复执行、乱序到达或 stale-state 路径。
- 至少说明一个恢复、释放、补偿或人工介入方向的剩余风险。

## 9. 使用结果要求

当 payment、authz、media、OCR、callback、async recovery 或 order/fulfillment/reservation/inventory 变更交付时，至少应在说明中写清：

1. 套用了哪一类专项清单。
2. 哪几项已验证。
3. 哪几项仍是剩余风险。
4. 是否需要同步 runbook、policy、docs 或 workflow。