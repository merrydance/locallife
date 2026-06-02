# Baofoo Merchant Personal Opening Implementation Plan

Updated: 2026-06-02

## Background

Baofoo/BaoCaiTong work is `G3`: it touches account opening, payment readiness, external provider contracts, identity and bank-card data, WeChat merchant report, and APPLET authorization.

Current behavior:

- Merchant settlement account API fixes `owner_type=merchant` and `account_type=business`.
- `baofuOpeningAccountType()` maps merchant/platform to `business`, and rider/operator to `personal`.
- Merchant business opening already proceeds through account open, WeChat `merchant_report`, `merchant_report_query`, and `bind_sub_config(APPLET)`.
- Merchant report currently assumes business-license based material (`NATIONAL_LEGAL_MERGE` + `business_license`).

Target behavior:

- A merchant can choose the Baofoo account opening mode:
  - `business`: existing flow stays intact. It can use public or private settlement cards according to the existing business/self-employed rules. Limited companies and other non-individual-business subjects remain forced to public/company account handling by the caller/product data contract.
  - `personal`: use Baofoo personal four-factor opening with personal name, ID card, personal bank card, and reserved bank mobile. Personal opening must only use an individual card.
- Merchant report remains mandatory for every merchant, regardless of Baofoo account mode.
- APPLET authorization binding remains mandatory for every reported merchant `subMchId`.
- Business/address/contact/report material remains available for merchant report even when the Baofoo account is opened as personal.

Canonical flow after the change:

```text
merchant chooses account_opening_mode
  -> Baofoo account open as business or personal
  -> persist contractNo / sharing_mer_id
  -> merchant_report with bctMerId = sharing_mer_id
  -> merchant_report_query until subMchId is available
  -> bind_sub_config(authType=APPLET, authContent=<LocalLife mini appid>)
  -> payment readiness only after account + report + APPLET binding are usable
```

Important identifier boundary:

- `contractNo` / `sharing_mer_id`: BaoCaiTong second-level account and share receiver.
- `subMchId`: WeChat channel merchant identity used by unified order.
- APPLET binding gates WeChat mini-program payment, not profit-share receiver identity.

## Design Notes

- Do not let clients submit raw `account_type`; keep service-owned account type selection.
- Add a narrow merchant-only request field such as `account_opening_mode` or `opening_mode`.
- Preserve current default for existing clients: missing merchant mode means `business`.
- Store the chosen mode through existing `account_type` columns (`business` or `personal`) in `baofu_account_opening_profiles`, `baofu_account_opening_flows`, and `baofu_account_bindings`.
- The account opening profile must support personal Baofoo fields and separate merchant-report fields. If a single DTO is reused, validation must use both `owner_type` and `account_type`, not only owner type.
- Merchant report request builder must use the profile account type to choose report certificate mapping:
  - business account: current `NATIONAL_LEGAL_MERGE` + business license number.
  - personal account: `IDENTITY_CARD` + personal ID card number.
- The report still needs merchant name, short name, service phone, category, address, and bankcard info.
- Recovery paths must continue from persisted `flow.account_type`; they must not infer account type again from the current request body.

## Files And Responsibilities

- `locallife/logic/baofu_account_onboarding_service.go`: input DTOs and service entrypoint.
- `locallife/logic/baofu_account_onboarding_helpers.go`: account-type resolution and missing-field rules.
- `locallife/logic/baofu_account_onboarding_profile.go`: profile persistence and certificate-type selection.
- `locallife/logic/baofu_account_onboarding_open.go`: Baofoo account open request construction.
- `locallife/logic/baofu_account_merchant_report_service.go`: merchant report input construction from account-opening profile and merchant data.
- `locallife/logic/baofu_merchant_report_service.go`: provider report and APPLET binding orchestration.
- `locallife/api/baofu_settlement_account*.go`: request decoding, role field allowlist, merchant scope, response shaping, default profile merge.
- `locallife/baofu/account/contracts/**`: provider personal/business account DTO validation, only if current adapter cannot encode needed behavior.
- `locallife/baofu/merchantreport/contracts/**`: report certificate enum/validation, likely already supports `IDENTITY_CARD`.
- `locallife/db/query/**`: only if existing SQL cannot carry the chosen account type. Current schema already allows `personal` and `business`.
- `.github/standards/domains/baofu-payment/API_CONTRACT_COVERAGE_AUDIT.md`: update if implementation changes the documented operating scope.

## Task Checklist

### Task 1: Persistent Plan And Baseline Review

Status: completed

- [x] Re-read LocalLife routing, backend instructions, Baofoo domain README, and payment review prompt.
- [x] Review current merchant account opening, merchant report, and APPLET binding code paths.
- [x] Save this implementation plan.
- [x] Record baseline working tree status.

Validation:

```bash
cd /home/sam/locallife
git status --short
```

### Task 2: Add Failing Tests For Merchant Personal Mode

Status: completed

Test targets:

- [x] `locallife/logic/baofu_account_onboarding_service_test.go`
  - merchant personal opening stores profile/flow/binding with `account_type=personal`.
  - merchant personal opening uses Baofoo personal four-factor request fields.
  - merchant personal active result still enters `merchant_report_processing`, not `ready`.
  - personal merchant report uses `IDENTITY_CARD` and the personal certificate number while still using merchant/address/contact/report fields.
- [x] `locallife/api/baofu_settlement_account_test.go`
  - merchant can submit `account_opening_mode=personal`.
  - omitted mode defaults to `business` is covered by existing business tests.

Expected first run:

```bash
cd /home/sam/locallife/locallife
go test ./logic -run 'TestBaofuAccountOnboardingService.*Merchant.*Personal|TestBaofuAccountMerchantReportService.*Personal' -count=1
go test ./api -run 'TestBaofuSettlementAccountMerchant.*Personal|TestBaofuSettlementAccountMerchant.*OpeningMode' -count=1
```

Expected: tests fail because mode fields and personal merchant behavior are not implemented yet.

### Task 3: Implement Account Mode Plumbing

Status: completed

Implementation scope:

- Add service input field for requested account type/mode.
- Resolve merchant account type from explicit mode, defaulting to `business`.
- Keep platform `business`; rider/operator `personal`.
- Use persisted flow/profile account type in recovery and result mapping.
- Adjust missing-field logic so merchant personal opening requires personal identity/bank fields for Baofoo opening.
- Keep current business missing-field behavior unchanged.

Validation:

```bash
cd /home/sam/locallife/locallife
go test ./logic -run 'TestBaofuAccountOnboardingService.*Merchant.*Personal|TestBaofuAccountOnboardingServiceStart_Business' -count=1
```

### Task 4: Implement API Request/Response Support

Status: completed

Implementation scope:

- Add merchant request field such as `account_opening_mode`.
- Reject this field for non-merchant scopes unless a later requirement explicitly allows it.
- Keep rejecting raw `account_type`.
- Return selected `account_type` from persisted result, not only static scope, so read responses reflect personal merchant accounts.
- Preserve current behavior for existing merchant requests with no mode.

Validation:

```bash
cd /home/sam/locallife/locallife
go test ./api -run 'TestBaofuSettlementAccountMerchant.*Personal|TestBaofuSettlementAccountMerchant.*OpeningMode|TestBaofuSettlementAccountMerchantOwnerCanReadSafeSummary' -count=1
```

### Task 5: Implement Personal Merchant Report Mapping

Status: completed

Implementation scope:

- In merchant report input construction, branch on `profile.account_type`.
- Business mode: keep existing `NATIONAL_LEGAL_MERGE` + business license behavior.
- Personal mode: use `IDENTITY_CARD` + decrypted personal certificate number.
- Preserve merchant name, short name, service phone, channel config, business category, address, and bankcard info.
- Ensure APPLET binding still follows report success for both account modes.

Validation:

```bash
cd /home/sam/locallife/locallife
go test ./logic -run 'TestBaofuAccountMerchantReportService.*Personal|TestBaofuAccountOnboardingService.*MerchantReport|TestBaofuAccountOnboardingService.*Applet' -count=1
```

### Task 6: Contract Drift And Focused Regression

Status: completed

Validation:

```bash
cd /home/sam/locallife/locallife
go test ./baofu/account/contracts ./baofu/merchantreport/contracts -count=1
go test ./logic -run 'TestBaofuAccountOnboardingService|TestBaofuAccountMerchantReportService|TestBaofuMerchantReportService' -count=1
go test ./api -run 'TestBaofuSettlementAccount' -count=1
make check-baofu-contract
```

Regeneration expectation:

- `make sqlc`: not required by source changes; `make check-generated` ran it and found no sqlc drift.
- `make swagger`: required because public API request fields changed; regenerated.
- `make mock`: not required by source changes; `make check-generated` ran it and found no mock drift.

### Task 7: Documentation Update And Final Handoff

Status: completed

- Update this plan after each completed task.
- Update Baofoo domain docs only if the implemented operating scope differs from current domain guidance. Current Baofoo domain docs already state account opening, merchant report, report query, APPLET binding, and payment-readiness boundaries; no separate domain-doc change was needed for this narrow API behavior.
- Final handoff must state:
  - risk class `G3`;
  - code paths changed;
  - validation commands run and results;
  - regeneration commands required/run/not required;
  - residual risk, especially real Baofoo/WeChat report and APPLET binding evidence not covered by local tests.

### Task 8: Mini Program Merchant Entry

Status: completed

- [x] Add Mini Program request support for merchant `account_opening_mode`.
- [x] Add merchant personal Baofoo payload mapping from name, ID card, personal bank card, and reserved mobile.
- [x] Add merchant submit-page opening mode selection; default remains business-license opening.
- [x] Keep business-license opening on the existing form and bank-account selector path.
- [x] Add personal-opening form branch and submit action.
- [x] Extend the Baofoo personal profile guard script so merchant personal payload and mode submission stay covered.

Validation:

```bash
cd /home/sam/locallife/weapp
npm run check:baofu-personal-profile-form
npm run compile
npm run lint
```

### Task 9: Full Chain Status Recovery And Long-Wait Root Cause

Status: completed

Background:

- Baofoo account query, merchant report query, and APPLET `bind_sub_config` can return the next state nearly in real time.
- The Mini Program submit workflow can wait for final payment readiness (`ready/failed/profile_pending/voided`), but the wait panel must not look static while backend has already moved from account opening into `merchant_report_processing` or `applet_auth_pending`.
- Backend settlement-account GET recovered account-opening `opening_processing`, but after the active binding moved to merchant report/auth states the status read path only rendered the persisted merchant report state. If the scheduler had not run yet, the user-facing polling loop could keep waiting for scheduler recovery even though the provider would answer quickly.

Root cause:

- Frontend root cause: polling progress only exposed elapsed/remaining time. It did not carry the latest account/status snapshot to the wait UI, so intermediate backend state changes were invisible during long waits.
- Backend root cause: `loadBaofuSettlementAccount()` opportunistically recovered Baofoo account opening but did not opportunistically recover merchant report/query or APPLET auth binding on GET.

Implementation:

- Mini Program submit/payment polling still waits for the final backend terminal state.
- Each polling GET now publishes the latest account/status snapshot to the wait UI.
- Submit and status wait panels rebuild their title, description, theme, and action from the latest backend status, so users can see transitions such as `opening_processing` -> `merchant_report_processing` -> `applet_auth_pending` -> `ready`.
- Backend settlement-account GET now attempts merchant report/auth recovery for merchant flows in `merchant_report_processing` or `applet_auth_pending` when the Baofoo merchant-report client and config are available.
- Read-side recovery reuses `BaofuAccountMerchantReportService.RecoverMerchantReportFlow`, so the provider contract and persistence owner stay in the existing logic layer.
- Read-side recovery failure is logged and downgraded to the current processing status rather than returning a user-visible 500 from status polling.

Validation:

```bash
cd /home/sam/locallife/weapp
npm run check:baofu-onboarding-long-wait
npm run check:baofu-personal-profile-form
npm run compile
npm run lint

cd /home/sam/locallife/locallife
PATH=/usr/local/go/bin:$HOME/go/bin:$PATH go test ./api -run 'TestBaofuSettlementAccountMerchantGetRecoversAppletAuthPendingFromBaofoo|TestBaofuSettlementAccountMerchantGetKeepsProcessingWhenReadRecoveryFails' -count=1
PATH=/usr/local/go/bin:$HOME/go/bin:$PATH go test ./api -run 'TestBaofuSettlementAccount' -count=1
PATH=/usr/local/go/bin:$HOME/go/bin:$PATH go test ./logic -run 'TestBaofuAccountMerchantReportService|TestBaofuAccountOnboardingService|TestBaofuPaymentReadiness' -count=1
PATH=/usr/local/go/bin:$HOME/go/bin:$PATH go test ./baofu/account/contracts ./baofu/merchantreport/contracts -count=1
PATH=/usr/local/go/bin:$HOME/go/bin:$PATH make check-baofu-contract
PATH=/usr/local/go/bin:$HOME/go/bin:$PATH make check-generated
```

## Progress Log

- 2026-06-02: Created plan after static review confirmed current merchant flow is fixed to business account opening while merchant report and APPLET binding are already owner-type gated.
- 2026-06-02: Task 1 completed. Baseline `git status --short` showed only this new artifact before test edits.
- 2026-06-02: Task 2 completed. Red tests added. `/usr/local/go/bin/go test ./logic -run 'TestBaofuAccountOnboardingServiceStart_MerchantPersonalModeUsesPersonalFourFactorAndContinuesReport|TestBaofuAccountOnboardingServiceApplyAccountOpenResult_MerchantPersonalReportUsesIdentityCardAndBindsApplet' -count=1` fails at compile time because `BaofuAccountOpeningInput.AccountOpeningMode` does not exist. `/usr/local/go/bin/go test ./api -run 'TestBaofuSettlementAccountMerchantPostPersonalOpeningModeUsesPersonalAccount' -count=1` fails with HTTP 400 before onboarding because personal mode and personal profile aliases are not accepted for merchants yet.
- 2026-06-02: Tasks 3 and 4 completed for the narrow path. Added merchant account opening mode plumbing, default business mode, personal merchant API acceptance, personal four-factor missing-field handling, and actual account type in responses. `/usr/local/go/bin/go test ./api -run 'TestBaofuSettlementAccountMerchantPostPersonalOpeningModeUsesPersonalAccount' -count=1` passes. Logic narrow run now reaches report mapping and fails because the report still uses `NATIONAL_LEGAL_MERGE` instead of `IDENTITY_CARD`.
- 2026-06-02: Task 5 completed for the narrow path. Personal merchant report mapping now uses `IDENTITY_CARD` while business mode keeps the existing license mapping. Narrow tests pass: `/usr/local/go/bin/go test ./logic -run 'TestBaofuAccountOnboardingServiceStart_MerchantPersonalModeUsesPersonalFourFactorAndContinuesReport|TestBaofuAccountOnboardingServiceApplyAccountOpenResult_MerchantPersonalReportUsesIdentityCardAndBindsApplet' -count=1` and `/usr/local/go/bin/go test ./api -run 'TestBaofuSettlementAccountMerchantPostPersonalOpeningModeUsesPersonalAccount' -count=1`.
- 2026-06-02: Task 6 completed. Focused and contract validation passed: `/usr/local/go/bin/go test ./baofu/account/contracts ./baofu/merchantreport/contracts -count=1`; `/usr/local/go/bin/go test ./logic -run 'TestBaofuAccountOnboardingService|TestBaofuAccountMerchantReportService|TestBaofuPaymentReadiness' -count=1`; `/usr/local/go/bin/go test ./api -run 'TestBaofuSettlementAccount' -count=1`; `PATH=/usr/local/go/bin:$PATH make check-baofu-contract`; `PATH=/usr/local/go/bin:$HOME/go/bin:$PATH make swagger`; `PATH=/usr/local/go/bin:$HOME/go/bin:$PATH make check-generated`. A final API rerun after rejecting the non-public `opening_mode` alias also passed: `PATH=/usr/local/go/bin:$HOME/go/bin:$PATH go test ./api -run 'TestBaofuSettlementAccount' -count=1`.
- 2026-06-02: Task 7 completed. Final implementation keeps the public API to `account_opening_mode` only; raw `account_type` remains server-controlled and `opening_mode` is rejected to avoid validation/execution mismatch. Baofoo domain docs did not need a new update because the existing guidance already describes merchant report/query and APPLET binding as mandatory readiness steps.
- 2026-06-02: Task 8 completed. Mini Program merchant submit page now lets merchants choose business-license or personal Baofoo opening. Business mode stays on the existing enterprise + bank selector path. Personal mode submits `account_opening_mode=personal` with personal name, ID card, personal bank card, bank mobile, card holder, and report contact aliases. Validation passed: `npm run check:baofu-personal-profile-form`, `npm run compile`, and `npm run lint`.
- 2026-06-02: Task 9 completed. Root cause found in two places: Mini Program polling did not publish intermediate backend status snapshots to the wait UI, and backend GET did not opportunistically recover report/auth states. Frontend now keeps waiting for the final backend terminal state, while every polling GET updates the wait-panel title/description from the latest status. Backend read-side merchant report/auth recovery was added in `loadBaofuSettlementAccount` using the existing `BaofuAccountMerchantReportService`, with failure downgraded to the current processing state. Validation passed: `npm run check:baofu-onboarding-long-wait`; `npm run check:baofu-personal-profile-form`; `npm run compile`; `npm run lint`; `go test ./api -run 'TestBaofuSettlementAccountMerchantGetRecoversAppletAuthPendingFromBaofoo|TestBaofuSettlementAccountMerchantGetKeepsProcessingWhenReadRecoveryFails' -count=1`; `go test ./api -run 'TestBaofuSettlementAccount' -count=1`; `go test ./logic -run 'TestBaofuAccountMerchantReportService|TestBaofuAccountOnboardingService|TestBaofuPaymentReadiness' -count=1`; `go test ./baofu/account/contracts ./baofu/merchantreport/contracts -count=1`; `make check-baofu-contract`; `make check-generated`.
- 2026-06-02: Review fix completed. Found a real account-mode consistency bug: merchant active flow lookup is keyed by owner, so switching from a business draft to personal opening could reuse the old business `baofu_account_opening_flows.account_type`. Fixed by voiding only same-owner `profile_pending` drafts when the requested account type changes, creating a fresh flow for the selected mode, and rejecting mode changes once a different-type flow is already processing or an active binding exists. Added focused regression tests for draft switch, processing conflict, and active-binding conflict. Narrow validation passed: `PATH=/usr/local/go/bin:$HOME/go/bin:$PATH go test ./logic -run 'TestBaofuAccountOnboardingServiceStart_MerchantOpeningModeChange' -count=1`.
