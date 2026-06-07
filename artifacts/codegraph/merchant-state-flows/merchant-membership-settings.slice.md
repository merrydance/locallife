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
- Implementing a new customer-facing `takeaway` checkout entry if no current Mini Program path reaches it.

## Product Invariant

When a merchant configures member balance and bonus-money rules, the saved backend truth should be the same truth used by customer checkout preview, final order creation, and any rules-engine decision that depends on balance usability. Scene names must be consistent across Mini Program types, API binding, backend defaults, DB defaults, preview logic, and direct order validation.

Current product contract is intentionally limited to `dine_in` and `takeaway`. `takeout` delivery and `reservation` are excluded because member stored-value funds are already in the merchant's account, while those order paths can require platform/provider split settlement or reservation-specific payment handling that cannot be funded by merchant-held balance. The historical scene-name drift was that the Mini Program and original migration exposed `takeout` and `reservation`; this branch aligns Mini Program settings, backend binding/support helpers, and a forward migration to `dine_in/takeaway`.

PUT request fields are optional and behave as a partial merge over the existing durable settings row. If no row exists, logic defaults are used as the merge base. Explicit empty scene arrays are preserved as empty arrays, which means "disable this usable scene set" rather than "fall back to defaults".

## Primary Forward Chain

1. The merchant dashboard and config page route merchants to the settings page as `叠加规则`.
   Evidence: `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts:186`, `weapp/miniprogram/pages/merchant/config/index.ts:60`, `weapp/miniprogram/app.json:308`.

2. The Mini Program API wrapper declares `MerchantMembershipScene = 'dine_in' | 'takeaway'` and wraps `GET/PUT /v1/merchants/me/membership-settings`.
   Evidence: `weapp/miniprogram/api/merchant.ts:491`, `weapp/miniprogram/api/merchant.ts:493`, `weapp/miniprogram/api/merchant.ts:502`, `weapp/miniprogram/api/merchant.ts:618`, `weapp/miniprogram/api/merchant.ts:630`.

3. The membership settings page offers only two scene options: `dine_in` and `takeaway`.
   Evidence: `weapp/miniprogram/pages/merchant/settings/membership/index.ts:20`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:21`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:22`.

4. `loadSettings` reads backend truth, builds a local `form`, stores an `initialForm`, and clears dirty state. Pull refresh is blocked while dirty.
   Evidence: `weapp/miniprogram/pages/merchant/settings/membership/index.ts:125`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:179`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:180`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:184`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:185`.

5. Toggling switches, scenes, and max deduction percent mutates only local draft state until save.
   Evidence: `weapp/miniprogram/pages/merchant/settings/membership/index.ts:213`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:222`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:230`.

6. `onSave` validates `max_deduction_percent` in the Mini Program, sends all fields to PUT, rehydrates from the response, clears dirty state, and navigates back.
   Evidence: `weapp/miniprogram/pages/merchant/settings/membership/index.ts:247`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:258`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:263`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:270`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:271`, `weapp/miniprogram/pages/merchant/settings/membership/index.ts:279`.

7. Backend registers GET under normal auth and PUT under owner-only merchant staff middleware.
   Evidence: `locallife/api/server.go:672`, `locallife/api/server.go:689`, `locallife/api/server.go:690`, `locallife/api/server.go:693`, `locallife/api/server.go:696`.

8. `getMerchantMembershipSettings` first requires an owned merchant for the user, then calls `logic.GetMembershipSettingsForOwner` and returns the logic result.
   Evidence: `locallife/api/membership.go:809`, `locallife/api/membership.go:812`, `locallife/api/membership.go:828`, `locallife/api/membership.go:837`.

9. `updateMerchantMembershipSettings` binds request scenes with `oneof=dine_in takeaway`, matching the Mini Program scene type, then calls `logic.UpdateMembershipSettingsForOwner`.
   Evidence: `locallife/api/membership.go:847`, `locallife/api/membership.go:848`, `locallife/api/membership.go:849`, `locallife/api/membership.go:869`, `locallife/api/membership.go:894`.

10. Logic resolves the merchant by user, verifies ownership, and returns logic defaults when no settings row exists.
    Evidence: `locallife/logic/membership_settings.go:51`, `locallife/logic/membership_settings.go:52`, `locallife/logic/membership_settings.go:59`, `locallife/logic/membership_settings.go:63`, `locallife/logic/membership_settings.go:66`.

11. Logic defaults are `[]string{"dine_in", "takeaway"}` for balance and `[]string{"dine_in"}` for bonus.
    Evidence: `locallife/logic/membership_settings.go:29`, `locallife/logic/membership_settings.go:32`, `locallife/logic/membership_settings.go:33`.

12. The sanitizer and supported-scene helper only preserve `dine_in` and `takeaway`.
    Evidence: `locallife/logic/membership_balance_scenes.go:3`, `locallife/logic/membership_balance_scenes.go:5`, `locallife/logic/membership_balance_scenes.go:14`, `locallife/logic/membership_balance_scenes.go:22`.

13. Update logic loads the existing settings row as the merge base, falls back to logic defaults only when the row is missing, overlays only non-nil request fields, preserves explicit empty scene arrays, then writes the merged full row through `UpsertMerchantMembershipSettings`.
    Evidence: `locallife/logic/membership_settings.go:86`, `locallife/logic/membership_settings.go:87`, `locallife/logic/membership_settings.go:93`, `locallife/logic/membership_settings.go:102`, `locallife/logic/membership_settings.go:105`, `locallife/logic/membership_settings.go:118`, `locallife/logic/membership_balance_scenes.go:13`.

14. SQL truth is the unique `merchant_membership_settings` row. The runtime writer is upsert; partial-update SQL exists but is not used by the observed handler path.
    Evidence: `locallife/db/query/merchant_membership_settings.sql:3`, `locallife/db/query/merchant_membership_settings.sql:19`, `locallife/db/query/merchant_membership_settings.sql:31`, `locallife/db/query/merchant_membership_settings.sql:42`.

15. The original migration default and comments use `takeout` and `reservation`, not `takeaway`.
    Evidence: `locallife/db/migration/000030_add_merchant_membership_settings.up.sql:8`, `locallife/db/migration/000030_add_merchant_membership_settings.up.sql:9`, `locallife/db/migration/000030_add_merchant_membership_settings.up.sql:38`.

16. Forward migration `000252` removes unsupported and null stored scenes from existing rows, changes DB defaults to `dine_in/takeaway` for balance and `dine_in` for bonus, and adds CHECK constraints so future persisted scenes cannot drift back to `takeout`, `reservation`, or `NULL`.
    Evidence: `locallife/db/migration/000252_align_membership_settings_scenes.up.sql:5`, `locallife/db/migration/000252_align_membership_settings_scenes.up.sql:23`, `locallife/db/migration/000252_align_membership_settings_scenes.up.sql:27`, `locallife/db/migration/000252_align_membership_settings_scenes.up.sql:33`.

17. Forward-hardening migration `000253` repeats the same cleanup/default/constraint convergence after first dropping existing scene constraints, so environments already marked past `000252` but missing the expected schema still converge.
    Evidence: `locallife/db/migration/000253_harden_membership_settings_scene_constraints.up.sql:1`, `locallife/db/migration/000253_harden_membership_settings_scene_constraints.up.sql:5`, `locallife/db/migration/000253_harden_membership_settings_scene_constraints.up.sql:23`, `locallife/db/migration/000253_harden_membership_settings_scene_constraints.up.sql:27`.

18. Customer cart preview calls `logic.CalculateCartPreview`, which runs `PromotionEngine.CalculateFinalPrice` and returns `PaymentAssessment`.
    Evidence: `locallife/api/cart.go:617`, `locallife/api/cart.go:636`, `locallife/api/cart.go:661`, `locallife/logic/cart_calculation.go:135`, `locallife/logic/cart_calculation.go:136`, `locallife/logic/cart_calculation.go:148`.

19. Order preview calls `OrderService.CalculateOrderPreview`, which delegates to `logic.CalculateOrderPreview`, runs `PromotionEngine.CalculateFinalPrice`, and returns `PaymentAssessment`.
    Evidence: `locallife/api/order.go:2572`, `locallife/api/order.go:2594`, `locallife/api/order.go:2650`, `locallife/logic/order_calculation.go:239`, `locallife/logic/order_calculation.go:240`, `locallife/logic/order_calculation.go:260`.

20. `PromotionEngine` loads membership settings, applies the same `dine_in/takeaway` support gate, scene filters, stacking rules, and max deduction percent, then curates final balance-payable hints.
    Evidence: `locallife/logic/promotion_engine.go:355`, `locallife/logic/promotion_engine.go:358`, `locallife/logic/promotion_engine.go:369`, `locallife/logic/promotion_engine.go:379`, `locallife/logic/promotion_engine.go:383`, `locallife/logic/promotion_engine.go:386`, `locallife/logic/promotion_engine.go:397`, `locallife/logic/promotion_engine.go:402`, `locallife/logic/promotion_engine.go:410`, `locallife/logic/promotion_engine.go:470`.

21. Direct order creation accepts all four order types, but `use_balance` is documented and enforced only for `dine_in/takeaway`; `OrderService.CreateOrder` calls `ValidateMembershipPayment` before computing totals when balance is selected.
    Evidence: `locallife/api/order.go:104`, `locallife/api/order.go:135`, `locallife/api/order.go:529`, `locallife/api/order.go:534`, `locallife/logic/order_service.go:225`, `locallife/logic/order_service.go:226`, `locallife/logic/order_service.go:229`.

22. `ValidateMembershipPayment` rejects unsupported order types before loading membership and settings. It then checks `merchant_membership_settings.balance_usable_scenes` only if settings load succeeds.
    Evidence: `locallife/logic/membership_payment.go:20`, `locallife/logic/membership_payment.go:21`, `locallife/logic/membership_payment.go:25`, `locallife/logic/membership_payment.go:36`, `locallife/logic/membership_payment.go:39`, `locallife/logic/membership_payment.go:45`.

23. The rules engine has a `balance_scene_allowed` condition that uses the same `IsMembershipBalanceSupportedOrderType` gate before reading settings.
    Evidence: `locallife/api/rules_engine_db.go:251`, `locallife/api/rules_engine_db.go:259`, `locallife/api/rules_engine_db.go:446`, `locallife/api/rules_engine_db.go:450`, `locallife/api/rules_engine_db.go:453`.

24. Dine-in checkout shows backend payment hints and can submit `use_balance` if the user selects stored-value payment. Its page type still includes `reservation`, even though backend membership balance rejects reservation, and its payment method disabled state currently uses local member balance rather than `payment_assessment.is_balance_payable`.
    Evidence: `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:38`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:83`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:249`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:254`, `locallife/logic/membership_payment_test.go:36`.

25. Takeout order-confirm maps `payment_assessment.payment_hint` into each cart view but does not send `use_balance`. The direct `takeout` balance-payment path is rejected by backend tests.
    Evidence: `weapp/miniprogram/pages/takeout/order-confirm/_utils/takeout-order-confirm-support.ts:372`, `weapp/miniprogram/pages/takeout/order-confirm/_utils/takeout-order-confirm-support.ts:398`, `weapp/miniprogram/pages/takeout/order-confirm/index.wxml:156`, `locallife/api/order_test.go:5060`, `locallife/api/order_test.go:5147`.

26. A customer-facing Mini Program `takeaway` checkout path that both reaches checkout and sends `use_balance` was not found in this trace. The backend has order and promotion support for `takeaway`, but Mini Program reachability remains a separate proof/work item.
    Evidence: `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:38`, `weapp/miniprogram/pages/dine-in/menu/menu.ts:49`, `weapp/miniprogram/pages/takeout/order-confirm/_utils/takeout-order-confirm-support.ts:418`, `locallife/logic/promotion_engine_test.go:151`, `locallife/logic/promotion_engine_test.go:192`.

## Reverse-Reference Findings

- Only the merchant membership settings page calls the GET/PUT wrapper directly.
- The durable settings row is read by `logic.GetMembershipSettingsForOwner`, `PromotionEngine.loadMembershipSettings`, `ValidateMembershipPayment`, and the rules-engine DB adapter.
- Generated SQL includes `CreateMerchantMembershipSettings` and `UpdateMerchantMembershipSettings`, but runtime references found in handlers/logic use `GetMerchantMembershipSettings` and `UpsertMerchantMembershipSettings`.
- Customer order type constants include `takeout`, `dine_in`, `takeaway`, and `reservation`; the merchant membership settings scene contract is narrower by design and now exposes only `dine_in/takeaway`.
- `curatePaymentBalance` still has explicit `takeout` and `reservation` branches, but `applyMembershipSettings` zeroes principal/bonus first for unsupported order types. Those branches remain unreachable for ordinary membership settings unless another caller pre-fills assessment values elsewhere.

## SQL And Durable State Boundaries

- Table: `merchant_membership_settings`.
- Unique owner key: `merchant_id`.
- Fields: `balance_usable_scenes TEXT[]`, `bonus_usable_scenes TEXT[]`, `allow_with_voucher`, `allow_with_discount`, `max_deduction_percent`.
- Historical DB defaults from migration `000030`: balance `['dine_in', 'takeout', 'reservation']`, bonus `['dine_in']`.
- Forward DB defaults and constraints from migrations `000252` and `000253`: balance `['dine_in', 'takeaway']`, bonus `['dine_in']`, with unsupported/null scene cleanup before CHECK constraints are added.
- Logic defaults: balance `['dine_in', 'takeaway']`, bonus `['dine_in']`.
- Runtime write path: partial-merge semantics in logic, persisted as a full-row upsert on conflict by `merchant_id`.
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
- Runtime PUT behaves as partial merge over existing durable settings. Omitted fields preserve existing values, or default values when the merchant has no settings row yet. Explicit empty scene arrays are persisted as empty arrays.

## Recovery And Async Convergence Paths

- No scheduler, worker, callback, outbox, websocket, or polling path was found for membership settings.
- Changes take effect synchronously on the next cart/order preview, rules evaluation, or direct order creation.
- Frontend silent refresh preserves last synced settings if refresh fails.
- Save rehydrates from backend response. After this branch the settings page no longer offers unsupported scene values, so backend sanitization should not make supported selections disappear.

## Frontend Draft And Backend Rehydration

- `form` is local draft; `initialForm` is the dirty-state baseline.
- `onPullDownRefresh` refuses refresh while dirty.
- `onSave` sends all current form fields, rehydrates from PUT response, and resets dirty state.
- The page no longer lets merchants select `takeout` or `reservation` for membership balance/bonus scenes.
- If existing DB rows include `takeout`, `reservation`, or null scene elements, `000252` and `000253` forward-clean them; GET also sanitizes unsupported scenes through logic.

## Test Coverage Signals

Observed tests:

- `locallife/api/security_authz_test.go` covers manager denial for PUT membership settings.
- `locallife/logic/membership_payment_test.go` covers direct balance-payment validation, including explicit rejection for `takeout` and `reservation`.
- `locallife/api/order_test.go` covers direct order creation with balance for dine-in, scene disallow, partial balance, voucher plus balance, and explicit `takeout` rejection.
- `locallife/logic/promotion_engine_test.go` covers membership scene/cap/stacking behavior using `takeaway`, and separately covers `curatePaymentBalance` hints for `takeout` and `reservation`.
- `locallife/api/order_calculate_test.go` verifies `payment_assessment` is returned by order calculation response shape.
- `weapp/scripts/check-merchant-membership-settings-scenes.test.js` verifies Mini Program membership scene type/options, backend binding/support helper, and migrations `000252/000253` default/CHECK/null-cleanup contract remain aligned to `dine_in/takeaway`.
- `locallife/logic/membership_settings_test.go` and `locallife/api/membership_test.go` cover partial PUT preserving omitted existing fields, explicit empty scene arrays staying non-nil empty arrays, and logic missing-row updates using defaults as the merge base.

Missing high-value tests:

- Broader API tests for `GET/PUT /v1/merchants/me/membership-settings` successful owner flow and request validation beyond the partial PUT regression.
- Direct-order API happy-path test for `takeaway` with `use_balance=true`, if backend product support is expected to remain first-class.
- Migration/applied-schema test proving `000252/000253` clean historical `takeout/reservation` and null scene elements before adding constraints.
- Mini Program user-reachable checkout proof for `takeaway` balance payment, if self-pickup users should pay with member balance from the current Mini Program.
- Rules-engine `balance_scene_allowed` test covering the same scene semantics as the settings API.

## Gaps And Refactor Notes

- Scene enum drift is fixed in this branch by choosing the product contract `dine_in/takeaway` and excluding `takeout/reservation` intentionally.
- Backend direct order and transaction code can support `takeaway` as an order type, and promotion preview tests cover `takeaway` membership scenes. The current Mini Program trace still lacks a user-reachable `takeaway` balance checkout path that sends `use_balance`.
- The `reservation` branch in payment assessment remains a drift/zombie candidate for membership balance because direct membership payment rejects reservation before settings are consulted.
- Runtime update now defines optional request fields as partial merge in logic while continuing to persist through the existing upsert. The unused generated partial-update SQL remains a cleanup candidate, not the runtime contract.
- `CreateMerchantMembershipSettings` and `UpdateMerchantMembershipSettings` SQL methods are generated and appear unused by runtime. They are zombie candidates if no tests or migration helpers require them.
- Dine-in checkout payment-method UI disables balance only by local member balance, while backend `payment_assessment` can say balance is unusable due to merchant settings or stacking. This may let the user choose balance and then fail at order creation.

## Branch Exhaustion

- Entry branches checked: Mini Program membership settings page, dashboard/config entry, scene toggles, stacking toggles, max deduction percent, cart/order preview, dine-in checkout payment method, direct order creation, promotion engine rule condition, and membership payment validation. Flutter App has no membership settings entry in `merchant_app/lib/features/**`.
- Request branches checked: `GET/PUT /v1/merchants/me/membership-settings`, frontend wrappers, owner lookup, partial-merge upsert path, unused generated create/update SQL, cart/order calculate, direct order creation, and membership payment validation.
- Backend state branches checked: settings row/defaults, DB defaults versus logic defaults, balance scenes, bonus scenes, voucher/discount stacking, max deduction percent, scene sanitizer, optional request fields, empty scene arrays, partial merge, and payment assessment order-type handling.
- Async branches checked: none found. Settings take effect synchronously on next preview/order/rules evaluation; no worker, scheduler, websocket, outbox, or cache invalidation path exists.
- Failure/retry branches checked: duplicate save guard, last-write-wins concurrent PUT, omitted fields preserving existing/default merge-base values, explicit empty scene arrays, historical frontend `takeout/reservation` values, DB rows with unsupported/null scenes cleaned or sanitized, preview/direct-order scene mismatch, and UI allowing balance choice before backend rejects.
- Reader/consumer branches checked: merchant settings page, cart/order preview, dine-in checkout payment selection, direct order creation, promotion engine `balance_scene_allowed`, membership payment validation, and customer-facing payment assessment.
- Authorization/tenant branches checked: Mini Program merchant console access, owner-only GET/PUT via owned-merchant checks and owner middleware, backend-resolved merchant id, and downstream customer flows using authenticated user plus merchant/order context rather than client-provided settings.
- Zombie/unreachable branches checked: generated `CreateMerchantMembershipSettings` and `UpdateMerchantMembershipSettings` SQL appear unused; `curatePaymentBalance` `takeout/reservation` branches are unreachable under current sanitizer/direct-payment rejection; original migration `000030` remains historical and is superseded by forward migrations `000252/000253`.
- Test-proof gaps checked: existing tests cover manager denial, direct balance validation, dine-in order balance, scene disallow, takeout/reservation rejection, promotion engine takeaway behavior, payment assessment shape, local migration convergence through `000253`, partial PUT merge semantics including explicit empty arrays and missing-row default merge base, and the new frontend/backend/migration scene contract script. Missing proof remains for broader owner GET/PUT API contract, direct `takeaway` balance order creation, clean/dirty scratch migration fixtures, Mini Program `takeaway` checkout reachability, per-order-type preview/direct consistency, and rules-engine scene parity.
