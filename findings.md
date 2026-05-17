# Findings: WeApp User-Experience Function Audit

## Standards Context

- Mini Program work routes through `.github/instructions/weapp-mini-program.instructions.md`.
- Review output should be findings-first and should separate proven defects from upgrade opportunities.
- The active Mini Program baseline requires task-first page boundaries, backend-truth state, complete service-state-view wiring, visible loading/empty/error/retry states, duplicate-tap protection, and product copy.

## Evidence Log

- `weapp/miniprogram/app.json` registers a large multi-role app: consumer takeout/reservation/user center, dine-in, notification, merchant, rider, operator, platform, order, payment, registration.
- `weapp/package.json` exposes relevant gates: `compile`, `lint`, `quality:check`, and `gate:weapp`.
- The workspace has pre-existing modified files around Baofoo settlement account flows. Treat them as current state for audit, but do not revert or overwrite.
- Role-side visual standards both apply: consumer surfaces use `DESIGN_SYSTEM.md`; merchant/operator/platform/rider use `NON_CONSUMER_DESIGN_SYSTEM.md`.
- Route audit with local Node:
  - `app.json` registers 152 pages/subpackage pages.
  - All registered pages have a TS/JS file, WXML, and JSON.
  - 8 registered pages lack page-local WXSS, all in merchant finance/applyment settlement surfaces; this is not by itself a runtime break because pages may rely on shared WXSS, but it is a style-consistency audit point.
  - Two literal page references are not registered: `/pages/platform/merchants/applications` and `/pages/platform/riders/applications` in `weapp/miniprogram/pages/platform/dashboard/templates/pc-content.wxml`; must confirm whether that template is active.
- Confirmed candidate issues needing deeper review:
  - `components/map-view/index.ts` hardcodes demo Beijing coordinates and marker paths.
  - `components/delivery-map/index.ts` uses real props but still has asset-path assumptions for marker icons.
  - `pages/platform/dashboard/dashboard.ts` has a fallback "功能开发中", but current visible `actions` all provide URLs.
  - Reservation forms still contain placeholder-as-label drift such as `请输入姓名` and `请输入手机号`.
- Handler-binding static check:
  - 1,262 WXML handler references scanned.
  - 95 references are not directly found in same-path TS files. Many are likely false positives for behavior/runtime-mixin pages or WXML templates, so each candidate must be confirmed before reporting.
  - Highest-priority candidates to confirm: dine-in checkout voucher handlers, rider dashboard/order-hall runtime handlers, Baofoo settlement behavior handlers, merchant registration runtime handlers, and cart unavailable item handler.
- `platform/dashboard/templates/pc-content.wxml` has stale unregistered links, but `dashboard.wxml` does not import these templates and current dashboard uses registered links. Classify as dead/stale template debt unless a build tool includes it.
- Confirmed defect candidate:
  - `pages/dine-in/checkout/checkout.wxml` binds `onGuestCountChange`, `onVoucherPopupChange`, `closeVoucherPopup`, `onClearVoucher`, `onSelectVoucher`, and `onClaimVoucher`, and references `voucherVisible`, `voucherLoading`, `selectedVoucher`, `vouchers`.
  - `pages/dine-in/checkout/checkout.ts` defines none of those handlers/state fields. Current TS uses backend `calculateCheckoutCart` and displays `voucher_trials`, but the WXML still contains a local voucher-selection popup and editable guest-count stepper.
  - User impact: visible or reachable checkout controls can fail silently or produce Mini Program runtime warnings; guest count appears editable but does not update state; voucher choice UI has no service-state-view loop.
- Confirmed false-positive group:
  - Rider dashboard/order-hall handler bindings are provided by `riderDashboardRuntimeMethods` spread into the Page, so they are not missing handlers.
  - Baofoo settlement status/submit retry and primary action bindings are provided by shared behaviors.
  - Merchant store registration handler bindings are provided by `merchantStoreRegistrationRuntimeMethods`.
- Confirmed defect candidate:
  - `pages/takeout/cart/index.wxml` renders an "已下架" state and an "移除" button with `bindtap="onRemoveUnavailable"`.
  - `pages/takeout/cart/index.ts` does not define `onRemoveUnavailable`; it only has `onDecrease`, `onIncrease`, `removeLocalItem`, and `onClearMerchant`.
  - User impact: checkout blocks when selected groups contain unavailable items (`onCheckout` says "部分商品已下架，请移除后再结算"), but the visible per-item remove affordance is not wired.

## Validation Evidence

- `PATH="$HOME/.local/bin:$PATH" npm run compile` from `weapp/`: exited 0.
- Local route audit:
  - registered route count: 152
  - missing TS/JS/WXML/JSON among registered pages: 0
  - registered pages without page-local WXSS: 8, likely shared-style pages rather than direct runtime defects
- Local handler audit:
  - WXML handler references scanned: 1,262
  - Confirmed two real missing-action groups after filtering behavior/runtime false positives.

## Second UX Review Evidence

- `PATH="$HOME/.local/bin:$PATH" npm run check:wxml-handlers` from `weapp/`: exited 0 and validated 156 page WXML files.
- `PATH="$HOME/.local/bin:$PATH" node scripts/check-non-consumer-ui-patterns.js` from `weapp/`: failed on current non-consumer UI issues, mainly text-only local row actions and explanatory-card blocks.
- Confirmed user-visible wording issues:
  - `weapp/miniprogram/pages/reservation/confirm/index.wxml` uses internal flow copy such as "当前步骤", "本步将完成", and "本页只负责提交预订".
  - `weapp/miniprogram/app.json` and reservation pages mix "预定" and "预订" in TabBar, page titles, and visible labels.
  - `weapp/miniprogram/pages/merchant/printers/index.wxml` exposes device/merchant IDs, raw printer type/role values, and vendor-facing status labels such as "云端返回".
  - `weapp/miniprogram/pages/operator/region/config.wxml`, `operator/timeslot/index.wxml`, and `operator/delivery-fee/index.ts` expose "ID" / "区域ID" in operator-visible fallback and error text.
- Confirmed maintenance risk:
  - `weapp/miniprogram/pages/platform/dashboard/templates/mobile-content.wxml`, `tablet-content.wxml`, and `pc-content-full.wxml` are not referenced by active `dashboard.wxml`, but still contain stale responsive dashboard copy and handler bindings.

## Current Findings Summary

### Baseline Violations

1. Dine-in checkout exposes incomplete controls for guest count and voucher selection.
2. Takeout cart blocks checkout on unavailable items but the visible "移除" action is not implemented.

### Upgrade Opportunities

1. Remove or quarantine stale platform dashboard templates with unregistered links.
2. Replace or delete demo map wrapper components before they are used in delivery/order surfaces.
3. Consumer empty states should reduce B-side onboarding prominence unless the user's intent is clearly provider onboarding.
4. Reservation forms still have placeholder-as-label copy drift.
5. Reservation confirmation copy should describe user outcome and next action instead of internal page responsibilities.
6. Merchant printer management should hide backend/vendor fields from the daily task surface and move diagnostics behind an advanced or copy-for-support action.
7. Operator region configuration should use recoverable user wording instead of exposing region IDs.
8. Non-consumer row actions and explanatory cards should be brought back under the Mini Program UI gate.

### Positive Evidence

- Payment flow does not treat `wx.requestPayment` as business-final; it routes through backend polling/requery and a pending-confirmation result page.
- Workbench entry resolution is backend/profile-driven and does not dump every B-side console entry onto all users.
