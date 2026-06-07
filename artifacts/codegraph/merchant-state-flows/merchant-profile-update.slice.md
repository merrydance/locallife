# Merchant Profile Update Slice

Status: third merchant-state flow slice; category replacement and shop-image latest-row scoping fixes applied 2026-06-06; Logo conflict recovery, shop-image pending-sync proof, and merchant-owned live shop-image truth applied 2026-06-07
Risk class: G2 - merchant-visible profile state spans multiple truth sources, has media async recovery, optimistic locking, and remaining product-truth schema risk
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
- Historical migration cleanup after live storefront/environment image truth moved onto `merchants`.

## Product Invariant

When a merchant changes shop-facing profile data, the backend durable truth must converge before the merchant-visible page claims success. Basic profile and logo changes must use the merchant version as an optimistic lock. Category replacement should be all-or-nothing. Storefront/environment images should update one authoritative record for the resolved merchant owner and must not drift between local pending state, media asset state, and public/search readers.

Current implementation meets the optimistic-lock invariant for basic profile and logo. Fixed 2026-06-06: category replacement now rejects duplicate tag IDs before write and uses a store transaction that locks the merchant row before replacing tag rows. Fixed 2026-06-06: shop-image persistence was narrowed to one latest editable application row while the live-truth migration was pending. Fixed 2026-06-07: profile-images Logo PATCH conflicts now roll back to the previous trusted Logo, show conflict-specific retry copy, and force a latest profile reload with server Logo truth preferred and recovery refresh failures kept silent to avoid duplicate Toast feedback. Fixed 2026-06-07: storefront/environment pending-sync recovery now has Mini Program proof for ambiguous persistence, missing server echo, reload retry, and media public-URL polling branches. Fixed 2026-06-07: live storefront/environment images now live on `merchants.storefront_images/environment_images`; `merchant_applications` remains onboarding/draft material and only serves as compatibility fallback when the merchant live field is `NULL` and the owner has exactly one non-deleted merchant. Migration backfill is conservative: it copies valid images from the latest approved application only for owners with one non-deleted merchant, safely ignores historical non-array/non-string/over-limit JSONB, and leaves ambiguous multi-store owners without application fallback until a merchant-specific write or reconciliation. Empty live arrays mean the merchant intentionally cleared images and must not fall back.

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

9. `setMerchantTags` validates every requested tag exists, rejects duplicate tag IDs, verifies each tag has type `merchant`, then delegates replace-all to `SetMerchantTagsTx` and returns the transaction result.
   Evidence: `locallife/api/tag.go:241`, `locallife/api/tag.go:263`, `locallife/api/tag.go:265`, `locallife/api/tag.go:271`, `locallife/api/tag.go:286`, `locallife/api/tag.go:295`.

10. `SetMerchantTagsTx` wraps `LockMerchantForUpdate`, `ClearMerchantTags`, per-tag `AddMerchantTag`, and `ListMerchantTags` in `execTx`, so same-merchant replacements serialize and insert/list failures roll back the clear step.
    Evidence: `locallife/db/sqlc/tx_merchant_tags.go:17`, `locallife/db/sqlc/tx_merchant_tags.go:21`, `locallife/db/sqlc/tx_merchant_tags.go:22`, `locallife/db/sqlc/tx_merchant_tags.go:26`, `locallife/db/sqlc/tx_merchant_tags.go:30`, `locallife/db/sqlc/tx_merchant_tags.go:39`.

11. The profile-images page loads `GET /v1/merchants/me` for logo/version and merchant-owned storefront/environment image JSON. It also loads `GET /v1/merchant/application` as a compatibility fallback when merchant live image fields are absent. If application fetch fails in non-strict mode, it preserves local image state and schedules pending persistence when needed.
    Evidence: `weapp/miniprogram/pages/merchant/profile-images/index.ts:91`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:93`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:96`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:115`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:117`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:168`.

12. Logo upload calls the media upload wrapper, then persists the resulting `mediaId` through `PATCH /v1/merchants/me` with the current merchant version.
    Evidence: `weapp/miniprogram/pages/merchant/profile-images/index.ts:230`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:246`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:263`, `weapp/miniprogram/api/merchant.ts:526`.

13. Media upload uses session creation, direct OSS/dev upload, and completion. Completion creates/returns a media asset and can trigger moderation; frontend can later poll `GET /v1/media/:id` for public display URLs.
    Evidence: `weapp/miniprogram/utils/media.ts:271`, `weapp/miniprogram/utils/media.ts:279`, `weapp/miniprogram/utils/media.ts:281`, `locallife/api/media.go:76`, `locallife/api/media.go:133`, `locallife/api/media.go:164`, `locallife/api/media.go:165`, `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts:858`.

14. Storefront/environment upload persists URL arrays through `PATCH /v1/merchants/me/shop-images`. Ambiguous persistence failures set retry state instead of immediately discarding local state.
    Evidence: `weapp/miniprogram/pages/merchant/profile-images/index.ts:376`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:400`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:412`, `weapp/miniprogram/pages/merchant/profile-images/index.ts:422`, `weapp/miniprogram/pages/merchant/_utils/merchant-profile-images-lifecycle.ts:354`, `weapp/miniprogram/pages/merchant/_utils/merchant-profile-images-lifecycle.ts:358`.

15. `updateCurrentMerchantShopImages` validates max counts, requires the current merchant from middleware context, normalizes URLs, serializes the image arrays with explicit error handling, writes `merchants.storefront_images/environment_images` for the resolved merchant id, decodes the stored JSON, resolves public URLs, and returns arrays to the client. `UpdateMerchantShopImages` updates only the merchant row and leaves application image JSON untouched.
    Evidence: `locallife/api/merchant.go:449`, `locallife/api/merchant.go:453`, `locallife/api/merchant.go:458`, `locallife/api/merchant.go:464`, `locallife/api/merchant.go:471`, `locallife/api/merchant.go:482`, `locallife/api/merchant.go:490`, `locallife/api/merchant.go:502`, `locallife/api/merchant.go:513`, `locallife/db/query/merchant.sql:156`, `locallife/db/query/merchant.sql:161`, `locallife/db/query/merchant.sql:165`.

16. Public/storefront readers use these fields: public merchant detail returns profile fields, logo, tags, and the first merchant live storefront cover, falling back to the latest approved application cover only when the live merchant field is `NULL` and the owner has exactly one non-deleted merchant; search/category list rows use `COALESCE(m.storefront_images, ma.storefront_images)` with the same approved-only single-merchant-owner fallback so migrated merchant truth wins while unmigrated single-store rows stay compatible.
    Evidence: `locallife/api/merchant.go:1225`, `locallife/api/merchant.go:1239`, `locallife/api/merchant.go:1269`, `locallife/api/merchant.go:1281`, `locallife/api/search.go:888`, `locallife/api/search.go:900`, `locallife/db/query/merchant.sql:208`, `locallife/db/query/merchant.sql:213`, `locallife/db/query/merchant.sql:658`, `locallife/db/query/merchant.sql:663`.

## Reverse-Reference Findings

- Basic merchant profile fields and logo are shared by dashboard, current-merchant cache, public merchant detail, search/listing, dine-in/table/cart/order display, membership display, and review/favorite/order response builders.
- Storefront/environment images for approved merchants are now read from `merchants.storefront_images/environment_images`; `merchant_applications` remains the onboarding/draft image source and compatibility fallback for live merchant rows that still have `NULL` image fields.
- Merchant categories are read by the profile page, public merchant detail, search/category filtering, and category list aggregation.
- Duplicated `_main_shared/api/onboarding.ts` copies exist under merchant, operator, register, and user_center path trees; the merchant profile-images page uses the merchant copy. These are drift candidates if one copy changes upload or shop-image behavior without the others.

## SQL And Durable State Boundaries

- `merchants.name`, `phone`, `address`, `description`, `latitude`, `longitude`: basic shop profile truth.
- `merchants.logo_media_asset_id`: logo media truth, rendered to `logo_url` by `publicImageURL`.
- `merchants.version`: optimistic-lock token for profile/logo writes.
- `merchant_tags`: selected merchant categories.
- `tags`: available category dictionary, filtered by `type='merchant'` and `status='active'`.
- `media_upload_sessions` and `media_assets`: media upload/session/asset truth, including moderation and public URL variants.
- `merchants.storefront_images` and `environment_images`: live storefront/environment image URL-array truth for approved merchant profile display.
- `merchant_applications.storefront_images` and `environment_images`: onboarding/draft image material and compatibility fallback while merchant live image fields are `NULL`.

## Trust, Authorization, And Tenant Checks

- Frontend pages call `ensureMerchantConsoleAccess` before loading.
- Backend writes under `/v1/merchants/me` require merchant profile write roles `owner` or `manager`.
- Profile and tag handlers resolve the merchant from the authenticated user and do not accept a client-supplied merchant id.
- Media complete verifies upload session ownership via authenticated user.
- `GET /v1/media/:id` checks owner only for private assets; public assets can be read by any authenticated user, which matches public media behavior.
- Shop-image update uses the resolved merchant id, not the authenticated staff user's id or owner application row. The route middleware enforces profile-write role first, and the handler fails closed without writing if middleware did not bind merchant context.

## Idempotency And Duplicate-Submit Checks

- Profile page blocks duplicate save with `saving`.
- Profile/logo PATCH is guarded by `merchants.version`; stale profile writes fail with conflict and frontend reloads profile truth.
- Logo upload now restores the previous trusted Logo and forces profile reload when the subsequent merchant PATCH hits a version conflict. The already uploaded media asset can still remain unreferenced after PATCH conflict or failure.
- Shop-image upload/remove uses per-section saving flags and generation checks. The backend PATCH is last-write-wins over provided URL arrays.
- Frontend retries ambiguous shop-image persistence failures with backoff and eventually rehydrates from server truth.
- Tag PUT has no idempotency key. Repeating the same successful request converges to the same category set, duplicate tag IDs are rejected as 400, and replacement is transaction-backed with a merchant row lock.

## Recovery And Async Convergence Paths

- Profile page silent refresh preserves previously loaded state on refresh failure and blocks pull-to-refresh with unsaved changes.
- Version conflict reloads the profile and tells the merchant to retry from latest data.
- Profile-images Logo version conflict restores the previous Logo, maps the 409/409xx conflict to product retry copy, and forces `loadData(false, true, false, { preferServerLogo: true, suppressRefreshErrorToast: true })` so the page version and Logo are refreshed from backend truth without a second failure Toast.
- Profile-images `finalizePendingLogo` and `finalizePendingShopImage` poll media asset URLs after moderation/public variants become available.
- `resumePendingImageRecovery` retries images that have asset IDs but are not persisted in the application image arrays.
- `flushPendingShopImagesPersistence` retries pending shop-image persistence and can fall back to server truth on explicit sync failures.
- No backend scheduler/worker was found for shop-image convergence. The async recovery is primarily frontend-side plus media moderation/public URL generation.

## Frontend Draft And Backend Rehydration

- Basic profile edits are local draft until save; save rehydrates from `PATCH /v1/merchants/me`.
- Categories are local selection state until save; save rehydrates from `PUT /v1/merchants/me/tags`.
- Logo update is immediate once the media asset is uploaded and PATCH succeeds; the page stores the returned version. On version conflict, it restores the previous local Logo and refreshes latest backend profile truth before the merchant retries.
- Storefront/environment images are semi-optimistic: local images appear during upload and are reconciled with `/shop-images` response or later merchant live-image reload.
- If application read fails during image load, the page still uses merchant live image truth when present. Only merchants whose live image fields are absent depend on application fallback, and the page can keep local image state plus schedule persistence retry for weak-network compatibility.

## Test Coverage Signals

Observed tests:

- `locallife/api/merchant_test.go` covers merchant update version behavior and `GET /v1/merchants/me` logo URL resolution.
- `locallife/api/merchant_test.go` covers invalid stored shop-image JSON returning internal server error.
- `locallife/api/merchant_test.go` covers staff/manager shop-image writes using the resolved merchant id, current-merchant public URL resolution for live image arrays, invalid live image JSON failure, public detail live-cover preference, approved-only `NULL` application fallback, draft no-fallback, and empty live-array no-fallback semantics.
- `locallife/api/merchant_test.go` covers fail-closed shop-image handling when merchant context is missing.
- `locallife/api/security_authz_test.go` denies unauthorized profile/shop-image/tag writes.
- `locallife/api/media_test.go` and `media_moderation_test.go` cover media upload session/complete idempotency and moderation/public URL behavior.
- `locallife/api/tag_test.go` covers `PUT /v1/merchants/me/tags` calling `SetMerchantTagsTx` and rejecting duplicate tag IDs before write.
- `locallife/db/sqlc/merchant_test.go` covers basic merchant tag insert/list, `GetMerchantWithTags`, missing-merchant rejection, and rollback of `SetMerchantTagsTx` when replacement insertion fails.
- `locallife/db/sqlc/merchant_test.go` covers `UpdateMerchantShopImages` updating only the merchant row while leaving application image JSON unchanged.
- `locallife/db/sqlc/merchant_test.go` covers `GetMerchantApplicationDraft`, `SearchMerchants`, and `SearchMerchantsByTag`; search readers prefer merchant live images over approved-only application fallback, and multi-store owners do not receive ambiguous application fallback images.
- `locallife/db/sqlc/tx_merchant_application_test.go` covers approval copying application images into merchant live truth and creating the applicant's own merchant when the applicant is only staff on another merchant.
- `weapp/scripts/check-merchant-profile-images-logo-conflict-recovery.test.js` covers Logo PATCH conflict detection, rollback to previous trusted Logo, conflict-specific copy, and forced profile refresh after conflict.
- `weapp/scripts/check-merchant-profile-images-pending-sync-recovery.test.js` covers storefront/environment pending-sync recovery: ambiguous persistence keeps the local pending image and schedules retry, confirmed server echo clears pending markers, reload resumes retry when local asset ids are absent from server truth, and missing public URLs resume media polling.

Missing high-value tests:

- No missing frontend proof remains for the currently implemented profile-images recovery branches. Future tests should be added if product changes the compatibility fallback or retires application image fallback entirely.

## Gaps And Refactor Notes

- Fixed 2026-06-06: `setMerchantTags` now rejects duplicate tag IDs and calls `SetMerchantTagsTx`; the transaction locks the merchant row before replacing tags, and DB coverage proves missing merchants fail plus old categories remain after a replacement insert failure.
- Fixed 2026-06-06: `UpdateMerchantApplicationShopImages` now targets a single latest editable application row with `ORDER BY created_at DESC, id DESC LIMIT 1`, and the handler passes the resolved merchant owner user id so manager staff cannot write their own nonexistent or wrong application row. The handler also fails closed if the expected merchant context is absent.
- Fixed 2026-06-06: shop-image handler now checks `json.Marshal` errors before storing image JSON.
- Fixed 2026-06-06: `SearchMerchants` and `SearchMerchantsByTag` now use one latest editable application row, avoiding duplicate merchant rows or older storefront covers when a merchant owner has multiple application records.
- Fixed 2026-06-07: Logo PATCH conflict after media upload now uses a dedicated Logo conflict mapper, restores the previous Logo state, shows retry guidance, and forces a latest profile reload so the optimistic version is refreshed.
- Fixed 2026-06-07: storefront/environment pending-sync recovery proof now exercises helper behavior for unconfirmed persistence, confirmed server echo, reload retry, and public URL polling.
- Fixed 2026-06-07: approved merchant live shop images moved from `merchant_applications` to `merchants.storefront_images/environment_images`. Migration `000249` adds string-array/count constraints, safely rejects non-array and non-string-array merchant writes, and backfills valid image JSON from the latest approved application only for owners with one non-deleted merchant; approval copies only valid application image arrays into merchant live truth so invalid legacy application JSON does not block merchant approval; profile PATCH writes merchant rows; public/search readers prefer merchant live truth and only fall back to latest approved application rows while live fields are `NULL` and the owner has exactly one non-deleted merchant.
- Duplicated onboarding API files under multiple Mini Program role trees should be treated as drift candidates before changing shared upload behavior.

## Branch Exhaustion

- Entry branches checked: Mini Program profile settings, logo upload, merchant category/tag selection, profile-images page, storefront/environment image upload/remove/retry, media upload session/complete/read, current merchant cache readers, public merchant detail/search, and onboarding image overlap. Flutter App has no merchant profile editing entry in `merchant_app/lib/features/**`.
- Request branches checked: `GET/PATCH /v1/merchants/me`, `GET/PUT /v1/merchants/me/tags`, `GET /v1/tags?type=merchant`, media upload session/complete/read, `PATCH /v1/merchants/me/shop-images`, `GET /v1/merchant/application`, merchant live image writeback SQL, and application image fallback reads.
- Backend state branches checked: merchant basic fields, logo media asset id, optimistic version, merchant tags, global tag dictionary, media sessions/assets/moderation/public URL variants, merchant live storefront/environment URL arrays, application draft/fallback storefront/environment URL arrays, and approved/draft application image ownership.
- Async branches checked: media moderation/public URL availability, frontend polling for media URL, pending logo/shop-image recovery, shop-image persistence retry/backoff, and application reload. No backend scheduler/worker was found for shop-image convergence.
- Failure/retry branches checked: profile version conflict, fixed Logo conflict rollback/copy/refresh after media upload, uploaded logo asset orphan after PATCH conflict/failure, transaction-backed tag replace rollback, duplicate tag-id rejection, shop-image last-write-wins on merchant live fields, conservative latest-approved/single-merchant-owner migration/read fallback versus ambiguous multi-store no-fallback, historical invalid application JSONB ignored during migration and approval copy, application read failure while keeping local image state, `NULL` fallback versus empty-array no-fallback semantics, checked JSON marshal branch, and duplicated role API drift.
- Reader/consumer branches checked: dashboard/current merchant cache, public merchant list/detail/search, dish/order/cart display of logo/name/address, profile images page, merchant application page, media readers, and tag/category displays.
- Authorization/tenant branches checked: Mini Program merchant console access, backend owner/manager profile-write routes, server-side merchant resolution, media session owner checks, private/public media read distinction, shop-image route middleware plus fail-closed resolved owner-user update, and tag route current merchant resolution.
- Zombie/unreachable branches checked: live storefront images are now merchant-owned; the older application image writer remains for onboarding/draft material and compatibility history; duplicated onboarding APIs can drift.
- Test-proof gaps checked: existing tests cover profile version/logo URL, invalid shop-image JSON, authz denial, media upload/moderation, tag SQL basics, transactional tag replacement, duplicate tag-id rejection, missing-merchant rejection, rollback on tag replacement failure, shop-image staff-merchant resolution, fail-closed missing merchant context, merchant-owned shop-image update, search/category merchant-live image preference, single-store application fallback, multi-store no-fallback, approval copy into live truth, Logo conflict rollback/copy/refresh, storefront/environment pending-sync recovery, and application fallback semantics. Remaining proof should follow any future removal of application fallback or historical cleanup migration.
