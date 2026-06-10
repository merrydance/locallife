# Merchant Device Display Config Slice

Status: partially fixed; Mini Program voice-control removal/backend no-op compatibility fixed 2026-06-10
Risk class: G2 - merchant configuration controls automatic order acceptance, print task dispatch, cloud-printer provider state, and async recovery surfaces
Scope: Mini Program display/printer pages -> device/display-config APIs -> durable config/device truth -> payment outbox auto-accept, print workers, provider calls, reconciliation jobs, and print-log consumers

## Variant Coverage

This slice covers:

- Merchant Mini Program display config page for print dispatch, trigger mode, and auto-accept. Deprecated voice fields remain backend compatibility state but are no longer shown or submitted by the Mini Program.
- Merchant Mini Program printer list and printer edit pages for create, update, delete, test print, and live status.
- Flutter merchant App local Bluetooth printer scan/connect/disconnect and order-receipt printing.
- Flutter merchant App local notification settings for sound, voice, auto-accept, and BLE auto-print after accept.
- Backend device access, printer CRUD/status/test, display-config read/write, deprecated voice compatibility/no-op handling, and printer reconciliation endpoints.
- Downstream consumers: payment-domain outbox auto-accept, order print scheduling, print worker dispatch, print-log callbacks/anomalies, and printer reconciliation jobs.

This slice does not fully cover:

- Merchant app foreground notification/audio implementation, except reverse search for display voice-setting consumers.
- Customer order payment fact creation before payment-domain outbox.
- Full order operation status/refund/print chain already captured in `merchant-order-operations`.
- Platform/operator finance reconciliation, which is unrelated to cloud-printer reconciliation despite sharing the word reconciliation.

## Product Invariant

Device and display settings must be truthful configuration, not decorative toggles:

- `order_display_configs` is the durable source for whether paid orders can be auto-accepted and when/what order types are printed.
- `cloud_printers` is the durable local source for which merchant printers are active and which order types/roles they support.
- Provider registration/removal/status is external truth; local DB and provider state must either change together or expose a recovery path.
- Auto-accept may only happen when backend truth allows it and at least one eligible active printer exists.
- Product decision 2026-06-10: Flutter BLE receipt printing must honor backend display config and must gain observability/deduplication boundaries instead of remaining an unlimited invisible local side effect.
- Product decision 2026-06-10: Flutter App local auto-accept and backend `auto_accept_paid_orders` must merge into one backend truth; the App should only execute/display backend-authorized behavior.
- Test print command acceptance is not proof of terminal print success; printed truth still lives in provider status/callback or `print_logs` for order tasks.

## Primary Forward Chain

1. Merchant dashboard/config entry routes device settings to display-config and printer pages.
   Evidence: `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts:176`, `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts:177`, `weapp/miniprogram/pages/merchant/config/index.ts:81`, `weapp/miniprogram/pages/merchant/config/index.ts:82`.

2. Display-config page checks device-management access, then loads backend config.
   Evidence: `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:104`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:115`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:135`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:164`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:189`.

3. Display-config page maps backend truth into a local form for `enable_print`, order-type print flags, dispatch mode, trigger mode, and auto-accept only. Fixed 2026-06-10: the Mini Program no longer maps, displays, or submits deprecated voice fields. The page blocks pull refresh while dirty.
   Evidence: `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:12`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:46`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:56`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:150`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:297`.

4. Display-config page updates only local draft on switches/mode selections and saves through `displayConfigService.updateDisplayConfig`, then rehydrates from the backend response.
   Evidence: `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:231`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:244`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:267`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:290`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:303`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:315`.

5. The display-config UI disables the auto-accept switch when `enable_print` is false, but the save payload still sends the stored form value for both fields.
   Evidence: `weapp/miniprogram/pages/merchant/settings/display-config/index.wxml:55`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:303`, `weapp/miniprogram/pages/merchant/settings/display-config/index.ts:310`.

6. Device-management access uses `GET /v1/merchant/devices/access`; frontend caches capability for 60 seconds and only grants the page when backend says `can_manage`.
   Evidence: `weapp/miniprogram/utils/console-access.ts:20`, `weapp/miniprogram/utils/console-access.ts:191`, `weapp/miniprogram/utils/console-access.ts:203`, `weapp/miniprogram/utils/console-access.ts:289`, `weapp/miniprogram/utils/console-access.ts:303`, `weapp/miniprogram/utils/console-access.ts:304`.

7. Device/display wrappers map the frontend calls to backend routes. Reconciliation wrappers exist in the service but are not called by the current printer pages.
   Evidence: `weapp/miniprogram/api/table-device-management.ts:481`, `weapp/miniprogram/api/table-device-management.ts:498`, `weapp/miniprogram/api/table-device-management.ts:523`, `weapp/miniprogram/api/table-device-management.ts:544`, `weapp/miniprogram/api/table-device-management.ts:557`, `weapp/miniprogram/api/table-device-management.ts:569`, `weapp/miniprogram/api/table-device-management.ts:581`, `weapp/miniprogram/api/table-device-management.ts:593`, `weapp/miniprogram/api/table-device-management.ts:611`, `weapp/miniprogram/api/table-device-management.ts:622`.

8. Backend route registration exposes device access, printer CRUD/status/test/reconciliation, and display-config GET/PUT. Device and display groups require owner/manager.
   Evidence: `locallife/api/server.go:1289`, `locallife/api/server.go:1292`, `locallife/api/server.go:1293`, `locallife/api/server.go:1295`, `locallife/api/server.go:1297`, `locallife/api/server.go:1298`, `locallife/api/server.go:1300`, `locallife/api/server.go:1301`, `locallife/api/server.go:1302`, `locallife/api/server.go:1303`, `locallife/api/server.go:1307`, `locallife/api/server.go:1308`, `locallife/api/server.go:1310`, `locallife/api/server.go:1311`.

9. Device access backend resolves merchant staff identity, checks merchant status/region, and grants only owner/manager.
   Evidence: `locallife/api/device_access.go:35`, `locallife/api/device_access.go:37`, `locallife/api/device_access.go:58`, `locallife/api/device_access.go:63`, `locallife/api/device_access.go:64`, `locallife/api/device_access.go:66`.

10. Display-config GET reads `order_display_configs` by merchant. Missing rows return default config without persisting it.
    Evidence: `locallife/api/device.go:721`, `locallife/api/device.go:726`, `locallife/api/device.go:737`, `locallife/api/device.go:739`, `locallife/api/device.go:741`, `locallife/api/device.go:749`, `locallife/api/device.go:790`.

11. Display-config PUT resolves the merchant, checks whether a row exists, then creates or updates a partial config. Fixed 2026-06-10: legacy `enable_voice`, `voice_takeout`, and `voice_dine_in` request fields are accepted for old-client compatibility but ignored; new rows keep default compatibility values and existing rows keep their prior voice values. The response is built from the persisted row.
    Evidence: `locallife/api/device.go:875`, `locallife/api/device.go:883`, `locallife/api/device.go:925`, `locallife/api/device.go:945`, `locallife/api/device.go:970`, `locallife/api/device.go:987`, `locallife/api/device.go:1009`, `locallife/api/device.go:1036`.

12. SQL stores display config in one unique row per merchant and supports partial update by `COALESCE`.
    Evidence: `locallife/db/query/order_display_config.sql:1`, `locallife/db/query/order_display_config.sql:24`, `locallife/db/query/order_display_config.sql:28`, `locallife/db/query/order_display_config.sql:31`, `locallife/db/query/order_display_config.sql:37`, `locallife/db/query/order_display_config.sql:47`, `locallife/db/migration/000010_add_orders.up.sql:180`, `locallife/db/migration/000239_add_auto_accept_paid_orders.up.sql:1`.

13. Printer list page checks device-management access, loads printers, preserves old list when refresh fails, and navigates to create/edit pages with a reload-on-show flag.
    Evidence: `weapp/miniprogram/pages/merchant/printers/index.ts:232`, `weapp/miniprogram/pages/merchant/printers/index.ts:247`, `weapp/miniprogram/pages/merchant/printers/index.ts:276`, `weapp/miniprogram/pages/merchant/printers/index.ts:307`, `weapp/miniprogram/pages/merchant/printers/index.ts:331`, `weapp/miniprogram/pages/merchant/printers/index.ts:348`, `weapp/miniprogram/pages/merchant/printers/index.ts:354`.

14. Printer list deletes/test-prints through confirmation dialog actions. Delete reloads backend list afterward; test print only shows command-accepted text.
    Evidence: `weapp/miniprogram/pages/merchant/printers/index.ts:400`, `weapp/miniprogram/pages/merchant/printers/index.ts:406`, `weapp/miniprogram/pages/merchant/printers/index.ts:412`, `weapp/miniprogram/pages/merchant/printers/index.ts:424`, `weapp/miniprogram/pages/merchant/printers/index.ts:427`, `weapp/miniprogram/pages/merchant/printers/index.ts:437`, `weapp/miniprogram/pages/merchant/printers/index.ts:440`.

15. Printer list live-status popup only queries Feieyun printers and guards stale async responses with a request token.
    Evidence: `weapp/miniprogram/pages/merchant/printers/index.ts:485`, `weapp/miniprogram/pages/merchant/printers/index.ts:488`, `weapp/miniprogram/pages/merchant/printers/index.ts:499`, `weapp/miniprogram/pages/merchant/printers/index.ts:508`, `weapp/miniprogram/pages/merchant/printers/index.ts:510`, `weapp/miniprogram/pages/merchant/printers/index.ts:517`.

16. Printer edit page checks access, loads detail when editing, validates create fields, then calls create or update. It navigates back instead of rehydrating in-place.
    Evidence: `weapp/miniprogram/pages/merchant/printers/edit/index.ts:110`, `weapp/miniprogram/pages/merchant/printers/edit/index.ts:133`, `weapp/miniprogram/pages/merchant/printers/edit/index.ts:152`, `weapp/miniprogram/pages/merchant/printers/edit/index.ts:173`, `weapp/miniprogram/pages/merchant/printers/edit/index.ts:223`, `weapp/miniprogram/pages/merchant/printers/edit/index.ts:231`, `weapp/miniprogram/pages/merchant/printers/edit/index.ts:235`, `weapp/miniprogram/pages/merchant/printers/edit/index.ts:256`, `weapp/miniprogram/pages/merchant/printers/edit/index.ts:268`, `weapp/miniprogram/pages/merchant/printers/edit/index.ts:275`.

17. Backend printer create enforces active printer SN uniqueness, calls provider add first for Feieyun, then writes `cloud_printers`. Soft-deleted historical printer rows may keep the same SN; if local create fails after provider success, the backend records a pending remove reconciliation job.
    Evidence: `locallife/api/device.go:89`, `locallife/api/device.go:100`, `locallife/api/device.go:111`, `locallife/api/device.go:139`, `locallife/api/device.go:141`, `locallife/api/device.go:152`, `locallife/api/device.go:164`, `locallife/api/device.go:165`.

18. Backend printer read/update/status/test/delete all resolve the merchant and verify the printer belongs to that merchant before operating.
    Evidence: `locallife/api/device.go:266`, `locallife/api/device.go:277`, `locallife/api/device.go:288`, `locallife/api/device.go:299`, `locallife/api/device.go:324`, `locallife/api/device.go:342`, `locallife/api/device.go:351`, `locallife/api/device.go:424`, `locallife/api/device.go:452`, `locallife/api/device.go:462`, `locallife/api/device.go:522`, `locallife/api/device.go:544`, `locallife/api/device.go:554`, `locallife/api/device.go:613`, `locallife/api/device.go:635`, `locallife/api/device.go:645`.

19. Backend printer update is local-only; it can change name, key, role, order-type print flags, and active state. It does not rename/update provider-side printer metadata.
    Evidence: `locallife/api/device.go:467`, `locallife/api/device.go:472`, `locallife/api/device.go:475`, `locallife/api/device.go:478`, `locallife/api/device.go:481`, `locallife/api/device.go:490`, `locallife/api/device.go:495`, `locallife/db/query/cloud_printer.sql:34`, `locallife/db/query/cloud_printer.sql:37`, `locallife/db/query/cloud_printer.sql:43`.

20. Fixed 2026-06-09: backend printer delete still calls provider remove first, then soft-deletes/deactivates local `cloud_printers` instead of physically deleting the row. If the local soft delete fails after provider success, it still records a pending register reconciliation job.
    Evidence: `locallife/api/device.go:559`, `locallife/api/device.go:561`, `locallife/api/device.go:568`, `locallife/api/device.go:570`, `locallife/api/device.go:571`, `locallife/db/query/cloud_printer.sql:48`, `locallife/db/migration/000255_soft_delete_cloud_printers.up.sql`.

21. Fixed 2026-06-09: `print_logs.printer_id` still references `cloud_printers(id)` for historical observability, but delete no longer physically removes the printer row. Current printer reads/lists/updates exclude `deleted_at IS NOT NULL`, active printer SN uniqueness ignores soft-deleted rows, order print-job status can read soft-deleted historical printers without issuing a cloud query, retry on a soft-deleted printer is rejected/recorded as `printer is deleted`, and migration down preserves both duplicate-SN historical rows by tombstoning soft-deleted SNs before restoring the old global unique index.
    Evidence: `locallife/db/migration/000010_add_orders.up.sql:156`, `locallife/db/migration/000010_add_orders.up.sql:160`, `locallife/db/migration/000255_soft_delete_cloud_printers.up.sql`, `locallife/db/migration/000255_soft_delete_cloud_printers.down.sql`, `locallife/db/query/cloud_printer.sql`, `locallife/api/order.go`, `locallife/worker/task_print_order.go`, `locallife/db/sqlc/cloud_printer_test.go`, `locallife/db/sqlc/cloud_printer_migration_test.go`, `locallife/api/order_test.go`, `locallife/worker/task_print_order_test.go`.

22. Printer test and live status call provider APIs directly; test print does not create `print_logs`.
    Evidence: `locallife/api/device.go:650`, `locallife/api/device.go:661`, `locallife/api/device.go:662`, `locallife/api/device.go:672`, `locallife/api/device.go:355`, `locallife/api/device.go:360`, `locallife/api/device.go:365`, `locallife/api/device.go:371`.

23. Reconciliation jobs are stored with pending uniqueness by merchant, SN, and desired action. Listing and retry are backend-supported and tenant-checked.
    Evidence: `locallife/api/device_reconciliation.go:65`, `locallife/api/device_reconciliation.go:122`, `locallife/api/device_reconciliation.go:145`, `locallife/api/device_reconciliation.go:178`, `locallife/api/device_reconciliation.go:196`, `locallife/api/device_reconciliation.go:205`, `locallife/db/query/cloud_printer_reconciliation_job.sql:1`, `locallife/db/query/cloud_printer_reconciliation_job.sql:16`, `locallife/db/query/cloud_printer_reconciliation_job.sql:34`.

24. Reconciliation retry executes the desired provider action, increments failure retry count on provider error, and marks resolved on provider success. It does not re-check local DB convergence after provider success.
    Evidence: `locallife/api/device_reconciliation.go:218`, `locallife/api/device_reconciliation.go:220`, `locallife/api/device_reconciliation.go:226`, `locallife/api/device_reconciliation.go:235`, `locallife/api/device_reconciliation.go:236`, `locallife/api/device_reconciliation.go:255`, `locallife/db/query/cloud_printer_reconciliation_job.sql:40`, `locallife/db/query/cloud_printer_reconciliation_job.sql:50`.

25. Payment-domain outbox consumes `auto_accept_paid_orders`. It only auto-accepts paid orders when display config enables auto-accept, printing, order type, accepted trigger, and at least one eligible active Feieyun printer.
    Evidence: `locallife/worker/task_payment_domain_outbox.go:119`, `locallife/worker/task_payment_domain_outbox.go:168`, `locallife/worker/task_payment_domain_outbox.go:183`, `locallife/worker/task_payment_domain_outbox.go:191`, `locallife/worker/task_payment_domain_outbox.go:198`, `locallife/worker/task_payment_domain_outbox.go:202`, `locallife/worker/task_payment_domain_outbox.go:206`, `locallife/worker/task_payment_domain_outbox.go:210`, `locallife/worker/task_payment_domain_outbox.go:223`.

26. Order service and print worker also consume display config for normal/manual printing. Missing display-config rows fall back to default print-enabled accepted-trigger semantics.
    Evidence: `locallife/logic/order_service.go:762`, `locallife/logic/order_service.go:770`, `locallife/logic/order_service.go:773`, `locallife/logic/order_service.go:775`, `locallife/logic/order_service.go:780`, `locallife/worker/task_print_order.go:109`, `locallife/worker/task_print_order.go:112`, `locallife/worker/task_print_order.go:124`.

27. Print worker consumes active printers, filters unsupported printer type and order-type flags, then uses `print_dispatch_mode=split` and `printer_role` to decide full/front/kitchen print jobs.
    Evidence: `locallife/worker/task_print_order.go:128`, `locallife/worker/task_print_order.go:280`, `locallife/worker/task_print_order.go:288`, `locallife/worker/task_print_order.go:292`, `locallife/worker/task_print_order.go:305`, `locallife/worker/task_print_order.go:323`, `locallife/worker/task_print_order.go:325`, `locallife/worker/task_print_order.go:333`.

28. Fixed 2026-06-10: voice broadcast fields are now deprecated compatibility fields in this flow. GET responses still include persisted/default `voice_*` values, and PUT still accepts the old request fields, but the backend ignores them and the Mini Program no longer displays or submits them. Reverse search found no runtime consumer outside API compatibility tests.
    Evidence: `locallife/api/device.go:780`, `locallife/api/device.go:883`, `locallife/api/device_test.go:1594`, `locallife/api/device_test.go:1617`, `weapp/scripts/check-merchant-display-config-auto-accept.js:39`.

29. Flutter merchant App has a separate local Bluetooth printer state path: it scans through `FlutterBluePlus`, stores a saved device id in shared preferences, connects locally, and prints accepted-order receipts over BLE without writing backend printer state.
    Evidence: `merchant_app/lib/features/printer/printer_provider.dart:48`, `merchant_app/lib/features/printer/printer_provider.dart:83`, `merchant_app/lib/features/printer/printer_provider.dart:109`, `merchant_app/lib/features/printer/printer_provider.dart:127`, `merchant_app/lib/features/printer/printer_provider.dart:146`, `merchant_app/lib/features/printer/printer_provider.dart:198`.

30. Flutter App settings and unauthenticated order page copy expose Bluetooth printer setup independently of Mini Program cloud-printer configuration.
    Evidence: `merchant_app/lib/features/order/order_list_page.dart:260`, `merchant_app/lib/features/settings/settings_page.dart:87`, `merchant_app/lib/features/settings/settings_page.dart:92`.

31. Flutter App's auto-accept and auto-print toggles are local SharedPreferences settings. On incoming order alerts, local `autoAcceptEnabled` calls the same backend accept endpoint through `OrderNotifier.acceptOrder`, then local `autoPrintAfterAcceptEnabled` can print through BLE if a device is connected. These toggles do not read or write backend `order_display_configs`.
    Evidence: `merchant_app/lib/features/settings/notification_settings_provider.dart:41`, `merchant_app/lib/features/settings/notification_settings_provider.dart:68`, `merchant_app/lib/features/settings/notification_settings_provider.dart:74`, `merchant_app/lib/features/order/order_alert_coordinator.dart:127`, `merchant_app/lib/features/order/order_alert_coordinator.dart:140`, `merchant_app/lib/features/order/order_alert_coordinator.dart:378`, `merchant_app/lib/features/order/order_alert_coordinator.dart:388`, `merchant_app/lib/features/order/order_alert_coordinator.dart:390`.

## Reverse-Reference Findings

- `auto_accept_paid_orders` is not a dead field. It is consumed by `task_payment_domain_outbox.go` after order payment success.
- Auto-accept is intentionally coupled to print configuration: worker requires `AutoAcceptPaidOrders`, `EnablePrint`, order-type flag, `PrintTriggerMode=accepted`, and eligible active printer.
- Normal accepted/ready/manual print flows consume the same `order_display_configs` and `cloud_printers` state through order service and print worker.
- Fixed 2026-06-10: Mini Program display-config voice controls are removed/hidden because the Mini Program environment cannot satisfy the merchant keep-alive/push-data requirement for this feature. Persisted `enable_voice`, `voice_takeout`, and `voice_dine_in` fields are deprecated/no-op compatibility state until safe cleanup; GET still exposes stored/default values, while PUT ignores legacy request values. No backend, Mini Program merchant page, or Flutter merchant app consumer was found in this trace.
- `DeviceManagementService.listPrinterReconciliationJobs` and `retryPrinterReconciliationJob` wrappers exist, but the current merchant printer pages do not call them.
- `getActivePrinters` and `batchTestPrinters` helper exports exist but no current caller was found in the Mini Program search.
- Printer update changes local `printer_key` and `printer_name`, but no provider update/rename path was found. Provider/local metadata can drift by design after update.
- Fixed 2026-06-09: printer deletion is now a soft-delete/deactivate operation, so historical `print_logs` keep their printer reference and no longer block local deletion after provider removal. The active SN partial unique index allows a replacement active printer with the same SN, Yilianyun authorization rebind clears stale soft-deleted printer links before attaching the replacement, and historical print status/retry paths now report soft-deleted printers as local inactive/deleted rather than disappearing as 404.
- Product decision 2026-06-10: Flutter local Bluetooth printing must obey backend display config and define observability/deduplication boundaries. Current App BLE printer state is not represented in `cloud_printers`, `print_logs`, or display config, and can print App receipts even when backend cloud-printer config is disabled, depending on App action flow.
- Product decision 2026-06-10: Flutter App local auto-accept and backend display-config auto-accept must be merged into one backend truth; the App only executes or displays the backend-authorized state. Current runtime still has two auto-accept controls with different truth owners: backend outbox `auto_accept_paid_orders` and App-local `autoAcceptEnabled`.

## SQL And Durable State Boundaries

- `order_display_configs`: owns print enablement, order-type print flags, dispatch mode, trigger mode, auto-accept, deprecated/no-op voice compatibility fields, KDS flag, and KDS URL. Unique by merchant.
- `cloud_printers`: owns local printer name, serial number, secret key, provider type, role, per-order-type flags, active flag, and soft-delete timestamp. Active printer serial numbers are unique while soft-deleted historical rows may keep the same SN.
- `cloud_printer_reconciliation_jobs`: owns pending/resolved provider/local drift jobs after provider-first create/delete local failures. Pending jobs are unique by `(merchant_id, printer_sn, desired_action)`.
- `print_logs`: owns order-print execution observability and references `cloud_printers(id)`. It keeps historical printer identity after soft delete.
- Feieyun provider: owns real registration, removal, live status, printer info, and test/order print command acceptance.
- Flutter local Bluetooth device id in shared preferences currently owns App-local printer reconnect state only; backend cannot observe or reconcile it. Product decision 2026-06-10 requires this path to honor backend display config and add observability/deduplication boundaries.
- Flutter notification settings in shared preferences currently own App-local sound, voice, auto-accept, and auto-print behavior only; they are not synchronized with `order_display_configs`. Product decision 2026-06-10 requires App auto-accept to merge into backend truth, with the App only executing/displaying backend-authorized behavior.

## Trust, Authorization, And Tenant Checks

- Frontend page guard calls `ensureMerchantDeviceManagementAccess`, which first checks general merchant console access and then `GET /v1/merchant/devices/access`.
- Backend `GET /merchant/devices/access` resolves merchant staff identity and grants only owner/manager for active/approved merchants with region configured.
- Backend device/display route groups use `MerchantStaffMiddleware("owner", "manager")`.
- Printer read/update/delete/test/status handlers resolve current merchant and check `printer.MerchantID`.
- Reconciliation list is scoped by current merchant; retry loads the job and checks `job.MerchantID`.
- Downstream print/auto-accept workers read durable merchant ids from orders and printers, not from client input.

## Idempotency And Duplicate-Submit Checks

- Display-config page blocks duplicate save with `settingsSaving`. Backend PUT is partial create/update and last-write-wins; no version or idempotency key exists.
- Printer edit page blocks duplicate submit with `submitting`. Backend create checks active SN uniqueness and provider add is called before local insert.
- Printer list blocks duplicate delete/test via per-action ids and confirmation dialog `confirmDialogSubmitting`.
- Provider/local create and delete are not atomic. Reconciliation jobs provide durable recovery signals only for local DB failure after provider success.
- Reconciliation retry is idempotent only at the local job state level: resolved jobs return as-is; pending retries call the provider action again.
- Payment outbox auto-accept calls conditional merchant order logic, so repeated execution after status changes will no-op or skip through state checks.
- Print tasks dedupe accepted/ready re-entry by stable task key and printer; manual/test print are intentionally command-like and can create repeated provider commands.
- Flutter BLE print commands are currently local side effects with no backend idempotency, print log, or provider callback; duplicate App actions can print duplicate paper receipts. Product decision 2026-06-10 requires explicit observability and deduplication boundaries.
- Flutter local auto-accept currently relies on backend order status conditionality plus App deduplication, not a shared backend auto-accept truth/idempotency contract. Product decision 2026-06-10 requires a single backend truth for auto-accept.

## Recovery And Async Convergence Paths

- Display-config page rehydrates from PUT response and records freshness timestamp.
- Printer list reloads after delete and on return from edit pages; refresh failure preserves last trusted list with a visible message.
- Provider-first create/delete local failures record `cloud_printer_reconciliation_jobs`.
- Reconciliation jobs can be listed and retried through backend APIs, but no Mini Program UI caller was found.
- Auto-accept runs asynchronously in payment-domain outbox processing after paid order facts.
- Accepted/ready/manual print tasks run asynchronously in Redis print worker and update `print_logs`; order print callbacks/anomalies are covered by `merchant-order-operations`.
- Live status is a direct provider query and is not persisted as printer truth.
- Flutter BLE connect/print errors remain local App state and are not visible in backend device/printer recovery pages. Product decision 2026-06-10 requires adding observability/deduplication boundaries for this local-print surface.
- Flutter local auto-accept and BLE auto-print remain invisible to Mini Program display-config UI; backend order state can still change because the App calls the ordinary accept endpoint. Product decision 2026-06-10 requires the App auto-accept path to become a backend-truth execution/display path rather than a second local preference.

## Frontend Draft And Backend Rehydration

- Display config is a local draft until save. Save sends all visible fields and uses the backend response as the new initial form.
- Pull refresh is blocked while display config is dirty.
- Printer edit loads detail into local form, but after create/update it navigates back rather than rehydrating in-place. Parent list reloads on `onShow`.
- Printer delete reloads the list after success; test print does not alter list state.
- Live status popup uses request tokens to avoid late provider responses overwriting a closed or changed popup.
- Current UI does not expose reconciliation jobs, so provider/local drift recovery is invisible to merchants.

## Test Coverage Signals

Observed tests:

- `locallife/api/device_test.go` covers printer create/update/delete provider paths, reconciliation job creation after local failure, display-config default/create/update, `auto_accept_paid_orders` propagation, and deprecated `voice_*` PUT compatibility/no-op semantics.
- `locallife/api/device_reconciliation_test.go` covers reconciliation list and retry success/failure/resolved paths.
- `locallife/worker/task_payment_domain_outbox_test.go` covers auto-accept after paid order under display/printer config.
- `locallife/worker/task_print_order_test.go` covers split front/kitchen printing, manual-trigger gating, unsupported printer skips, retry print-log replay, and duplicate task-key re-entry.
- `locallife/logic/order_service_print_test.go` covers order-service print scheduling decisions.

Missing high-value tests:

- Mini Program wrapper/page test for display config save/reload and auto-accept/enable-print combined semantics.
- Fixed 2026-06-10: Mini Program display-config contract check proves voice controls are hidden/removed and the save payload no longer submits deprecated `voice_*` fields; backend API tests prove deprecated request fields are ignored while compatibility response/default state remains.
- Fixed 2026-06-09: deletion-with-existing-print-logs DB test now proves soft delete/deactivate preserves historical print logs and permits active SN re-registration; Yilianyun rebind after soft delete, historical print-job status over soft-deleted printers, skipped retry failure logging, and duplicate-SN migration down are also covered.
- End-to-end test that a paid order with `auto_accept_paid_orders=true` updates order state and enqueues one accepted print task only once across outbox retries.
- Reconciliation UI coverage if merchants are expected to recover provider/local drift themselves.
- Flutter local-printer tests for duplicate receipt printing, saved-device reconnect, disconnected-device failure copy, backend display-config enforcement, and backend observability/deduplication boundaries.
- Cross-client auto-accept tests proving backend display-config auto-accept and Flutter App alert handling converge safely through one backend truth without duplicate print side effects.

## Gaps And Refactor Notes

- Fixed 2026-06-10: Mini Program display-config voice controls are hidden/removed. Persisted backend `voice_*` fields remain deprecated/no-op compatibility state until a safe API/schema cleanup removes or quarantines them.
- Add a merchant-visible reconciliation section or remove/defer the Mini Program reconciliation wrappers to avoid unreachable recovery controls.
- Fixed 2026-06-09: physical printer deletion has been replaced with a soft deactivate/deleted state, preserving print-log history while excluding deleted printers from current printer reads and active printer selection.
- Decide whether provider printer metadata should be updated when local name/key changes. If key changes are only for future provider commands, document the boundary.
- Normalize `auto_accept_paid_orders` and `enable_print` semantics: either clear auto-accept when print is disabled, or make the UI/response explain that auto-accept is stored but inactive.
- Consider persisting a display-config row when GET returns defaults so future audits have one durable truth shape instead of a default-response path plus persisted path.
- Product decision 2026-06-10: Flutter local Bluetooth printing must honor backend display config and add backend-visible observability/deduplication boundaries. Implement this instead of keeping BLE receipts as an unconstrained invisible local convenience.
- Product decision 2026-06-10: Flutter local auto-accept must be unified with backend `auto_accept_paid_orders` as the same backend truth. The App should execute/display backend-authorized behavior, not own a separate auto-accept preference.

## Branch Exhaustion

- Entry branches checked: Mini Program display-config settings, printer list/edit/delete/test/status, print anomaly linkage, hidden reconciliation wrappers, backend auto-accept after payment, backend cloud-print tasks, Flutter local notification settings, Flutter local auto-accept, Flutter BLE printer connect/reconnect/print, and App order alert flow.
- Request branches checked: display-config GET/PUT, device access check, printer CRUD/test/live-status, reconciliation list/retry, order accept endpoint used by Flutter local auto-accept, backend print-task enqueue, and Flutter local shared-preferences paths with no backend request.
- Backend state branches checked: default versus persisted display config, `auto_accept_paid_orders`, `enable_print`, per-order-type print flags, deprecated voice fields, KDS fields, cloud printer registration and delete, provider-first/local-failure reconciliation jobs, print logs, manual/test print commands, accepted/ready print triggers, and App-local BLE state outside backend.
- Async branches checked: payment-domain outbox auto-accept, Redis print worker, provider print callback/anomaly scheduler, reconciliation retry, provider live status direct query, Flutter local BLE print after manual/local auto-accept, and App notification/voice delivery. Flutter BLE currently has no backend recovery or observability path; product decision 2026-06-10 requires that boundary to be added and tied to backend display config.
- Failure/retry branches checked: duplicate save/delete/test guards, provider add/delete success followed by local DB failure, reconciliation retry repeatability, soft-delete with historical print logs, default display-config path without row persistence, outbox retry idempotency, print task key dedupe, manual/test repeated commands, BLE duplicate paper prints, and local auto-accept retries. Product decision 2026-06-10 requires single backend auto-accept truth and BLE print deduplication/observability.
- Reader/consumer branches checked: display settings UI, printer list, print anomalies, order outbox, order service print scheduler, print worker, Flutter order alerts, Flutter local printer page, and merchant staff device access gate.
- Authorization/tenant branches checked: Mini Program device-management access guard, backend owner/manager device routes, active/approved/region device access check, printer merchant ownership checks, reconciliation merchant scope, and downstream print/auto-accept loading merchant ids from durable orders/printers.
- Zombie/unreachable branches checked: Mini Program voice settings are deprecated/no-op and now removed/hidden from the display-config page; backend PUT treats legacy `voice_*` request fields as ignored compatibility input; reconciliation wrappers have no merchant UI surface; Flutter notification settings are not synchronized with backend display config; Flutter BLE prints are not `print_logs`; Flutter local auto-accept is a second auto-accept path separate from backend config despite the 2026-06-10 decision to unify it with backend truth.
- Test-proof gaps checked: backend tests cover display config, deprecated `voice_*` no-op compatibility, printer provider/reconciliation, soft-delete with historical print logs, Yilianyun rebind after soft delete, historical print-job status/retry behavior for soft-deleted printers, duplicate-SN migration down, outbox auto-accept, print tasks, and print scheduling. Mini Program contract proof covers hidden/removed voice controls and the absence of voice fields in the save payload. Missing proof remains for one accepted print across outbox retries, reconciliation UI, Flutter BLE backend-config/dedupe/observability behavior, and cross-client backend-truth auto-accept convergence.
