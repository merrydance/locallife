# WeApp Full-Score Audit And Improvement Plan

**Date:** 2026-05-28

**Scope:** `weapp/` WeChat Mini Program, including customer, merchant, rider, operator, platform, payment, refund, withdrawal, registration, and console surfaces.

**Current Score:** 78 / 100

**First Remediation Checkpoint:** 91 / 100 after the 2026-05-28 first implementation pass.

**Second Remediation Checkpoint:** 93 / 100 after the first Phase 4 ownership extraction slice.

**Third Remediation Checkpoint:** 94 / 100 after the merchant registration view-owner extraction slice.

**Fourth Remediation Checkpoint:** 95 / 100 after the merchant registration OCR display and upload-feedback owner extraction slice.

**Target Score:** 100 / 100

**Review Baseline:**

- `.github/instructions/weapp-mini-program.instructions.md`
- `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
- `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`
- `.github/standards/weapp/REVIEW_CHECKLIST.md`

**Validation Evidence From Review:**

- `npm run compile`: passed.
- `npm run quality:check`: failed at `gate:business-status-boundary`.
- `npm run gate:payment-workflow-boundary`: passed.

**Validation Evidence After First Remediation Pass:**

- `npm run check:action-feedback`: passed.
- `npm run check:placeholder-label-drift`: passed.
- `npm run compile`: passed.
- `npm run quality:check`: passed.

**First Remediation Scope:**

- Closed the blocking business-status quality gate by moving platform merchant/operator/rider status views and delivery tracking poll eligibility out of page scripts.
- Added a placeholder-label drift gate and fixed the visible repeated placeholders found during the audit.
- Replaced final-sounding Toast-only feedback for platform pause/resume and printer commands with durable in-page result states.
- Remaining gap to 100 is concentrated in Phase 4 ownership extraction and Phase 5 scenario evidence for money, async, weak-network, and re-entry paths.

**Second Remediation Scope:**

- Extracted rider dashboard delivery view ownership from `rider-dashboard-runtime.ts` into `rider-dashboard-delivery-view.ts`.
- Shared the same dashboard delivery builder between rider workbench summary data and rider dashboard runtime refresh data.
- Added `check:rider-dashboard-runtime-owner` to verify behavior and block the extracted delivery view/distance rules from drifting back into the large runtime file.

**Third Remediation Scope:**

- Extracted merchant store registration image rendering, persisted image URL, map-location label, safe number, and region text matching helpers into `merchant-store-registration-view.ts`.
- Added `check:merchant-store-registration-view-owner` to verify behavior and block those pure view-owner helpers from drifting back into `merchant-store-registration-runtime.ts`.
- Kept OCR display feedback, upload submission, media signing, backend sync, and page orchestration inside the existing runtime for later, smaller ownership slices.

**Fourth Remediation Scope:**

- Extracted merchant store registration OCR readiness, processing, failure, progress-message, and upload-feedback view-state rules into `merchant-store-registration-view.ts`.
- Kept `merchant-store-registration-runtime.ts` as the page-orchestration boundary for current image arrays, upload calls, and backend refresh, while delegating OCR display state construction to the view owner.
- Extended `check:merchant-store-registration-view-owner` with OCR display and upload-feedback behavior assertions, plus forbidden runtime patterns to prevent the extracted OCR view rules from drifting back into the runtime file.

---

## Executive Summary

The Mini Program is already beyond prototype quality: TypeScript compiles, lint ran cleanly inside `quality:check`, payment workflow boundaries are enforced, and many domain-specific gates exist for payment, refund, withdrawal, rider delivery, merchant order, reconciliation, and food-safety paths.

The gap to 100 is not basic build health. The main gap is production governance closure: business state semantics still leak into page scripts, some large page/runtime files still own too much workflow logic, several forms still repeat labels in placeholders, and some high-impact state-changing actions rely on success Toasts after reloads instead of stronger visible result states. These are fixable, but they need a structured convergence pass rather than scattered polish.

---

## Problem List

### P0. Full WeApp Quality Gate Fails

**Severity:** High

**Evidence:**

- `npm run quality:check` exits with code 1.
- Failure occurs at `npm run gate:business-status-boundary`.

**Impact:**

The Mini Program cannot be treated as fully release-clean under the repository's own quality baseline. Even though compile and lint pass, the business-status boundary failure means page scripts still interpret business state directly instead of consuming shared status helpers.

**Representative files:**

- `weapp/miniprogram/pages/orders/tracking/index.ts`
- `weapp/miniprogram/pages/platform/merchants/detail.ts`
- `weapp/miniprogram/pages/platform/merchants/index.ts`
- `weapp/miniprogram/pages/platform/operators/detail.ts`
- `weapp/miniprogram/pages/platform/operators/index.ts`
- `weapp/miniprogram/pages/platform/riders/detail.ts`
- `weapp/miniprogram/pages/platform/riders/index.ts`

**Acceptance Target:**

- `npm run quality:check` passes from `weapp/`.

### P1. Page Scripts Hardcode Business Status Semantics

**Severity:** High

**Evidence:**

- `weapp/miniprogram/pages/orders/tracking/index.ts:331` compares delivery status with `'pending'` in the page layer.
- `weapp/miniprogram/pages/platform/merchants/detail.ts:36` switches merchant statuses such as `'active'`, `'approved'`, `'suspended'`, `'pending'`, and `'rejected'` in the page layer.
- `weapp/miniprogram/pages/platform/merchants/index.ts:27` duplicates merchant status label/theme mapping.
- `weapp/miniprogram/pages/platform/operators/detail.ts:33` switches operator statuses in the page layer.
- `weapp/miniprogram/pages/platform/riders/index.ts:26` switches rider statuses in the page layer.

**Impact:**

Status label, color, action eligibility, and polling behavior can drift across pages when backend enum semantics evolve. Platform list/detail pages already duplicate mappings, so future fixes can land in one page but not another.

**Acceptance Target:**

- All page/component status rendering goes through shared helpers or domain view builders.
- `npm run gate:business-status-boundary` passes.
- Platform merchant/operator/rider list/detail pages share one status view source per role.

### P2. Large Page And Runtime Files Still Blur Task Ownership

**Severity:** Medium-High

**Evidence:**

- `weapp/miniprogram/pages/merchant/settings/application/index.ts`: 648 lines.
- `weapp/miniprogram/pages/rider/deposit/index.ts`: 647 lines.
- `weapp/miniprogram/pages/merchant/reservations/index.ts`: 647 lines.
- `weapp/miniprogram/pages/merchant/tables/edit/index.ts`: 646 lines.
- `weapp/miniprogram/pages/takeout/cart/index.ts`: 637 lines.
- `weapp/miniprogram/utils/merchant-store-registration-runtime.ts`: 1893 lines.
- `weapp/miniprogram/utils/rider-dashboard-runtime.ts`: 1023 lines.
- `weapp/miniprogram/utils/request.ts`: 961 lines.

**Impact:**

These files are not automatically wrong, but they are strong evidence that page shell, domain workflow, view-model mapping, local validation, async recovery, and feedback orchestration may still be coupled. That raises review cost and makes weak-network/re-entry regressions easier to introduce.

**Acceptance Target:**

- Top high-risk runtime files have documented owners and focused extraction plans.
- New changes do not increase these files unless an ownership note explicitly allows it.
- At least the highest-risk files have task-domain controllers/view builders extracted and covered by focused checks.

### P3. Form Placeholder Drift Remains In Visible Pages

**Severity:** Medium

**Evidence:**

- `weapp/miniprogram/pages/register/rider/index.wxml:108` uses label `联系电话` with placeholder `请输入联系电话`.
- `weapp/miniprogram/pages/register/operator/index.wxml:86` uses label `联系人电话` with placeholder `请输入负责人手机号`.
- `weapp/miniprogram/pages/register/merchant/store/index.wxml:121` uses label `联系电话` with placeholder `请输入商户联系电话`.
- `weapp/miniprogram/pages/user/bind-merchant/index.wxml:33` uses label `邀请码` with placeholder `请输入邀请码`.

**Impact:**

This violates `PAGE_DELIVERY_BASELINE`: labels own the field purpose, and placeholders should only add format, constraint, example, or state-specific guidance. Repeating the label increases text density and makes important validation hints less visible.

**Acceptance Target:**

- Standard form rows avoid `请输入/请选择 + field label` when a visible label exists.
- Placeholders carry format or constraint hints, for example `11位手机号`, `商户提供的6位邀请码`, or are omitted.
- Add or extend a gate so new placeholder drift is caught.

### P4. State-Changing Platform Actions Use Weak Success Feedback

**Severity:** Medium

**Evidence:**

- `weapp/miniprogram/pages/platform/riders/detail.ts:129` submits pause/resume, reloads detail, then shows a success Toast at `:137`.
- `weapp/miniprogram/pages/platform/operators/detail.ts:132` updates operator status, reloads detail, then shows a success Toast at `:136`.
- `weapp/miniprogram/pages/platform/merchants/detail.ts:149` pauses/resumes merchant, reloads detail, then shows a success Toast at `:157`.

**Impact:**

The backend call and reload are real, which is good. The remaining weakness is that the durable result is not clearly promoted as the main feedback channel. If reload is slow, stale, or partially fails, users may rely on a transient Toast instead of seeing an explicit post-action state.

**Acceptance Target:**

- After state-changing actions, the page shows the updated status in-page with a visible timestamp or state strip.
- Toast is either removed or kept as secondary feedback only when the page state has already changed visibly.
- Reload failure after mutation preserves last known state and explains that confirmation is pending or sync failed.

### P5. Success Toasts Are Used For Some Async Or Eventually-Consistent Actions

**Severity:** Medium

**Evidence:**

- `weapp/miniprogram/pages/merchant/printers/index.ts:428` sends a printer test command and immediately shows `测试命令已发送` at `:430`.
- `weapp/miniprogram/pages/user_center/payment-detail/index.ts:209` shows `充值已完成` after rider deposit recharge workflow reports paid.

**Impact:**

The payment example is much safer because it uses workflow status before showing success. The printer example is weaker: the command being sent is not the same as the printer physically succeeding. For async commands, the UI should distinguish "sent", "processing", "completed", "failed", and "unknown".

**Acceptance Target:**

- Async command pages separate accepted/submitted from final success.
- Long-running command results have visible status rows, retry, and detail inspection paths.

### P6. Request Infrastructure Is Strong But Too Centralized

**Severity:** Medium

**Evidence:**

- `weapp/miniprogram/utils/request.ts:310` owns request orchestration, cache, single-flight, loading, auth refresh, envelope parsing, retry, and error conversion.
- `weapp/miniprogram/utils/request.ts:843` owns token refresh lock and fallback login.
- Direct `wx.request` also exists in special utilities such as media upload and logger paths.

**Impact:**

The implementation has good features, but one large module owns many responsibilities. This makes request behavior changes risky and makes it harder to prove login recovery, retry behavior, and direct request exceptions are all consistent.

**Acceptance Target:**

- Keep public API stable but split internals into request transport, auth refresh, response envelope, retry policy, and direct-request exception helpers.
- Add tests for auth refresh single-flight, 401 retry, envelope business error mapping, GET single-flight, and direct-request exception behavior.

### P7. Review And Gate Coverage Is Broad But Not Complete Enough For 100

**Severity:** Medium

**Evidence:**

- `npm run quality:check` contains many targeted checks and gates.
- `gate:non-consumer-ui-patterns` runs with `--changed-only`.
- `gate:frontend-architecture-boundary` currently reports no changed Mini Program page/component files when no diff exists.

**Impact:**

Changed-only gates are useful for preventing new drift, but they do not prove historical pages are already clean. A 100-point target needs either all-scope gates for critical classes or an explicit historical debt ledger with no open high-risk drift.

**Acceptance Target:**

- All high-risk gates run all-scope or have a published debt ledger with zero P0/P1 items.
- Historical drift cleanup is tracked by page group and role.

### P8. Payment Workflow Is A Strength, But Needs End-To-End Scenario Evidence

**Severity:** Medium

**Evidence:**

- `weapp/miniprogram/services/payment-workflow.ts:198` invokes WeChat pay and then polls backend status.
- `weapp/miniprogram/pages/payment/result/index.ts:89` refreshes payment status through `waitForPaymentWorkflowTerminalResult`.
- `npm run gate:payment-workflow-boundary` passes.

**Impact:**

Static boundary gates are good, but a 100 score should require scenario validation for successful pay, cancel, unknown result, delayed callback, re-entry, and detail/list consistency across takeout, reservation, dine-in, rider deposit, claim recovery, and Baofoo verification fee flows.

**Acceptance Target:**

- Add scenario checks or manual evidence scripts for all payment business types.
- Payment result, order detail, reservation detail, ledger, and history surfaces agree after delayed callback and re-entry.

---

## Score Gap Model

| Area | Current | Target |
| --- | ---: | ---: |
| Build and Type Safety | 10 / 10 | 10 / 10 |
| Existing Gate Coverage | 13 / 15 | 15 / 15 |
| Business Status Boundaries | 3 / 10 | 10 / 10 |
| Payment and Money Flow Boundaries | 14 / 15 | 15 / 15 |
| Page State and Recovery | 10 / 15 | 15 / 15 |
| Task-Domain Ownership | 9 / 15 | 15 / 15 |
| UI Copy and Form Baseline | 6 / 10 | 10 / 10 |
| Request/Auth Robustness | 8 / 10 | 10 / 10 |
| End-To-End Evidence | 5 / 10 | 10 / 10 |
| **Total** | **78 / 100** | **100 / 100** |

---

## Improvement Plan To 100

### Phase 1: Make The Quality Gate Green

**Goal:** Move from 78 to about 84 by eliminating the known blocking gate failure.

**Files:**

- Modify: `weapp/miniprogram/utils/status-tag.ts`
- Create or modify: role-specific shared status helpers under `weapp/miniprogram/utils/` or `weapp/miniprogram/services/platform-management.ts`
- Modify: `weapp/miniprogram/pages/orders/tracking/index.ts`
- Modify: `weapp/miniprogram/pages/platform/merchants/detail.ts`
- Modify: `weapp/miniprogram/pages/platform/merchants/index.ts`
- Modify: `weapp/miniprogram/pages/platform/operators/detail.ts`
- Modify: `weapp/miniprogram/pages/platform/operators/index.ts`
- Modify: `weapp/miniprogram/pages/platform/riders/detail.ts`
- Modify: `weapp/miniprogram/pages/platform/riders/index.ts`

**Steps:**

- [ ] Add shared status view helpers for platform merchant, platform operator, platform rider, and delivery tracking poll eligibility.
- [ ] Replace all page-level `switch` and direct status comparisons flagged by `gate:business-status-boundary`.
- [ ] Add focused script tests if the status helpers are not already covered.
- [ ] Run `npm run gate:business-status-boundary`.
- [ ] Run `npm run quality:check`.

**Exit Criteria:**

- `npm run quality:check` passes.
- No platform list/detail page owns duplicated status label/theme logic.

### Phase 2: Close Placeholder And Product Copy Drift

**Goal:** Move from about 84 to about 88 by making the form baseline visibly consistent.

**Files:**

- Modify: `weapp/miniprogram/pages/register/rider/index.wxml`
- Modify: `weapp/miniprogram/pages/register/operator/index.wxml`
- Modify: `weapp/miniprogram/pages/register/merchant/store/index.wxml`
- Modify: `weapp/miniprogram/pages/user/bind-merchant/index.wxml`
- Review and modify additional matches from `rg -n "请输入|请选择" miniprogram/pages miniprogram/components`
- Create or modify: `weapp/scripts/check-placeholder-label-drift.js`
- Modify: `weapp/package.json`

**Steps:**

- [ ] Replace repeated label placeholders with useful constraints or empty placeholders.
- [ ] For phone inputs, prefer `11位手机号`.
- [ ] For invitation codes, prefer a constraint hint such as `商户提供的绑定码` only if that hint is more useful than an empty placeholder.
- [ ] Add a gate that detects `placeholder="请输入{{label}}"`, `placeholder="请输入字段名"`, and common static duplicate patterns where feasible.
- [ ] Add the gate to `quality:check` or `gate:weapp` after confirming false positives are manageable.
- [ ] Run `npm run compile`.
- [ ] Run the new placeholder gate.
- [ ] Run `npm run quality:check`.

**Exit Criteria:**

- No standard form row repeats visible label text in placeholder.
- New placeholder drift is blocked automatically.

### Phase 3: Upgrade State-Changing Action Feedback

**Goal:** Move from about 88 to about 92 by making sensitive state changes visibly durable.

**Files:**

- Modify: `weapp/miniprogram/pages/platform/riders/detail.ts`
- Modify: `weapp/miniprogram/pages/platform/riders/detail.wxml`
- Modify: `weapp/miniprogram/pages/platform/operators/detail.ts`
- Modify: `weapp/miniprogram/pages/platform/operators/detail.wxml`
- Modify: `weapp/miniprogram/pages/platform/merchants/detail.ts`
- Modify: `weapp/miniprogram/pages/platform/merchants/detail.wxml`
- Modify: `weapp/miniprogram/pages/merchant/printers/index.ts`
- Modify: `weapp/miniprogram/pages/merchant/printers/index.wxml`
- Add or extend focused scripts for platform action feedback and printer test status.

**Steps:**

- [ ] For platform pause/resume flows, add `lastActionResult` or equivalent view state that records the submitted action, backend-confirmed status, and sync time.
- [ ] Render a compact state strip near the status area after successful reload.
- [ ] If reload after mutation fails, keep the mutation attempt visible as "状态同步中" and provide retry.
- [ ] For printer test command, rename success copy from final-sounding success to accepted/submitted wording and render a visible command status row if backend exposes job state.
- [ ] Add tests or gates asserting these flows do not rely on Toast as the only durable result.
- [ ] Run focused checks, `npm run compile`, and `npm run quality:check`.

**Exit Criteria:**

- User can see the post-action state without relying on Toast memory.
- Weak network after mutation has a visible recovery path.

### Phase 4: Reduce High-Risk Ownership Debt

**Goal:** Move from about 92 to about 96 by extracting the worst ownership hotspots.

**Files:**

- Modify or split: `weapp/miniprogram/utils/merchant-store-registration-runtime.ts`
- Modify or split: `weapp/miniprogram/utils/rider-dashboard-runtime.ts`
- Modify or split: `weapp/miniprogram/utils/request.ts`
- Modify: associated page scripts and focused scripts under `weapp/scripts/`
- Update: `weapp/docs/architecture-ownership/*.md` where ownership changes.

**Steps:**

- [ ] For `merchant-store-registration-runtime.ts`, extract OCR/document workflow, location/region workflow, image persistence workflow, and submission workflow into focused modules.
- [ ] For `rider-dashboard-runtime.ts`, extract live location/realtime lifecycle, delivery action controller, and dashboard refresh model into focused modules.
- [ ] For `request.ts`, keep the exported request API stable while splitting auth refresh, response envelope parsing, retry, and transport internals.
- [ ] Add tests for extracted pure view builders and workflow controllers.
- [ ] Run focused checks after each extraction.
- [ ] Run `npm run compile`, `npm run lint:all`, and `npm run quality:check`.

**Exit Criteria:**

- No extraction changes user-visible behavior.
- High-risk runtime files shrink or have documented ownership boundaries that stop future uncontrolled growth.
- Existing focused gates still pass.

### Phase 5: Add Scenario Evidence For Money And Async Flows

**Goal:** Move from about 96 to 100 by adding evidence for the paths that static gates cannot fully prove.

**Files:**

- Add or extend scripts under `weapp/scripts/`.
- Add scenario notes under `artifacts/` or the relevant domain folder when manual sandbox evidence is required.
- Touch payment-related pages only if scenario checks expose a gap.

**Scenario Matrix:**

- Takeout order payment: success, cancel, unknown result, delayed callback, re-entry.
- Reservation payment: success, cancel, unknown result, detail/list consistency.
- Dine-in checkout: paid session close success, paid but session close delayed, re-entry.
- Rider deposit recharge: success, cancel, pending confirmation, payment detail recovery.
- Claim recovery payment: success, pending confirmation, claim detail recovery.
- Baofoo account verification fee: paid, pending confirmation, onboarding long-wait state.
- Refund/cancel flows: refund processing, terminal success, terminal failure, profit-sharing return visibility.
- Withdrawal flows: create, pending, success, fail, role-specific list/detail consistency.

**Steps:**

- [ ] Define the minimal fixture or mock strategy for each scenario.
- [ ] Add script checks where static source checks are enough.
- [ ] Add sandbox/manual evidence notes where backend callbacks or provider states are required.
- [ ] Confirm each scenario has an owner page, workflow owner, visible state, and recovery path.
- [ ] Run all focused money-flow checks.
- [ ] Run `npm run quality:check`.

**Exit Criteria:**

- All money and async paths have either automated checks or recorded sandbox/manual evidence.
- No path treats `requestPayment` success, command accepted, or submitted as final business success without backend truth.
- Delayed callback and re-entry behavior are documented and validated.

---

## Final 100-Point Acceptance Checklist

- [ ] `npm run compile` passes.
- [ ] `npm run lint:all` passes.
- [ ] `npm run gate:weapp` passes.
- [ ] `npm run quality:check` passes.
- [ ] Business status semantics are centralized in shared helpers or domain view builders.
- [ ] Platform merchant/operator/rider list/detail status behavior is consistent.
- [ ] No visible form uses placeholder-as-label drift in standard labeled fields.
- [ ] High-risk state-changing actions have in-page durable feedback and weak-network recovery.
- [ ] Payment, refund, withdrawal, and async command paths distinguish submitted, processing, terminal success, terminal failure, and unknown.
- [ ] High-risk runtime files have clear task-domain ownership and focused checks.
- [ ] Scenario evidence exists for money, login recovery, duplicate tap, delayed callback, and page re-entry.

---

## Recommended Execution Order

1. Phase 1 first, because the currently failing quality gate is the hard blocker.
2. Phase 2 next, because copy drift is broad but low-risk and improves user-facing polish quickly.
3. Phase 3 before large refactors, because it closes concrete state-recovery risk.
4. Phase 4 in small slices, one runtime owner at a time, with focused checks after each slice.
5. Phase 5 last, because scenario evidence is most valuable once the static boundaries are green.
