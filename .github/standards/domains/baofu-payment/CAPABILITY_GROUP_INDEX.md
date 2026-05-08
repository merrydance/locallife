# Baofoo BaoCaiTong Capability Group Index

Updated: 2026-05-08
Risk class: G3 - funds/payment/refund/share/withdraw/callback contract boundary.

This file is a capability-level navigation layer for Baofoo BaoCaiTong 2.0 work. It does not define field truth, endpoint truth, enum truth, or validation truth. Use it to choose the correct capability group before reading the source and field matrices.

Authoritative contract detail remains:

1. Baofoo official cleaned extracts under `official-sources/doc-mandao/`
2. `CONTRACT_SOURCE_MATRIX.md`
3. `BAOCAITONG_FIELD_CONTRACT_MATRIX.md`
4. `CONTRACT_IMPLEMENTATION_MAP.md`
5. `SANDBOX_EVIDENCE_LEDGER.md` for C4 or environment evidence only

## Status Labels

| Status | Meaning |
| --- | --- |
| `Current` | First-version LocalLife production scope. Changes require contract matrix, implementation map, tests, and `make check-baofu-contract` alignment. |
| `Current subset` | The capability group is current, but some official pages in the group are reference or deferred. Do not enable the deferred pages by analogy. |
| `Deferred` | Officially documented, retained for drift detection, but not production scope until a new design promotes it. |
| `Reference` | Product background, demo, historical, certificate, or auxiliary material. It may explain transport or samples, but cannot override official field tables. |

## How To Use This Index

1. Pick the capability group first. Baofoo tasks should not start from a single endpoint page in isolation.
2. Read the shared sources plus the group-specific official pages.
3. Check the matching rows in `CONTRACT_SOURCE_MATRIX.md` and `BAOCAITONG_FIELD_CONTRACT_MATRIX.md`.
4. Trace the local boundary in `CONTRACT_IMPLEMENTATION_MAP.md` before editing code.
5. If a production-used field changes, update DTO/parser/validator tests and `locallife/scripts/check_baofu_contract_drift.sh` when guardable.
6. Keep sandbox, demo, and support answers as evidence labels. They do not become contract truth unless recorded in the source matrix.

## Shared Transport And Contract Sources

Read these with any group that crosses the relevant transport boundary.

| Boundary | Status | Official extracts | Local boundary | Drift traps |
| --- | --- | --- | --- | --- |
| Account union-gw | `Current` | `notice.md`, `unionGw.md`, `appendix.md`, `bct-1fjpm4fpns79f.md`, `question.md` | `locallife/baofu/uniongw.go`, `locallife/baofu/account/**`, `locallife/baofu/config.go` | Account APIs use encrypted `header/body` through `union-gw`; account callbacks use encrypted query/form `data_content`; do not confuse this with aggregate `dataContent`. |
| Aggregate and merchant public envelope | `Current` | `bct-1f9qhae400u5l.md`, `bct-1f9qhakcna6te.md`, `bct-1f9qrd02b5bka.md`, `bct-1f9qrdaere1nb.md`, `bct-1f9qrfsj2fcbu.md` | `locallife/baofu/envelope.go`, `locallife/baofu/signing.go`, `locallife/baofu/aggregatepay/**`, `locallife/baofu/merchantreport/**` | Requests carry signed `bizContent`; responses and callbacks carry signed `dataContent`; verify signatures before business parsing; `signSn/ncrptnSn` are S(10) Baofoo certificate indexes, not long X.509 serial numbers. |
| Official source refresh | `Current` | `official-sources/doc-mandao/INDEX.md`, `RAW_SOURCE_LEDGER.md` | `.github/standards/domains/baofu-payment/**` | Store cleaned Markdown only. Do not commit raw HTML/TXT, cookies, credentials, token URLs, PFX/CER files, private keys, or unmasked production payloads. |

## Capability Groups

### 1. Account Opening And Account Lifecycle

Status: `Current subset`

Current LocalLife scope: personal/business account opening, account query, balance query, account opening notification, and Baofu account-opening orchestration for merchant, platform, rider, and operator owners. Other lifecycle pages are retained for drift detection unless explicitly promoted.

| Source role | Official extracts |
| --- | --- |
| Common account transport and appendices | `notice.md`, `unionGw.md`, `appendix.md`, `bct-1fjpm2f1e1ei5.md`, `bct-1fjpm4fpns79f.md`, `question.md` |
| Current open/query/balance/notify | `openAcc.md`, `bct-1gj4ccsdha6d8.md`, `queryAcc.md`, `queryBalace.md`, `openAccNotify.md` |
| Lifecycle reference | `queryCard.md`, `updateCard.md`, `accDetails.md`, `bct-1gahm5f1j79sk.md`, `bct-1gahm7jsqr66u.md`, `bct-1gahm7u2hj1s1.md`, `bct-1gb82plhhdbst.md`, `bct-1gbbtmk773oah.md`, `bct-1gpdo5i47lrsj.md` |

Local boundary:

- Provider contract: `locallife/baofu/account/**`
- Shared account transport/config: `locallife/baofu/uniongw.go`, `locallife/baofu/config.go`, `locallife/baofu/client.go`
- Business orchestration: Baofu account-opening logic, workers, callbacks, and persistence under `locallife/logic/**`, `locallife/api/**`, and `locallife/worker/**`

Drift traps:

- `openAcc` uses `version=4.1.0`; `queryAcc` uses `version=4.0.0`.
- Personal two-factor opening is official and retained for drift audit, but LocalLife first version rejects it at runtime.
- `contractNo` is the BaoCaiTong account identifier and current source for local `sharing_mer_id`; it is not the channel `subMchId`.
- Open-account synchronous state and account notification state must not be collapsed into a single unconditional success path.
- Account callback ACK is uppercase `OK`; callback payload uses encrypted `data_content`.

Validation focus:

- `cd locallife && make check-baofu-contract`
- Focused Go tests for `locallife/baofu/account/**`
- Account-opening orchestration tests when owner routing, persistence, callback application, or retry behavior changes

### 2. Account Withdrawal And Account Transfer

Status: `Current subset`

Current LocalLife scope: account withdrawal, withdrawal query, and withdrawal notification. Account-to-account transfer and month-end balance are retained as deferred/reference sources.

| Source role | Official extracts |
| --- | --- |
| Current withdrawal flow | `accWithdrawal.md`, `queryWithdrawal.md`, `withdrawNotify.md` |
| Deferred or reference money movement | `accTransfer.md`, `queryTransfer.md`, `bct-1g721ak74fap8.md` |
| Shared account transport and appendices | `notice.md`, `unionGw.md`, `appendix.md`, `bct-1fjpm4fpns79f.md` |

Local boundary:

- Provider contract: `locallife/baofu/account/contracts/official_withdraw.go`
- Client/transport: `locallife/baofu/account/client.go`
- Business flow: withdrawal logic, recovery scheduler, callback API, and persistence

Drift traps:

- Withdrawal synchronous `state=1/2` is acceptance or processing, not final money movement success.
- Withdrawal query requires original `tradeTime=yyyy-MM-dd`; recovery must not replace it with the current date.
- Amount and fee fields are yuan decimals at the provider boundary; LocalLife persistence and domain logic must keep unit conversion explicit.
- Real withdrawal execution is a funds action. Positive C4 evidence requires explicit approval, funding, and masked evidence in `SANDBOX_EVIDENCE_LEDGER.md`.
- Account transfer pages do not become production scope just because withdrawal is implemented.

Validation focus:

- `cd locallife && make check-baofu-contract`
- Focused provider tests for withdrawal request/query/notification DTOs
- Worker or scheduler tests when recovery behavior changes

### 3. Merchant Report And APPLET Authorization

Status: `Current subset`

Current LocalLife scope: WeChat merchant report, merchant-report query, and `bind_sub_config(APPLET)` for LocalLife mini program authorization. Report modification and demo pages are reference unless explicitly promoted.

| Source role | Official extracts |
| --- | --- |
| Product and transport | `bct-1f9o5nti54urb.md`, `bct-1f9o5qri554o2.md`, `bct-1f9o5s1lqlean.md` |
| Current business APIs | `bct-1f9o62bulbiqd.md`, `bct-1f9o63b6ufii5.md`, `bct-1f9o63qmkndkc.md`, `bct-1f9o6qi1pf2r8.md` |
| Reference | `bct-1f9o62opbejct.md`, `bctjuheDEMO.md` |

Local boundary:

- Provider contract: `locallife/baofu/merchantreport/**`
- Business orchestration: `locallife/logic/baofu_merchant_report_service.go` and readiness logic
- Shared envelope/signing: `locallife/baofu/envelope.go`, `locallife/baofu/signing.go`

Drift traps:

- Merchant report uses the aggregate-style public envelope but endpoint `/mch-service/api`; do not reuse account union-gw.
- `merchant_report` and `merchant_report_query` return channel `subMchId` / `channelRetParam.sub_mch_id`; this is not a share receiver.
- `bind_sub_config(APPLET)` binds the LocalLife mini program appid to the reported sub-merchant; it does not create a BaoCaiTong account.
- WeChat `reportInfo`, `address_info`, `bankcard_info`, and `service_codes` must remain nested DTOs. Do not flatten or rename them from local page needs.
- APPLET binding success may have operational propagation delay; do not treat immediate payment failure after bind as contract drift without evidence.

Validation focus:

- `cd locallife && make check-baofu-contract`
- Focused tests for `locallife/baofu/merchantreport/**`
- Readiness/orchestration tests when report state, APPLET authorization, or payment readiness changes

### 4. Aggregate WeChat JSAPI Payment

Status: `Current`

Current LocalLife scope: WeChat JSAPI unified order, order query, order close, and payment notification for the aggregate Baofoo path.

| Source role | Official extracts |
| --- | --- |
| Product, interface, and public envelope | `bct-1f9os3k1eh3p6.md`, `bct-1f9qlkje6qhk7.md`, `bct-1f9qhae400u5l.md`, `bct-1f9qhakcna6te.md` |
| Current payment APIs | `bct-1f9qlvjef634j.md`, `bct-1f9qm13po92jq.md`, `bct-1f9qm0flcca1k.md`, `bct-1f9qm4ujg50cv.md` |
| Payment appendices | `bct-1f9qrd02b5bka.md`, `bct-1f9qrdaere1nb.md`, `bct-1f9qrdjnaqra5.md`, `bct-1f9qrdro3gtv1.md`, `bct-1f9qre51sa7dg.md`, `bct-1f9qrefjkin2b.md`, `bct-1f9qrf159lni6.md`, `bct-1f9qrfb580243.md`, `bct-1f9qrfk0843tm.md`, `bct-1f9qrfsj2fcbu.md`, `bct-1fao0d2gko14o.md` |
| Reference | `bct-1fao0j33tr8nj.md`, `bct-1fboi33hhe4is.md`, `bct-1fgg2hs8f1gta.md`, `bct-1f9qm3q0l3bdo.md`, `bct-1fkgcm5kmrb3f.md` |

Local boundary:

- Provider contract: `locallife/baofu/aggregatepay/**`
- Shared envelope/signing: `locallife/baofu/envelope.go`, `locallife/baofu/signing.go`
- Callback API and payment business logic under `locallife/api/**`, `locallife/logic/**`, workers, schedulers, and persistence

Drift traps:

- Request public field is `bizContent`; response and callback public field is `dataContent`.
- `returnCode=SUCCESS` is communication success only. Terminal payment state comes from payment callback or order query.
- Production WeChat/Alipay reported merchant payment must send `subMchId`; sandbox may omit it only as a recorded environment exception.
- Payment callbacks may omit `outTradeNo`; `tradeNo` fallback through persisted transaction id or Baofoo query is required.
- Callback route must match official `notifyType=PAYMENT`, and unsupported `txnState` or `resultCode` must fail closed.
- Current BCT aggregate docs do not contain a combined-payment request contract. Baofoo combined checkout remains split unless a new BCT aggregate combined interface is officially introduced and promoted.

Validation focus:

- `cd locallife && make check-baofu-contract`
- Focused tests for aggregate payment contracts, envelope signature verification, payment callback parser/API, and payment recovery logic
- C4 evidence only after real callback/fact application is captured and masked in `SANDBOX_EVIDENCE_LEDGER.md`

### 5. Share After Pay

Status: `Current`

Current LocalLife scope: share after pay, share query, and share notification.

| Source role | Official extracts |
| --- | --- |
| Shared aggregate envelope | `bct-1f9qhae400u5l.md`, `bct-1f9qhakcna6te.md` |
| Current share APIs | `bct-1f9qlvu1em0tb.md`, `bct-1f9qm1m0u1s68.md`, `bct-1f9qm58emskkg.md` |
| Shared appendices | `bct-1f9qrdaere1nb.md`, `bct-1f9qre51sa7dg.md`, `bct-1f9qrfsj2fcbu.md`, `bct-1f9qrd02b5bka.md` |

Local boundary:

- Provider contract: `locallife/baofu/aggregatepay/contracts/**`
- Notification parser: `locallife/baofu/aggregatepay/notification/**`
- Business flow: payment/share logic, recovery workers, callbacks, and persistence

Drift traps:

- Share receiver is `sharingDetails[].sharingMerId` from the BaoCaiTong account binding (`sharing_mer_id` / `contractNo`), not channel `subMchId`.
- Share request must not carry `subMchId`.
- Synchronous success is acceptance or processing; final share result comes from `SHARING` callback or share query.
- Official notify type is `SHARING`, not local shorthand `SHARE`.
- Positive share evidence requires a real paid order first; fake-order sandbox reachability proves parser/error handling only.

Validation focus:

- `cd locallife && make check-baofu-contract`
- Focused tests for share request DTOs, share query parsing, notification parser, callback API, and recovery logic

### 6. Pre-Share Refund

Status: `Current`

Current LocalLife scope: pre-share refund, refund query, and refund notification. Post-share refund is not first-version production scope.

| Source role | Official extracts |
| --- | --- |
| Shared aggregate envelope | `bct-1f9qhae400u5l.md`, `bct-1f9qhakcna6te.md` |
| Current refund APIs | `bct-1f9qm06dmb1a9.md`, `bct-1f9qm246c6cp8.md`, `bct-1f9qm5hspcd9v.md` |
| Shared appendices | `bct-1f9qrdaere1nb.md`, `bct-1f9qre51sa7dg.md`, `bct-1f9qrfsj2fcbu.md`, `bct-1f9qrd02b5bka.md` |

Local boundary:

- Provider contract: `locallife/baofu/aggregatepay/contracts/**`
- Notification parser: `locallife/baofu/aggregatepay/notification/**`
- Business flow: refund logic, recovery workers, callbacks, and persistence

Drift traps:

- First version supports refund before share. Do not enable post-share refund fields such as sharing refund arrays by analogy.
- Refund and share must not be initiated concurrently for the same payment order.
- Synchronous success is acceptance; terminal refund state comes from `REFUND` callback or refund query.
- `refundState` is optional in the callback page; when absent, required `resultCode=SUCCESS/FAIL` drives terminal interpretation.
- Non-success `errCode` or unsupported state values must fail closed even when the public communication layer succeeded.

Validation focus:

- `cd locallife && make check-baofu-contract`
- Focused tests for refund request DTOs, refund query parsing, notification parser, callback API, and recovery logic

### 7. Deferred Payment Products And Reference Sources

Status: `Deferred` / `Reference`

These official extracts are retained to prevent field drift and future design mistakes. They are not current LocalLife production scope unless a new design explicitly promotes a group and updates the matrices, code, tests, and rollout evidence.

| Reference group | Official extracts | Do not do |
| --- | --- | --- |
| Protocol payment | `bct-1f9orkbeu25k3.md`, `bct-1f9qh6r2sn5vf.md`, `bct-1f9qhjesrl6rt.md`, `bct-1f9qhmtf1i131.md`, `bct-1flg461i2c8u8.md`, `bct-1flg46hal5l5j.md`, `bct-1f9qhnattitmu.md`, `bct-1f9qhnknf46v5.md`, `bct-1f9qho10m3inj.md`, `bct-1f9qhob58tblr.md`, `bct-1f9qhoo5vj18a.md`, `bct-1f9qhp1tv3e22.md`, `bct-1f9qhplnq3ck3.md`, `bct-1f9qhq2hp0kus.md`, `bct-1gcis40fhnlpr.md`, `bct-1f9qh88e87joe.md`, `historyorder.md` | Do not borrow protocol-payment status, callback, or bind-card semantics for aggregate JSAPI payment. |
| Cloud QuickPass and signing reference | `bct-1fao034siup3b.md`, `bct-1fao041fn94pt.md`, `bct-1fao053hgjkir.md`, `bct-1fao05etpa819.md`, `bct-1fao07u6loaqf.md`, `bct-1fao09442ads5.md`, `bct-1fao0dgb1o7b7.md`, `bct-1fao0e9njjkfm.md`, `bct-1fao0eosv44i1.md`, `bct-1fao0k4r3fjga.md`, `bct-1fao0kpfs1nlf.md`, `bct-1fao0l416pqpd.md` | Do not infer LocalLife WeChat JSAPI or APPLET behavior from Cloud QuickPass pages. |
| Transfer payment product | `bct-zz.md`, `bct-1facj8t3su8go.md`, `bct-1facj99v1hke5.md` | Do not mix transfer-payment methods with BaoCaiTong account withdrawal or aggregate share/refund flows. |
| Online banking/acquiring references | `bct-1fbc9p6aqkea7.md`, `bct-1fdiul8ggbs5f.md`, `bct-1ghvm72o9hqk4.md`, `bct-1fdiulk101ku7.md`, `bct-1fdiunilrjhhe.md`, `bct-1fdiunqpks85d.md`, `bct-1fdiuvesc61am.md`, `bct-1fd9huiq8tuvn.md` | Do not reuse online-banking refund or notification fields for current aggregate pre-share refund. |
| Product intro, credential, and demo material | `Introduction.md`, `juhe_pay-1f03799ice2ps.md`, `bctjuheDEMO.md` | Demo and credential pages are auxiliary evidence only. Do not promote sample transport, sample payloads, or operational certificate material into contract truth without an official field row and source-matrix label. |

## Promotion Rule For Deferred Groups

Before enabling any deferred or reference group in production:

1. Add or update rows in `CONTRACT_SOURCE_MATRIX.md`.
2. Add field-level rows to `BAOCAITONG_FIELD_CONTRACT_MATRIX.md`.
3. Extend `CONTRACT_IMPLEMENTATION_MAP.md` with local owners and caller propagation.
4. Add DTO/parser/validator tests and drift guard coverage where mechanical.
5. Record sandbox or production evidence in `SANDBOX_EVIDENCE_LEDGER.md` only after masking sensitive values.
6. Run `cd locallife && make check-baofu-contract` plus the smallest relevant Go package tests.
