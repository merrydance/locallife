# Baofu Official Source Ledger

Updated: 2026-05-08

This ledger tracks Baofoo official-source provenance for contract extraction. It deliberately separates commit-safe cleaned official extracts from sensitive operational files and uncommitted raw capture material.

## Storage Policy

- Commit-safe official docs: cleaned Markdown extracts from authenticated Baofoo doc.mandao article content are stored under `.github/standards/domains/baofu-payment/official-sources/doc-mandao/`.
- Do not commit raw doc.mandao HTML/TXT, login pages, cookies, credentials, authentication-token URLs, PFX/CER files, provider test packages, commercial agreements, or email material. Keep sensitive or raw capture material in controlled local or private storage and record hash/provenance only.
- Java demo files can contain demo merchant identifiers and certificate-loading examples. Treat them as auxiliary evidence and do not promote demo-only behavior into contract truth.

## Committed Official Page Extracts

| Snapshot | Local path | Files | Hash ledger | Notes |
| --- | --- | ---: | --- | --- |
| Authenticated Baofoo BCT doc.mandao complete catalog cleaned article extracts | `.github/standards/domains/baofu-payment/official-sources/doc-mandao/` | 107 Markdown article pages plus `INDEX.md` | `.github/standards/domains/baofu-payment/official-sources/doc-mandao/SHA256SUMS` | Refreshed from the authenticated 117-page BCT catalog on 2026-05-08. Ten catalog entries are navigation/container pages without article bodies and are recorded in `INDEX.md`. Used to audit the source matrix and field matrix without retaining raw HTML/TXT or authentication material. `CONTRACT_SOURCE_MATRIX.md` and `BAOCAITONG_FIELD_CONTRACT_MATRIX.md` remain the hot-path contract reading layer. |

The cleaned index records every authenticated catalog entry from the refresh. There are no known missing article extracts from the 2026-05-08 authenticated catalog; `queryCard`, `bct-1f9o5s1lqlean`, and `bct-1f9qrdjnaqra5` are now committed as cleaned Markdown extracts.

Primary first-version pages in the cleaned extract set include:

- `unionGw`, `notice`, `openAcc`, `bct-1gj4ccsdha6d8`, `queryAcc`, `queryBalace`, `accWithdrawal`, `queryWithdrawal`, `openAccNotify`, `withdrawNotify`, `appendix`, account FAQ and error-code pages.
- Aggregate payment pages for public envelope, unified order, order query, share, share query, refund, refund query, close, payment/share/refund callbacks, notification type, order-state appendix, pay-code appendix, and error-code appendix.
- Merchant report pages for product notes, interface notice, merchant-report request entry, merchant report, merchant report query, `bind_sub_config`, and appendices.
- Additional authenticated catalog pages for protocol payment, transfer payment, UnionPay, online banking, account binding/upgrade, file upload, history/order helper pages, and demo/example pages are retained for drift detection and future design. They do not expand the current LocalLife production scope by themselves.

## Sensitive Or Auxiliary Local Sources

These files were available locally during extraction but are not committed as raw artifacts.

| Source | Local path | SHA-256 | Sensitivity | Repo handling |
| --- | --- | --- | --- | --- |
| BaoCaiTong test package archive | `/home/sam/文档/分账/宝付/BaoCaiTongTestInfo_OHTERV1.0.0.2.zip` | `b0e3858416c632708a47eee8d30c13d9935f0c18cde250e3c60a08998883e8ca` | May contain provider test certificates, merchant/terminal material, and operational setup notes. | Hash only. Do not commit raw archive or extracted certificate material. |
| Baofoo merchant-report Java demo archive | `/tmp/baofu_demo/baofujuhereport-java_master1.0.0.1.zip` | `1700965b0626e1a32c86476b1b84a72bc7c8fb7c3cfc98167149ead9014f8b77` | Demo auxiliary; sample code contains demo merchant/terminal literals and certificate-loading examples. | Hash only. Use only as `DEMO_AUXILIARY` evidence. |
| CFCA certificate download manual | `/home/sam/文档/分账/宝付/CFCA数字证书下载操作手册.docx` | `68520dd451a0f1a9239c3b6474618c1b851cc709e94835747103ca4399de2c84` | Operational certificate manual. | Hash only. Extract stable non-secret operational rules into standards if needed. |
| Baofoo certificate export manual | `/home/sam/文档/分账/宝付/宝付证书下载导出说明手册（含国密证书）.docx` | `b43653d06db33b5899a57621326da463c136e1c9d000d0f0d5ba0e2945e5a7da` | Operational certificate manual. | Hash only. |
| BaoCaiTong service agreement PDF | `/home/sam/文档/分账/宝付/宝财通产品服务协议-宝财通2.0-来富网络（宁晋）有限公司.pdfd9ed8c8ba4a7453186fab5705c2d2d1c20260320142152.pdf` | `2e6ea1021ef4c1aa82999058c06b3c7c53fcdf9564ea065cc96c280a193f357e` | Commercial/legal agreement. | Hash only. Extract durable technical constraints only when needed. |
| Merchant category/MCC workbook | `/home/sam/文档/分账/宝付/经营类目&MCC.xlsx` | `c521b7b15397a5aa63be9a3d8297c8a8c207e68e7d7fea7a26f8450945b4793f` | Business category source. | Hash only unless a generated sanitized allowlist is needed. |
| Network payment cooperation agreement PDF | `/home/sam/文档/分账/宝付/网络支付合作协议V2.0-来富网络（宁晋）有限公司.pdfab97876cfba746fe8052f5e32f19d1d420260320142225.pdf` | `ebdc5db69ead5733a79c3751d763c697cb1cda6af2bf63fb417e4005cf81cace` | Commercial/legal agreement. | Hash only. |
| Baofoo email notes | `/home/sam/文档/分账/宝付/邮件内容.md` | `1f560fa7db738b43912a89cf89a28f643450aa5090c584c4b539125208a4bb3e` | May contain operational context or contacts. | Hash only; extract confirmed provider answers into `CONTRACT_SOURCE_MATRIX.md` with date and scope. |

## Refresh Procedure

When refreshing Baofoo source material:

1. Re-fetch authenticated doc.mandao pages into a temporary directory.
2. Convert article content into cleaned Markdown under `official-sources/doc-mandao/`; do not retain raw HTML/TXT in the repository.
3. Compare hashes against `official-sources/doc-mandao/SHA256SUMS` and review changed pages against `CONTRACT_SOURCE_MATRIX.md` and `BAOCAITONG_FIELD_CONTRACT_MATRIX.md`.
4. Do not update runtime code directly from raw HTML. First convert the official row into a contract matrix entry and a failing/updated test.
5. If a raw source is sensitive or uncleaned, update this ledger with a hash and do not commit the file.
