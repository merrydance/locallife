# Customer Profile Address Wallet Membership Reviews Slice

Status: customer-state flow slice created 2026-06-14
Risk class: G2/G3 mix - customer PII, addresses/location, media/avatar, wallet ledger, membership recharge/payment, vouchers, favorites, notifications, reviews
Scope: user center/profile/address/wallet/membership/coupons/favorites/reviews/agreements/notifications/about pages -> users/media/address/payment/membership/voucher/favorite/review/agreement/notification backend routes -> SQL tables and payment/media boundaries

## Variant Coverage

This slice covers:

- User center profile load, role/workbench display boundary, avatar upload/update, nickname update, and navigation to ordinary customer pages.
- Address list/edit/select mode, WeChat address import, map/location/geocode hints, default address, update/delete, and checkout return selection.
- Wallet page membership balance summary, payment ledger read, payment/refund detail navigation, and payment ledger terminal-state presentation.
- Membership page membership list/details and recharge entry/payment boundary.
- Coupons page user voucher/available coupon reads.
- Favorites page merchant/dish favorites list/delete and detail navigation.
- Reviews list/create/update/delete handoff and review image upload boundary.
- Notification page list/unread/read-all/delete/preferences and agreement/about static support.

This slice does not fully cover:

- Merchant/rider/operator/platform role workbenches or bind merchant; explicit account/role boundary exit.
- Payment provider internals for membership recharge; payment-domain boundary.
- Media moderation/private access internals; media-domain boundary.
- Review moderation/operator/merchant reply behavior beyond customer create/update/delete/read.

## Product Invariant

User center owns customer account-facing state, but not cross-role authority:

- Profile, address, favorites, notification preferences, reviews, membership, wallet, and vouchers must be scoped to authenticated user.
- Avatar/review media writes must persist media asset ids and respect media access rules.
- Address/location drafts must be revalidated by backend address and checkout/delivery-fee logic.
- Wallet/payment ledger is read-only customer visibility over payment/refund/membership durable state.
- Role exits are not ordinary customer lifecycle closure.

## Primary Forward Chain

1. Customer Mini Program declares user center, notification, and user-center subpackages for addresses, coupons, favorites, membership, reviews, wallet, payment detail, refund detail, reservations, agreements, about, and service center.
   Evidence: `weapp/miniprogram/app.json:5`, `weapp/miniprogram/app.json:7`, `weapp/miniprogram/app.json:216`, `weapp/miniprogram/app.json:223`, `weapp/miniprogram/app.json:229`, `weapp/miniprogram/app.json:235`, `weapp/miniprogram/app.json:241`, `weapp/miniprogram/app.json:248`, `weapp/miniprogram/app.json:254`, `weapp/miniprogram/app.json:260`, `weapp/miniprogram/app.json:272`, `weapp/miniprogram/app.json:279`, `weapp/miniprogram/app.json:285`.

2. User center loads current user/profile roles, navigates to customer pages, updates avatar through media upload and `/v1/users/me`, and updates nickname.
   Evidence: `weapp/miniprogram/pages/user_center/index.ts:38`, `weapp/miniprogram/pages/user_center/index.ts:60`, `weapp/miniprogram/pages/user_center/index.ts:86`, `weapp/miniprogram/pages/user_center/index.ts:150`, `weapp/miniprogram/pages/user_center/index.ts:521`, `weapp/miniprogram/pages/user_center/index.ts:524`, `weapp/miniprogram/pages/user_center/index.ts:578`, `locallife/api/server.go:650`, `locallife/api/user.go:226`, `locallife/api/user.go:276`.

3. Address list/edit pages load addresses, support select mode, WeChat address import, edit/default/delete/create/update, and backend address routes.
   Evidence: `weapp/miniprogram/pages/user_center/addresses/index.ts:10`, `weapp/miniprogram/pages/user_center/addresses/index.ts:42`, `weapp/miniprogram/pages/user_center/addresses/index.ts:73`, `weapp/miniprogram/pages/user_center/addresses/index.ts:117`, `weapp/miniprogram/pages/user_center/addresses/index.ts:133`, `weapp/miniprogram/pages/user_center/addresses/index.ts:155`, `weapp/miniprogram/pages/user_center/addresses/index.ts:172`, `weapp/miniprogram/pages/user_center/addresses/edit/index.ts:31`, `weapp/miniprogram/pages/user_center/addresses/edit/index.ts:248`, `weapp/miniprogram/pages/user_center/addresses/edit/index.ts:356`, `weapp/miniprogram/pages/user_center/addresses/edit/index.ts:366`.

4. Wallet page loads memberships and payment ledger, displays membership balance summary, and navigates to membership/payment/refund detail.
   Evidence: `weapp/miniprogram/pages/user_center/wallet/index.ts:80`, `weapp/miniprogram/pages/user_center/wallet/index.ts:94`, `weapp/miniprogram/pages/user_center/wallet/index.ts:115`, `weapp/miniprogram/pages/user_center/wallet/index.ts:121`, `weapp/miniprogram/pages/user_center/wallet/index.ts:126`, `weapp/miniprogram/pages/user_center/wallet/_main_shared/api/membership.ts:25`, `weapp/miniprogram/pages/user_center/wallet/_main_shared/api/payment.ts:568`.

5. Membership page loads memberships and supports recharge/auto-recharge entry into payment workflow.
   Evidence: `weapp/miniprogram/pages/user_center/membership/index.ts:25`, `weapp/miniprogram/pages/user_center/membership/index.ts:39`, `weapp/miniprogram/pages/user_center/membership/index.ts:56`, `weapp/miniprogram/pages/user_center/membership/index.ts:102`, `weapp/miniprogram/pages/user_center/membership/_main_shared/api/membership.ts:29`, `locallife/api/membership.go:126`, `locallife/api/membership.go:664`, `locallife/api/membership.go:722`.

6. Coupons and favorites pages load user vouchers/favorites and delete favorite records by target id.
   Evidence: `weapp/miniprogram/pages/user_center/coupons/index.ts:18`, `weapp/miniprogram/pages/user_center/coupons/index.ts:52`, `weapp/miniprogram/pages/user_center/coupons/_main_shared/api/coupon.ts:301`, `weapp/miniprogram/pages/user_center/coupons/_main_shared/api/coupon.ts:347`, `weapp/miniprogram/pages/user_center/favorites/index.ts:33`, `weapp/miniprogram/pages/user_center/favorites/index.ts:67`, `weapp/miniprogram/pages/user_center/favorites/index.ts:146`, `weapp/miniprogram/pages/user_center/favorites/_api/favorite.ts:54`, `weapp/miniprogram/pages/user_center/favorites/_api/favorite.ts:78`, `weapp/miniprogram/pages/user_center/favorites/_api/favorite.ts:100`.

7. Review create page loads order/review data, uploads images, creates or updates review, and review APIs expose list/get/delete.
   Evidence: `weapp/miniprogram/pages/user_center/reviews/create/index.ts:54`, `weapp/miniprogram/pages/user_center/reviews/create/index.ts:87`, `weapp/miniprogram/pages/user_center/reviews/create/index.ts:186`, `weapp/miniprogram/pages/user_center/reviews/create/index.ts:216`, `weapp/miniprogram/pages/user_center/reviews/create/index.ts:251`, `weapp/miniprogram/pages/user_center/reviews/create/index.ts:259`, `weapp/miniprogram/pages/user_center/reviews/_main_shared/api/review.ts:99`, `weapp/miniprogram/pages/user_center/reviews/_main_shared/api/review.ts:123`, `weapp/miniprogram/pages/user_center/reviews/_main_shared/api/review.ts:131`.

8. Notification page loads notifications, marks read/read-all, deletes, and uses notification preferences API.
   Evidence: `weapp/miniprogram/pages/notification/index.ts:136`, `weapp/miniprogram/pages/notification/index.ts:161`, `weapp/miniprogram/pages/notification/index.ts:177`, `weapp/miniprogram/pages/notification/index.ts:259`, `weapp/miniprogram/pages/notification/index.ts:294`, `weapp/miniprogram/pages/notification/index.ts:320`, `weapp/miniprogram/api/notification.ts:55`, `weapp/miniprogram/api/notification.ts:74`, `weapp/miniprogram/api/notification.ts:81`, `weapp/miniprogram/api/notification.ts:104`, `weapp/miniprogram/api/notification.ts:111`.

9. Agreements pages read legal agreement list/detail from `/v1/agreements`; about page is a static support surface.
   Evidence: `weapp/miniprogram/pages/user_center/agreements/index.ts:4`, `weapp/miniprogram/pages/user_center/agreements/index.ts:33`, `weapp/miniprogram/pages/user_center/agreements/detail/index.ts:16`, `weapp/miniprogram/pages/user_center/agreements/detail/index.ts:60`, `weapp/miniprogram/pages/user_center/about_us/index.ts:3`, `locallife/api/server.go:602`, `locallife/api/agreement.go:23`, `locallife/api/agreement.go:51`.

10. Backend route surface maps customer profile/address/wallet/membership/voucher/favorite/review/notification/agreement reads and writes to durable tables.
    Evidence: `locallife/api/server.go:650`, `locallife/api/server.go:657`, `locallife/api/server.go:677`, `locallife/api/server.go:1271`, `locallife/api/server.go:1554`, `locallife/api/server.go:1577`, `locallife/api/server.go:1597`, `locallife/api/server.go:1693`.

## SQL And Durable State Boundaries

- `users`: customer profile, avatar media id, nickname/full name.
- `media_assets` and upload session tables: avatar/review image media boundary.
- `user_addresses`: customer address book, default address, region/geocode fields.
- `memberships`, `membership_transactions`, and recharge rules: membership wallet/balance/recharge state.
- `payment_orders`, `refund_orders`: wallet ledger and payment/refund detail state.
- `vouchers`, `user_vouchers`: coupons and available voucher state.
- `favorite_merchants`, `favorite_dishes`: customer favorites.
- `reviews` and `review_images`: customer review lifecycle.
- `notifications` and notification preferences: customer notification center.
- Agreement source tables/static material: agreement list/detail projection.

## Trust, Authorization, And Tenant Checks

- All customer profile/address/wallet/review/favorite/notification routes use authenticated user id.
- Address edit/delete/default must validate ownership.
- Review create/update/delete must validate order/review ownership and status.
- Favorite add/delete/status must bind current user and target merchant/dish id.
- Payment ledger/detail must only return current user's payment/refund rows.
- Media private access must not leak another user's private assets.
- Notification list/read/delete/preferences are scoped to current user.

## Idempotency And Duplicate-Submit Checks

- Avatar/nickname updates optimistically update UI but restore on backend failure.
- Address list reloads after create/update/default/delete.
- Favorites delete updates local list after backend success; backend constraints own duplicate favorite add semantics.
- Membership recharge/payment uses payment workflow pending-confirmation instead of client success.
- Notification mark read/read-all/delete are idempotent from customer perspective.
- Review create/update uses `submitting` guard and backend review/order constraints.

## Recovery And Async Convergence Paths

- User center refreshes current user and restores cached avatar when backend avatar is empty.
- Address pages can reload list/detail after navigation or mutation.
- Wallet reloads membership and ledger; non-terminal ledger entries route to payment/refund detail.
- Membership recharge converges through payment callback/fact/recovery.
- Notifications can be refreshed/paginated and read-all after backend failures.
- Review image upload failure is tracked per file before final review submit.

## Frontend Draft And Backend Rehydration

- Avatar local file path, nickname input, address form, selected region/geocode, review text/images, coupon filter state, and notification tab state are frontend drafts.
- User profile, addresses, wallet ledger, memberships, favorites, coupons, notifications, agreements, and reviews are backend-rehydrated.
- Role/workbench/bind merchant affordances are account boundary exits, not customer state transitions.

## Test Coverage Signals

Observed tests:

- `locallife/api/membership_test.go` covers membership recharge/payment validation branches.
- Review, favorite, notification, address, and agreement handlers have dedicated backend modules and SQL query sources.
- Payment/refund tests cover wallet ledger/detail/refund status dependencies.

Missing high-value tests:

- User center avatar upload/update rollback when media or profile update fails.
- Address select-mode return into checkout with backend delivery calculation.
- Membership recharge -> payment result -> wallet ledger refresh.
- Favorite duplicate add/delete consistency across restaurant/dish detail and favorites page.
- Notification category/filter drift regression.

## Gaps And Refactor Notes

- User center contains cross-role workbench and bind merchant exits. These are intentionally excluded from ordinary customer flow closure.
- API wrappers for payment/membership/review/address are copied in multiple page-local folders; keep contract changes synchronized.
- Wallet mixes membership balance and payment ledger visibility; do not infer cash balance or withdrawal capability from this customer page.

## Branch Exhaustion

- Entry branches checked: user center, notification, addresses list/edit/select, coupons, favorites, membership, wallet, payment detail, refund detail handoff, reviews list/create, agreements list/detail, about.
- Request branches checked: `/v1/users/me`, `/v1/media`, `/v1/addresses`, `/v1/location/**`, `/v1/payments/ledger`, `/v1/memberships`, `/recharge`, `/:id/transactions`, `/v1/vouchers/me`, `/available`, `/v1/favorites/**`, `/v1/reviews/**`, `/v1/notifications/**`, `/v1/agreements/**`.
- Backend state branches checked: profile missing avatar/local cached avatar, empty/non-empty address book, default address, membership empty/non-empty, ledger terminal/non-terminal, coupon available/claimed, favorite merchant/dish, review create/update/delete, notification unread/read/deleted/preferences.
- Async branches checked: media upload/complete, membership payment callback/recovery, notification producer boundary.
- Dead/orphan branches checked: no ordinary user-center customer page omitted; role/bind exits excluded by boundary.
