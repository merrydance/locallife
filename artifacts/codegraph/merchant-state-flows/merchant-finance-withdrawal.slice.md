# Merchant Finance Withdrawal Slice

Status: withdrawal idempotency, manager create permission, callback fact durability, provider-error command diagnostics, applied-schema idempotency hardening, provider/local balance truth-boundary proof, and Mini Program durable-detail terminal polling proof implemented 2026-06-10; Baofu account-opening provider failure reasons preserved 2026-06-07
Risk class: G3 - merchant money movement, Baofu settlement-account onboarding, provider callbacks, recovery schedulers, sensitive profile data, and finance authorization boundaries
Scope: Mini Program merchant finance pages -> merchant finance read APIs -> Baofu settlement account onboarding -> Baofu withdrawal create/read APIs -> provider callback/recovery fact application -> durable finance truth

## Variant Coverage

This slice covers:

- Merchant Mini Program finance entry, bills, settlement records, settlement-account status/submit, withdrawal list/create/detail, and the external cancel-withdraw entry.
- Merchant finance read APIs under `/v1/merchant/finance/**`.
- Merchant Baofu settlement-account GET/POST under `/v1/merchant/settlement-account`.
- Merchant Baofu withdrawal GET/list/create under `/v1/merchant/finance/baofu-withdrawal/**`.
- Baofu account-open callback, account-open recovery scheduler, merchant report recovery path, Baofu withdraw callback, and Baofu withdraw recovery scheduler.

This slice does not fully cover:

- Platform/operator/rider finance variants except where shared code materially affects the merchant path.
- Full Baofu aggregate payment/profit-sharing calculation internals before `profit_sharing_orders` are produced.
- Refund/recovery money movement outside merchant withdrawals; order refund is captured in `merchant-order-operations` and claim recovery in `merchant-claim-recovery`.
- WeChat direct payment internals for non-merchant verify-fee onboarding; merchant Baofu account onboarding currently does not support payment recovery in the Mini Program status page.

## Product Invariant

Merchant finance state must separate read-only financial reporting from money movement:

- Merchant finance summaries are derived from durable settlement facts such as `profit_sharing_orders` and `merchant_settlement_adjustments`; page presentation must not invent totals.
- Implemented 2026-06-10: finance reporting and withdrawable balance are intentionally separate truth sources. Local settlement facts are report truth, while Baofu `QueryBalance` is withdrawable truth; the two responses must not leak or substitute each other's fields.
- Merchant settlement-account onboarding is owner-only and must preserve sensitive profile fields encrypted or masked while keeping provider status recoverable.
- Implemented 2026-06-10: merchant withdrawal creation requires a client/server `Idempotency-Key`; same owner/key/amount replays the existing withdrawal, while same owner/key/different amount is rejected as a conflict before any provider call.
- Implemented 2026-06-10: withdrawal idempotency schema is hardened for clean installs and already-applied weak migration environments. `000260` defines key/hash columns, per-owner unique index, and the strong pair CHECK for clean databases; `000261` forward-repairs databases that had already applied a weaker `000260`, cleans partial historical pairs, and rejects future half-written key/hash rows.
- Implemented 2026-06-10: merchant managers may operate withdrawal create/list/detail/balance flows. Settlement-account submission remains owner-only unless a later finance policy changes it.
- Merchant withdrawal creation must create durable local withdrawal intent before/alongside provider command tracking.
- Implemented 2026-06-10: withdrawal provider callbacks first persist a durable `external_payment_facts` callback fact, then asynchronously enqueue terminal withdrawal-state application.
- Provider callback, query recovery, and task application must converge `baofu_withdrawal_orders` from `processing` to a terminal state without regressing already terminal rows.
- A merchant-visible "withdrawal submitted" result must mean there is a durable local withdrawal row the merchant can later inspect.
- Implemented 2026-06-10: Mini Program accepted/unknown withdrawal create results redirect with the backend-returned durable withdrawal id, the detail page reloads that id, polls non-terminal states while visible, and stops polling once terminal truth is observed.

## Primary Forward Chain

1. Merchant finance entry exposes order bills, settlement records, settlement account, and withdrawals.
   Evidence: `weapp/miniprogram/pages/merchant/_utils/merchant-finance-entry-view.ts:1`, `weapp/miniprogram/pages/merchant/_utils/merchant-finance-entry-view.ts:21`, `weapp/miniprogram/pages/merchant/_utils/merchant-finance-entry-view.ts:36`, `weapp/miniprogram/pages/merchant/_utils/merchant-finance-entry-view.ts:42`.

2. Finance bill/settlement pages use `merchant-finance-workflow` to build date ranges, load summary/list APIs, and preserve partial data when one request fails.
   Evidence: `weapp/miniprogram/pages/merchant/_services/merchant-finance-workflow.ts:1`, `weapp/miniprogram/pages/merchant/_services/merchant-finance-workflow.ts:181`, `weapp/miniprogram/pages/merchant/_services/merchant-finance-workflow.ts:208`, `weapp/miniprogram/pages/merchant/_services/merchant-finance-workflow.ts:262`, `weapp/miniprogram/pages/merchant/_services/merchant-finance-workflow.ts:283`.

3. Finance API wrappers call `/v1/merchant/finance/overview`, `/orders`, `/service-fees`, `/promotions`, `/daily`, `/settlements`, and `/settlement-timeline`.
   Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/merchant-finance.ts:149`, `weapp/miniprogram/pages/merchant/_main_shared/api/merchant-finance.ts:157`, `weapp/miniprogram/pages/merchant/_main_shared/api/merchant-finance.ts:167`, `weapp/miniprogram/pages/merchant/_main_shared/api/merchant-finance.ts:176`, `weapp/miniprogram/pages/merchant/_main_shared/api/merchant-finance.ts:185`, `weapp/miniprogram/pages/merchant/_main_shared/api/merchant-finance.ts:194`, `weapp/miniprogram/pages/merchant/_main_shared/api/merchant-finance.ts:204`.

4. Backend finance read routes use merchant staff middleware for owner/manager.
   Evidence: `locallife/api/server.go:1268`, `locallife/api/server.go:1271`, `locallife/api/server.go:1272`, `locallife/api/server.go:1273`, `locallife/api/server.go:1274`, `locallife/api/server.go:1275`, `locallife/api/server.go:1276`, `locallife/api/server.go:1277`.

5. Finance read handlers resolve the selected accessible merchant and read `profit_sharing_orders`, promotions, and settlement adjustments.
   Evidence: `locallife/api/merchant.go:70`, `locallife/api/permission_helpers.go:70`, `locallife/api/permission_helpers.go:102`, `locallife/api/merchant_finance.go:59`, `locallife/api/merchant_finance.go:79`, `locallife/api/merchant_finance.go:90`, `locallife/api/merchant_finance.go:101`, `locallife/api/merchant_finance.go:111`.

6. Finance SQL derives merchant order income and settlement records from `profit_sharing_orders`, with fee fields adapting to `baofu_fee_v2`.
   Evidence: `locallife/db/query/profit_sharing_order.sql:353`, `locallife/db/query/profit_sharing_order.sql:381`, `locallife/db/query/profit_sharing_order.sql:397`, `locallife/db/query/profit_sharing_order.sql:414`, `locallife/db/query/profit_sharing_order.sql:431`, `locallife/db/query/profit_sharing_order.sql:474`.

7. Settlement-account status page uses a merchant access guard and shared Baofu settlement-account behavior to load backend truth and long-poll waiting states.
   Evidence: `weapp/miniprogram/pages/merchant/finance/settlement-account/index.ts:14`, `weapp/miniprogram/pages/merchant/finance/settlement-account/index.ts:34`, `weapp/miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-status.ts:65`, `weapp/miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-status.ts:109`, `weapp/miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-status.ts:192`, `weapp/miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-status.ts:257`.

8. Settlement-account submit page loads masked/default profile data, keeps a local draft, validates enterprise bank/profile fields, then starts Baofu onboarding and polls status.
   Evidence: `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.ts:70`, `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.ts:94`, `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.ts:148`, `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.ts:155`, `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.ts:186`.

9. Settlement-account wrappers call `GET/POST /v1/merchant/settlement-account`.
   Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-account.ts:157`, `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-account.ts:170`, `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-account.ts:177`, `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-account.ts:193`, `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-account.ts:195`.

10. Backend merchant settlement-account routes use `MerchantOwnerOnlyMiddleware`, which deliberately skips active/region checks for onboarding but requires the current user to be the owner.
    Evidence: `locallife/api/server.go:710`, `locallife/api/server.go:711`, `locallife/api/server.go:713`, `locallife/api/server.go:716`, `locallife/api/server.go:717`, `locallife/api/server.go:719`, `locallife/api/rbac_middleware.go:139`, `locallife/api/rbac_middleware.go:162`.

11. Settlement-account GET composes binding, encrypted/masked profile, latest opening flow, payment data, and merchant-report readiness into one response. It can opportunistically recover provider-progress flows on read.
    Evidence: `locallife/api/baofu_settlement_account.go:28`, `locallife/api/baofu_settlement_account.go:235`, `locallife/api/baofu_settlement_account_read.go:12`, `locallife/api/baofu_settlement_account_read.go:23`, `locallife/api/baofu_settlement_account_read.go:34`, `locallife/api/baofu_settlement_account_read.go:46`, `locallife/api/baofu_settlement_account_read.go:51`, `locallife/api/baofu_settlement_account_read.go:81`, `locallife/api/baofu_settlement_account_read.go:204`.

12. Settlement-account POST decodes and rejects client-controlled provider fields, merges merchant defaults when applicable, then calls `StartOrRecoverOpening`.
    Evidence: `locallife/api/baofu_settlement_account.go:51`, `locallife/api/baofu_settlement_account.go:244`, `locallife/api/baofu_settlement_account.go:245`, `locallife/api/baofu_settlement_account.go:251`, `locallife/api/baofu_settlement_account.go:253`, `locallife/api/baofu_settlement_account.go:260`, `locallife/api/baofu_settlement_account.go:261`, `locallife/api/baofu_settlement_account.go:279`.

13. Baofu onboarding upserts encrypted profile truth, creates or reuses an opening flow, sends provider open request, creates/updates account binding, and applies active/failed/abnormal provider results.
    Evidence: `locallife/logic/baofu_account_onboarding_service.go:153`, `locallife/logic/baofu_account_onboarding_service.go:174`, `locallife/logic/baofu_account_onboarding_service.go:178`, `locallife/logic/baofu_account_onboarding_service.go:189`, `locallife/logic/baofu_account_onboarding_service.go:241`, `locallife/logic/baofu_account_onboarding_open.go:14`, `locallife/logic/baofu_account_onboarding_open.go:35`, `locallife/logic/baofu_account_onboarding_open.go:47`, `locallife/logic/baofu_account_onboarding_open.go:59`, `locallife/logic/baofu_account_onboarding_apply.go:26`.

14. Merchant active Baofu account is not enough for payment readiness; merchant report and applet authorization must also converge before settlement account status is ready.
    Evidence: `locallife/api/baofu_settlement_account_read.go:204`, `locallife/api/baofu_settlement_account_read.go:211`, `locallife/api/baofu_settlement_account_read.go:227`, `locallife/api/baofu_settlement_account_read.go:240`, `locallife/logic/baofu_account_onboarding_apply.go:51`, `locallife/logic/baofu_account_onboarding_apply.go:60`, `locallife/logic/baofu_account_onboarding_apply.go:65`.

15. Baofu account-open callback validates parser and provider identity, records a callback fact, resolves the opening flow, and applies callback state.
    Evidence: `locallife/api/server.go:533`, `locallife/api/baofu_callback.go:67`, `locallife/api/baofu_callback.go:80`, `locallife/api/baofu_callback.go:91`, `locallife/api/baofu_callback.go:96`, `locallife/api/baofu_account_open_callback_fact.go:18`, `locallife/api/baofu_account_open_callback_fact.go:57`, `locallife/api/baofu_account_open_callback_fact.go:64`, `locallife/api/baofu_account_open_callback_fact.go:204`.

16. Account-opening recovery scheduler runs every five minutes, queries recoverable opening/report/app-auth flows, and either recovers Baofu account status or merchant report/app-auth status.
    Evidence: `locallife/main.go:275`, `locallife/worker/baofu_account_opening_recovery_scheduler.go:19`, `locallife/worker/baofu_account_opening_recovery_scheduler.go:75`, `locallife/worker/baofu_account_opening_recovery_scheduler.go:132`, `locallife/worker/baofu_account_opening_recovery_scheduler.go:147`, `locallife/worker/baofu_account_opening_recovery_scheduler.go:150`, `locallife/worker/baofu_account_opening_recovery_scheduler.go:160`, `locallife/worker/baofu_account_opening_recovery_scheduler.go:171`.

17. Withdrawal list page loads balance and withdrawal records in parallel; if only balance fails, records can still render while create is disabled.
    Evidence: `weapp/miniprogram/pages/merchant/finance/withdrawals/index.ts:82`, `weapp/miniprogram/pages/merchant/finance/withdrawals/index.ts:143`, `weapp/miniprogram/pages/merchant/finance/withdrawals/index.ts:145`, `weapp/miniprogram/pages/merchant/finance/withdrawals/index.ts:155`, `weapp/miniprogram/pages/merchant/finance/withdrawals/index.ts:182`, `weapp/miniprogram/pages/merchant/finance/withdrawals/index.ts:190`.

18. Withdrawal create page loads balance, validates local amount against backend balance view, blocks duplicate submit locally, keeps a per-draft withdrawal idempotency key, passes that key on submit, and redirects to durable detail.
    Evidence: `weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.ts:17`, `weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.ts:30`, `weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.ts:80`, `weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.ts:98`, `weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.ts:101`, `weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.ts:102`.

19. Withdrawal wrappers call `GET /balance`, `GET /withdrawals`, `GET /withdrawals/:id`, and `POST /withdraw`; create wrappers require `idempotencyKey` and send it as the `Idempotency-Key` header. Shared operator/platform/rider wrapper copies were updated with the same contract to avoid role drift.
    Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-withdrawal.ts:72`, `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-withdrawal.ts:79`, `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-withdrawal.ts:94`, `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-withdrawal.ts:101`, `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-withdrawal.ts:106`, `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-withdrawal.ts:115`, `weapp/scripts/check-baofu-withdrawal-workflow.js:61`, `weapp/scripts/check-baofu-withdrawal-workflow.js:75`.

20. Backend withdrawal read/list/balance/create routes allow owner/manager through the merchant finance group. All routes derive owner scope from server-side merchant context.
    Evidence: `locallife/api/server.go:1304`, `locallife/api/server.go:1305`, `locallife/api/server.go:1314`, `locallife/api/server.go:1315`, `locallife/api/server.go:1316`, `locallife/api/server.go:1317`, `locallife/api/baofu_withdrawal.go:200`.

21. Withdrawal balance uses active Baofu binding and provider `QueryBalance`. It does not derive available balance from local finance tables, and the finance overview path does not call the provider balance endpoint.
    Evidence: `locallife/api/baofu_withdrawal.go:249`, `locallife/logic/baofu_withdraw_service.go:92`, `locallife/logic/baofu_withdraw_service.go:101`, `locallife/logic/baofu_withdraw_service.go:107`, `locallife/logic/baofu_withdraw_service.go:119`.

22. Withdrawal create validates amount upper bound, requires `Idempotency-Key`, replays same-key/same-request local orders, rejects same-key/different-amount conflicts, generates server-side out-request number, confirms active binding/fee-member id, queries provider balance, persists a `processing` local withdrawal order with idempotency key/hash, calls provider create, records external payment command, preserves provider-specific create-error code/message diagnostics internally, and returns created/accepted/unknown/rejected semantics.
    Evidence: `locallife/api/baofu_withdrawal.go:294`, `locallife/api/baofu_withdrawal.go:303`, `locallife/api/baofu_withdrawal.go:317`, `locallife/logic/baofu_withdraw_service.go:143`, `locallife/logic/baofu_withdraw_service.go:147`, `locallife/logic/baofu_withdraw_service.go:183`, `locallife/logic/baofu_withdraw_service.go:191`, `locallife/logic/baofu_withdraw_service.go:200`, `locallife/logic/baofu_withdraw_service.go:221`, `locallife/logic/baofu_withdraw_service.go:376`, `locallife/logic/baofu_withdraw_service.go:464`, `locallife/db/query/baofu_withdrawal_order.sql:1`, `locallife/db/query/baofu_withdrawal_order.sql:36`, `locallife/db/migration/000260_add_baofu_withdrawal_idempotency.up.sql:1`, `locallife/db/migration/000260_add_baofu_withdrawal_idempotency.up.sql:16`, `locallife/db/migration/000261_harden_baofu_withdrawal_idempotency_pair.up.sql:1`, `locallife/db/migration/000261_harden_baofu_withdrawal_idempotency_pair.up.sql:16`.

23. Withdrawal callback validates parser and payout identity, looks up the withdrawal by provider serial, persists a durable callback fact, and only then enqueues fact application. The callback does not directly mutate the withdrawal row.
    Evidence: `locallife/api/server.go:535`, `locallife/api/baofu_callback.go:113`, `locallife/api/baofu_callback.go:131`, `locallife/api/baofu_callback.go:142`, `locallife/api/baofu_callback.go:155`, `locallife/api/baofu_callback.go:161`, `locallife/api/baofu_callback.go:170`, `locallife/api/baofu_withdrawal_callback_fact.go:16`, `locallife/api/baofu_withdrawal_callback_fact.go:30`, `locallife/api/baofu_withdrawal_callback_fact.go:34`, `locallife/api/baofu_withdrawal_callback_fact.go:57`.

24. Withdrawal recovery scheduler runs every five minutes, scans processing withdrawals older than five minutes, queries provider by out request number, and enqueues the same fact-application task when provider result is terminal.
    Evidence: `locallife/main.go:276`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:17`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:61`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:109`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:122`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:128`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:142`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:151`.

25. Fact-application worker maps upstream state to local `succeeded/failed/returned`, updates only `processing` rows, and treats already-terminal rows as idempotent no-ops.
    Evidence: `locallife/worker/processor.go:288`, `locallife/worker/task_baofu_withdrawal_fact_application.go:17`, `locallife/worker/task_baofu_withdrawal_fact_application.go:47`, `locallife/worker/task_baofu_withdrawal_fact_application.go:55`, `locallife/worker/task_baofu_withdrawal_fact_application.go:59`, `locallife/worker/task_baofu_withdrawal_fact_application.go:62`, `locallife/worker/task_baofu_withdrawal_fact_application.go:71`, `locallife/worker/task_baofu_withdrawal_fact_application.go:80`.

## Reverse-Reference Findings

- Merchant finance read APIs are not money-movement writers; they are derived readers over `profit_sharing_orders`, promotion expense queries, and `merchant_settlement_adjustments`.
- Fixed 2026-06-10: managers may operate withdrawal flows. Merchant finance read and withdrawal create/list/detail/balance routes now share owner/manager middleware; settlement-account onboarding remains owner-only unless product changes it.
- Merchant settlement-account status and submit pages reuse shared role-agnostic Baofu onboarding behaviors. The merchant variant explicitly sets `supportPaymentRecovery=false`.
- Baofu account opening is multi-stage for merchants: account binding active -> merchant report processing -> applet authorization pending -> payment ready.
- Fixed 2026-06-07: Baofu account-opening/provider failures now preserve the provider failure code and sanitized provider reason for internal diagnosis. User-facing status can show sanitized provider reasons when they are safe and actionable; otherwise it falls back to classified guidance while internal diagnostics still retain the sanitized provider cause instead of silently replacing it with LocalLife stable copy.
- Fixed 2026-06-10: Baofu withdrawal balance and local finance reporting are proof-covered as separate truth sources. Provider balance is real withdrawable balance and does not locally reserve or freeze funds when creating a withdrawal; finance overview/timeline remain local settlement-reporting views and must not be treated as provider withdrawable balance.
- Fixed 2026-06-10: withdrawal create requires `Idempotency-Key`. `baofu_withdrawal_orders` stores `idempotency_key` plus `idempotency_request_hash`, has a per-owner partial unique index, and now has forward-hardened key/hash pair constraints for both clean and already-applied weak-schema databases; the Mini Program wrapper/page sends a stable draft key so ambiguous same-intent retries replay instead of creating another withdrawal order.
- Fixed 2026-06-10: `external_payment_commands` records withdrawal provider commands with internal Baofu diagnostics. Provider create errors preserve upstream code, classified/sanitized message, and sanitized diagnostic snapshot in command rows; user-facing create-unknown copy remains stable and non-sensitive. Withdrawal create idempotency now lives on `baofu_withdrawal_orders`, while command rows remain the provider-call audit trail.
- Withdrawal callback and recovery both converge through the same fact-application task, which is a good single write path for terminal status.
- Fixed 2026-06-10: withdrawal callbacks persist a durable `external_payment_facts` callback fact before asynchronous terminal application; if fact persistence fails, the callback returns failure and does not enqueue the application task.
- Fixed 2026-06-10: Mini Program withdrawal create/detail runtime proof covers duplicate-submit blocking, stable draft `Idempotency-Key`, durable-id redirect after accepted/unknown create response, detail reload by that id, visible-page polling for non-terminal rows, terminal status refresh, and polling cleanup.

## SQL And Durable State Boundaries

- `profit_sharing_orders`: primary finance report source for order income, fees, pending/finished settlement rows, and finance timeline inputs.
- `merchant_settlement_adjustments`: manual/platform settlement adjustments included in overview and timeline.
- `baofu_account_opening_profiles`: encrypted/masked settlement-account profile source, unique by owner type/id.
- `baofu_account_opening_flows`: onboarding state machine: `profile_pending`, `verify_fee_pending`, `verify_fee_processing`, `opening_processing`, `merchant_report_processing`, `applet_auth_pending`, `ready`, `failed`, `voided`.
- `baofu_account_bindings`: durable Baofu account binding/open-state source, unique by owner type/id; active binding includes `contract_no` and `sharing_mer_id` needed for withdrawals.
- `baofu_merchant_reports`: merchant-specific payment readiness/report/app-auth truth after Baofu account activation.
- `baofu_withdrawal_orders`: local withdrawal order truth with unique `out_request_no`, provider withdrawal number, amount, status, raw snapshot, finished timestamp, `idempotency_key`, and `idempotency_request_hash`.
- `external_payment_commands`: audit trail for Baofu withdrawal create commands, including sanitized provider create-error diagnostics for internal investigation.
- `external_payment_facts`: durable Baofu withdrawal callback facts before async terminal application.
- Baofu provider: external truth for account opening, merchant report/app-auth, account balance, withdrawal acceptance, and withdrawal terminal result.

## Trust, Authorization, And Tenant Checks

- Merchant finance read routes use `MerchantStaffMiddleware("owner", "manager")`; selected merchant can come from context, path/query/header selection, owner association, or active merchant staff association.
- Settlement-account read/write routes use `MerchantOwnerOnlyMiddleware`, requiring the current user to own the selected merchant but not requiring merchant active status/region, because onboarding may be needed before activation.
- Withdrawal read/list/balance/create routes use owner/manager staff middleware. Product decision 2026-06-10 says managers may operate withdrawal flows; settlement-account submission remains owner-only.
- Settlement-account POST controls owner type/id/account type/industry/provider request fields server-side and rejects client-controlled provider fields through `decodeBaofuSettlementAccountRequest`.
- Baofu account-open callback validates configured parser and collect merchant/terminal identity before applying state.
- Baofu withdrawal callback validates parser and payout merchant/terminal identity, requires provider serial number, and resolves a local withdrawal by server-generated out-request number.
- Withdrawal detail checks `owner_type` and `owner_id` before returning; cross-owner rows are returned as not found.
- Sensitive settlement profile defaults are returned as masks/flags; encrypted raw fields are used only server-side when building provider requests.

## Idempotency And Duplicate-Submit Checks

- Finance read pages are idempotent GETs.
- Settlement-account submit has frontend `submitting/syncing` guards. Backend `StartOrRecoverOpening` reuses active binding and active/latest opening flows where possible, and SQL state updates are conditional by state.
- Account-opening recovery, callback application, duplicate-account reconciliation, and failed-flow recovery all include explicit duplicate or mismatch safeguards.
- Withdrawal create has frontend `submitting` guard plus a stable draft `Idempotency-Key`. Backend stores the key/hash on `baofu_withdrawal_orders`, rejects half-written key/hash pairs at the DB boundary, replays same-key/same-amount requests without provider calls, rejects same-key/different-amount conflicts, and handles unique-conflict races by reloading the existing idempotency row.
- Withdrawal status update is conditionally `WHERE status='processing'`; repeated callback/recovery after terminal status is a no-op.
- Withdrawal recovery enqueue uses a 30-second unique task TTL, which reduces duplicate task bursts but is not a durable long-term idempotency key.

## Recovery And Async Convergence Paths

- Settlement-account status pages long-poll GET after submit/status refresh.
- Settlement-account GET can opportunistically recover `opening_processing` or selected failed flows when `baofuAccountClient` is configured.
- Baofu account-open callback records external payment fact and applies state.
- Account-opening recovery scheduler scans recoverable opening/report/app-auth states every five minutes.
- Payment fact application can continue account opening after verify-fee payment for roles that require it; merchant onboarding does not depend on verify-fee payment in the current Mini Program path.
- Withdrawal callback persists a durable callback fact first, then enqueue/applies asynchronously. Withdrawal recovery scheduler queries provider for old processing rows and enqueues the same terminal-application task.
- Withdrawal detail page polls non-terminal rows while visible and stops once terminal status is observed. Withdrawal list still has no realtime subscription; list recovery is refresh/re-entry.
- There is no discovered local wallet/freeze reconciliation path for merchant withdrawals because available balance is provider-owned.

## Frontend Draft And Backend Rehydration

- Finance bill/settlement pages are read-only views; they build ViewState from backend responses and show partial/unavailable summaries when only secondary requests fail.
- Settlement-account status page always uses backend response as truth and can auto-redirect to submit when profile is pending.
- Settlement-account submit page drafts enterprise/profile fields from backend defaults/masks, validates locally, then relies on `startBaofuAccountOnboarding` and subsequent polling for backend truth.
- Withdrawal list page uses provider balance response to decide whether create is allowed; when balance is unavailable, it still shows records but blocks create.
- Withdrawal create page keeps a local per-draft idempotency key, refreshes it when the amount changes, and reuses it for retry while the draft is unchanged. After successful/accepted/unknown response with a durable ID, it redirects to detail.
- Withdrawal detail page reloads the durable row by id, polls non-terminal rows while visible, and stops polling after terminal convergence. Withdrawal list terminal convergence still depends on refresh/re-entry.

## Test Coverage Signals

Observed tests:

- `locallife/api/merchant_finance_test.go` covers merchant finance overview response fields and invalid date handling for owner.
- `locallife/api/baofu_settlement_account_test.go` covers merchant owner read/write, manager denial, safe masks/defaults, provider failures, sanitized provider failure-reason display, active binding readiness, applet auth, and many request-validation branches.
- `locallife/logic/baofu_account_onboarding_service_test.go` covers onboarding profile validation, provider errors, duplicate/recovery branches, merchant report continuation, failed-flow recovery, provider failure-reason persistence, and sensitive snapshot behavior.
- `locallife/worker/baofu_account_opening_recovery_scheduler_test.go` covers account-opening recovery, merchant-report recovery, failed/processing/noop branches, and safe logging.
- `locallife/api/baofu_withdrawal_contract_test.go` covers withdrawal balance owner scope, pagination/status text, cross-owner detail denial, required idempotency header, same-key replay, conflicting same-key rejection, manager create permission, invalid amount/provider errors, missing fee member, insufficient balance, accepted/rejected/unknown create branches, provider request fields, and the finance-overview versus Baofu-balance truth-source boundary.
- `locallife/logic/baofu_withdraw_service_test.go` covers provider balance gating, active binding/fee-member checks, idempotent replay/conflict behavior, command logging, merchant-funds owner mapping, rejected acceptance, unknown result handling, and provider create-error diagnostics preserved in the command ledger.
- `locallife/db/sqlc/baofu_withdrawal_order_test.go` covers withdrawal idempotency uniqueness per owner, terminal-status non-regression, and DB rejection of partial idempotency key/hash pairs.
- `locallife/api/baofu_callback_test.go` covers Baofu withdrawal callback parse/identity, durable fact persistence before enqueue/ACK, and no enqueue when fact persistence fails.
- `locallife/worker/baofu_withdrawal_recovery_scheduler_test.go` and `task_baofu_withdrawal_fact_application_test.go` cover provider query enqueue and terminal fact application.
- `weapp/scripts/check-baofu-withdrawal-workflow.js` covers Mini Program withdrawal API wrapper scope, required client idempotency key, shared role route registration, merchant create-page duplicate-submit blocking, durable-id redirect after accepted/unknown create response, and merchant detail-page durable reload plus terminal polling cleanup.

Missing high-value tests:

- Manager finance read test across all finance list/detail/balance routes, including selected-merchant header behavior.
- End-to-end withdrawal ambiguity test from POST timeout/provider unknown -> recovery query -> terminal detail/list update.
- Cross-check finance overview/timeline totals against settlement adjustments and Baofu v2 fee fields after refunds/returns.

## Gaps And Refactor Notes

- Fixed 2026-06-10: merchant withdrawal create has a real client/server `Idempotency-Key`; duplicate POSTs for the same unchanged intent replay and conflicting reuse is rejected.
- Fixed 2026-06-10: already-applied weak withdrawal idempotency CHECK constraints are forward-hardened by `000261`, so new partial key/hash rows are rejected even in databases that had already run the earlier migration version.
- Fixed 2026-06-10: Baofu withdrawal provider create errors now keep provider code and sanitized diagnostic detail in `external_payment_commands`; frontend create-unknown guidance remains stable and non-sensitive.
- Fixed 2026-06-10: managers may operate withdrawal create/list/detail/balance while settlement-account onboarding stays owner-only.
- Fixed 2026-06-10: withdrawal callbacks persist durable callback facts before asynchronous fact application, matching the account-open/payment/refund callback audit model.
- Fixed 2026-06-10: provider withdrawable balance versus local finance-report reconciliation is documented as an intentional split truth boundary and covered by `TestMerchantFinanceOverviewAndBaofuWithdrawalBalanceUseSeparateTruthSources`.
- Fixed 2026-06-10: Mini Program withdrawal accepted/unknown create response -> durable detail redirect -> non-terminal polling -> terminal refresh behavior is covered by `check:baofu-withdrawal-workflow`.
- Clarify product copy and backend behavior for provider `returned`: whether it should trigger any merchant-facing balance refresh hint, alert, or manual handling.
- Add Mini Program state recovery coverage for settlement-account submit/status long-wait flows.

## Branch Exhaustion

- Entry branches checked: Mini Program finance entry, bills, settlements, settlement-account status, settlement-account submit, withdrawal list, withdrawal create, withdrawal detail, external cancel-withdraw detail link, dashboard Baofoo readiness precheck, and shared role Baofu onboarding behaviors. Flutter App search found no settlement, withdrawal, or finance write entry in `merchant_app/lib/features/**`; App is out of this flow except for shared merchant status readiness depending on settlement truth. Web is intentionally out of current scope.
- Request branches checked: read-only merchant finance APIs, settlement-account `GET/POST`, withdrawal `balance/list/detail/create`, Baofu account wrappers, Baofu withdrawal wrappers, and shared role wrapper variants. Operator/platform/rider variants are noted only as shared-code drift risks because the current audit scope is merchant Mini Program plus App.
- Backend state branches checked: finance reads over `profit_sharing_orders` and `merchant_settlement_adjustments`; settlement onboarding profile defaults, masked fields, encrypted fields, active binding reuse, latest opening flow reuse, provider opening states, merchant report states, applet authorization states, and failed-flow recovery; withdrawal active binding lookup, fee-member validation, provider balance query, provider accepted/rejected/unknown responses, local `processing` row creation, command logging, callback lookup, and terminal fact application.
- Async branches checked: account-open callback fact persistence, account-opening recovery scheduler, merchant report/app-auth recovery, withdrawal callback enqueue, withdrawal recovery scheduler, fact-application worker, Mini Program settlement-account long-poll/refresh recovery, and Mini Program withdrawal detail polling until terminal status. No realtime withdrawal subscription was found.
- Failure/retry branches checked: frontend duplicate-submit guards, onboarding reuse/recover behavior, Baofu account-opening provider failure-reason persistence/display with sanitization boundaries, provider unavailable balance, withdrawal unknown result with durable id, withdrawal callback after terminal status, 30-second task de-dupe, and ambiguous create retry. Fixed 2026-06-10: withdrawal create now has durable client/server idempotency and Mini Program page-level durable-detail terminal-refresh proof.
- Reader/consumer branches checked: finance reports, dashboard settlement readiness, manual open-status readiness, withdrawal list/detail, settlement-account status, and provider balance versus local finance totals. Fixed 2026-06-10: the intentionally split boundary is proof-covered; provider balance is withdrawable truth and local finance tables are reporting truth.
- Authorization/tenant branches checked: owner/manager finance reads, owner-only settlement-account submission, owner/manager withdrawal reads and creates, selected merchant resolution, callback provider identity checks, and cross-owner withdrawal detail denial.
- Zombie/unreachable branches checked: shared Baofu verify-fee payment recovery exists but merchant settlement status config disables payment recovery; role-agnostic wrappers exist beyond merchant scope; App finance/withdrawal entries were not found; no local wallet/freeze repair path was found because merchant withdrawal balance is provider-owned.
- Test-proof gaps checked: backend coverage is broad for settlement onboarding, withdrawal create/callback/recovery, required withdrawal `Idempotency-Key` semantics, callback durable fact parity, manager create permission, finance overview, and finance-report-vs-provider-balance boundary proof. Mini Program withdrawal page-level unknown-result recovery is now proof-covered; remaining Mini Program proof gap is settlement-account submit/status long-wait runtime behavior plus broader end-to-end provider-timeout recovery.
