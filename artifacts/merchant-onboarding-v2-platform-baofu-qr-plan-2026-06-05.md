# Merchant Onboarding V2 Mini Program Frontend Refactor Plan

Updated: 2026-06-05
Risk class: G3 - this is a Mini Program refactor around merchant onboarding, Baofoo/Baofu account-opening status, identity/bank-document UX, payment-readiness guidance, and account-willingness confirmation copy. Backend code is not in scope.
Target area: `weapp/` only. `locallife/` backend files are reference material, not implementation targets.

## 1. Background And Current Decision

Merchant onboarding remains a three-stage business process:

1. 平台入驻: merchant submits LocalLife application material and waits for platform approval.
2. 宝付开户: approved merchant owner submits Baofoo settlement-account material through the existing Baofoo account-opening flow.
3. 微信扫码确认开户意愿: merchant legal representative uses WeChat to scan LocalLife's Baofoo expansion QR code and completes the external confirmation flow outside LocalLife.

Baofoo has confirmed that stage 3 cannot be completed by LocalLife backend API. This round is therefore a frontend flow refactor:

- Do not change backend routes, SQL, Swagger, config, workers, or Baofoo provider contracts.
- Do not implement Baofoo secondary-authentication APIs.
- Build a new isolated Mini Program page group that composes the existing platform-application and Baofoo-opening frontend APIs.
- Make the third step a standalone guide UI that displays the expansion QR code and saves it to the phone album. The QR image domain is already in the Mini Program whitelist.

## 2. Explicit Non-Goals

Do not implement or wire these Baofoo interfaces in this round:

- `merchant_confirm`
- `image_upload`
- `confirm_apply_state_query`
- `confirm_state_query`

Do not add or modify:

- `GET /v1/merchant/onboarding-v2` or any other backend aggregation endpoint.
- Backend config such as `BAOFU_WECHAT_EXPANSION_QR_*`.
- Backend feature flags.
- SQL/sqlc queries, migrations, mocks, Swagger annotations, workers, schedulers, or provider clients.
- Existing `/v1/merchant/application` behavior.
- Existing `/v1/merchant/settlement-account` behavior.
- Existing merchant application page or existing Baofoo settlement-account pages during the pilot.
- Any local "开户意愿已确认" truth flag without backend/provider observability.

## 3. Existing Frontend And Backend Capabilities To Reuse

### 3.1 Platform Application

Current Mini Program page:

- `weapp/miniprogram/pages/merchant/settings/application/index.ts`
- `weapp/miniprogram/pages/merchant/settings/application/index.wxml`
- `weapp/miniprogram/pages/merchant/settings/application/index.wxss`
- `weapp/miniprogram/pages/merchant/settings/application/index.json`

Current shared API:

- `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts`

Important existing calls:

- `getMerchantApplication()` calls `GET /v1/merchant/application` and may create or return a draft.
- `getMyApplication()` calls `GET /v1/merchants/applications/me` and is the preferred read call for the new hub so the hub does not create a draft merely by loading.
- Platform application actions remain owned by the existing application page.

### 3.2 Baofoo Settlement Account

Current Mini Program pages:

- `weapp/miniprogram/pages/merchant/finance/settlement-account/index.ts`
- `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.ts`

Current shared frontend files:

- `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-account.ts`
- `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-account-status.ts`
- `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-account-view.ts`
- `weapp/miniprogram/pages/merchant/_main_shared/services/baofu-account-role-page.ts`
- `weapp/miniprogram/pages/merchant/_main_shared/services/baofu-account-onboarding.ts`
- `weapp/miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-submit.ts`
- `weapp/miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-status.ts`
- `weapp/miniprogram/pages/merchant/_components/baofu-onboarding-wait/index.*`

Important existing call:

- `getMerchantBaofuSettlementAccount()` calls the existing owner-only Baofoo settlement-account GET.

Known current UX issue:

- `baofu-account-role-page.ts` auto-enters submit context for `profile_pending`, which hides the larger three-stage onboarding picture.
- The new hub must not reuse that auto-redirect behavior.

Existing long-wait capability:

- The current Mini Program already implements the long wait after Baofoo profile submission.
- `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.ts` opens the wait panel immediately after submit, starts `_startBaofuSubmitPendingTick(sessionId)`, calls `startBaofuAccountOnboarding(...)`, and passes `onProgress` into `_handleBaofuOnboardingProgress(...)`.
- `weapp/miniprogram/pages/merchant/_main_shared/services/baofu-account-onboarding.ts` owns `pollBaofuSettlementAccountStatus(...)`, `formatBaofuOnboardingPollProgress(...)`, and `buildBaofuOnboardingWaitViewFromAccount(...)`.
- `weapp/miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-submit.ts` owns submit-page timer/session guards, progress rendering, manual refresh, and cancellation on hide/unload.
- `weapp/miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-status.ts` owns status-page long wait and recovery when an existing account is still processing.
- `baofu-onboarding-wait` is already used by both the merchant Baofoo status page and submit page.
- The new v2 flow must reuse these existing long-wait services/behaviors/components. It must not reimplement the submit-page long poll or create a second competing wait loop.
- To keep existing finance entrypoints unchanged while making stage 2 success continue into the v2 guide, create a v2 Baofoo submit adapter page under `merchant/onboarding-v2/`. The adapter reuses the existing shared Baofoo submit behavior, onboarding service, profile form helpers, and `baofu-onboarding-wait`, but its terminal return path is the v2 hub/guide instead of the legacy settlement-account status page.

## 4. Target Frontend Architecture

Build an isolated Mini Program page group:

- `weapp/miniprogram/pages/merchant/onboarding-v2/index.*`
- `weapp/miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.*`
- `weapp/miniprogram/pages/merchant/onboarding-v2/intent/index.*`

Add focused frontend orchestration and ViewState modules:

- `weapp/miniprogram/pages/merchant/_main_shared/services/merchant-onboarding-v2-runtime.ts`
- `weapp/miniprogram/pages/merchant/_main_shared/services/merchant-onboarding-v2-view.ts`
- `weapp/miniprogram/pages/merchant/_main_shared/config/merchant-onboarding-v2.ts`
- `weapp/scripts/check-merchant-onboarding-v2-view.js`

The runtime service composes existing APIs:

1. Load platform state with `getMyApplication()`.
2. Treat missing application as `platform_not_started` only when the error is the known no-application predicate defined below.
3. Do not call Baofoo status until platform application is `approved`.
4. After platform approval, call `getMerchantBaofuSettlementAccount()`.
5. If Baofoo read is denied because the approved merchant owner is not ready for Baofoo access, show `owner_not_ready_after_approval` and do not navigate to owner-only submit.
6. Map existing Baofoo statuses into the new page ViewState.
7. Keep the third step locked until Baofoo status is `ready`.
8. When returning from the v2 Baofoo submit adapter with a successful terminal Baofoo state, refresh backend truth first, then open the third-step guide once `baofu_ready` is confirmed.

No backend truth is invented by the frontend. The hub is a task-oriented ViewState over existing backend responses.

### 4.1 Error Mapping And Recovery Boundaries

The runtime service must classify errors before turning them into business states:

- `platform_not_started` only when `getMyApplication()` fails with `AppError.statusCode === 404` and either the request/envelope code is the existing not-found code or the mapped user message is `您还没有申请记录` / no-application. Do not map network, timeout, 5xx, 401, or unrelated 403 errors to `platform_not_started`.
- Login/auth failures (`401`, `ErrorType.AUTH`, token-expired messages) are page errors that ask the user to re-enter/login; they are not onboarding states.
- Platform permission failures (`403` without the known no-application signal) are page errors with mapped Chinese copy; they are not `platform_not_started`.
- Baofoo missing/profile-needed states are only the response status `profile_pending` or a backend response already normalized by the existing Baofoo API/view helpers. Do not guess profile-pending from network failure or provider text.
- `owner_not_ready_after_approval` is only for approved platform application plus a Baofoo read/access error whose `statusCode`, `code`, or mapped user message matches the existing merchant-owner-not-ready / account-not-active / role-not-ready semantics. Generic 403, 404, network, timeout, and 5xx errors remain recoverable load/refresh errors.
- UI copy must use existing mapped Chinese messages such as `getErrorUserMessage()` output or a new local view mapper. Do not display raw backend, SQL, gateway, or Baofoo provider messages.

Hub runtime state must explicitly distinguish:

- `initialLoading`
- `initialError`
- `loaded`
- `refreshing`
- `refreshError`
- `lastTrustedViewState`
- `requestSeq`

When refresh fails after a trusted ViewState exists, keep `lastTrustedViewState` visible and show an inline `refreshError`. A slower earlier response must not overwrite a newer response; compare `requestSeq` before applying results. Pull-down refresh and primary refresh buttons share the same duplicate-tap guard.

## 5. Frontend ViewState

### 5.1 Platform States

| Existing application result | Onboarding v2 state | Primary action |
| --- | --- | --- |
| no application | `platform_not_started` | `开始平台入驻` -> existing application page |
| `draft` | `platform_draft` | `继续填写入驻资料` -> existing application page |
| `submitted` | `platform_submitted` | `刷新审核状态` |
| `rejected` | `platform_rejected` | `修改后重新提交` -> existing application page |
| `approved` | `platform_approved` | continue to Baofoo step |

The hub must not use `getMerchantApplication()` on first load, because that endpoint can create a draft. Draft creation remains an explicit result of entering the existing application page.

### 5.2 Baofoo States

The Baofoo step must follow the existing backend state machine and current Mini Program status UI, not a simplified new state model.

Backend truth sources checked for this plan:

- State constants: `locallife/db/sqlc/constants.go`
- GET response composition: `locallife/api/baofu_settlement_account_read.go` and `locallife/api/baofu_settlement_account_response.go`
- POST/start/recover orchestration: `locallife/api/baofu_settlement_account.go`, `locallife/logic/baofu_account_onboarding_service.go`, `locallife/logic/baofu_account_onboarding_open.go`, `locallife/logic/baofu_account_onboarding_apply.go`
- Merchant report / APPLET authorization continuation: `locallife/logic/baofu_account_merchant_report_service.go`, `locallife/logic/baofu_payment_readiness.go`, `locallife/worker/baofu_account_opening_recovery_scheduler.go`
- Current Mini Program status UI: `weapp/miniprogram/pages/merchant/finance/settlement-account/index.*`, `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.ts`, `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-account-status.ts`, `weapp/miniprogram/pages/merchant/_main_shared/api/baofu-account-view.ts`, `weapp/miniprogram/pages/merchant/_main_shared/services/baofu-account-role-page.ts`, `weapp/miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-status.ts`

Backend flow summary for merchant:

1. No profile/flow/binding returns or behaves as `profile_pending`.
2. Merchant may open as `business` or `personal`; the existing submit page owns this choice and form validation.
3. Merchant does not use the rider/operator user-paid verify-fee path in ordinary flow; if `verify_fee_pending` / `verify_fee_processing` appears, display it as a recoverable backend state, not as a new merchant primary path.
4. Complete profile moves the flow to `opening_processing`; API/worker/callback recovery applies Baofoo account-open results.
5. Baofoo account `active` is not sufficient for merchant payment readiness. Merchant then continues through WeChat merchant report: `merchant_report_processing`.
6. When merchant report succeeds but APPLET authorization directory binding is still pending, response is `applet_auth_pending`.
7. Only when backend reports `ready` / `payment_ready=true` should the new hub unlock the third guide step.
8. `failed` may come from Baofoo account open, merchant report failure, or APPLET binding failure. The hub can route to existing submit only when existing view helpers expose profile resubmission; otherwise use refresh/contact fallback.
9. `voided` is a terminal local flow state for abandoned/replaced flow and should not lead to Baofoo submit from the new hub.

| Existing Baofoo result | Onboarding v2 state | Primary action |
| --- | --- | --- |
| platform not approved | `locked_until_platform_approved` | none |
| platform approved but owner/Baofoo read denied | `owner_not_ready_after_approval` | `刷新`, secondary `联系平台处理` |
| missing or `profile_pending` | `baofu_profile_pending` | `提交宝付开户资料` -> v2 Baofoo submit adapter |
| `verify_fee_pending` | `baofu_verify_fee_pending` | `刷新开户状态`; merchant path should rarely show this |
| `verify_fee_processing` | `baofu_verify_fee_processing` | `刷新开户状态` |
| `opening_processing` | `baofu_opening_processing` | `刷新开户状态` |
| `merchant_report_processing` | `baofu_merchant_report_processing` | `刷新开户状态` |
| `applet_auth_pending` | `baofu_applet_auth_pending` | `刷新开户状态` |
| `failed` | `baofu_failed` | `修改开户资料` -> v2 Baofoo submit adapter when existing view helper permits resubmission; otherwise `联系平台处理` |
| `voided` | `baofu_voided` | `联系平台处理` |
| `ready` | `baofu_ready` | unlock third step |

Important copy boundary:

- Use backend labels or existing frontend status helpers for status meanings. Do not invent new labels for Baofoo flow states in the hub.
- `merchant_report_processing` and `applet_auth_pending` are Baofoo merchant-report / APPLET authorization states.
- They are not account-willingness confirmation states.
- Do not label them as "开户意愿已确认" or "待确认开户意愿".
- `ready` means the existing Baofoo settlement/payment readiness contract says the merchant account can proceed; it does not mean the third external account-willingness confirmation has been completed.
- The hub action that opens the third-step page should be `查看确认流程` or `保存确认二维码`; do not use `去确认开户意愿`, because the confirmation itself happens in the legal representative's WeChat, outside LocalLife.

### 5.3 Third-Step States

| Condition | Intent state | Primary action |
| --- | --- | --- |
| Baofoo not `ready` | `locked_until_baofu_ready` | `返回入驻进度` |
| Baofoo `ready` and QR configured | `intent_qr_pending` | `保存二维码` |
| Baofoo `ready` but QR config missing in frontend | `intent_qr_unavailable` | `联系平台处理` / `返回入驻进度` |

This round does not need a backend-observed terminal state for account-willingness confirmation. The third-step page remains guide-only even after the merchant returns from WeChat. Any status refresh on this page only rechecks platform/Baofoo account readiness; it must not present account-willingness confirmation as an in-app synchronized result.

`联系平台处理` must be an actionable fallback, not only static copy:

- Prefer the existing merchant support/customer-service entry if one is available in the merchant shell.
- Otherwise provide a small action to copy the current merchant id or Baofoo account owner id when available, with copy text such as `复制商户编号`.
- The fallback copy should tell the merchant to send that编号 to platform support, without exposing provider raw errors.

## 6. QR Source And Save Behavior

QR source:

- Store the QR image URL in `weapp/miniprogram/pages/merchant/_main_shared/config/merchant-onboarding-v2.ts`.
- Example export:

```ts
export const MERCHANT_ONBOARDING_V2_INTENT_QR_URL = 'https://...'
export const MERCHANT_ONBOARDING_V2_INTENT_QR_UPDATED_AT = '2026-06-05'
```

Operational facts:

- The QR image domain is already in the Mini Program whitelist.
- No backend config or domain validation is needed in this round.
- If the QR changes, update the frontend constant and include the new `updated_at` value in the release.
- The guard script must fail release validation when `MERCHANT_ONBOARDING_V2_INTENT_QR_URL` is empty, still equals the placeholder `https://...`, is not `https`, or is outside the configured whitelisted QR host list for this frontend module.

Save behavior:

- Use `wx.downloadFile` for the configured QR URL.
- Use `wx.saveImageToPhotosAlbum` to save it to the phone album.
- Reuse or mirror the existing image save pattern in `weapp/miniprogram/pages/merchant/_utils/merchant-tables-shared.ts`.
- Define page state `qrSaveState`: `idle`, `downloading`, `saving`, `saved`, `download_failed`, `save_failed`, `permission_denied`.
- Disable the save button while `qrSaveState` is `downloading` or `saving`.
- If album permission is denied, show a modal with `去设置`; after returning from settings, re-check permission and keep the QR visible.
- If refresh fails after the QR was loaded, keep the last trusted QR image visible and show an inline refresh error.

Third-step copy must cover both operation patterns:

- 法人可在本人微信中从相册识别保存的二维码。
- 法人也可用本人微信扫描另一台设备上展示的二维码。
- Do not imply that a non-legal-person WeChat account can complete the confirmation.

## 7. Mini Program Page Plan

### 7.1 Hub Page

Files:

- Create `weapp/miniprogram/pages/merchant/onboarding-v2/index.ts`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/index.wxml`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/index.wxss`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/index.json`
- Modify `weapp/miniprogram/app.json` by appending `onboarding-v2/index` to the existing `pages/merchant` subpackage.

UI structure:

- TDesign `steps` for 平台入驻 / 宝付开户 / 开户意愿确认.
- Current-task panel with concise status, next action, and failure/locked guidance.
- Compact completed/waiting rows for the other stages.
- Bottom action area with safe-area padding.

Behavior:

- Load on enter and pull-down refresh.
- Avoid calling Baofoo read until platform is approved.
- Use `lastTrustedViewState`, `refreshError`, and `requestSeq` to keep a stable trusted progress view during weak-network refresh failures and out-of-order responses.
- Disable duplicate refresh/navigation while loading.
- Preserve re-entry behavior after navigating to the existing platform application page or the v2 Baofoo submit adapter.
- Route Baofoo profile submission to the v2 Baofoo submit adapter, not the legacy finance submit route.
- Render first-screen error as an inline page state, not toast-only.
- Use `查看确认流程` or `保存确认二维码` for the third-stage hub action when Baofoo is `ready`.

### 7.2 Baofoo Submit Adapter Page

Files:

- Create `weapp/miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.ts`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.wxml`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.wxss`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.json`
- Modify `weapp/miniprogram/app.json` by appending `onboarding-v2/baofu-submit/index` to the existing `pages/merchant` subpackage.

Ownership:

- Reuse `baofuSettlementSubmitBehavior(...)`, `startBaofuAccountOnboarding(...)`, `pollBaofuSettlementAccountStatus(...)`, Baofoo profile form helpers, and `baofu-onboarding-wait`.
- Keep merchant account-opening mode, enterprise form, personal form, bank form, validation, duplicate-submit guards, submit timer, `onProgress`, and terminal-state polling aligned with the existing merchant Baofoo submit page.
- Change only the v2 adapter's route shell and terminal return behavior. The legacy finance submit/status pages remain unchanged.

Behavior:

- On submit, immediately show the existing long-wait panel and elapsed timer.
- While backend states are `opening_processing`, `merchant_report_processing`, `applet_auth_pending`, or `verify_fee_processing`, keep the wait panel visible and update it from `pollBaofuSettlementAccountStatus(...)` progress.
- If terminal result is `ready`, redirect/return to the v2 hub with a source marker such as `from=baofu_submit`. The hub must refresh backend truth and then open the third-step guide only after it confirms `baofu_ready`.
- If terminal result is `failed`, `voided`, `profile_pending`, `verify_fee_pending`, or a recoverable error, return to the v2 hub or keep the wait panel action consistent with existing wait view helpers; do not jump to the QR guide.
- If the user leaves the adapter mid-wait, cancel the session exactly as the shared behavior already does. On re-entry to the hub, show backend truth from `getMerchantBaofuSettlementAccount()`.

Copy rules:

- Use existing Baofoo submit-page copy for form fields and validation.
- Use "开户状态同步中" / "正在提交资料" for waiting states.
- Do not mention account-willingness confirmation inside the submit form or wait panel. The third step begins only after backend reports `ready`.

### 7.3 Intent QR Guide Page

Files:

- Create `weapp/miniprogram/pages/merchant/onboarding-v2/intent/index.ts`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/intent/index.wxml`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/intent/index.wxss`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/intent/index.json`
- Modify `weapp/miniprogram/app.json` by appending `onboarding-v2/intent/index` to the existing `pages/merchant` subpackage.

UI structure:

- Status tag: `需法人微信确认` or `二维码待配置`.
- QR image with stable dimensions and nonblank/error fallback.
- Operation steps:
  1. 保存二维码到手机相册。
  2. 请法人使用本人微信打开扫一扫。
  3. 从相册识别二维码，或扫描另一台设备上展示的二维码。
  4. 按微信页面提示完成确认后，可返回小程序查看入驻进度。
- Actions:
  - `保存二维码`
  - `返回入驻进度`

Copy rules:

- Use "确认开户意愿" only for the third external WeChat step.
- Use "宝付开户" for the second stage.
- Use "授权目录绑定" only for APPLET binding state.
- Do not write "系统将自动确认开户意愿".
- Do not write "已确认" unless a later backend-observable truth source exists.
- Do not expose raw Baofoo terms such as `merchant_confirm`, `confirmState`, or provider error codes.

## 8. Isolation And Rollout

First implementation:

- Add the new page routes to `app.json`.
- Do not change current merchant dashboard/config/finance entrypoints.
- Open the new route only by direct path or DevTools during pilot.
- Existing platform application page remains the action owner for stage 1.
- The v2 Baofoo submit adapter owns stage 2 inside the new flow, while existing Baofoo status/submit pages remain unchanged for existing finance entrypoints.

Pilot:

1. Test direct route with no application, draft, submitted, rejected, approved.
2. Test approved but owner/Baofoo read denied.
3. Test Baofoo profile pending, failed, processing, APPLET auth pending, ready.
4. Test Baofoo submit adapter long wait: submit starts wait immediately, progress updates from backend polling, terminal `ready` returns to v2 hub and enters the QR guide after refresh.
5. Test QR display and save-to-album flow.
6. Test legal representative WeChat scan outside LocalLife.

Cutover:

- A later frontend-only patch may update `weapp/miniprogram/pages/merchant/config/index.ts`, `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts`, or another release-owner-selected entrypoint.
- Rollback is hiding/reverting that frontend entrypoint patch. Existing pages and APIs remain unchanged.

## 9. Implementation Tasks

### Task 1: ViewState And Guard Script

Files:

- Create `weapp/miniprogram/pages/merchant/_main_shared/config/merchant-onboarding-v2.ts`
- Create `weapp/miniprogram/pages/merchant/_main_shared/services/merchant-onboarding-v2-view.ts`
- Create `weapp/miniprogram/pages/merchant/_main_shared/services/merchant-onboarding-v2-runtime.ts`
- Create `weapp/scripts/check-merchant-onboarding-v2-view.js`

Guard cases:

- platform not started/draft/submitted/rejected/approved;
- owner not ready after approval maps to refresh/contact and never Baofoo submit;
- Baofoo profile pending/processing/failed/voided/ready;
- processing states map through the same labels/wait semantics as existing Baofoo helpers;
- APPLET auth pending is not account-willingness confirmation;
- v2 Baofoo submit success requires backend-confirmed `baofu_ready` before entering the QR guide;
- QR available/unavailable;
- QR save states;
- QR URL is non-empty, non-placeholder, `https`, and within the module's configured allowed host list;
- error predicates do not collapse auth/network/5xx failures into business states;
- stale refresh preserves `lastTrustedViewState` and older responses cannot overwrite newer ones;
- no text implies API-based or in-app account-willingness confirmation.

Validation:

```bash
cd /home/sam/locallife/weapp
node scripts/check-merchant-onboarding-v2-view.js
npm run compile
```

### Task 2: Hub Page

Files:

- Create `weapp/miniprogram/pages/merchant/onboarding-v2/index.ts`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/index.wxml`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/index.wxss`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/index.json`
- Modify `weapp/miniprogram/app.json`

Implementation steps:

- Append route to existing merchant subpackage pages array.
- Load runtime ViewState on enter and pull-down refresh.
- Route stage 1 actions to `/pages/merchant/settings/application/index`.
- Route stage 2 actions to `/pages/merchant/onboarding-v2/baofu-submit/index` only when state permits.
- Route stage 3 action to `/pages/merchant/onboarding-v2/intent/index`.
- When entered with `from=baofu_submit`, refresh backend truth first. If the refreshed ViewState is `baofu_ready`, open or foreground the QR guide; otherwise keep the current Baofoo state visible.
- Show locked/recoverable states for platform not approved and owner not ready.
- Implement `lastTrustedViewState`, inline `refreshError`, and `requestSeq` guards.
- Implement actionable platform-support fallback by reusing the existing merchant support entry or copying the merchant/Baofoo owner id for support.
- Disable duplicate refresh and duplicate navigation.

Validation:

```bash
cd /home/sam/locallife/weapp
node scripts/check-merchant-onboarding-v2-view.js
npm run compile
npm run lint
```

### Task 3: Baofoo Submit Adapter Page

Files:

- Create `weapp/miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.ts`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.wxml`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.wxss`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.json`
- Modify `weapp/miniprogram/app.json`

Implementation steps:

- Reuse the existing merchant Baofoo form composition, profile builders, validation helpers, and `baofuSettlementSubmitBehavior(...)`.
- Set the v2 adapter return/status path to the v2 hub, with a `from=baofu_submit` marker for terminal results.
- Keep `startBaofuAccountOnboarding(...)`, `_startBaofuSubmitPendingTick(...)`, `onProgress`, `_handleBaofuOnboardingProgress(...)`, and `baofu-onboarding-wait` behavior intact.
- Preserve duplicate-submit guards, cancellation on hide/unload, manual retry/refresh actions, and mapped Chinese error copy.
- Confirm `opening_processing`, `merchant_report_processing`, `applet_auth_pending`, and `verify_fee_processing` remain visible as a long wait until backend terminal state or user leaves.
- Do not add account-willingness QR copy or QR actions to the submit adapter.

Validation:

```bash
cd /home/sam/locallife/weapp
node scripts/check-merchant-onboarding-v2-view.js
npm run compile
npm run lint
```

### Task 4: Intent QR Guide Page

Files:

- Create `weapp/miniprogram/pages/merchant/onboarding-v2/intent/index.ts`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/intent/index.wxml`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/intent/index.wxss`
- Create `weapp/miniprogram/pages/merchant/onboarding-v2/intent/index.json`

Implementation steps:

- Load runtime ViewState and require `baofu_ready` before showing QR guide.
- Render QR image from frontend config.
- Implement download-and-save-to-album.
- Track `qrSaveState` and render inline errors.
- Keep QR visible after refresh/download/save errors when previously loaded.
- Add `返回入驻进度`.
- Do not add a local "我已完成确认" submit action.
- Do not show a refresh action that implies LocalLife can observe account-willingness confirmation in this round.

Validation:

```bash
cd /home/sam/locallife/weapp
node scripts/check-merchant-onboarding-v2-view.js
npm run compile
npm run lint
npm run quality:check
```

Manual DevTools checks:

- QR image renders.
- Save succeeds after album permission grant.
- Permission denial opens settings guidance and recovers.
- Download/save failure renders inline recoverable state.
- Weak-network refresh preserves last trusted QR.
- Hub re-entry after returning from the platform application page or v2 Baofoo submit adapter refreshes platform/Baofoo account truth.
- Intent page re-entry never marks account-willingness confirmation as completed.

Required preview/experience-build checks:

- On a real device or experience build, verify the configured QR host passes Mini Program download-domain policy for `wx.downloadFile`.
- On a real device or experience build, verify `wx.saveImageToPhotosAlbum` succeeds after permission grant.
- On a real device or experience build, verify permission denial opens settings guidance and recovers after the user grants `scope.writePhotosAlbum`.
- If these real-device checks cannot be completed before release, the handoff must explicitly mark QR save and album-permission behavior as unverified residual risk.

## 10. Final Frontend Validation

```bash
cd /home/sam/locallife/weapp
node scripts/check-merchant-onboarding-v2-view.js
npm run compile
npm run lint
npm run gate:weapp
npm run quality:check
```

No backend validation, `make sqlc`, `make swagger`, or `make check-generated` is required for this round because no backend files or contracts are changed.

Manual validation for the QR save path must include a real device or Mini Program experience build, not only DevTools, because download-domain enforcement and album permission behavior can differ from local simulation.

## 11. Acceptance Criteria

- New Mini Program page group shows the three-stage onboarding flow.
- No backend files, routes, SQL, Swagger, workers, schedulers, or provider clients are modified.
- Current merchant application page remains usable and unchanged.
- Current Baofoo settlement-account pages remain usable and unchanged.
- V2 Baofoo submit adapter reuses the existing Baofoo long-wait behavior, progress polling, wait panel, duplicate-submit guard, and hide/unload cancellation.
- After v2 Baofoo submit reaches backend-confirmed `ready`, the hub refreshes backend truth and then enters the QR guide.
- Hub does not create a platform application draft on first load.
- Hub does not call Baofoo status before platform application is approved.
- `owner_not_ready_after_approval` cannot navigate to Baofoo submit.
- Third step unlocks only after Baofoo `ready`.
- Third step is guide-only and does not call Baofoo secondary-authentication APIs.
- QR saves to phone album using the already-whitelisted image domain.
- QR save permission, download, save, refresh, duplicate-tap, and re-entry states are handled visibly.
- QR host/domain and album-save permission flow are verified on a real device or experience build, or explicitly handed off as residual risk.
- Guard script prevents false in-app confirmation copy and status-mapping drift.
