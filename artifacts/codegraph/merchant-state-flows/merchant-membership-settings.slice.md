# Merchant Membership Settings Slice

Status: fourth merchant-state flow slice
Risk class: G2 - merchant configuration affects customer balance-payment eligibility, order preview, order creation, and rules-engine decisions
Scope: merchant membership settings page -> membership settings API -> durable settings row -> cart/order preview and direct order balance-payment readers

## Variant Coverage

This slice covers:

- Merchant-side membership settings UI for balance usable scenes, bonus usable scenes, voucher stacking, discount stacking, and max deduction percent.
- `GET/PUT /v1/merchants/me/membership-settings`.
- Durable membership settings stored in `merchant_membership_settings`.
- Customer-facing cart/order preview readers that expose `payment_assessment`.
- Direct order creation balance-payment validation.
- Rules-engine condition reader `balance_scene_allowed`.

This slice does not cover:

- Merchant member list and manual balance adjustment.
- Recharge rule CRUD.
- Voucher, discount-rule, or delivery-promotion management except where they interact with membership stacking during preview.
- Fixing the discovered enum and partial-update drift.

## Product Invariant

When a merchant configures member balance and bonus-money rules, the saved backend truth should be the same truth used by customer checkout preview, final order creation, and any rules-engine decision that depends on balance usability. Scene names must be consistent across Mini Program types, API binding, backend defaults, DB defaults, preview logic, and direct order validation.

Current implementation violates the scene-name invariant: the Mini Program and original migration expose `takeout` and `reservation`, while backend update binding and membership payment logic only support `dine_in` and `takeaway`.

## Primary Forward Chain

1. The merchant dashboard and config page route merchants to the settings page as `叠加规则`.
   Evidence: `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts:186`, `weapp/miniprogram/pages/merchant/config/index.ts:60`, `weapp/miniprogram/app.json:308`.

2. The Mini Program API wrapper declares `MerchantMembershipScene = 'dine_in' | 'takeout' | 'reservation'` and wraps `GET/PUT /v1/merchants/me/membership-settings`.
   Evidence: `weapp/miniprogram/api/merchant.ts:479`, `weapp/miniprogram/api/merchant.ts:481`, `weapp/miniprogram/api/merchant.ts:490`, `weapp/miniprogram/api/merchant.ts:606`, `weapp/miniprogram/api/merchant.ts:617`.

3. The membership settings page offers three scene options: `takeout`, `dine_in`, and `reservation`.
   Evidence: `weapp/miniprogram/pages/merchant/settings/membership/index.ts:20`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:21`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:22`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:23`.

4. `loadSettings` reads backend truth, builds a local `form`, stores an `initialForm`, and clears dirty state. Pull refresh is blocked while dirty.
   Evidence: `weapp/miniprogram/pages/merchant/settings/membership/index.ts:127`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:181`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:182`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:186`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:187`.

5. Toggling switches, scenes, and max deduction percent mutates only local draft state until save.
   Evidence: `weapp/miniprogram/pages/merchant/settings/membership/index.ts:215`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:224`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:232`.

6. `onSave` validates `max_deduction_percent` in the Mini Program, sends all fields to PUT, rehydrates from the response, clears dirty state, and navigates back.
   Evidence: `weapp/miniprogram/pages/merchant/settings/membership/index.ts:249`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:258`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:265`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:272`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:273`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:281`.

7. Backend registers GET under normal auth and PUT under owner-only merchant staff middleware.
   Evidence: `locallife/api/server.go:672`, `locallife/api/server.go:689`, `locallife/api/server.go:690`, `locallife/api/server.go:693`, `locallife/api/server.go:696`.

8. `getMerchantMembershipSettings` first requires an owned merchant for the user, then calls `logic.GetMembershipSettingsForOwner` and returns the logic result.
   Evidence: `locallife/api/membership.go:809`, `locallife/api/membership.go:812`, `locallife/api/membership.go:828`, `locallife/api/membership.go:837`.

9. `updateMerchantMembershipSettings` binds request scenes with `oneof=dine_in takeaway`, not the Mini Program `takeout/reservation` values, then calls `logic.UpdateMembershipSettingsForOwner`.
   Evidence: `locallife/api/membership.go:847`, `locallife/api/membership.go:848`, `locallife/api/membership.go:849`, `locallife/api/membership.go:869`, `locallife/api/membership.go:894`.

10. Logic resolves the merchant by user, verifies ownership, and returns logic defaults when no settings row exists.
    Evidence: `locallife/logic/membership_settings.go:51`, `locallife/logic/membership_settings.go:52`, `locallife/logic/membership_settings.go:59`, `locallife/logic/membership_settings.go:63`, `locallife/logic/membership_settings.go:66`.

11. Logic defaults are `[]string{"dine_in", "takeaway"}` for balance and `[]string{"dine_in"}` for bonus.
    Evidence: `locallife/logic/membership_settings.go:29`, `locallife/logic/membership_settings.go:32`, `locallife/logic/membership_settings.go:33`.

12. The sanitizer and supported-scene helper only preserve `dine_in` and `takeaway`.
    Evidence: `locallife/logic/membership_balance_scenes.go:3`, `locallife/logic/membership_balance_scenes.go:5`, `locallife/logic/membership_balance_scenes.go:14`, `locallife/logic/membership_balance_scenes.go:22`.

13. Update logic fills omitted fields from logic defaults, then uses full-row `UpsertMerchantMembershipSettings`.
    Evidence: `locallife/logic/membership_settings.go:86`, `locallife/logic/membership_settings.go:93`, `locallife/logic/membership_settings.go:96`, `locallife/logic/membership_settings.go:99`, `locallife/logic/membership_settings.go:102`, `locallife/logic/membership_settings.go:105`, `locallife/logic/membership_settings.go:109`.

14. SQL truth is the unique `merchant_membership_settings` row. The runtime writer is upsert; partial-update SQL exists but is not used by the observed handler path.
    Evidence: `locallife/db/query/merchant_membership_settings.sql:3`, `locallife/db/query/merchant_membership_settings.sql:19`, `locallife/db/query/merchant_membership_settings.sql:31`, `locallife/db/query/merchant_membership_settings.sql:42`.

15. The original migration default and comments use `takeout` and `reservation`, not `takeaway`.
    Evidence: `locallife/db/migration/000030_add_merchant_membership_settings.up.sql:8`, `locallife/db/migration/000030_add_merchant_membership_settings.up.sql:9`, `locallife/db/migration/000030_add_merchant_membership_settings.up.sql:38`.

16. Customer cart preview calls `logic.CalculateCartPreview`, which runs `PromotionEngine.CalculateFinalPrice` and returns `PaymentAssessment`.
    Evidence: `locallife/api/cart.go:617`, `locallife/api/cart.go:636`, `locallife/api/cart.go:661`, `locallife/logic/cart_calculation.go:135`, `locallife/logic/cart_calculation.go:136`, `locallife/logic/cart_calculation.go:148`.

17. Order preview calls `OrderService.CalculateOrderPreview`, which delegates to `logic.CalculateOrderPreview`, runs `PromotionEngine.CalculateFinalPrice`, and returns `PaymentAssessment`.
    Evidence: `locallife/api/order.go:2572`, `locallife/api/order.go:2594`, `locallife/api/order.go:2650`, `locallife/logic/order_calculation.go:239`, `locallife/logic/order_calculation.go:240`, `locallife/logic/order_calculation.go:260`.

18. `PromotionEngine` loads membership settings, applies scene filters, stacking rules, and max deduction percent, then curates hints for `takeout` and `reservation`.
    Evidence: `locallife/logic/promotion_engine.go:355`, `locallife/logic/promotion_engine.go:358`, `locallife/logic/promotion_engine.go:369`, `locallife/logic/promotion_engine.go:379`, `locallife/logic/promotion_engine.go:383`, `locallife/logic/promotion_engine.go:386`, `locallife/logic/promotion_engine.go:397`, `locallife/logic/promotion_engine.go:402`, `locallife/logic/promotion_engine.go:410`, `locallife/logic/promotion_engine.go:446`, `locallife/logic/promotion_engine.go:465`.

19. Direct order creation passes `use_balance` into `OrderService.CreateOrder`, which calls `ValidateMembershipPayment` before computing totals.
    Evidence: `locallife/api/order.go:525`, `locallife/api/order.go:544`, `locallife/api/order.go:555`, `locallife/logic/order_service.go:221`, `locallife/logic/order_service.go:224`, `locallife/logic/order_service.go:237`.

20. `ValidateMembershipPayment` rejects unsupported order types before loading membership and settings. It then checks `merchant_membership_settings.balance_usable_scenes` only if settings load succeeds.
    Evidence: `locallife/logic/membership_payment.go:20`, `locallife/logic/membership_payment.go:21`, `locallife/logic/membership_payment.go:25`, `locallife/logic/membership_payment.go:36`, `locallife/logic/membership_payment.go:39`, `locallife/logic/membership_payment.go:45`.

21. The rules engine has a `balance_scene_allowed` condition that uses the same `IsMembershipBalanceSupportedOrderType` gate before reading settings.
    Evidence: `locallife/api/rules_engine_db.go:251`, `locallife/api/rules_engine_db.go:259`, `locallife/api/rules_engine_db.go:446`, `locallife/api/rules_engine_db.go:450`, `locallife/api/rules_engine_db.go:453`.

22. Dine-in/reservation checkout shows backend payment hints and can submit `use_balance` if the user selects stored-value payment. Its payment method disabled state currently uses local member balance, not `payment_assessment.is_balance_payable`.
    Evidence: `weapp/miniprogram/pages/dine-in/_utils/dine-in-checkout-view.ts:85`, `weapp/miniprogram/pages/dine-in/_utils/dine-in-checkout-view.ts:126`, `weapp/miniprogram/pages/dine-in/_utils/dine-in-checkout-view.ts:148`, `weapp/miniprogram/pages/dine-in/checkout/checkout.wxml:139`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:249`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:254`.

23. Takeout order-confirm maps `payment_assessment.payment_hint` into each cart view. The direct takeout balance-payment path is currently rejected by backend tests.
    Evidence: `weapp/miniprogram/pages/takeout/order-confirm/_utils/takeout-order-confirm-support.ts:372`, `weapp/miniprogram/pages/takeout/order-confirm/index.wxml:156`, `locallife/api/order_test.go:4867`, `locallife/api/order_test.go:4954`.

## Reverse-Reference Findings

- Only the merchant membership settings page calls the GET/PUT wrapper directly.
- The durable settings row is read by `logic.GetMembershipSettingsForOwner`, `PromotionEngine.loadMembershipSettings`, `ValidateMembershipPayment`, and the rules-engine DB adapter.
- Generated SQL includes `CreateMerchantMembershipSettings` and `UpdateMerchantMembershipSettings`, but runtime references found in handlers/logic use `GetMerchantMembershipSettings` and `UpsertMerchantMembershipSettings`.
- Customer order type constants include `takeout`, `dine_in`, `takeaway`, and `reservation`, while the merchant settings scene contract is split between `takeout/reservation` and `takeaway`.
- `curatePaymentBalance` has explicit `takeout` and `reservation` branches, but `applyMembershipSettings` zeroes principal/bonus first for unsupported order types. Those branches are unreachable for ordinary membership settings until supported scenes include those order types or the function is called with pre-filled assessment elsewhere.

## SQL And Durable State Boundaries

- Table: `merchant_membership_settings`.
- Unique owner key: `merchant_id`.
- Fields: `balance_usable_scenes TEXT[]`, `bonus_usable_scenes TEXT[]`, `allow_with_voucher`, `allow_with_discount`, `max_deduction_percent`.
- DB defaults from migration: balance `['dine_in', 'takeout', 'reservation']`, bonus `['dine_in']`.
- Logic defaults: balance `['dine_in', 'takeaway']`, bonus `['dine_in']`.
- Runtime write path: full-row upsert on conflict by `merchant_id`.
- Existing partial-update SQL is generated but not used by the current PUT handler path.

## Trust, Authorization, And Tenant Checks

- Mini Program page calls `ensureMerchantConsoleAccess` before loading settings.
- GET route requires an authenticated user and then calls `requireOwnedMerchantForUser`; managers are rejected.
- PUT route uses owner-only merchant staff middleware and then repeats the owned-merchant check in handler/logic.
- The client never sends `merchant_id` for settings writes; backend resolves merchant by authenticated user.
- Downstream cart/order previews and order creation run in authenticated customer context. They read settings by the merchant being checked out, not from client-provided membership settings.

## Idempotency And Duplicate-Submit Checks

- Frontend blocks duplicate save through `saving`.
- PUT has no idempotency key and no version. Identical repeated saves converge to the same row.
- Concurrent saves are last-write-wins.
- Runtime PUT behaves as full replace. Because request fields are pointers/optional in API shape but logic fills omissions from defaults, partial requests can reset omitted values instead of preserving the existing durable row.

## Recovery And Async Convergence Paths

- No scheduler, worker, callback, outbox, websocket, or polling path was found for membership settings.
- Changes take effect synchronously on the next cart/order preview, rules evaluation, or direct order creation.
- Frontend silent refresh preserves last synced settings if refresh fails.
- Save rehydrates from backend response, which means backend sanitizer drift is visible as scenes disappearing after save or reload.

## Frontend Draft And Backend Rehydration

- `form` is local draft; `initialForm` is the dirty-state baseline.
- `onPullDownRefresh` refuses refresh while dirty.
- `onSave` sends all current form fields, rehydrates from PUT response, and resets dirty state.
- If the merchant selects Mini Program-only scenes (`takeout` or `reservation`), current backend binding rejects the PUT before rehydration.
- If existing DB rows include `takeout` or `reservation`, GET sanitizes them out through logic, so the merchant page cannot see that DB truth as originally stored.

## Test Coverage Signals

Observed tests:

- `locallife/api/security_authz_test.go` covers manager denial for PUT membership settings.
- `locallife/logic/membership_payment_test.go` covers direct balance-payment validation, including explicit rejection for `takeout` and `reservation`.
- `locallife/api/order_test.go` covers direct order creation with balance for dine-in, scene disallow, partial balance, voucher plus balance, and explicit `takeout` rejection.
- `locallife/logic/promotion_engine_test.go` covers membership scene/cap/stacking behavior using `takeaway`, and separately covers `curatePaymentBalance` hints for `takeout` and `reservation`.
- `locallife/api/order_calculate_test.go` verifies `payment_assessment` is returned by order calculation response shape.

Missing high-value tests:

- API tests for `GET/PUT /v1/merchants/me/membership-settings` successful owner flow and request validation.
- Contract test proving frontend scene values are accepted or deliberately hidden by backend.
- Test for existing DB default `takeout/reservation` rows being sanitized away in GET.
- Test defining whether PUT is full replace or partial merge.
- End-to-end preview-vs-direct-order consistency test for each supported order type.
- Rules-engine `balance_scene_allowed` test covering the same scene semantics as the settings API.

## Gaps And Refactor Notes

- Scene enum drift is the primary defect. Mini Program and migration expose `takeout`/`reservation`; backend settings API and payment logic only support `dine_in`/`takeaway`.
- The product needs a single explicit distinction between `takeout` delivery and `takeaway` self-pickup before code changes. Existing order APIs support both, but merchant settings UI only exposes `takeout`.
- The `reservation` branch in payment assessment looks intentional, but direct membership payment rejects reservation before settings are consulted.
- Runtime update uses full upsert while SQL also offers a partial update. Align API docs, wrapper optional fields, and logic behavior before relying on partial updates.
- `CreateMerchantMembershipSettings` and `UpdateMerchantMembershipSettings` SQL methods are generated and appear unused by runtime. They are zombie candidates if no tests or migration helpers require them.
- Dine-in checkout payment-method UI disables balance only by local member balance, while backend `payment_assessment` can say balance is unusable due to merchant settings or stacking. This may let the user choose balance and then fail at order creation.

## Branch Exhaustion

- Entry branches checked: Mini Program membership settings page, dashboard/config entry, scene toggles, stacking toggles, max deduction percent, cart/order preview, dine-in checkout payment method, direct order creation, promotion engine rule condition, and membership payment validation. Flutter App has no membership settings entry in `merchant_app/lib/features/**`.
- Request branches checked: `GET/PUT /v1/merchants/me/membership-settings`, frontend wrappers, owner lookup, full upsert path, unused generated create/update SQL, cart/order calculate, direct order creation, and membership payment validation.
- Backend state branches checked: settings row/defaults, DB defaults versus logic defaults, balance scenes, bonus scenes, voucher/discount stacking, max deduction percent, scene sanitizer, optional request fields, full replace versus partial merge, and payment assessment order-type handling.
- Async branches checked: none found. Settings take effect synchronously on next preview/order/rules evaluation; no worker, scheduler, websocket, outbox, or cache invalidation path exists.
- Failure/retry branches checked: duplicate save guard, last-write-wins PUT, optional omitted fields resetting to defaults, frontend `takeout/reservation` values rejected/sanitized, DB rows with unsupported scenes hidden on GET, preview/direct-order scene mismatch, and UI allowing balance choice before backend rejects.
- Reader/consumer branches checked: merchant settings page, cart/order preview, dine-in checkout payment selection, direct order creation, promotion engine `balance_scene_allowed`, membership payment validation, and customer-facing payment assessment.
- Authorization/tenant branches checked: Mini Program merchant console access, owner-only GET/PUT via owned-merchant checks and owner middleware, backend-resolved merchant id, and downstream customer flows using authenticated user plus merchant/order context rather than client-provided settings.
- Zombie/unreachable branches checked: generated `CreateMerchantMembershipSettings` and `UpdateMerchantMembershipSettings` SQL appear unused; `curatePaymentBalance` `takeout/reservation` branches are unreachable under current sanitizer/direct-payment rejection; DB migration defaults expose scenes that runtime sanitizes away.
- Test-proof gaps checked: existing tests cover manager denial, direct balance validation, dine-in order balance, scene disallow, takeout rejection, promotion engine takeaway behavior, and payment assessment shape. Missing proof remains for owner GET/PUT contract, frontend/backend scene enum contract, existing DB default sanitization, full-replace versus partial-merge semantics, per-order-type preview/direct consistency, and rules-engine scene parity.
