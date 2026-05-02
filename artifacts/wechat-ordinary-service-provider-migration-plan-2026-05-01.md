# 普通服务商支付迁移实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将商户主业务的普通支付、合单支付、退款、分账从平台收付通迁移到微信普通服务商特约商户模式，同时保持现有直连支付链路不变。

**Architecture:** 新增 `locallife/wechat/ordinaryserviceprovider` 独立 bounded module。该模块自拥有 client、contracts、errorcodes、notification 解密、validation、error mapping、mock 和测试；对外 Go 类型使用 `OrdinaryServiceProvider` 前缀，数据通道使用 `payment_channel='ordinary_service_provider'`。业务层只通过该模块暴露的最小能力接口创建新商户订单、退款和分账，不直接依赖微信官方 DTO 或模块内部 helper。现有直连支付与平台收付通实现不作为该模块的设计输入；平台收付通仅冷备保留，不承载新商户交易。

**Tech Stack:** Go, sqlc, PostgreSQL migrations, gomock, 微信支付 API v3, 官方 `wechatpay-go` SDK 作为模块私有 adapter, LocalLife backend `locallife/`.

**Risk Class:** G3。该计划触及支付、退款、分账、回调、外部接口契约、资金状态、商户准入和收付通冷备边界。

---

## 1. 确认后的业务边界

- 普通服务商迁移范围：商户进件、商户开户意愿确认、普通小程序支付、合单支付、退款、分账、分账回退、剩余资金解冻、分账接收方、账单、结算账户查询与修改。
- 普通服务商运营必备能力：商户被管控能力及原因查询、商户平台处置通知；不活跃商户身份核实仅在管控查询返回 `VERIFY_INACTIVE_MERCHANT_IDENTITY` 解脱路径时使用。
- 模块边界：普通服务商特约商户按独立模块开发，模块根包为 `locallife/wechat/ordinaryserviceprovider`；模块内部拥有 contracts、errorcodes、client、notification、validation、transport adapter、mock 和测试。
- 命名边界：新模块必须显著区别于直连支付和平台收付通，对外类型使用 `OrdinaryServiceProvider`，数据通道使用 `ordinary_service_provider`，不得使用泛化的 `partner_service`、`partner`、`merchant_payment` 或复用 `ecommerce`。
- 依赖边界：普通服务商模块不依赖直连支付或平台收付通实现作为模板；实现依据只来自官方普通服务商文档、`.github/standards/domains/wechat-payment/README.md` 和后端工程标准。
- 系统中不存在需要继续兼容的平台收付通老商户；本计划不设计 ecommerce 到普通服务商的双轨兼容分支，不做历史收付通订单清算主线。
- 不迁移直连支付：骑手保证金、骑手赎回、商户向平台追偿、骑手向平台追偿继续走现有 direct client。
- 不保留平台收付通资金管理：小程序内二级商户余额查询、平台账户余额查询、预约提现、注销后提现全部下线。
- 不依赖平台收付通垫付退款：平台赔付与责任方索赔继续由业务系统处理，不使用微信收付通垫付回补接口。
- 不调整分账比例和解冻时机：沿用现有业务规则，只替换微信接口与 contract 真值来源。
- 平台收付通代码可以冷备保留，以便未来微信支付政策变化时重新评估；冷备代码不得被新支付、退款、分账、进件或商户管理链路调用。

## 2. 官方文档真值入口

普通服务商能力链接已落入 `.github/standards/domains/wechat-payment/README.md` 的 `4.10 普通服务商迁移能力组`。实施时必须以该 README 中对应能力组的官方文档作为请求、响应、枚举、错误码、回调结构和前端文案来源。

## 3. SDK 采用策略

- 默认不 fork `github.com/wechatpay-apiv3/wechatpay-go`。使用官方 SDK 能降低签名、验签、证书、公钥、回调解密、敏感字段加解密和安全修复维护成本。
- `wechatpay-go` 只能作为 `locallife/wechat/ordinaryserviceprovider` 的模块私有 adapter 或中性基础设施依赖；SDK DTO、`core.APIResult`、`core.APIError` 和 `services/*` 类型不得出现在 `api/`、`logic/`、`worker/`、`db/` 的公开接口中。
- SDK 已覆盖能力优先封装后使用：`partnerpayments/app|jsapi|native|h5`、`refunddomestic`、`profitsharing`、`core.Client`、`core/notify`、证书/公钥/敏感字段能力。
- SDK 未覆盖能力由普通服务商模块内补齐：`/v3/applyment4sub/*`、`/v3/combine-transactions/*`、`/v3/apply4subject/*`、`/v3/mch-operation-manage/*`、商户平台处置通知配置、不活跃商户身份核实。实现方式是 `core.Client.Request` 或等价中性 signed transport + 本模块 contracts/errorcodes。
- 不以 fork SDK 作为常规路径。只有当缺口接口数量和复用半径扩大到需要长期维护通用 SDK，并准备 upstream 给官方仓库或正式维护内部 SDK 时，才重新评估 fork。
- `wechatpay-skills` 仅作为官方知识、示例、排障和质量检查参考，不作为代码依赖或 contract 真值来源；contract 真值仍以微信支付官方接口文档和本模块固化结构为准。

## 4. 文件结构规划

### 4.1 新增普通服务商独立模块

- Create package root: `locallife/wechat/ordinaryserviceprovider/`，包名 `ordinaryserviceprovider`。
- Create: `locallife/wechat/ordinaryserviceprovider/client.go`，实现普通服务商 API v3 调用与 SDK-backed adapters。
- Create: `locallife/wechat/ordinaryserviceprovider/interface.go`，定义 `OrdinaryServiceProviderClientInterface` 和对业务层开放的能力接口。
- Create: `locallife/wechat/ordinaryserviceprovider/config.go`，定义模块私有配置校验和构造参数，不读取全局状态。
- Create: `locallife/wechat/ordinaryserviceprovider/transport.go`，封装 `wechatpay-go/core.Client`、`core.Client.Request`、签名、证书、公钥、敏感字段加解密和响应解密。
- Create: `locallife/wechat/ordinaryserviceprovider/notification.go`，封装 `wechatpay-go/core/notify` 并统一验签、解密和解析普通服务商支付、退款、分账、商户处置通知。
- Create: `locallife/wechat/ordinaryserviceprovider/validation.go`，集中校验金额、币种、appid/mchid/sub_mchid、openid/sub_openid、notify_url HTTPS、receiver、敏感字段。
- Create package: `locallife/wechat/ordinaryserviceprovider/contracts/`，包内按能力拆分 `applyment.go`、`merchant_management.go`、`payment.go`、`combine.go`、`refund.go`、`profit_sharing.go`、`settlement.go`、`notification.go`。
- Create package: `locallife/wechat/ordinaryserviceprovider/errorcodes/`，包内按能力拆分 `applyment.go`、`merchant_management.go`、`payment.go`、`refund.go`、`profit_sharing.go`。
- Create package: `locallife/wechat/ordinaryserviceprovider/mock/` after running `make mock`，生成 `MockOrdinaryServiceProviderClientInterface`。
- Do not add new ordinary-service-provider contracts or errorcodes to shared `locallife/wechat/contracts/` or `locallife/wechat/errorcodes/` packages.

### 4.2 数据与常量

- Create migration via `cd locallife && make new_migration name=add_ordinary_service_provider_payment_channel`, then fill the generated `.up.sql` and `.down.sql` files.
- Modify: `locallife/db/sqlc/constants.go` after `make sqlc`.
- Modify: `locallife/db/sqlc/payment_order_channel_boundary.go`.
- Modify SQL queries that create or select active merchant payment flows so they use `payment_channel = 'ordinary_service_provider'`. Do not widen active merchant queries to include ecommerce as a compatibility branch.

### 4.3 业务层与 worker

- Modify: `locallife/main.go` to build and inject the ordinary service provider module client.
- Modify: `locallife/Makefile` mockgen targets to include the new interface.
- Modify: `locallife/logic/payment_order_service.go` for ordinary single-payment order creation.
- Modify: `locallife/logic/combined_payment_service.go` and `locallife/logic/reservation_dishes.go` for combine payment creation.
- Modify: `locallife/logic/payment_order_query_wechat.go` and payment timeout workers for ordinary-service-provider-channel query/close behavior through the module interface.
- Modify refund callers: `locallife/logic/refund_service.go`, `locallife/logic/merchant_reject_refund.go`, `locallife/logic/reservation.go`, `locallife/logic/reservation_cancel.go`, `locallife/logic/replace_order.go`, `locallife/logic/payment_fact_service.go`.
- Modify profit-sharing callers: `locallife/logic/profit_sharing_receiver_sync_service.go`, `locallife/logic/profit_sharing_receiver_lifecycle_service.go`, `locallife/worker/task_process_payment.go`.
- Modify callback/fact application paths: `locallife/api/wechat.go`, `locallife/logic/payment_fact_application_service.go`, `locallife/worker/payment_fact_application_scheduler.go`.

## 5. 阶段执行与 review 闭环

每个阶段必须按同一闭环推进：实现一张或一组同阶段任务卡，运行该卡列出的验证命令，做后端 review，修复 review findings，再复审，直到没有未解决 findings。G3 路径如果无法 sandbox 或集成验证，必须在阶段交付中记录残余风险、owner 和下一次复验触发条件。

阶段交付必须说明：风险等级、改动文件、生成命令、测试命令、已验证层级、未验证路径、是否触发 SQL/mock/swagger regeneration、是否有回滚或观测缺口。

## 6. 阶段化任务卡

### Phase 0: Governance And SDK Decision

#### Card 0.1: 固化文档入口与 SDK 策略

**Boundary:** 只改文档，不改后端代码。

**Files:** `.github/standards/domains/wechat-payment/README.md`, `artifacts/wechat-ordinary-service-provider-migration-plan-2026-05-01.md`

- [x] 确认 README 写明普通服务商模块路径、命名、channel、SDK 采用策略和不 fork 策略。
- [x] 确认计划写明 `wechatpay-go` 只作为模块私有 adapter，`wechatpay-skills` 只作为参考。
- [x] 确认计划没有使用 `partner_service` / `PartnerService` 作为模块身份。

**Validation:** `rg "wechatpay-go|wechatpay-skills|ordinaryserviceprovider|ordinary_service_provider|不 fork|SDK DTO" .github/standards/domains/wechat-payment/README.md artifacts/wechat-ordinary-service-provider-migration-plan-2026-05-01.md`

**Review Gate:** 文档 review 只看边界是否清晰、是否会让实现者误 fork SDK、误复用 ecommerce/direct 或误把 SDK DTO 暴露给业务层。

#### Card 0.2: 建立阶段 review 模板

**Boundary:** 只补充执行记录模板或复用现有 review closeout 文档，不进入实现。

**Files:** `artifacts/wechat-ordinary-service-provider-migration-plan-2026-05-01.md`; optional existing review checklist docs under `.github/standards/backend/`

- [x] 每个阶段交付记录包含风险等级、命令输出、验证层级、未验证路径、review findings、修复记录和复审结论。
- [x] 若实现阶段发现新的重复缺陷模式，将 finding 回写到合适的 standards、prompt、workflow 或 repo memory。

**Validation:** `rg "阶段执行与 review 闭环|review findings|复审|残余风险" artifacts/wechat-ordinary-service-provider-migration-plan-2026-05-01.md`

**Review Gate:** review 模板必须足够支持“实现 -> review -> 修复 -> 复审 -> 无 findings”的循环。

### Phase 1: Data Channel Foundation

#### Card 1.1: 新增 payment channel migration

**Boundary:** 只增加 `ordinary_service_provider` channel 的数据库约束，不改变业务调用路径。

**Files:** generated migration under `locallife/db/migration/`

- [x] Run `cd locallife && make new_migration name=add_ordinary_service_provider_payment_channel`.
- [x] 在 `.up.sql` 中将 `ordinary_service_provider` 加入 payment channel check constraint。
- [x] 在 `.down.sql` 中恢复到迁移前的 check constraint。
- [x] 不删除 `direct`，不删除冷备 `ecommerce`。

**Validation:** 在本地测试数据库可用时运行 `cd locallife && make migrateup`。

**Review Gate:** SQL review 必须确认 migration 可逆、没有改变历史数据语义、没有把 `ecommerce` 当 active 兼容分支。

#### Card 1.2: 生成 sqlc 常量并补 channel helper

**Boundary:** 只处理生成常量和 channel 判定 helper，不切换业务流。

**Files:** `locallife/db/sqlc/constants.go`, `locallife/db/sqlc/payment_order_channel_boundary.go`, channel helper tests

- [x] Run `cd locallife && make sqlc`.
- [x] 确认生成 `PaymentChannelOrdinaryServiceProvider = "ordinary_service_provider"`。
- [x] 增加 `PaymentOrderUsesOrdinaryServiceProviderChannel`。
- [x] 保留 direct helper 和 ecommerce cold-reserve helper，禁止新增把 ecommerce 与 ordinary 合并的 merchant helper。

**Validation:** `cd locallife && go test ./db/sqlc -run 'PaymentChannel|OrdinaryServiceProviderChannel'`

**Review Gate:** review 必须确认 helper 命名显式、无隐式兼容、无魔法字符串流入 logic/worker。

#### Card 1.3: 审计 active merchant channel 查询

**Boundary:** 只做查询审计和最小必要替换，不改业务逻辑。

**Files:** `locallife/db/query/**`, generated sqlc files after `make sqlc`, targeted tests

- [x] 找出 active merchant payment 查询中硬编码 `payment_channel = 'ecommerce'` 的位置。
- [x] 只把“新商户主业务”语义替换为 `ordinary_service_provider`。
- [x] 不把 active 查询扩成 `IN ('ecommerce','ordinary_service_provider')`。
- [x] 如果某个 `ecommerce` 是明确冷备或历史查询，保留并在测试名中说明边界。

**Validation:** `cd locallife && rg "payment_channel.*ecommerce|ordinary_service_provider" db/query db/sqlc logic worker`

**Review Gate:** review 必须逐条确认查询语义，避免误伤 direct、历史审计或冷备路径。

#### Phase 1 Closeout: 2026-05-01

**Risk Class:** G3 payment foundation change. 本阶段只改变 payment channel 数据约束、持久化常量/helper 和 missing profit-sharing recovery 查询边界，不接入真实微信支付、退款、分账或回调调用。

**Changed Files:** `locallife/db/migration/000223_add_ordinary_service_provider_payment_channel.up.sql`, `locallife/db/migration/000223_add_ordinary_service_provider_payment_channel.down.sql`, `locallife/db/query/profit_sharing_order.sql`, `locallife/db/sqlc/constants.go`, `locallife/db/sqlc/payment_order_channel_boundary.go`, `locallife/db/sqlc/payment_order_channel_boundary_test.go`, generated `locallife/db/sqlc/profit_sharing_order.sql.go`, `locallife/db/sqlc/profit_sharing_order_recovery_test.go`.

**Red/Green Evidence:**

- RED: `go test ./db/sqlc -run TestPaymentOrderUsesOrdinaryServiceProviderChannel` failed before implementation because `PaymentChannelOrdinaryServiceProvider` and `PaymentOrderUsesOrdinaryServiceProviderChannel` were undefined.
- RED: `go test ./db/sqlc -run TestListCompletedOrdersMissingProfitSharing_IncludesNonTakeout` failed after switching the fixture to `PaymentChannelOrdinaryServiceProvider`, proving the old recovery query still filtered `ecommerce`.
- GREEN: `go test ./db/sqlc -run 'TestPaymentOrderUsesOrdinaryServiceProviderChannel|TestListCompletedOrdersMissingProfitSharing'` passed after adding the channel, helper, migration, and query change.

**Regeneration:** Ran `make sqlc`; generated `profit_sharing_order.sql.go` changed. The Makefile also refreshed mocks, but no mock file diff remained.

**Fresh Validation:**

- `go test ./db/sqlc -run 'TestPaymentOrderUsesOrdinaryServiceProviderChannel|TestListCompletedOrdersMissingProfitSharing'` passed.
- `go test ./db/sqlc` passed.
- `make check-generated` passed with `generated artifacts are in sync`.
- `bash ../.github/scripts/test_backend_sql_guard.sh` passed from the backend working directory.
- `git diff --check` passed.

**Query Audit:** `locallife/db/query/profit_sharing_order.sql` was the only active merchant recovery query changed to `payment_channel = 'ordinary_service_provider'`. `locallife/db/query/refund_order.sql` keeps `ListEcommerceRefundOrdersForReconciliation` as explicit收付通 historical/cold-reserve reconciliation, not active new merchant flow. No query was widened to `IN ('ecommerce','ordinary_service_provider')`.

**Not Yet Verified In This Phase:** No external WeChat API, callback, retry, refund, or profit-sharing endpoint behavior is verified here; those remain Phase 2-7 scope. `make migrateup` against a non-test local database was not run; db/sqlc tests exercise migrations against the test database.

### Phase 2: Ordinary Service Provider Module Skeleton

#### Card 2.1: 创建模块骨架与配置

**Boundary:** 只创建包、配置和构造校验，不发起真实微信请求。

**Files:** `locallife/wechat/ordinaryserviceprovider/config.go`, `locallife/wechat/ordinaryserviceprovider/client.go`, `locallife/wechat/ordinaryserviceprovider/config_test.go`

- [x] 创建 `ordinaryserviceprovider` 包。
- [x] 定义 `Config`，包含服务商 appid/mchid、证书序列号、私钥/APIv3 key、公钥或证书策略、base URL、notify URL 配置。
- [x] 校验必填配置和 HTTPS notify URL。
- [x] 不读取全局 config，不导入 `logic`、`api`、`db`、`worker`。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider -run 'Config|NewOrdinaryServiceProvider'`

**Review Gate:** review 必须确认模块无 runtime global、无 direct/ecommerce import、构造失败可观测。

#### Card 2.2: 封装官方 SDK transport adapter

**Boundary:** 只封装 `wechatpay-go` 基础设施，不定义业务 endpoint。

**Files:** `locallife/wechat/ordinaryserviceprovider/transport.go`, `locallife/wechat/ordinaryserviceprovider/transport_test.go`, `locallife/go.mod`, `locallife/go.sum`

- [x] 引入官方 `github.com/wechatpay-apiv3/wechatpay-go`。
- [x] 用 `core.Client` 和 option 初始化签名、验签、证书/公钥、敏感字段能力。
- [x] 提供模块私有 `requestJSON` / `requestNoBody` / `unmarshalAPIError` helper。
- [x] 将 `core.APIResult` 和 `core.APIError` 转换为模块内部结果与错误分类。
- [x] 不把 SDK 类型放进 public interface。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider -run 'Transport|APIError|SDK'`

**Review Gate:** review 必须确认 SDK 只在模块内部，错误映射不会丢失 request id、status code、provider code。

#### Card 2.3: 封装普通服务商 notification adapter

**Boundary:** 只做验签解密和 envelope 解析，不落库、不执行业务状态转换。

**Files:** `locallife/wechat/ordinaryserviceprovider/notification.go`, `locallife/wechat/ordinaryserviceprovider/notification_test.go`, `locallife/wechat/ordinaryserviceprovider/contracts/notification.go`

- [x] 用 `core/notify` 封装普通服务商回调验签、解密、body rewind 行为。
- [x] 输出模块内 `NotificationEnvelope`，包含通知 ID、event type、resource type、summary、plaintext、raw headers。
- [x] 支持解析未知通知到 `map[string]any`，但业务已支持类型必须解析到强类型 contracts。
- [x] 不在 adapter 中做 fact application 或数据库写入。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider -run 'Notification|Notify|Decrypt'`

**Review Gate:** review 必须确认重复读取 body 安全、验签失败不吞错、plaintext 不进入普通日志。

#### Card 2.4: 定义 public client interface 并生成 mock

**Boundary:** 只定义接口和 mock，不接入业务层。

**Files:** `locallife/wechat/ordinaryserviceprovider/interface.go`, `locallife/Makefile`, generated `locallife/wechat/ordinaryserviceprovider/mock/client.go`

- [x] 定义 `OrdinaryServiceProviderClientInterface`，方法按 applyment、merchant management、payment、combine、refund、profit sharing、notification 分组。
- [x] 方法参数和返回值只使用本模块 contracts/errorcodes 或 Go 标准类型。
- [x] 增加 mockgen 目标并运行 `make mock`。
- [x] 不修改 direct/ecommerce interfaces，除非编译需要 imports 清理。

**Validation:** `cd locallife && make mock && go test ./wechat/ordinaryserviceprovider/... -run 'Interface|Mock|OrdinaryServiceProvider'`

**Review Gate:** review 必须确认 interface 是 capability-shaped，不是 SDK-shaped，也不暴露 `core.Client`。

#### Phase 2 Closeout: 2026-05-01

**Risk Class:** G3 integration foundation. 本阶段建立普通服务商独立模块、配置校验、SDK 私有 transport、错误封装、通知 envelope、public interface 和 gomock，不接入业务层，不发起真实微信请求。

**Changed Files:** `locallife/go.mod`, `locallife/go.sum`, `locallife/Makefile`, `locallife/wechat/ordinaryserviceprovider/client.go`, `config.go`, `transport.go`, `errors.go`, `notification.go`, `interface.go`, tests under `locallife/wechat/ordinaryserviceprovider/`, generated `locallife/wechat/ordinaryserviceprovider/mock/client.go`.

**Red/Green Evidence:**

- RED: focused package tests initially failed because ordinary-service-provider config, transport, error mapping, notification envelope and interface symbols were undefined.
- GREEN: `go test ./wechat/ordinaryserviceprovider/...` passed after adding the module skeleton and generated mock.
- GREEN: `make mock` passed and generated `MockOrdinaryServiceProviderClientInterface` without exposing SDK DTOs through the public interface.

**Regeneration:** Ran `make mock`; updated `locallife/Makefile` so future `make mock` and `make sqlc` regenerate the ordinary service provider mock.

**Boundary Evidence:** `OrdinaryServiceProviderClientInterface` methods use module-local contracts plus Go standard types only. `core.Client`, `core.APIResult`, `core.APIError`, SDK services and notify handler remain inside `ordinaryserviceprovider` internals.

**Review Follow-up:**

- Finding fixed: `GenerateJSAPIPayParams` still returned shared `wechat/contracts.JSAPIPayParams`, which drifted from the module-local public-interface contract and let an old shared payment DTO cross the ordinary-service-provider boundary. Added `TestOrdinaryServiceProviderClientInterfaceUsesOnlyModuleContracts`, moved JSAPI pay params into `ordinaryserviceprovider/contracts`, converted to the existing business-facing `wechat.JSAPIPayParams` only in `logic/`, and regenerated `wechat/ordinaryserviceprovider/mock/client.go`.
- RED: `go test ./wechat/ordinaryserviceprovider -run TestOrdinaryServiceProviderClientInterfaceUsesOnlyModuleContracts -count=1 -v` failed on `GenerateJSAPIPayParams` before the fix because it exposed `github.com/merrydance/locallife/wechat/contracts`.
- GREEN: `go test ./api ./logic ./wechat/ordinaryserviceprovider/... -count=1` passed after the interface boundary fix and mock regeneration.

**Not Yet Verified In This Phase:** Real WeChat signing, sandbox calls, callback delivery, business fact application, timeout/retry, payment/refund/profit-sharing state transitions and typed notification resource mapping are deferred to later phases.

### Phase 3: Contracts And Error Codes

#### Card 3.1: 商户进件与结算账户 contracts

**Boundary:** 只定义 contracts 和 validation，不实现 client endpoint。

**Files:** `locallife/wechat/ordinaryserviceprovider/contracts/applyment.go`, `locallife/wechat/ordinaryserviceprovider/contracts/settlement.go`, tests

- [x] 定义 `/v3/applyment4sub/applyment/` 提交、按申请单 ID 查询、按业务申请编号查询结构。
- [x] 定义结算账户查询、修改、修改状态查询结构。
- [x] 校验主体类型仅支持个体工商户和企业。
- [x] 定义图片/视频 media id 字段，不在 contract 中处理文件读取。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider/contracts -run 'Applyment|Settlement'`

**Review Gate:** review 必须对照官方文档检查条件必填、敏感字段、状态枚举和本地业务范围。

#### Card 3.2: 开户意愿确认 contracts

**Boundary:** 只定义开户意愿确认 contracts 和状态语义。

**Files:** `locallife/wechat/ordinaryserviceprovider/contracts/merchant_management.go`, tests

- [x] 定义提交、撤销、查询审核结果、获取授权状态结构。
- [x] 固化 `AUTHORIZE_STATE_UNAUTHORIZED` 和 `AUTHORIZE_STATE_AUTHORIZED`。
- [x] 明确商户激活必须同时满足普通服务商进件完成和授权状态已完成。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider/contracts -run 'AccountWillingness|AuthorizeState'`

**Review Gate:** review 必须确认授权状态不会被 applyment 状态替代或弱化。

#### Card 3.3: 商户管控、处置通知与不活跃核实 contracts

**Boundary:** 只定义商户运营恢复 contracts，不接入 operator API。

**Files:** `locallife/wechat/ordinaryserviceprovider/contracts/merchant_management.go`, `locallife/wechat/ordinaryserviceprovider/contracts/notification.go`, tests

- [x] 定义管控查询响应：limited functions、reason types、recover ways、help URL、immediate/delayed control dates。
- [x] 定义商户平台处置通知配置 create/query/update/delete 结构。
- [x] 定义处置通知 decrypted resource，包含 `record_id`、`sub_mchid`、`event_type`、`risk_type`、`punish_plan`、`punish_time`、`risk_description`。
- [x] 定义不活跃商户身份核实 create/query 结构，并约束仅用于 `VERIFY_INACTIVE_MERCHANT_IDENTITY` 恢复路径。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider/contracts -run 'MerchantLimit|Violation|InactiveMerchant'`

**Review Gate:** review 必须确认该能力是运营恢复路径，不阻塞首批支付切换主线。

#### Card 3.4: 普通小程序支付 contracts

**Boundary:** 只定义单笔支付 contracts，不实现 client。

**Files:** `locallife/wechat/ordinaryserviceprovider/contracts/payment.go`, tests

- [x] 定义 `/v3/pay/partner/transactions/jsapi` 下单、查单、关单、支付通知结构。
- [x] 定义调起支付参数结构，由模块内部生成并返回本地 contract。
- [x] 校验 `sp_appid`、`sp_mchid`、`sub_mchid`、`sub_openid`、amount、notify URL。
- [x] 保留 `profit_sharing` 语义，不加入平台收付通补差字段。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider/contracts -run 'Payment|JSAPI|Prepay'`

**Review Gate:** review 必须确认 ordinary payment contract 与 SDK `partnerpayments/jsapi` DTO 不外泄。

#### Card 3.5: 合单支付 contracts

**Boundary:** 只定义合单 contracts；这是 SDK 缺口能力，必须按官方文档自有建模。

**Files:** `locallife/wechat/ordinaryserviceprovider/contracts/combine.go`, tests

- [x] 定义 `/v3/combine-transactions/jsapi` 下单、查单、关单、通知结构。
- [x] 定义 combine appid/mchid、sub orders、amount allocation、notify URL 校验。
- [x] 明确不得包含 `subsidy_amount` 或平台收付通补差字段。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider/contracts -run 'Combine|SubOrder|CombinePayment'`

**Review Gate:** review 必须确认自有合单 contract 能覆盖现有业务合单语义且无 ecommerce 字段漂移。

#### Card 3.6: 退款 contracts

**Boundary:** 只定义退款 contracts 和通知结构。

**Files:** `locallife/wechat/ordinaryserviceprovider/contracts/refund.go`, tests

- [x] 定义 `/v3/refund/domestic/refunds` 创建退款、按商户退款单号查询、按微信退款单号查询结构。
- [x] 定义退款通知 decrypted resource。
- [x] 校验金额、币种、订单号/交易号、refund reason、notify URL。
- [x] 禁止 `REFUND_SOURCE_PARTNER_ADVANCE` 等收付通垫付语义进入普通服务商 contract。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider/contracts -run 'Refund|OutRefundNo'`

**Review Gate:** review 必须确认 `NOT_ENOUGH` 后续由 errorcodes/business flow 处理，而不是 contract 层伪装成垫付能力。

#### Card 3.7: 分账 contracts

**Boundary:** 只定义分账 contracts，不接入 worker。

**Files:** `locallife/wechat/ordinaryserviceprovider/contracts/profit_sharing.go`, tests

- [x] 定义 receiver add/delete、create order、query order、return、query return、unfreeze、remaining amount、max ratio、split bill 结构。
- [x] 定义分账动账通知 resource。
- [x] 校验 receiver type、receiver account/name、amount、description、out order no、transaction id。
- [x] 保持骑手、运营商、平台分账比例由业务层控制，不写入 SDK adapter。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider/contracts -run 'ProfitSharing|Receiver|Unfreeze|RemainingAmount'`

**Review Gate:** review 必须确认 contracts 只表达微信协议，不编码 LocalLife 分账比例。

#### Card 3.8: 普通服务商 errorcodes 分类

**Boundary:** 只定义错误码和分类，不接入业务补偿。

**Files:** `locallife/wechat/ordinaryserviceprovider/errorcodes/*.go`, tests

- [x] 按 applyment、merchant management、payment/combine、refund、profit sharing 拆分错误码。
- [x] 区分 retryable provider/system failure、business conflict、merchant control、auth/config、validation failure。
- [x] 保留 `NOT_ENOUGH`、`NO_AUTH`、`SIGN_ERROR`、`SYSTEM_ERROR`、`FREQUENCY_LIMITED` 等官方差异。
- [x] 不 import direct/ecommerce errorcodes。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider/errorcodes -run 'Applyment|Merchant|Payment|Refund|ProfitSharing|Classify'`

**Review Gate:** review 必须抽样对照官方错误码文档，并确认业务层能基于分类做恢复或用户提示。

#### Phase 3 Closeout: 2026-05-01

**Risk Class:** G3 protocol truth foundation. 本阶段把普通服务商主链路的 contracts、validation 和 errorcode 分类固化在独立模块中，防止普通服务商迁移时复用平台收付通 schema 或让 SDK DTO 穿透业务层。

**Changed Files:** `locallife/wechat/ordinaryserviceprovider/contracts/types.go`, `contracts/applyment_test.go`, `contracts/payment_test.go`, `contracts/refund_profitsharing_test.go`, `errorcodes/errorcodes.go`, `errorcodes/errorcodes_test.go`, root `errors.go`.

**Red/Green Evidence:**

- RED: `go test ./wechat/ordinaryserviceprovider/contracts ./wechat/ordinaryserviceprovider/errorcodes` failed while contracts 仍是 placeholder，缺少 request fields、`Validate` 方法和 `errorcodes.Classify`。
- GREEN: `go test ./wechat/ordinaryserviceprovider/contracts ./wechat/ordinaryserviceprovider/errorcodes` passed after implementing local contracts, validation and errorcode metadata.
- GREEN: `go test ./wechat/ordinaryserviceprovider/...` passed after root `errors.go` delegated provider code classification to `ordinaryserviceprovider/errorcodes`.

**Semantic Decisions:**

- 退款 request 不暴露 `funds_account`，避免把平台收付通垫付退款或资金来源控制误带入普通服务商。
- `NOT_ENOUGH`, `NO_AUTH`, `SYSTEM_ERROR`, `REQUEST_BLOCKED` 等 provider codes 映射到稳定 category 和 frontend guidance。
- 结算账户、开户意愿、商户管控、不活跃身份核实字段按最近官方路径补齐关键响应字段，例如 `application_no`, `authorize_state`, `verification_id`。

**Not Yet Verified In This Phase:** 仍未做 sandbox 请求；通知 decrypted resource 的强类型 mapping、业务状态落库、商户激活判断和 operator recovery 展示留到业务接入阶段。

### Phase 4: Client Endpoint Implementation

#### Card 4.1: SDK 封装的单笔支付 client

**Boundary:** 只实现普通服务商单笔支付 endpoint，不接入 payment_order service。

**Files:** `locallife/wechat/ordinaryserviceprovider/client.go`, `payment_client_test.go`

- [x] 封装 `services/partnerpayments/jsapi` 下单、查单、关单。
- [x] 生成小程序调起支付参数并转换为本模块 contract。
- [x] 将 SDK error 转为普通服务商 errorcodes 分类。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider -run 'PaymentClient|Prepay|QueryOrder|CloseOrder'`

**Review Gate:** review 必须确认 SDK DTO 不越界，request id 和 provider code 被保留。

#### Card 4.2: 自建模型的合单支付 client

**Boundary:** 只实现合单 endpoint；因为 SDK 未覆盖，使用 `core.Client.Request`。

**Files:** `locallife/wechat/ordinaryserviceprovider/client.go`, `combine_client_test.go`

- [x] 实现 combine prepay、query、close。
- [x] 使用模块自有 `contracts.Combine*` request/response。
- [x] 覆盖 path、query、body serialization 和 non-2xx error mapping 测试。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider -run 'CombineClient|CombinePrepay|CombineQuery|CombineClose'`

**Review Gate:** review 必须确认 signed request 逻辑复用中性 transport，未复制 direct/ecommerce client。

#### Card 4.3: SDK 封装的退款 client

**Boundary:** 只实现退款创建、查询和退款通知解析。

**Files:** `locallife/wechat/ordinaryserviceprovider/client.go`, `refund_client_test.go`, `notification_test.go`

- [x] 封装 `services/refunddomestic` create/query。
- [x] 将 SDK response 转为本模块 refund contract。
- [x] 退款通知用 notification adapter 解密后映射为 refund notification contract。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider -run 'RefundClient|RefundNotification'`

**Review Gate:** review 必须确认无平台收付通垫付字段和垫付失败恢复假设。

#### Card 4.4: SDK 封装的分账 client

**Boundary:** 只实现分账 API，不接入 worker。

**Files:** `locallife/wechat/ordinaryserviceprovider/client.go`, `profit_sharing_client_test.go`

- [x] 封装 `services/profitsharing` receiver add/delete、create/query、return/query return、unfreeze、remaining amount、max ratio、split bill。
- [x] 将 SDK DTO 转为本模块 profit sharing contracts。
- [x] 覆盖 receiver name 加密、错误分类和幂等 out order no 传递。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider -run 'ProfitSharingClient|Receiver|Unfreeze|Return'`

**Review Gate:** review 必须确认业务分账比例没有进入 client，client 只负责微信协议。

#### Card 4.5: 自建模型的进件与结算账户 client

**Boundary:** 只实现进件和结算账户 endpoint，不接入 onboarding logic。

**Files:** `locallife/wechat/ordinaryserviceprovider/client.go`, `applyment_client_test.go`

- [x] 用 `core.Client.Request` 实现 submit applyment、query by applyment id、query by business code。
- [x] 实现 settlement query、modify、modify status query。
- [x] 覆盖敏感字段加密 serial header、media id、条件必填和 provider error mapping。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider -run 'ApplymentClient|SettlementClient'`

**Review Gate:** review 必须确认商户进件主链路不依赖 ecommerce applyment 结构。

#### Card 4.6: 自建模型的开户意愿 client

**Boundary:** 只实现开户意愿确认 endpoint。

**Files:** `locallife/wechat/ordinaryserviceprovider/client.go`, `account_willingness_client_test.go`

- [x] 实现 submit、cancel、query result、query authorization state。
- [x] 将授权状态转换为本模块 contract。
- [x] 保留 `AUTHORIZE_STATE_UNAUTHORIZED` 与 `AUTHORIZE_STATE_AUTHORIZED` 的业务可判定性。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider -run 'AccountWillingness|AuthorizeState'`

**Review Gate:** review 必须确认 authorization state 不会被 applyment state 替代。

#### Card 4.7: 自建模型的商户管控与处置通知 client

**Boundary:** 只实现商户运营恢复 endpoint 和通知配置 endpoint。

**Files:** `locallife/wechat/ordinaryserviceprovider/client.go`, `merchant_management_client_test.go`, `notification_test.go`

- [x] 实现 merchant limitation query。
- [x] 实现 violation callback create/query/update/delete。
- [x] 实现 inactive merchant identity verification create/query。
- [x] 解密并映射 violation notification resource。

**Validation:** `cd locallife && go test ./wechat/ordinaryserviceprovider -run 'MerchantLimit|Violation|InactiveMerchant'`

**Review Gate:** review 必须确认这些能力可作为 operator recovery path，且通知 idempotency key 清晰。

#### Phase 4 Closeout: 2026-05-01

**Risk Class:** G3 external endpoint boundary. 本阶段实现普通服务商 client endpoint 方法，但仍停留在模块级，不接入 payment_order、refund、profit-sharing worker 或 callback fact application。

**Changed Files:** `locallife/wechat/ordinaryserviceprovider/endpoints.go`, `transport.go`, `notification.go`, `interface_test.go`, `client_endpoints_test.go`, plus contract refinements in `contracts/types.go`.

**Red/Green Evidence:**

- RED: `go test ./wechat/ordinaryserviceprovider/...` failed because `Client` 缺少所有 `CreatePayment`、`QueryPayment`、`CreateRefund`、`CreateProfitSharingOrder`、applyment、settlement、merchant-management endpoint methods。
- RED: interface compile assertion then failed until `Client` matched `OrdinaryServiceProviderClientInterface`, including `CancelAccountWillingness`, `UnfreezeProfitSharing` and `ParseNotification` signatures.
- GREEN: `go test ./wechat/ordinaryserviceprovider/...` passed after endpoint implementation, combine endpoint coverage, notification parser and interface assertion.

**Implemented Endpoint Groups:**

- 特约商户进件：submit、query by applyment id、query by business code。
- 结算账户：query、modify、modification query。
- 开户意愿：submit、cancel、query audit result、query authorize state。
- 商户运营恢复：merchant limitation query、violation notification config create/query/update/delete、inactive merchant identity verification create/query。
- 普通小程序支付：create/query/close。
- 合单支付：create/query/close。
- 退款：create/query by out_refund_no。
- 分账：receiver add/delete、order create/query、return create/query、unfreeze、remaining amount。
- 通知：`ParseNotification` uses SDK RSA verifier and AES-GCM decryptor, returns module `NotificationEnvelope` and never logs plaintext by default.

**Fresh Validation:**

- `go test ./wechat/ordinaryserviceprovider/...` passed.
- VS Code diagnostics for `locallife/wechat/ordinaryserviceprovider` reported no errors after edits.

**Not Yet Verified In This Phase:** No sandbox request IDs, callback IDs, duplicate-delivery idempotency, retry classification against live provider, payment/fund state persistence, or business-layer cutover has been exercised. Those remain Phase 5-7 scope.

### Phase 5: Composition And Business Cutover

#### Card 5.1: 注入普通服务商 client

**Boundary:** 只改 composition root 和 constructors，不切换业务行为。

**Files:** `locallife/main.go`, `locallife/util/config.go`, relevant constructor tests

- [x] 增加 `buildOrdinaryServiceProviderClient(config util.Config)`。
- [x] 将普通服务商 client 注入 payment、combine、refund、profit sharing、callback、timeout、fact application services。
- [x] 保持 direct client builder 不变。
- [x] ecommerce client 仅冷备构造或不注入 active merchant constructors。

**Validation:** `cd locallife && go test ./... -run 'Constructor|ClientBuilder|OrdinaryServiceProvider'`

**Review Gate:** review 必须确认 composition root 清晰、无 package-level runtime global、无隐式 ecommerce fallback。

#### Card 5.2: 切换商户进件和开户意愿业务服务

**Boundary:** 只切换 merchant onboarding，不改支付下单。

**Files:** `locallife/logic/ordinary_service_provider_applyment_service.go`, related `api/` handlers and tests

- [x] 新建普通服务商 applyment service，不扩展 ecommerce applyment 作为真值。
- [x] 持久化普通服务商 applyment facts 和 authorization state。
- [x] 商户激活要求 applyment 完成且开户意愿授权完成。
- [x] 暴露结算账户查询/修改，不暴露余额/提现。

**Validation:** `cd locallife && go test ./logic ./api -run 'Applyment|AccountWillingness|AuthorizeState|Settlement'`

**Review Gate:** review 必须确认准入状态链完整，handler 不承载业务逻辑。

#### Card 5.3: 切换商户单笔支付创建

**Boundary:** 只切换 merchant main business single payment creation。

**Files:** `locallife/logic/payment_order_service.go`, `locallife/db/sqlc` transaction helper/tests

- [x] 新商户订单写入 `payment_channel='ordinary_service_provider'`。
- [x] 使用普通服务商 payment client 创建 JSAPI order。
- [x] 保留 `requires_profit_sharing`、金额、owner、幂等语义。
- [x] 不触碰 direct payment 主链路。

**Validation:** `cd locallife && go test ./logic ./db/sqlc -run 'CreatePaymentOrder|PaymentOrderService|OrdinaryServiceProvider|DirectPayment'`

**Review Gate:** review 必须确认资金写入、provider command record、幂等键和失败回滚边界。

#### Card 5.4: 切换合单支付创建

**Boundary:** 只切换 combine payment creation。

**Files:** `locallife/logic/combined_payment_service.go`, `locallife/logic/reservation_dishes.go`, related db transaction tests

- [x] 商户合单调用普通服务商 combine client。
- [x] 子订单和合单事实写入 ordinary channel。
- [x] 保留现有合单数量限制、金额分配、owner 和 profit sharing 语义。

**Validation:** `cd locallife && go test ./logic ./db/sqlc -run 'CombinedPayment|ReservationDishes|CreateCombinedPayment|OrdinaryServiceProvider'`

**Review Gate:** review 必须确认合单与单笔支付 channel 判定一致，失败时无半写事务。

#### Card 5.5: 切换查单、关单、超时恢复

**Boundary:** 只切换 recovery/query/close routing。

**Files:** `locallife/logic/payment_order_query_wechat.go`, `locallife/worker/task_payment_timeout.go`, `locallife/worker/task_combined_payment_timeout.go`, tests

- [x] ordinary channel 路由到 ordinary single/combine query/close。
- [x] direct channel 保持现状。
- [x] ecommerce channel 仅冷备，不作为正常 runtime compatibility。
- [x] unknown channel fail closed。

**Validation:** `cd locallife && go test ./logic ./worker -run 'PaymentOrderQuery|PaymentTimeout|CombinedPaymentTimeout|Channel'`

**Review Gate:** review 必须确认 timeout/retry 不会把普通服务商订单 fallback 到 direct 或 ecommerce。

#### Card 5.6: 切换退款业务

**Boundary:** 只切换 merchant main business refund paths。

**Files:** `locallife/logic/refund_service.go`, `merchant_reject_refund.go`, `reservation.go`, `reservation_cancel.go`, `replace_order.go`, `payment_fact_service.go`, tests

- [x] ordinary channel refund 调用 ordinary refund client。
- [x] 持久化 ordinary refund command/fact。
- [x] `NOT_ENOUGH` 分类为业务失败，保留平台赔付/责任方索赔业务流。
- [x] 不引入收付通垫付回补 fallback。

**Validation:** `cd locallife && go test ./logic -run 'Refund|MerchantReject|ReservationCancel|ReplaceOrder|PaymentFact|OrdinaryServiceProvider'`

**Review Gate:** review 必须确认退款状态机、补偿路径、重复退款幂等和用户可见失败语义。

#### Card 5.7: 切换分账与分账恢复

**Boundary:** 只切换 ordinary channel profit sharing paths。

**Files:** `locallife/logic/profit_sharing_receiver_sync_service.go`, `profit_sharing_receiver_lifecycle_service.go`, `locallife/worker/task_process_payment.go`, `locallife/db/query/profit_sharing_order.sql`, tests

- [x] receiver add/delete 使用 ordinary profit sharing client。
- [x] split order、return、unfreeze、remaining amount 使用 ordinary client。
- [x] recovery/query 按 payment order channel 路由，不把 ordinary failure fallback 到 ecommerce。
- [x] 保留骑手、运营商、平台分账业务规则。

**Validation:** `cd locallife && make sqlc && go test ./logic ./worker ./db/sqlc -run 'ProfitSharing|ReceiverLifecycle|ProcessPayment|OrdinaryServiceProvider'`

**Review Gate:** review 必须确认分账、回退、解冻、剩余金额查询的幂等与重试分类。

#### Card 5.8: 切换支付、退款、分账回调 fact application

**Boundary:** 只切换 ordinary service provider callback ingestion and fact application。

**Files:** `locallife/api/wechat.go`, `locallife/logic/payment_fact_application_service.go`, `locallife/worker/payment_fact_application_scheduler.go`, `locallife/db/query/external_payment_fact.sql`, tests

- [x] 支持普通服务商单笔支付、合单支付、退款、分账通知。
- [x] facts 使用 `provider='wechat'`、`channel='ordinary_service_provider'` 和 ordinary capability constants。
- [x] duplicate callback delivery idempotent。
- [x] 不改 unrelated callback routes。

**Validation:** `cd locallife && make sqlc && go test ./api ./logic ./worker ./db/sqlc -run 'Wechat|Callback|Notification|PaymentFactApplication|OrdinaryServiceProvider'`

**Review Gate:** review 必须确认回调验签解密、幂等 key、fact 状态推进和 retry scheduler 完整闭环。

#### Phase 5 Closeout: 2026-05-01

**Risk Class:** G3 money movement cutover. 本阶段把商户主业务 ordinary single payment、combine payment、refund、profit sharing、profit sharing return、unfreeze、remaining amount query、callback ingestion 和 fact application 接入普通服务商通道；direct payment 保持不变，ordinary failure 不 fallback 到 ecommerce。

**Implementation Evidence:**

- Single payment and combine payment create/query/close now route ordinary channel to `ordinaryserviceprovider` client and write ordinary command/fact channel.
- Refund create、worker initiate、query recovery、callback fact recording now support `ordinary_service_provider` with `partner_refund` capability and explicit provider error mapping for frontend guidance.
- Profit sharing worker create/query/finish/return/query-return now routes ordinary payment orders to ordinary profit sharing APIs and records channel-aware command/fact dedupe keys.
- Operator capability APIs now accept ordinary profit sharing payment orders for remaining amount query and receiver delete, resolving `sub_mchid` from the profit sharing order's merchant payment config.
- Ordinary webhook routes now include payment、combine、refund、profit-sharing notifications and record ordinary channel facts with retry-on-FAIL behavior for validation, ownership, query, and fact persistence errors.
- Profit sharing fact application validators now accept ordinary and ecommerce main-business profit sharing facts; direct channel remains excluded.
- Review finding fixed: `ProcessTaskPaymentFactApplication` originally injected only the platform ecommerce refund creator, so an ordinary `profit_sharing_return` success fact could mark the return successful but fail when continuing the ordinary refund. The worker now injects a channel-aware refund creator that preserves ecommerce cold-reserve behavior and calls the ordinary refund API for `payment_channel='ordinary_service_provider'`, with no ecommerce fallback.

**Validation Run:**

- `go test ./api -run '^$' -count=1`
- `go test ./logic -run '^$' -count=1`
- `go test ./worker -run '^$' -count=1`
- `go test ./wechat/ordinaryserviceprovider/... -count=1`
- `go test ./worker -run 'TestProcessTaskProfitSharing_|TestProcessTaskProfitSharingReturnResult_' -count=1`
- `go test ./api -run 'Test(GetProfitSharingAmounts|DeleteProfitSharingReceiver)' -count=1`
- `go test ./logic -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication_(OrdinaryProfitSharingSuccessFinishesOrder|ProfitSharingSuccessFinishesOrder|ProfitSharingReturnSuccessContinuesRefund)' -count=1`
- `go test ./db/sqlc -run 'TestPaymentOrder' -count=1`

**Fresh Revalidation: 2026-05-02**

- `go test ./... -run 'Constructor|ClientBuilder|OrdinaryServiceProvider' -count=1`
- `go test ./logic ./api -run 'Applyment|AccountWillingness|AuthorizeState|Settlement' -count=1`
- `go test ./logic ./db/sqlc -run 'CreatePaymentOrder|PaymentOrderService|OrdinaryServiceProvider|DirectPayment' -count=1`
- `go test ./logic ./db/sqlc -run 'CombinedPayment|ReservationDishes|CreateCombinedPayment|OrdinaryServiceProvider' -count=1`
- `go test ./logic ./worker -run 'PaymentOrderQuery|PaymentTimeout|CombinedPaymentTimeout|Channel' -count=1`
- `go test ./logic -run 'Refund|MerchantReject|ReservationCancel|ReplaceOrder|PaymentFact|OrdinaryServiceProvider' -count=1`
- `go test ./logic ./worker ./db/sqlc -run 'ProfitSharing|ReceiverLifecycle|ProcessPayment|OrdinaryServiceProvider' -count=1`
- `go test ./api ./logic ./worker ./db/sqlc -run 'Wechat|Callback|Notification|PaymentFactApplication|OrdinaryServiceProvider' -count=1`
- `go test ./logic ./worker -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication_OrdinaryProfitSharingReturnSuccessContinuesOrdinaryRefund|TestProcessTaskPaymentFactApplication|TestProcessTaskProfitSharingReturn|TestProcessTaskRefundResult_Ordinary|TestProcessTaskProfitSharing_Ordinary' -count=1 -v`

**Not Yet Verified In This Phase:** No live WeChat sandbox request IDs or real callback delivery IDs were available in this local pass. Merchant-agnostic receiver lifecycle prewarm remains historical/cold-path; ordinary runtime receiver add happens per payment order with concrete `sub_mchid` before the split request, because ordinary receiver APIs require sub-merchant scope.

#### Card 5.9: 接入商户管控诊断与处置通知

**Boundary:** 只接入普通服务商运营恢复能力，不阻塞首批支付主线。

**Files:** ordinary service provider client/contracts/errorcodes, relevant `api/`, `logic/`, `worker/` paths and tests

- [x] payment/refund/profit-sharing 失败出现 merchant control-like 分类时，可触发或展示 merchant limitation diagnostic。
- [x] 商户平台处置通知按 `record_id` 幂等落 fact/alert。
- [x] 不活跃商户身份核实只由 operator action 触发，且仅用于对应 recover way。

**Validation:** `cd locallife && go test ./wechat ./api ./logic ./worker -run 'MerchantLimit|Violation|InactiveMerchant|OrdinaryServiceProvider'`

**Review Gate:** review 必须确认运营诊断不会绕过权限，不会把敏感 risk payload 过度返回或过度记录。

#### Card 5.9 Review Closeout: 2026-05-02

**Risk Class:** G3 operational recovery for money movement. 本卡不直接创建资金交易，但决定支付、退款、分账被微信管控时平台和前端能否给出明确恢复路径。

**Review Findings And Fixes:**

- Finding fixed: 普通服务商进件 service 在 ordinary client 缺失时曾走“已保存/稍后处理”式成功语义并推进主体状态。已改为 fail closed：返回 `ordinary service provider client not configured`，不推进状态；聚焦测试 `TestSubmitOrdinaryServiceProviderApplymentWithoutOrdinaryClientReturnsError` 先红后绿。
- Finding fixed: 普通服务商开户银行/省市/支行目录接口曾把 gin binding、英文参数名或上游 provider/raw 失败文本直接返回给前端，且部分上游失败只返回 `internal server error`。已改为所有参数错误写 warning 日志并返回中文行动指引；上游目录查询失败通过 `loggedServerError` 落真实错误，前端只收到“稍后重试/联系平台管理员检查微信支付目录服务日志”的中文指引，不暴露 SQL、provider、request_id 或 raw diagnostic token。
- Finding fixed: 支付/退款、结算账户、分账接收方和普通服务商商户管控/处置通知配置的参数绑定或本地校验失败曾有残留 gin binding、英文字段名或本地 validation 文案直返前端。已统一为先写 warning 日志（包含 operation、path/request_id 或 payment_order_id 等诊断上下文），再返回中文行动指引；覆盖 `TestCreatePaymentOrderAPI_InvalidPayloadReturnsChineseGuidance`、`TestModifyMerchantSettlementAccountMissingFields`、`TestGetMerchantSettlementAccountInvalidMaskRule`、`TestDeleteProfitSharingReceiver_ValidationErrorReturnsBadRequest`、`TestCreateOrdinaryViolationNotificationConfig_InvalidNotifyURLReturnsChineseGuidance`、`TestListOrdinaryWechatMerchantViolations_InvalidLimitReturnsChineseGuidance`、`TestCreateInactiveMerchantIdentityVerification_InvalidJSONReturnsChineseGuidance`。
- Finding fixed: `wechat_merchant_violations` 表注释仍只写平台收付通。新增 `000226_update_wechat_merchant_violation_comment`，明确该运营告警/事实表由平台收付通和普通服务商处置通知共用，并按微信 `record_id` 幂等持久化。
- Review passed: `/v1/platform/finance/wechat-ordinary/merchant-limitations/:sub_mch_id` 只在管理员路由下暴露，异常全部通过 `loggedServerError` 落日志，前端收到中文行动指引，不返回 SQL、provider raw payload 或 request_id。
- Review passed: `/v1/platform/finance/wechat-ordinary/merchant-limitations/:sub_mch_id/inactive-identity-verifications` 会先查询管控诊断，只在 recover way 为 `VERIFY_INACTIVE_MERCHANT_IDENTITY` 时允许 operator/admin action。
- Review passed: `/v1/webhooks/wechat-ordinary/violation-notify` 通过普通服务商通知解析后落 `wechat_merchant_violations`，重复 `record_id` upsert；解析、商户解析和持久化失败均写错误日志并释放通知 claim。

**Validation Run:**

- `go test ./wechat ./api ./logic ./worker -run 'MerchantLimit|Violation|InactiveMerchant|OrdinaryServiceProvider' -count=1`
- `go test ./logic ./api -run 'SubmitOrdinaryServiceProviderApplyment|Applyment|AccountWillingness|AuthorizeState|Settlement|OrdinaryServiceProviderGates|PlatformEcommerceFundManagementRoutesStayDisabled' -count=1`
- `go test ./api -run 'ApplymentBank|MerchantLimit|Violation|InactiveMerchant|OrdinaryServiceProviderGates|PlatformEcommerceFundManagementRoutesStayDisabled' -count=1`
- `go test ./api -run 'TestCreatePaymentOrderAPI_InvalidPayloadReturnsChineseGuidance|TestDeleteProfitSharingReceiver_ValidationErrorReturnsBadRequest|TestCreateOrdinaryViolationNotificationConfig_InvalidNotifyURLReturnsChineseGuidance|TestListOrdinaryWechatMerchantViolations_InvalidLimitReturnsChineseGuidance|TestCreateInactiveMerchantIdentityVerification_InvalidJSONReturnsChineseGuidance|TestGetMerchantSettlementAccountInvalidMaskRule|TestModifyMerchantSettlementAccountMissingFields' -count=1`
- `go test ./api -count=1`
- `go test ./db/sqlc -run '^$' -count=1`

**Residual Risk:** 未做真实微信处置通知投递；真实 callback ID、微信 `record_id` 与数据库记录的生产样本仍需 Phase 7.2 sandbox/live evidence。

### Phase 6: Unsupported Surface Removal And Cold Reserve

#### Card 6.1: 下线平台收付通资金管理入口

**Boundary:** 只移除或 gate 不再承诺的资金管理入口。

**Files:** backend fund-management handlers/routes, Swagger annotations if changed, frontend/weapp pages only if currently exposed

- [x] 移除或 gate 二级商户余额、平台余额、预约提现、注销后提现、补差、垫付回补入口。
- [x] 保留结算账户查询/修改和指向微信支付商户平台/商家助手的产品说明。
- [x] 不改 direct payment pages or APIs。

**Validation:** route tests for affected backend paths; `npm run lint` / `npm run compile` only if web/weapp files changed; `cd locallife && make swagger` if Swagger annotations changed.

**Review Gate:** review 必须确认 UI/API 不再承诺普通服务商没有的资金能力。

#### Card 6.2: 审计平台收付通冷备边界

**Boundary:** 只隔离 cold-reserve wiring，不删除 ecommerce code。

**Files:** backend construction/wiring docs/tests where ecommerce client remains injected

- [x] 确认 active merchant onboarding、payment、combine、refund、profit-sharing、callback、timeout、fact application 不依赖 `EcommerceClientInterface`。
- [x] 保留 ecommerce contracts/client 作为 cold reserve，但无新商户业务入口。
- [x] 不创建历史订单清算报表，除非生产数据证明存在 active 收付通订单。

**Validation:** `cd locallife && rg "EcommerceClientInterface|payment_channel.*ecommerce" main.go api logic worker scheduler db/query db/sqlc`

**Review Gate:** review 必须逐个残留引用分类为 cold reserve、historical query 或 defect。

#### Phase 6 Closeout: 2026-05-02

**Risk Class:** G3 capability removal and cold-reserve isolation. 本阶段防止普通服务商模式下继续承诺平台收付通资金管理、补差、垫付回补和全局 receiver lifecycle 能力。

**Unsupported Surface Evidence:**

- Backend route gates: `locallife/api/ordinary_unsupported_surface.go` 对商户余额、商户预约提现、注销提现、平台余额、运营商补差/补差回退/取消补差、平台异常退款、全局 receiver lifecycle repair 返回 `503`，并通过 `loggedServerError` 记录真实禁用原因。
- Backend guidance: API 返回中文业务语义，明确“普通服务商模式不支持在平台内查询余额、发起提现、注销提现、补差或垫付回补，请前往微信支付商户平台/商家助手处理资金操作；本平台仅支持结算账户查询和修改”。
- Mini Program: 删除 `merchant-finance-account.ts` 旧资金账户 API；商户提现、提现详情、注销提现页面只保留迁移指引和返回入口，不再调用余额/提现/注销提现 API。
- Web: 商户财务账户和运营财务页展示普通服务商迁移说明，资金操作指向微信支付商户平台/商家助手；错误展示走 `getUserFacingErrorMessage`，不泄漏 raw backend/provider 文本。
- Direct payment: 骑手押金提现、直连退款、索赔赔付相关直连支付页面和 API 未纳入普通服务商下线范围。

**Cold-Reserve Classification:**

- `main.go buildEcommerceClient` and worker injection: cold reserve for historical platform ecommerce callbacks, complaints, withdraw/cancel-withdraw recovery and explicit historical diagnostics; active merchant constructors receive ordinary client through `SetOrdinaryServiceProviderClient` and ordinary facade constructors.
- `logic.NewPaymentOrderServiceWithClients` and `logic.NewCombinedPaymentService`: legacy/cold constructors used by old tests or explicit ecommerce paths; active API composition uses `NewDefaultPaymentFacadeWithOrdinaryServiceProvider`, `NewPaymentOrderServiceWithOrdinaryServiceProvider` and `NewCombinedPaymentServiceWithOrdinaryServiceProvider`.
- `api/merchant_finance.go`, `api/merchant_cancel_withdraw.go`, `api/platform_finance.go`, `api/subsidy.go`, `api/profit_sharing_receiver_lifecycle_service.go`: retained implementation code is unreachable from ordinary active routes except through explicit gate wrappers; no new merchant UI/API entry points call them as supported capabilities.
- `worker/task_merchant_withdraw_result.go`, `worker/task_merchant_cancel_withdraw_result.go`, `worker/task_profit_sharing_receiver_lifecycle.go`: historical/cold task processors for old platform ecommerce records and diagnostics; ordinary runtime receiver sync happens per payment order and `sub_mchid`.
- `db/query/refund_order.sql ListEcommerceRefundOrdersForReconciliation`: historical/cold reconciliation query only; no new history-clearing report was created because no production evidence for active platform ecommerce orders was provided.
- `api/payment_callback.go` and `/v1/webhooks/wechat-ecommerce/*`: historical platform ecommerce callback receivers remain isolated from `/v1/webhooks/wechat-ordinary/*`; ordinary callback fact application uses `channel='ordinary_service_provider'`.
- `logic/payment_fact_application_service.go`: ecommerce withdraw/cancel-withdraw fact branches remain historical; main-business payment/refund/profit-sharing application accepts ordinary channel explicitly and direct channel remains excluded where required.

**Validation Run:**

- `rg "EcommerceClientInterface|payment_channel.*ecommerce|ecommerceClient|EcommerceClient|PaymentChannelEcommerce|ecommerce.*payment_channel" main.go api logic worker scheduler db/query db/sqlc -g'*.go' -g'*.sql' -g'!*_test.go'`
- `go test ./logic ./api -run 'SubmitOrdinaryServiceProviderApplyment|Applyment|AccountWillingness|AuthorizeState|Settlement|OrdinaryServiceProviderGates|PlatformEcommerceFundManagementRoutesStayDisabled' -count=1`
- `npm run lint`, `npm run compile`, `npm run quality:check` in `weapp/`
- `npm run lint`, `npm run build` in `web/`

**Residual Risk:** 平台收付通历史 records 若生产中仍存在，只能通过保留的 cold-reserve worker/API 排障；本迁移不新增清算报表，也不恢复平台资金管理入口。

### Phase 7: G3 Validation And Release Evidence

#### Card 7.1: 生成物和 focused unit validation

**Boundary:** 只做本地生成物和单元/包级验证收口。

**Files:** generated sqlc, generated mocks, generated swagger if affected, test output evidence

- [x] Run `cd locallife && make sqlc` after SQL/query changes。
- [x] Run `cd locallife && make mock` after interface changes。
- [x] Run `cd locallife && make swagger` if routes or annotations changed。
- [x] Run `cd locallife && make test-unit`。
- [x] Run `cd locallife && make check-generated` if generated sources changed。

**Validation:** command output must be attached to phase closeout.

**Review Gate:** review cannot close if generated files are stale or if a required generation command was skipped without reason.

#### Card 7.1 Closeout: 2026-05-02

**Generation And Local Validation Evidence:**

- `make sqlc` passed after SQL/query and migration changes.
- `make mock` passed after ordinary service provider interface/mock changes.
- `make swagger` passed after ordinary routes, disabled surface routes and response annotations changed.
- `make check-generated` passed and reported `generated artifacts are in sync`.
- `make test-safety` passed for high-risk safety regressions.
- `go test ./api -count=1` passed.
- `make test-unit` passed.
- `go test ./wechat/ordinaryserviceprovider -run TestOrdinaryServiceProviderClientInterfaceUsesOnlyModuleContracts -count=1 -v` passed after the module-contract boundary fix.
- `go test ./api ./logic ./wechat/ordinaryserviceprovider/... -count=1` passed after regenerating the ordinary service provider mock.
- After the final applyment bank catalog error-contract fix, re-ran `go test ./api -count=1`, `make check-generated`, `make test-safety`, `make test-unit`, `npm run lint && npm run build` in `web/`, and `npm run lint && npm run compile && npm run quality:check` in `weapp/`; all passed.
- `git diff --check` passed with no whitespace errors.

**Review Result:** generated sqlc, gomock, Swagger docs and backend unit/safety checks are synchronized with the codebase. No local generated-artifact finding remains open.

**Additional Local Review Pass: 2026-05-02**

- Finding fixed: `POST /v1/orders/{id}/replace` 在 ordinary service provider client 缺失时会落入通用 500。已改为 `503`，通过 `loggedServerError` 记录真实异常，前端只收到“商户支付能力未完成配置，当前无法处理改菜支付或退款，请联系平台处理后重试”的中文行动指引。
- Finding fixed: `replaceOrder` 的 URI/body binding 失败曾把 gin/strconv/raw field 文本直接返回前端。已改为 warning 日志 + 中文行动指引：“改菜订单编号无效，请刷新页面后重试”或“改菜请求参数格式无效，请至少选择一个菜品后重试”。
- Finding fixed: 普通服务商退款创建错误映射遗漏 ordinary refund client missing 分支。已映射为 `503` `logic.RequestError`，保留原始 cause 供日志诊断，前端收到“微信服务商退款配置未完成，当前无法发起退款，请联系平台处理”。
- Finding fixed: 共享 `writeLogicRequestError` 曾把旧英文 `logic.RequestError` 文案直接返回前端；现在统一保留日志里的原始错误，并把旧支付/退款/分账/设备推送服务商错误映射为中文行动指引，未知英文错误按状态码返回稳定中文兜底，不暴露 SQL、provider、request_id 或内部字段名。
- Finding fixed: `TestListRefundOrdersByPaymentAPI/Forbidden_NotOwner` 依赖随机用户 ID 不碰撞，导致偶发进入成功列表分支。已固定 other user fixture，避免权限回归测试随机漂移。

**Additional Validation Run: 2026-05-02**

- `go test ./api -run 'TestRegisterMerchantAppDeviceAPI/UnsupportedProvider|TestListRefundOrdersByPaymentAPI/Forbidden_NotOwner|TestWriteLogicRequestError' -count=1 -v`
- `go test ./api -count=1`
- `go test ./api ./logic ./worker ./wechat/ordinaryserviceprovider/... -count=1`
- `make check-generated`
- `make test-safety`
- `make test-unit`
- `npm run lint && npm run build` in `web/`
- `npm run lint && npm run compile && npm run quality:check` in `weapp/`
- `git diff --check`

**Additional Review Result:** 本地 review 未发现新的普通服务商代码/文档不同步项；未勾选的仍只有 Card 7.2 所列外部 WeChat sandbox/live 证据。

#### Card 7.2: 高风险集成与 sandbox validation

**Boundary:** 只验证端到端外部路径，不改实现，除非发现 bug 后回到对应阶段修复。

**Files:** integration tests, sandbox evidence, callback/fact records

- [ ] Verify applyment/account-willingness state if sandbox supports it。
- [ ] Verify single payment create/query/close。
- [ ] Verify combine create/query/close。
- [ ] Verify refund create/query and refund callback。
- [ ] Verify profit sharing create/query/return/unfreeze and notification。
- [ ] Verify merchant limitation query and violation callback setup if sandbox supports it。
- [x] Record unsupported sandbox paths as residual risk with owner and recheck trigger。

**Validation:** targeted integration tests plus sandbox request IDs, callback IDs, and database fact/application records.

**Review Gate:** G3 release review must explicitly call out unverified callback, retry, idempotency, payment, refund, profit-sharing, and merchant-control paths.

#### Card 7.2 Sandbox Evidence Status: 2026-05-02

**Status:** Release-blocking external evidence remains unavailable in this local environment. No live WeChat sandbox request IDs, callback IDs, or provider-side transaction/fund records were present in repository config or test fixtures during this pass.

**Recorded Residual Risk And Owner:**

- Owner: platform payment release owner / operator with WeChat sandbox credentials.
- Recheck trigger: before enabling ordinary service provider for any production merchant, and after every WeChat certificate/APIv3 key/notify URL rotation.
- Required evidence: applyment/application IDs, account-willingness authorization state, single payment out_trade_no/transaction_id, combine out_trade_no/sub_order IDs, refund out_refund_no/refund_id, profit sharing out_order_no/order_id, return out_return_no/return_id, unfreeze result, merchant limitation sub_mchid query result, violation callback notification ID and `record_id`, and corresponding database fact/application/command IDs.
- Release decision: do not claim G3 external readiness until the above evidence is attached. Local unit/safety validation is sufficient for code review closure, not for live cutover.

**Why This Is Not A Downgrade:** The implementation fails closed when ordinary runtime config/client is missing, and the release evidence explicitly blocks live enablement instead of falling back to platform ecommerce or pretending sandbox was exercised.

#### Card 7.3: Release closeout and rollback readiness

**Boundary:** 只做发布判断、证据归档和 rollback/readiness 文档。

**Files:** release checklist/runbook only if implementation creates new recurring operational process

- [x] Summarize changed trust boundaries and production impact radius。
- [x] Record validation evidence for api/logic/db/sqlc/worker/scheduler/callback layers。
- [x] Record rollback or disablement option for ordinary service provider active merchant flow。
- [x] Record observability gaps: request id, provider code, callback id, fact id, command id, merchant sub_mchid。
- [x] Confirm no unresolved review findings remain。

**Validation:** release evidence must include commands, sandbox IDs, unresolved risk register entries, and review closeout.

**Review Gate:** release is blocked if high-risk paths have no evidence and no explicit owner/recheck trigger.

#### Phase 7 Closeout: 2026-05-02

**Trust Boundary Changes:**

- New ordinary service provider integration boundary lives in `locallife/wechat/ordinaryserviceprovider`; SDK DTOs and `core.APIResult` / `core.APIError` stay inside that boundary.
- Active merchant main-business payment, combine, refund, profit-sharing, callback, timeout and fact application now use `payment_channel='ordinary_service_provider'`.
- Platform ecommerce is retained only as cold reserve/historical diagnostics and webhook/task handlers; active merchant constructors must not use it as fallback.
- Unsupported platform ecommerce fund-management capabilities are gated at route level and removed from supported Mini Program/Web merchant affordances.

**Production Impact Radius:**

- Impacted: merchant onboarding/applyment, account-willingness authorization, settlement account query/modify, merchant payment order creation, combine payment, refund, profit sharing, profit sharing return/unfreeze/remaining amount query, ordinary payment/refund/profit-sharing/violation callbacks, timeout/recovery schedulers, merchant/operator finance guidance.
- Not impacted by design: direct payment for rider deposit, claim recovery payments, direct refunds and direct transfer/payout flows.

**Rollback / Disablement Option:**

- Disable ordinary service provider config or notify URL in non-production to keep ordinary client nil and force fail-closed behavior for active merchant money paths.
- In production, `validateProductionPaymentRuntime` requires ordinary service provider runtime config; rollback must be an explicit deployment/config rollback, not an implicit ecommerce fallback.
- Route gates for unsupported platform ecommerce fund management should stay enabled unless product/legal requirements change and a new payment-domain review reopens the capability.

**Observability Gaps To Close Before Live Cutover:**

- Need live provider request IDs and provider codes for applyment, payment, combine, refund, profit-sharing and merchant-control calls.
- Need callback notification IDs, WeChat `record_id`, fact IDs, command IDs, fact application IDs and merchant `sub_mchid` samples from sandbox/live callbacks.
- Need dashboard or runbook links for ordinary callback failure counters, recovery scheduler failures and merchant limitation diagnostics after production enablement.

**Review Result:** No unresolved local code review findings remain after the applyment nil-client fix, applyment bank catalog error-contract fix, ordinary-service-provider JSAPI pay params boundary fix, replace-order/payment-request error-contract fixes, Mini Program unsupported API removal, Web lint cleanup, table-comment migration and full local validation. Release remains blocked only on Card 7.2 external sandbox/live evidence.

**Additional Contract Alignment Review: 2026-05-02**

- Finding fixed: `locallife/wechat/ordinaryserviceprovider/errorcodes` 未覆盖 README 4.10 已实现官方 endpoint 文档中的全部错误码。已补齐并分类 `APPLYMENT_NOTEXIST`、`APPLYMENT_NOT_EXIST`、`ALREADY_EXISTS`、`NOT_FOUND`、`OPENID_MISMATCH`、`FREQENCY_LIMIT`、`FREQUENCY_LIMIT_EXCEED` 等官方返回码，保持结构化日志里的 provider code/request id，同时前端仍收到稳定中文行动指引。
- Finding fixed: 普通服务商开户意愿确认契约曾把官方嵌套 `contact_info` 简化成字符串，且缺少 `subject_info.assist_prove_info`、`identification_info`、`addition_info.confirm_mchid_list` 等官方结构。已改为模块内自有 contract 类型，不再使用本地简化 DTO 作为微信请求结构。
- Finding fixed: 普通服务商查询剩余待分金额官方应答字段是 `unsplit_amount`，本地 contract 曾使用 `amount`。已改为 `UnsplitAmount json:"unsplit_amount"`，调用方同步读取官方字段。
- Finding fixed: 特约商户进件 `biz_store_info` 一旦出现，官方要求 `biz_store_name`、`biz_address_code`、`biz_store_address`、`store_entrance_pic`、`indoor_pic` 完整。已在 contract `Validate` 中 fail closed，避免再次向微信发送半截门店结构；当前业务只申报小程序场景时不再把店铺二维码伪装为线下场所门头图。
- Finding fixed: 支付、退款、合单、分账和商户处置通知缺少官方加密通知 envelope / 解密后 payload 的 module-owned contract 类型。已补 `NotificationRequest`、`NotificationResource`、`RefundNotificationPayload`、`ProfitSharingNotificationPayload`、`MerchantViolationNotificationPayload` 等结构，并用字段名测试固定。
- Evidence: 生成 `artifacts/wechat-ordinary-service-provider-official-contract-snapshot-2026-05-02.json`，来源为本 README 4.10 中普通服务商已实现 endpoint 的官方文档链接；自动比对后，除文件上传 multipart 字段外，本地 contracts 的 JSON 字段名覆盖官方请求、应答和通知 payload 叶子字段。
- Guidance updated: `.github/standards/domains/wechat-payment/README.md` 明确普通服务商模块 `contracts/errorcodes/endpoint path` 必须以官方 endpoint 文档为唯一结构真值，禁止从平台收付通、直连支付、历史 artifact 或本地业务 DTO 反推字段。

**Additional Contract Anti-Drift Review: 2026-05-02**

- Finding fixed: 普通服务商 `errorcodes` 只有通用 `Classify`，没有像上层 `wechat/errorcodes` 一样把每个官方 endpoint 的错误码表固化为独立 code set。已新增 `DocumentedCodeSet`、官方原始错误码常量、canonical alias（`NOAUTH`/`NO_AUTH`、`RULELIMIT`/`RULE_LIMIT`、`SYSTEMERROR`/`SYSTEM_ERROR`、`FREQENCY_LIMIT`/`FREQUENCY_LIMIT`、`APPLYMENT_NOTEXIST`/`APPLYMENT_NOT_EXIST`）以及已实现 endpoint 的专用 `*DocumentedCodes` 集合；测试逐项断言 README 4.10 snapshot 中的官方错误码列表。
- Finding corrected after review: endpoint 级官方 URL 文档目录不应进入生产代码，但普通服务商模块必须有可执行的能力组契约映射。已移除生产 `contracts.OfficialEndpoints` / `errorcodes.OfficialEndpointCodeSets` 文档注册表，改为在 `contracts.CapabilityGroups` / `EndpointContracts` 中按能力组固化 endpoint ID、method/path、operation、请求/响应 contract 类型、状态归属和 request validation 入口；`errorcodes.CapabilityCodeSetGroups` / `EndpointCodeSets` 按能力组固化每个有错误码表接口的 DocumentedCodes。官方标题/URL 仍只在 README、snapshot 和 `_test.go` 夹具中用于反漂移验证。
- Finding fixed: 文件上传是 multipart 官方请求结构，之前只有响应 `MediaUploadResponse`，registry 无法引用请求 contract。已补 `MediaUploadRequestMultipart` / `MediaUploadMeta`，明确 `file` 和 `meta.filename` / `meta.sha256` 的官方结构。
- Finding fixed: 不活跃商户身份核实创建请求曾把本地 `business_code` 发给微信；官方文档该请求只包含 `sub_mchid`。已将 `BusinessCode` 标记为 `json:"-"`，保留本地关联值但禁止进入微信请求体。
- Finding fixed: 合单查询官方响应包含 `combine_payer_info` 和 `scene_info`，本地 response 缺少这两个官方结构。已补入 `CombineQueryResponse`，避免后续业务从 map 或旧 ecommerce DTO 反推字段。
- Finding fixed: 退款申请官方请求包含可选 `funds_account`，旧测试用“业务当前不用资金账户”为理由禁止 contract 暴露该字段。已按官方文档恢复 `RefundCreateRequest.FundsAccount json:"funds_account,omitempty"`，业务层可以继续不传，但 contract 不再删减官方结构。
- Evidence: `contracts` 包测试确认 `artifacts/wechat-ordinary-service-provider-official-contract-snapshot-2026-05-02.json` 中 48 个 endpoint 标题与测试夹具完全一致，且本地 request/response struct JSON 字段覆盖官方字段；生产包暴露的是可执行 contract/errorcode 映射和 validation，不暴露官方 URL 文档目录。

**Additional Validation Run: 2026-05-02**

- `/usr/local/go/bin/go test ./wechat/ordinaryserviceprovider/... -count=1`
- `/usr/local/go/bin/go test ./logic ./api -run 'Applyment|Inactive|Violation|PaymentOrder|ProfitSharing|MerchantBindBank' -count=1`
- `contracts` anti-drift tests: snapshot endpoint count 48, test-only binding count 48, missing `[]`, extra `[]`；生产 `contracts` 包包含可执行能力组 contract map，不包含官方 URL 文档目录。
- Previous focused regression retained: `go test ./wechat/ordinaryserviceprovider/... ./logic ./api -run 'TestClassify|TestAccountWillingnessSubmitRequestJSONUsesOfficialNestedStructure|TestProfitSharingRemainingAmountResponseUsesOfficialUnsplitAmountField|TestApplymentSubmitRequestValidatesOfficialStoreSceneFields|TestClientRoutesApplymentSettlementAndMerchantManagementEndpoints|TestBuildOrdinaryServiceProviderApplymentRequestIncludesDiscountActivities|TestSubmitMerchantApplyment|TestApplyment|TestMerchantBindBank' -count=1`

**Residual Risk:** 本次为本地 contract/errorcode 对齐和字段名覆盖验证；仍未替代 Card 7.2 的真实微信 sandbox/live 请求、回调投递和 provider request id 证据。

**Additional Runtime Contract Wiring Review: 2026-05-02**

- Scope decision: 当前上线测试收口不实现交易账单、资金账单、分账账单申请/下载和分账最大比例查询；这些官方文档继续保留在 README 4.10 作为后续能力来源，但不计入本轮 live test 前阻塞项。
- Finding fixed: 普通服务商生产 client 方法此前仍有局部手写 validation，未统一走 `contracts.EndpointContracts` 中的 endpoint request validator。已将已实现 client endpoint 的请求校验改为 `contracts.ValidateEndpointRequest(endpointID, req)` 入口，主业务层继续只调用普通服务商模块能力方法。
- Finding fixed: 普通服务商 provider error 只有 operation，没有 endpoint/capability/code-set 元数据。已在 `ProviderError` 中补 `EndpointID`、`CapabilityID`、`DocumentedCodeSet` 和 `DocumentedProviderCode`，SDK APIError、local response decode error、validation error、媒体上传 error 都携带 endpoint-aware 诊断；日志边界会输出 endpoint/capability、官方错误码表和 provider code 是否属于该 endpoint 的 DocumentedCodes。
- Finding fixed: 缺少“契约里有但无运行时入口”的防线。已新增 `client_capability_alignment_test.go`，要求每个已映射 `EndpointContract` 都有 client method、local JSAPI pay params 生成器或 callback route owner；callback endpoint 明确归属到 `ParseNotification + api` 回调处理路径。
- Evidence: 新增红绿测试覆盖 validation failure 和 SDK provider failure 的 endpoint/capability/error-code-set 元数据；`UploadImage` 也按 `EndpointMerchantMediaUpload` 进入 endpoint-aware validation/error path。

## 7. Recommended Execution Order

1. Phase 0: 文档、SDK 策略、review 闭环。
2. Phase 1: channel migration、constants、query boundary。
3. Phase 2: ordinary service provider module skeleton、transport、notification、interface/mock。
4. Phase 3: contracts and errorcodes。
5. Phase 4: client endpoints。
6. Phase 5: composition and business cutover。
7. Phase 6: unsupported surfaces and platform收付通 cold reserve。
8. Phase 7: G3 validation and release evidence。

每个 phase 完成后必须 review；review findings 修完后复审，复审未通过不得进入下一 phase。若某 phase 内的 cards 相互独立，可以并行实现，但必须汇总到同一个 phase review。

## 8. Done Criteria

- New merchant business payments create `payment_channel='ordinary_service_provider'`.
- New module path and Go types use `ordinaryserviceprovider` / `OrdinaryServiceProvider`, while persisted channel values use `ordinary_service_provider`; no new code uses `partner_service` / `PartnerService` as the module identity.
- Official `wechatpay-go` remains the official dependency or module-private adapter; there is no project fork replacing it.
- SDK DTO、`core.APIResult`、`core.APIError` and `services/*` types do not cross the ordinary service provider module boundary.
- SDK-covered capabilities are wrapped inside the module; SDK-missing capabilities are implemented with module-owned contracts/errorcodes and signed transport.
- New merchant activation requires ordinary applyment success and completed account-willingness authorization.
- Merchant limitation query and merchant platform violation notification handling are available as ordinary service provider operational recovery paths.
- Unrelated payment paths are not modified by the ordinary service provider module work.
- No new merchant onboarding, payment, combine payment, refund, profit-sharing, callback, timeout, or fact-application path calls `EcommerceClientInterface`.
- Platform收付通 remains cold-reserved only; it is not implemented as a老商户兼容 branch.
- Platform收付通 fund management, withdraw, subsidy, and advance-refund reimbursement are not exposed as active merchant capabilities.
- Ordinary service provider contracts and errorcodes are the only source used by new ordinary-service-provider-channel paths.
- Callback, retry, timeout, query, and fact application paths are idempotent and channel-aware.
- `make sqlc`, `make mock`, `make test-unit`, and any route/swagger/UI validations required by touched files pass or have documented blockers with owners.
- Every phase has a review closeout record with findings fixed and re-reviewed until no unresolved findings remain.
