# Rider Income And Baofu Withdrawal Slice

Status: rider-state flow slice created 2026-06-08
Risk class: G3 - rider income visibility, Baofu settlement account readiness, verify-fee payment, account-open callback/recovery, income withdrawal, provider balance/withdraw callbacks, terminal idempotency
Scope: Mini Program rider income/settlement/withdrawal pages -> rider income and Baofu routes -> profit-sharing/settlement/withdrawal logic -> SQL tables -> Baofu provider callbacks/workers/recovery

## Variant Coverage

This slice covers:

- Mini Program rider income index, withdrawal list/create/detail, settlement-account status, and settlement-account submit pages.
- Read-only rider income summary/ledger/daily APIs under `/v1/rider/income/**`.
- Baofu profit-sharing status convergence for rider income rows: completed order bill trigger, command worker, `/webhooks/baofu/share`, query recovery, fact application, and rider finance notification.
- Rider Baofu settlement account GET/POST under `/v1/rider/settlement-account`, including personal-account profile defaults, verify-fee payment, account-opening flow, provider callback, and task recovery.
- Verify-fee Mini Program payment workflow, including generic payment polling/detail and query-triggered Baofu verify-fee fact application.
- Rider Baofu income withdrawal balance/list/detail/create routes under `/v1/rider/income/baofu-withdrawal/**`.
- Baofu withdrawal provider acceptance, callback, recovery scheduler, and terminal status application.

This slice does not fully cover:

- Delivery assignment/grab and rider delivery state transitions before rider income rows exist; delivery state is covered by `rider-delivery-lifecycle`.
- Rider deposit recharge/withdrawal; local deposit money movement is covered by `rider-deposit`.
- Merchant/platform/operator Baofu owner variants except where shared code has a rider-specific branch.

## Product Invariant

Rider income and income withdrawal must keep three durable truths separate:

- Income display is read-only over `profit_sharing_orders` for the current rider and does not mutate payout or deposit state.
- Rider income status changes are written only by Baofu profit-sharing command/callback/query fact application; income read APIs must not synthesize terminal status.
- Online/grab readiness depends on an active rider Baofu settlement binding; rider personal-account opening reaches ready directly when the binding becomes active.
- Rider income withdrawal uses Baofu provider balance and `baofu_withdrawal_orders`; it does not freeze rider deposit, reserve local income rows, or write `rider_deposits`.
- Withdrawal terminal truth converges only from Baofu callback/query status into `succeeded`, `failed`, or `returned`, and terminal rows must not regress.

## Primary Forward Chain

1. Rider Mini Program entries include income, withdrawal list/create/detail, settlement-account status, and settlement-account submit pages.
   Evidence: `weapp/miniprogram/app.json:12`, `weapp/miniprogram/app.json:17`, `weapp/miniprogram/app.json:18`, `weapp/miniprogram/app.json:19`, `weapp/miniprogram/app.json:20`, `weapp/miniprogram/app.json:21`.

2. The rider income page service loads income summary, daily preview, ledger page, rider status, and Baofu withdrawal balance together; settlement payment readiness is rendered from rider status.
   Evidence: `weapp/miniprogram/pages/rider/_services/rider-income.ts:266`, `weapp/miniprogram/pages/rider/_services/rider-income.ts:273`, `weapp/miniprogram/pages/rider/_services/rider-income.ts:282`, `weapp/miniprogram/pages/rider/_services/rider-income.ts:283`, `weapp/miniprogram/pages/rider/_services/rider-income.ts:291`.

3. Frontend rider income wrappers call `GET /v1/rider/income/summary`, `/ledger`, and `/daily`.
   Evidence: `weapp/miniprogram/pages/rider/_api/rider-income.ts:70`, `weapp/miniprogram/pages/rider/_api/rider-income.ts:73`, `weapp/miniprogram/pages/rider/_api/rider-income.ts:81`, `weapp/miniprogram/pages/rider/_api/rider-income.ts:89`.

4. Backend rider income and Baofu withdrawal routes are registered under `/v1/rider`.
   Evidence: `locallife/api/server.go:1144`, `locallife/api/server.go:1145`, `locallife/api/server.go:1146`, `locallife/api/server.go:1147`, `locallife/api/server.go:1148`, `locallife/api/server.go:1149`, `locallife/api/server.go:1150`, `locallife/api/server.go:1151`.

5. Income handlers parse date/page/status query params, derive the authenticated user id, and delegate to `RiderIncomeService`.
   Evidence: `locallife/api/rider_income.go:170`, `locallife/api/rider_income.go:177`, `locallife/api/rider_income.go:183`, `locallife/api/rider_income.go:185`, `locallife/api/rider_income.go:215`, `locallife/api/rider_income.go:230`, `locallife/api/rider_income.go:257`, `locallife/api/rider_income.go:272`.

6. Rider income service resolves the current rider by user id before every read; missing rider maps to 404.
   Evidence: `locallife/logic/rider_income.go:90`, `locallife/logic/rider_income.go:91`, `locallife/logic/rider_income.go:125`, `locallife/logic/rider_income.go:131`, `locallife/logic/rider_income.go:177`, `locallife/logic/rider_income.go:178`, `locallife/logic/rider_income.go:234`, `locallife/logic/rider_income.go:238`.

7. Summary reads finished rider profit-sharing stats plus per-status summary; ledger validates status `pending/processing/finished/failed`, normalizes pagination, counts, and lists rows; daily reads finished rows grouped by date.
   Evidence: `locallife/logic/rider_income.go:97`, `locallife/logic/rider_income.go:106`, `locallife/logic/rider_income.go:126`, `locallife/logic/rider_income.go:141`, `locallife/logic/rider_income.go:151`, `locallife/logic/rider_income.go:183`, `locallife/logic/rider_income.go:205`, `locallife/logic/rider_income.go:221`.

8. SQL income truth is `profit_sharing_orders`, joined to payment order, order, and merchant for ledger display; stats/daily count only finished income.
   Evidence: `locallife/db/query/profit_sharing_order.sql:489`, `locallife/db/query/profit_sharing_order.sql:497`, `locallife/db/query/profit_sharing_order.sql:501`, `locallife/db/query/profit_sharing_order.sql:508`, `locallife/db/query/profit_sharing_order.sql:512`, `locallife/db/query/profit_sharing_order.sql:526`, `locallife/db/query/profit_sharing_order.sql:541`, `locallife/db/query/profit_sharing_order.sql:550`.

9. Completed order profit-sharing trigger/recovery validates the paid Baofu payment order, refund safety, and rider portion for takeout delivery bills before scheduling a Baofu share command.
   Evidence: `locallife/logic/order_service.go:754`, `locallife/logic/order_service.go:758`, `locallife/logic/baofu_profit_sharing_trigger.go:15`, `locallife/logic/baofu_profit_sharing_trigger.go:33`, `locallife/logic/baofu_profit_sharing_trigger.go:50`, `locallife/logic/baofu_profit_sharing_trigger.go:72`, `locallife/worker/baofu_payment_recovery_scheduler.go:167`, `locallife/worker/baofu_payment_recovery_scheduler.go:174`.

10. Baofu profit-sharing worker accepts only pending/failed Baofu aggregate rows, prepares the command transaction to set processing, records external command audit, calls Baofu share-after-pay, and stores upstream share id when accepted; invalid request/build failures mark the local row failed.
    Evidence: `locallife/worker/task_baofu_profit_sharing.go:73`, `locallife/worker/task_baofu_profit_sharing.go:96`, `locallife/worker/task_baofu_profit_sharing.go:102`, `locallife/worker/task_baofu_profit_sharing.go:106`, `locallife/worker/task_baofu_profit_sharing.go:127`, `locallife/worker/task_baofu_profit_sharing.go:147`, `locallife/worker/task_baofu_profit_sharing.go:161`, `locallife/worker/task_baofu_profit_sharing.go:261`, `locallife/db/query/profit_sharing_order.sql:291`, `locallife/db/query/profit_sharing_order.sql:300`.

11. Baofu share callback verifies parser and collect merchant/terminal identity, resolves the local profit-sharing row by out order no or provider query fallback, records a terminal share fact/application when terminal, and enqueues payment fact application.
    Evidence: `locallife/api/server.go:556`, `locallife/api/baofu_callback.go:325`, `locallife/api/baofu_callback.go:337`, `locallife/api/baofu_callback.go:348`, `locallife/api/baofu_callback.go:353`, `locallife/api/baofu_callback.go:363`, `locallife/api/baofu_callback.go:377`, `locallife/logic/baofu_profit_sharing_service.go:192`, `locallife/logic/baofu_profit_sharing_service.go:230`, `locallife/logic/baofu_profit_sharing_service.go:245`.

12. Profit-sharing recovery scans processing rows, queries Baofu by trade no or out order no, records manual-reconciliation facts, enqueues fact application, and can return a processing row to failed when the provider reports the share relation does not exist.
    Evidence: `locallife/worker/baofu_payment_recovery_scheduler.go:342`, `locallife/worker/baofu_payment_recovery_scheduler.go:351`, `locallife/worker/baofu_payment_recovery_scheduler.go:353`, `locallife/worker/baofu_payment_recovery_scheduler.go:368`, `locallife/worker/baofu_payment_recovery_scheduler.go:384`, `locallife/worker/baofu_profit_sharing_recovery_query.go:17`, `locallife/worker/baofu_profit_sharing_recovery_query.go:93`.

13. Profit-sharing fact application validates terminal Baofu facts and updates `profit_sharing_orders` from `processing` to `finished` or `failed`; finished/failed rows are terminal-idempotent, and successful Baofu amount must match the locally calculated share amount.
    Evidence: `locallife/logic/payment_fact_application_service.go:255`, `locallife/logic/payment_fact_application_service.go:820`, `locallife/logic/payment_fact_application_service.go:854`, `locallife/logic/payment_fact_application_service.go:871`, `locallife/logic/payment_fact_application_service.go:886`, `locallife/logic/payment_fact_application_service.go:900`, `locallife/logic/payment_fact_application_service.go:914`, `locallife/db/query/profit_sharing_order.sql:306`, `locallife/db/query/profit_sharing_order.sql:314`.

14. Profit-sharing result processing can notify the rider user: success says the rider delivery fee arrived; failure/closed says settlement is still being checked, and both link back to rider income state through `profit_sharing_order_id`.
    Evidence: `locallife/worker/task_process_payment.go:1687`, `locallife/worker/task_process_payment.go:1724`, `locallife/worker/task_process_payment.go:1748`, `locallife/worker/task_process_payment.go:1753`, `locallife/worker/task_process_payment.go:1766`, `locallife/worker/task_process_payment.go:1773`.

15. Settlement-account frontend wrapper maps rider role to `/v1/rider/settlement-account`; rider submit page builds a personal profile, validates it, and starts Baofu onboarding.
   Evidence: `weapp/miniprogram/pages/rider/_main_shared/api/baofu-account.ts:163`, `weapp/miniprogram/pages/rider/_main_shared/api/baofu-account.ts:172`, `weapp/miniprogram/pages/rider/_main_shared/api/baofu-account.ts:176`, `weapp/miniprogram/pages/rider/_main_shared/api/baofu-account.ts:183`, `weapp/miniprogram/pages/rider/_main_shared/api/baofu-account.ts:194`, `weapp/miniprogram/pages/rider/_main_shared/api/baofu-account.ts:196`, `weapp/miniprogram/pages/rider/settlement-account/submit/index.ts:83`, `weapp/miniprogram/pages/rider/settlement-account/submit/index.ts:114`.

16. Rider settlement backend routes require `RiderMiddleware` and force `owner_type='rider'`, `owner_id=rider.id`, `owner_user_id=rider.user_id`, and `account_type='personal'`.
    Evidence: `locallife/api/server.go:1138`, `locallife/api/server.go:1139`, `locallife/api/baofu_settlement_account.go:72`, `locallife/api/baofu_settlement_account.go:78`, `locallife/api/baofu_settlement_account.go:79`, `locallife/api/baofu_settlement_account.go:80`, `locallife/api/baofu_settlement_account.go:81`, `locallife/api/baofu_settlement_account.go:82`, `locallife/api/baofu_settlement_account.go:103`, `locallife/api/baofu_settlement_account.go:109`.

17. Shared settlement POST rejects client-controlled owner/account fields, merges rider profile defaults when needed, calls `PrepareOpening`, and enqueues Baofu account opening when the flow is prepared for provider work.
    Evidence: `locallife/api/baofu_settlement_account.go:245`, `locallife/api/baofu_settlement_account.go:251`, `locallife/api/baofu_settlement_account.go:259`, `locallife/api/baofu_settlement_account.go:269`, `locallife/api/baofu_settlement_account.go:270`, `locallife/api/baofu_settlement_account.go:289`, `locallife/api/baofu_settlement_account.go:290`, `locallife/api/baofu_settlement_account.go:298`.

18. Baofu onboarding reuses an active binding, finds or creates an active flow, returns `profile_pending` when profile is incomplete, recovers provider-progress flows, creates/uses verify-fee payment for rider accounts, and prepares provider opening when payment is complete.
    Evidence: `locallife/logic/baofu_account_onboarding_service.go:191`, `locallife/logic/baofu_account_onboarding_service.go:214`, `locallife/logic/baofu_account_onboarding_service.go:231`, `locallife/logic/baofu_account_onboarding_service.go:235`, `locallife/logic/baofu_account_onboarding_service.go:242`, `locallife/logic/baofu_account_onboarding_service.go:263`, `locallife/logic/baofu_account_onboarding_service.go:276`, `locallife/logic/baofu_account_onboarding_service.go:297`.

19. Rider verify-fee payment uses the shared Mini Program payment workflow; it polls generic payment status, fetches payment detail, and `GET /v1/payments/:id/query` can record/apply direct payment query facts for `baofu_account_verify_fee`, including terminal failure branches that close/fail the onboarding flow.
    Evidence: `weapp/miniprogram/pages/rider/_main_shared/services/baofu-account-onboarding.ts:526`, `weapp/miniprogram/pages/rider/_main_shared/services/payment-workflow.ts:181`, `weapp/miniprogram/pages/rider/_main_shared/services/payment-workflow.ts:218`, `weapp/miniprogram/pages/rider/_main_shared/services/payment-workflow.ts:245`, `weapp/miniprogram/pages/rider/_main_shared/api/payment.ts:743`, `locallife/api/server.go:1108`, `locallife/api/payment_order.go:389`, `locallife/logic/payment_order_query_wechat.go:304`, `locallife/logic/payment_order_query_wechat.go:342`, `locallife/logic/payment_order_query_wechat.go:364`, `locallife/logic/payment_fact_application_baofu_verify_fee.go:20`, `locallife/logic/payment_fact_application_baofu_verify_fee.go:56`.

20. Account-open callback parses and validates Baofu identity, records an external fact, and account-opening tasks/fact applications move flow/binding state forward.
    Evidence: `locallife/api/server.go:552`, `locallife/api/server.go:553`, `locallife/api/baofu_callback.go:69`, `locallife/api/baofu_callback.go:82`, `locallife/api/baofu_callback.go:93`, `locallife/api/baofu_callback.go:98`, `locallife/worker/task_baofu_account_opening.go:16`, `locallife/worker/task_baofu_account_opening.go:67`.

21. Active rider personal binding is marked active directly; only merchant/platform active binding writes a platform fee ledger or can continue merchant report. Rider personal account does not use merchant report continuation.
    Evidence: `locallife/logic/baofu_account_onboarding_apply.go:343`, `locallife/logic/baofu_account_onboarding_apply.go:370`, `locallife/logic/baofu_account_onboarding_apply.go:379`, `locallife/logic/baofu_account_onboarding_apply.go:391`, `locallife/logic/baofu_account_onboarding_apply.go:409`, `locallife/logic/baofu_account_onboarding_apply.go:455`.

22. Rider online and grab gates consume Baofu readiness from the shared rider readiness helper; income withdrawal itself requires the same active binding through `readyBinding`.
    Evidence: `locallife/api/rider_baofu_readiness.go:10`, `locallife/api/rider.go:840`, `locallife/logic/delivery_grab.go:75`, `locallife/logic/baofu_withdraw_service.go:101`, `locallife/logic/baofu_withdraw_service.go:142`.

23. Baofu withdrawal frontend wrapper maps rider role to `/v1/rider/income/baofu-withdrawal`; create page loads provider balance, validates amount, creates withdrawal, and redirects to detail.
    Evidence: `weapp/miniprogram/pages/rider/_main_shared/api/baofu-withdrawal.ts:59`, `weapp/miniprogram/pages/rider/_main_shared/api/baofu-withdrawal.ts:68`, `weapp/miniprogram/pages/rider/_main_shared/api/baofu-withdrawal.ts:72`, `weapp/miniprogram/pages/rider/_main_shared/api/baofu-withdrawal.ts:79`, `weapp/miniprogram/pages/rider/_main_shared/api/baofu-withdrawal.ts:90`, `weapp/miniprogram/pages/rider/_main_shared/api/baofu-withdrawal.ts:97`, `weapp/miniprogram/pages/rider/income/withdrawals/create/index.ts:52`, `weapp/miniprogram/pages/rider/income/withdrawals/create/index.ts:92`.

24. Rider withdrawal handlers use `GetRiderFromContext`, force `owner_type='rider'` and `owner_id=rider.id`, and use shared balance/list/detail/create handlers. Detail/list are owner-scoped.
    Evidence: `locallife/api/baofu_withdrawal.go:164`, `locallife/api/baofu_withdrawal.go:170`, `locallife/api/baofu_withdrawal.go:173`, `locallife/api/baofu_withdrawal.go:179`, `locallife/api/baofu_withdrawal.go:182`, `locallife/api/baofu_withdrawal.go:188`, `locallife/api/baofu_withdrawal.go:191`, `locallife/api/baofu_withdrawal.go:197`, `locallife/api/baofu_withdrawal.go:236`, `locallife/api/baofu_withdrawal.go:241`, `locallife/api/baofu_withdrawal.go:374`.

25. Balance queries require Baofu withdrawal service config, active binding, collect merchant/terminal, and provider balance. Response exposes available/pending/ledger/frozen and min/max withdraw thresholds.
    Evidence: `locallife/api/baofu_withdrawal.go:249`, `locallife/api/baofu_withdrawal.go:253`, `locallife/api/baofu_withdrawal.go:261`, `locallife/api/baofu_withdrawal.go:264`, `locallife/api/baofu_withdrawal.go:268`, `locallife/api/baofu_withdrawal.go:270`, `locallife/logic/baofu_withdraw_service.go:92`, `locallife/logic/baofu_withdraw_service.go:98`, `locallife/logic/baofu_withdraw_service.go:101`, `locallife/logic/baofu_withdraw_service.go:107`.

26. Withdrawal create validates amount/service config, re-queries provider balance, rejects insufficient balance, creates `baofu_withdrawal_orders(status='processing')`, calls Baofu create-withdraw, and records accepted/rejected/unknown external command audit.
    Evidence: `locallife/api/baofu_withdrawal.go:275`, `locallife/api/baofu_withdrawal.go:288`, `locallife/api/baofu_withdrawal.go:292`, `locallife/logic/baofu_withdraw_service.go:126`, `locallife/logic/baofu_withdraw_service.go:135`, `locallife/logic/baofu_withdraw_service.go:150`, `locallife/logic/baofu_withdraw_service.go:162`, `locallife/logic/baofu_withdraw_service.go:166`, `locallife/logic/baofu_withdraw_service.go:179`, `locallife/logic/baofu_withdraw_service.go:188`, `locallife/logic/baofu_withdraw_service.go:210`, `locallife/logic/baofu_withdraw_service.go:249`.

27. Provider create rejection marks the local withdrawal failed; unknown create result stays processing for recovery. Accepted processing stores Baofu withdraw no when present.
    Evidence: `locallife/logic/baofu_withdraw_service.go:189`, `locallife/logic/baofu_withdraw_service.go:192`, `locallife/logic/baofu_withdraw_service.go:210`, `locallife/logic/baofu_withdraw_service.go:231`, `locallife/logic/baofu_withdraw_service.go:249`, `locallife/logic/baofu_withdraw_service.go:260`.

28. Baofu withdraw callback validates payout identity, resolves the local withdrawal by out request no, and enqueues fact application with upstream state/raw snapshot.
    Evidence: `locallife/api/server.go:554`, `locallife/api/baofu_callback.go:115`, `locallife/api/baofu_callback.go:133`, `locallife/api/baofu_callback.go:144`, `locallife/api/baofu_callback.go:149`, `locallife/api/baofu_callback.go:155`, `locallife/api/baofu_callback.go:161`.

29. Baofu withdrawal recovery scans `processing` rows older than five minutes, queries provider by out request no/trade date, ignores still-processing results, and enqueues fact application for terminal upstream statuses.
    Evidence: `locallife/worker/baofu_withdrawal_recovery_scheduler.go:90`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:109`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:128`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:142`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:143`, `locallife/worker/baofu_withdrawal_recovery_scheduler.go:151`.

30. Fact application maps upstream state to local status and updates only rows still `processing`; existing terminal rows are idempotent no-ops. Terminal statuses are `succeeded`, `failed`, and `returned`.
    Evidence: `locallife/worker/task_baofu_withdrawal_fact_application.go:47`, `locallife/worker/task_baofu_withdrawal_fact_application.go:55`, `locallife/worker/task_baofu_withdrawal_fact_application.go:59`, `locallife/worker/task_baofu_withdrawal_fact_application.go:62`, `locallife/worker/task_baofu_withdrawal_fact_application.go:71`, `locallife/worker/task_baofu_withdrawal_fact_application.go:94`, `locallife/db/query/baofu_withdrawal_order.sql:66`, `locallife/db/query/baofu_withdrawal_order.sql:72`, `locallife/db/query/baofu_withdrawal_order.sql:75`.

## SQL And Durable State Boundaries

- `profit_sharing_orders`: rider income source of truth, including rider amount, gross amount, payment fee, status, finished time, and provider identifiers.
- `external_payment_commands`, `external_payment_facts`, and `external_payment_fact_applications`: Baofu share command/callback/query evidence and terminal status application for rider income rows.
- `payment_orders`, `orders`, `merchants`: joined read context for rider income ledger rows.
- `baofu_account_opening_profiles`: rider personal profile defaults/submitted profile.
- `baofu_account_opening_flows`: profile/verify-fee/provider/opening state machine for owner `rider`.
- `baofu_account_bindings`: active rider settlement account binding, contract no, sharing member id, and readiness truth.
- `payment_orders`, `external_payment_commands`, `external_payment_facts`, and `external_payment_fact_applications`: Baofu verify-fee payment and provider command/fact evidence for account opening.
- `baofu_withdrawal_orders`: rider income withdrawal intent, owner scope, amount, provider withdraw no, processing/terminal status, and raw provider snapshot.

## Trust, Authorization, And Tenant Checks

- Income reads resolve `riders` by authenticated user id before querying `profit_sharing_orders`.
- Baofu share callbacks verify configured collect identity; query recovery uses provider credentials and local `profit_sharing_orders` identifiers, not client input.
- Settlement-account and income-withdrawal money routes require `RiderMiddleware`.
- Rider settlement POST ignores client owner fields and builds scope from `GetRiderFromContext`.
- Rider withdrawal list/detail returns 404 when the requested withdrawal order owner type/id does not match the current rider.
- Baofu callbacks verify collect or payout member/terminal identity before recording or enqueueing provider facts.

## Idempotency And Duplicate-Submit Checks

- Income reads are idempotent and do not mutate state.
- Baofu profit-sharing command can be retried for pending/failed rows; terminal fact application does not regress finished/failed rows.
- Settlement account GET/POST reuses active bindings and active flows; an incomplete profile returns `profile_pending`, and in-progress provider states can be recovered.
- Verify-fee payment is reused while pending/paid through the shared onboarding service.
- Withdrawal create currently has no client idempotency key; every accepted POST can create a new `baofu_withdrawal_orders` row after balance check.
- Provider create `unknown` is intentionally treated as submitted/processing and converges through recovery query.
- Withdrawal fact application is terminal-idempotent and only updates rows still `processing`.

## Recovery And Async Convergence Paths

- Baofu account opening can converge through frontend polling, verify-fee payment fact application, account-open callback fact, `baofu:process_account_opening` worker, and provider-progress recovery.
- Rider income status can converge through completed-order scheduling, Baofu share command retry, share callback, share query recovery, and payment fact application retries.
- Online/grab readiness recovers automatically once `baofu_account_bindings.open_state='active'`.
- Baofu withdrawal converges through callback or five-minute recovery scans of processing rows.
- Frontend withdrawal detail/list refresh is only a read-side recovery handle; backend `baofu_withdrawal_orders` is canonical.

## Frontend Draft And Backend Rehydration

- Income page treats income/settlement/withdrawal balance responses as authoritative after each load.
- Income page does not force status convergence; it rehydrates whatever `profit_sharing_orders.status` currently says after callback/query fact application.
- Settlement submit page keeps a local personal form but refreshes account state through the shared onboarding wait workflow.
- Withdrawal create page redirects to detail after a created order id; detail/list pages rehydrate from the backend owner-scoped withdrawal order.

## Test Coverage Signals

Observed tests:

- `locallife/api/rider_income_test.go` covers rider income summary, ledger, and daily API branches.
- `locallife/logic/baofu_profit_sharing_service_test.go`, `locallife/worker/task_baofu_profit_sharing_test.go`, `locallife/worker/baofu_payment_recovery_scheduler_test.go`, and `locallife/logic/payment_fact_application_service_test.go` cover share command/fact/status convergence branches.
- `locallife/api/baofu_settlement_account_test.go` covers rider settlement account GET/POST owner/account-type restrictions and readiness variants.
- `locallife/api/baofu_withdrawal_contract_test.go` covers rider Baofu withdrawal route contracts.
- `locallife/worker/task_baofu_account_opening_test.go`, `locallife/worker/task_payment_fact_application_test.go`, and Baofu onboarding logic tests cover account-opening async convergence.
- `locallife/worker/baofu_withdrawal_recovery_scheduler.go` and `locallife/worker/task_baofu_withdrawal_fact_application.go` have adjacent scheduler/fact application tests in the Baofu withdrawal suite.

Missing high-value tests:

- Mini Program settlement-account long-wait workflow through verify-fee payment and active binding refresh.
- End-to-end completed rider delivery -> Baofu share callback/recovery -> rider income ledger status `finished` refresh.
- Rider withdrawal duplicate submit after provider timeout/unknown create result.
- End-to-end rider income withdrawal create -> callback/recovery -> terminal detail refresh.
- Cross-check that rider personal account never enters merchant-report continuation states.

## Gaps And Refactor Notes

- Add a durable idempotency key to rider income withdrawal create to avoid double withdrawal on network retries.
- Consider surfacing provider `unknown` create results explicitly in withdrawal detail copy.
- Keep Baofu share amount mismatch alerts visible to operations because rider income can remain processing/failed until manual reconciliation.
- Keep rider personal-account Baofu docs separate from merchant business-account report continuation to avoid future shared-code regressions.

## Dead And Orphan Paths

- No rider-specific dead income or Baofu withdrawal route was found.
- Shared merchant report continuation exists in Baofu onboarding code, but the rider personal-account active binding path does not use it.

## Branch Exhaustion

- Entry branches checked: income summary/daily/ledger load, status-tab/page load, settlement account status, settlement personal profile submit, verify-fee payment wait, withdrawal balance, withdrawal list/detail, withdrawal create.
- Request branches checked: `/v1/rider/income/summary`, `/ledger`, `/daily`, `/settlement-account` GET/POST, generic `/v1/payments/:id` and `/query` for verify fee, `/income/baofu-withdrawal/balance`, `/withdrawals`, `/withdrawals/:id`, `/withdraw`, `/webhooks/baofu/share`.
- Backend state branches checked: missing rider, invalid date/status/page, pending/processing/finished/failed profit-sharing rows, Baofu share pending/failed command eligibility, refund safety, provider create rejected/accepted/unknown, callback identity mismatch, share fact non-terminal/terminal success/failure/closed, success amount missing/mismatch, no/active Baofu binding, profile pending, verify fee pending/processing/paid/failed/closed/expired, provider opening processing, ready, failed, abnormal, voided, missing service config, insufficient provider balance, withdrawal processing/succeeded/failed/returned.
- Async branches checked: Baofu share command retry, share callback, share query recovery, profit-sharing fact application, rider finance notification, verify-fee payment fact, account-open callback, account-opening worker, provider-progress recovery, withdraw callback, withdraw recovery scheduler, fact application terminal no-op.
- Dead/orphan branches checked: no stale rider income/Baofu endpoint discovered; rider path excludes merchant report continuation.
