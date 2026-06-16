# Operator Finance And Baofu Withdrawal Slice

Status: operator-state flow slice created 2026-06-14
Risk class: G3 boundary - operator finance reads rely on profit-sharing money records, while Baofu settlement account and withdrawal paths touch external provider contracts, identity/bank data, idempotency, callbacks, workers, and recovery schedulers
Scope: Mini Program operator finance pages -> operator finance/commission APIs -> Baofu settlement account status/profile workflow -> Baofu withdrawal balance/list/create/detail -> provider callbacks -> command/fact workers -> SQL tables and legacy withdrawal boundary

## Variant Coverage

This slice covers:

- Operator finance overview page, commission preview, pull refresh, and navigation to bills, settlement account, and withdrawals.
- Operator commission bill page, date range selection, 365-day validation, pagination, refresh, and backend commission query.
- Operator Baofu withdrawal list, balance/records partial failure handling, create-entry gating, create page validation, frontend idempotency key, detail page polling, and terminal-state stop.
- Shared Mini Program Baofu withdrawal API and workflow mapping for operator role.
- Operator Baofu settlement account status page, submit page, personal-profile validation/defaults, verify-fee payment recovery, polling, and pending workflow context.
- Backend operator finance overview, commission, profit-sharing config route boundary, settlement account read/write, withdrawal balance/list/detail/create, and Baofu account/withdraw callbacks.
- Baofu withdrawal durable submission, async command dispatch, callback fact recording, recovery scheduler query, fact application, and terminal state updates.
- SQL state boundaries for profit sharing, Baofu account bindings/opening flows/profiles, withdrawal orders, external payment commands/facts/applications, and the older `withdrawal_records` table as a non-current legacy candidate.

This slice does not fully cover:

- Merchant, rider, and platform Baofu settlement/withdrawal UX except where shared code is reused and operator role routing is explicit.
- WeChat direct-payment internals for the Baofu account verify fee; this slice treats verify-fee payment as a boundary exposed through `payment_orders`/pay params.
- Provider-side Baofoo/Baofu official documentation beyond the current repository matrices and contract map.
- Real-money positive C4 evidence. The domain standard still requires explicit funds-action approval, funding, and masked sandbox/production evidence before claiming real withdrawal execution is fully verified.

## Product Invariant

Operator finance has two different truth models:

- Finance overview and commission bills are read models over successful profit-sharing records. They do not create settlement or withdrawal state.
- Baofu settlement account and withdrawal are durable state machines. The Mini Program can submit profile or withdrawal intent, but final account/withdrawal truth comes from persisted backend state, provider callbacks, worker dispatch, and recovery queries.

For withdrawals, the operator page must never be treated as the source of final status. The page creates a client idempotency key and shows polling, while the backend persists `baofu_withdrawal_orders` and `external_payment_commands`, dispatches the actual provider request from a worker, records callback facts, and updates terminal order state only through callback/query fact application.

## Primary Forward Chain

1. Operator Mini Program declares seven finance entrypoints: overview/withdraw landing, bills, withdrawals list/create/detail, settlement-account status, and settlement-account submit.
   Evidence: `weapp/miniprogram/app.json:52`, `weapp/miniprogram/app.json:53`, `weapp/miniprogram/app.json:54`, `weapp/miniprogram/app.json:55`, `weapp/miniprogram/app.json:56`, `weapp/miniprogram/app.json:57`, `weapp/miniprogram/app.json:58`.

2. Finance overview page loads finance overview and commission preview on entry/pull refresh, then navigates to settlement account, bills, or withdrawals.
   Evidence: `weapp/miniprogram/pages/operator/finance/withdraw/index.ts:24`, `weapp/miniprogram/pages/operator/finance/withdraw/index.ts:38`, `weapp/miniprogram/pages/operator/finance/withdraw/index.ts:69`, `weapp/miniprogram/pages/operator/finance/withdraw/index.ts:73`, `weapp/miniprogram/pages/operator/finance/withdraw/index.ts:77`.

3. Operator finance service loads overview and commission preview concurrently, tolerates partial failure by filling the failed section with a user-facing error, and builds bill pages from the commission endpoint.
   Evidence: `weapp/miniprogram/pages/operator/_services/operator-finance.ts:200`, `weapp/miniprogram/pages/operator/_services/operator-finance.ts:201`, `weapp/miniprogram/pages/operator/_services/operator-finance.ts:213`, `weapp/miniprogram/pages/operator/_services/operator-finance.ts:219`, `weapp/miniprogram/pages/operator/_services/operator-finance.ts:227`.

4. Operator API wrapper sends overview to `/v1/operators/me/finance/overview` and commission to `/v1/operators/me/commission`; current overview/bill pages do not pass a `region_id`.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:258`, `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:264`, `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:275`, `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:280`.

5. Backend operator finance routes are under `/v1/operators/me`, protected by operator Casbin role and `LoadOperatorMiddleware`.
   Evidence: `locallife/api/server.go:1411`, `locallife/api/server.go:1413`, `locallife/api/server.go:1415`, `locallife/api/server.go:1420`, `locallife/api/server.go:1423`.

6. Finance overview resolves all managed regions when no `region_id` is supplied, loops region ids for current-month and all-time stats, and computes the operator share ratio from finished profit-sharing totals.
   Evidence: `locallife/api/operator_stats.go:686`, `locallife/api/operator_stats.go:693`, `locallife/api/operator_stats.go:699`, `locallife/api/operator_stats.go:718`, `locallife/api/operator_stats.go:736`, `locallife/api/operator_stats.go:756`, `locallife/api/operator_stats.go:772`, `locallife/api/operator_stats.go:787`.

7. Commission bills use the single-region helper, parse a date range capped at 365 days, then read daily trend rows for one resolved region.
   Evidence: `locallife/api/operator_stats.go:843`, `locallife/api/operator_stats.go:850`, `locallife/api/operator_stats.go:857`, `locallife/api/operator_stats.go:873`, `locallife/db/query/operator_stats.sql:73`.

8. `resolveOperatorRegionSelection` supports aggregate all-managed-region reads when `region_id` is absent; `getOperatorRegionID` requires an explicit region or a single/primary fallback and fails when multiple regions require disambiguation.
   Evidence: `locallife/api/delivery_fee.go:72`, `locallife/api/delivery_fee.go:100`, `locallife/api/delivery_fee.go:181`, `locallife/api/delivery_fee.go:193`, `locallife/api/delivery_fee.go:202`.

9. Profit-sharing config route is registered for operators and uses the single-region helper, but no current operator Mini Program caller was found under `weapp/miniprogram/pages/operator`.
   Evidence: `locallife/api/server.go:1423`, `locallife/api/operator_profit_sharing_config.go:42`, `locallife/api/operator_profit_sharing_config.go:55`, `locallife/api/operator_profit_sharing_config.go:61`.

10. Withdrawals list page loads balance and records in parallel, keeps partial state when only one request fails, and blocks create navigation when balance is unavailable or `canSubmit` is false.
    Evidence: `weapp/miniprogram/pages/operator/finance/withdrawals/index.ts:57`, `weapp/miniprogram/pages/operator/finance/withdrawals/index.ts:82`, `weapp/miniprogram/pages/operator/finance/withdrawals/index.ts:143`, `weapp/miniprogram/pages/operator/finance/withdrawals/index.ts:155`, `weapp/miniprogram/pages/operator/finance/withdrawals/index.ts:182`, `weapp/miniprogram/pages/operator/finance/withdrawals/index.ts:190`.

11. Withdrawal create page generates a client idempotency key, refreshes that key when the amount changes, validates amount against backend balance view, and calls create with `Idempotency-Key`.
    Evidence: `weapp/miniprogram/pages/operator/finance/withdrawals/create/index.ts:17`, `weapp/miniprogram/pages/operator/finance/withdrawals/create/index.ts:30`, `weapp/miniprogram/pages/operator/finance/withdrawals/create/index.ts:80`, `weapp/miniprogram/pages/operator/finance/withdrawals/create/index.ts:92`, `weapp/miniprogram/pages/operator/finance/withdrawals/create/index.ts:98`, `weapp/miniprogram/pages/operator/finance/withdrawals/create/index.ts:101`.

12. Withdrawal detail page loads by id, rehydrates from backend detail, and polls every 15 seconds only while the returned status view is non-terminal.
    Evidence: `weapp/miniprogram/pages/operator/finance/withdrawals/detail/index.ts:29`, `weapp/miniprogram/pages/operator/finance/withdrawals/detail/index.ts:88`, `weapp/miniprogram/pages/operator/finance/withdrawals/detail/index.ts:119`, `weapp/miniprogram/pages/operator/finance/withdrawals/detail/index.ts:128`.

13. Shared Mini Program withdrawal API maps operator role to `/v1/operators/me/finance/baofu-withdrawal` and refuses to send a create request without an idempotency key.
    Evidence: `weapp/miniprogram/pages/operator/_main_shared/api/baofu-withdrawal.ts:63`, `weapp/miniprogram/pages/operator/_main_shared/api/baofu-withdrawal.ts:69`, `weapp/miniprogram/pages/operator/_main_shared/api/baofu-withdrawal.ts:101`, `weapp/miniprogram/pages/operator/_main_shared/api/baofu-withdrawal.ts:106`, `weapp/miniprogram/pages/operator/_main_shared/api/baofu-withdrawal.ts:110`.

14. Backend operator withdrawal handlers use owner scope `operator.ID` plus owner type `operator`, so list/detail/create are tenant-scoped to the current loaded operator.
    Evidence: `locallife/api/baofu_withdrawal.go:131`, `locallife/api/baofu_withdrawal.go:140`, `locallife/api/baofu_withdrawal.go:149`, `locallife/api/baofu_withdrawal.go:158`, `locallife/api/baofu_withdrawal.go:226`, `locallife/api/baofu_withdrawal.go:231`.

15. Withdrawal create validates service availability, permission, body, amount upper bound, required/length-bounded `Idempotency-Key`, then calls `BaofuWithdrawService.CreateWithdrawal`; a replay returns HTTP 200 while a new accepted submission returns HTTP 201.
    Evidence: `locallife/api/baofu_withdrawal.go:278`, `locallife/api/baofu_withdrawal.go:286`, `locallife/api/baofu_withdrawal.go:291`, `locallife/api/baofu_withdrawal.go:295`, `locallife/api/baofu_withdrawal.go:304`, `locallife/api/baofu_withdrawal.go:318`.

16. The withdrawal service checks idempotency before creating a new order, requires active binding/contract/fee member, verifies provider balance, then persists a processing withdrawal order plus a submitted async command in one transaction.
    Evidence: `locallife/logic/baofu_withdraw_service.go:124`, `locallife/logic/baofu_withdraw_service.go:140`, `locallife/logic/baofu_withdraw_service.go:141`, `locallife/logic/baofu_withdraw_service.go:152`, `locallife/logic/baofu_withdraw_service.go:156`, `locallife/logic/baofu_withdraw_service.go:160`, `locallife/logic/baofu_withdraw_service.go:175`, `locallife/db/sqlc/tx_baofu_withdrawal.go:23`, `locallife/db/sqlc/tx_baofu_withdrawal.go:37`, `locallife/db/sqlc/tx_baofu_withdrawal.go:61`.

17. The create handler does not directly execute the provider withdrawal. The async worker claims submitted commands, builds the provider request from the active binding and order, calls `CreateWithdraw`, and records accepted, rejected, or unknown outcomes.
    Evidence: `locallife/worker/task_baofu_withdrawal_command_dispatch.go:74`, `locallife/worker/task_baofu_withdrawal_command_dispatch.go:90`, `locallife/worker/task_baofu_withdrawal_command_dispatch.go:118`, `locallife/worker/task_baofu_withdrawal_command_dispatch.go:125`, `locallife/worker/task_baofu_withdrawal_command_dispatch.go:130`, `locallife/worker/task_baofu_withdrawal_command_dispatch.go:144`, `locallife/worker/task_baofu_withdrawal_command_dispatch.go:163`, `locallife/worker/task_baofu_withdrawal_command_dispatch.go:186`, `locallife/worker/task_baofu_withdrawal_command_dispatch.go:205`.

18. Baofu withdrawal callback parses and verifies the encrypted notification, checks payout identity, loads the local withdrawal by `transSerialNo`, records an external payment fact, enqueues fact application, and ACKs with uppercase `OK`.
    Evidence: `locallife/api/server.go:554`, `locallife/api/baofu_callback.go:115`, `locallife/api/baofu_callback.go:132`, `locallife/api/baofu_callback.go:144`, `locallife/api/baofu_callback.go:149`, `locallife/api/baofu_callback.go:155`, `locallife/api/baofu_callback.go:161`, `locallife/api/baofu_callback.go:170`, `locallife/api/baofu_callback.go:186`, `locallife/baofu/account/notification/notification.go:317`.

19. Callback fact recording maps upstream state to terminal/non-terminal payment fact semantics and stores a dedupe key tied to out request number and provider withdraw number/upstream state.
    Evidence: `locallife/api/baofu_withdrawal_callback_fact.go:15`, `locallife/api/baofu_withdrawal_callback_fact.go:23`, `locallife/api/baofu_withdrawal_callback_fact.go:29`, `locallife/api/baofu_withdrawal_callback_fact.go:57`, `locallife/api/baofu_withdrawal_callback_fact.go:68`, `locallife/api/baofu_withdrawal_callback_fact.go:77`.

20. Withdrawal fact application updates only non-terminal processing orders. Recovery runs every five minutes, re-enqueues submitted commands, queries processing orders using the original order creation date as `tradeTime`, and enqueues fact application only for non-processing provider results.
    Evidence: `locallife/worker/task_baofu_withdrawal_fact_application.go:47`, `locallife/worker/task_baofu_withdrawal_fact_application.go:59`, `locallife/worker/task_baofu_withdrawal_fact_application.go:62`, `locallife/worker/task_baofu_withdrawal_fact_application.go:71`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:17`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:109`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:123`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:151`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:165`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:174`.

21. Provider contract mapping treats withdrawal synchronous `state=1/2` as acceptance/processing, not final success, while query/callback states map `1/0/2/3` to succeeded/failed/processing/returned.
    Evidence: `.github/standards/domains/baofu-payment/CAPABILITY_GROUP_INDEX.md:78`, `.github/standards/domains/baofu-payment/CAPABILITY_GROUP_INDEX.md:98`, `.github/standards/domains/baofu-payment/CAPABILITY_GROUP_INDEX.md:99`, `locallife/baofu/account/contracts/types.go:234`, `locallife/baofu/account/contracts/types.go:249`, `locallife/baofu/account/client.go:521`, `locallife/baofu/account/client.go:538`, `locallife/baofu/account/notification/notification.go:301`.

22. Settlement-account status page injects operator role into the shared Baofu settlement status behavior and enables payment recovery.
    Evidence: `weapp/miniprogram/pages/operator/finance/settlement-account/index.ts:4`, `weapp/miniprogram/pages/operator/finance/settlement-account/index.ts:7`, `weapp/miniprogram/pages/operator/finance/settlement-account/index.ts:10`, `weapp/miniprogram/pages/operator/_main_shared/behaviors/baofu-settlement-status.ts:54`, `weapp/miniprogram/pages/operator/_main_shared/behaviors/baofu-settlement-status.ts:66`.

23. Settlement submit page injects operator role, loads defaults, validates personal profile, and submits through `startBaofuAccountOnboarding` with operator role.
    Evidence: `weapp/miniprogram/pages/operator/finance/settlement-account/submit/index.ts:31`, `weapp/miniprogram/pages/operator/finance/settlement-account/submit/index.ts:48`, `weapp/miniprogram/pages/operator/finance/settlement-account/submit/index.ts:83`, `weapp/miniprogram/pages/operator/finance/settlement-account/submit/index.ts:114`.

24. Shared account API maps operator to `/v1/operators/me/settlement-account`; onboarding handles payment, polling, start/resume, continue payment, and pending-context cleanup.
    Evidence: `weapp/miniprogram/pages/operator/_main_shared/api/baofu-account.ts:163`, `weapp/miniprogram/pages/operator/_main_shared/api/baofu-account.ts:167`, `weapp/miniprogram/pages/operator/_main_shared/api/baofu-account.ts:204`, `weapp/miniprogram/pages/operator/_main_shared/services/baofu-account-onboarding.ts:444`, `weapp/miniprogram/pages/operator/_main_shared/services/baofu-account-onboarding.ts:491`, `weapp/miniprogram/pages/operator/_main_shared/services/baofu-account-onboarding.ts:591`, `weapp/miniprogram/pages/operator/_main_shared/services/baofu-account-onboarding.ts:640`, `weapp/miniprogram/pages/operator/_main_shared/services/baofu-account-onboarding.ts:669`, `weapp/miniprogram/pages/operator/_main_shared/services/baofu-account-onboarding.ts:752`.

25. Backend operator settlement-account scope is personal account type and explicitly does not include merchant report/applet binding; non-merchant active bindings mark payment-ready directly.
    Evidence: `locallife/api/baofu_settlement_account.go:120`, `locallife/api/baofu_settlement_account.go:131`, `locallife/api/baofu_settlement_account.go:137`, `locallife/api/baofu_settlement_account.go:148`, `locallife/api/baofu_settlement_account.go:162`, `locallife/api/baofu_settlement_account.go:168`, `locallife/api/baofu_settlement_account_read.go:174`, `locallife/api/baofu_settlement_account_read.go:175`.

26. Backend settlement-account create decodes only allowed fields, rejects non-merchant `account_opening_mode`, merges missing defaults for non-merchant profiles, prepares opening through the onboarding service, optionally enqueues the account-opening worker, and returns status/pay params from backend truth.
    Evidence: `locallife/api/baofu_settlement_account.go:245`, `locallife/api/baofu_settlement_account.go:251`, `locallife/api/baofu_settlement_account.go:257`, `locallife/api/baofu_settlement_account.go:259`, `locallife/api/baofu_settlement_account.go:269`, `locallife/api/baofu_settlement_account.go:289`, `locallife/api/baofu_settlement_account.go:295`, `locallife/api/baofu_settlement_account.go:309`.

27. Baofu account onboarding uses active-binding short-circuit, existing flow recovery, user verify-fee flow for operator/rider, prepared async opening, callback/recovery convergence, and a five-minute recovery scheduler for recoverable opening flows.
    Evidence: `locallife/logic/baofu_account_onboarding_service.go:169`, `locallife/logic/baofu_account_onboarding_service.go:191`, `locallife/logic/baofu_account_onboarding_service.go:214`, `locallife/logic/baofu_account_onboarding_service.go:242`, `locallife/logic/baofu_account_onboarding_service.go:263`, `locallife/logic/baofu_account_onboarding_service.go:297`, `locallife/logic/baofu_account_onboarding_service.go:382`, `locallife/worker/task_baofu_account_opening.go:67`, `locallife/worker/baofu_account_opening_recovery_scheduler.go:114`, `locallife/worker/baofu_account_opening_recovery_scheduler.go:132`.

## SQL And Durable State Boundaries

- `profit_sharing_orders`: finance overview and commission source, filtered to `status='finished'`.
- `merchants`, `payment_orders`, `regions`: joined by finance stats and daily trends to derive regional GMV, commission, orders, active users, and merchant dimensions.
- `operators`, `operator_regions`: server-side authority for operator region selection.
- `baofu_account_bindings`: current Baofu account owner binding, contract number, sharing member id, open state, and masked card fields.
- `baofu_account_opening_profiles`: stored encrypted/masked opening profile and profile completeness state.
- `baofu_account_opening_flows`: durable account-opening state machine, verify-fee order linkage, provider request snapshot, failure codes, and recovery eligibility.
- `payment_orders`: verify-fee payment boundary for operator/rider account opening; direct payment internals are outside this slice.
- `baofu_withdrawal_orders`: current withdrawal source of truth for operator list/detail/status.
- `external_payment_commands`: async provider command outbox for create-withdraw dispatch; `dispatch_mode=async_worker` marks the current path.
- `external_payment_facts`: callback/query fact ledger for provider-observed withdrawal/account states.
- `external_payment_fact_applications`: retryable fact application boundary for provider facts.
- `withdrawal_records`: older generic withdrawal table in `operator_finance.sql`; it is not the current operator Mini Program Baofu withdrawal path and should remain a legacy/non-current candidate unless a caller is proven.

## Trust, Authorization, And Tenant Checks

- `/v1/operators/me` finance routes use operator role middleware and loaded operator context.
- Finance overview supports all managed regions when no explicit `region_id` is passed.
- Commission and profit-sharing config use single-region `getOperatorRegionID`; this is stricter than overview and can fail for multi-region operators when no region is selected.
- Withdrawal owner scope uses backend-loaded `operator.ID`, not a client-supplied operator id.
- Withdrawal detail returns 404 if a record exists but owner type/id does not match the current operator scope.
- Settlement account owner type/id/account type are server-controlled. Operator account opening is personal and cannot set merchant-only `account_opening_mode`.
- Baofu callbacks are unauthenticated externally triggered routes, but they parse/decrypt/verify provider payloads and validate configured collect/payout identity before persisting facts or ACKing.
- Frontend-visible settlement and withdrawal responses expose masked/sanitized state only; raw provider payloads remain in backend internal snapshots/facts.

## Idempotency And Duplicate-Submit Checks

- Finance overview, commission, balance, list, detail, and settlement status reads are idempotent.
- Withdrawal create uses a client idempotency key and backend `Idempotency-Key` requirement. The backend stores idempotency key plus request hash per owner.
- Reusing the same idempotency key with the same owner/amount replays the existing order; reusing it with a different request hash returns conflict.
- The create transaction creates a withdrawal order and submitted command atomically. The provider call occurs later, so duplicate taps do not directly duplicate provider withdrawals.
- External payment commands are unique by provider/channel/capability/command/external object. Worker claim and recovery can safely retry submitted/unknown command states.
- Callback facts use a dedupe key by out request number and provider withdraw number/upstream state. Fact application updates only non-terminal processing orders.
- Account-opening worker uses unique task TTL and durable flow state; recovery queries existing flow/binding instead of creating a second owner binding.

## Recovery And Async Convergence Paths

- Finance pages rehydrate from backend reads on load, pull refresh, range change, pagination, and detail polling.
- Withdrawal detail polling is UI rehydration only. The actual convergence path is worker dispatch, provider callback, recovery query, and fact application.
- Withdrawal command dispatch records unknown outcomes without mutating the order, allowing recovery to query and converge later.
- Withdrawal recovery runs every five minutes, also re-enqueues submitted async commands, and uses `order.CreatedAt.Format("2006-01-02")` as provider `tradeTime`.
- Account-opening status supports long wait, payment recovery, start/resume, and polling. Backend recovery runs every five minutes for opening/merchant-report/app-auth states, but operator account opening does not continue into merchant-report/app-auth.
- Baofu account callback ACKs `OK` only after a callback fact is persisted; withdrawal callback ACKs `OK` after fact persistence and fact-application enqueue.

## Frontend Draft And Backend Rehydration

- Finance overview partial errors are local view projections; backend truth is fetched again on refresh.
- Commission bill date range is frontend-validated for UX and backend-parsed again before querying.
- Withdrawal amount input is local draft; backend rechecks max amount, active account, fee member id, provider balance, idempotency, and persisted command creation.
- Create page does not mark the withdrawal terminal; it redirects to detail for backend rehydration.
- Settlement-account form uses operator profile defaults and masks, but backend reconstructs/merges missing defaults and validates the owner/account constraints again.
- Pending payment/onboarding context is a UX recovery helper; backend flow/payment/order state remains authoritative.

## Test Coverage Signals

Observed tests:

- `locallife/api/baofu_withdrawal_contract_test.go` covers balance/list/detail owner boundaries, required idempotency key, same-key replay, conflicting key rejection, invalid amount, provider balance failure, missing fee member, insufficient balance, submitted command persistence, and deferred provider acceptance/rejection/unknown behavior.
- `locallife/logic/baofu_withdraw_service_test.go` covers withdrawal service idempotency, request hash conflicts, balance checks, missing fee member, async command submission, and business-owner mapping.
- `locallife/worker/task_baofu_withdrawal_command_dispatch_test.go` covers accepted/rejected/unknown command outcomes, no command outcome before order update, repair after order update, provider error unknown state, and non-async-worker rejection.
- `locallife/worker/task_baofu_withdrawal_fact_application_test.go` covers returned-state mapping and terminal order non-regression.
- `locallife/worker/baofu_withdrawal_recovery_scheduler_test.go` covers processing-order query recovery and submitted-command re-enqueue.
- `locallife/api/baofu_callback_test.go` covers withdrawal callback fact persistence/enqueue paths.
- `locallife/api/baofu_settlement_account_test.go` covers operator settlement-account GET/POST scenarios among shared role coverage.
- `locallife/api/operator_profit_sharing_config_test.go` covers the backend operator profit-sharing configs route.
- `locallife/api/casbin_enforcer_test.go` covers operator settlement-account Casbin entries.

Missing high-value tests:

- Mini Program tests for operator withdrawal list partial-failure UX, create duplicate tap/idempotency-key retention, and detail polling stop on terminal status.
- Operator-specific API contract tests for Baofu withdrawal endpoints using `owner_type=operator`, not only shared/merchant/rider coverage.
- Multi-region operator tests proving finance overview aggregates all regions while commission/profit-sharing config fail or require explicit region; this is the most visible product asymmetry in this slice.
- Settlement-account Mini Program tests for operator payment recovery, pending-context cleanup, and failed/default profile re-entry.
- Real Baofu positive withdrawal callback/query evidence. Current evidence is parser/unit/worker level, not funds-action C4.

## Gaps And Refactor Notes

- Multi-region asymmetry: finance overview uses `resolveOperatorRegionSelection` and aggregates all managed regions, while commission bills and profit-sharing config use `getOperatorRegionID`. Current Mini Program overview/bills do not pass `region_id`, so multi-region operators can see aggregate overview but fail or disagree on commission/config views.
- Profit-sharing config is a registered operator backend route with tests, but no current operator Mini Program page/service caller was found. Treat it as backend/API boundary, not a live page capability.
- `withdrawal_records` and `operator_finance.sql` still exist but are not the current Baofu withdrawal path for operator Mini Program; current path uses `baofu_withdrawal_orders` and external payment command/fact tables.
- Settlement-account defaults are included for `profile_pending`, but failed non-merchant/operator default rehydration is narrower than merchant failed-flow defaults. Keep this in mind if operator failed-profile retry UX is expanded.
- Withdrawal response item does not expose raw provider state, fee, or failure details beyond local status text/sync message. This is desirable for leakage control, but user support workflows may need a separate masked operator-support view later.
- Positive C4 for withdrawal remains open by standard: real funds-action approval, funding, callback/query evidence, and masked evidence ledger entry are required before claiming end-to-end provider success.

## Branch Exhaustion

- Entry branches checked: overview load/pull refresh/retry/navigation, bills initial load/range/quick range/pagination/pull refresh, withdrawal list initial load/pull/load-more/partial failure/create gate/detail nav, withdrawal create balance load/retry/amount change/submit/retry failure, withdrawal detail invalid id/load/refresh/poll/terminal stop, settlement status load/refresh/continue payment/submit navigation, settlement submit load/profile edit/validate/submit/wait/result.
- Request branches checked: finance overview, commission, profit-sharing configs, Baofu withdrawal balance/list/detail/create, settlement account GET/POST, Baofu account open callback, Baofu withdrawal callback.
- Backend state branches checked: no operator loaded, invalid/unmanaged/multi-region selection, no assigned region, not-ready settlement account, missing contract no, missing fee member id, insufficient balance, provider balance unavailable, idempotency replay/conflict, owner mismatch detail, terminal/non-terminal withdrawal update, submitted/unknown/accepted/rejected command states, processing/succeeded/failed/returned withdrawal states.
- Durable-state branches checked: finished profit-sharing reads, Baofu account binding/profile/flow, verify-fee payment link, withdrawal order creation, external payment command submitted/claimed/outcome, callback fact dedupe, fact application, recovery query, legacy withdrawal table non-current boundary.
- Dead/orphan branches checked: no current operator Mini Program caller found for `/v1/operators/me/profit-sharing/configs`; no current operator Mini Program use of `withdrawal_records`; no direct provider withdraw call in request handler.
