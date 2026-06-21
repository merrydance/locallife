# Packaging Domain Production Refactor Review

Generated: 2026-06-21
Branch: `feat/packaging-domain-refactor`

## Scope

This review covers the packaging-domain refactor from `origin/main` through the
current feature branch, including the final closure fixes for merchant context
propagation, Mini Program permission gating, print-worker mocks, and SQL guard
behavior.

The refactor moves packaging from a legacy `dishes.is_packaging` special case to
an additive packaging domain:

- Merchant packaging settings and options.
- Cart packaging selection with `selection_version`.
- Backend-owned preview and create-order packaging fee calculation.
- Order packaging snapshots plus `orders.packaging_fee`.
- Merchant packaging settings page in the Mini Program.
- Legacy packaging dish migration and server-side freeze gate.

## Review Findings

No blocking findings remain from the main-thread review.

Important fixes applied during closeout:

- Mini Program packaging requests now carry selected merchant context through
  `X-Merchant-ID`, and GET requests also include `merchant_id` query data to
  avoid request single-flight/cache drift.
- The packaging settings page now uses `ensureMerchantPackagingManagementAccess`
  instead of broad merchant-console access, synchronizes current merchant context,
  resets stale draft state on merchant switch, and saves with merchant context.
- The merchant config entry hides packaging settings for roles that backend
  owner/manager packaging routes reject.
- Device-access capability caching is keyed by merchant instead of using one
  global cache entry.
- Print-worker tests now expect the new order packaging item lookup.
- SQL guard now detects only top-level unscoped `UPDATE` / `DELETE FROM`
  statements and no longer misclassifies PostgreSQL `INSERT ... ON CONFLICT DO
  UPDATE` as an unscoped write.

Non-blocking findings / residual engineering debt:

- `make lint-filesize` still fails because the repository has historical
  oversized `api/`, `logic/`, and `worker/` files. A changed-file audit confirms
  this branch adds packaging wiring to some already-oversized files, while
  packaging-specific new files stay bounded. This is a maintainability debt, not
  a packaging safety blocker.
- A full `go test ./db/sqlc -count=1 -p 1` on a fresh temporary database still
  fails on pre-existing non-packaging fixture/schema issues, including
  `media_assets.bucket_type` references in OCR/review tests and foreign-key setup
  gaps in profit-sharing/rider-application tests. Targeted packaging SQLC tests
  pass on a fresh temporary database.
- A final delegated reviewer sub-agent was started for independent review but
  remained running through repeated waits and was closed without a result. This
  document therefore records the main-thread review and validation evidence only.

## Design Goal Review

- Packaging no longer requires creating or editing a normal dish for the new
  flow. Merchant settings/options are managed through dedicated packaging APIs
  and Mini Program page.
- Customer-facing menu/search/favorites/browse-history/cart paths can exclude
  legacy packaging dishes when the freeze gate is enabled.
- Cart preview and order preview resolve packaging through backend logic and
  return backend-computed `packaging_fee`, total amount, selected option identity,
  and `selection_version`.
- Order creation validates packaging option ownership and stale
  `packaging_selection_version`, then writes order packaging snapshots in the
  order transaction.
- Order response, merchant fee breakdown, payment amount, refund/profit-sharing
  relevant totals, and print rendering read persisted order amount/snapshot data
  instead of current merchant packaging settings.
- Legacy migration is additive and idempotent through unique keys and
  `ON CONFLICT`; the version-276 down migration is intentionally no-op to avoid
  deleting merchant configuration or breaking references after cutover.
- The freeze flag defaults to `false`, so the backend can deploy additive schema
  and APIs before legacy dish writes are rejected.

## Authorization, Idempotency, And Sequencing

Authorization:

- Merchant packaging settings/options are under
  `MerchantStaffMiddleware("owner", "manager")`.
- Backend merchant selection is resolved from path/query/header and verified
  against merchants associated with the authenticated user.
- Mini Program permission gating mirrors backend owner/manager capability but is
  not treated as the security boundary.
- Customer cart packaging selection validates the cart belongs to the current
  user and that the selected option belongs to the target merchant.
- Client-provided packaging name, price, or merchant ownership is not trusted for
  order creation; order snapshots come from backend-loaded packaging options.

Idempotency:

- Settings `PUT` is convergent via upsert.
- Cart packaging selection `PUT` is convergent and preserves `selection_version`
  for repeated selection of the same option.
- Cart packaging clear is convergent.
- Order creation idempotency includes packaging identity/version in the request
  path, and stale selection versions fail before creating a new order.
- Legacy migration reruns converge by `legacy_dish_id` and validates no legacy
  packaging dish was missed.

Sequencing:

- Additive tables and `orders.packaging_fee` are introduced before cutover.
- Existing legacy packaging dishes are copied to packaging options.
- Legacy dish freeze is flag-controlled and defaults off.
- The Mini Program merchant page is wired before enabling the freeze gate.
- Rollback should disable the freeze flag and hide the Mini Program packaging
  entry before any schema rollback. Do not run version-275 down after production
  orders reference packaging tables unless a separate data-retention plan exists.

## Validation Evidence

Passed:

- `git diff --check`
- `bash .github/scripts/test_backend_sql_guard.sh`
- `bash .github/scripts/backend_sql_guard.sh 1cb704ebad55148be00624320126f8bdd45986d8 HEAD`
- `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make check-generated`
- `node .github/scripts/prompt_governance_lint.js`
- `node .github/scripts/prompt_routing_test.js`
- Fresh temporary DB targeted SQLC packaging suite:
  `go test ./db/sqlc -run 'Test(BrowseHistoryFilteredExcludesPackagingDish|FavoriteDishStatusFiltersPackaging|Cart.*Packaging|GetUserCarts.*Packaging|CreateOrderTx.*Packaging|Packaging)' -count=1 -p 1`
- `go test ./api ./logic ./worker -count=1`
- `go test ./api -run 'Test(MerchantPackaging|.*Packaging|CreateOrder|CalculateOrder|Cart|Favorite|Search|Dish|Combo)' -count=1`
- `go test ./logic -run 'Test(ValidateReservationItemsRejectsLegacyPackagingDishWhenFreezeEnabled|ValidateReservationItemsRejectsComboWithLegacyPackagingChildWhenFreezeEnabled|.*Packaging|CalculateCartItemsSubtotal|BuildCartResponse|CreateOrder|Payment|Refund|FeeBreakdown)' -count=1`
- `go test ./worker -run 'TestProcessTaskPrintOrder' -count=1`
- `make test-safety`
- `node scripts/check-merchant-packaging-settings-contract.test.js`
- `node scripts/check-merchant-dish-availability-contract.test.js`
- `npm run compile`
- `WEAPP_GATE_SCOPE=changed npm run quality:check`

Not fully passing / not used as release blocker:

- `make lint-filesize` fails on historical oversized files.
- Full `go test ./db/sqlc -count=1 -p 1` on a fresh DB fails on pre-existing
  non-packaging fixture/schema issues; targeted packaging SQLC tests pass.

## Release Notes

- Deploy backend migrations and APIs first with
  `PACKAGING_LEGACY_DISH_FREEZE_ENABLED=false`.
- Verify migrated packaging settings/options counts against the baseline audit.
- Deploy Mini Program merchant packaging settings and customer packaging checkout
  surfaces.
- Smoke a merchant with migrated legacy packaging and a merchant with no
  packaging configured.
- Enable the legacy freeze flag only after backend, Mini Program merchant
  settings, and customer checkout flows are deployed and smoke-tested.

## Rollback Plan

1. Set `PACKAGING_LEGACY_DISH_FREEZE_ENABLED=false`.
2. Hide or roll back the Mini Program packaging settings entry if merchant
   operations need to return to legacy dish management temporarily.
3. Keep additive packaging tables and `orders.packaging_fee` in place while any
   production orders may reference packaging snapshots.
4. If amount defects are found after payment, stop rollout and run finance /
   reconciliation review before data correction.
5. Only run destructive schema rollback with an explicit production data
   retention and order-reference plan.
