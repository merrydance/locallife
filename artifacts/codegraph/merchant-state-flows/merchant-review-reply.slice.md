# Merchant Review Reply Slice

Status: merchant-state flow slice created
Risk class: G2 - merchant-authored public response content is written after external content-safety check and affects customer-visible review surfaces
Scope: merchant reviews page -> merchant all-reviews/reply APIs -> review reply durable state -> public/customer review readers

## Variant Coverage

This slice covers:

- Merchant Mini Program review list and reply popup.
- Merchant all-reviews read path, including hidden reviews.
- Merchant reply write path through `POST /v1/reviews/:id/reply`.
- Review image enrichment as a read-side dependency.
- Public/customer readers that expose `merchant_reply` and `replied_at`.

This slice does not fully cover:

- Customer create/update/delete review workflow.
- Operator moderation/delete workflow.
- Review image upload/media moderation, except where merchant review list reads image URLs.

## Product Invariant

Merchant review replies should have one trustworthy content boundary:

- Only the owning merchant should be able to list all of its reviews and reply.
- Reply content must pass content-safety review before it becomes durable or customer-visible.
- The persisted reply and `replied_at` should be the truth shown to merchants and customers after reload.
- Hidden reviews can be replied to, but product copy and downstream readers must keep visibility semantics clear.

## Primary Forward Chain

1. Merchant reviews page checks owner-aware review-management access, then loads merchant review data only for owner-capable roles.
   Evidence: `weapp/miniprogram/pages/merchant/reviews/index.ts:123`, `weapp/miniprogram/pages/merchant/reviews/index.ts:126`, `weapp/miniprogram/pages/merchant/reviews/index.ts:135`, `weapp/miniprogram/utils/console-access.ts:100`, `weapp/miniprogram/utils/console-access.ts:299`.

2. The page resolves merchant id from `current_merchant` cache or `getMyMerchantProfile`.
   Evidence: `weapp/miniprogram/pages/merchant/reviews/index.ts:267`, `weapp/miniprogram/pages/merchant/reviews/index.ts:272`, `weapp/miniprogram/pages/merchant/reviews/index.ts:282`.

3. `loadReviews` calls `ReviewService.listMerchantAllReviews(merchantId, page, pageSize)`, maps backend rows into card state, and preserves existing data on silent refresh failure.
   Evidence: `weapp/miniprogram/pages/merchant/reviews/index.ts:303`, `weapp/miniprogram/pages/merchant/reviews/index.ts:337`, `weapp/miniprogram/pages/merchant/reviews/index.ts:339`, `weapp/miniprogram/pages/merchant/reviews/index.ts:345`, `weapp/miniprogram/pages/merchant/reviews/index.ts:361`.

4. The Mini Program review wrapper maps merchant all-reviews to `GET /v1/reviews/merchants/:id/all`.
   Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/review.ts:107`, `weapp/miniprogram/pages/merchant/_main_shared/api/review.ts:109`.

5. Backend merchant review routes are under `MerchantStaffMiddleware("owner")`, so managers/cashiers/chefs cannot list all reviews through this group or reply.
   Evidence: `locallife/api/server.go:1589`, `locallife/api/server.go:1591`, `locallife/api/server.go:1593`, `locallife/api/server.go:1594`.

6. `listMerchantAllReviews` compares route merchant id with the middleware-loaded merchant id, then returns all reviews including hidden rows and a real count.
   Evidence: `locallife/api/review.go:344`, `locallife/api/review.go:359`, `locallife/api/review.go:366`, `locallife/api/review.go:372`, `locallife/api/review.go:383`, `locallife/db/query/review.sql:87`, `locallife/db/query/review.sql:94`.

7. Merchant opens the reply popup from a list row. Existing reply text is used as the draft for update.
   Evidence: `weapp/miniprogram/pages/merchant/reviews/index.ts:190`, `weapp/miniprogram/pages/merchant/reviews/index.ts:194`, `weapp/miniprogram/pages/merchant/reviews/index.ts:197`.

8. Submit validates non-empty and length <= 500, then calls `ReviewService.replyToReview`.
   Evidence: `weapp/miniprogram/pages/merchant/reviews/index.ts:229`, `weapp/miniprogram/pages/merchant/reviews/index.ts:232`, `weapp/miniprogram/pages/merchant/reviews/index.ts:237`, `weapp/miniprogram/pages/merchant/reviews/index.ts:246`.

9. The frontend wrapper maps reply to `POST /v1/reviews/:id/reply`.
   Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/review.ts:174`, `weapp/miniprogram/pages/merchant/_main_shared/api/review.ts:176`.

10. `replyReview` loads the review, verifies it belongs to the middleware-loaded merchant, then fetches the merchant user and requires a WeChat OpenID.
    Evidence: `locallife/api/review.go:481`, `locallife/api/review.go:497`, `locallife/api/review.go:507`, `locallife/api/review.go:514`, `locallife/api/review.go:520`, `locallife/api/review.go:530`.

11. Reply text goes through WeChat `MsgSecCheck` before the SQL update. Risky content returns a business error; provider/check failure returns a gateway/internal error.
    Evidence: `locallife/api/review.go:534`, `locallife/api/review.go:535`, `locallife/api/review.go:539`.

12. `UpdateMerchantReply` overwrites `reviews.merchant_reply` and sets `replied_at = now()`.
    Evidence: `locallife/api/review.go:543`, `locallife/api/review.go:544`, `locallife/db/query/review.sql:45`, `locallife/db/query/review.sql:47`, `locallife/db/query/review.sql:48`.

13. Reply response enriches review image URLs, then the Mini Program updates only the affected review row locally. Reply submission failures are mapped through `getMerchantReviewReplyErrorMessage` so missing WeChat OpenID, risky text, and content-safety provider/gateway failures use Chinese product copy instead of raw backend/provider diagnostics.
    Evidence: `locallife/api/review.go:553`, `locallife/api/review.go:554`, `locallife/api/review.go:700`, `weapp/miniprogram/pages/merchant/reviews/index.ts:246`, `weapp/miniprogram/pages/merchant/reviews/index.ts:249`, `weapp/miniprogram/pages/merchant/reviews/index.ts:263`, `weapp/miniprogram/pages/merchant/_utils/merchant-review-reply-error.ts:31`.

14. Public merchant review list only returns visible reviews; user review list returns the user's reviews. Both response builders expose merchant reply fields when present.
    Evidence: `locallife/api/review.go:285`, `locallife/api/review.go:300`, `locallife/api/review.go:413`, `locallife/api/review.go:771`, `locallife/db/query/review.sql:20`, `locallife/db/query/review.sql:32`.

## Reverse-Reference Findings

- Merchant all-reviews intentionally includes hidden reviews, while public merchant reviews filter `is_visible = true`.
- Reply is a single overwrite field plus timestamp; there is no durable reply history, delete/clear reply endpoint, or optimistic version.
- Fixed 2026-06-06: merchant owner-only route middleware now matches the page's owner-aware `ensureMerchantReviewManagementAccess` gate, so non-owner staff see an owner-only denied state before all-review/reply API calls.
- Customer review update uses `UpdateReviewTx` for content/images, but merchant reply uses a direct SQL update because only one review row is changed.
- Review images are public-resolved for merchant all-review response, not owner-only-resolved. Hidden review text can be listed to the merchant, but image visibility follows the public URL resolver.

## SQL And Durable State Boundaries

- `reviews`: owns customer content, visibility, `merchant_reply`, and `replied_at`.
- `review_images`: owns review image attachment order.
- Merchant reply writes only `reviews.merchant_reply` and `reviews.replied_at`.
- Customer review create/update/delete and operator delete/update visibility are separate writers outside this merchant reply slice.

## Trust, Authorization, And Tenant Checks

- Frontend checks owner-aware review-management access for `merchant`/`merchant_owner` roles before loading the page.
- Backend write/read merchant management routes require merchant owner role.
- Backend validates route merchant id for list-all and review merchant ownership for reply.
- Reply content is checked using the merchant user's WeChat OpenID before persistence.

## Idempotency And Duplicate-Submit Checks

- The page blocks duplicate reply submit with `replySubmitting`.
- Backend reply is last-write-wins; repeated identical submit updates `replied_at` again.
- There is no idempotency key, version, or compare-and-set against the previous reply text.

## Recovery And Async Convergence Paths

- No worker, scheduler, websocket, or outbox path was found.
- Page auto-refreshes after a short freshness window and pull refresh can reload backend truth.
- On successful reply, the page patches the local row from the response instead of doing a full list reload.
- Content-safety provider failure blocks persistence; there is no queued retry for reply.

## Frontend Draft And Backend Rehydration

- Reply popup draft is local and discarded on close.
- Opening an already replied review pre-fills the current backend/list reply text.
- Submit response rehydrates the updated row and clears popup state.
- Silent list refresh keeps previous list when reload fails.

## Test Coverage Signals

Observed tests:

- API tests cover merchant reply happy path and content-safety call behavior.
- SQL tests cover `UpdateMerchantReply`.
- API tests cover merchant review list/count and review image URL enrichment.
- Fixed 2026-06-06: API/sqlc tests cover hidden-review reply visibility, proving merchant owner all-list/reply includes hidden reviews, public merchant list stays visible-only, and the customer/user list retains the user's hidden review.
- Fixed 2026-06-06: SQL tests cover reply overwrite/timestamp semantics, proving repeated identical replies refresh `replied_at`, later different replies overwrite the single stored reply, and visibility is unchanged.
- Fixed 2026-06-06: API tests cover merchant all-review image resolver behavior, proving hidden-review all-list returns approved public image URLs, withholds pending review images from merchants, and does not expose review media asset ids.
- Fixed 2026-06-06: Mini Program `check-merchant-review-reply-error-copy` covers missing WeChat OpenID, text content-safety failure, and content-safety provider/gateway failure copy for reply submission.

Missing high-value tests:

- Fixed 2026-06-06: Mini Program owner-access coverage in `weapp/scripts/check-merchant-review-owner-access.test.js` proves `merchant_staff` is denied by the page gate and `merchant_owner` is granted.
- No remaining Mini Program reply failure-copy test gap is known for the currently traced missing OpenID/content-safety branches.

## Gaps And Refactor Notes

- Fixed 2026-06-06: the product path remains owner-only; the Mini Program page copy/access gate is owner-aware and no longer relies on backend denial for non-owner staff.
- Decide whether reply update should preserve reply history or expose a clear/delete reply action.
- Fixed 2026-06-06: repeated identical reply semantics are explicit and non-idempotent; the backend treats it as a new edit and refreshes `replied_at`.
- Fixed 2026-06-06: merchant all-review image URLs follow the public resolver contract; pending review images remain visible only to the uploading user through the owner-view resolver.
- Fixed 2026-06-06: review reply failure copy is task-owned in the Mini Program and does not expose backend English, OpenID terminology, or WeChat provider diagnostics for the traced reply submit failures.

## Branch Exhaustion

- Entry branches checked: Mini Program review list, reply popup, existing reply edit, all-review filter/count, hidden review visibility, public merchant review list, user review list, and content-safety provider path. Flutter App has no review management/reply entry in `merchant_app/lib/features/**`.
- Request branches checked: merchant all-review list/count, merchant reply POST/PATCH path, public merchant review list, user review list, customer review create/update/delete adjacency, operator visibility updates, media image URL enrichment, and WeChat content-safety check.
- Backend state branches checked: `reviews.merchant_reply`, `replied_at`, `is_visible`, review image rows, single overwrite semantics, hidden review inclusion for merchant owner, public visible-only readers, and customer-owned review writers outside this slice.
- Async branches checked: none found for merchant reply. Content-safety is synchronous and blocks persistence; page refresh/re-entry is the only recovery path.
- Failure/retry branches checked: duplicate reply submit guard, last-write-wins reply, repeated identical reply updating timestamp, no version/history/delete endpoint, missing OpenID/provider failure, local row patch after success, silent list refresh failure, and hidden-review image public-resolver behavior.
- Reader/consumer branches checked: merchant review list/count, public merchant review list, user review list, review image resolver, dashboard/stat counts if any, and customer-visible reply fields.
- Authorization/tenant branches checked: Mini Program owner-aware review-management access, backend owner-only review management routes, merchant ownership check for reply, route merchant id validation for list, and content-safety check using merchant user's WeChat OpenID.
- Zombie/unreachable branches checked: non-owner page entry drift is fixed; no clear/delete/history path despite reply edit UI; merchant hidden-review images use public URL resolver and withhold pending images; managers' product permission remains owner-only unless product later changes backend middleware.
- Test-proof gaps checked: existing tests cover reply happy path/content-safety call, SQL update, list/count, image enrichment, Mini Program non-owner page denial, hidden-review reply visibility across merchant/public/user readers, repeated identical reply timestamp-refresh semantics, merchant all-review public image resolver behavior, and Mini Program missing OpenID/content-safety reply failure copy.
