# Rider Baofu Withdrawal Balance Unopened Account Fix Plan

**Risk class:** G3. This touches Baofoo/Baofu withdrawal and settlement-account readiness, so it affects funds availability, provider calls, and user-visible financial state.

**Incident signal:** On 2026-06-16 15:29:04 +08:00, `GET /v1/rider/income/baofu-withdrawal/balance` returned 502 for `user_id=868` because `baofu_account_bindings` had no `owner_type='rider', owner_id=4` row. The rider settlement-account status endpoint returned 200 and correctly represented the user as `profile_pending`.

**Design goal:** A missing or not-ready Baofu settlement account is a business empty state for balance reads, not a provider failure. Balance reads must return a stable 200 unavailable state without calling Baofoo, while withdrawal creation must remain blocked and auditable.

## Plan Review

The proposed fix is viable with two constraints:

- Enforce the invariant at the logic service boundary so all balance readers get the same no-provider-call behavior.
- Preserve create-withdrawal blocking semantics. The fix must not allow withdrawal orders to be created without an active binding, contract number, and fee member ID.

No SQL, route, or Swagger source changes are required if the existing balance response fields are reused.

## Task 1: Logic Empty-State Result

**Files:**
- Modify: `locallife/logic/baofu_withdraw_service.go`
- Test: `locallife/logic/baofu_withdraw_service_test.go`

**TDD steps:**

- [x] Add a failing test: `TestBaofuWithdrawServiceQueryBalanceReturnsUnavailableWhenBindingMissing`
  - `GetBaofuAccountBindingByOwner` returns `db.ErrRecordNotFound`.
  - Assert `QueryBalance` returns nil error.
  - Assert `AccountStatus == "profile_pending"`, `CanWithdraw == false`, amounts are zero.
  - Assert the fake Baofoo client received no balance request.

- [x] Add a failing test: `TestBaofuWithdrawServiceQueryBalanceReturnsUnavailableWhenBindingNotActive`
  - Binding exists with `open_state='processing'`.
  - Assert nil error, `AccountStatus == "not_ready"`, `CanWithdraw == false`.
  - Assert provider was not called.

- [x] Implement the minimal logic:
  - Extend `BaofuBalanceQueryResult` with `AccountStatus`, `StatusDesc`, `DisabledReason`, `CanWithdraw`.
  - Add a small helper for unavailable balance results.
  - In `QueryBalance`, handle `db.ErrRecordNotFound` and non-ready binding before provider calls.
  - Leave unexpected DB errors and provider errors as errors.

- [x] Run focused logic tests:
  - `cd locallife && go test ./logic -run 'TestBaofuWithdrawServiceQueryBalance'`

**Review checklist after task:**
- [x] Missing binding does not call provider.
- [x] Non-active binding does not call provider.
- [x] Active binding behavior remains unchanged.
- [x] No internal SQL or provider detail is converted into user-facing text.

**Task 1 review result:** Accepted. Focused logic tests passed with `go test ./logic -run 'TestBaofuWithdrawServiceQueryBalance'`. The implementation also covers active bindings that are missing contract number or fee member ID, so balance reads cannot accidentally query Baofoo with incomplete local account state.

## Task 2: API Balance Contract

**Files:**
- Modify: `locallife/api/baofu_withdrawal.go`
- Test: `locallife/api/baofu_withdrawal_contract_test.go`

**TDD steps:**

- [x] Add a failing test: `TestRiderBaofuWithdrawalBalanceReturnsUnopenedStateWhenBindingMissing`
  - Resolve rider from token.
  - `GetBaofuAccountBindingByOwner` returns `db.ErrRecordNotFound`.
  - Assert HTTP 200.
  - Assert `account_status == "profile_pending"`.
  - Assert all amount fields are zero, `can_withdraw == false`, disabled reason guides account opening.
  - Assert fake provider balance request count is zero.

- [x] Implement the minimal API mapping:
  - Build response from `BaofuBalanceQueryResult.AccountStatus`, `StatusDesc`, `DisabledReason`, and `CanWithdraw`.
  - Keep existing active response fields unchanged.
  - Continue to use `respondBaofuWithdrawalError` only for actual errors.

- [x] Run focused API tests:
  - `cd locallife && go test ./api -run 'TestRiderBaofuWithdrawalBalanceReturnsUnopenedStateWhenBindingMissing|TestBaofuWithdrawalBalanceRoutesUseServerResolvedOwnerScope'`

**Review checklist after task:**
- [x] API empty state follows `API_CONTRACT_STANDARDS.md`: 200 plus status field.
- [x] Existing active balance route still returns real provider amounts.
- [x] Provider failures still map to 502 with stable public text.

**Task 2 review result:** Accepted. Focused API tests passed with `go test ./api -run 'TestRiderBaofuWithdrawalBalanceReturnsUnopenedStateWhenBindingMissing|TestBaofuWithdrawalBalanceRoutesUseServerResolvedOwnerScope'`. The API now reuses existing response fields and keeps active-account behavior compatible.

## Task 3: Create-Withdrawal Guard Regression And Frontend Compatibility Check

**Files:**
- Test: `locallife/api/baofu_withdrawal_contract_test.go`
- Inspect only: `weapp/miniprogram/pages/rider/_main_shared/services/baofu-withdrawal-workflow.ts`

**TDD steps:**

- [x] Add or confirm a failing API regression before implementation if missing: create withdrawal with missing binding must return 409 and must not create an order.
- [x] Run it red before relying on it if newly added.
- [x] Keep or adjust implementation only if the new balance-empty-state change weakens create blocking.
- [x] Inspect frontend balance view builder:
  - It must already honor `can_withdraw=false` and `disabled_reason`.
  - No frontend contract change is required for the backend 200 empty state.

- [x] Run focused create guard test:
  - `cd locallife && go test ./api -run 'TestCreate.*BaofuWithdrawal.*Missing.*Binding|TestCreateBaofuWithdrawalRejectsMissingFeeMemberBeforeProviderCall'`

**Review checklist after task:**
- [x] GET balance can return 200 empty state.
- [x] POST withdraw still returns conflict for missing/not-ready account.
- [x] No withdrawal order is persisted for missing/not-ready account.
- [x] Frontend remains compatible with existing response fields.

**Task 3 review result:** Accepted. The new create-withdrawal missing-binding regression first failed with a 502, then passed after `readyBinding` mapped `db.ErrRecordNotFound` to `ErrBaofuWithdrawAccountNotReady`. Focused create guard tests passed with `go test ./api -run 'TestCreateBaofuWithdrawalRejectsMissingBindingBeforeProviderCall|TestCreateBaofuWithdrawalRejectsMissingFeeMemberBeforeProviderCall'`. Existing frontend code already honors `can_withdraw=false` and `disabled_reason`, so no Mini Program change was needed.

## Final Verification

Run from `locallife/`:

- `go test ./logic -run 'TestBaofuWithdrawServiceQueryBalance|TestBaofuWithdrawServiceCreateWithdrawal'`
- `go test ./api -run 'Test.*BaofuWithdrawal.*Balance|TestCreate.*BaofuWithdrawal'`
- `make test-safety`
- `make check-baofu-contract`
- `make check-generated`
- `cd ../weapp && PATH="$HOME/.local/bin:$PATH" npm run check:baofu-withdrawal-workflow`
- `git diff --check`

Regeneration expected:

- `make sqlc`: not required unless SQL/query signatures change.
- `make swagger`: not required unless route annotations or public structs change.
- `make mock`: not required unless mocked store interfaces change.

## Final Review Criteria

- The original production symptom cannot recur for a missing rider Baofu binding: balance read returns 200 empty state, not 502.
- The implementation does not call Baofoo for missing/not-ready local account state.
- Active account balance behavior and response fields remain compatible.
- Create withdrawal remains blocked before order creation when the account is not ready.
- 5xx/502 paths remain reserved for infrastructure/provider failures and keep safe public messages.

## Final Review Result

- [x] Missing rider Baofu binding now returns 200 balance empty state with `account_status=profile_pending`, zero amounts, `can_withdraw=false`, and account-opening guidance.
- [x] Missing/not-ready/incomplete local account state does not call Baofoo balance query.
- [x] Active binding balance response still uses provider amounts and keeps the existing response fields.
- [x] Create withdrawal with a missing binding returns 409 and does not create a withdrawal order or call Baofoo.
- [x] Provider balance failures still return 502 with stable public text.
- [x] Existing Mini Program balance view remains compatible because it already consumes `can_withdraw`, `disabled_reason`, and `status_desc`.

Final validation run:

- `go test ./logic -run 'TestBaofuWithdrawServiceQueryBalance|TestBaofuWithdrawServiceCreateWithdrawal'`
- `go test ./api -run 'Test.*BaofuWithdrawal.*Balance|TestCreate.*BaofuWithdrawal'`
- `make test-safety`
- `make check-baofu-contract`
- `make check-generated`
- `cd ../weapp && PATH="$HOME/.local/bin:$PATH" npm run check:baofu-withdrawal-workflow`
- `git diff --check`

Regeneration review:

- `make sqlc`: not required; SQL/query signatures were not changed.
- `make swagger`: not required; routes and Swagger annotations were not changed.
- `make mock`: not required; mock-backed store interfaces were not changed.
- `make check-generated`: passed, confirming generated artifacts remain in sync.

Frontend compatibility review:

- Existing Mini Program Baofoo withdrawal workflow consumes `can_withdraw`, `disabled_reason`, and `status_desc` and does not require a frontend code change for the new 200 empty-state response.
