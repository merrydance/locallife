# 宝付二级户开户端到端实施计划

Updated: 2026-05-08
Risk class: G3 - payment/funds/account opening/callback/async recovery.

## 1. Goal

完成 LocalLife 平台、商户、骑手、运营商的宝付宝财通 2.0 二级户开户流程，并把商户开户后的微信聚合商户报备和小程序绑定授权目录接入到可恢复、可查询、可审计的端到端状态机。

本计划覆盖后端、数据库、宝付开户接口、微信直连核验费支付、小程序支付进度体验和恢复查询。实施时不得阻断可恢复的异步流程：支付、宝付开户、商户报备、授权目录绑定都必须以本地持久化状态和查询/回调终态为准。

## 2. Confirmed Decisions

- 宝付侧开户账户类型只有个人/机构。LocalLife 内部 `owner_type=platform` 只是本平台的主体类型，不是宝付账户类型。
- LocalLife 不再保留容易误导的 `BaofuAccountTypePlatform`。宝付账户类型只允许：
  - `personal`
  - `business`
- owner 到宝付账户类型映射：
  - 商户：`business`
  - 平台主体：`business`
  - 骑手：`personal`
  - 运营商：`personal`
- 所有商户，包括平台自己，宝财通机构开户 `industryId=9931`，对应宝付“公司所属行业”附录中的“零售B2C”。
- `sharingMerId` 按宝付客服口径等于开户/查询返回的 `contractNo`。
- 宝付已确认当前接入开户不需要 `qualificationTransSerialNo`。
- 宝付已确认当前接入开户不需要调用“资质文件上传接口1”，也不需要上传任何资质文件、证照图片或平台合作协议材料。
- 宝付已确认当前接入不需要 `platformNo` 和 `platformTerminalId`。这两个字段是代理模式条件字段，LocalLife 当前不开启代理模式，不配置、不传入、不作为开户前置条件。
- 平台协议模板仍是 LocalLife 自身的法务/用户确认材料，不进入宝付开户请求，也不通过宝付资质上传接口提交。
- 只有商户需要做微信聚合商户报备和绑定授权目录。
- 平台、骑手、运营商不做聚合商户报备和绑定授权目录，因为微信用户不直接向这些主体付款，它们只是后续分账接收方。
- 绑定授权目录属于异主体报备，目的是让微信用户支付时看到商户名而不是平台名。该实现方向是正确的，必须保留。
- 骑手/运营商开户前需要先支付核验费。默认金额为 `200` 分，由微信支付直连支付给平台。
- 商户/平台开户核验费由平台承担，不需要前置微信支付。
- 骑手/运营商核验费由平台预先存入宝付，宝付扣款；平台不承担用户侧费用，因此前端必须先完成直连支付，支付完成后进入宝付开户流程。

## 3. Baofoo Contract Facts

### 3.1 Account Open

- 接口：个人/机构开户接口 `T-1001-013-01`。
- `accType=1`：个人。
- `accType=2`：企业/个体。
- `businessType=BCT2.0`。
- 当前 LocalLife 请求构造规则：
  - 不传 `qualificationTransSerialNo`。
  - 不传 `platformNo`。
  - 不传 `platformTerminalId`。
  - 不调用资质文件上传接口。
  - 不上传平台合作协议、营业执照、身份证、银行卡或其他开户资质文件。
- 如果未来宝付要求切换代理模式或另行要求资质文件，需要先形成新的接口确认记录，再新增独立设计；不得把这几个字段在当前实现中静默改成必填。
- 同步返回中的 `state`：
  - `1` 成功
  - `0` 失败
  - `-1` 异常
  - `2` 开户处理中
- 开户是异步风险流程。即使同步调用返回处理中，也必须保存处理中状态，由开户结果通知或开户查询推进终态。

### 3.2 Qualification Upload

- 当前 LocalLife 开户流程不使用宝付“资质文件上传接口1”。
- 后端不新增 `baofu/qualificationupload` client、资质上传表、协议 artifact 表、宝付上传 ZIP 构建器或相关 OSS/media 依赖。
- 计划和实现中不得再以 `qualificationTransSerialNo` 作为开户本地前置条件。
- 法务协议、用户勾选记录、入驻资料等可以继续作为 LocalLife 自有合规材料保存，但这些材料不参与宝付开户接口。

### 3.3 Industry

- 机构开户字段 `industryId` 来自宝财通“公司所属行业”附录。
- LocalLife 当前统一使用 `9931`。
- `9931` 对应“零售B2C”。

## 4. Existing Repo Anchors

### 4.1 Backend

- 宝付开户服务：`locallife/logic/baofu_account_service.go`
- 宝付开户 client：`locallife/baofu/account/client.go`
- 开户 DTO：`locallife/baofu/account/contracts/official_open.go`
- 宝付账户绑定 SQL：`locallife/db/query/baofu_account_binding.sql`
- 宝付商户报备服务：`locallife/logic/baofu_merchant_report_service.go`
- 宝付商户报备 SQL：`locallife/db/query/baofu_merchant_report.sql`
- 宝付开户回调：`locallife/api/baofu_callback.go`
- 商户宝付 readiness：`locallife/api/merchant_baofu_readiness.go`
- 支付订单 API：`locallife/api/payment_order.go`
- 骑手押金直连支付：`locallife/api/rider.go`
- 支付订单 SQL：`locallife/db/query/payment_order.sql`
- 业务常量：`locallife/db/sqlc/constants.go`
- 配置：`locallife/util/config.go`
- 示例配置：`locallife/app.env.example`

### 4.2 Mini Program

- 通用支付工作流：`weapp/miniprogram/services/payment-workflow.ts`
- 骑手押金支付工作流：`weapp/miniprogram/services/rider-deposit-payment.ts`
- 支付 API：`weapp/miniprogram/api/payment.ts`
- 骑手 API：`weapp/miniprogram/api/rider.ts`

## 5. Target Architecture

新增一个“宝付开户流程”编排层，统一 owner 类型、核验费、开户、商户报备、授权目录绑定和恢复查询。

核心原则：

- `baofu_account_bindings` 继续表达最终宝付账户绑定结果。
- 新增流程表表达开户过程、核验费、开户流水和可恢复状态。
- 宝付开户请求由后端基于已审核主体资料构建；小程序不传宝付协议字段、代理模式字段或开户资质文件。
- 当前实现不依赖宝付资质上传接口、OSS 资质包、本地协议文件或 ZIP 上传。
- 开户 request builder 必须显式体现当前模式：`qualificationTransSerialNo`、`platformNo`、`platformTerminalId` 为空时不序列化，不从配置或 API 输入中补值。
- 所有上游请求先落 command 或本地流程记录，再调用宝付/微信。
- 所有回调和查询结果先落 fact 或 raw snapshot，再由业务服务推进状态。
- 前端只展示后端状态，不持有宝付/微信协议真值。
- 小程序所有支付都以查询结果作为终态，不以 `wx.requestPayment` 返回值作为最终真相。

### 5.1 Implementation Handoff Invariants

这一节是交接硬约束。实现者不能用“统一 owner 流程”“宝付字段看起来可选”“小程序传参更方便”等理由改写这里的设计。

#### 5.1.1 Role And Route Authority Matrix

| Owner | Backend route | Required auth/middleware | Server-side owner resolution | Client-provided owner fields |
| --- | --- | --- | --- | --- |
| merchant | `GET/POST /v1/merchant/settlement-account` | `RoleMerchantOwner` + `MerchantOwnerOnlyMiddleware` or equivalent owner-only check | `requireOwnedMerchantForUser(auth.user_id)` | None. Do not accept `owner_type` or `owner_id`. |
| rider | `GET/POST /v1/rider/settlement-account` | `RoleRider` + `RiderMiddleware` | `GetRiderByUserID(auth.user_id)` | None. |
| operator | `GET/POST /v1/operators/me/settlement-account` | `RoleOperator` + `LoadOperatorMiddleware` | `GetOperatorFromContext(ctx)` | None. |
| platform | `GET/POST /v1/platform/finance/settlement-account` | `RoleAdmin` | constant `owner_type=platform`, `owner_id=0` | None. |

Do not add a Mini Program or merchant/operator self-service endpoint that accepts arbitrary owner identifiers. If a future admin repair endpoint needs explicit owner input, it must live under an admin/platform route, require `RoleAdmin`, write an audit log, and never share request DTOs with the self-service routes.

#### 5.1.2 Account Profile Source Of Truth

Baofoo open-account payloads must be built from backend-owned profile data, not from page parameters. Add a normalized profile layer instead of letting each route assemble provider fields independently.

Create `baofu_account_opening_profiles` and keep it separate from `baofu_account_opening_flows`:

- One row per `(owner_type, owner_id)` with `profile_status`.
- Store the Baofoo-required identity and bank fields in normalized columns matching the local `OpenAccountRequest` shape.
- Encrypt sensitive local-rest fields with the existing `DATA_ENCRYPTION_KEY` / AES-GCM utility, including certificate numbers, bank account numbers, reserved mobile numbers, email and private-card cardholder names.
- Store only masked or redacted values in `provider_request_snapshot`; do not persist plaintext ID numbers or bank card numbers in flow snapshots.
- Profiles are backend controlled. Mini Program may submit missing settlement fields through role-specific profile APIs, but the service must validate owner identity and write the encrypted profile before any Baofoo request is constructed.
- Do not depend on `ecommerce_applyments.account_number` or other WeChat applyment encrypted fields as Baofoo plaintext. Those fields were prepared for WeChat applyment and must not be treated as LocalLife-decryptable Baofoo source data.

Field source matrix:

| Baofoo field | merchant business | platform business | rider personal | operator personal |
| --- | --- | --- | --- | --- |
| `owner_type` / `owner_id` | Merchant owner route resolves merchant ID | fixed `platform/0` | rider route resolves rider ID | operator route resolves operator ID |
| `account_type` | `business` | `business` | `personal` | `personal` |
| `transSerialNo` | generated/persisted per opening attempt as `open_trans_serial_no` | same | same | same |
| `loginNo` | deterministic helper, e.g. `LLBFOM0000000123` | `LLBFOP0000000000` | `LLBFOR0000000123` | `LLBFOO0000000123` |
| `customerName` | approved merchant profile legal business name | platform profile legal business name | rider legal name | operator legal person name |
| `certificateType` | `LICENSE` | `LICENSE` | `ID` | `ID` |
| `certificateNo` | approved merchant application/profile business license number | platform profile business license number | rider ID number | operator legal person ID number |
| `corporateName` | approved merchant legal person name | platform profile legal person name | not sent | not sent |
| `corporateCertType` | `ID` | `ID` | not sent | not sent |
| `corporateCertId` | approved merchant legal person ID number | platform profile legal person ID number | not sent | not sent |
| `email` | merchant Baofoo profile settlement email | platform Baofoo profile settlement email | not sent | not sent |
| `industryId` | config/default `9931`, no UI override | config/default `9931`, no UI override | not sent | not sent |
| `selfEmployed` | `false` | `false` | not sent | not sent |
| `cardNo` | merchant Baofoo profile bank account number | platform Baofoo profile bank account number | rider Baofoo profile personal bank card number | operator Baofoo profile personal bank card number |
| `mobileNo` | not sent | not sent | rider Baofoo profile bank reserved mobile | operator Baofoo profile bank reserved mobile |
| `cardUserName` | business account optional only when Baofoo explicitly requires; otherwise omit | same | same as legal name | same as legal person name |
| `bankName` / `depositBankProvince` / `depositBankCity` / `depositBankName` | merchant Baofoo profile bank metadata | platform Baofoo profile bank metadata | not sent | not sent |
| `contactName` / `contactMobile` | merchant contact from approved application/profile | platform profile contact | not sent | not sent |
| `needUploadFile` | `false` | `false` | `false` | `false` |
| `qualificationTransSerialNo` | omit | omit | omit | omit |
| `platformNo` / `platformTerminalId` | omit | omit | omit | omit |

Profile completion rules:

- Merchant identity fields can be initialized from the approved `merchant_applications` / `merchants.application_data`; bank/account/email/branch fields must come from the Baofoo profile if not already stored in a LocalLife-decryptable source.
- Platform profile is created or updated only through the admin-only backend route `POST /v1/platform/finance/settlement-account`. It must not be exposed in Mini Program.
- Rider identity fields can be initialized from `riders`; bank card number, reserved mobile and cardholder confirmation come from the rider Baofoo profile form.
- Operator identity fields come from the approved `operator_applications` legal person fields; personal bank card number, reserved mobile and cardholder confirmation come from the operator Baofoo profile form.

#### 5.1.3 Baofoo Payload Construction Rules

- `transSerialNo` is not `payment_order.out_trade_no`. It is the Baofoo opening attempt serial and must be persisted on the flow before calling Baofoo.
- `loginNo` is not the same as `transSerialNo`. Add a local helper or request field so the official Baofoo request uses a stable max-32-character login number.
- Current personal opening uses the personal four-factor path (`name + ID + bank card + bank reserved mobile`). Do not implement or switch to the personal two-factor path in this rollout.
- Baofoo account query must use `contractNo` when available. Before `contractNo` exists, it must query by the persisted `loginNo` plus certificate identity; update the local query DTO/client so `loginNo` is explicit and no longer abuses `OutRequestNo` as login number.
- Current-mode Baofoo account query must not require or send `platformNo`. Update the official query validation if needed so non-agent mode works without `platformNo`.
- `needUploadFile=false` for all current owner types.
- `qualificationTransSerialNo`, `platformNo` and `platformTerminalId` must stay empty in the local request builder and must be omitted by JSON serialization.
- `sharingMerId` is always normalized to Baofoo `contractNo` when the provider does not return a separate value.
- A request serialization test must assert the exact absence of `qualificationTransSerialNo`, `platformNo` and `platformTerminalId`.

#### 5.1.4 Flow State Machine

Canonical flow states:

- `profile_pending`
- `verify_fee_pending`
- `verify_fee_processing`
- `opening_processing`
- `merchant_report_processing`
- `applet_auth_pending`
- `ready`
- `failed`
- `voided`

Allowed transitions:

| From | Event | To | Owner scope |
| --- | --- | --- | --- |
| none | start/query with missing profile | `profile_pending` | all |
| `profile_pending` | profile completed, no user-side fee required | `opening_processing` | merchant/platform |
| `profile_pending` | profile completed, user-side fee required | `verify_fee_pending` | rider/operator |
| `verify_fee_pending` | pending payment returned/invoked | `verify_fee_processing` | rider/operator |
| `verify_fee_pending` / `verify_fee_processing` | payment query/callback confirms paid | `opening_processing` | rider/operator |
| `verify_fee_pending` / `verify_fee_processing` | payment closed/failed | `verify_fee_pending` | rider/operator |
| `opening_processing` | Baofoo success for merchant | `merchant_report_processing` | merchant |
| `opening_processing` | Baofoo success for non-merchant | `ready` | platform/rider/operator |
| `opening_processing` | Baofoo processing | `opening_processing` | all |
| `opening_processing` | Baofoo terminal failure | `failed` | all |
| `merchant_report_processing` | report success | `applet_auth_pending` | merchant |
| `merchant_report_processing` | report processing | `merchant_report_processing` | merchant |
| `merchant_report_processing` | report terminal failure | `failed` | merchant |
| `applet_auth_pending` | APPLET auth success | `ready` | merchant |
| `applet_auth_pending` | APPLET auth terminal failure | `failed` | merchant |
| any non-terminal | admin voids invalid profile/fraud/wrong owner | `voided` | all |

Database and retry rules:

- Add one partial unique active-flow index on `(owner_type, owner_id)` where `state IN ('profile_pending','verify_fee_pending','verify_fee_processing','opening_processing','merchant_report_processing','applet_auth_pending')`.
- Do not create a new flow when a `ready` binding already exists. Return the ready account.
- A `failed` flow may be retried by creating a replacement flow after the profile is corrected. Rider/operator replacement flows must reuse the owner-level paid verify-fee proof unless an admin voided it for fraud, wrong owner identity or legally invalid application.
- State updates must be conditional, e.g. `WHERE id=$1 AND state IN (...)`, so duplicate callbacks and scheduler races are idempotent.

#### 5.1.5 Verify-Fee Idempotency

The verify-fee payment idempotency key is owner-level, not flow-level:

```text
business:baofu_account_verify_fee;owner_type:<owner_type>;owner_id:<owner_id>;purpose:initial_open
```

- Do not include `flow_id` in `payment_orders.attach`. Replacement flows would otherwise bypass the paid proof and charge the user twice.
- Store the link from flow to payment in `baofu_account_opening_flows.verify_fee_payment_order_id`.
- Reuse an unexpired `pending` payment when `(business_type, attach, amount, user_id)` match.
- Reuse a `paid` payment as proof for all replacement flows for the same owner and purpose.
- If the configured amount changes after a payment is paid, do not charge the delta automatically. The paid `payment_orders.amount` remains the proof amount for that owner opening intent.

#### 5.1.6 Callback And Query Trust Rules

- WeChat direct payment callback trust comes only from the existing WeChat signature/decryption path and payment query verification. `wx.requestPayment` success is never terminal truth.
- Baofoo account callback trust comes only from the existing Baofoo signature/decryption client path.
- Match Baofoo account callbacks primarily by `transSerialNo == baofu_account_opening_flows.open_trans_serial_no`.
- If `transSerialNo` is missing but `contractNo` is present, fallback only to an existing unique `baofu_account_bindings.contract_no` match. Never infer owner from callback text or raw payload.
- A callback/query result must cross-check the flow owner, account type and current state before mutating rows.
- Raw provider responses may be stored for audit, but user-facing responses must use safe Chinese messages and not expose provider internals.
- Unmatched callbacks should be recorded as external payment facts or raw operational evidence and alerted; they must not create owner bindings without a matching local flow.

#### 5.1.7 Fee Accounting Rules

- Merchant/platform: no user-side `payment_order`. On successful Baofoo activation, always record one `baofu_fee_ledger` row with `fee_type='account_open_verify_fee'`, `payer_type='platform'`, `amount=BAOFU_ACCOUNT_VERIFY_FEE_FEN`, `business_object_type='baofu_account_binding'`, `business_object_id=<binding_id>`.
- Rider/operator: the user-side 2 yuan fee is represented by `payment_orders.business_type='baofu_account_verify_fee'`. Do not create an `account_open_verify_fee` ledger row with `payer_type='platform'` for rider/operator, because that would report the fee as platform-borne. If a future reconciliation needs to represent Baofoo prepaid-account deduction separately, add a distinct provider-cost design instead of reusing the platform-borne fee ledger semantics.
- Duplicate Baofoo callbacks must not create duplicate fee ledger rows; keep the existing unique ledger key by `(fee_type, business_object_type, business_object_id)`.

#### 5.1.8 Merchant Report And Auth Boundary

- Only `owner_type=merchant` enters merchant report and APPLET auth.
- `bctMerId` is `sharing_mer_id`, which is normalized to `contractNo`.
- Merchant flow reaches `ready` only after Baofoo account active, WECHAT merchant report success and `BindSubConfig(authType=APPLET)` success.
- Platform/rider/operator flow reaches `ready` as soon as Baofoo account binding is active.

## 6. Backend Implementation Plan

### Task 1: Config And Constants

Files:

- Modify: `locallife/util/config.go`
- Modify: `locallife/app.env.example`
- Modify: `locallife/db/sqlc/constants.go`
- Modify: `locallife/logic/baofu_account_service.go`

Steps:

- [ ] Add `BAOFU_ACCOUNT_VERIFY_FEE_FEN`, default `200`.
- [ ] Add `BAOFU_BUSINESS_INDUSTRY_ID`, default `9931`.
- [ ] Do not add Baofoo qualification upload URL/config for current opening flow.
- [ ] Do not add `BAOFU_PLATFORM_NO` or `BAOFU_PLATFORM_TERMINAL_ID` as required config for current opening flow.
- [ ] Remove remaining hard-coded `BaofuAccountOpenVerifyFeeFen = 100` behavior.
- [ ] Ensure platform/merchant business account requests default `IndustryID` to configured `9931` unless explicitly overridden by trusted backend configuration.
- [ ] Keep `BaofuAccountTypePersonal` and `BaofuAccountTypeBusiness` as the only account type constants.
- [ ] Ensure any previous draft code/migration referring to `BaofuAccountTypePlatform`, qualification upload, agreement artifacts, `qualification_trans_serial_no`, `platform_no`, or `platform_terminal_id` is removed or left unused only as official optional DTO fields.

Validation:

- [ ] `PATH="/usr/local/go/bin:$PATH" go test ./util ./logic -run 'Test.*Baofu|Test.*Config' -count=1`

### Task 2: Payment Business Type For Account Verify Fee

Files:

- Create migration under `locallife/db/migration/`
- Modify: `locallife/db/query/payment_order.sql`
- Modify: `locallife/db/sqlc/constants.go`
- Modify: `locallife/api/payment_order.go` only if response enum/contracts need expansion
- Modify: `weapp/miniprogram/api/payment.ts`

Steps:

- [ ] Add `payment_orders.business_type='baofu_account_verify_fee'` by dropping and recreating `payment_orders_business_type_check` with the new value included.
- [ ] Add constant `PaymentBusinessTypeBaofuAccountVerifyFee = "baofu_account_verify_fee"` in the backend SSOT constants and use it everywhere instead of magic strings.
- [ ] Store canonical owner-level `attach` in the existing key-value style used by payment code: `business:baofu_account_verify_fee;owner_type:<owner_type>;owner_id:<owner_id>;purpose:initial_open`.
- [ ] Add a partial unique index so one owner onboarding intent cannot have multiple active verify-fee payments:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS payment_orders_baofu_verify_fee_active_uidx
    ON payment_orders (business_type, attach)
    WHERE business_type = 'baofu_account_verify_fee'
      AND status IN ('pending', 'paid');
```

- [ ] Add `GetBaofuVerifyFeePaymentByAttach` and `GetReusableBaofuVerifyFeePayment` queries. Reuse `pending` payments that are unexpired and exactly match owner, purpose, authenticated payer and amount; reuse `paid` payments as the fee proof for the same owner/purpose even when a failed flow is replaced.
- [ ] Link the selected payment to the active flow through `baofu_account_opening_flows.verify_fee_payment_order_id`. Do not make `flow_id` part of the payment idempotency key.
- [ ] Create/reuse verify-fee payment only inside the Baofoo onboarding service after resolving the authenticated rider/operator. Do not expose a generic client-controlled “create baofu verify fee” payment endpoint where the Mini Program can forge owner, amount, purpose or flow.
- [ ] Ensure verify-fee payment uses `payment_type='miniprogram'`, `payment_channel='direct'`, `requires_profit_sharing=false`, `order_id IS NULL`, `reservation_id IS NULL`, and `user_id` equal to the authenticated payer.
- [ ] Ensure query payment endpoint can query this payment type with existing direct payment query path, or return the payment status through the settlement-account query response.
- [ ] Ensure payment ledger/listing renders this business type as “开户核验费” and does not classify it as order payment, rider deposit, membership recharge, or claim recovery.
- [ ] Ensure direct payment callback routes this business type to the Baofoo onboarding consumer and does not trigger rider deposit side effects.
- [ ] Extend Mini Program `BusinessType` union with `baofu_account_verify_fee`.

Validation:

- [ ] `PATH="/usr/local/go/bin:$PATH" make sqlc`
- [ ] `PATH="/usr/local/go/bin:$PATH" make check-generated`
- [ ] Focused API tests for create/query verify-fee payment.

### Task 3: Opening Flow Persistence

Files:

- Create migration under `locallife/db/migration/`
- Create: `locallife/db/query/baofu_account_opening_profile.sql`
- Create: `locallife/db/query/baofu_account_opening_flow.sql`
- Modify: `locallife/db/sqlc/constants.go`

Create table `baofu_account_opening_profiles`:

- `id`
- `owner_type`
- `owner_id`
- `account_type`
- `profile_status`
- `legal_name`
- `certificate_type`
- `certificate_no_ciphertext`
- `certificate_no_mask`
- `email_ciphertext`
- `email_mask`
- `customer_name`
- `alias_name`
- `corporate_name`
- `corporate_cert_type`
- `corporate_cert_id_ciphertext`
- `corporate_cert_id_mask`
- `corporate_mobile_ciphertext`
- `corporate_mobile_mask`
- `industry_id`
- `contact_name`
- `contact_mobile_ciphertext`
- `contact_mobile_mask`
- `bank_account_no_ciphertext`
- `bank_account_no_mask`
- `bank_mobile_ciphertext`
- `bank_mobile_mask`
- `bank_name`
- `deposit_bank_province`
- `deposit_bank_city`
- `deposit_bank_name`
- `card_user_name`
- `source_snapshot`
- timestamps

Create table `baofu_account_opening_flows`:

- `id`
- `owner_type`
- `owner_id`
- `account_type`
- `profile_id`
- `state`
- `verify_fee_amount`
- `verify_fee_payment_order_id`
- `open_trans_serial_no`
- `login_no`
- `account_binding_id`
- `merchant_report_id`
- `failure_code`
- `failure_message`
- `provider_request_snapshot`
- `raw_snapshot`
- timestamps

Canonical states:

- `profile_pending`
- `verify_fee_pending`
- `verify_fee_processing`
- `opening_processing`
- `merchant_report_processing`
- `applet_auth_pending`
- `ready`
- `failed`
- `voided`

Constraints:

- Unique profile per `(owner_type, owner_id)`.
- Unique active flow per `(owner_type, owner_id)` where `state IN ('profile_pending','verify_fee_pending','verify_fee_processing','opening_processing','merchant_report_processing','applet_auth_pending')`.
- Owner type check: `merchant`, `platform`, `rider`, `operator`.
- Account type check: `personal`, `business`.
- Flow `profile_id` references the profile row used to build the Baofoo request.
- Rider/operator flow must have verify-fee state before opening unless a paid verify-fee payment is attached.
- `open_trans_serial_no` is unique when present.
- `login_no` is required before entering `opening_processing`.
- There is no `qualification_upload_id`, `qualification_trans_serial_no`, `agreement_artifact_id`, `platform_no`, or `platform_terminal_id` column in the profile or flow table.

Validation:

- [ ] `PATH="/usr/local/go/bin:$PATH" make sqlc`
- [ ] DB tests for owner/account type constraints, encrypted-field persistence, active-flow uniqueness, ready/failed replacement and idempotent upsert.

### Task 4: Baofoo Account Open/Query Request Contract

Files:

- Modify: `locallife/baofu/account/contracts/official_open.go`
- Modify: `locallife/baofu/account/contracts/official_query.go`
- Modify: `locallife/baofu/account/contracts/types.go`
- Modify: `locallife/baofu/account/client.go`
- Modify or create focused tests under `locallife/baofu/account/contracts/` and `locallife/baofu/account/`

Steps:

- [ ] Keep official conditional fields such as `QualificationTransSerialNo`, `PlatformNo`, and `PlatformTerminalId` only if the DTO already models them or the official contract requires the struct fields for future compatibility.
- [ ] Mark these fields `omitempty` in JSON serialization.
- [ ] Do not populate `QualificationTransSerialNo`, `PlatformNo`, or `PlatformTerminalId` in current LocalLife request construction.
- [ ] Do not expose these fields in Mini Program APIs, backend public APIs, or ordinary app config.
- [ ] Add request serialization tests proving current-mode account open payload omits:
  - `qualificationTransSerialNo`
  - `platformNo`
  - `platformTerminalId`
- [ ] Add explicit `LoginNo` to the local `QueryAccountRequest` and stop mapping `OutRequestNo` into official query `loginNo`.
- [ ] Update official account query validation so non-agent query by `loginNo + certificateNo + certificateType` does not require `platformNo`.
- [ ] Add query serialization tests proving current-mode account query omits `platformNo`.
- [ ] Add a guard test proving the onboarding service can call Baofoo open account without a qualification upload row or agreement artifact.
- [ ] Ensure no new package/table/migration is created for `baofu/qualificationupload`, `baofu_qualification_uploads`, `baofu_agreement_templates`, or `baofu_agreement_artifacts`.

Validation:

- [ ] `PATH="/usr/local/go/bin:$PATH" go test ./baofu/account/... -count=1`

### Task 5: Account Opening Orchestration

Files:

- Create or expand: `locallife/logic/baofu_account_onboarding_service.go`
- Create: `locallife/logic/baofu_account_opening_profile_service.go`
- Modify: `locallife/logic/baofu_account_service.go`
- Modify: `locallife/baofu/account/client.go`
- Modify: `locallife/baofu/account/contracts/official_open.go`
- Modify: `locallife/api/baofu_callback.go`

Steps:

- [ ] Add `StartOrRecoverOpening(ctx, owner)` as the single orchestration entry.
- [ ] Resolve owner account type:
  - merchant -> business
  - platform -> business
  - rider -> personal
  - operator -> personal
- [ ] Resolve or create the `baofu_account_opening_profiles` row before creating/opening a flow.
- [ ] If required profile fields are missing, persist/return `profile_pending` with safe missing-field codes; do not call Baofoo.
- [ ] Build the Baofoo request only from the backend profile row and trusted owner data.
- [ ] Generate and persist `open_trans_serial_no` once per opening attempt.
- [ ] Generate stable `login_no` separately from `open_trans_serial_no` and use it in the official Baofoo request.
- [ ] For rider/operator:
  - if no owner-level paid verify-fee payment exists, create/reuse the owner-level direct payment order and return `verify_fee_pending` or `verify_fee_processing`.
  - if paid, link `verify_fee_payment_order_id` and continue to Baofoo account opening.
- [ ] For merchant/platform:
  - skip user-side verify-fee payment.
  - after account activation, always record one platform-borne `account_open_verify_fee` row in `baofu_fee_ledger`.
- [ ] For rider/operator, do not record a platform-borne `account_open_verify_fee` row.
- [ ] Do not call Baofoo qualification upload before opening.
- [ ] Do not require `QualificationTransSerialNo`.
- [ ] Do not pass `PlatformNo` or `PlatformTerminalId`.
- [ ] For business accounts, set `IndustryID=9931`.
- [ ] For all owner types, set official `needUploadFile=false`.
- [ ] Call Baofoo open account.
- [ ] If sync result is `state=2`, persist `opening_processing` and return without blocking.
- [ ] If sync result is success, activate binding immediately and normalize `sharingMerId=contractNo`.
- [ ] If active result lacks `sharingMerId`, use `contractNo` as `sharingMerId`.
- [ ] If failed/abnormal, persist explicit failure state.
- [ ] State updates must follow the transition table in section 5.1.4 with conditional SQL updates.

Validation:

- [ ] Unit tests for each owner type.
- [ ] Unit tests for `sharingMerId == contractNo`.
- [ ] Unit tests that rider/operator cannot open before paid verify-fee payment.
- [ ] Unit tests that merchant/platform can open without user-side verify-fee payment.
- [ ] Unit tests that merchant/platform write platform-borne fee ledger rows and rider/operator do not.
- [ ] Unit tests that missing profiles return `profile_pending` and never call Baofoo.
- [ ] Unit tests that no qualification upload dependency blocks opening.

### Task 6: Verify Fee Payment Callback Propagation

Files:

- Modify: `locallife/api/payment_callback.go`
- Modify: `locallife/logic/payment_fact_application_service.go`
- Modify: `locallife/db/query/payment_order.sql`
- Create focused tests in `locallife/api/payment_callback_test.go` and/or `locallife/logic`

Steps:

- [ ] Recognize `payment_orders.business_type='baofu_account_verify_fee'`.
- [ ] On direct payment success, mark payment paid through existing payment path.
- [ ] Apply account-opening side effect once: find the active rider/operator flow by owner-level attach, link paid payment to `verify_fee_payment_order_id`, move flow to `opening_processing`, enqueue or call opening continuation.
- [ ] On payment closed/failed, leave flow at `verify_fee_pending` with retry guidance.
- [ ] Ensure duplicate callbacks do not trigger duplicate opening attempts by using conditional flow updates and checking `open_trans_serial_no`.
- [ ] Ensure a paid owner-level verify-fee payment can be reused by a replacement flow after Baofoo opening failure.

Validation:

- [ ] Duplicate callback test.
- [ ] Payment success then opening continuation test.
- [ ] Payment closed/failed does not open account.
- [ ] Replacement-flow reuse test: same owner/purpose paid payment opens without charging again.

### Task 7: Account Opening Recovery Scheduler

Files:

- Create: `locallife/worker/baofu_account_opening_recovery_scheduler.go`
- Modify: `locallife/main.go`
- Modify: `locallife/worker/processor.go` if async tasks are introduced

Steps:

- [ ] Scan `opening_processing` and abnormal/retryable flows older than a configured minimum age.
- [ ] Call Baofoo account query. Prefer `contractNo` when known; otherwise use persisted `loginNo` plus certificate identity. Do not send `platformNo`.
- [ ] Apply terminal states to `baofu_account_bindings` and `baofu_account_opening_flows`.
- [ ] After merchant binding active, trigger merchant report.
- [ ] Leave platform/rider/operator report/auth states as not required.
- [ ] Do not implement a qualification upload recovery scheduler in the current flow.
- [ ] Alert on unmatched Baofoo callbacks/query contradictions; do not create bindings without a matching local flow.

Validation:

- [ ] Scheduler unit tests for terminal success/failure/processing.
- [ ] No-op tests when client/config is missing.

### Task 8: Merchant Report And Applet Auth

Files:

- Modify: `locallife/logic/baofu_merchant_report_service.go`
- Modify: `locallife/db/query/baofu_merchant_report.sql`
- Modify: `locallife/logic/baofu_account_onboarding_service.go`
- Existing scheduler: `locallife/worker/baofu_merchant_report_recovery_scheduler.go`

Steps:

- [ ] Keep `baofu_merchant_reports.owner_type` restricted to `merchant`.
- [ ] After merchant account active, submit `merchant_report(WECHAT)`.
- [ ] Use merchant `sharing_mer_id` as `bctMerId`.
- [ ] On report success, persist `subMchID`.
- [ ] Call `BindSubConfig` with `authType=APPLET` and LocalLife Mini Program appid.
- [ ] Mark merchant flow `ready` only after account active, report succeeded, and applet auth succeeded.
- [ ] For platform/rider/operator, skip this task and mark readiness based only on account active.

Validation:

- [ ] Merchant active account triggers report/auth.
- [ ] Platform/rider/operator never create merchant report rows.
- [ ] Existing bind auth tests remain passing.

### Task 9: Backend API

Files:

- Create: `locallife/api/baofu_account_onboarding.go`
- Modify: `locallife/api/server.go`
- Modify: `locallife/api/merchant_baofu_readiness.go`
- Modify: `locallife/api/operator_application.go` if operator application needs to surface settlement status
- Modify Swagger annotations, then regenerate if API annotations change

Routes:

- `GET /v1/merchant/settlement-account`
- `POST /v1/merchant/settlement-account`
- `GET /v1/rider/settlement-account`
- `POST /v1/rider/settlement-account`
- `GET /v1/operators/me/settlement-account`
- `POST /v1/operators/me/settlement-account`
- `GET /v1/platform/finance/settlement-account`
- `POST /v1/platform/finance/settlement-account`

`POST` semantics:

- If body is empty, start or resume the current authenticated owner flow.
- If body contains `profile`, validate and persist the role-specific Baofoo settlement profile first, then start/resume.
- Do not accept `owner_type`, `owner_id`, `account_type`, `industry_id`, `qualificationTransSerialNo`, `platformNo`, or `platformTerminalId` from self-service clients.
- Platform profile submission is admin-only under `/v1/platform/finance/settlement-account`.

Role-specific profile request fields:

| Route | Allowed profile fields |
| --- | --- |
| merchant | `email`, `bank_account_no`, `bank_name`, `deposit_bank_province`, `deposit_bank_city`, `deposit_bank_name`, `contact_name`, `contact_mobile` |
| rider | `bank_account_no`, `bank_mobile`, `card_user_name` |
| operator | `bank_account_no`, `bank_mobile`, `card_user_name` |
| platform | `legal_name`, `business_license_number`, `legal_person_name`, `legal_person_id_number`, `email`, `bank_account_no`, `bank_name`, `deposit_bank_province`, `deposit_bank_city`, `deposit_bank_name`, `contact_name`, `contact_mobile` |

Response shape:

```json
{
  "owner_type": "rider",
  "owner_id": 123,
  "account_type": "personal",
  "state": "verify_fee_pending",
  "state_label": "待支付开户核验费",
  "payment_ready": false,
  "profile": {
    "status": "complete",
    "missing_fields": []
  },
  "verify_fee": {
    "required": true,
    "amount": 200,
    "payment_order_id": 456,
    "status": "pending",
    "pay_params": {}
  },
  "account": {
    "open_state": "processing",
    "contract_no": "",
    "sharing_mer_id": ""
  },
  "merchant_report": {
    "required": false,
    "report_state": "not_required",
    "applet_auth_state": "not_required"
  }
}
```

Rules:

- Handler only resolves authenticated subject and delegates to logic.
- Do not accept arbitrary `owner_type` from Mini Program.
- Merchant route must be owner-only, not merchant staff manager/cashier.
- Rider and operator routes must use their role middleware and the current user binding.
- Platform route must be admin-only and use `owner_id=0`.
- Do not expose raw provider errors to users.
- Do not expose `qualificationTransSerialNo`, `platformNo`, or `platformTerminalId` in public response/request contracts.
- Do not return plaintext sensitive profile fields; return only status, missing field codes and masked display values if needed.
- Use stable Chinese product copy for `state_label`.

Validation:

- [ ] Handler tests for each role.
- [ ] Unauthorized/forbidden owner access tests.
- [ ] Tests proving arbitrary client owner fields are ignored/rejected.
- [ ] `PATH="/usr/local/go/bin:$PATH" make swagger` if Swagger annotations changed.

## 7. Mini Program Implementation Plan

### Task 10: API Contracts

Files:

- Create: `weapp/miniprogram/api/baofu-account.ts`
- Modify: `weapp/miniprogram/api/payment.ts`

Steps:

- [ ] Add settlement account DTOs matching backend response.
- [ ] Add role-specific start/query methods.
- [ ] Extend `BusinessType` with `baofu_account_verify_fee`.
- [ ] Keep provider details out of page code.
- [ ] Do not model `qualificationTransSerialNo`, `platformNo`, or `platformTerminalId` in Mini Program types.

Validation:

- [ ] TypeScript compile.

### Task 11: Shared Payment Progress Orchestrator

Files:

- Modify: `weapp/miniprogram/services/payment-workflow.ts`
- Create: `weapp/miniprogram/services/payment-progress-toast.ts` or equivalent service-level helper
- Refactor: `weapp/miniprogram/services/rider-deposit-payment.ts`
- Modify only the Baofoo verify-fee and rider-deposit call sites in this rollout unless another touched page already uses the same helper naturally.

Steps:

- [ ] Provide one helper that shows long-duration TDesign loading toast while payment is being invoked and queried.
- [ ] After `wx.requestPayment`, always call backend query/polling to decide terminal result.
- [ ] Treat user cancel as non-terminal until backend query confirms `closed`/`failed` or still `pending`; keep pending context recoverable.
- [ ] Apply the helper now to:
  - Baofoo account verify fee
  - rider deposit
- [ ] Do not refactor order payment, reservation payment or claim recovery in this Baofoo rollout unless those files are already touched for another required reason.
- [ ] Add a short follow-up note in the service comment or backlog section that existing order/reservation/claim payment workflows should migrate to the helper later.
- [ ] Store pending payment context so page re-entry can recover.

Validation:

- [ ] Baofoo verify-fee and rider-deposit pages still compile.
- [ ] Weak-network/re-entry behavior for Baofoo verify fee and rider deposit reviewed manually in DevTools.

### Task 12: Account Opening Workflow Service

Files:

- Create: `weapp/miniprogram/services/baofu-account-onboarding.ts`
- Modify role pages that surface settlement readiness

Steps:

- [ ] `startOrResumeMerchantSettlementAccount()`
- [ ] `startOrResumeRiderSettlementAccount()`
- [ ] `startOrResumeOperatorSettlementAccount()`
- [ ] Platform settlement-account calls are admin-console/backend routes, not normal Mini Program user flows. Do not add a platform Mini Program self-service entry unless a platform admin Mini Program surface exists and is protected by admin auth.
- [ ] If backend returns verify-fee `pay_params`, call shared payment progress orchestrator.
- [ ] After verify-fee payment reaches paid, call backend start/resume again.
- [ ] Poll settlement account status until:
  - `ready`
  - `failed`
  - long-running processing timeout, with recoverable status shown
- [ ] Persist pending onboarding context by owner role so re-entry resumes query rather than restarting blindly.

Validation:

- [ ] Unit-level service tests if the project has test harness for services.
- [ ] Manual role flow walkthrough in Mini Program DevTools.

### Task 13: Role Surfaces

Files to inspect before implementation:

- Rider settlement/deposit pages under `weapp/miniprogram/pages/rider/**`
- Operator application/workbench pages under `weapp/miniprogram/pages/operator/**`
- Merchant settings/applyment pages under `weapp/miniprogram/pages/merchant/**`
- No normal Mini Program platform page is in scope. Platform settlement account is admin-only backend/admin-console work.

Page rules:

- Use TDesign components first.
- Do not leak raw provider fields such as `qualificationTransSerialNo`, `platformNo`, or `platformTerminalId` into product copy.
- Merchant copy may mention“微信支付展示商户名称配置中”。
- Rider/operator copy should make the 2 元核验费 clear before invoking payment.
- Platform/merchant should not show user-side verify-fee payment.

Validation:

- [ ] `PATH="$HOME/.local/bin:$PATH" npm run compile`
- [ ] `PATH="$HOME/.local/bin:$PATH" npm run lint`
- [ ] `PATH="$HOME/.local/bin:$PATH" npm run quality:check`

## 8. End-To-End Flow Details

### 8.1 Merchant

```text
POST merchant settlement-account
  -> create/recover flow
  -> if profile incomplete: profile_pending
  -> open Baofoo business account with industryId=9931
  -> wait callback/query if processing
  -> active binding: contractNo, sharingMerId=contractNo
  -> record platform-borne account_open_verify_fee ledger
  -> submit WeChat merchant report
  -> query/recover report
  -> bind_sub_config(APPLET, LocalLife appid)
  -> ready
```

### 8.2 Platform

```text
POST platform settlement-account
  -> create/recover flow
  -> if profile incomplete: profile_pending
  -> open Baofoo business account with industryId=9931
  -> wait callback/query if processing
  -> active binding: contractNo, sharingMerId=contractNo
  -> record platform-borne account_open_verify_fee ledger
  -> no merchant report
  -> no applet auth
  -> ready
```

### 8.3 Rider

```text
POST rider settlement-account
  -> create/recover flow
  -> if profile incomplete: profile_pending
  -> create/reuse direct WeChat verify-fee payment for 200 fen
  -> Mini Program invokes payment with long TDesign progress toast
  -> Mini Program/backend query payment terminal result
  -> paid
  -> open Baofoo personal account
  -> wait callback/query if processing
  -> active binding: contractNo, sharingMerId=contractNo
  -> no merchant report
  -> no applet auth
  -> ready
```

### 8.4 Operator

```text
POST operators/me settlement-account
  -> create/recover flow
  -> if profile incomplete: profile_pending
  -> create/reuse direct WeChat verify-fee payment for 200 fen
  -> Mini Program invokes payment with long TDesign progress toast
  -> Mini Program/backend query payment terminal result
  -> paid
  -> open Baofoo personal account
  -> wait callback/query if processing
  -> active binding: contractNo, sharingMerId=contractNo
  -> no merchant report
  -> no applet auth
  -> ready
```

## 9. Idempotency And Recovery Requirements

- Repeated start calls must return the current flow, not create duplicate flows.
- Repeated verify-fee payment taps must reuse an unexpired pending payment when owner, purpose, authenticated payer and amount match.
- A paid verify-fee payment must not be charged again for the same owner and `purpose=initial_open`.
- Rider/operator retries after Baofoo opening failure must reuse the same paid verify-fee proof for the same owner and onboarding intent. Creating a replacement flow must link the paid `verify_fee_payment_order_id`; it must not require a second user-side verify-fee payment unless an admin explicitly voids the original proof for fraud, wrong owner identity, or a legally invalid application.
- Duplicate WeChat callbacks must not trigger duplicate Baofoo opening.
- Duplicate Baofoo account callbacks must not create duplicate fee ledger rows.
- Baofoo account callbacks must match by `open_trans_serial_no`; `contractNo` fallback is allowed only for an existing unique binding cross-check.
- Baofoo sync processing states must be recovered by scheduler.
- Merchant report `processing` and applet auth `pending` continue to use the existing recovery model.
- If Mini Program exits after payment, re-entry must query backend status and continue from the persisted flow.

## 10. Validation Matrix

Backend:

- [ ] `PATH="/usr/local/go/bin:$PATH" make sqlc`
- [ ] `PATH="/usr/local/go/bin:$PATH" make check-generated`
- [ ] `PATH="/usr/local/go/bin:$PATH" go test ./baofu/account/... -count=1`
- [ ] `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'Test.*Baofu.*Account|Test.*Opening|Test.*MerchantReport' -count=1`
- [ ] `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'Test.*Baofu|Test.*SettlementAccount|TestHandlePaymentNotify.*VerifyFee' -count=1`
- [ ] `PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'Test.*Baofu.*Opening|Test.*MerchantReport' -count=1`

Mini Program:

- [ ] `PATH="$HOME/.local/bin:$PATH" npm run compile`
- [ ] `PATH="$HOME/.local/bin:$PATH" npm run lint`
- [ ] `PATH="$HOME/.local/bin:$PATH" npm run quality:check`

Manual/evidence:

- [ ] Merchant sandbox or provider-confirmed test: open business account without qualification upload -> merchant report -> APPLET auth.
- [ ] Rider sandbox or provider-confirmed test: verify-fee payment paid -> open personal account without qualification upload.
- [ ] Operator sandbox or provider-confirmed test: verify-fee payment paid -> open personal account without qualification upload.
- [ ] Platform sandbox or provider-confirmed test: open business account without qualification upload, no merchant report rows.

## 11. Resolved Designs And Residual Risks

- `qualificationTransSerialNo` is not required in LocalLife current flow. The implementation must not require it, must not send it, and must not call the qualification upload interface.
- `platformNo` and `platformTerminalId` are not required in LocalLife current flow. They are agent-mode conditional fields and must not be configured or sent unless a future confirmed agent-mode rollout changes the contract.
- No Baofoo qualification file upload is implemented for current opening. There is no provider ZIP, no `fileNameMap`, no `401` platform agreement upload, and no Baofoo upload recovery scheduler.
- `baofu_account_opening_profiles` is the chosen backend-owned profile source for Baofoo request construction. Missing fields are solved by role-specific profile completion APIs, not by Mini Program passing provider DTOs directly.
- `payment_orders` is the chosen payment ledger for rider/operator verify fee. No extra user-payment table is needed. The opening flow row references the verify-fee `payment_order_id`, and a partial unique index on `(business_type, attach)` prevents duplicate active payments per owner/purpose.
- Verify-fee payment is isolated from existing payment domains by `business_type='baofu_account_verify_fee'`, `payment_channel='direct'`, `requires_profit_sharing=false`, canonical `attach`, and a dedicated callback/application consumer.
- Merchant/platform Baofoo verify fee is always recorded as a platform-borne `baofu_fee_ledger` row on activation. Rider/operator user-side verify fee is recorded by `payment_orders`, not by a platform-borne fee ledger row.
- Baofoo开户可能几秒完成，也可能分钟级完成；系统按异步长流程实现。小程序可以展示长时间 toast/进度，但终态只信后端查询。
- 商户报备/授权只对商户执行，这是业务不变量。后续评审要特别防止为了“统一流程”把平台、骑手、运营商也送进 merchant report。
- Residual release evidence: still need one sandbox or Baofoo-supported verification that current direct opening succeeds without `qualificationTransSerialNo`, `platformNo`, `platformTerminalId`, and qualification upload for each owner/account type.

## 12. Suggested Execution Order

1. 配置、常量、迁移、sqlc。
2. profile/flow 表、敏感字段加密与 sqlc。
3. 核验费支付业务类型和回调推进。
4. 宝付开户/查询 DTO request builder 清理和 omission tests。
5. 宝付开户编排和回调/查询恢复。
6. 商户报备/授权接入 flow。
7. 后端角色 API。
8. 小程序 API/service。
9. 小程序 TDesign 支付进度统一改造。
10. 各角色页面接入与全链路验证。
