# Baofu Merchant Opening Defaults Fix Plan - 2026-05-28

## Background

On 2026-05-28, merchant `owner_id=11` failed Baofoo BaoCaiTong account opening after submitting `/v1/merchant/settlement-account`.

Observed production evidence:

- `2026-05-28 14:02:41`: `T-1001-013-01` returned local upstream code `BF0020`; LocalLife mapped it to `BAOFU_MANUAL_REVIEW` and showed `支付通道异常，请联系平台处理`.
- `2026-05-28 14:12:28`: a later submission for the same merchant failed as `BF00061 企业法人四要素验证失败`.
- The Baofoo profile persisted for the merchant used a legal name that differed from the reviewed merchant application.
- The reviewed merchant application had the corrected business-license identity data. Concrete production identifiers are intentionally omitted from this plan.

The actual business issue is not only a provider error code. The account-opening form allowed manual drift from already-reviewed merchant identity data:

- Extra character in the store subject name, e.g. a typed food-category character that is absent from the reviewed license subject.
- Punctuation drift: full-width parentheses `（个体工商户）` instead of half-width `(个体工商户)`.

Baofoo identity/account verification is sensitive to exact subject identity fields. Merchant opening should reuse LocalLife's reviewed merchant application and credential data instead of asking business staff to retype authoritative fields.

## Current Production Path

Primary path:

1. Mini Program merchant page `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.ts` loads `GET /v1/merchant/settlement-account`.
2. The page uses `response.profile_defaults` to prefill `legal_name`, `business_license_number`, `legal_person_name`, and bank draft fields.
3. Backend handler `getMerchantBaofuSettlementAccount` builds scope from the current merchant and calls `loadBaofuSettlementAccount`.
4. `loadBaofuSettlementAccount` calls `applyBaofuSettlementAccountProfileDefaults` only when status is `profile_pending`.
5. Default loading dispatches by `owner_type`.
6. Merchant default loading currently reaches `loadMerchantBaofuSettlementAccountProfileDefaults`, which is an empty stub and always returns no defaults.
7. On submit, backend decodes the user-entered profile and only merges defaults when fields are missing. It does not override mistyped merchant identity fields, so a fully-filled but wrong manual submission can still reach Baofoo.

Relevant code:

- `locallife/api/baofu_settlement_account.go`
- `locallife/api/baofu_settlement_account_read.go`
- `locallife/api/baofu_settlement_account_profile_defaults.go`
- `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.ts`
- `weapp/miniprogram/services/baofu-account-profile-form.ts`

## Root Cause

Merchant profile defaults were designed but not wired:

- Rider defaults come from rider identity.
- Operator defaults come from operator profile/application.
- Merchant defaults are a stub:

```go
func (server *Server) loadMerchantBaofuSettlementAccountProfileDefaults(ctx context.Context, merchantID int64) (baofuSettlementAccountProfileDefaultsWithSecrets, bool, error) {
	_ = ctx
	_ = merchantID
	return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, nil
}
```

There is also a precedence issue:

- Existing Baofoo opening profile is merged before external authoritative defaults.
- `mergeFrom` fills only blank fields.
- If an existing Baofoo profile already contains a bad merchant subject name, merchant application defaults will not correct it unless the implementation deliberately gives authoritative merchant identity fields higher precedence.

## Desired Invariant

For merchant Baofoo business account opening, LocalLife must treat reviewed merchant onboarding data as the authoritative source for merchant identity fields:

- `legal_name` / Baofoo `customerName`
- `business_license_number` / Baofoo `certificateNo`
- `legal_person_name` / Baofoo `corporateName`
- `legal_person_id_number` when available

Manual input should remain available for fields that are not already authoritative in LocalLife, especially bank account details and contact email/mobile when not stored. Merchant identity fields should be corrected server-side from the reviewed application even if the Mini Program submits non-empty manual values.

If a previous Baofoo opening profile conflicts with reviewed merchant identity data, the next default load and partial submit should prefer the reviewed merchant data for merchant identity fields.

## Contract And Security Notes

Risk class: `G3`.

Reasons:

- Baofoo account opening touches payment account eligibility, identity data, bank data, and provider contract boundaries.
- The fix changes which fields are sent to `T-1001-013-01`.
- Sensitive data must not be exposed in Mini Program responses except through existing masks/flags.

Provider contract shape is not changing. We are changing LocalLife's source of truth for already-supported request fields. No Baofoo DTO field names, endpoint names, or callback parsers should change in this fix.

Sensitive-field policy:

- Do not add full ID numbers, bank cards, or mobiles to API responses.
- Returning masks and `has_*` flags is allowed and already established.
- Server-side merge can use stored sensitive values when needed, but logs must stay masked.

## Implementation Plan

### 1. Add backend regression tests first

Target test file: `locallife/api/baofu_settlement_account_test.go`.

Add tests that prove the missing behavior:

- Merchant `GET /v1/merchant/settlement-account` returns `profile_defaults` from an approved merchant application when no Baofoo profile exists.
- Merchant default loading prefers merchant application identity over an existing Baofoo profile with a mistyped legal name.
- Merchant submit merges authoritative merchant identity defaults server-side before opening, including the case where the Mini Program submits non-empty but mistyped identity fields.

The first two tests should fail before implementation because merchant defaults are currently empty.

### 2. Implement merchant defaults from approved merchant/application data

Owner: backend API response/default assembly layer.

Implementation target: `locallife/api/baofu_settlement_account_profile_defaults.go`.

Use existing store methods where possible:

- Load merchant by ID.
- Use merchant owner user ID or merchant application relation to find the reviewed merchant application.
- Prefer approved/submitted reviewed application data over free-form merchant display fields.
- Fall back to `merchant.application_data` if the application row is unavailable but merchant has approval snapshot data.

Default mapping:

- `LegalName`: application `merchant_name`; prefer business license OCR enterprise name only if product policy says OCR is more authoritative than reviewed application text. For this fix, use reviewed application field as the stable source because it is what approval stored.
- `BusinessLicenseNumber`: application `business_license_number`.
- `LegalPersonName`: application `legal_person_name`.
- `LegalPersonIDNumber`: application `legal_person_id_number`; expose only mask/flag in response.
- `CardUserName`: application `legal_person_name`.
- `Source`: `merchant_application`.

Keep bank fields unset unless they already come from a Baofoo profile. Merchant application does not own settlement bank data.

### 3. Fix precedence for merchant identity defaults

The existing merge order should not allow a previous bad Baofoo profile to keep overriding reviewed merchant identity.

Expected behavior:

- Existing Baofoo profile can keep bank, email, contact, and sensitive bank/account data.
- Merchant application should overwrite merchant identity response defaults and server-side default secrets for merchant business identity fields.

Keep this scoped to merchant business-account defaults. Do not change rider/operator/platform behavior.

### 4. Review Mini Program behavior

Expected frontend effect after backend fix:

- The existing merchant submit page should receive `profile_defaults` and prefill the form automatically.
- No page code change should be required for basic reuse because `applyAccount` already calls `buildBaofuEnterpriseFormFromDefaults`.

Potential follow-up, only if needed after backend verification:

- Make merchant identity fields read-only or visually sourced from approved application data to prevent retyping drift.
- This may require WXML/UI changes and should be validated under Mini Program rules. It is not required for the first backend invariant if the server overrides identity fields.

### 5. Validation

Run from `locallife/`:

```bash
go test ./api -run 'TestBaofuSettlementAccount.*Merchant.*Defaults|TestBaofuSettlementAccount.*Merchant.*Partial' -count=1
go test ./api -run 'TestBaofuSettlementAccount' -count=1
```

If SQL query sources change, run:

```bash
make sqlc
make check-generated
```

Expected no regeneration if the fix uses existing queries.

Frontend validation is only required if Mini Program files change:

```bash
cd ../weapp
npm run compile
```

## Non-Goals

- Do not change Baofoo provider DTO field names or endpoint handling.
- Do not expose full sensitive identity/bank fields to the Mini Program.
- Do not solve all historical failed flows automatically.
- Do not change provider error code classification for `BF0020` in this fix.

## Residual Risks To Call Out

- Historical flows already failed with wrong submitted identity. The fix prevents recurrence on new/continued submits but does not automatically reconcile Baofoo-side historical records.
- If the reviewed merchant application itself contains manual text drift from OCR/license truth, this fix will reuse the reviewed field. A later hardening pass can compare reviewed fields against OCR `enterprise_name` after product policy decides which source wins.
- If Baofoo requires exact punctuation from the certificate image, backend should eventually preserve certificate-source text separately and avoid UI-normalized display text for provider requests.
