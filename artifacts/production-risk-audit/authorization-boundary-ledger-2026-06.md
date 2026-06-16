# Production Risk Audit: Authorization Boundary Ledger

Month: 2026-06
Risk theme: authorization boundaries
Status: active

## Scope

This ledger records actor, role, tenant, region, owner, provider, and resource
boundaries for all reusable codegraph flows. It is a documentation-only audit
and does not change production code.

## Boundary Rules

- Client-supplied user, merchant, rider, region, order, payment, refund, or
  provider identifiers are hints until reloaded and checked server-side.
- Provider callbacks must authenticate provider origin and then derive the
  local owner/resource from durable local rows.
- Operator flows require region authority, not just an operator role.
- Merchant staff flows require both staff role and resource ownership.
- Rider flows resolve rider identity from the authenticated user before any
  state mutation.
- Customer flows scope reads and writes to the authenticated user unless a
  documented public discovery route exists.

## Flow Ledger

| Flow ID | Flow | Boundary Owner | Existing Boundary | Gap / Residual Risk | Decision |
| --- | --- | --- | --- | --- | --- |
| AB-BAOFU-AGGREGATE-PAYMENT | Baofoo aggregate payment | Provider + local payment owner | Signature parser, collect identity check, local payment lookup, fact owner derived from payment row. | Successful real callback evidence remains limited. | Keep provider identity separate from business owner derivation. |
| AB-BAOFU-REFUND | Baofoo refund | Provider + local refund/payment owner | Signature, collect identity, local refund row by out-refund-no, payment-order business type routing. | Callback local miss behavior should be validated before parser changes. | Keep callback trust anchored to local refund/payment rows. |
| AB-CUSTOMER-DINE-IN-CHECKOUT | Dine-in checkout | Customer session/table/billing group | Authenticated scan/session routes, table/merchant/session checks, group membership checks. | QR/table trust belongs to merchant/table flow; dedicated source audit before weakening code paths. | Treat QR params as untrusted entry hints. |
| AB-CUSTOMER-DISCOVERY-BROWSE | Discovery and browse | Authenticated customer + public-visible merchant data | Current routes are authenticated; writes use current user id; location/region are query hints. | Public search behavior should be separately reviewed if anonymous browse is introduced. | Keep visibility/orderability filters backend-owned. |
| AB-CUSTOMER-ORDER-AFTER-SALES | Order/refund/claim after-sales | Customer order owner | Customer order/payment/refund/claim/review routes scoped to authenticated user; status actions validate ownership. | Food-safety/claim duplicate ownership rules need source-level review before changes. | Do not trust customer-supplied order/payment ownership. |
| AB-CUSTOMER-PROFILE-WALLET | Profile/address/wallet/reviews | Customer account owner | Profile/address/favorite/review/notification routes use authenticated user id; media access rules apply. | Media visibility and review ownership should be source-audited before media contract changes. | Keep PII and wallet read paths user-scoped. |
| AB-CUSTOMER-RESERVATION | Reservation lifecycle | Customer reservation/payment owner | Reservation/payment/refund routes validate current user and shared reservation/table constraints. | Dine-in handoff and add-on ownership should get source-level proof before changes. | Keep reservation state shared but owner-checked. |
| AB-CUSTOMER-RUNTIME-AUTH | Runtime auth/session | Auth service/session owner | Auth routes issue tokens/sessions; web login session state is explicit; telemetry authenticated. | Cross-device web-login confirmation is high risk and needs fresh source audit before modification. | Fail closed on ambiguous token/session ownership. |
| AB-CUSTOMER-TAKEOUT-CHECKOUT | Takeout checkout/payment | Customer order/payment owner | Authenticated cart/order/payment routes; backend validates ownership, address, pricing, merchant/item state. | Page-local API wrapper copies can drift from backend auth contract. | Backend route ownership remains source of truth. |
| AB-MERCHANT-APP-BIND | Merchant App bind/device | Merchant owner/staff + one-time bind credential | Bind flow rechecks merchant id and one-time credential; device registration scoped by App token/session. | Long-lived token issuance should be source-audited before auth changes. | Preserve consume-after-recheck semantics. |
| AB-MERCHANT-ONBOARDING | Merchant onboarding | Applicant/owner/operator review boundary | Private identity docs, owner authorization, review/recovery state, credential ledger. | Owner-only approval lookup and edit/reset paths already had fixes; re-audit before changing review states. | Keep identity/OCR artifacts private and owner-scoped. |
| AB-MERCHANT-BIZ-HOURS-AUTO | Business-hours auto open | Merchant config owner + scheduler | Merchant writes business hours; scheduler mutates open status from durable config. | Scheduler must not bypass merchant ownership when selecting rows. | Keep automatic writer bounded to stored merchant config. |
| AB-MERCHANT-CLAIM-RECOVERY | Merchant claim/recovery | Merchant/rider/claim owner | Merchant claim APIs tenant-scoped; disputes and recovery payment facts checked against owner. | Manager dispute-create proof exists but should be revalidated before role changes. | Preserve tenant checks on recovery release/payment. |
| AB-MERCHANT-COMBO-CATALOG | Combo/catalog | Merchant owner/staff | Merchant API controls catalog; public readers see visible/orderable state. | Downstream reservation/cart/order readers rely on backend filters. | Do not move catalog ownership to frontend. |
| AB-MERCHANT-DEVICE-DISPLAY | Device/display/printer config | Merchant owner/staff + provider device | Config APIs own auto-accept/print state; backend truth now controls Flutter auto-accept. | Cloud-printer provider credentials/device ownership remain sensitive. | Keep backend config as authority for auto side effects. |
| AB-MERCHANT-DISH-INVENTORY | Dish/inventory | Merchant owner/staff | Merchant-controlled dish/customization/inventory writes; public readers filtered. | Multiple writer ownership needs source-level review before refactor. | Keep merchant ownership checked on every writer. |
| AB-MERCHANT-FINANCE-WITHDRAWAL | Merchant finance/withdrawal | Merchant finance role + Baofu account owner | Finance reads/writes scoped to merchant; manager create permission recorded; provider callbacks derive local order. | Sensitive profile/bank data and provider callback evidence remain high risk. | Keep finance money routes role-gated and provider-verified. |
| AB-MERCHANT-MANUAL-OPEN | Manual open status | Merchant owner/staff | Dashboard/App switch writes merchant availability; scheduler convergence uses durable merchant row. | Multiple writer precedence is state/transaction risk, not an auth shortcut. | Keep staff role checks on manual writes. |
| AB-MERCHANT-MARKETING-RULES | Marketing rules | Merchant owner/staff | Merchant rule APIs own merchant-scoped rule tables; public checkout reads only active backend rules. | Rule stacking can leak into cross-merchant checkout if scoping drifts. | Keep merchant id on rule writes and reads. |
| AB-MERCHANT-MEMBER-BALANCE | Member balance adjustment | Authorized merchant staff + customer membership | Manager adjustment permission repaired; ledger transaction owns balance writes. | Future direct writers are authorization bypass risks. | Maintain ledger-only mutation path. |
| AB-MERCHANT-MEMBERSHIP-SETTINGS | Membership settings | Merchant owner/staff | Settings row is merchant-owned; checkout readers consume backend settings. | Cross-merchant membership setting reads need regression when changing checkout. | Keep settings scoped by merchant. |
| AB-MERCHANT-ORDER-OPS | Merchant order operations | Merchant/kitchen staff + order merchant | Merchant routes require owner/manager/cashier; kitchen owner/manager/chef; logic rechecks order.MerchantID. | Complete transaction is broader than merchant logic and should not be exposed without guards. | Keep service-level ownership checks before shared SQL primitives. |
| AB-MERCHANT-PROFILE-UPDATE | Merchant profile/media | Merchant owner/staff + media owner | Profile/category/media writes are merchant-owned; live shop-image truth scoped. | Media recovery rows can cross owner boundaries if scoping regresses. | Keep media ownership and optimistic lock checks. |
| AB-MERCHANT-RESERVATION-TABLE | Merchant reservation/table | Merchant staff + shared customer reservation | Merchant/table APIs own table/session state; payment/refund callbacks are provider-owned. | No-show and dine-in handoff cross customer/merchant boundary. | Source-audit before modifying shared reservation/table ownership. |
| AB-MERCHANT-REVIEW-REPLY | Review reply | Merchant owner + public review | Merchant all-reviews/reply APIs own public replies after content safety. | Review/reply ownership needs provider moderation failure audit before changes. | Keep reply restricted to review's merchant. |
| AB-MERCHANT-STAFF-GROUP | Staff/group | Merchant/group owner + invite holder | Staff/group routes, invite credentials, OCR, and private docs define boundaries. | Reusable invite credentials are auth-sensitive. | Preserve invite and group membership checks. |
| AB-MERCHANT-STATS-ANALYTICS | Merchant stats | Merchant owner/staff | Read APIs expose merchant revenue/order/customer signals to scoped merchant. | Aggregated contact/profile exposure requires privacy review before broadening roles. | Keep stats read role and tenant scoped. |
| AB-OPERATOR-DASHBOARD | Operator dashboard | Operator region authority | Operator route groups and notification reads scoped to operator/region. | Finance summary exposure should be rechecked before role broadening. | Region authority is mandatory. |
| AB-OPERATOR-DISPATCH-HALL | Dispatch hall | Operator region authority | Pending deliveries filtered by managed region; alert recipients active operator regions. | Cross-role cancellation/refund handoff remains outside operator page. | Do not let operator visibility imply mutation authority. |
| AB-OPERATOR-FINANCE-WITHDRAWAL | Operator finance/withdrawal | Platform/operator finance boundary | Operator finance reads and Baofu admin/provider flows are separate. | Legacy withdrawal boundary requires source audit before changes. | Keep read visibility separate from money movement authority. |
| AB-OPERATOR-MERCHANT-MGMT | Merchant management | Operator region authority | Region checks precede merchant visibility and capability writes. | System-label reconciliation can broaden effects if region scoping drifts. | Preserve region authority on writes and reads. |
| AB-OPERATOR-REGION-RULES | Region rules/expansion | Operator managed region + platform approval | Delivery-fee/peak-hour/rule writes check managed region; platform-only keys read-only. | Region expansion approval handoff crosses platform boundary. | Keep platform-owned approvals out of operator self-service writes. |
| AB-OPERATOR-RIDER-MGMT | Rider management | Operator region authority | Rider reads filtered by managed region and masked identity fields. | Status/deposit mutations live outside this read flow. | Keep management as scoped read unless explicit write route says otherwise. |
| AB-OPERATOR-SAFETY-RECOVERY | Safety/recovery | Operator region + food-safety ownership | Food-safety resolution releases only food-safety-owned suspension; recovery reads region-scoped. | Recovery dispute mutations are merchant/rider/automatic, not operator UI route. | Preserve food-safety-owned reason checks. |
| AB-RIDER-ONBOARDING | Rider onboarding | Applicant/rider + review authority | Authenticated rider application routes, OCR docs, role activation, credential ledger. | Identity document privacy and review authority need source audit before changes. | Keep role activation review-owned. |
| AB-RIDER-CLAIMS-RECOVERY | Rider claims/recovery | Assigned rider/recovery target | Claims scoped through delivery rider id; recovery pay checks rider target. | Inline/worker result side effects need owner checks preserved. | Keep rider id derived from auth context. |
| AB-RIDER-DELIVERY-LIFECYCLE | Delivery lifecycle | Assigned rider | Rider actions resolve rider by user id; status transitions require delivery.rider_id match. | Generic customer-facing delivery detail route has separate ownership semantics. | Rider pages should prefer rider-scoped active/history/detail data. |
| AB-RIDER-DEPOSIT | Rider deposit | Rider account owner + provider callback | Deposit routes resolve rider by auth user; payment query scopes user; refund callback verifies business type. | Generic payment/refund exposure must stay business/user scoped. | Keep deposit state user-owned and provider facts verified. |
| AB-RIDER-INCOME-WITHDRAWAL | Rider income/withdrawal | Rider account owner + Baofu binding | Income reads resolve rider; settlement/withdrawal require rider middleware; callbacks provider-verified. | Baofu personal-account data is sensitive and must not be shared with merchant report paths. | Keep rider settlement docs/code distinct from merchant account flows. |
| AB-RIDER-WORKBENCH-LOCATION | Rider status/location | Rider account owner + active delivery | Status/location routes resolve rider; supplied delivery id must match current active delivery when present. | Notification category filtering is not an auth boundary. | Keep location tied to online rider and active delivery guard. |

## Cross-Flow Backlog

| Backlog ID | Flow | Finding | Suggested Follow-Up |
| --- | --- | --- | --- |
| AB-BACKLOG-001 | Customer runtime auth | Cross-device web-login scan/confirm is a high-risk auth flow that deserves dedicated source-level flow material before any change. | Promote to dedicated authorization audit before implementation. |
| AB-BACKLOG-002 | Merchant App bind | One-time bind code and long-lived App token issuance are credential boundaries. | Require source-level review before changing bind-code TTL, payload, or consumption order. |
| AB-BACKLOG-003 | Operator finance | Operator finance visibility and provider money commands must stay separated. | Source-audit legacy withdrawal boundary before any operator finance mutation work. |

## Validation Notes

This ledger was validated as a documentation artifact only. No authz tests,
security scans, provider callbacks, or route integration tests were run.
