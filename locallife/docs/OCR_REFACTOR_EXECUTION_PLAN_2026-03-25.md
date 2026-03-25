# OCR 改造执行计划（由当前代理执行）

## 1. 文档目的

本文档用于固定本轮证件图审核与 OCR 改造的已确认结论、当前实现现状、后续阶段目标、执行顺序与可勾选任务清单，避免在持续改造过程中出现规则漂移、阶段遗漏或实现偏离。

约束如下：

- 本文件是当前 OCR 改造的执行基线，后续新增结论应先更新本文档，再继续改代码。
- 所有任务必须满足“能勾选、能验收、能回滚”。
- 对未上线业务做生产级收敛，不保留无必要的旧路径兼容、双轨 OCR 入口和长期本地文件依赖。
- 证件可见性、审核门禁、OCR 入口与异步任务边界必须统一设计，不能各条业务线各自演化。
- 服务尚未上线且无旧数据包袱，本次改造按一次性切净原则执行，不保留任何业务壳层、兼容窗口、双轨 provider 调用或旧式同步 OCR 主链路。
- 开发过程可以按业务线分阶段提交，但每完成一条业务线迁移，就同步删除该业务线旧 OCR 入口、旧 worker payload 与旧路径读取代码；主分支最终态不允许两套链路并存。

## 2. 当前已确认的业务结论

### 2.1 图审与公开展示规则

- 系统生成二维码属于 `source_client=server` 资产。
- 系统生成二维码不进入图片安全审核流。
- 系统生成二维码创建后默认标记为 `approved`。
- 对外可展示图片必须同时满足：`visibility=public` 且 `moderation_status=approved`。
- “是否需要审核”与“是否允许公开展示”是两条独立规则，不能混用。

### 2.2 证件可见性规则

- 营业执照是用户可见证件，归类为 public 媒体。
- 食品经营许可证 / 小餐饮小作坊登记证是用户可见证件，归类为 public 媒体。
- 身份证正反面是真正私密证件，归类为 private 媒体。
- 身份证只要求“用户本人可看 + OCR 可用”，不要求管理员可看。
- 身份证私有访问目前已收紧为 owner-only，而非 admin-readable。

### 2.3 OCR 适用证件范围

当前核心 OCR 证件范围如下：

- 营业执照
- 身份证
- 食品经营许可证 / 小餐饮小作坊登记证

当前代码中还存在扩展场景：

- 骑手健康证

### 2.4 OCR 未来目标规则

- 所有 OCR 主入口最终统一收敛到 `media_asset_id`。
- OCR 服务端内部读取图片时，必须基于媒体对象读取能力，不再依赖本地 uploads 路径。
- OCR 执行模型最终统一为“创建任务 -> 异步处理 -> 回写结构化结果”。
- Provider 与文档类型路由必须抽象，不允许在 handler 内直接堆积第三方调用细节。
- 阿里云 OCR 作为主 provider，微信 OCR 仅作为后续可选的第二 provider，不作为当前主实现依赖。
- 原始供应商响应与归一化结构化结果都要保留，便于审计与后续切换供应商。

## 3. 当前代码状态

### 3.1 已完成项

- [x] 删除二维码旧 uploads 公共路径兼容分支。
- [x] 删除二维码旧路径兼容测试。
- [x] 系统生成二维码资产创建后显式标记为 `approved`。
- [x] `publicImageURL` 与 `batchPublicImageURLs` 收紧为仅 public + approved 才返回 URL。
- [x] 营业执照与食品经营许可证的媒体策略已改为 public。
- [x] 身份证正反面继续保持 private。
- [x] 身份证私有访问已收紧为仅 owner 可访问。
- [x] 已完成针对图审门禁、公开 URL、身份证访问控制、二维码生成的定向测试。

### 3.2 当前 OCR 现状

- 微信 OCR 已有实现，位于 `wechat/ocr.go`。
- 商户、运营商、骑手三条申请线的 OCR 入口分散在各自 handler 中。
- 当前 OCR 仍是双入口混用：`multipart file` 与 `media_asset_id` 并存。
- 当前 OCR 执行模型不统一：部分同步请求内执行，部分异步任务执行。
- 当前多个 OCR 链路仍通过 `mediaAssetLocalPath` 或本地文件路径取图。
- food permit OCR 已有异步雏形，但任务边界仍偏旧。

### 3.3 当前主要问题

1. OCR 入口不统一，业务 handler 里同时处理上传、鉴权、取图、供应商调用、解析与回写。
2. 图片读取仍耦合本地路径，不适合作为最终媒体架构下的长期方案。
3. 文档类型与供应商能力没有抽象层，扩展或切换 provider 时会继续复制逻辑。
4. 同步 OCR 与异步 OCR 并存，客户端行为和失败语义不一致。
5. 原始 OCR 结果、归一化结果、任务状态没有统一持久化模型。
6. 身份证这类私密证件虽然已收紧访问，但 OCR 链路内部仍需显式保证不走公开 URL。

## 4. Provider 方向结论

### 4.1 阿里云 OCR 的定位

- 作为本次 OCR 改造的主 provider。
- 优先承接营业执照、身份证、食品经营许可证 / 小餐饮小作坊登记证、健康证等核心证件识别。
- 所有新 OCR 主链路都以阿里云 provider 为默认执行路径。
- 接入方式必须经过统一 provider 抽象，不允许业务 handler 直接调用阿里云 SDK 或签名逻辑。

### 4.2 微信 OCR 的定位

- 作为第二 provider 的预留实现，而不是当前主实现。
- 保留其已有专用证照识别能力的接入价值，但不作为本轮上线依赖项。
- 只有在统一 provider 抽象稳定后，才评估是否接入为补充能力、对照能力或应急切换能力。

根据阿里云 OCR 文档首页与 API 概览，当前可见能力分组包括：

- OCR 统一识别
- 通用文字识别
- 个人证照识别
- 票据凭证识别
- 企业资质识别
- 车辆物流识别
- 教育场景识别
- 小语种文字识别
- 医疗场景识别
- 票证核验

对当前项目最相关的能力分组是：

- 个人证照识别
- 企业资质识别
- 通用文字识别
- OCR 统一识别

### 4.3 路由原则

目标路由策略建议如下：

- 营业执照：优先阿里云企业资质识别能力。
- 身份证：优先阿里云个人证照识别能力。
- 食品经营许可证 / 小餐饮小作坊登记证：优先阿里云企业资质识别或通用文字识别能力。
- 健康证：优先阿里云通用文字识别能力。
- 微信 provider 不参与当前主链路路由；若后续启用，只作为显式配置的第二 provider，而不是隐式兼容回退。

### 4.4 阿里云 OCR 接入与配置原则

根据阿里云 OCR 开发参考，OCR OpenAPI 采用 RPC 签名风格，官方提供 SDK，也支持自签名调用。因此服务端接入时必须提供“可用于 OpenAPI 签名的凭证”，而不是匿名调用。

配置原则固定如下：

- 不使用阿里云主账号 AccessKey 直接落业务配置。
- 优先使用 RAM 用户最小权限 AccessKey，或等价的 RAM 角色临时凭证方案。
- 如果部署环境支持实例角色 / STS / OIDC 等免长钥方案，优先使用临时凭证而不是长期 AccessKey。
- OCR 凭证只保存在服务端配置中，不下发客户端。
- provider 配置必须独立于微信 OCR 配置，不允许混用或隐式复用。

若采用最直接的服务端 SDK 接入，至少需要以下配置项：

- `ALIYUN_OCR_ENABLED`
- `ALIYUN_OCR_ENDPOINT`
- `ALIYUN_OCR_REGION`
- `ALIYUN_OCR_ACCESS_KEY_ID`
- `ALIYUN_OCR_ACCESS_KEY_SECRET`

若采用 RAM 角色临时凭证方案，建议扩展为：

- `ALIYUN_OCR_STS_ENABLED`
- `ALIYUN_OCR_ROLE_ARN`
- `ALIYUN_OCR_ROLE_SESSION_NAME`
- `ALIYUN_OCR_ROLE_EXTERNAL_ID`

权限原则：

- 仅授予 OCR OpenAPI 调用所需最小权限。
- 不把 OSS、短信、其他 AI 产品权限和 OCR 权限绑在同一组高权限凭证上。
- AccessKey 或临时凭证的轮换策略要写进运维手册。

### 4.5 Provider 启用原则

- 当前上线目标只要求阿里云 provider 跑通完整主链路。
- 微信 provider 不作为当前上线阻塞项。
- 若后续接入微信 provider，必须通过统一 provider 接口接入，不允许恢复旧式 `wechat/ocr.go -> handler` 直连模式。
- 是否启用第二 provider，由独立配置项显式控制，不允许在代码中隐式 fallback。

## 5. 目标最终态

### 5.1 数据模型

新增统一 OCR 任务表 `ocr_jobs`，至少包含：

- `id`
- `idempotency_key`
- `document_type`
- `provider`
- `provider_task_id`
- `media_asset_id`
- `owner_type`
- `owner_id`
- `status`
- `attempt_count`
- `max_attempts`
- `next_retry_at`
- `leased_at`
- `lease_owner`
- `error_code`
- `error_message`
- `raw_result`
- `normalized_result`
- `result_version`
- `retention_until`
- `requested_by`
- `created_at`
- `started_at`
- `finished_at`
- `updated_at`

建议状态枚举：

- `pending`
- `processing`
- `succeeded`
- `failed`
- `cancelled`

必要约束：

- 同一业务语义下必须存在幂等键，防止重复创建 OCR 任务。
- worker 领取任务必须具备 lease 语义，防止并发重复消费。
- 重试必须具备次数上限与下一次重试时间，不能无限重试。
- `raw_result`、`normalized_result`、错误信息都应允许审计与问题排查。

幂等键规则固定如下：

- 默认幂等键由 `media_asset_id + document_type + owner_type + owner_id + side` 组成。
- 同一媒体资产、同一证件类型、同一业务主体只创建一个 OCR 任务。
- 用户重新上传图片后会生成新的 `media_asset_id`，因此会创建新任务，而不是复用旧任务。
- 用户点击“重试”时，只增加同一 `ocr_job` 的 `attempt_count`，不创建新幂等键。

### 5.2 服务分层

建议新增 `ocr` 包，职责如下：

- `ocr/provider.go`：定义 provider 接口。
- `ocr/provider_aliyun.go`：阿里云 OCR provider 主实现。
- `ocr/provider_wechat.go`：微信 OCR provider 第二 provider 预留实现。
- `ocr/router.go`：按 document type 路由 provider。
- `ocr/service.go`：统一创建任务、执行任务、查询结果、回写业务。
- `ocr/parser.go`：将 provider 原始结果解析为统一结构。
- `ocr/projector.go`：将统一结构投影回 merchant/operator/rider 各业务模型。

强制边界如下：

- 允许的调用路径：`handler -> ocr.Service`。
- 禁止的调用路径：`handler -> ocr.Provider`。
- 禁止的调用路径：`handler -> aliyun SDK` 或 `handler -> wechat SDK`。
- 所有 provider 调用、错误映射、重试策略都必须收敛在 `ocr` 包内。

### 5.3 图片读取能力

服务端内部 OCR 读取图片时统一走媒体对象读取能力，不直接依赖本地路径。建议能力如下：

```go
type BinaryReader interface {
    ReadMediaAsset(ctx context.Context, mediaAssetID int64) ([]byte, string, error)
}
```

要求如下：

- public 媒体不通过公开 URL 回读。
- private 媒体不生成公开可见地址来给 OCR 使用。
- 本地开发环境与对象存储环境都通过同一抽象读取字节流。
- 身份证等 private 证件只允许服务端读取字节流后直接上传给 provider；如果某 provider 只支持 URL 模式，则该模式不得用于 private 证件。

### 5.4 接口模型

最终对外 OCR 接口应统一为：

- 请求：提交 `media_asset_id` 与必要业务上下文。
- 响应：返回 `ocr_job_id`、任务状态与可轮询结果。

建议明确为统一 OCR 提交接口族，而不是继续让 merchant/operator/rider 各自发散：

- `POST /v1/ocr/jobs`
    - 用途：创建 OCR 任务。
    - 请求字段：`document_type`、`media_asset_id`、`owner_type`、`owner_id`、`side`、`idempotency_key`。
    - 响应字段：`ocr_job_id`、`status`、`provider`、`created_at`。
- `GET /v1/ocr/jobs/:id`
    - 用途：查询单个 OCR 任务状态。
    - 响应字段：`ocr_job_id`、`status`、`document_type`、`provider`、`error_code`、`error_message`、`started_at`、`finished_at`。
- `GET /v1/ocr/jobs/:id/result`
    - 用途：查询 OCR 归一化结果。
    - 响应字段：`ocr_job_id`、`status`、`normalized_result`、`result_version`。
- `POST /v1/ocr/jobs/:id/retry`
    - 用途：对失败且允许重试的任务发起重试。
    - 限制：只允许任务所有者或具备审核权限的后台调用。
- `POST /v1/ocr/jobs:batch-query`
    - 用途：批量查询多个 OCR 任务状态，减少列表页逐条轮询。

业务线对外接口的长期形态应为：

- 前端如需独立发起 OCR，统一调用 `/v1/ocr/jobs` 接口族。
- merchant/operator/rider 业务 handler 只允许内部调用 `ocr.Service`，不再暴露各自独立的 OCR 对外接口。
- 业务详情接口按需回传当前关联的 `ocr_job_id`、`ocr_status` 和结构化结果摘要。

未上线场景下不保留业务侧 OCR 壳层接口，例如：

- `POST /v1/merchant/application/foodpermit/ocr`
- `POST /v1/operator/application/license/ocr`
- `POST /v1/rider/application/idcard/ocr`

这些旧接口应直接删除或改造为统一 OCR 接口族的调用方，而不是继续作为对外 API 存活。

本次未上线场景下，执行原则进一步固定为：

- 旧业务侧 OCR 对外接口直接删除。
- 若业务保存流程需要触发 OCR，由业务 handler 内部调用 `ocr.Service`，而不是保留旧 `/.../ocr` 路由。

不再作为长期模型保留：

- 直接上传文件并在同一个请求里同步完成 OCR。
- 在 handler 里拼接本地临时路径后直接调用第三方 OCR。
- merchant/operator/rider 各自维护一套对外 OCR 提交接口。

### 5.5 生产级保证

如果目标真的是把 OCR 链路做到 10 分，还必须补齐以下生产级保证：

- 任务幂等：相同证件、相同媒体、相同业务上下文不能无限重复创建任务。
- 并发安全：同一任务不能被多个 worker 并发执行并重复回写。
- 失败恢复：区分可重试失败、不可重试失败、人工介入失败。
- 结果可审计：保留 provider 原始结果、归一化结果、最终投影结果与更新时间。
- 隐私治理：身份证等高敏信息的原始结果要定义留存期、访问边界与脱敏策略。
- 观测闭环：必须能看到成功率、耗时、失败码、重试次数、堆积量。
- 发布可控：要有一次性切换顺序、回滚原则与验收脚本。
- 质量可量化：要有样本集、基线准确率、回归对比，而不是只看“能跑通”。

结果留存分层如下：

- `ocr_jobs` 保存任务状态、错误信息和必要的结果索引字段。
- `raw_result` 对身份证等高敏信息必须支持脱敏访问与自动过期删除。
- 审计日志不保存完整敏感原文，只记录任务创建、执行、完成、失败及访问行为。

## 6. 分阶段改造清单

## 阶段 A：固定基线与文档收口

- [x] A1. 固定证件可见性规则。
- [x] A2. 固定二维码不进审核流的规则。
- [x] A3. 固定 public URL 必须 public + approved 的门禁。
- [x] A4. 固定 OCR 统一收敛到 `media_asset_id` 的目标。
- [x] A5. 固定阿里云为主 provider。
- [x] A6. 固定微信仅为后续可选第二 provider，而非当前主实现。
- [x] A7. 将本轮结论沉淀到执行文档。

验收标准：

- 所有后续改造都有统一文档基线，不再依赖口头上下文。

## 阶段 B：建立 OCR 基础设施

- [ ] B1. 新增 `ocr_jobs` migration。
- [ ] B2. 新增 sqlc 查询与模型。
- [ ] B3. 新增 `ocr` 包基础目录与接口。
- [ ] B4. 定义 provider 接口、router、service 基础结构。
- [ ] B5. 定义统一 normalized result 结构。
- [ ] B6. 定义 document type 枚举与 owner type 枚举。
- [ ] B7. 增加基础单元测试。

验收标准：

- 不改现有业务 handler 的前提下，已有独立 OCR 基础设施可供接入。

## 阶段 C：统一媒体读取能力

- [ ] C1. 在媒体层补充服务端内部二进制读取能力。
- [ ] C2. 优先复用对象存储下载能力，而非本地路径拼接。
- [ ] C3. 为 local / OSS 两种环境补齐一致实现。
- [ ] C4. 为 private 证件读取补充权限边界与内部调用约束。
- [ ] C5. 增加读取能力测试。

验收标准：

- OCR 服务内部可以只依赖 `media_asset_id` 读取字节流，不需要本地文件路径。

## 阶段 D：接入阿里云 Primary Provider

- [ ] D1. 实现 `ocr/provider_aliyun.go`。
- [ ] D2. 接入营业执照识别能力。
- [ ] D3. 接入身份证识别能力。
- [ ] D4. 接入食品经营许可证 / 健康证所需的通用识别能力。
- [ ] D5. 统一阿里云 provider 错误映射。
- [ ] D6. 保留原始响应，输出 normalized result。
- [ ] D7. 增加 provider 级测试。

验收标准：

- 阿里云 OCR 不再由各 handler 直接调用，而是通过 `ocr.Service` 驱动。

## 阶段 E：先迁移商户食品经营许可证 OCR

- [ ] E1. 将商户 food permit OCR 改为“创建 `ocr_job`”。
- [ ] E2. worker 改为消费 `ocr_job_id` 而不是旧路径参数。
- [ ] E3. 在 worker 中通过媒体读取能力拉取图像。
- [ ] E4. 通过 provider 执行 OCR。
- [ ] E5. 通过 parser / projector 回写业务字段。
- [ ] E6. 调整 API 返回为异步任务语义。
- [ ] E7. 补充集成测试。

验收标准：

- 商户 food permit OCR 成为第一条完整跑通的新架构链路。

## 阶段 F：迁移营业执照与身份证 OCR

- [ ] F1. 迁移商户营业执照 OCR。
- [ ] F2. 迁移商户身份证 OCR。
- [ ] F3. 迁移运营商营业执照 OCR。
- [ ] F4. 迁移运营商身份证 OCR。
- [ ] F5. 迁移骑手身份证 OCR。
- [ ] F6. 收口旧同步 OCR 入口。
- [ ] F7. 补齐业务回归测试。

验收标准：

- 核心证件 OCR 主路径全部进入统一任务模型。

## 阶段 G：迁移扩展证件与收尾

- [ ] G1. 迁移骑手健康证 OCR。
- [ ] G2. 将 remaining printed-text 证件统一接入。
- [ ] G3. 删除 `mediaAssetLocalPath` 在 OCR 主链路中的依赖。
- [ ] G4. 直接删除旧 multipart OCR 主入口。
- [ ] G5. 更新 Swagger 与相关设计文档。
- [ ] G6. 增加运维说明与问题排查手册。

验收标准：

- OCR 主链路不再依赖旧式本地路径和分散实现。

## 阶段 H：接入第二 Provider 能力

- [ ] H1. 评估并实现微信 OCR provider。
- [ ] H2. 为适合的证件建立第二 provider 路由策略。
- [ ] H3. 明确第二 provider 的启用方式、切换方式和禁用方式。
- [ ] H4. 做 provider 结果差异校验。
- [ ] H5. 增加微信 provider 配置项与启动校验。
- [ ] H6. 补充第二 provider SDK 封装测试。
- [ ] H7. 明确第二 provider 仅通过统一 provider 抽象接入。

验收标准：

- 文档类型与 OCR provider 已解耦，供应商切换不影响业务 handler。
- 阿里云为主 provider，微信如启用也只是通过统一抽象接入。

## 阶段 I：质量、稳定性与观测补强

- [ ] I1. 固定 `ocr_jobs` 状态机与状态流转约束。
- [ ] I2. 增加任务 lease / 抢占 / 超时回收机制。
- [ ] I3. 增加任务幂等键与重复提交去重策略。
- [ ] I4. 增加可重试错误分类与退避重试策略。
- [ ] I5. 增加死信任务或人工介入队列策略。
- [ ] I6. 增加 OCR 结构化日志、metrics、关键告警。
- [ ] I7. 增加原始结果与敏感字段的脱敏/留存策略。
- [ ] I8. 增加审计记录，记录谁在什么业务上下文触发了 OCR。
- [ ] I9. 增加失败注入测试、并发重复消费测试、幂等测试。
- [ ] I10. 建立 OCR 样本集与回归评估基线。

验收标准：

- OCR 链路不仅能跑通，而且在重试、并发、异常和观测层面达到生产级稳态。

## 阶段 J：一次性切换、验收与回滚

- [ ] J1. 定义统一 OCR 接口族替换旧 OCR 入口的实施顺序。
- [ ] J2. 在代码层直接删除旧 OCR 对外接口与旧 worker 入参模型。
- [ ] J3. 编写测试环境端到端联调脚本。
- [ ] J4. 编写生产发布步骤与回滚原则。
- [ ] J5. 明确 OCR 任务表与业务回写表的回滚边界。
- [ ] J6. 补充上线后验收 checklist。

验收标准：

- 统一 OCR 接口族一次性替换旧入口，代码库内不残留旧 OCR 主链路。

## 7. 小任务执行清单

本节用于后续逐项勾选执行。原则是每个任务尽量控制在一次提交或一轮实现内完成。

### 7.1 当前最优先执行的小任务

- [ ] T0. 固定 Aliyun 主 provider 的文档类型到 API 能力映射。
- [ ] T1. 新增 `ocr_jobs` 表 migration 文件。
- [ ] T2. 为 `ocr_jobs` 补充 `.down.sql`。
- [ ] T3. 在 SQL 查询层增加 create / update / get / list 查询。
- [ ] T4. 重新生成 sqlc。
- [ ] T5. 新增 `ocr/types.go`，定义 document type、status、normalized result。
- [ ] T6. 新增 `ocr/provider.go`，定义 provider 接口。
- [ ] T7. 新增 `ocr/router.go`，定义 document type 到 provider 的路由规则。
- [ ] T8. 新增 `ocr/service.go`，实现 CreateJob / ExecuteJob / GetJobResult。
- [ ] T9. 在媒体层补充 `ReadMediaAsset` 内部能力。
- [ ] T10. 为 `ReadMediaAsset` 补 local/OSS 测试。

### 7.2 商户 food permit 迁移小任务

- [ ] T11. 识别商户 food permit 现有请求结构与返回结构。
- [ ] T12. 将 handler 改为只创建 `ocr_job`。
- [ ] T13. 调整 worker 入参为 `ocr_job_id`。
- [ ] T14. worker 中加载 job、读取 media、执行 provider。
- [ ] T15. 输出 normalized result 并持久化。
- [ ] T16. 将 normalized result 投影回 merchant application。
- [ ] T17. 补充 handler 测试。
- [ ] T18. 补充 worker 测试。

### 7.3 营业执照与身份证迁移小任务

- [ ] T19. 抽离营业执照专用解析结构。
- [ ] T20. 抽离身份证正反面专用解析结构。
- [ ] T21. 迁移商户营业执照 OCR。
- [ ] T22. 迁移商户身份证 OCR。
- [ ] T23. 迁移运营商营业执照 OCR。
- [ ] T24. 迁移运营商身份证 OCR。
- [ ] T25. 迁移骑手身份证 OCR。
- [ ] T26. 清理各 handler 内直接调用 provider SDK 的旧代码。

### 7.4 扩展与收尾小任务

- [ ] T27. 迁移骑手健康证 OCR。
- [ ] T28. 新增统一 OCR 接口族：create / get / result / retry / batch-query。
- [ ] T29. 更新 Swagger 注释与生成文档。
- [ ] T30. 增加 OCR 失败重试策略。
- [ ] T31. 增加 OCR 告警字段与监控点。
- [ ] T32. 清理 OCR 主链路中的 `mediaAssetLocalPath` 依赖。
- [ ] T33. 编写 OCR 运维 Runbook。
- [ ] T33G. 删除 merchant/operator/rider 旧 OCR 对外接口与对应路由。
- [ ] T33H. 删除旧 multipart OCR 请求结构、旧 worker payload 与旧路径读取代码。

### 7.4A 阿里云 provider 接入小任务

- [ ] T33A. 梳理阿里云 OCR 在本项目涉及的能力映射：身份证、企业资质、通用文字。
- [ ] T33B. 在配置层增加阿里云 OCR 开关、endpoint、region、凭证配置。
- [ ] T33C. 启动阶段增加阿里云 OCR 配置校验。
- [ ] T33D. 优先按 RAM 最小权限方案设计接入，不使用主账号长钥。
- [ ] T33E. 封装阿里云 OCR client，不允许业务 handler 直接调用 SDK。
- [ ] T33F. 补充凭证缺失、签名失败、权限不足、限流等错误映射测试。
- [ ] T33I. 固定 private 证件只走服务端字节流上传，不允许 URL 模式上传。

### 7.5 质量与稳定性小任务

- [ ] T34. 为 `ocr_jobs` 增加唯一幂等键与相关索引。
- [ ] T35. 为 `ocr_jobs` 增加 `attempt_count`、`max_attempts`、`next_retry_at`。
- [ ] T36. 为 `ocr_jobs` 增加 `leased_at`、`lease_owner` 或等价抢占字段。
- [ ] T37. 固定状态机流转规则并写成测试。
- [ ] T38. 实现可重试错误分类。
- [ ] T39. 实现指数退避或固定间隔重试策略。
- [ ] T40. 实现死信任务查询或人工处理入口。
- [ ] T41. 为 OCR job 创建与完成写审计日志。
- [ ] T42. 为身份证类 raw result 增加脱敏与留存期策略。
- [ ] T43. 增加 Prometheus 指标或等价 metrics。
- [ ] T44. 增加任务堆积、失败率、连续失败告警。
- [ ] T45. 增加并发重复消费测试。
- [ ] T46. 增加失败注入测试。
- [ ] T47. 建立 OCR 样本集与基线评估脚本。
- [ ] T47A. 固定核心指标口径：成功率、P95/P99 耗时、失败码分布、堆积量、重试量。

### 7.6 发布切换小任务

- [ ] T48. 编写统一 OCR 接口族的一次性切换清单。
- [ ] T49. 编写测试环境端到端联调清单。
- [ ] T50. 编写生产发布步骤。
- [ ] T51. 编写回滚步骤与不允许回滚的数据边界。
- [ ] T52. 编写上线后验收 checklist。
- [ ] T53. 确认代码库中不再存在 merchant/operator/rider 旧 OCR 壳层接口。
- [ ] T54. 确认代码库中不再存在旧 multipart OCR 主入口与旧 worker payload。

## 8. 验收标准

完成阶段 G 后，至少满足：

- 所有核心证件 OCR 都以 `media_asset_id` 为主入口。
- OCR 主链路不依赖本地 uploads 路径。
- 业务 handler 不再直接耦合任何 provider SDK 细节。
- private 证件不通过公开 URL 提供给 OCR。
- 所有 OCR 执行状态都可追踪、可审计、可重试。
- 业务字段回写与原始 OCR 结果都有持久化记录。

完成阶段 H 后，进一步满足：

- provider 切换不需要改业务 handler。
- 阿里云与微信可以按文档类型独立路由。

完成阶段 I 与阶段 J 后，才可认为 OCR 链路达到“10 分级”交付标准：

- 对重复提交、并发消费、失败重试和人工介入都有确定行为。
- 对敏感证件的原始结果存储、脱敏和留存期有明确治理。
- 对准确率、失败率、耗时和积压有量化指标与告警。
- 对发布、切换、回滚和验收有成套 runbook。

## 9. 风险与注意事项

- 旧 handler 中同步 OCR 逻辑较多，迁移时容易出现返回语义变化，需要同步校准前端调用预期。
- 身份证虽然不公开展示，但 OCR 内部读取能力必须确保不经由 public URL 兜底。
- food permit 识别高度依赖文本解析规则，迁移 provider 抽象时不要把规则散落回 handler。
- 阿里云 provider 必须前置接入并成为主链路，不能继续拖到后续阶段，否则文档与实现顺序会再次错位。
- 阿里云 OCR 接入如果直接使用主账号长钥，会把 OCR 从工程问题变成安全问题，这是不能接受的。
- 既然当前服务尚未上线，就不应该再人为保留兼容壳层；保留壳层只会增加实现噪音和未来误用概率。
- 如果不补状态机、重试与观测，链路即使重构完成，也只能算“结构更好”，还不能算生产级 10 分方案。
- 如果不补样本集与准确率基线，后续 provider 切换时无法证明效果变好还是变差。
- 如果不补隐私留存与脱敏策略，身份证类 OCR 原始结果会形成新的合规风险。

## 10. 当前执行状态

- [x] 已完成图审与证件可见性规则收口。
- [x] 已完成二维码审核豁免与 `approved` 标记。
- [x] 已完成 public URL 门禁收紧。
- [x] 已完成身份证 owner-only 私有访问控制。
- [x] 已完成 OCR 当前实现盘点。
- [x] 已完成阿里云主 provider、微信第二 provider 的目标定位收口。
- [x] 已形成统一 OCR 最终态方案。
- [x] 已补充 10 分级 OCR 所需的稳定性、观测、发布与隐私治理任务。
- [ ] 阶段 B 尚未开始编码。
- [ ] 当前下一步应从 T0、T1-T10、T33A-T33F 开始落地基础设施与阿里云主 provider。