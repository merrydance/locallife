# Production Risk Audit: External Dependency Ledger

Month: 2026-06
Risk theme: external dependencies
Status: active

## Scope

This ledger records provider, SDK, map, OCR, media, content-safety, printer,
push, websocket, Redis, scheduler, and payment dependencies referenced by the
40 reusable codegraph flows. It is documentation-only and does not change
production code.

## Dependency Rules

- Provider synchronous success is not automatically business success.
- Callback/query/fact application must own terminal provider truth.
- Third-party failures should be observable and should not silently mutate
  business terminal state unless the domain contract explicitly allows it.
- UI-visible guidance should distinguish retryable, queryable, terminal, and
  configuration failures.
- External side effects should remain outside transaction-owned critical
  sections unless the code has an explicit durable command boundary.

## Flow Ledger

| Flow ID | Flow | External Dependencies | Existing Boundary | Gap / Residual Risk | Decision |
| --- | --- | --- | --- | --- | --- |
| ED-BAOFU-AGGREGATE-PAYMENT | Baofoo aggregate payment | Baofoo aggregate pay/share callbacks/query; Redis/Asynq outbox. | Signed callback parser, fact application, share command/fact recovery. | Real positive payment/share callback evidence remains a Baofu domain gap. | Follow Baofu domain source matrix before provider changes. |
| ED-BAOFU-REFUND | Baofoo refund | Baofoo refund create/callback/query; Redis/Asynq. | Synchronous create is command acceptance; callback/query facts terminalize. | Real positive refund callback evidence remains a gap. | Keep `CreateRefund` success as processing, not terminal. |
| ED-CUSTOMER-DINE-IN-CHECKOUT | Dine-in checkout | Payment provider via shared payment domain; QR/table scan; websocket/payment result. | Backend session/payment convergence. | QR code trust and payment terminal truth need source-level proof before changes. | Do not trust client payment result or raw QR params. |
| ED-CUSTOMER-DISCOVERY-BROWSE | Discovery/browse | Location/region hints, search SQL, voucher/wanted flows. | Backend public/search readers. | Map/location precision affects ranking only. | Keep availability/orderability backend-owned. |
| ED-CUSTOMER-ORDER-AFTER-SALES | After-sales | Payment/refund providers, map route planning, notifications, claim/food-safety workers. | Provider facts and backend route planning are separate from customer page state. | Claim payout and recovery dependencies require downstream domain audit. | Treat tracking map as display-only. |
| ED-CUSTOMER-PROFILE-WALLET | Profile/wallet | Media upload/storage, payment/membership providers, notifications. | Media asset ids and payment ledgers own durable truth. | Media visibility and upload recovery need source audit before changes. | Keep media/provider state separate from profile UI. |
| ED-CUSTOMER-RESERVATION | Reservation | Payment/refund providers, room/table state, dine-in handoff. | Provider facts terminalize money; reservation/table transactions own local state. | Add-on provider paths need focused proof. | Keep provider money truth in payment domain. |
| ED-CUSTOMER-RUNTIME-AUTH | Runtime auth | WeChat login/session APIs, token storage, web login channel, error telemetry. | Server sessions/tokens own identity; telemetry secondary. | External auth drift is high impact. | Use auth standards/source audit before changes. |
| ED-CUSTOMER-TAKEOUT-CHECKOUT | Takeout checkout | WeChat/Baofoo payment, payment result, refunds, delivery-fee calculation. | Backend calculation and provider callback/recovery. | End-to-end provider callback evidence belongs to payment domain. | Keep checkout pricing and payment terminal truth server-side. |
| ED-MERCHANT-APP-BIND | Merchant App bind/device | Native push platform, App token/session, device heartbeat. | Device registration cleanup and terminal-failure degradation boundaries. | Push-provider terminal failures require operational evidence. | Keep alert push as side effect, not credential truth. |
| ED-MERCHANT-ONBOARDING | Onboarding | OCR, media, merchant credential ledger. | OCR async writeback, stale cleanup, review/credential repair. | OCR/provider drift can affect identity approval. | Fail closed on unverified identity/OCR state. |
| ED-MERCHANT-BIZ-HOURS-AUTO | Business-hours auto | Scheduler and websocket publish. | Durable business hours drive scheduler; websocket post-write. | Scheduler deployment/config affects convergence. | Track under release config too. |
| ED-MERCHANT-CLAIM-RECOVERY | Claim recovery | WeChat direct payment facts, async workers, notifications. | Fact-driven payment release convergence. | Recovery payment provider evidence should be refreshed before changes. | Keep payment release provider-fact driven. |
| ED-MERCHANT-COMBO-CATALOG | Combo/catalog | Public menu/search/order readers. | Durable backend catalog state. | No external provider beyond shared readers. | No provider-specific action. |
| ED-MERCHANT-DEVICE-DISPLAY | Device/display | Cloud printer provider, BLE local printer, native push, Redis/Asynq print workers. | Backend config controls cloud print/auto-accept; BLE local receipt separate. | Provider callback/status and local BLE failures need separate observability. | Do not confuse BLE print success with backend print truth. |
| ED-MERCHANT-DISH-INVENTORY | Dish/inventory | Public menu/order/reservation readers. | Backend durable product state. | No third-party provider. | No provider-specific action. |
| ED-MERCHANT-FINANCE-WITHDRAWAL | Merchant finance withdrawal | Baofu account opening/settlement/withdrawal, callbacks, recovery. | Command/fact durability, provider error diagnostics, local/provider balance proof. | Real withdrawal funds action evidence remains high risk. | Keep Baofu domain contract matrix authoritative. |
| ED-MERCHANT-MANUAL-OPEN | Manual open | Scheduler/websocket. | Durable merchant status plus refresh. | Websocket failure leaves clients stale until readback. | Keep read refresh as recovery. |
| ED-MERCHANT-MARKETING-RULES | Marketing rules | Voucher expiry scheduler, delivery-fee/rule engine, payment membership recharge. | Durable config/transaction rows. | Rule-engine or cache failures can affect totals. | Revalidate at order creation. |
| ED-MERCHANT-MEMBER-BALANCE | Member balance | Payment/order/refund adjacent readers. | Ledger transactions. | No direct provider dependency for manual adjustment. | Keep away from payment provider semantics. |
| ED-MERCHANT-MEMBERSHIP-SETTINGS | Membership settings | Checkout/rules engine readers. | Durable setting row. | No third-party provider. | No provider-specific action. |
| ED-MERCHANT-ORDER-OPS | Order operations | Baofu refund, print provider, websocket, push, notifications, BLE printer. | Refund command/fact/recovery, print logs/provider callback, backend readback. | Historical failed refund candidates are visible only for manual review; BLE not logged. | Preserve distinct provider boundaries. |
| ED-MERCHANT-PROFILE-UPDATE | Profile update | Media storage/async sync. | Media pending-sync proof and owner-scoped live truth. | Media provider/storage failure can stale public image. | Keep pending-sync/recovery visible. |
| ED-MERCHANT-RESERVATION-TABLE | Reservation/table | Payment/refund provider, alerts, possibly dine-in/session integration. | Provider facts plus reservation/table SQL. | No-show/refund provider paths need source audit. | Keep provider terminal truth separate. |
| ED-MERCHANT-REVIEW-REPLY | Review reply | Content-safety provider. | Reply written only after external content-safety acceptance. | Moderation timeout/failure copy and retry should be checked before changes. | Fail closed on ambiguous moderation. |
| ED-MERCHANT-STAFF-GROUP | Staff/group | OCR/media for identity documents. | OCR worker and private document boundary. | OCR drift can activate wrong affiliation. | Keep review/credential writes guarded. |
| ED-MERCHANT-STATS-ANALYTICS | Stats | Aggregate SQL only. | Read-only. | No external provider. | No provider-specific action. |
| ED-OPERATOR-DASHBOARD | Dashboard | Notifications, finance/profit-sharing records. | Operator reads over backend ledgers. | Finance figures depend on provider money flows. | Keep provider evidence in finance/withdrawal ledgers. |
| ED-OPERATOR-DISPATCH-HALL | Dispatch hall | Scheduler, Asynq worker, notifications. | Dedupe ledger and worker retry. | Notification delivery/readback not end-to-end validated. | Preserve scheduler/worker observability. |
| ED-OPERATOR-FINANCE-WITHDRAWAL | Finance withdrawal | Baofu settlement/withdrawal provider callbacks/workers. | Command/fact and recovery paths. | Legacy withdrawal boundary and positive provider evidence. | Use Baofu domain standards for any change. |
| ED-OPERATOR-MERCHANT-MGMT | Merchant management | System-label reconciliation, food-safety/recovery side paths. | Durable capability transaction. | Recovery/food-safety external side effects are adjacent. | Keep side effects explicit. |
| ED-OPERATOR-REGION-RULES | Region rules | Weather coefficient cache/provider, rule engine, platform approval. | Cache invalidation after weather rule writes; rule-engine region scoping. | Cache invalidation/provider weather failures affect pricing. | Track release/config and validation matrix before changes. |
| ED-OPERATOR-RIDER-MGMT | Rider management | Rule-driven suspension/deposit boundaries. | Read-only in this slice. | No direct provider. | No provider-specific action. |
| ED-OPERATOR-SAFETY-RECOVERY | Safety/recovery | Food-safety workflow, recovery dispute workers, notifications. | Transaction resolves food-safety-owned state; workers process recovery effects. | Worker retry/provider notification evidence not run. | Preserve worker effect idempotency. |
| ED-RIDER-ONBOARDING | Rider onboarding | OCR/media, credential ledger. | OCR async writeback and review/credential activation. | Health certificate validity/provider/OCR drift. | Fail closed on identity uncertainty. |
| ED-RIDER-CLAIMS-RECOVERY | Rider claims/recovery | WeChat recovery payment, worker result effects. | Payment facts and automatic resolution worker. | Result-effect notification/payment proof should be refreshed before changes. | Keep payment/provider facts terminal. |
| ED-RIDER-DELIVERY-LIFECYCLE | Delivery lifecycle | Map route provider, Redis/WebSocket, scheduler/Asynq, Baofu rider profit-sharing bill. | SQL pool/delivery state canonical; broadcasts/map are best-effort. | Map provider failure should not affect delivery transition. | Keep map and realtime outside critical transaction. |
| ED-RIDER-DEPOSIT | Rider deposit | WeChat direct payment/refund, payment/refund recovery schedulers, credit expiry scheduler. | Payment/refund facts terminalize; withdrawal request idempotency prevents duplicate local refund-order creation; credit scheduler handles expiry. | External risk is callback/query/recovery evidence and operations for abnormal direct refunds, not request duplicate creation. | Keep provider facts, request idempotency, and credit ledgers separate. |
| ED-RIDER-INCOME-WITHDRAWAL | Rider income withdrawal | Baofu settlement account, verify-fee payment, share/withdraw callbacks/query. | Account-opening callback/recovery, share command/fact, shared Baofu withdrawal idempotency, submitted-command dispatch, withdrawal callback/query recovery. | Real withdrawal positive evidence and provider `unknown`/manual-recovery handling remain gaps. | Keep Baofu account/withdrawal contract matrix authoritative. |
| ED-RIDER-WORKBENCH-LOCATION | Workbench/location | GPS, map/geofence logic, notifications. | Location SQL plus geofence event dedupe; frontend queue best effort. | Location duplicates can affect analytics/geofence noise. | Keep manual delivery buttons as recovery path. |

## Cross-Flow Backlog

| Backlog ID | Flow | Finding | Suggested Follow-Up |
| --- | --- | --- | --- |
| ED-BACKLOG-001 | Baofoo payment/refund/share/withdrawal | Baofu source matrix records remaining positive callback evidence gaps for real provider flows; a read-only evidence gate wrapper now exists but does not create provider evidence by itself. | Use `flows/external-dependency-baofu-provider-evidence-gate-2026-06-15.md` and `locallife/scripts/baofu_provider_evidence_gate.sh`; do not change provider semantics without updating Baofu domain evidence. |
| ED-BACKLOG-002 | Cloud/BLE printing | Merchant device/display and order operations distinguish backend cloud print logs from local BLE receipt side effects. | Keep UI copy and observability separate. |
| ED-BACKLOG-003 | Map/location | Route planning and location are display/recovery aids, not state transition truth. | Avoid using client/provider route data as delivery authorization. |

## Validation Notes

This ledger was validated as a documentation artifact only. No provider sandbox,
live callback, map, OCR, printer, push, Redis, or worker integration tests were
run.
