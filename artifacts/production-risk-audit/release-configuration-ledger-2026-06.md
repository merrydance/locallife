# Production Risk Audit: Release Configuration Ledger

Month: 2026-06
Risk theme: release configuration
Status: active

## Scope

This ledger records release, deployment, configuration, scheduler, worker,
provider credential, migration, generated-artifact, feature-flag, cache, and
runtime-readiness concerns for the 40 reusable codegraph flows. It is
documentation-only and does not change production code or configuration.

## Release Rules

- A flow that depends on callback routes, workers, schedulers, or outboxes is
  release-ready only when all those processes are deployed and configured
  consistently.
- Provider capabilities require correct credentials, certificates, callback
  URLs, public keys, terminal/member ids, and environment mode.
- SQL/migration-dependent invariants are only release-ready after actual
  environment schema is verified, not merely because migration source exists.
- Generated artifacts such as sqlc, Swagger, mocks, app wrappers, and API
  clients must be regenerated only when their source changes; this pass changes
  none of them.

## Flow Ledger

| Flow ID | Flow | Release/Config Surface | Risk | Decision |
| --- | --- | --- | --- | --- |
| RC-BAOFU-AGGREGATE-PAYMENT | Baofoo aggregate payment | Baofoo credentials/certs/public key/callback URL; payment fact/outbox schedulers; share worker/recovery. | Misconfigured callback or disabled scheduler can leave paid facts unapplied or share commands unrecovered. | Verify provider env and scheduler/worker boot before release. |
| RC-BAOFU-REFUND | Baofoo refund | Baofoo refund callback URL/public key; refund recovery scheduler; payment fact/outbox workers. | Treating sync create as terminal in release docs would be unsafe. | Release runbook must include callback/query recovery readiness. |
| RC-CUSTOMER-DINE-IN-CHECKOUT | Dine-in checkout | QR/table routing, payment config, session/order/payment workers, `dine-in-checkout-recovery` scheduler registration, migration `000269`, recovery Prometheus counters, and alert evidence schema. | Missing payment recovery or disabled dine-in checkout recovery scheduler can strand paid sessions; missing migration `000269` can make recovery scans inefficient; missing filled alert evidence can hide repeated recovery failures. | Run `scripts/release_readiness_smoke.sh --target` with disposable fixture IDs and confirm `scheduler:dine-in-checkout-recovery`, session/payment convergence smoke, schema version/index check, and a filled `check:dine-in-recovery-alert-evidence` target-environment proof before release. |
| RC-CUSTOMER-DISCOVERY-BROWSE | Discovery/browse | Search/public route deployment, voucher/wanted merchant tables, region/location config. | Search/orderability filters may drift if backend/frontend wrappers mismatch. | Include wrapper contract scan before search API changes. |
| RC-CUSTOMER-ORDER-AFTER-SALES | After-sales | Payment/refund callbacks, claim/food-safety workers, notification config. | Cross-role workflows can appear stuck if worker/scheduler disabled. | Release notes should name async dependencies. |
| RC-CUSTOMER-PROFILE-WALLET | Profile/wallet | Media storage, upload config, notification config, payment/membership workers. | Media/provider config drift can break avatar/review/recharge flows. | Validate media and payment config when changing profile/wallet releases. |
| RC-CUSTOMER-RESERVATION | Reservation | Reservation/table APIs, payment/refund callback/recovery, scheduler behavior. | No-show/refund/add-on release requires payment and reservation workers; source-level card now defines the reservation pre-change validation surface. | Include reservation payment/refund smoke before release and use `flows/state-sequencing-customer-reservation-checkout-addon-noshow-2026-06-15.md`; real provider evidence and customer reservation Mini Program contract proof remain required for full closure. |
| RC-CUSTOMER-RUNTIME-AUTH | Runtime auth | Auth secrets/token TTL, WeChat credentials, web-login session config. | Token/session config drift is high-impact. | Treat auth config as release gate. |
| RC-CUSTOMER-TAKEOUT-CHECKOUT | Takeout checkout | Cart/order/payment/refund routes, optional `Idempotency-Key` on order create, Mini Program stable order-create key propagation, payment callbacks, timeout workers. | Frontend wrapper drift, disabled timeout/recovery, old clients without the stable key, or provider callback/recovery gaps can strand checkout state; same-order payment-create, backend order-create key replay/concurrency, and current Mini Program key propagation are now covered by focused tests/contracts. | Include order create/payment pending/callback recovery checklist; rerun the order-create idempotency, same-order payment-create, and takeout checkout Mini Program contract checks before checkout/payment transaction changes. |
| RC-MERCHANT-APP-BIND | App bind/device | App bind code TTL, token/session config, native push credentials, device cleanup scheduler. | Push config drift can break order alerts; bind code config drift can issue stale credentials. | Include push/bind smoke and stale-device cleanup readiness. |
| RC-MERCHANT-ONBOARDING | Onboarding | OCR/media config, review workers, credential repair migrations. | OCR disabled/stale cleanup missing can block approval/recovery. | Include OCR queue and credential repair checks. |
| RC-MERCHANT-BIZ-HOURS-AUTO | Business-hours auto | Scheduler registration, timezone/business-hours config, websocket publish. | Scheduler disabled can leave merchants stuck open/closed. | Release readiness must check scheduler boot. |
| RC-MERCHANT-CLAIM-RECOVERY | Claim recovery | Payment facts, async recovery/dispute workers, notification config. | Disabled workers can delay release/sanction convergence. | Include claim recovery worker readiness. |
| RC-MERCHANT-COMBO-CATALOG | Combo/catalog | API wrappers, generated backend/API docs if routes change. | Contract drift can break customer readers. | No regeneration required in this docs pass. |
| RC-MERCHANT-DEVICE-DISPLAY | Device/display | Cloud printer credentials, native push/BLE app config, display config defaults, reconciliation scheduler. | Print/auto-accept behavior can change by config even without code changes. | Release notes must distinguish cloud vs BLE print readiness. |
| RC-MERCHANT-DISH-INVENTORY | Dish/inventory | API wrappers, inventory schema/indexes, public menu readers. | Migration/schema drift affects orderability. | Verify schema before inventory releases. |
| RC-MERCHANT-FINANCE-WITHDRAWAL | Finance withdrawal | Baofu merchant account credentials/certs, withdrawal callbacks, recovery scheduler, sensitive profile config. | Provider config drift can move money incorrectly or block withdrawal. | Treat as G3 release gate with Baofu runbook. |
| RC-MERCHANT-MANUAL-OPEN | Manual open | Scheduler/websocket config and App/Mini Program wrappers. | Manual/auto precedence can appear wrong if schedulers differ by environment. | Include open-status scheduler smoke. |
| RC-MERCHANT-MARKETING-RULES | Marketing rules | Rule-engine config, voucher expiry scheduler, delivery-fee config, membership recharge config. | Stale cache or disabled expiry can affect checkout totals. | Include rules/cache/voucher expiry checks. |
| RC-MERCHANT-MEMBER-BALANCE | Member balance | Idempotency/ledger schema and API wrapper contract. | Missing migration or stale wrapper can bypass repaired idempotency path. | Verify ledger/idempotency schema before release changes. |
| RC-MERCHANT-MEMBERSHIP-SETTINGS | Membership settings | Settings schema/API wrapper and checkout reader deployment. | Frontend/backend version skew can misstate balance-payment eligibility. | Release checkout readers with setting writer changes. |
| RC-MERCHANT-ORDER-OPS | Order operations | Websocket message type, refund worker/recovery, print worker/provider, Flutter/Mini Program wrapper versions. | Version skew can break realtime or refund submission truth. | Include `order_update`, refund, and print readiness checks. |
| RC-MERCHANT-PROFILE-UPDATE | Profile update | Media config, schema hardening, public reader cache. | Shop-image latest-row scoping depends on schema and queries. | Verify media and schema migrations before release. |
| RC-MERCHANT-RESERVATION-TABLE | Reservation/table | Reservation/table APIs, payment/refund recovery, alert schedulers. | Shared customer/merchant state can drift under partial release. | Release customer/merchant wrappers together when contracts change. |
| RC-MERCHANT-REVIEW-REPLY | Review reply | Content-safety provider config and route deployment. | Missing moderation config should fail closed. | Include moderation provider readiness. |
| RC-MERCHANT-STAFF-GROUP | Staff/group | Invite code TTL/config, OCR/media config, credential migrations. | Invite and OCR config drift can break affiliation onboarding. | Include invite/OCR smoke. |
| RC-MERCHANT-STATS-ANALYTICS | Stats | Aggregate SQL indexes and API deployment. | Read-heavy release can expose slow/stale aggregates. | Include query performance check for large datasets before changes. |
| RC-OPERATOR-DASHBOARD | Dashboard | Operator routes, finance source ledgers, notification config. | Finance summary depends on money ledgers and regional authority config. | Include region/notification/finance smoke. |
| RC-OPERATOR-DISPATCH-HALL | Dispatch hall | Scheduler registration, Asynq worker, notification channels. | Disabled scheduler/worker prevents 3-minute alert delivery. | Release gate: scheduler and worker both live. |
| RC-OPERATOR-FINANCE-WITHDRAWAL | Finance withdrawal | Baofu provider config, callback routes, recovery workers, legacy boundary flags. | Operator finance release can mislead if provider jobs disabled. | Use Baofu and finance runbooks. |
| RC-OPERATOR-MERCHANT-MGMT | Merchant management | Region authority config, capability/system-label schema. | Partial migration can break capability reconciliation. | Verify schema and region authority. |
| RC-OPERATOR-REGION-RULES | Region rules | Delivery-fee/peak-hour/rule engine/weather cache config, platform approval workflow. | Cache invalidation/weather dependency can stale pricing. | Include cache/weather/rule release checks. |
| RC-OPERATOR-RIDER-MGMT | Rider management | Region authority and masked-field config/DTOs. | Masking/region filters can regress under DTO changes. | Include privacy/region checks before release. |
| RC-OPERATOR-SAFETY-RECOVERY | Safety/recovery | Food-safety workflow, recovery dispute workers, notification config. | Disabled result workers can leave sanctions/releases pending. | Include recovery worker readiness. |
| RC-RIDER-ONBOARDING | Rider onboarding | OCR/media config, credential/role migrations. | OCR config or migration drift blocks role activation. | Treat as identity G3 release gate. |
| RC-RIDER-CLAIMS-RECOVERY | Claims/recovery | WeChat payment config, recovery dispute workers, notification config. | Payment/dispute worker skew can duplicate or miss side effects. | Include worker and payment callback readiness. |
| RC-RIDER-DELIVERY-LIFECYCLE | Delivery lifecycle | Scheduler registration, Redis/WebSocket, map provider config, Baofu rider bill setup. | Disabled scheduler/realtime/map config affects visibility but SQL remains truth. | Release checklist should separate hard state from best-effort realtime/map. |
| RC-RIDER-DEPOSIT | Rider deposit | WeChat payment/refund config, recovery schedulers, credit expiry scheduler. | Disabled recovery/expiry can strand deposit or refundable-credit state. | Include payment/refund/credit scheduler readiness. |
| RC-RIDER-INCOME-WITHDRAWAL | Rider income withdrawal | Baofu account/withdrawal credentials, callbacks, recovery worker, verify-fee payment config. | Provider config drift can block readiness, sharing, or withdrawal. | Use Baofu contract/runbook before release. |
| RC-RIDER-WORKBENCH-LOCATION | Workbench/location | GPS permissions/config, notification config, geofence scheduler/logic, map provider if used. | Client permission/config drift can make location-based flows stale. | Include fallback/manual transition guidance in release notes. |

## Cross-Flow Backlog

| Backlog ID | Flow | Finding | Suggested Follow-Up |
| --- | --- | --- | --- |
| RC-BACKLOG-001 | All callback/provider flows | Callback URL, public key, cert serial, terminal/member id, and environment mode must match deployed code. | Use `flows/external-dependency-baofu-provider-evidence-gate-2026-06-15.md`, `locallife/scripts/baofu_provider_evidence_gate.sh`, and the target release readiness smoke before provider-affecting releases. |
| RC-BACKLOG-002 | All scheduler-dependent flows | Many convergence paths depend on scheduler/worker boot, not request code. | Use `flows/release-scheduler-worker-readiness-gate-2026-06-15.md` and `scripts/release_readiness_smoke.sh --target`; remaining work is target-environment execution with disposable fixture IDs and filled alert evidence for recovery failure metrics. |
| RC-BACKLOG-003 | All migration-dependent flows | This docs pass did not verify actual environment schema. | Before code changes, verify `schema_migrations` and relevant constraints/indexes. |

## Validation Notes

This ledger was validated as a documentation artifact only. No deployment,
environment, scheduler boot, migration, config, provider credential, or
generated-artifact validation was run.
