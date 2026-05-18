# Baofu Payment Domain

This directory is the canonical LocalLife source for Baofoo BaoCaiTong 2.0 payment, account, merchant-report, refund, share, withdrawal, callback, and contract-drift guidance.

Baofoo payment work is `G3` by default. It touches funds, account opening, external callbacks, async recovery, sensitive identity/bank data, and provider contract boundaries.

## Read First

Use this order for any Baofoo implementation, bugfix, review, or prompt-system update:

1. `.github/standards/domains/baofu-payment/CAPABILITY_GROUP_INDEX.md` to choose the payment/account/report capability group and read related endpoints together
2. `.github/standards/domains/baofu-payment/CONTRACT_SOURCE_MATRIX.md`
3. `.github/standards/domains/baofu-payment/BAOCAITONG_FIELD_CONTRACT_MATRIX.md`
4. `.github/standards/domains/baofu-payment/API_CONTRACT_COVERAGE_AUDIT.md`
5. `.github/standards/domains/baofu-payment/CONTRACT_IMPLEMENTATION_MAP.md` when mapping contract rows to code
6. `.github/standards/domains/baofu-payment/SANDBOX_EVIDENCE_LEDGER.md` when claiming C4 or sandbox/production evidence
7. `.github/standards/domains/baofu-payment/RAW_SOURCE_LEDGER.md` when refreshing official pages or checking cleaned-source provenance

Do not start from `artifacts/baofu-payment/` for contract truth. Those files are historical plans, rollout notes, or compatibility pointers unless this README says otherwise.

## Contract Truth Rules

- Baofoo official documents are the only contract truth for request shape, response shape, required fields, conditional-required fields, field types, enum values, status semantics, error codes, endpoint URLs, callback ACK, and retry rules.
- Baofoo support answers can refine current LocalLife operating scope when they do not contradict official docs. If they contradict docs, record the discrepancy as a doc gap before changing production semantics.
- Sandbox results, production logs, Java demo code, smoke scripts, and local plans are evidence only. They can expose compatibility needs or questions, but they cannot create or override contract rules.
- Every production-used Baofoo field must be traceable from an official page row to a local DTO/parser/validator/test or to a documented decision to defer that field.
- Field drift is fixed only when the matrix, code, and executable guard are aligned. Updating prose alone is not enough.

## Current LocalLife Scope

First-version production scope covers:

- BaoCaiTong account open/query/balance/withdraw/query-withdraw.
- BaoCaiTong account open and withdrawal notifications.
- Aggregate WeChat JSAPI unified order, order query, close, pre-share refund, refund query, refund notification.
- Share after pay, share query, share notification.
- Merchant report, merchant-report query, and `bind_sub_config` for APPLET authorization.
- Baofu account-opening flows for merchant, platform, rider, and operator owners.

Deferred or non-current interfaces stay in the matrices for drift detection, but they are not production scope until a new design explicitly promotes them.

Use `CAPABILITY_GROUP_INDEX.md` to separate current production capability groups from deferred or reference Baofoo pages before implementation. Do not enable a deferred page just because a neighboring endpoint in the same official catalog is already implemented.

## Local Code Boundary

- Baofoo provider contracts live under `locallife/baofu/**`.
- Business orchestration lives in `locallife/logic/**`, workers, schedulers, callbacks, and persistence.
- Provider DTOs, upstream error strings, signatures, raw payloads, certificate material, full contract numbers, full secondary merchant IDs, bank cards, identity numbers, and phones must not leak into frontend-visible responses or unsafe logs.
- Provider request builders must encode the current operating mode explicitly. Do not silently add optional or conditional Baofoo fields because they exist in an official page.
- If a contract row changes, update the corresponding DTO/parser/validator tests and `make check-baofu-contract` when the drift is mechanically guardable.

## Validation

For Baofoo contract or payment-domain changes, prefer the smallest relevant package tests first, then run:

```bash
cd locallife
make check-baofu-contract
```

Also run the relevant regeneration commands when source files require them:

- `make sqlc` for SQL/query/schema changes.
- `make mock` for changed mock-backed interfaces.
- `make swagger` for route or Swagger annotation changes.
- `make check-generated` before closing SQL/API contract work that should leave generated artifacts stable.

For C4 claims, update `SANDBOX_EVIDENCE_LEDGER.md` with masked request/response/callback/query evidence. Never claim a real payment, share, refund, withdrawal, or callback path is C4 from parser tests or sandbox shape-only evidence.

## Official Source Policy

Cleaned official Markdown extracts are stored under `official-sources/doc-mandao/` with `INDEX.md` and `SHA256SUMS`. Do not commit raw doc.mandao HTML/TXT, login pages, cookies, credentials, or authentication-token URLs.

Do not commit Baofoo PFX/CER files, private keys, passwords, real merchant credentials, full raw production callbacks, or unredacted commercial agreements. Record those materials in `RAW_SOURCE_LEDGER.md` with hash, location, sensitivity, and extraction status instead.

If official docs are refreshed:

1. Fetch authenticated doc.mandao pages into a temporary controlled directory.
2. Convert article content into cleaned Markdown extracts, excluding navigation chrome, login controls, cookies, credentials, raw HTML, and token URLs.
3. Compare changed official pages against `BAOCAITONG_FIELD_CONTRACT_MATRIX.md`.
4. Update source and field matrices before changing code.
5. Add or update table-driven tests and `make check-baofu-contract` rules for repeated drift patterns.
6. Record sandbox or production evidence separately from contract truth.
