# WeChat Platform Surface Removal Review Fix Plan

**Risk class:** G3. This change touches payment-domain routing, callbacks, generated backend contracts, SQL migrations, async workers, and web/Mini Program callers.

**Context:** The working tree is removing WeChat platform ecommerce and ordinary service-provider surfaces while retaining WeChat direct-payment paths and Baofu/BaoCaiTong main-business payment paths. The review found incomplete propagation across build tools, retained Mini Program shipping callback routing, frontend callers, migration semantics, SQL/sqlc residuals, and generated artifacts.

**Operating boundary:**

- Do not reintroduce retired WeChat platform ecommerce or ordinary service-provider runtime surfaces.
- Keep retained WeChat direct-payment, merchant-transfer, deposit/recovery, refund callback, Mini Program shipping upload, and shipping settlement-event helpers.
- Keep Baofu/BaoCaiTong as the main-business provider for payment, refund, share, account opening, withdrawal, and provider callbacks.
- Prefer deletion or explicit archival over silent provider/channel relabeling.

## Original Review Findings

1. **阻断：后端全量编译失败。**
   `cmd/wechat_doc_extract/main.go` 仍声明 `applyment/ordering/subsidy/refund/...` 等 audit scope，并调用已删除的 `wechatdoc.Audit*Alignment` 函数。
   验证：`go test ./... -run '^$'` 失败，集中报 `undefined: wechatdoc.AuditApplymentAlignment` 等错误。CI/构建目前过不了。

2. **高风险：保留文档写着 `settlement-notify` 仍 active，但路由已被移除。**
   `.github/standards/domains/wechat-payment/README.md` 仍把 `POST /v1/webhooks/wechat-miniprogram/settlement-notify` 列为保留回调。
   但当前 `api/server.go` 的 webhook 路由只剩 media-check、直连支付、宝付回调，没有 settlement-notify。`worker` 仍保留微信发货上报任务并读取 `WechatShippingSettleNotifyURL`。
   如果这条回调用于小程序发货/结算事件闭环，生产会变成 404；如果是刻意移除，需要同步改 domain README、配置、测试辅助和发货结算语义。

3. **高风险：前端/小程序仍调用已删除的普通服务商/收付通接口。**
   后端删除了 `/v1/merchant/applyment/*`、`/v1/merchant/complaints/*`、`/v1/merchant/finance/account/settlement-account*` 等旧路由，但调用端还在：
   `weapp/miniprogram/api/merchant-applyment.ts` 调 `/v1/merchant/applyment/status` 和 `/bindbank`。
   `weapp/miniprogram/api/merchant-complaints.ts` 调投诉旧接口。
   `weapp/miniprogram/api/merchant-settlement-account.ts` 调旧结算账户接口。
   Web 端也仍展示/提交“普通服务商进件”。
   这会直接造成商户后台/小程序 404 或错误引导。

4. **高风险：迁移 `000235` 把历史渠道改成宝付，但没有同步 provider/capability，账务语义会被污染。**
   `000235` 将所有非 `direct/baofu_aggregate` 的 `payment_orders`、`external_payment_commands`、`external_payment_facts` channel 改成 `baofu_aggregate`，但没有改 `provider` 和 `capability`。当前 Baofu fact 应用逻辑要求 `provider=baofu + channel=baofu_aggregate + capability=baofu_*`。
   结果可能出现 `provider=wechat, channel=baofu_aggregate` 的历史事实，既不像微信，也不会被宝付处理链正确识别。更糟的是 `worker/order_payment_fact.go` 仍有新写入 `Provider: wechat` + `Channel: baofu_aggregate` 的路径，需要确认是否死代码或同步清理。

5. **中高：数据库和 sqlc 残留还没清干净。**
   `000235` 只 drop 了 `profit_sharing_receiver_*` 和 `ecommerce_applyments`。但旧微信平台能力表/查询还在，包括 `wechat_complaints`、`subsidy_orders`、`wechat_merchant_violations`、`merchant_cancel_withdraw_applications`。
   如果要保留历史审计，需要明确 read-only/archival 策略；如果目标是拔除运行面，这些 query/sqlc model 仍是残留面。

6. **中：关键迁移文件目前是 untracked。**
   `git status` 显示 `000235_drop_legacy_wechat_platform_payment_surfaces.{up,down}.sql` 和 `000236_update_subsidy_order_comment.{up,down}.sql` 还没被跟踪。若这是准备提交的清理迁移，当前状态下不会随提交进入版本库。

7. **中：生成物状态需要重新收口。**
   `make check-generated` 之前失败，并运行了 `make sqlc`/`make swagger`，导致当前工作区里生成文件已有再生成后的 diff。这个分支需要在修完编译和接口残留后重新跑生成检查，并确认 `docs/sqlc/mock` 的变更是完整且有意的。

## Fix Tasks

### Task 1: Remove retired audit mode from `cmd/wechat_doc_extract`

**Files:**

- Modify: `locallife/cmd/wechat_doc_extract/main.go`

**Steps:**

- [x] Reproduce compile failure with `go test ./cmd/wechat_doc_extract -run '^$'`.
- [x] Remove the deleted audit dispatch surface and stale output envelope.
- [x] Verify with `go test ./cmd/wechat_doc_extract -run '^$'`.
- [x] Re-run `go test ./... -run '^$'` to reveal any next compile blocker.

**Acceptance:** The extractor remains a markdown extraction CLI and no longer references retired WeChat platform audit functions.

### Task 2: Restore or explicitly retire Mini Program settlement callback

**Files:**

- Inspect: historical `locallife/api/payment_callback.go`
- Modify: `locallife/api/payment_callback.go`
- Modify: `locallife/api/server.go`
- Test: `locallife/api/payment_callback_test.go`

**Decision:** Restore the retained route as a no-main-business WeChat Mini Program shipping settlement callback, because the domain README, env config, and shipping upload worker still require it.

**Steps:**

- [x] Recover the previous `handleOrderSettlementNotify` semantics from git history.
- [x] Add/adjust a focused route test that fails while the route is missing.
- [x] Register `POST /v1/webhooks/wechat-miniprogram/settlement-notify`.
- [x] Verify the focused API test.

**Acceptance:** The retained callback route returns the intended webhook response and no ordinary-service-provider/payment-acquisition behavior is restored.

### Task 3: Remove or reroute active Mini Program callers of deleted backend routes

**Scope update:** User confirmed the `web/` project can be treated as archived and will not be updated for now. Keep the original Web finding as historical context, but do not modify or validate `web/` in this repair pass.

**Files:**

- Modify/remove active Mini Program applyment, complaint, and old settlement-account API consumers.
- Do not modify archived `web/` files in this pass.
- Update stale product copy that promises retired WeChat platform onboarding.

**Steps:**

- [x] Map all active callers of `/v1/merchant/applyment/*`, `/v1/merchant/complaints*`, and `/v1/merchant/finance/account/settlement-account*`.
- [x] For merchant settlement account, reroute to current Baofu endpoints where an equivalent exists (`/v1/merchant/settlement-account`).
- [x] For applyment and complaints, remove active navigation/calls or replace them with disabled/retired-state copy, because the backend surfaces were deleted.
- [x] Run the smallest relevant `weapp` validation commands available locally.

**Acceptance:** Active Mini Program code no longer calls deleted backend routes; archived `web/` residuals are explicitly out of scope for this pass.

### Task 4: Fix migration and fact/channel semantics

**Files:**

- Modify: `locallife/db/migration/000235_drop_legacy_wechat_platform_payment_surfaces.up.sql`
- Modify: `locallife/db/migration/000235_drop_legacy_wechat_platform_payment_surfaces.down.sql`
- Inspect/fix: `locallife/worker/order_payment_fact.go`

**Steps:**

- [x] Replace blanket relabeling of retired WeChat channels as `baofu_aggregate` with explicit archival/deletion or leave historical rows with valid legacy enum support until cleanup is safe.
- [x] Ensure no active code writes `provider=wechat` with `channel=baofu_aggregate` for main-business paths.
- [x] Keep constraints compatible with retained historical rows or delete/archive rows before tightening constraints.
- [x] Run SQL regeneration and focused tests if query/schema assumptions change.

**Acceptance:** No new or migrated fact/command records have contradictory `provider/channel/capability` semantics.

### Task 5: Clean residual SQL/sqlc legacy surfaces or mark them archival

**Files:**

- Delete or archive: `locallife/db/query/wechat_complaint.sql`
- Delete or archive: `locallife/db/query/subsidy_order.sql`
- Delete or archive: `locallife/db/query/wechat_merchant_violation.sql`
- Delete or archive: `locallife/db/query/merchant_cancel_withdraw.sql`
- Modify migrations and generated sqlc as required.

**Steps:**

- [x] Confirm whether each residual table is still used by retained direct WeChat complaint/shipping behavior or only by retired platform surfaces.
- [x] Remove retired query files and drop retired tables in `000235`, or document them as historical read-only if retention is required.
- [x] Run `make sqlc`.
- [x] Remove stale tests/mocks generated from deleted queries.

**Implemented notes:**

- Removed `wechat_complaint.sql`, `subsidy_order.sql`, `wechat_merchant_violation.sql`, and `merchant_cancel_withdraw.sql`.
- Extended `000235` to drop `wechat_complaints`, `subsidy_orders`, `wechat_merchant_violations`, and `merchant_cancel_withdraw_applications`; the down migration recreates empty legacy schemas without promising data recovery.
- Converted `000236_update_subsidy_order_comment` to a no-op because `subsidy_orders` is removed by `000235`.
- Regenerated sqlc/mocks and deleted stale generated/test files for the removed query surfaces.
- Tightened `PaymentCommandService` accepted command/object/channel sets so active code no longer validates retired WeChat platform command objects; Baofu merchant-report/account owner values remain because they are active Baofu flows.

**Acceptance:** Runtime store interfaces no longer expose deleted WeChat platform operations unless intentionally retained as documented archival reads.

### Task 6: Track migration files and generated artifacts

**Files:**

- Add to git: intended `000235` and `000236` migration files.
- Regenerate: sqlc, mocks, Swagger as triggered by source changes.

**Steps:**

- [x] Run `make sqlc` after SQL changes.
- [x] Run `make swagger` after route/annotation changes.
- [x] Run `make check-generated`.
- [x] Review `git status --short --untracked-files=all` and identify unrelated untracked work separately.

**Validation notes:**

- `PATH="/usr/local/go/bin:$PATH" make sqlc` passed.
- `PATH="/usr/local/go/bin:$PATH" make swagger` passed.
- `PATH="/usr/local/go/bin:$PATH" make check-generated` passed and reported generated artifacts are in sync.
- `git status --short --untracked-files=all` still shows broad intended removal/regeneration work plus untracked migration/plan files; `.worktrees/feat-cancel-refund-progress/` and unrelated artifact files remain outside this repair scope.

**Acceptance:** Required migrations and generated artifacts are present in the working tree and generation checks pass.

### Task 7: Final validation and residual-risk report

**Steps:**

- [x] Run backend compile smoke: `go test ./... -run '^$'`.
- [x] Run targeted payment/callback tests affected by changes.
- [x] Run frontend validation for touched active apps.
- [x] Report any validation not run with concrete residual risk.

**Validation record:**

- `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestHandle(OrderSettlement|PaymentNotify|RefundNotify|MerchantTransferNotify)|TestCreatePaymentOrder|TestClosePaymentOrder' -count=1` passed.
- `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestPaymentCommandService|TestBaofu|Test.*Payment|Test.*Refund|Test.*ProfitSharing' -count=1` passed.
- `PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'Test.*Payment|Test.*Refund|Test.*ProfitSharing|Test.*Baofu' -count=1` passed.
- `PATH="/usr/local/go/bin:$PATH" go test ./cmd/wechat_doc_extract -run '^$' -count=1` passed.
- `PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc ./logic ./api -run '^$'` passed.
- `PATH="/usr/local/go/bin:$PATH" go test ./... -run '^$'` passed.
- `PATH="/usr/local/go/bin:$PATH" make check-baofu-contract` passed.
- `PATH="$HOME/.local/bin:$PATH" npm run compile` in `weapp/` passed.
- `git diff --check` passed.

**Residual risk:**

- `web/` was intentionally not modified or validated because the user marked it archived for this pass.
- `locallife/media/policy.go` still contains `CategoryMerchantCancelWithdrawMaterial = "merchant_cancel_withdraw"` for historical private media object keys. It is not an active API/query surface, and changing it could break access to already-uploaded private assets.
- The migration down path recreates empty legacy tables only; it cannot restore data dropped by `000235`.

**Acceptance:** The branch has a clear pass/fail validation record and no hidden completion claim.
