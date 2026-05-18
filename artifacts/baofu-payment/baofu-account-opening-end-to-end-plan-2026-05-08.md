# 宝付二级户开户端到端实施计划

Updated: 2026-05-09
Risk class: G3 - payment/funds/account opening/callback/async recovery.

## 1. Goal

完成 LocalLife 平台、商户、骑手、运营商的宝付宝财通 2.0 二级户开户流程，并把商户开户后的微信聚合商户报备和小程序绑定授权目录接入到可恢复、可查询、可审计的端到端状态机。

本计划覆盖后端、数据库、宝付开户接口、微信直连核验费支付、小程序支付进度体验和恢复查询。实施时不得阻断可恢复的异步流程：支付、宝付开户、商户报备、授权目录绑定都必须以本地持久化状态和查询/回调终态为准。

## 1.1 Current Branch Status Snapshot

Last synced with working tree: 2026-05-09 document/task-card audit.

This section is the current implementation ledger. Checkboxes below reflect the dirty working tree as inspected during the 2026-05-08/2026-05-09 task-card audits, not production release readiness. If a later code change lands without updating this section and the task cards, treat the document as stale.

Implemented or partially implemented in the current branch:

- Backend config now has `BAOFU_ACCOUNT_VERIFY_FEE_FEN=200`, `BAOFU_BUSINESS_INDUSTRY_ID=9931`, and dedicated merchant-report config `BAOFU_MERCHANT_REPORT_CHANNEL_ID`, `BAOFU_MERCHANT_REPORT_CHANNEL_NAME`, `BAOFU_MERCHANT_REPORT_BUSINESS`.
- `758-2` is kept only as the Baofoo WeChat merchant-report `reportInfo.business` category/default; it is not used as Baofoo account-opening `industryId`.
- Baofoo account-opening `industryId` defaults to `9931` for business accounts.
- SQL migration/query/sqlc/mock artifacts for `baofu_account_opening_profiles`, `baofu_account_opening_flows`, and `baofu_account_verify_fee` payment queries exist in the working tree.
- Backend orchestration, callback continuation, account-opening recovery scheduler, merchant-report continuation, and role settlement-account routes exist in code.
- Merchant self-service settlement-account route uses owner-only access for both GET and POST.
- Platform settlement-account route is admin-only under `/v1/platform/finance/settlement-account`; it is not a Mini Program self-service flow.
- Mini Program API/service/rider settlement page exist in code.

Corrections already applied since the earlier draft:

- Merchant settlement-account GET and POST/start-resume readiness now gate `ready/payment_ready=true` on account active + merchant report success + APPLET auth success. Focused tests were added for the previous early-ready bug.
- Mini Program Baofoo verify-fee flow now POSTs start/resume after a paid payment before polling Baofoo opening status.
- Mini Program Baofoo verify-fee cancellation now re-queries backend settlement-account state instead of treating local cancel as terminal.

Still-open gaps in the current branch:

- Full backend validation, Mini Program manual DevTools walkthrough, and Baofoo sandbox/provider evidence remain pending unless separately recorded below.
- `make sqlc`, `make mock`, `make swagger`, `make check-generated`, and `make check-baofu-contract` have been confirmed for the current branch via the validation ledger below.
- Payment ledger/listing now maps `baofu_account_verify_fee` to “开户核验费” in the Mini Program wallet ledger. Backend ledger still returns raw `business_type` as designed; the Mini Program maps the product title.
- Verify-fee closed/failed/expired callback handling, duplicate full API callback path coverage, and replacement-flow paid-payment reuse are now covered by focused backend tests.
- Operator-specific onboarding service coverage is now present; operator/platform settlement-account handler coverage is now present in the API test suite.
- Account-opening recovery scheduler now has focused coverage for terminal failure, provider still-processing, missing dependency/config no-op logging, unmatched callback alerting, callback/query out-request-no mismatch blocking, and Baofoo query contract-owner contradiction blocking.
- Mini Program Baofoo onboarding persists owner-role pending context and re-entry resumes by backend query/polling rather than blind restart.
- Merchant Baofoo settlement-account surface is wired into merchant finance/config entries. Existing merchant WeChat/ecommerce settlement-account modification pages remain separate surfaces.
- Operator Baofoo settlement-account page/surface is wired from the operator finance withdraw page.
- Current working tree also touched claim-recovery and combined-payment workflow code through the shared payment helper; Task 11 records this as intentional scope and the Mini Program quality gates have been run for the touched rollout.
- Error logging and frontend guidance have Task 10-13 review entries below. New task-card closure still requires the same review/fix/re-review discipline.

Validation already run during the 2026-05-08 sync:

- `PATH="/usr/local/go/bin:$PATH" go test ./util -run TestLoadConfig -count=1`
- `PATH="/usr/local/go/bin:$PATH" go test ./baofu/account/... -count=1`
- `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'Test.*Baofu.*Account|Test.*Opening|Test.*MerchantReport|TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFee' -count=1`
- `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'Test.*Baofu|Test.*SettlementAccount|TestHandlePaymentNotify.*VerifyFee' -count=1`
- `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestBaofuSettlementAccount' -count=1`
- `PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'Test.*Baofu.*Opening|Test.*MerchantReport' -count=1`

Validation already run during the 2026-05-09 Mini Program Task 13 closeout and final re-check:

- `PATH="$HOME/.local/bin:$PATH" npm run compile`
- `PATH="$HOME/.local/bin:$PATH" npm run lint`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:weapp`
- `PATH="$HOME/.local/bin:$PATH" npm run quality:check`

Validation already run during the 2026-05-09 backend generated/contract closeout:

- `PATH="/usr/local/go/bin:$PATH" make sqlc`
- `PATH="/usr/local/go/bin:$PATH" make mock`
- `PATH="/usr/local/go/bin:$PATH" make swagger`
- `PATH="/usr/local/go/bin:$PATH" make check-generated`
- `PATH="/usr/local/go/bin:$PATH" make check-baofu-contract`
- `PATH="/usr/local/go/bin:$PATH" go test ./util ./logic -run 'Test.*Baofu|Test.*Config' -count=1`
- `PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -run 'TestBaofuAccountOpening' -count=1`
- `PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -count=1`
- `PATH="/usr/local/go/bin:$PATH" go test ./baofu/account/... ./logic ./api ./worker -run 'Test.*Baofu|Test.*SettlementAccount|TestHandlePaymentNotify.*VerifyFee|Test.*MerchantReport' -count=1`

## 1.2 Task Card Closeout Rules

This plan must be executed as task cards, not as a loose checklist.

For every task card:

- Write or perform a focused review before marking any new checkbox complete.
- If the review finds a defect, fix the defect in scope, then review the same item again.
- Mark a checkbox only after the implementation, required generated artifacts, focused validation, and re-review evidence all support it.
- Do not accept downgrade handling. Do not replace a required backend state transition, callback branch, scheduler branch, frontend recovery path, or generated-code step with a note saying it can be handled manually later.
- If validation cannot run, or only ran partially, the validation checkbox stays open and the residual risk must be named in this document.
- If a change touches shared payment helpers outside Baofoo verify fee and rider deposit, either validate those call sites as intentional scope or revert those call-site edits before release. Do not leave them in an ambiguous state.

Review output for each task must be findings-first:

- If issues are found, record the finding, fix, and re-review result in section 10.2.
- If no issues are found, record the reviewed scope and remaining unverified paths.
- Do not use "looks OK" as completion evidence; cite code paths, tests, generated commands, sandbox/provider evidence, or manual DevTools walkthroughs.

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

#### 5.1.6.1 Error Logging And Frontend Guidance

No error in this rollout may disappear silently.

Backend logging rules:

- Unexpected infrastructure/provider/storage/encryption errors must be logged once at the boundary that has owner/payment/flow context. Use structured fields such as `owner_type`, `owner_id`, `flow_id`, `payment_order_id`, `out_trade_no`, `open_trans_serial_no`, `report_id`, and scheduler/callback name.
- User-correctable business errors must return stable Chinese guidance through `label` and/or `status_desc`, without raw Baofoo/WeChat/internal SQL text.
- Callback and scheduler failures must log enough context for operations to retry or investigate. A no-op is allowed only when it is an expected state, and it must be covered by a test or documented branch.
- Duplicate, unmatched, contradictory, and stale callback/query states must be warn/error logged and must not mutate unrelated owner rows.
- Sensitive fields must stay masked or omitted in logs and API responses: certificate numbers, bank accounts, reserved mobile, email, `contractNo`, `sharingMerId`, `subMchID`, raw provider payloads, signatures and secrets.

Frontend guidance rules:

- The Mini Program must map every backend state to an actionable Chinese state: complete profile, pay verify fee, payment confirming, account opening, merchant report/auth configuring, ready, failed/retry.
- Payment cancel/timeout/weak-network states must tell users that the backend will continue checking payment/opening status and provide a refresh/retry action.
- `profile_pending` and `failed` must show concrete next action text. If backend can return missing-field codes in a later refinement, the frontend should render field-level guidance; until then it must avoid provider jargon.
- `verify_fee_pending` must show the configured fee amount in yuan and explain that rider/operator pay this fee before Baofoo account opening starts.
- `merchant_report_processing` and `applet_auth_pending` must be product copy about WeChat payment display-name configuration, not Baofoo implementation terms.
- Frontend services should log caught unexpected exceptions with enough local context (`role`, action name, payment id when present) while still rendering safe Chinese messages to users.

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

- [x] Add `BAOFU_ACCOUNT_VERIFY_FEE_FEN`, default `200`.
- [x] Add `BAOFU_BUSINESS_INDUSTRY_ID`, default `9931`.
- [x] Add dedicated WeChat merchant-report config: `BAOFU_MERCHANT_REPORT_CHANNEL_ID`, `BAOFU_MERCHANT_REPORT_CHANNEL_NAME`, `BAOFU_MERCHANT_REPORT_BUSINESS`.
- [x] Keep `BAOFU_MERCHANT_REPORT_BUSINESS=758-2` scoped to WeChat merchant report `reportInfo.business`; never feed it to account-opening `industryId`.
- [x] Do not add Baofoo qualification upload URL/config for current opening flow.
- [x] Do not add `BAOFU_PLATFORM_NO` or `BAOFU_PLATFORM_TERMINAL_ID` as required config for current opening flow.
- [x] Remove remaining hard-coded `BaofuAccountOpenVerifyFeeFen = 100` behavior.
- [x] Ensure platform/merchant business account requests default `IndustryID` to configured `9931` unless explicitly overridden by trusted backend configuration.
- [x] Keep `BaofuAccountTypePersonal` and `BaofuAccountTypeBusiness` as the only account type constants.
- [x] Ensure any previous draft code/migration referring to `BaofuAccountTypePlatform`, qualification upload, agreement artifacts, `qualification_trans_serial_no`, `platform_no`, or `platform_terminal_id` is removed or left unused only as official optional DTO fields.

Validation:

- [x] `PATH="/usr/local/go/bin:$PATH" go test ./util -run TestLoadConfig -count=1`
- [x] `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'Test.*Baofu.*Account|Test.*MerchantReport' -count=1`
- [x] Full `PATH="/usr/local/go/bin:$PATH" go test ./util ./logic -run 'Test.*Baofu|Test.*Config' -count=1`

### Task 2: Payment Business Type For Account Verify Fee

Files:

- Create migration under `locallife/db/migration/`
- Modify: `locallife/db/query/payment_order.sql`
- Modify: `locallife/db/sqlc/constants.go`
- Modify: `locallife/api/payment_order.go` only if response enum/contracts need expansion
- Modify: `weapp/miniprogram/api/payment.ts`

Steps:

- [x] Add `payment_orders.business_type='baofu_account_verify_fee'` by dropping and recreating `payment_orders_business_type_check` with the new value included.
- [x] Add constant `PaymentBusinessTypeBaofuAccountVerifyFee = "baofu_account_verify_fee"` in the backend SSOT constants and use it everywhere instead of magic strings.
- [x] Store canonical owner-level `attach` in the existing key-value style used by payment code: `business:baofu_account_verify_fee;owner_type:<owner_type>;owner_id:<owner_id>;purpose:initial_open`.
- [x] Add a partial unique index so one owner onboarding intent cannot have multiple active verify-fee payments:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS payment_orders_baofu_verify_fee_active_uidx
    ON payment_orders (business_type, attach)
    WHERE business_type = 'baofu_account_verify_fee'
      AND status IN ('pending', 'paid');
```

- [x] Add `GetBaofuVerifyFeePaymentByAttach` and `GetReusableBaofuVerifyFeePayment` queries. Reuse `pending` payments that are unexpired and exactly match owner, purpose, authenticated payer and amount; reuse `paid` payments as the fee proof for the same owner/purpose even when a failed flow is replaced.
- [x] Link the selected payment to the active flow through `baofu_account_opening_flows.verify_fee_payment_order_id`. Do not make `flow_id` part of the payment idempotency key.
- [x] Create/reuse verify-fee payment only inside the Baofoo onboarding service after resolving the authenticated rider/operator. Do not expose a generic client-controlled “create baofu verify fee” payment endpoint where the Mini Program can forge owner, amount, purpose or flow.
- [x] Ensure verify-fee payment uses `payment_type='miniprogram'`, `payment_channel='direct'`, `requires_profit_sharing=false`, `order_id IS NULL`, `reservation_id IS NULL`, and `user_id` equal to the authenticated payer.
- [x] Ensure query payment endpoint can query this payment type with existing direct payment query path, or return the payment status through the settlement-account query response.
- [x] Ensure payment ledger/listing renders this business type as “开户核验费” and does not classify it as order payment, rider deposit, membership recharge, or claim recovery. Backend ledger returns raw `business_type`; Mini Program wallet title mapping now renders `baofu_account_verify_fee` as “开户核验费” with refund fallback “开户核验费退款”.
- [x] Ensure direct payment callback routes this business type to the Baofoo onboarding consumer and does not trigger rider deposit side effects.
- [x] Extend Mini Program `BusinessType` union with `baofu_account_verify_fee`.

Validation:

- [x] `PATH="/usr/local/go/bin:$PATH" make sqlc`
- [x] `PATH="/usr/local/go/bin:$PATH" make check-generated`
- [x] Focused API tests for create/query verify-fee payment: `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'Test.*Baofu.*Settlement|Test.*Baofu' -count=1`.

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

- [x] `PATH="/usr/local/go/bin:$PATH" make sqlc`
- [x] `PATH="/usr/local/go/bin:$PATH" make mock` because `db.Store`/mock methods changed for the new sqlc-backed flow/profile/payment queries.
- [x] sqlc artifacts and mocks exist in current working tree.
- [x] DB tests for owner/account type constraints, encrypted-field persistence, active-flow uniqueness, ready/failed replacement and idempotent upsert.

### Task 4: Baofoo Account Open/Query Request Contract

Files:

- Modify: `locallife/baofu/account/contracts/official_open.go`
- Modify: `locallife/baofu/account/contracts/official_query.go`
- Modify: `locallife/baofu/account/contracts/types.go`
- Modify: `locallife/baofu/account/client.go`
- Modify or create focused tests under `locallife/baofu/account/contracts/` and `locallife/baofu/account/`

Steps:

- [x] Keep official conditional fields such as `QualificationTransSerialNo`, `PlatformNo`, and `PlatformTerminalId` only if the DTO already models them or the official contract requires the struct fields for future compatibility.
- [x] Mark these fields `omitempty` in JSON serialization.
- [x] Do not populate `QualificationTransSerialNo`, `PlatformNo`, or `PlatformTerminalId` in current LocalLife request construction.
- [x] Do not expose these fields in Mini Program APIs, backend public APIs, or ordinary app config.
- [x] Add request serialization tests proving current-mode account open payload omits:
  - `qualificationTransSerialNo`
  - `platformNo`
  - `platformTerminalId`
- [x] Add explicit `LoginNo` to the local `QueryAccountRequest` and stop mapping `OutRequestNo` into official query `loginNo`.
- [x] Update official account query validation so non-agent query by `loginNo + certificateNo + certificateType` does not require `platformNo`.
- [x] Add query serialization tests proving current-mode account query omits `platformNo`.
- [x] Add a guard test proving the onboarding service can call Baofoo open account without a qualification upload row or agreement artifact.
- [x] Ensure no new package/table/migration is created for `baofu/qualificationupload`, `baofu_qualification_uploads`, `baofu_agreement_templates`, or `baofu_agreement_artifacts`.

Validation:

- [x] `PATH="/usr/local/go/bin:$PATH" go test ./baofu/account/... -count=1`
- [x] `PATH="/usr/local/go/bin:$PATH" make check-baofu-contract`

### Task 5: Account Opening Orchestration

Files:

- Create or expand: `locallife/logic/baofu_account_onboarding_service.go`
- Create: `locallife/logic/baofu_account_opening_profile_service.go`
- Modify: `locallife/logic/baofu_account_service.go`
- Modify: `locallife/baofu/account/client.go`
- Modify: `locallife/baofu/account/contracts/official_open.go`
- Modify: `locallife/api/baofu_callback.go`

Steps:

- [x] Add `StartOrRecoverOpening(ctx, owner)` as the single orchestration entry.
- [x] Resolve owner account type:
  - merchant -> business
  - platform -> business
  - rider -> personal
  - operator -> personal
- [x] Resolve or create the `baofu_account_opening_profiles` row before creating/opening a flow.
- [x] If required profile fields are missing, persist/return `profile_pending` and do not call Baofoo.
- [x] Return safe missing-field codes or equivalent frontend-actionable profile guidance for `profile_pending`.
- [x] Build the Baofoo request only from the backend profile row and trusted owner data.
- [x] Generate and persist `open_trans_serial_no` once per opening attempt.
- [x] Generate stable `login_no` separately from `open_trans_serial_no` and use it in the official Baofoo request.
- [x] Implement the shared user-side verify-fee branch for rider/operator:
  - if no owner-level paid verify-fee payment exists, create/reuse the owner-level direct payment order and return `verify_fee_pending` or `verify_fee_processing`.
  - if paid, link `verify_fee_payment_order_id` and continue to Baofoo account opening.
- [x] For merchant/platform:
  - skip user-side verify-fee payment.
  - after account activation, always record one platform-borne `account_open_verify_fee` row in `baofu_fee_ledger`.
- [x] Implement non-platform-borne fee-ledger behavior for non-business-receiver user-side fee owners. Rider proof exists; operator proof remains a validation item below.
- [x] Do not call Baofoo qualification upload before opening.
- [x] Do not require `QualificationTransSerialNo`.
- [x] Do not pass `PlatformNo` or `PlatformTerminalId`.
- [x] For business accounts, set `IndustryID=9931`.
- [x] For all owner types, set official `needUploadFile=false`.
- [x] Call Baofoo open account.
- [x] If sync result is `state=2`, persist `opening_processing` and return without blocking.
- [x] If sync result is success, activate binding immediately and normalize `sharingMerId=contractNo`.
- [x] If active result lacks `sharingMerId`, use `contractNo` as `sharingMerId`.
- [x] If failed/abnormal, persist explicit failure state.
- [x] Implement service-layer state transitions from section 5.1.4: merchant success moves to report/auth states; non-merchant success moves to `ready`. Operator-specific proof remains a validation item below.
- [x] Merchant active-binding shortcut in `StartOrRecoverOpening` no longer returns `ready` until report success and APPLET auth success are present.

Validation:

- [x] Unit tests for each owner type. Merchant/platform/rider paths are covered; operator-specific onboarding path now has explicit coverage.
- [x] Unit tests for `sharingMerId == contractNo`.
- [x] Unit test that rider cannot open before paid verify-fee payment.
- [x] Unit test that operator cannot open before paid verify-fee payment.
- [x] Unit tests that merchant/platform can open without user-side verify-fee payment.
- [x] Unit tests that merchant/platform write platform-borne fee ledger rows.
- [x] Unit test that rider does not write a platform-borne fee ledger row.
- [x] Unit test that operator does not write a platform-borne fee ledger row.
- [x] Unit tests that missing profiles return `profile_pending` and never call Baofoo.
- [x] Tests for frontend-actionable missing-field codes/guidance when profile data is incomplete.
- [x] Unit tests that no qualification upload dependency blocks opening.
- [x] Regression test that an active merchant binding with pending APPLET auth returns `applet_auth_pending`, not `ready`.

### Task 6: Verify Fee Payment Callback Propagation

Files:

- Modify: `locallife/api/payment_callback.go`
- Modify: `locallife/logic/payment_fact_application_service.go`
- Modify: `locallife/db/query/payment_order.sql`
- Create focused tests in `locallife/api/payment_callback_test.go` and/or `locallife/logic`

Steps:

- [x] Recognize `payment_orders.business_type='baofu_account_verify_fee'`.
- [x] On direct payment success, mark payment paid through existing payment path.
- [x] Implement account-opening side effect for paid user-side verify-fee owners: find the active flow by owner-level attach, link paid payment to `verify_fee_payment_order_id`, move flow to `opening_processing`, enqueue or call opening continuation.
- [x] On payment closed/failed/expired, leave flow at `verify_fee_pending` with retry guidance, return a user-actionable `label/status_desc`, and log the terminal payment fact with owner/payment/flow context.
- [x] Use payment fact application claiming, conditional opening-flow updates, and persisted `open_trans_serial_no` as the implementation guard against duplicate opening attempts.
- [x] Prove duplicate API callbacks cannot trigger duplicate opening attempts end-to-end.
- [x] Implement owner-level reusable verify-fee lookup for pending/paid payments by canonical `attach`, payer, and amount.
- [x] Prove a paid owner-level verify-fee payment is reused by a replacement flow after Baofoo opening failure.
- [x] Ensure all unexpected callback/fact-application errors are logged once with `payment_order_id`, `out_trade_no`, `consumer`, `application_id`, and enough owner/flow context for retry. Do not return raw provider/internal errors to Mini Program callers.

Validation:

- [x] Duplicate callback test for the full API callback path.
- [x] Payment success then opening continuation test.
- [x] Payment closed/failed/expired does not open account.
- [x] Replacement-flow reuse test: same owner/purpose paid payment opens without charging again.
- [x] Review -> fix -> re-review entry recorded in section 10.2 for verify-fee callback/fact propagation.

### Task 7: Account Opening Recovery Scheduler

Files:

- Create: `locallife/worker/baofu_account_opening_recovery_scheduler.go`
- Modify: `locallife/main.go`
- Modify: `locallife/worker/processor.go` if async tasks are introduced

Steps:

- [x] Scan `opening_processing` and report/auth recovery states older than a configured minimum age.
- [x] Call Baofoo account query. Prefer `contractNo` when known; otherwise use persisted `loginNo` plus certificate identity. Do not send `platformNo`.
- [x] Apply terminal states to `baofu_account_bindings` and `baofu_account_opening_flows`.
- [x] After merchant binding active, trigger merchant report through the merchant-report continuation when configured.
- [x] Implement report/auth recovery as merchant-only; non-merchant flows do not enter report/auth recovery states.
- [x] Do not implement a qualification upload recovery scheduler in the current flow.
- [x] Alert on unmatched Baofoo callbacks/query contradictions; do not create bindings without a matching local flow.
- [x] Log scheduler query/opening/report/auth failures with `flow_id`, `owner_type`, `owner_id`, `open_trans_serial_no`, current state, and provider operation. Missing dependency/config no-op branches must be explicit and tested.

Validation:

- [x] Scheduler unit tests for terminal success and merchant report/auth continuation.
- [x] Scheduler unit tests for terminal failure/processing and missing client/config no-op.
- [x] Review -> fix -> re-review entry recorded in section 10.2 for recovery scheduler behavior.

### Task 8: Merchant Report And Applet Auth

Files:

- Modify: `locallife/logic/baofu_merchant_report_service.go`
- Modify: `locallife/db/query/baofu_merchant_report.sql`
- Modify: `locallife/logic/baofu_account_onboarding_service.go`
- Existing scheduler: `locallife/worker/baofu_merchant_report_recovery_scheduler.go`

Steps:

- [x] Keep account-opening merchant-report continuation restricted to `owner_type=merchant`.
- [x] After merchant account active, submit `merchant_report(WECHAT)`.
- [x] Use merchant `sharing_mer_id` as `bctMerId`.
- [x] On report success, persist `subMchID`.
- [x] Call `BindSubConfig` with `authType=APPLET` and LocalLife Mini Program appid.
- [x] Mark merchant flow/API readiness `ready` only after account active, report succeeded, and applet auth succeeded.
- [x] Implementation skips merchant report/auth for non-merchant owner types and marks non-merchant readiness from account active.
- [x] Prove platform/rider/operator never create merchant report rows with explicit tests.
- [x] Merchant report/auth failures return safe Chinese `label/status_desc` guidance and log provider failures with owner/report/flow context. Do not leak raw Baofoo response text to Mini Program users.

Validation:

- [x] Merchant active account triggers report/auth.
- [x] Platform/rider/operator never create merchant report rows.
- [x] Existing bind auth focused tests remain passing in the latest recorded logic test run.
- [x] Review -> fix -> re-review entry recorded in section 10.2 for merchant-only report/auth boundary.

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
  "status": "verify_fee_pending",
  "state": "verify_fee_pending",
  "label": "待支付开户核验费",
  "status_desc": "",
  "payment_ready": false,
  "profile_status": "complete",
  "flow_id": 789,
  "flow_state": "verify_fee_processing",
  "verify_fee_amount": 200,
  "payment_order_id": 456,
  "amount": 200,
  "business_type": "baofu_account_verify_fee",
  "out_trade_no": "BFVF202605080001",
  "payment": {
    "payment_order_id": 456,
    "amount": 200,
    "business_type": "baofu_account_verify_fee",
    "out_trade_no": "BFVF202605080001",
    "pay_params": {}
  },
  "bank_account_no_mask": "6222********1234",
  "bank_mobile_mask": "138****0000",
  "contact_mobile_mask": "",
  "email_mask": "",
  "wechat_sub_mch_id_mask": "",
  "updated_at": "2026-05-08T10:00:00Z"
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
- Use stable Chinese product copy for `label` / `status_desc`.
- Unexpected handler/service errors must reach a structured log boundary before a 5xx/503 response. Business-correctable states must return stable Chinese guidance and a retry/profile/payment next action.

Validation:

- [x] Handler tests for each role. Merchant owner GET, merchant manager GET/POST forbidden, rider POST, and operator/platform coverage are present in `TestBaofuSettlementAccount`.
- [x] Unauthorized/forbidden owner access tests for merchant manager GET/POST.
- [x] Tests proving arbitrary client owner fields are rejected before service call.
- [x] Regression test that merchant active binding does not return `payment_ready=true` until APPLET auth succeeds.
- [x] `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestBaofuSettlementAccount' -count=1`
- [x] `PATH="/usr/local/go/bin:$PATH" make swagger` after the route annotations and response DTO split.
- [x] Error/logging review proves all 5xx/503 paths are logged and all 4xx/business states return stable Chinese guidance.

## 7. Mini Program Implementation Plan

### Task 10: API Contracts

Files:

- Create: `weapp/miniprogram/api/baofu-account.ts`
- Modify: `weapp/miniprogram/api/payment.ts`

Steps:

- [x] Add settlement account DTOs for the current backend response fields.
- [x] Prove the Mini Program DTOs are contract-aligned with backend through TypeScript compile and review. Current status/profile/open-state fields now use explicit backend enums instead of permissive `string` fallbacks.
- [x] Add role-specific query/submit methods for merchant, rider and operator.
- [x] Extend `BusinessType` with `baofu_account_verify_fee`.
- [x] Keep provider details out of page code.
- [x] Do not model `qualificationTransSerialNo`, `platformNo`, or `platformTerminalId` in Mini Program types.
- [x] Ensure API/service status helpers expose stable next-action guidance for all backend states: `profile_pending`, `verify_fee_pending`, `verify_fee_processing`, `opening_processing`, `merchant_report_processing`, `applet_auth_pending`, `ready`, `failed`, and unknown/fallback.

Validation:

- [x] `PATH="$HOME/.local/bin:$PATH" npm run compile`
- [x] API/service review -> fix -> re-review entry recorded in section 10.2.

### Task 11: Shared Payment Progress Orchestrator

Files:

- Modify: `weapp/miniprogram/services/payment-workflow.ts`
- Create: `weapp/miniprogram/services/payment-progress-toast.ts` or equivalent service-level helper
- Refactor: `weapp/miniprogram/services/rider-deposit-payment.ts`
- Modify only the Baofoo verify-fee and rider-deposit call sites in this rollout unless another touched page already uses the same helper naturally.

Steps:

- [x] Provide one helper that shows long-duration TDesign loading toast while payment is being invoked and queried.
- [x] After `wx.requestPayment`, Baofoo verify-fee flow calls backend payment/settlement queries to decide terminal result.
- [x] Treat Baofoo verify-fee user cancel as non-terminal by re-querying backend settlement-account state; the shared helper itself still returns `cancelled` for legacy callers.
- [x] Apply the helper in code to Baofoo account verify fee.
- [x] Apply the helper in code to rider deposit.
- [x] Prove Baofoo verify-fee and rider-deposit helper usage by Mini Program compile and focused payment-flow review.
- [x] Resolve current scope drift: keep `claim-recovery-payment.ts`, combined-payment, order/reservation, dine-in checkout and payment-detail call-site changes as intentional scope because the product rule is now all Mini Program payment flows touched in this rollout must use the shared long-toast/query-terminal pattern.
- [x] Add a short follow-up note in the service comment or backlog section that any remaining future payment workflow must migrate to the helper before release.
- [x] Store pending payment context so page re-entry can recover. Baofoo verify-fee stores owner-role payment context and rider deposit keeps its existing pending recharge context; storage failure is logged and blocks starting payment with safe Chinese guidance instead of degrading recovery.
- [x] Catch unexpected payment workflow errors, log local context, hide long-running toast in `finally`, and return/render safe Chinese retry guidance instead of raw JS/backend/provider messages.
- [x] For all touched payment call sites, terminal UI must be driven by backend query/polling result, not by local `wx.requestPayment` success/cancel alone.

Validation:

- [x] Baofoo verify-fee and rider-deposit pages still compile.
- [ ] Weak-network/re-entry behavior for Baofoo verify fee and rider deposit reviewed manually in DevTools.
- [x] Shared payment workflow review -> fix -> re-review entry recorded in section 10.2.

### Task 12: Account Opening Workflow Service

Files:

- Create: `weapp/miniprogram/services/baofu-account-onboarding.ts`
- Modify role pages that surface settlement readiness

Steps:

- [x] `startOrResumeMerchantSettlementAccount()`
- [x] `startOrResumeRiderSettlementAccount()`
- [x] `startOrResumeOperatorSettlementAccount()`
- [x] Platform settlement-account calls are admin-console/backend routes, not normal Mini Program user flows. No platform Mini Program self-service entry has been added.
- [x] If backend returns verify-fee `pay_params`, call shared payment progress orchestrator.
- [x] After verify-fee payment reaches paid, call backend start/resume again.
- [x] Implement polling settlement account status until:
  - `ready`
  - `failed`
  - long-running processing timeout, with recoverable status shown
- [ ] Prove polling timeout and recoverable status behavior with service tests or Mini Program DevTools weak-network walkthrough.
- [x] Persist pending onboarding context by owner role so re-entry resumes query rather than restarting blindly.
- [x] Surface user guidance for each workflow terminal or recoverable state:
  - `profile_pending`: 补全开户资料。
  - `verify_fee_pending`: 支付 2 元核验费后继续开户。
  - `payment_pending` / `verify_fee_processing`: 支付结果确认中，可稍后刷新。
  - `opening_processing`: 宝付开户处理中，可稍后刷新。
  - `merchant_report_processing` / `applet_auth_pending`: 微信支付展示商户名称配置中。
  - `ready`: 已开通。
  - `failed`: 展示失败原因或安全兜底文案，并提供重试/联系客服指引。
- [x] Catch unexpected service exceptions, log `role` and action context, and return safe Chinese retry guidance to page code.

Validation:

- [ ] Unit-level service tests if the project has test harness for services.
- [ ] Manual role flow walkthrough in Mini Program DevTools.
- [x] Workflow service review -> fix -> re-review entry recorded in section 10.2.

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

Current implementation status:

- [x] Rider settlement-account page exists and is linked from rider income.
- [x] Merchant Baofoo settlement-account surface is wired into merchant finance/config entries. Existing `/pages/merchant/settings/applyment/settlement-account/index` remains the WeChat/ecommerce settlement-account modification surface, not this Baofoo account-opening flow.
- [x] Operator settlement-account page/surface is wired from the operator finance withdraw page.
- [x] No normal Mini Program platform self-service page has been added.
- [x] Rider page copy and actions provide clear guidance for profile completion, 2 元核验费 payment, long-running opening, failure retry, and ready state. Provider/internal field names are not shown.
- [x] Merchant and operator surfaces use shared backend state/view helpers and do not duplicate provider-state parsing in page code.
- [x] Page-level unexpected errors are logged with page/action/role context and rendered as safe Chinese retry guidance.

Validation:

- [x] `PATH="$HOME/.local/bin:$PATH" npm run compile`
- [x] `PATH="$HOME/.local/bin:$PATH" npm run lint`
- [x] `PATH="$HOME/.local/bin:$PATH" npm run quality:check`
- [x] Role surface review -> fix -> re-review entry recorded in section 10.2.

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

- [x] `PATH="/usr/local/go/bin:$PATH" make sqlc`
- [x] `PATH="/usr/local/go/bin:$PATH" make mock`
- [x] `PATH="/usr/local/go/bin:$PATH" make swagger`
- [x] `PATH="/usr/local/go/bin:$PATH" make check-generated`
- [x] `PATH="/usr/local/go/bin:$PATH" make check-baofu-contract`
- [x] `PATH="/usr/local/go/bin:$PATH" go test ./baofu/account/... -count=1`
- [x] `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'Test.*Baofu.*Account|Test.*Opening|Test.*MerchantReport|TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFee' -count=1`
- [x] `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'Test.*Baofu|Test.*SettlementAccount|TestHandlePaymentNotify.*VerifyFee' -count=1`
- [x] `PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'Test.*Baofu.*Opening|Test.*MerchantReport' -count=1`

Mini Program:

- [x] `PATH="$HOME/.local/bin:$PATH" npm run compile`
- [x] `PATH="$HOME/.local/bin:$PATH" npm run lint`
- [x] `PATH="$HOME/.local/bin:$PATH" npm run quality:check`

Manual/evidence:

- [ ] Merchant sandbox or provider-confirmed test: open business account without qualification upload -> merchant report -> APPLET auth.
- [ ] Rider sandbox or provider-confirmed test: verify-fee payment paid -> open personal account without qualification upload.
- [ ] Operator sandbox or provider-confirmed test: verify-fee payment paid -> open personal account without qualification upload.
- [ ] Platform sandbox or provider-confirmed test: open business account without qualification upload, no merchant report rows.

## 10.1 Code Sync Audit Notes

These notes explain why some task cards remain unchecked even though nearby code exists.

- `baofu_account_verify_fee` exists as a backend payment business type and Mini Program `BusinessType`, and the Baofoo onboarding service creates direct Mini Program payments with description “宝付开户核验费”. The wallet/payment ledger display now maps payment entries to “开户核验费” and refund entries to “开户核验费退款”.
- Merchant/platform/rider/operator onboarding paths have focused tests in `logic/baofu_account_onboarding_service_test.go`. Operator-specific tests now prove verify-fee gating, personal account type, no platform-borne fee ledger, and no merchant-report side effect.
- Direct payment callbacks create Baofoo verify-fee payment facts, and the payment fact application continues opening after a success fact. Closed/failed/expired terminal facts reset the flow to retryable `verify_fee_pending`, clear `verify_fee_payment_order_id`, log payment/fact/application/owner/flow context, and do not call Baofoo opening continuation.
- `api/payment_callback_test.go` now replays the same Baofoo verify-fee callback through `/v1/webhooks/wechat-pay/notify`; the second replay exits through notification idempotency and does not enqueue a second payment fact application.
- `worker/baofu_account_opening_recovery_scheduler_test.go` now covers opening success, merchant report/auth continuation, terminal failure, provider still-processing, missing dependency/config no-op logging, and per-flow failure log context. `api/baofu_callback_test.go` covers unmatched account-open callback alert persistence and blocks non-local callback `outRequestNo` even when `contractNo` exists. `logic/baofu_account_onboarding_service_test.go` covers Baofoo query result `outRequestNo` mismatch blocking, contract-owner contradiction blocking, and alert persistence.
- `weapp/miniprogram/pages/merchant/settings/applyment/settlement-account/index` is the existing WeChat/ecommerce settlement-account modification page. It is not the Baofoo account-opening role surface described by this plan.
- Mini Program `api/baofu-account.ts` and `services/baofu-account-onboarding.ts` have the role methods and payment/polling code. TypeScript compile, lint and quality gates have passed; weak-network/re-entry DevTools walkthrough is still missing.
- `weapp/miniprogram/services/baofu-account-onboarding.ts` re-queries after cancellation, starts/resumes after paid verify fee, and persists owner-role pending Baofoo onboarding context so page re-entry resumes by backend query/polling.

## 10.2 Review / Fix / Re-review Ledger

Use this ledger for this rollout. Each item may move to done only after review, fix, and re-review are recorded with validation evidence.

| Task | Review finding | Fix required | Re-review evidence | Status |
| --- | --- | --- | --- | --- |
| Task 2 payment ledger title | Wallet mapping lacked `baofu_account_verify_fee`, so users would see generic “支付记录”. | Added explicit wallet ledger titles: payment “开户核验费” and refund fallback “开户核验费退款”. | Re-review passed: `PATH="$HOME/.local/bin:$PATH" npm run compile`; `PATH="$HOME/.local/bin:$PATH" npx eslint miniprogram/pages/user_center/wallet/index.ts`; `rg -n "baofu_account_verify_fee|开户核验费|开户核验费退款|支付记录|businessTitleMap" miniprogram/pages/user_center/wallet/index.ts miniprogram/api/payment.ts miniprogram/services/payment-workflow.ts miniprogram/services/baofu-account-onboarding.ts`. | Done |
| Task 5 operator onboarding coverage | Existing tests covered merchant/platform/rider; operator was inferred from shared branch but not proven. | Added explicit operator tests for verify-fee gate, account type personal, no platform-borne fee ledger, and no merchant report side effect. | Re-review passed: `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuAccountOnboardingServiceStart_OperatorRequiresVerifyFeeBeforeOpening|TestBaofuAccountOnboardingServiceContinueAfterVerifyFeePaid_OpensOperatorWithoutPlatformFeeLedgerOrMerchantReport|TestBaofuAccountOnboardingServiceApplyAccountOpenResult_NonMerchantOwnersSkipMerchantReport' -count=1`; broader focused `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'Test.*Baofu.*Operator|Test.*Baofu.*Account|Test.*MerchantReport' -count=1`. | Done |
| Task 5 profile guidance and open-error boundary | Review found `profile_pending` guidance existed but Task 5 was not closed; follow-up review found POST opening could return raw Baofoo/provider/internal errors as `400` without owner-scoped logging. | Added stable `missing_fields`/Chinese `status_desc` assertions for logic/API profile-pending responses; added Baofoo open-error mapping with `RequestErrorWithCause`, owner-scoped settlement-account request logging, safe Chinese API guidance for provider and unexpected open failures, and active-binding no-`missing_fields` regressions. | Re-review passed: `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuAccountOnboardingServiceStart_(MissingProfileDoesNotCallBaofu|ProviderOpenErrorBecomesSafeRequestError|RiderRequiresVerifyFeeBeforeOpening|OperatorRequiresVerifyFeeBeforeOpening|BusinessOwnerOpensWithIndustry9931AndPlatformFeeLedger|ReplacementFlowReusesPaidVerifyFeeWithoutChargingAgain|MerchantActiveBindingWaitsForAppletAuth)|TestBaofuAccountOnboardingServiceContinueAfterVerifyFeePaid_(OpensRiderWithoutPlatformFeeLedger|OpensOperatorWithoutPlatformFeeLedgerOrMerchantReport)|TestBaofuAccountOnboardingServiceApplyAccountOpenResult_(NonMerchantOwnersSkipMerchantReport|MerchantActiveWaitsForReport)' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestBaofuSettlementAccount(MerchantOwnerCanReadSafeSummary|MerchantPostBaofooProviderFailureReturnsSafeGuidance|MerchantPostUnexpectedOpenFailureReturnsSafeGuidance|RiderProfilePendingReturnsMissingFieldGuidance|RiderActiveBindingDoesNotReturnProfileMissingFields|RiderPostActiveBindingDoesNotReturnProfileMissingFields|RiderVerifyFeePendingReturnsRetryGuidance|RiderPostCreatesVerifyFeeBeforeBaofuOpening)' -count=1`; `git diff --check -- locallife/logic/baofu_error_mapping.go locallife/logic/baofu_account_onboarding_service.go locallife/logic/baofu_account_onboarding_service_test.go locallife/api/baofu_settlement_account.go locallife/api/baofu_settlement_account_test.go artifacts/baofu-payment/baofu-account-opening-end-to-end-plan-2026-05-08.md`. | Done |
| Task 1 config/constants full validation | The task had focused config and Baofoo logic tests, but the broader requested util/logic config validation remained open in the task card. | Re-ran the full focused util/logic command for Baofoo/config tests after generated and DB closeout. | Re-review passed: `PATH="/usr/local/go/bin:$PATH" go test ./util ./logic -run 'Test.*Baofu|Test.*Config' -count=1`. | Done |
| Task 3 DB constraints | Schema constraints existed in migration/query/sqlc, but the task card still lacked executable DB tests for owner/account type constraints, sensitive-field encrypted persistence, active-flow uniqueness, ready/failed replacement, and idempotent profile upsert. | Added `db/sqlc/baofu_account_opening_flow_test.go` with focused DB-level coverage for profile upsert/masked-vs-ciphertext persistence, invalid owner/account checks, login/payment guards before opening, active-flow uniqueness, ready replacement, and failed replacement after paid verify fee. | Re-review passed: `PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -run 'TestBaofuAccountOpening' -count=1`; broader `PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -count=1`; `PATH="/usr/local/go/bin:$PATH" make check-generated` reported `generated artifacts are in sync`. | Done |
| Task 6 verify-fee closed/failed/expired handling | Review found closed/failed implementation existed but the document was stale; expired and frontend-facing retry guidance were not proven. | Added expired terminal fact test and GET settlement-account retry-guidance test; verified closed/failed/expired reset flow to `verify_fee_pending`, clear payment link, log context, and do not call Baofoo opening continuation. | Re-review passed: `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFee(Closed\|Failed\|Expired)ReturnsFlowToRetryablePending\|TestBaofuAccountOnboardingServiceStart_ReplacementFlowReusesPaidVerifyFeeWithoutChargingAgain' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestHandlePaymentNotify_BaofuVerifyFeeRecordsPaymentFactAndDedupesReplay\|TestBaofuSettlementAccountRiderVerifyFeePendingReturnsRetryGuidance' -count=1`; broader focused logic/API runs also passed. | Done |
| Task 6 duplicate callback/replacement reuse | Duplicate full API callback and replacement-flow paid reuse were implemented by shape but lacked proof. | Added API replay test proving the duplicate callback exits through notification idempotency without a second application enqueue; added onboarding test proving a replacement flow reuses the paid verify-fee payment and opens without creating a new payment order. | Re-review passed: `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuVerifyFee\|TestPaymentOrderServiceQueryPaymentOrder\|TestBaofuAccountOnboardingServiceStart_ReplacementFlowReusesPaidVerifyFeeWithoutChargingAgain\|TestBaofuAccountOnboardingServiceContinueAfterVerifyFeePaid' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestHandlePaymentNotify_BaofuVerifyFeeRecordsPaymentFactAndDedupesReplay\|TestHandlePaymentNotifyIdempotency\|TestBaofuSettlementAccountRiderVerifyFeePendingReturnsRetryGuidance\|TestBaofuSettlementAccountRiderPostCreatesVerifyFeeBeforeBaofuOpening' -count=1`. | Done |
| Task 7 recovery scheduler | Review found success/report continuation existed, but terminal failure, provider still-processing, missing client/config no-op logging, scheduler per-flow log context, unmatched callbacks, and query contract-owner contradictions were not proven. Follow-up read-only review then found a High issue: callback `outRequestNo` miss could fallback by `contractNo`, and query result `OutRequestNo` mismatch was not blocked; it also found incomplete merchant-report config no-op lacked direct test coverage. | Added focused scheduler tests for failure/processing/no-op/log context; added unmatched account-open callback platform alert; added onboarding contract-owner contradiction guard; then tightened callback/query serial matching so non-local `outRequestNo` is alerted and blocked, added query result `OutRequestNo` mismatch alert/block, added incomplete merchant-report config no-op log test, and fixed fake provider tests to echo request流水号 for normal paths. Removed an unused worker alias constant during re-review cleanup. | Re-review passed: `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestBaofuAccountOpenCallback(BlocksWhenOutRequestNoMissesEvenIfContractExists\|PersistsAlertWhenFlowCannotBeMatched)' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuAccountOnboardingServiceRecoverOpening(AlertsAndRejectsMismatchedOutRequestNo\|AlertsAndRejectsContractOwnedByDifferentOwner\|QueriesByLoginNoAndAppliesResult)' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'TestBaofuAccountOpeningRecoveryScheduler(Log\|Logs\|Queries\|Marks\|Leaves\|Submits)' -count=1`; broader focused `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestBaofuAccountOpenCallback' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuAccountOnboardingService(RecoverOpening\|ApplyAccountOpenResult\|ContinueAfterVerifyFeePaid\|Start_)' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'TestBaofuAccountOpeningRecoveryScheduler' -count=1`; `git diff --check -- locallife/api/baofu_callback.go locallife/api/baofu_callback_test.go locallife/logic/baofu_account_onboarding_service.go locallife/logic/baofu_account_onboarding_service_test.go locallife/worker/baofu_account_opening_recovery_scheduler_test.go artifacts/baofu-payment/baofu-account-opening-end-to-end-plan-2026-05-08.md`. | Done |
| Task 8 merchant-only report/auth | Code restricts report/auth to merchant, but platform/rider/operator no-report rows were not explicitly tested. | Added table-driven service test proving platform/rider/operator active account results go `ready` without merchant report rows or report client calls. | Re-review passed: `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuAccountOnboardingServiceApplyAccountOpenResult_NonMerchantOwnersSkipMerchantReport|Test.*MerchantReport' -count=1`; broader focused logic run also passed. | Done |
| Task 8 report/auth failure guidance and logging | Review found merchant report and `bind_sub_config` provider failures could propagate as raw provider errors, active merchant GET/POST failure states used generic or empty guidance, and provider-error logs did not always include report/flow context; follow-up review found provider failures during initial report submit/query/bind could drop the persisted report row before logging. | Added merchant-report provider error mapping with `RequestErrorWithCause`, a provider context wrapper carrying `flow_id`, owner, current state, `merchant_report_id`, `merchant_report_no`, operation and capability; settlement-account request logging now emits that context. GET and POST active-merchant failed states return safe Chinese `status_desc` for report failure vs authorization-directory failure and never expose persisted `failure_message`. Report submit/query/bind provider failures now return the persisted report row so upstream logging can include report context. | Re-review passed: `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuAccountOnboardingServiceStart_(MerchantActiveBindingWaitsForAppletAuth\|MerchantAppletAuthFailedReturnsGuidance\|ProviderOpenErrorBecomesSafeRequestError)\|TestBaofuAccountOnboardingServiceApplyAccountOpenResult_(NonMerchantOwnersSkipMerchantReport\|MerchantActiveWaitsForReport)\|TestBaofuAccountMerchantReportServiceRecoverProviderErrorReturnsSafeContext\|TestBaofuMerchantReportService(RequiresMerchantSharingMerID\|BindsAppletAfterReportSuccess\|RecoversProcessingReportAndBindsApplet\|ReturnsPersistedReportOnProviderFailures)\|TestBaofuPaymentReadiness(RequiresMerchantSubMchIDAndAppletAuth\|UsesMerchantReportSubMchIDAfterAppletAuth)\|TestMapBaofuMerchantReport(ProviderErrorKeepsRawTextOutOfPublicMessage\|AppletAuthProviderErrorReturnsSpecificGuidance)' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestBaofuSettlementAccount(MerchantOwnerCanReadSafeSummary\|MerchantActiveBindingWaitsForAppletAuth\|MerchantReportFailedReturnsSafeGuidance\|MerchantAppletAuthFailedReturnsSafeGuidance\|MerchantPostAppletAuthFailedReturnsSafeGuidance\|RequestErrorLogIncludesMerchantReportContext\|MerchantPostBaofooProviderFailureReturnsSafeGuidance\|MerchantPostUnexpectedOpenFailureReturnsSafeGuidance)' -count=1`; `git diff --check -- locallife/logic/baofu_error_mapping.go locallife/logic/baofu_account_merchant_report_service.go locallife/logic/baofu_account_onboarding_service.go locallife/logic/baofu_merchant_report_service.go locallife/logic/baofu_merchant_report_service_test.go locallife/logic/baofu_account_onboarding_service_test.go locallife/api/baofu_settlement_account.go locallife/api/baofu_settlement_account_test.go`. | Done |
| Task 9 role API/logging/swagger | Review found duplicated handler/request/response/helper definitions remained in `baofu_settlement_account.go` after the API split, and the file still needed one more pass to satisfy the file-size guardrail. | Removed the duplicated DTO/helper definitions from the handler file, kept the route handlers/wiring only, moved the residual service-ready helper into the types file, and kept the existing safe-Chinese/logged failure behavior. | Re-review passed: `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestBaofuSettlementAccount' -count=1`; `PATH="/usr/local/go/bin:$PATH" make swagger`; `PATH="/usr/local/go/bin:$PATH" make check-generated`; `PATH="/usr/local/go/bin:$PATH" make check-baofu-contract`; `git diff --check -- locallife/api/baofu_settlement_account.go locallife/api/baofu_settlement_account_types.go locallife/api/baofu_settlement_account_request.go locallife/api/baofu_settlement_account_response.go locallife/api/baofu_settlement_account_test.go locallife/docs/docs.go locallife/docs/swagger.json locallife/docs/swagger.yaml artifacts/baofu-payment/baofu-account-opening-end-to-end-plan-2026-05-08.md`. | Done |
| Backend generated/contract gates | Task cards had generated-artifact and Baofoo contract gates open after SQL/API/Swagger-facing changes, so handoff could not distinguish code implementation from generated drift readiness. | Re-ran required generated and contract gates from the backend project directory and kept generated artifacts in sync. | Re-review passed: `PATH="/usr/local/go/bin:$PATH" make sqlc`; `PATH="/usr/local/go/bin:$PATH" make mock`; `PATH="/usr/local/go/bin:$PATH" make swagger`; `PATH="/usr/local/go/bin:$PATH" make check-generated` reported `generated artifacts are in sync`; `PATH="/usr/local/go/bin:$PATH" make check-baofu-contract` reported `baofu contract drift guard passed`. | Done |
| Task 10 Mini Program API contract/status helpers | Review found Mini Program DTOs were still permissive and incomplete: the response type omitted backend-owned fields such as owner/account/state/flow/missing-fields/masked values, status fields allowed arbitrary strings, status helpers collapsed backend states into generic copy, and the rider page assumed a plaintext `profile` response that the backend intentionally does not return. | Expanded `api/baofu-account.ts` to mirror the backend settlement-account response, added explicit status/profile/open-state enums, added stable next-action guidance for all backend states plus unknown fallback, removed legacy virtual backend-state aliases from the onboarding service, and removed the rider page's nonexistent plaintext-profile prefill path. | Re-review passed: `PATH="$HOME/.local/bin:$PATH" npm run compile`; `PATH="$HOME/.local/bin:$PATH" npx eslint miniprogram/api/baofu-account.ts miniprogram/services/baofu-account-onboarding.ts miniprogram/pages/rider/settlement-account/index.ts`; `rg -n "status: BaofuSettlementAccountStatus \\| string|state: BaofuSettlementAccountStatus \\| string|flow_state\\?: BaofuSettlementAccountStatus \\| string|profile_status\\?: .*string|open_state\\?: string|payment_pending|verifying|case 'opening'|case 'processing'|profile\\?|fail_reason|failed_reason" weapp/miniprogram/api/baofu-account.ts weapp/miniprogram/services/baofu-account-onboarding.ts weapp/miniprogram/pages/rider/settlement-account/index.ts` returned no matches. | Done |
| Task 11 shared payment progress orchestrator | Review found payment workflow catches returned pending states without logging, Baofoo verify-fee lacked owner-role pending context recovery, rider deposit storage/read fallback errors were silent, `payment-detail`/dine-in/order-list touched payment call sites still used short native loading or missed page context, and scope drift across order/reservation/claim-recovery/combined-payment paths needed an explicit decision. Follow-up review found pending Baofoo context could resume by POST instead of query/poll and payment-list still wrapped helper calls with native loading. | Added structured logs for non-cancel `requestPayment`, query, poll, create-payment, combined-payment and storage failures; made Baofoo verify-fee persist role-scoped pending context before payment and recover by GET/query/poll on re-entry; made storage failure block payment with safe Chinese guidance; passed page context through touched payment call sites; added `t-toast` to `payment-detail`; removed native loading wrappers from touched order-list payment entries; kept scope drift as intentional because all touched Mini Program payment paths now follow shared long-toast/query-terminal semantics. | Re-review passed: `PATH="$HOME/.local/bin:$PATH" npm run compile`; `PATH="$HOME/.local/bin:$PATH" npx eslint miniprogram/services/payment-workflow.ts miniprogram/services/baofu-account-onboarding.ts miniprogram/services/rider-deposit-payment.ts miniprogram/api/payment.ts miniprogram/pages/rider/settlement-account/index.ts miniprogram/pages/rider/deposit/index.ts miniprogram/pages/user_center/payment-detail/index.ts miniprogram/pages/orders/list/index.ts miniprogram/pages/reservation/confirm/index.ts miniprogram/services/dine-in-checkout.ts miniprogram/pages/dine-in/checkout/checkout.ts`; `PATH="$HOME/.local/bin:$PATH" npm run gate:payment-workflow-boundary`; focused scans showed no bare catch returns in payment workflow services and direct `wx.requestPayment`/`invokeWechatPay` only in `api/payment.ts` and `services/payment-workflow.ts`. Manual DevTools weak-network/re-entry walkthrough is still open and not claimed. | Done |
| Task 12 account opening workflow service | Review found the workflow service existed, but role/action logging was inconsistent, thrown service exceptions did not always carry safe Chinese `userMessage`, failed-state copy could miss the backend `status_desc`, and the document had not separated code evidence from DevTools/manual walkthrough evidence. | Added `logAndThrowWorkflowError(action, role, ...)` so polling, start/resume, profile submit and continue-payment failures all log role/action context and return safe Chinese guidance to pages; tightened pending-context read failure to log and surface a safe retry message; kept failed-state feedback using backend `status_desc` or safe fallback; preserved owner-role pending context recovery through GET/query/poll instead of blind POST restart. | Re-review passed: `PATH="$HOME/.local/bin:$PATH" npm run compile`; `PATH="$HOME/.local/bin:$PATH" npx eslint miniprogram/services/baofu-account-onboarding.ts miniprogram/pages/rider/settlement-account/index.ts`; focused Node checks proved polling timeout returns `pending_confirmation`, pending owner-role context recovery is wired through `getBaofuSettlementAccount`/poll/complete paths, and feedback cases cover terminal/recoverable states. Unit-test harness and DevTools walkthrough remain open and are not claimed. | Done |
| Task 13 role surfaces | Merchant/operator Baofoo surfaces were missing, rider copy/logging/re-entry handling needed a closeout pass, and direct business-status comparisons in pages tripped the Mini Program business-status boundary gate. Merchant finance/config entries also still pointed only at existing WeChat/ecommerce settlement-account surfaces. | Added merchant and operator Baofoo settlement-account pages, routes and entries; added shared role view helper and shared non-consumer settlement-account styles; updated rider copy/actions and pending-context read error handling; moved pending-context clear status logic into the onboarding service helper; added page-level role/action logging and safe Chinese guidance. | Re-review passed: `PATH="$HOME/.local/bin:$PATH" npm run compile`; `PATH="$HOME/.local/bin:$PATH" npm run lint`; `PATH="$HOME/.local/bin:$PATH" npm run gate:weapp`; `PATH="$HOME/.local/bin:$PATH" npm run quality:check` re-run on 2026-05-09; `git diff --check` on touched Mini Program role-surface files; focused scans showed no provider/internal field leakage in role copy, no direct `wx.requestPayment`/`invokeWechatPay` in role pages, role/action context in Baofoo page logs, and the old merchant WeChat settlement page remains separate. Manual DevTools weak-network/re-entry walkthrough is still open and not claimed. | Done |
| Task 10-13 Mini Program validation | API/service/page code exists and compile/lint/quality now pass, but weak-network/re-entry manual DevTools walkthrough remains open. | Keep Mini Program validation evidence current; complete manual weak-network/re-entry walkthrough before release. | Re-review passed for automated gates: `PATH="$HOME/.local/bin:$PATH" npm run quality:check` re-run on 2026-05-09, including lint, compile, `gate:weapp`, `business-status-boundary`, and `payment-workflow-boundary`. Manual DevTools evidence remains pending. | Open |

Document-only review closure on 2026-05-09:

- Review findings:
  - Some checked items used combined wording such as rider/operator, platform/rider/operator, duplicate callback, replacement-flow reuse, and DTO alignment in ways that could be read as stronger than the available evidence.
  - Error logging, frontend guidance, generated-code gates, and review/fix/re-review acceptance were not explicit enough for a handoff.
- Fix applied:
  - Split implementation-exists checkboxes from proof/validation checkboxes.
  - Kept operator-specific coverage, duplicate full API callback coverage, replacement-flow reuse proof, missing-field guidance, Mini Program contract proof, weak-network/re-entry walkthrough, and shared-helper scope drift open.
  - Added section 1.2 closeout rules, section 5.1.6.1 logging/frontend guidance rules, and this review ledger.
  - Added missing validation gates for `make mock`, `make swagger`, and `make check-baofu-contract`.
- Re-review evidence:
  - `rg -n "\\[x\\].*(rider/operator|platform/rider/operator|duplicate callback|replacement|missing-field|contract-aligned|compile|weak-network|DevTools|scope drift|closed/failed|never create merchant report|matching backend|operator-specific|Prove|validated|闭环)" artifacts/baofu-payment/baofu-account-opening-end-to-end-plan-2026-05-08.md` now only reports implementation-exists items, not proof/validation completion claims.
  - `git diff --check -- artifacts/baofu-payment/baofu-account-opening-end-to-end-plan-2026-05-08.md` passed with no whitespace errors.
- Remaining risk:
  - No code or tests were changed in this document-only pass. All open code/test/validation gaps remain open and must not be treated as completed by this review.

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
