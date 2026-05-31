# Merchant Profile Update Slice

Status: third merchant-state flow slice
Risk class: G2 - merchant-visible profile state spans multiple truth sources, has media async recovery, optimistic locking, and partial-write risk
Scope: merchant profile settings page and profile-images page -> profile/category/media/shop-image persistence -> public/storefront readers

## Variant Coverage

This slice covers:

- Merchant basic profile edit: name, phone, address, description, latitude, longitude.
- Merchant category selection on the profile page.
- Merchant logo upload/remove on the profile-images page.
- Storefront and environment image upload/remove on the profile-images page.
- Media upload session, direct upload, complete, and public URL polling used by the image page.
- Consumer/public readers that expose merchant name, logo, cover image, tags, and location.

This slice does not cover:

- Merchant onboarding document/OCR workflow except where profile-images reuses the application endpoint for shop-image truth.
- Operator-side merchant profile management.
- Payment/settlement profile submission.
- Changing the discovered partial-write and application-row scoping issues.

## Product Invariant

When a merchant changes shop-facing profile data, the backend durable truth must converge before the merchant-visible page claims success. Basic profile and logo changes must use the merchant version as an optimistic lock. Category replacement should be all-or-nothing. Storefront/environment images should update one authoritative record for the current merchant and must not drift between local pending state, media asset state, and public/search readers.

Current implementation meets the optimistic-lock invariant for basic profile and logo. Category and shop-image persistence have weaker boundaries documented below.

## Primary Forward Chain

1. The profile settings page loads current merchant profile and category state after merchant console access. `loadProfile` reads `GET /v1/merchants/me`; `loadCategories` reads selected merchant tags and all available merchant tags.
   Evidence: `weapp/miniprogram/pages/merchant/settings/profile/index.ts:153`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:154`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:157`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:205`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:207`.

2. The profile form keeps a local draft and `version`. Location selection writes address, latitude, and longitude into the draft. Pull refresh is blocked when unsaved changes exist.
   Evidence: `weapp/miniprogram/pages/merchant/settings/profile/index.ts:158`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:241`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:352`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:79`.

3. `onSave` builds a PATCH payload with the current `version`, writes `PATCH /v1/merchants/me`, then rehydrates the form and version from the backend response.
   Evidence: `weapp/miniprogram/pages/merchant/settings/profile/index.ts:440`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:445`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:454`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:457`.

4. If categories changed, the same save flow then calls `PUT /v1/merchants/me/tags` and rehydrates category state from the response.
   Evidence: `weapp/miniprogram/pages/merchant/settings/profile/index.ts:484`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:485`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:486`, `weapp/miniprogram/pages/merchant/settings/profile/index.ts:488`.

5. Frontend API wrappers map profile and logo to `GET/PATCH /v1/merchants/me`.
   Evidence: `weapp/miniprogram/api/merchant.ts:513`, `weapp/miniprogram/api/merchant.ts:526`, `weapp/miniprogram/api/merchant.ts:544`.

6. Backend routes register profile read, tag read, and write endpoints. Profile, shop-images, status, business-hours, and tags writes share the merchant profile write group for owner/manager roles.
   Evidence: `locallife/api/server.go:667`, `locallife/api/server.go:671`, `locallife/api/server.go:674`, `locallife/api/server.go:682`, `locallife/api/server.go:683`, `locallife/api/server.go:686`.

7. `updateCurrentMerchant` binds optional profile fields and required `version`, resolves the current merchant from auth context, checks the current version, validates coordinates, and calls either `UpdateMerchant` or `ClearMerchantLogo`.
   Evidence: `locallife/api/merchant.go:248`, `locallife/api/merchant.go:275`, `locallife/api/merchant.go:286`, `locallife/api/merchant.go:300`, `locallife/api/merchant.go:314`, `locallife/api/merchant.go:342`, `locallife/api/merchant.go:371`.

8. `UpdateMerchant` and `ClearMerchantLogo` update `merchants` with `WHERE id = ... AND version = ... AND deleted_at IS NULL`, incrementing `version`.
   Evidence: `locallife/db/query/merchant.sql:114`, `locallife/db/query/merchant.sql:126`, `locallife/db/query/merchant.sql:128`, `locallife/db/query/merchant.sql:133`, `locallife/db/query/merchant.sql:137`.

9. `setMerchantTags` validates every requested tag exists and has type `merchant`, then clears all merchant tags and inserts requested tag IDs before returning the latest list.
   Evidence: `locallife/api/tag.go:241`, `locallife/api/tag.go:263`, `locallife/api/tag.go:280`, `locallife/api/tag.go:285`, `locallife/api/tag.go:296`.

10. SQL for merchant tags is a separate delete and insert path over `merchant_tags`.
    Evidence: `locallife/db/query/merchant.sql:244`, `locallife/db/query/merchant.sql:256`, `locallife/db/query/merchant.sql:270`.

11. The profile-images page loads `GET /v1/merchants/me` for logo/version and `GET /v1/merchant/application` for storefront/environment image JSON. If application fetch fails in non-strict mode, it preserves local image state and schedules pending persistence when needed.
    Evidence: `weapp/miniprogram/pages/merchant/profile-images/index.ts:91`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:93`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:96`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:115`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:117`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:168`.

12. Logo upload calls the media upload wrapper, then persists the resulting `mediaId` through `PATCH /v1/merchants/me` with the current merchant version.
    Evidence: `weapp/miniprogram/pages/merchant/profile-images/index.ts:230`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:246`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:263`, `weapp/miniprogram/api/merchant.ts:526`.

13. Media upload uses session creation, direct OSS/dev upload, and completion. Completion creates/returns a media asset and can trigger moderation; frontend can later poll `GET /v1/media/:id` for public display URLs.
    Evidence: `weapp/miniprogram/utils/media.ts:271`, `weapp/miniprogram/utils/media.ts:279`, `weapp/miniprogram/utils/media.ts:281`, `locallife/api/media.go:76`, `locallife/api/media.go:133`, `locallife/api/media.go:164`, `locallife/api/media.go:165`, `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts:858`.

14. Storefront/environment upload persists URL arrays through `PATCH /v1/merchants/me/shop-images`. Ambiguous persistence failures set retry state instead of immediately discarding local state.
    Evidence: `weapp/miniprogram/pages/merchant/profile-images/index.ts:376`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:400`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:412`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:422`, `weapp/miniprogram/pages/merchant/_utils/merchant-profile-images-lifecycle.ts:354`, `weapp/miniprogram/pages/merchant/_utils/merchant-profile-images-lifecycle.ts:358`.

15. `updateCurrentMerchantShopImages` validates max counts, normalizes URLs, writes `merchant_applications.storefront_images/environment_images`, decodes the stored JSON, resolves public URLs, and returns arrays to the client.
    Evidence: `locallife/api/merchant.go:414`, `locallife/api/merchant.go:422`, `locallife/api/merchant.go:431`, `locallife/api/merchant.go:434`, `locallife/api/merchant.go:449`, `locallife/api/merchant.go:461`, `locallife/api/merchant.go:467`, `locallife/api/merchant.go:483`.

16. Public/storefront readers use these fields: public merchant detail returns profile fields, logo, tags, and first storefront cover; search/category list rows include merchant tags and `merchant_applications.storefront_images`.
    Evidence: `locallife/api/merchant.go:1195`, `locallife/api/merchant.go:1209`, `locallife/api/merchant.go:1239`, `locallife/api/merchant.go:1257`, `locallife/api/search.go:888`, `locallife/api/search.go:900`, `locallife/db/query/merchant.sql:176`, `locallife/db/query/merchant.sql:194`.

## Reverse-Reference Findings

- Basic merchant profile fields and logo are shared by dashboard, current-merchant cache, public merchant detail, search/listing, dine-in/table/cart/order display, membership display, and review/favorite/order response builders.
- Storefront/environment images for approved merchants are read from `merchant_applications`, the same table family used by onboarding. The profile-images page writes through `/v1/merchants/me/shop-images`, while onboarding draft image update writes through `/v1/merchant/application/images`.
- Merchant categories are read by the profile page, public merchant detail, search/category filtering, and category list aggregation.
- Duplicated `_main_shared/api/onboarding.ts` copies exist under merchant, operator, register, and user_center path trees; the merchant profile-images page uses the merchant copy. These are drift candidates if one copy changes upload or shop-image behavior without the others.

## SQL And Durable State Boundaries

- `merchants.name`, `phone`, `address`, `description`, `latitude`, `longitude`: basic shop profile truth.
- `merchants.logo_media_asset_id`: logo media truth, rendered to `logo_url` by `publicImageURL`.
- `merchants.version`: optimistic-lock token for profile/logo writes.
- `merchant_tags`: selected merchant categories.
- `tags`: available category dictionary, filtered by `type='merchant'` and `status='active'`.
- `media_upload_sessions` and `media_assets`: media upload/session/asset truth, including moderation and public URL variants.
- `merchant_applications.storefront_images` and `environment_images`: storefront/environment image URL-array truth for this page.

## Trust, Authorization, And Tenant Checks

- Frontend pages call `ensureMerchantConsoleAccess` before loading.
- Backend writes under `/v1/merchants/me` require merchant profile write roles `owner` or `manager`.
- Profile and tag handlers resolve the merchant from the authenticated user and do not accept a client-supplied merchant id.
- Media complete verifies upload session ownership via authenticated user.
- `GET /v1/media/:id` checks owner only for private assets; public assets can be read by any authenticated user, which matches public media behavior.
- Shop-image update uses authenticated `user_id`, not merchant id, but it does not call `resolveMerchantForUser` in the handler itself. The route middleware enforces profile-write role first.

## Idempotency And Duplicate-Submit Checks

- Profile page blocks duplicate save with `saving`.
- Profile/logo PATCH is guarded by `merchants.version`; stale writes fail with conflict and frontend reloads profile truth.
- Logo upload can leave an uploaded media asset unreferenced if the subsequent merchant PATCH conflicts or fails.
- Shop-image upload/remove uses per-section saving flags and generation checks. The backend PATCH is last-write-wins over provided URL arrays.
- Frontend retries ambiguous shop-image persistence failures with backoff and eventually rehydrates from server truth.
- Tag PUT has no idempotency key. Repeating the same successful request converges to the same category set, but the implementation is not atomic.

## Recovery And Async Convergence Paths

- Profile page silent refresh preserves previously loaded state on refresh failure and blocks pull-to-refresh with unsaved changes.
- Version conflict reloads the profile and tells the merchant to retry from latest data.
- Profile-images `finalizePendingLogo` and `finalizePendingShopImage` poll media asset URLs after moderation/public variants become available.
- `resumePendingImageRecovery` retries images that have asset IDs but are not persisted in the application image arrays.
- `flushPendingShopImagesPersistence` retries pending shop-image persistence and can fall back to server truth on explicit sync failures.
- No backend scheduler/worker was found for shop-image convergence. The async recovery is primarily frontend-side plus media moderation/public URL generation.

## Frontend Draft And Backend Rehydration

- Basic profile edits are local draft until save; save rehydrates from `PATCH /v1/merchants/me`.
- Categories are local selection state until save; save rehydrates from `PUT /v1/merchants/me/tags`.
- Logo update is immediate once the media asset is uploaded and PATCH succeeds; the page stores the returned version.
- Storefront/environment images are semi-optimistic: local images appear during upload and are reconciled with `/shop-images` response or later application reload.
- If application read fails during image load, the page can keep local image state and schedule persistence retry, which is useful for weak networks but makes backend truth harder to see at a glance.

## Test Coverage Signals

Observed tests:

- `locallife/api/merchant_test.go` covers merchant update version behavior and `GET /v1/merchants/me` logo URL resolution.
- `locallife/api/merchant_test.go` covers invalid stored shop-image JSON returning internal server error.
- `locallife/api/security_authz_test.go` denies unauthorized profile/shop-image/tag writes.
- `locallife/api/media_test.go` and `media_moderation_test.go` cover media upload session/complete idempotency and moderation/public URL behavior.
- `locallife/db/sqlc/merchant_test.go` covers basic merchant tag insert/list and `GetMerchantWithTags`.

Missing high-value tests:

- API or DB test proving `PUT /v1/merchants/me/tags` is atomic under insert failure.
- DB/API test proving `/v1/merchants/me/shop-images` updates exactly one intended application row for the current merchant/user.
- Frontend or integration test for pending shop-image persistence recovery after ambiguous network failure.
- Test for logo PATCH conflict after a media upload, including local rollback/user guidance.

## Gaps And Refactor Notes

- `setMerchantTags` comment says atomic replace, but the code clears old tags and inserts new tags without an explicit transaction. This can leave partial categories on failure.
- `UpdateMerchantApplicationShopImages` uses `WHERE user_id = $1 RETURNING *` without status/order/limit. If multiple application rows exist for a user, the write target is ambiguous and may affect or return more than one row.
- Storing approved merchant shop images on `merchant_applications` couples live storefront assets to onboarding records. A merchant-owned image table or explicitly selected latest approved application would be clearer.
- `json.Marshal` errors are ignored for string slices in shop-image handler. They are practically impossible for `[]string`, but the pattern weakens the persisted-data standard.
- Duplicated onboarding API files under multiple Mini Program role trees should be treated as drift candidates before changing shared upload behavior.

## Branch Exhaustion

- Entry branches checked: Mini Program profile settings, logo upload, merchant category/tag selection, profile-images page, storefront/environment image upload/remove/retry, media upload session/complete/read, current merchant cache readers, public merchant detail/search, and onboarding image overlap. Flutter App has no merchant profile editing entry in `merchant_app/lib/features/**`.
- Request branches checked: `GET/PATCH /v1/merchants/me`, `GET/PUT /v1/merchants/me/tags`, `GET /v1/tags?type=merchant`, media upload session/complete/read, `PATCH /v1/merchants/me/shop-images`, `GET /v1/merchant/application`, and application image writeback SQL.
- Backend state branches checked: merchant basic fields, logo media asset id, optimistic version, merchant tags, global tag dictionary, media sessions/assets/moderation/public URL variants, application storefront/environment URL arrays, and approved/draft application image ownership.
- Async branches checked: media moderation/public URL availability, frontend polling for media URL, pending logo/shop-image recovery, shop-image persistence retry/backoff, and application reload. No backend scheduler/worker was found for shop-image convergence.
- Failure/retry branches checked: profile version conflict, uploaded logo asset orphan after PATCH conflict/failure, non-transactional tag replace, shop-image last-write-wins, ambiguous application row update by user id, application read failure while keeping local image state, ignored JSON marshal pattern, and duplicated role API drift.
- Reader/consumer branches checked: dashboard/current merchant cache, public merchant list/detail/search, dish/order/cart display of logo/name/address, profile images page, merchant application page, media readers, and tag/category displays.
- Authorization/tenant branches checked: Mini Program merchant console access, backend owner/manager profile-write routes, server-side merchant resolution, media session owner checks, private/public media read distinction, shop-image route middleware plus user-id scoped update, and tag route current merchant resolution.
- Zombie/unreachable branches checked: live storefront images are stored on application rows rather than merchant-owned state; `UpdateMerchantApplicationShopImages` can target multiple/no unstable rows; duplicated onboarding APIs can drift; tag replace claims atomic behavior without transaction.
- Test-proof gaps checked: existing tests cover profile version/logo URL, invalid shop-image JSON, authz denial, media upload/moderation, and tag SQL basics. Missing proof remains for atomic tag replace, exact shop-image application scoping, pending image recovery, and logo conflict rollback/copy.
