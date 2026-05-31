# Merchant Member Balance Adjustment Slice

Status: merchant-state flow slice created; idempotency, labels, pagination, and cancellation split-balance repaired 2026-05-31
Risk class: G3 - merchant staff can mutate customer stored-value balance, durable ledgers, and checkout eligibility
Scope: merchant member list/detail page -> member balance adjustment API -> membership balance transaction -> customer checkout readers and adjacent recharge/order/refund writers

## Variant Coverage

This slice covers:

- Merchant member list and member detail page under `weapp/miniprogram/pages/merchant/settings/members`.
- Manual merchant balance adjustment through `POST /v1/merchants/:id/members/:user_id/balance`.
- Adjacent merchant offline recharge through `POST /v1/merchants/:id/members/:user_id/recharge`.
- Shared durable balance truth in `merchant_memberships`.
- Shared ledger truth in `membership_transactions`.
- Order creation balance deduction, order cancellation balance rollback, preview/payment readers, and transaction-history readers where they depend on the same balance truth.

This slice does not cover:

- Membership settings scene/stacking configuration, already captured in `merchant-membership-settings`.
- Recharge-rule CRUD except where merchant recharge resolves a bonus rule.
- Provider payment/refund execution outside the membership balance ledger.

## Product Invariant

Manual balance changes must leave one auditable truth:

- `merchant_memberships.balance` must equal `principal_balance + bonus_balance`.
- Every balance mutation must have a ledger row in `membership_transactions`.
- Repeated ambiguous submissions must not accidentally apply the same merchant intent twice.
- Customer checkout preview and final order creation must read the same current balance and principal/bonus split.
- Merchant-visible transaction labels must understand every transaction type the backend writes.

Fixed 2026-05-31 in `62839932`: manual adjustment now has a durable idempotency contract, Mini Program transaction labels cover adjustment transaction types, member list pagination uses backend full count, and order cancellation preserves principal/bonus split fields. Remaining high-impact product decision: whether managers may manually mutate stored-value balance.

## Primary Forward Chain

1. Merchant dashboard exposes the member list entry.
   Evidence: `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts:187`, `weapp/miniprogram/app.json:309`.

2. The member page resolves current merchant id from storage or `getMyMerchantProfile`, then loads `GET /v1/merchants/:id/members`.
   Evidence: `weapp/miniprogram/pages/merchant/settings/members/index.ts:126`, `weapp/miniprogram/pages/merchant/settings/members/index.ts:136`, `weapp/miniprogram/pages/merchant/settings/members/index.ts:167`.

3. The Mini Program wrapper maps member list, detail, and adjustment to `/v1/merchants/:id/members`, `/members/:user_id`, and `/members/:user_id/balance`.
   Evidence: `weapp/miniprogram/api/merchant.ts:850`, `weapp/miniprogram/api/merchant.ts:862`, `weapp/miniprogram/api/merchant.ts:873`.

4. Fixed 2026-05-31 in `62839932`: the backend returns the full matched member count, and the Mini Program uses that `total` to infer `hasMore`.
   Evidence: `locallife/api/membership.go`, `locallife/logic/membership_merchant.go`, `weapp/miniprogram/pages/merchant/settings/members/index.ts`.

5. Opening member detail calls `GET /v1/merchants/:id/members/:user_id`, displays the latest membership fields, and maps recent transaction rows into local tags.
   Evidence: `weapp/miniprogram/pages/merchant/settings/members/index.ts:207`, `weapp/miniprogram/pages/merchant/settings/members/index.ts:215`, `weapp/miniprogram/pages/merchant/settings/members/index.ts:217`, `weapp/miniprogram/pages/merchant/settings/members/index.ts:219`.

6. The adjustment popup collects direction, amount in yuan, and required notes, then converts the amount to signed cents.
   Evidence: `weapp/miniprogram/pages/merchant/settings/members/index.ts:244`, `weapp/miniprogram/pages/merchant/settings/members/index.ts:274`, `weapp/miniprogram/pages/merchant/settings/members/index.ts:279`, `weapp/miniprogram/pages/merchant/settings/members/index.ts:284`.

7. Fixed 2026-05-31 in `62839932`: submit keeps a per-draft adjustment idempotency key and calls `adjustMerchantMemberBalance` with `Idempotency-Key`.
   Evidence: `weapp/miniprogram/pages/merchant/settings/members/index.ts`, `weapp/miniprogram/api/merchant.ts`, `weapp/scripts/check-merchant-member-balance-adjust-contract.test.js`.

8. The backend member route group allows owner and manager for list, detail, recharge, and balance adjustment.
   Evidence: `locallife/api/server.go:1640`, `locallife/api/server.go:1641`, `locallife/api/server.go:1650`, `locallife/api/server.go:1653`.

9. `adjustMemberBalance` binds signed `amount` and required notes, resolves the merchant from middleware context, and calls `logic.AdjustMemberBalance`.
   Evidence: `locallife/api/membership.go:1131`, `locallife/api/membership.go:1178`, `locallife/api/membership.go:1191`, `locallife/api/membership.go:1197`.

10. Logic rejects zero amount, checks route merchant against authenticated merchant, loads membership by merchant plus user, and calls `AdjustMemberBalanceTx`.
    Evidence: `locallife/logic/membership_balance_adjust.go:28`, `locallife/logic/membership_balance_adjust.go:31`, `locallife/logic/membership_balance_adjust.go:35`, `locallife/logic/membership_balance_adjust.go:46`.

11. `AdjustMemberBalanceTx` locks membership with `FOR UPDATE`, adjusts principal balance only, computes the new aggregate balance, and rejects negative aggregate balance.
    Evidence: `locallife/db/sqlc/tx_membership.go:315`, `locallife/db/sqlc/tx_membership.go:321`, `locallife/db/sqlc/tx_membership.go:322`, `locallife/db/sqlc/tx_membership.go:324`.

12. The transaction updates `merchant_memberships`, then creates a `membership_transactions` row in the same DB transaction.
    Evidence: `locallife/db/sqlc/tx_membership.go:342`, `locallife/db/sqlc/tx_membership.go:355`, `locallife/db/query/membership.sql:33`, `locallife/db/query/membership.sql:133`.

13. Positive adjustment writes type `adjustment_credit`; negative adjustment writes `adjustment_debit`.
    Evidence: `locallife/db/sqlc/tx_membership.go:331`, `locallife/db/sqlc/tx_membership.go:334`, `locallife/db/sqlc/tx_membership.go:335`, `locallife/db/sqlc/tx_membership.go:338`.

14. Fixed 2026-05-31 in `62839932`: the Mini Program transaction tag helper maps `adjustment_credit` and `adjustment_debit` to product labels.
    Evidence: `weapp/miniprogram/pages/merchant/_utils/membership-transaction-view.ts`, `weapp/scripts/check-merchant-member-balance-adjust-contract.test.js`.

15. Merchant offline recharge is adjacent to the same route group but uses an explicit `Idempotency-Key` header, resolves matching recharge rule bonus, and writes a recharge transaction with the key.
    Evidence: `locallife/api/membership.go:1243`, `locallife/api/membership.go:1265`, `locallife/logic/membership_recharge.go:94`, `locallife/logic/membership_recharge.go:118`, `locallife/logic/membership_recharge.go:131`.

16. Fixed 2026-05-31 in `62839932`: manual adjustment now has an adjustment-specific durable idempotency key and conflict handling.
    Evidence: `locallife/db/migration/000241_add_membership_adjustment_idempotency.up.sql`, `locallife/db/sqlc/tx_membership.go`, `locallife/logic/membership_balance_adjust.go`.

17. Order creation locks the membership, checks aggregate balance, deducts principal and bonus split, writes aggregate plus split balances, and records a `consume` transaction.
    Evidence: `locallife/db/sqlc/tx_create_order.go:115`, `locallife/db/sqlc/tx_create_order.go:121`, `locallife/db/sqlc/tx_create_order.go:187`, `locallife/db/sqlc/tx_create_order.go:197`, `locallife/db/sqlc/tx_create_order.go:201`, `locallife/db/sqlc/tx_create_order.go:215`.

18. Fixed 2026-05-31 in `62839932`: order cancellation rollback now preserves principal/bonus split fields while restoring aggregate balance and total consumed.
    Evidence: `locallife/db/sqlc/tx_order_status.go`, `locallife/db/sqlc/tx_order_status_test.go`.

19. Customer preview and checkout logic read split balances to decide payable principal/bonus amounts.
    Evidence: `locallife/logic/promotion_engine.go:377`, `locallife/logic/promotion_engine.go:378`, `locallife/logic/membership_payment.go:20`, `locallife/logic/order_service.go:224`.

## Reverse-Reference Findings

- Only the merchant member page directly calls the manual adjustment wrapper.
- The backend also exposes merchant offline recharge on the same route group, but no Mini Program caller was found in the current search.
- Runtime writers of `merchant_memberships` include membership join/create, merchant recharge, order balance deduction, order cancellation rollback, refund helpers, and manual adjustment.
- Generated SQL `IncrementMembershipBalance` and `DecrementMembershipBalance` exist but were not found in runtime call sites; they are zombie candidates because they bypass the richer transaction-specific ledger semantics used by current transactions.
- The transaction-history DTO hides `principal_amount`, `bonus_amount`, `payment_order_id`, and `idempotency_key`, so the merchant member detail page cannot show split provenance or idempotency evidence.

## SQL And Durable State Boundaries

- `merchant_memberships`: durable membership balance row keyed by merchant and user; stores aggregate balance, principal balance, bonus balance, total recharged, and total consumed.
- `membership_transactions`: durable ledger rows; stores type, signed amount, split amounts, balance after, related order, recharge rule, payment order, notes, and optional idempotency key.
- Manual adjustment uses `UpdateMembershipBalance` plus `CreateMembershipTransaction` in one transaction.
- Merchant recharge uses `RechargeTx` and can use `CreateMembershipRechargeTransaction` with an idempotency key.
- Order creation uses `CreateOrderTx` to deduct balance and create the `consume` ledger row in the same transaction.
- Order cancellation uses `CancelOrderTx` to roll back balance and create a refund ledger row.

## Trust, Authorization, And Tenant Checks

- The route group uses `MerchantStaffMiddleware("owner", "manager")`.
- Logic repeats a merchant-id equality check between authenticated merchant context and route merchant id.
- Membership lookup uses `(merchant_id, user_id)`, so a manager cannot adjust a member from another merchant if middleware context and logic checks hold.
- Managers can manually adjust customer stored-value balance. This may be intentional, but it is a high-impact permission contract that should be explicitly product-owned.

## Idempotency And Duplicate-Submit Checks

- The Mini Program has an in-memory `adjustSubmitting` guard.
- Fixed 2026-05-31 in `62839932`: manual adjustment requires an idempotency key, persists the key with the ledger row, and returns conflict for a duplicate key with a different request.
- A timeout or ambiguous response followed by retry should reuse the same adjustment key for the unchanged draft; the remaining product decision is whether managers should be allowed to initiate these adjustments at all.
- Merchant recharge already has a stronger contract: clients must send `Idempotency-Key`, and duplicate keys return or conflict against the existing transaction.

## Recovery And Async Convergence Paths

- Manual adjustment, recharge, order deduction, and cancellation rollback are synchronous DB transactions.
- No worker, scheduler, websocket, or outbox path repairs or announces manual adjustment state.
- The page updates the list row from the response, then reloads detail through the normal detail API.
- If detail reload fails after a successful adjustment, the list row can show the new balance while the detail drawer closes or remains stale depending on failure handling.

## Frontend Draft And Backend Rehydration

- The member list is always backend-loaded; there is no long-lived editable draft for balances.
- The adjustment popup draft is local and discarded when opened again.
- After successful adjustment, the page rehydrates the changed list row from backend response and reloads detail.
- Fixed 2026-05-31 in `62839932`: the current adjustment transaction types have Mini Program product-label mappings. Future transaction types still require explicit mapping.

## Test Coverage Signals

Observed tests:

- `logic/membership_merchant_test.go` covers forbidden list/detail and detail success/not found.
- `logic/membership_recharge_test.go` covers merchant recharge validation, authz, success, duplicate idempotency key, and unique-conflict recovery.
- `db/sqlc/tx_membership_test.go` covers recharge, consume, refund, and concurrency for recharge/consume.
- `api/membership_test.go` covers membership APIs plus merchant recharge and manual adjustment idempotency request behavior.
- `logic/membership_balance_adjust_test.go` and `db/sqlc/tx_membership_test.go` cover manual adjustment idempotency/conflict and insufficient-balance branches added on 2026-05-31.
- `db/sqlc/tx_order_status_test.go` covers cancellation rollback preserving membership split-balance invariants.
- `weapp/scripts/check-merchant-member-balance-adjust-contract.test.js` covers Mini Program adjustment idempotency header, draft-key reuse, labels, and pagination total usage.
- `db/sqlc/tx_create_order_test.go` covers balance payment during order creation.

Missing high-value tests:

- Product-level manager permission decision and tests if the permission changes.
- Optional explicit response contract for returning `transaction_id` from manual adjustment.
- Ongoing contract test coverage when future backend transaction types are added.

## Gaps And Refactor Notes

- Fixed 2026-05-31 in `62839932`: manual adjustment requires a client/server idempotency key like merchant recharge.
- Fixed 2026-05-31 in `62839932`: frontend maps backend `adjustment_credit` and `adjustment_debit`.
- Fixed 2026-05-31 in `62839932`: member list `total` now uses full matched count.
- Fixed 2026-05-31 in `62839932`: order cancellation rollback preserves principal/bonus split fields.
- Consider returning `transaction_id` from manual adjustment so the Mini Program can show a durable audit anchor after success.

## Branch Exhaustion

- Entry branches checked: Mini Program merchant member list, member detail drawer, manual balance adjustment popup, adjacent offline recharge route group, transaction label helper, customer checkout readers, and order cancellation rollback readers/writers. Flutter App has no member, stored-value, recharge, or balance-adjustment entry in `merchant_app/lib/features/**`. Web is intentionally out of scope.
- Request branches checked: `GET /v1/merchants/:id/members`, `GET /v1/merchants/:id/members/:user_id`, `POST /v1/merchants/:id/members/:user_id/balance`, adjacent `POST /recharge`, transaction history readers, order creation balance deduction, order cancellation rollback, and preview/payment eligibility reads.
- Backend state branches checked: membership lookup by merchant/user, manual positive and negative adjustment, aggregate/principal/bonus balance invariant, negative-balance rejection, transaction ledger insert, merchant recharge with rule bonus and idempotency key, order consume split deduction, cancellation refund ledger, and generated direct balance SQL methods.
- Async branches checked: manual adjustment is synchronous only; no worker, scheduler, websocket, or outbox repair was found. Adjacent order/payment/refund flows can later mutate the same membership balance through their own transaction paths.
- Failure/retry branches checked: frontend `adjustSubmitting` guard, manual adjustment durable idempotency key, negative insufficient-balance branch, detail reload failure after committed adjustment, recharge duplicate-key recovery, and order cancellation rollback split-field preservation.
- Reader/consumer branches checked: merchant member list/detail, merchant transaction labels, customer membership views, checkout preview, direct order creation, promotion engine membership scene checks, and order cancellation/refund history.
- Authorization/tenant branches checked: route owner/manager middleware, logic merchant-id equality recheck, membership lookup by `(merchant_id,user_id)`, and manager permission to mutate stored value. The manager-write permission is a product decision needing explicit sign-off.
- Zombie/unreachable branches checked: generated `IncrementMembershipBalance` and `DecrementMembershipBalance` were not found in runtime call sites and bypass richer ledger semantics if reused; Mini Program has no caller for merchant offline recharge in the current searched paths.
- Test-proof gaps checked: existing tests now cover manual adjustment API/logic/DB behavior, split-balance invariant after cancellation, transaction-type display contract, pagination total semantics, and idempotency replay/conflict behavior. Remaining proof is product/permission-level manager write policy if that changes.
