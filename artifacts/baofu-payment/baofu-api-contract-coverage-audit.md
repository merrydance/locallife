# 宝付宝财通接口契约覆盖审计

生成日期：2026-05-04

## 1. 审计目标

本文件用于把 LocalLife 宝付宝财通接入需要使用的官方接口集中列出，并逐一核对。审计 findings 的执行修复计划见 `artifacts/baofu-payment/baofu-contract-drift-remediation-plan.md`。

本文件核对：

- 官方用途、请求方式、测试地址、生产地址。
- 请求、响应、错误码、必填、条件必填、字段类型、枚举值。
- 本项目是否已经引入契约层、服务层、持久化、运行时 wiring。
- 是否已经使用宝付测试地址做过真实联调。

结论口径：这是生产前缺口审计，不是“已完成清单”。凡未明确跑过宝付测试环境的接口，一律标记为“未做宝付测试地址联调”。

## 1.1 术语、资金主体与替换边界

为避免宝付和微信两个体系的“商户号/二级商户号”混淆，审计中统一使用以下术语：

| 术语 | 来源 | 用途 | 是否分账接收方 |
| --- | --- | --- | --- |
| 宝付收单一级商户号 | 宝付协议/测试资料中的收单商户号 | 宝财通开户、转账、聚合支付、确认分账等交易发起；主交易资金进入该一级商户号对应的待结算/资金管理体系 | 否 |
| 宝付代付一级商户号 | 宝付协议/测试资料中的代付商户号 | 宝财通提现、提现查询；承接平台预存的开户验证费 | 否 |
| 宝付二级商户号 | 宝财通开户返回的二级户标识，本地字段 `sharing_mer_id` | 商户、骑手、运营商、平台佣金分账接收方；二级户余额包含在途、可用、冻结等状态 | 是 |
| 微信渠道二级商户号 / `subMchId` | 宝付聚合商户报备返回 | 宝付 `unified_order.subMchId`，用于微信渠道交易主体识别和合规报备 | 否 |
| 付款人 `sub_openid` | 微信小程序用户身份 | 宝付 `payExtend.sub_openid`，用于调起微信 JSAPI 支付 | 否 |

资金链路核对口径：顾客下单后通过宝付聚合支付在微信生态完成付款；宝付以宝付收单一级商户号和微信渠道 `subMchId` 发起主交易。交易完成并满足本地不可退款条件后，LocalLife 调用 `share_after_pay`，宝付按 `sharingDetails[].sharingMerId` 将资金分入对应宝付二级户。宝财通文档明确二级户资金先进入在途余额，结算后进入可用余额，可用余额才能提现。

渠道开通口径：宝付个人/机构进件和宝财通开户先产生宝付二级商户号；项目内微信普通服务商特约商户进件流程不再作为宝付主链路前置条件。已向宝付确认支持异主体报备：宝付开户成功后，主业务商户需要继续做聚合商户报备取得微信渠道 `subMchId`，再调用绑定授权目录 `bind_sub_config(authType=APPLET, authContent=<LocalLife 小程序 appid>)`，把该商户的微信渠道 `subMchId` 绑定到 LocalLife 平台小程序。由于 `share_after_pay` 不需要 `subMchId`，`subMchId` 只决定微信小程序统一下单的渠道主体和顾客支付展示主体；商户、骑手、运营商的实际分账接收方仍是宝财通二级户 `sharing_mer_id`。

替换边界：宝付只替换主业务订单中原普通服务商/平台收付通方向的支付、分账、账户、提现和分账前退款互斥能力；不替换微信直连支付。骑手保证金缴纳/赎回、商户追偿向平台付款、骑手追偿向平台付款及其查询、退款、通知继续走微信直连支付。

拓展码/扫码确认：当前在线接口页和本地测试资料未明确给出“拓展码/扩展码”的接口、字段或回包；聚合商户报备附录存在 `CONTACT_CONFIRM`、`LEGAL_CONFIRM`、`AUTHORIZED`、`UNAUTHORIZED` 等确认/认证状态，宝财通协议提到可通过宝付商户前台页面或接口提交资质并取得授权。上线前必须向宝付确认扫码确认属于开户、报备还是电子签/认证步骤，以及生成接口、返回字段、扫码主体、有效期、查询状态和失败处理。

## 2. 覆盖等级

| 等级 | 含义 |
| --- | --- |
| C0 未引入 | 项目没有对应 DTO/client/service。 |
| C1 契约草稿 | 有部分 DTO/枚举/测试，但未覆盖官方完整字段、条件必填和错误码。 |
| C2 服务层草稿 | 有业务 service 或 worker 调用边界，但依赖 mock/interface，未接真实 HTTP。 |
| C3 生产传输 | 有真实 HTTP client、签名/加密/验签、endpoint 配置、错误映射。 |
| C4 沙箱验证 | 已用宝付测试地址和测试商户号完成请求/回调/查询联调，并留存证据。 |

当前全局判断：项目内宝付代码已把聚合支付、聚合商户报备和部分宝财通账户能力推进到本地 C3（真实 HTTP client、endpoint 配置、httptest transport 和业务 wiring）。账户查询和聚合支付 `order_query` 已有宝付测试地址负向 smoke 证据，但没有任何生产必用接口达到完整正向 C4；凡未在 `baofu-sandbox-evidence.md` 留正向请求/回调/查询证据的接口仍不得视为已完成宝付测试地址联调。


## 2.1 本次补充审计结论

- 已确认 `subMchId` 策略：采用商户逐户聚合商户报备 + 平台小程序异主体授权目录绑定。代码不得再固化为“平台统一下单”，也不得复用项目内微信普通服务商特约商户进件结果。
- `share_after_pay` 官方字段不包含 `subMchId`。分账接收方仍必须是宝财通开户返回并同步到本地 `sharing_mer_id` 的宝付二级商户号。
- 本地已补入官方字段级 DTO、公共报文、聚合商户报备、退款、关单、官方错误码本地分类和微信类目 allowlist；剩余漂移风险集中在宝财通 union-gw 官方加密/签名 envelope 完整性、响应验签/数字信封、真实渠道错误组合、回调真实 payload 与沙箱证据。
- “防漂移”的实现标准不能只靠文档：必须把官方必填/条件必填、字段类型长度、枚举、错误码、金额单位、回调 ACK 形态、测试/生产 endpoint 都变成 typed constants、校验器和表驱动测试。
- 已新增 `make check-baofu-contract` 静态守卫，阻断已发现的漂移回归：响应层直接读 `BizContent`、误用旧 `https://api.baofoo.com`、分账携带 `subMchId`、用宝付一级商户号填 `sharingMerId`、重新引入静态 `BAOFU_AES_KEY`。


## 2.2 Source Ledger

| Group | Local source file/demo | Official URL | Current trust level | Notes |
| --- | --- | --- | --- | --- |
| union-gw account | `/home/sam/文档/分账/宝付/宝财通产品服务协议-宝财通2.0-来富网络（宁晋）有限公司.pdf...`, `/home/sam/文档/分账/宝付/BaoCaiTongTestInfo_OHTERV1.0.0.2/**`, doc.mandao pages | `unionGw`, `openAcc`, `queryAcc`, `queryBalace`, `accWithdrawal`, `queryWithdrawal`, `openAccNotify`, `withdrawNotify` | doc + local DTO/client tests + negative sandbox smoke | `verifyType=1` RSA/base64 is the local implementation baseline. `verifyType=2`, real notification encrypted payloads, and successful account-open/query samples remain C4-open. |
| aggregate pay | `/tmp/baofu_doc.html` catalog, `/tmp/baofu_demo/java/src/main/java/com/baofoo/Entitys/PostMasterEntity.java`, `/tmp/baofu_demo/java/src/main/java/com/baofoo/Entitys/ResultMasterEntity.java`, aggregate demo/entity classes | `bct-1f9qhakcna6te`, `bct-1f9qm38du1agl`, `bct-1f9qlvu1em0tb`, `bct-1f9qm06dmb1a9`, `bct-1f9qm13po92jq`, `bct-1f9qm1m0u1s68`, `bct-1f9qm246c6cp8`, callback pages | doc + Java demo + local tests + public-envelope sandbox smoke | Request public envelope uses form fields and request-side `bizContent`; response public envelope uses `dataContent`. Real business `dataContent` samples for order/query/share/refund remain C4-open. |
| merchant report | `/tmp/baofu_doc.html` catalog, `/tmp/baofu_demo/java/src/test/java/com/baofoo/Demo/MerchantReport.java`, `MerchantReportQuery.java`, `BindSubConfig.java`, `/home/sam/文档/分账/宝付/经营类目&MCC.xlsx` | `bct-1f9o62bulbiqd`, `bct-1f9o63b6ufii5`, `bct-1f9o63qmkndkc` | doc + Java demo + local DTO/service tests | 商户逐户报备取得 `subMchId`，再 `bind_sub_config(authType=APPLET, authContent=<LocalLife 小程序 appid>)`；真实报备/授权目录样本仍 C4-open。 |

Source inventory command for this audit slice:

```bash
rg -n "接口请求入口|bizContent|dataContent|riskInfo|share_after_pay|merchant_report|merchant_report_query|bind_sub_config|T-1001-013-01|T-1001-013-03|T-1001-013-06|T-1001-013-14|T-1001-013-15" \
  /home/sam/文档/分账/宝付 /tmp/baofu_demo/java \
  > /tmp/baofu_contract_source_hits.txt
```

本次命中确认了聚合支付 Java demo 的 `PostMasterEntity.bizContent`、`ResultMasterEntity.dataContent`，以及聚合商户报备 demo 的 `merchant_report`、`merchant_report_query`、`bind_sub_config` 方法名。union-gw 账户详细字段主要以 doc.mandao 页面和本地 BaoCaiTong 测试资料/协议为源，仍需后续沙箱正向样本把 C3 固化到 C4。

## 2.3 Status / Error Interpretation Matrix

下表只列本地会解释并影响业务状态、重试或前端语义的字段。未列字段不得在业务层临时解释；新增解释必须先补 DTO/枚举/测试。

| 接口组 | 字段 | 官方/本地取值 | 本地解释 | 失败闭合规则 | 本地证明 |
| --- | --- | --- | --- | --- | --- |
| union-gw 公共响应 | `header.sysRespCode/sysRespDesc` | `0000` 表示系统层成功，其他为系统层失败 | `ValidateResponse` 校验 `memberId/terminalId/serviceTp` 后再解释系统码 | 非 `0000` 转 `ProviderError`，`UpstreamMessage=sysRespDesc`，前端只给安全中文语义 | `TestUnionGWResponseValidationRejectsMismatchedServiceType`、`TestProviderErrorKeepsUpstreamMessageOutOfFrontendGuidance` |
| 宝财通账户业务响应 | `body.retCode/errorCode/errorMsg` | `1`/`SUCCESS` 成功；`0`、`2`、`BF****`、`errorCode` 表示失败或处理中 | `accountBusinessFailure` 统一转 `ProviderError`；`OpenStateFromUpstream` 只在业务成功后归一化账户状态 | 缺 `retCode` 但有错误字段 fail-closed；缺 `retCode` 且无错误字段也按 `MISSING_RET_CODE` fail-closed | `TestAccountClientReturnsProviderErrorForBusinessFailure`、`TestAccountClientReturnsProviderErrorWhenErrorCodeHasNoRetCode`、`TestBusinessFailureDetectorsFailClosedForMissingSuccessIndicators` |
| 宝财通开户状态 | `state` | `1/0/2/-1` | `active/failed/processing/abnormal` | 未知值归 `abnormal`，不得当成功开户 | `TestOpenStateFromUpstream` |
| 宝财通提现状态 | `state` | `1/0/2/3` | `succeeded/failed/processing/returned` | 未知值归 `processing`，由查询/通知继续收敛 | `TestOfficialWithdrawAmountConvertsFenToYuan`、提现通知 parser tests |
| 聚合支付/报备公共响应 | `returnCode/returnMsg` | `SUCCESS/FAIL` | 公共层成功后才读取 `dataContent` | `FAIL` 直接转 `ProviderError`；`SUCCESS` 缺 `dataContent` fail-closed；响应 `bizContent` 只作为本地兼容 fallback | `TestPublicResponseEnvelopeValidateHandlesTransportStatus`、`TestAggregateClientReturnsProviderErrorForEnvelopeFailure` |
| 聚合支付/报备业务响应 | `resultCode/errCode/errMsg` | `SUCCESS` 成功；`FAIL` 或错误码失败 | `publicBusinessFailure` 统一解释支付、报备、分账、退款、关单业务结果 | 缺 `resultCode` 但有错误字段 fail-closed；缺 `resultCode` 且无错误字段也按 `MISSING_RESULT_CODE` fail-closed | `TestAggregateClientReturnsProviderErrorForBusinessFailure`、`TestMerchantReportClientReturnsProviderErrorForBusinessFailure`、`TestBusinessFailureDetectorsFailClosedForMissingSuccessIndicators` |
| 聚合支付状态 | `txnState/state` | `WAIT_PAYING/SUCCESS/CLOSED/PAY_ERROR/REFUND/ABNORMAL` | `processing/success/closed/failed/success/unknown` | 未知值归 `unknown`，不触发成功事实应用 | `TestNormalizePaymentTerminalStatus` |
| 确认分账状态 | `txnState/state` | `PROCESSING/SUCCESS/CANCELED/ABNORMAL` | `processing/success/failed/unknown` | 未知值归 `unknown`，不触发成功事实应用 | `TestNormalizeShareTerminalStatus` |
| 退款状态 | `refundState/state` | `SUCCESS/REFUND/REFUND_ERROR/ABNORMAL` | `success/processing/failed/unknown` | 未知值归 `unknown`，由查询/通知继续收敛 | `TestNormalizeRefundTerminalStatus` |
| 聚合商户报备状态 | `reportState` | `SUCCESS/FAIL/PROCESSING` | `succeeded/failed/processing` | 未知值归 `unknown`，不得进入支付 readiness | `TestNormalizeMerchantReportState`、`TestBaofuPaymentReadinessRequiresMerchantSubMchIDAndAppletAuth` |
| 绑定授权目录 | `resultCode/errCode/errMsg` 与本地 `applet_auth_state` | `SUCCESS` 成功，其余失败/待确认 | 成功才写 `applet_auth_state=succeeded`；失败写 failed 并保留脱敏 code/message | 未成功不允许 `unified_order`，只返回产品语义“微信支付通道待开通” | `TestBaofuMerchantReportServiceBindsAppletAfterReportSuccess`、`TestPaymentOrderServiceCreatePaymentOrder_BaofuWechatChannelNotReadyFailsBeforeClientCall` |
| 错误码前端语义 | `ProviderError.UpstreamCode/UpstreamMessage` | 官方错误码、未知错误码、HTTP/解析错误 | `ClassifyBaofuError` 归类为资料需修改、平台配置、可重试、人工处理 | `UpstreamMessage` 只留在 `ProviderError` 供日志/运营诊断，`Frontend.Message` 不拼接上游原文 | `TestClassifyBaofuOfficialErrorTables`、`TestProviderErrorKeepsUpstreamMessageOutOfFrontendGuidance` |

## 3. 官方入口地址

| 能力组 | 官方文档 | 请求方式 | 测试地址 | 生产地址 | 生产备用地址 | 当前项目覆盖 |
| --- | --- | --- | --- | --- | --- | --- |
| 宝财通账户 API / union-gw | https://doc.mandao.com/docs/bct/unionGw | POST | `https://vgw.baofoo.com/union-gw/api/{报文编号}/transReq.do` | `https://public.baofu.com/union-gw/api/{报文编号}/transReq.do` | 未见 | C3：已拆官方 endpoint profile，账户 open/query/balance/withdraw/query withdraw concrete client 已改用 union-gw `verifyType=1` URL 参数、`content` 密文、`header/body` 明文 envelope，并校验响应 `sysRespCode/memberId/terminalId/serviceTp`；`verifyType=2`、真实宝付沙箱和通知密文仍需复核，不得升 C4。 |
| 宝付聚合支付 | https://doc.mandao.com/docs/bct/bct-1f9qhakcna6te | POST | `https://mch-juhe.baofoo.com/api` | `https://juhe.baofoo.com/api` | `https://juhe-backup.baofoo.com/api` | C3：已有 concrete HTTP client、httptest envelope、统一下单/查询/分账/退款/关单 DTO 和主业务 API runtime wiring；响应验签/数字信封与沙箱证据仍未完成；微信直连支付不在宝付替换范围。 |
| 聚合商户报备 | https://doc.mandao.com/docs/bct/bct-1f9o5s1lqlean | POST | `https://mch-juhe.baofoo.com/mch-service/api` | `https://juhe.baofoo.com/mch-service/api` | 未见 | C3：已有 `merchant_report`、`merchant_report_query`、`bind_sub_config` DTO/client、报备表、sqlc、service 和 readiness；真实资料映射、完整附录枚举和沙箱证据仍未完成。 |

公共报文要求：

- 账户 union-gw：URL 带 `memberId`、`terminalId`、`verifyType`、`content`、条件字段 `veryfyString`；body 明文由 `header` + `body` 组成，`header.serviceTp` 必须与 URL 报文编号一致。
- 聚合支付/聚合商户报备：请求公共字段包括 `merId`、`terId`、`method`、`charset=UTF-8`、`version=1.0`、`format=json`、`timestamp`、`signType`、`signSn`、`ncrptnSn`、`dgtlEnvlp`、`signStr`、`bizContent`；响应公共字段按宝付 Java demo 和沙箱回包使用 `dataContent` 承载业务 JSON，`bizContent` 只作为本地历史兼容 fallback。
- 当前项目已把三组入口拆成独立 endpoint 配置并限制官方测试/生产地址；仍未使用宝付测试地址完成真实联调，也未补齐响应验签/数字信封证据。

## 4. 必用接口总览

| 能力 | 接口 | 官方方法/报文编号 | 用途 | 本地契约 | 本地运行时 | 沙箱联调 |
| --- | --- | --- | --- | --- | --- | --- |
| 账户开户 | 个人/机构开户 | `T-1001-013-01` | 商户、骑手、运营商、平台二级户开户 | C2/C3：已有官方 `OfficialOpenAccountRequest`、个人二要素/四要素/机构 DTO 与校验；业务抽象仍需完善企业/个体全量资料映射 | C2/C3：有 service command 记录和 concrete client；union-gw envelope 完整性待复核 | 未做 |
| 账户开户 | 个人开户二要素 | `T-1001-013-01` 变体 | 骑手个人二要素开户候选 | C3：已区分 `OfficialPersonalTwoFactorAccountInfo` 与四要素开户 | C2/C3：client 可组装二要素请求；是否允许生产使用待宝付沙箱回包确认 | 未做 |
| 账户查询 | 开户查询 | `T-1001-013-03` | 同步 `contractNo`、二级商户号/状态 | C3：已有 `OfficialQueryAccountRequest` 和校验 | C2/C3：已有 concrete client；查询条件仍需按沙箱确认补齐 | 未做 |
| 开户通知 | 开户结果通知 | 通知 URL | 开户异步结果 | C1：notification parser 有本地测试 | C2：callback 落 fact | 未做 |
| 余额 | 账户余额查询 | `T-1001-013-06` | 二级户在途/可用/冻结余额 | C3：已有官方余额 DTO 和元/分转换测试 | C3：已有 concrete client 和提现 service 读余额边界 | 未做 |
| 提现 | 账户提现 | `T-1001-013-14` | 二级户可用余额提现到银行卡 | C3：已有官方提现 DTO 和分转元转换 | C2/C3：已有提现 service/worker/core client；公网 API 路由和沙箱证据待补 | 未做 |
| 提现查询 | 提现查询 | `T-1001-013-15` | 提现处理中恢复/对账 | C3：已有官方提现查询 DTO 和状态映射 | C3：已有 client、worker 状态应用和 `baofu-withdrawal-recovery` 调度查询入队；沙箱证据待补 | 未做 |
| 提现通知 | 提现结果通知 | 通知 URL | 提现终态通知 | C3：官方字段 parser、密文 envelope parser、纯文本 `OK` ACK 已有本地测试 | C3：`/v1/webhooks/baofu/withdraw` 已按 `transSerialNo` 定位提现单并入队提现 fact application；沙箱证据待补 | 未做 |
| 聚合商户报备 | 报备认证 | `merchant_report` | 商户宝付开户后逐户报备微信渠道，取得该商户 `subMchId` | C3：字段级 DTO、微信类目 allowlist 和校验已建 | C3：表/sqlc/service/client/readiness 已建；真实资料映射待补 | 未做 |
| 聚合商户报备 | 报备信息查询 | `merchant_report_query` | 查询平台或商户报备状态和 `subMchId` | C3：DTO/状态归一化已建 | C3：client/service 同步 `sub_mch_id` 边界已建，`baofu-merchant-report-recovery` 已补处理中报备和 APPLET 授权补偿 | 未做 |
| 聚合支付 | 统一下单交易创建 | `unified_order` | 主支付入口，微信 JSAPI 支付 | C3：官方字段、条件必填、金额关系、`riskInfo.clientIp` 校验已建 | C3：concrete client + API runtime main-business wiring 已建；不回退普通服务商 | 未做 |
| 聚合支付 | 支付订单查询 | `order_query` | 支付回调缺失恢复 | C3：DTO/client 已建 | C3：recovery scheduler 已可使用生产 aggregatepay client 查询并落 fact | 未做 |
| 聚合支付 | 支付结果通知 | 通知 URL | 支付终态回调 | C2/C3：notification parser 与 ACK 语义有本地测试 | C2/C3：callback 落 fact 并入队；真实验签/数字信封和沙箱回调待补 | 未做 |
| 确认分账 | 确认分账 | `share_after_pay` | 支付成功后按 `sharingMerId` 分账 | C3：DTO/Validate 已建，接收方只读 `sharing_mer_id` | C3：worker 已可使用生产 aggregatepay client 创建分账；沙箱证据待补 | 未做 |
| 分账查询 | 分账订单查询 | `share_query` | 分账处理中恢复 | C3：DTO/client 已建 | C3：scheduler 已可使用生产 aggregatepay client 查询落 fact；沙箱证据待补 | 未做 |
| 分账通知 | 分账结果通知 | 通知 URL | 分账终态回调 | C2/C3：parser/callback/fact application 已建 | C2/C3：真实验签/数字信封和沙箱回调待补 | 未做 |
| 退款 | 申请退款 | `order_refund` | 首版仅分账前退款 | C3：本地 DTO/client/业务互斥已建 | C3：分账前退款已接入，分账后禁退 | 待沙箱 |
| 退款查询 | 退款订单查询 | `refund_query` | 退款恢复/对账 | C3：DTO/client 已建 | C3：退款恢复 scheduler 已可使用生产 aggregatepay client 查询并落 fact | 待沙箱 |
| 退款通知 | 退款结果通知 | 通知 URL | 退款终态回调 | C2/C3：parser/ACK 语义已建 | C2/C3：API callback 已落 Baofu refund fact 并入队应用；真实验签/数字信封和沙箱回调待补 | 待沙箱 |
| 关单 | 交易关闭 | `order_close` | 上游支付失败/本地关闭时关单 | C3：DTO/client 已建 | C2：本地 pay data 失败已关上游，其他关闭路径待扩展 | 待沙箱 |

## 5. 暂缓或条件接口

| 接口 | 官方文档 | 是否首版必用 | 说明 | 当前覆盖 |
| --- | --- | --- | --- | --- |
| 查询绑定卡 | https://doc.mandao.com/docs/bct/queryCard | 否，除非资金页/运营后台要显示绑定卡状态 | 可作为开户后核验工具 | C0 |
| 账户信息修改 | https://doc.mandao.com/docs/bct/updateCard | 否，但商户/骑手资料变更后需要 | 后续变更银行卡/手机号/资质时要接 | C0 |
| 账户收支明细查询 | https://doc.mandao.com/docs/bct/accDetails | 对账增强项 | 可补充资金流水对账 | C0 |
| 月终余额查询 | https://doc.mandao.com/docs/bct/bct-1g721ak74fap8 | 对账增强项 | 月结核对 | C0 |
| 转账（账户间） | https://doc.mandao.com/docs/bct/accTransfer | 目前不进入主链路 | 只有需要二级户之间调账时使用 | C0 |
| 转账结果查询 | https://doc.mandao.com/docs/bct/queryTransfer | 同上 | 账户间转账恢复 | C0 |
| 报备信息修改 | https://doc.mandao.com/docs/bct/bct-1f9o62opbejct | 非首单必需，但生命周期必需 | 微信/支付宝渠道资料变更时使用 | C0 |
| 绑定授权目录 | https://doc.mandao.com/docs/bct/bct-1f9o63qmkndkc | 微信小程序支付上线前必须执行 | `authType=APPLET`、`authContent=<LocalLife 小程序 appid>`；每个完成聚合商户报备的商户 `subMchId` 都绑定平台小程序；不影响 `share_after_pay` 分账接收方 | C3：DTO/client/service/readiness 已建；沙箱证据待补 |

## 6. 账户 API 详细核对

### 6.1 个人/机构开户 `T-1001-013-01`

官方文档：https://doc.mandao.com/docs/bct/openAcc

用途：开通宝付账簿二级户。开户成功后存在在途户和可用余额户；分账先入在途户，结算后进入可用余额户。

请求结构摘要：

| 字段 | 必填 | 类型/长度 | 条件/枚举 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `version` | M | String(5) | 文档为 `4.1.0` | C3：`OfficialOpenAccountRequest.Version` 常量校验，`TestOfficialOpenAccountRequestRequiresBCT20Fields` |
| `accType` | M | int(1) | `1` 个人，`2` 企业/个体 | C3：官方 DTO 使用 `OfficialAccountTypePersonal/Business`，业务抽象只在 adapter 层转换 |
| `accInfo` | M | Object | 根据 `accType` 不同 | C3：个人二要素、个人四要素、企业/个体 DTO 分型并校验 unsupported type |
| `noticeUrl` | M | String(256) | 开户通知地址 | C3：HTTPS 校验，runtime 从 `NotifyBaseURL + /account/open` 组装 |
| `businessType` | M | String(32) | `BCT2.0` | C3：`OfficialBusinessTypeBCT20` 常量校验 |
| `accInfo.transSerialNo` | M | String(200) | 请求流水号 | C3：`OutRequestNo` 只在 adapter 层映射为官方 `transSerialNo` |
| `accInfo.loginNo` | M | String(32) | 全局唯一，长度 11 位以上 | C3：个人/企业均校验 `loginNo` 至少 11 位 |
| 个人：`customerName/certificateType/certificateNo/cardNo/mobileNo/cardUserName/needUploadFile` | M | String/boolean | `certificateType=ID`；二要素接口无 `cardNo/mobileNo` | C3：个人二要素允许无银行卡，四要素要求 `cardNo/mobileNo/cardUserName` |
| 企业/个体：`email/selfEmployed/customerName/certificateNo/certificateType/corporateName/corporateCertType/corporateCertId/industryId/cardNo/bankName/depositBankProvince/depositBankCity/depositBankName` | M/C/O | 多类型 | `selfEmployed=true` 且绑定对私卡时 `corporateMobile` 必传；证件类型有多枚举 | C3：首版 DTO 覆盖必填字段；`selfEmployed` 时 fail-closed 要求 `corporateMobile`；附件/证件类型长尾仍待沙箱 |
| `platformNo/platformTerminalId/qualificationTransSerialNo` | C | String | 代理模式/上传资质时条件必填 | 未覆盖 |

响应结构摘要：`retCode`、`errorCode`、`errorMsg`、`result[]`；`result.state` 枚举 `1` 成功、`0` 失败、`-1` 异常、`2` 开户处理中；返回 `transSerialNo`、`loginNo`、`customerName`、`contractNo` 等。

本地缺口：

- `locallife/baofu/account/contracts/types.go` 的 `OpenAccountRequest` 仍是业务抽象；官方字段级 DTO 已拆到 `official_open.go` 等文件。
- 已区分个人二要素、个人四要素、企业/个体字段集合；企业/个体首版必填字段、`loginNo` 长度、`selfEmployed -> corporateMobile` 条件已落入表驱动测试，资质附件和证件类型长尾仍需沙箱样例校准。
- 已把 `businessType=BCT2.0`、`version=4.1.0`、`accType`、个人开户条件必填和企业/个体首版字段落入契约校验。
- 已新增官方 DTO 和本地 client；client 已按 `verifyType=1` 构造 union-gw URL 参数和 `header/body` 密文 envelope；开户 `noticeUrl` 已改为运行时 `BAOFU_NOTIFY_BASE_URL + /account/open`，不再使用 placeholder。2026-05-05 已用测试地址跑过个人二要素和四要素负向开户，证明请求可达且响应可解析，但上游返回 `BF0001`/abnormal，未返回 `contractNo`/`sharing_mer_id`；四要素 smoke 暴露开户响应业务字段位于 `result[]`，且官方 `retCode` 为 int、示例 `result.state` 为数字。本地 parser 已改为优先读取第一条 `result`、兼容 string/number 标量，再回退 top-level 字段。正向开户、`verifyType=2` 和通知密文形态仍需复核。

### 6.2 开户查询 `T-1001-013-03`

官方文档：https://doc.mandao.com/docs/bct/queryAcc

请求字段：`version` M；`accType` M；按条件传 `certificateNo/certificateType/platformNo/loginNo`，或传 `contractNo`。

响应字段：`retCode/errorCode/errorMsg/result`；`result.contractNo` M；其他包括 `contractName/customerName/customerNo/customerType/certificateType/certificateNo/platformNo/bindMobile/email`。

本地缺口：

- 已新增 `OfficialQueryAccountRequest` 覆盖 `version`、`accType`、`contractNo/loginNo/certificateNo` 查询路径；本地强制一次只用一个查询 key，`certificateNo` 查询时必须带 `certificateType`，避免多 key 语义漂移。业务层 `QueryAccountRequest` 仍是归一化输入。
- 当前项目把 `contractNo` 与 `sharing_mer_id` 严格拆开是正确的；本地 active receiver 约束要求显式 `sharing_mer_id`。但官方查询页只明确 `contractNo`，开户/查询/通知哪个字段承载宝付二级商户号仍必须用实际沙箱回包确认后固化。

### 6.3 开户结果通知

官方文档：https://doc.mandao.com/docs/bct/openAccNotify

通知要求：商户收到后返回大写 `OK`；未确认会重发。通知 URL 形如 `...?member_id=...&terminal_id=...&data_type=JSON&data_content=密文`。

通知明文字段：`member_id`、`terminal_id`、`memberType`、`state`、`errorCode`、`errorMsg`、`transSerialNo`、`loginNo`、`customerName`、`contractNo`、`noticeType`。

本地覆盖：

- `locallife/baofu/account/notification/notification.go` 已按官方 `data_content` RSA/base64 解密后解析 `transSerialNo/state/errorCode/errorMsg/contractNo`，不再读取自造 `outRequestNo/sharingMerId/status` 字段，也不再依赖静态 `BAOFU_AES_KEY`。
- `locallife/api/baofu_callback.go` 已在开户回调落 fact 后返回 `text/plain` 纯文本 `OK`。
- 本地测试覆盖官方 query-string 通知参数、RSA/base64 `data_content` 解码、缺失 `data_content` fail-closed 和纯文本 `OK` ACK；沙箱回调证据未做。

### 6.4 账户余额查询 `T-1001-013-06`

官方文档：https://doc.mandao.com/docs/bct/queryBalace

请求字段：`version` M，`contractNo` M，`accType` M。响应字段：`retCode/errorCode/errorMsg/availableBal/pendingBal/currBal/freezeBal`。

本地覆盖：

- 已新增官方余额 DTO，并把官方元 `BigDecimal(10,2)` 与本地分金额转换集中到契约边界且有测试。
- 已有真实 client/httptest；未做测试地址联调。

### 6.5 账户提现 `T-1001-013-14`

官方文档：https://doc.mandao.com/docs/bct/accWithdrawal

请求字段：`version=4.2.0` M，`contractNo` M，`directPlatformNo` C，`transSerialNo` M，`dealAmount` M 元，`returnUrl` M，`feeMemberId` C，`reqReserved` O，`transAbstract` C。

响应字段：`retCode/errorCode/errorMsg/transSerialNo/contractNo/state/transRemark`；受理状态 `state=1` 受理成功，`2` 受理失败。

本地缺口：

- `WithdrawRequest` 仍使用本地 `AmountFen`，已在官方 DTO 边界转换为元金额并校验最多 2 位小数；`dealAmount=0` fail-closed。
- `directPlatformNo/feeMemberId/transAbstract/reqReserved` 仍未完整接入业务输入。
- 提现 client 已存在，但收/付一级商户号边界和开户手续费/提现资金账户扣款仍需沙箱验证。

### 6.6 提现查询 `T-1001-013-15` 与提现通知

官方文档：https://doc.mandao.com/docs/bct/queryWithdrawal 与 https://doc.mandao.com/docs/bct/withdrawNotify

查询请求：`version=4.2.0` M，`transSerialNo` M，`tradeTime` M (`yyyy-MM-dd`)。查询响应状态：`1` 成功、`0` 失败、`2` 处理中、`3` 提现退回。

通知要求：返回大写 `OK`；通知数据包含 `contractNo/orderId/transSerialNo/transMoney/transFee/transferTotalAmount/state/transRemark/reqReserved`，`state=3` 为提现退回。

本地覆盖：

- 已有 `WithdrawStatusFromUpstream` 包含 `3 -> returned`，方向正确。
- 已新增提现通知明文字段 parser 和 union-gw 密文 envelope parser，覆盖 `contractNo/orderId/transSerialNo/transMoney/transFee/transferTotalAmount/state/transRemark/reqReserved` 和元转分校验。
- 已新增 `/v1/webhooks/baofu/withdraw`，按 `transSerialNo` 查询本地提现单，入队 `BaofuWithdrawalFactApplicationPayload`，ACK 固定纯文本 `OK`；提现单终态仍由 worker 单写。
- 提现查询 DTO 已覆盖 `tradeTime`；当前 client 默认用当前日期，历史恢复场景需要显式传入交易日期。
- 未做测试地址联调。

## 7. 聚合商户报备详细核对

### 7.1 报备认证 `merchant_report`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9o62bulbiqd

用途：微信/支付宝渠道报备；宝付技术支持已确认本项目不再需要项目内微信支付特约商户进件流程，并支持异主体报备。宝付开户成功后，主业务商户逐户调用 `merchant_report` 做微信渠道报备，成功后取得该商户用于统一下单的 `subMchId`。由于宝付确认分账不读取 `subMchId`，该接口的关键作用是提供微信小程序统一下单需要的渠道主体和顾客支付展示主体；不得把 `subMchId` 当分账接收方。

请求字段摘要：

| 字段 | 必填 | 类型 | 条件/枚举 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `agentMerId/agentTerId` | 否 | S(16) | 代理场景 | 未接入首版 |
| `merId/terId` | 是 | S(16) | 宝付交易商户号/终端号 | C3 |
| `reportType` | 是 | E | `WECHAT`/`ALIPAY`，枚举见附录 | C3，首版只允许 `WECHAT` |
| `reportNo` | 是 | S(64) | 每次请求唯一 | C3 |
| `reportInfo` | 是 | C | 按报备类型传 JSON | C3 微信字段骨架 |
| `bctMerId` | 是 | S(64) | 宝财通二级商户号；主业务商户逐户报备时取商户 `sharing_mer_id` | C3 |

微信 `reportInfo` 关键字段：

| 字段 | 必填 | 类型 | 说明 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `merchant_name` | 是 | S(50) | 主体全称 | C3 |
| `merchant_shortname` | 是 | S(20) | 消费者展示名称 | C3 |
| `service_phone` | 是 | S(20) | 客服电话 | C3 |
| `contact/contact_phone/contact_email` | 否 | S | 联系信息 | C2/C3，字段骨架已建 |
| `channel_id/channel_name` | 是 | S | 测试环境文档给固定渠道参数示例 | C2/C3，取值待沙箱确认 |
| `business` | 是 | S(10) | 经营类目/MCC | C3，已从本地 XLSX 生成 110 项微信类目 allowlist |
| `service_codes` | 是 | C | 如 `JSAPI`、`APPLET` | C3 |
| `address_info` | 是 | C | 省市区、地址、经纬度等 | C2/C3，字段骨架已建 |
| `business_license/business_license_type` | 是 | S | 证照编号和类型 | C2/C3，字段骨架已建 |
| `bankcard_info` | 是 | C | 结算卡信息 | C2/C3，字段骨架已建，沙箱样例待确认 |

响应字段：`reportType/reportNo/reportState/subMchId/platformBizNo/resultCode/errCode/errMsg`；`subMchId` 在成功时有值，是微信/支付宝分配的商户识别码。该值按商户保存为商户微信渠道 `subMchId`，只作为 `unified_order.subMchId` 的来源，不写入分账接收方字段。

本地缺口：

- 已新增 `locallife/baofu/merchantreport/contracts` 字段级 DTO、报备/授权枚举、状态归一化和微信经营类目 allowlist； unsupported 经营类目、证件类型、服务类型、地址和结算卡必填缺失均 fail-closed。
- 已新增 `baofu_merchant_reports` 表、sqlc query、报备 service 和 readiness 合成逻辑；宝付支付下单不再读取普通服务商 `txResult.SubMchID`，改读报备成功且 APPLET 授权成功后的商户 `sub_mch_id`。
- 宝付聚合商户报备资料字段已有 service 输入骨架，附件/证照文件上传与真实资料来源映射仍需结合开户/商户入驻资料收口。
- 附录枚举已本地覆盖：报备类型、报备状态、微信服务类型、微信证件类型、授权类型、终端/联系人/商户/认证等长尾枚举，以及 110 项微信经营类目 allowlist；结算卡字段的真实渠道样例仍需沙箱确认。
- 未做测试地址联调：`https://mch-juhe.baofoo.com/mch-service/api`。

### 7.2 报备信息查询 `merchant_report_query`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9o63b6ufii5

请求字段：`agentMerId/agentTerId` O，`merId/terId` M，`reportType` M，`reportNo` M。

响应字段：`merId/terId/reportType/reportNo/reportState`，以及渠道参数 `channelRetParam`；微信/支付宝返回中包含渠道处理结果和 `subMchId`。

本地覆盖：C3。已有 `MerchantReportQueryRequest`、状态归一化、client 和 service 同步边界；生产必须继续用它做报备恢复和 `subMchId` 补偿同步。APPLET 授权未成功时 readiness 仍保持不可支付。未做沙箱联调。

### 7.3 绑定授权目录 `bind_sub_config`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9o63qmkndkc

用途：服务商给特约子商户配置支付目录/公众号/小程序授权。宝付文档写明 `authType=APPLET` 时，`authContent` 需填写小程序 appid。本项目微信小程序支付使用 LocalLife 平台小程序 appid；宝付已确认支持异主体报备，因此每个完成聚合商户报备的商户 `subMchId` 都需要绑定 LocalLife 平台小程序 appid。

请求字段摘要：

| 字段 | 必填 | 类型 | 条件/枚举 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `agentMerId/agentTerId` | 否 | S(16) | 代理场景 | 未接入首版 |
| `merId/terId` | 是 | S(16) | 宝付交易商户号/终端号 | C3 |
| `subMchId` | 是 | S(30) | 微信/支付宝分配的商户识别码，取报备成功结果 | C3 |
| `authType` | 是 | E | `AUTH` 支付目录、`JSAPI` 公众号、`APPLET` 微信小程序 | C3，首版使用 `APPLET` |
| `authContent` | 是 | S(256) | `APPLET` 填小程序 appid | C3，取 LocalLife 小程序 appid |
| `remark` | 是 | S(128) | 备注 | C3 |

边界结论：`bind_sub_config` 不进入确认分账接口，不能改变分账接收方；它只影响微信渠道支付配置。商户 `merchant_report` 成功后必须为该商户 `subMchId` 绑定 LocalLife 平台小程序 appid。若 `bind_sub_config` 失败，readiness 保持不可支付，风险首先是 `unified_order` 后续无法在平台小程序中正常拉起微信支付或展示主体不符合预期，而不是 `share_after_pay` 无法按宝付二级户分账。

## 8. 聚合支付详细核对

### 8.1 统一下单 `unified_order`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qlvjef634j

用途：主支付入口。微信 JSAPI 场景下返回 `chlRetParam.wc_pay_data` 给小程序调起支付。`prodType=SHARING` 且 `orderType=7` 时，支付成功后可确认分账。

请求字段核对：

| 字段 | 必填 | 类型 | 条件/枚举 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `agentMerId/agentTerId` | 否 | S(16) | 代理场景 | C3 DTO 字段存在，首版不主动填 |
| `merId/terId` | 是 | S(16) | 收款商户号/终端号 | C3：所有请求 Validate 必填，client 不允许缺失 POST |
| `outTradeNo` | 是 | S(32) | 同商户唯一 | C3：统一下单、分账、退款、关单按方法必填/条件必填校验 |
| `txnAmt/totalAmt` | 是 | I | 分，`totalAmt=txnAmt+营销金额` | C3：正数和 `totalAmt >= txnAmt` 校验 |
| `txnTime` | 是 | T | `yyyyMMddHHmmss` | C3：统一下单、分账、退款均校验格式 |
| `timeExpire` | 否 | I | 分钟；默认 30；最大 7 天 | C2/C3：字段存在，最大值未在本地强制，待沙箱/产品确认 |
| `prodType` | 是 | E | `SHARING` | C3：`ProductTypeSharing` 常量和负向测试 |
| `orderType` | 是 | S | 宝财通2.0 必传 `7` | C3：`BaoCaiTongOrderTypeSharing` 常量和负向测试 |
| `payCode` | 是 | E | 首版 `WECHAT_JSAPI` | C3：`PayCodeWechatJSAPI` 常量和负向测试 |
| `payExtend` | 是 | C | 按支付方式选择 | C3：微信 JSAPI 的 `sub_appid/sub_openid/body` 条件必填 |
| `subMchId` | 否/条件必填 | S(64) | 微信/支付宝必传，渠道报备二级商户号 | C3，来源为报备成功且 APPLET 授权成功后的 `baofu_merchant_reports.sub_mch_id` |
| `notifyUrl` | 否 | S(128) | 支付成功服务端通知 | C3 DTO 字段存在；运行时使用配置回调 URL |
| `pageUrl` | 否 | S(128) | https 跳转地址 | C3：非空时必须 HTTPS |
| `forbidCredit` | 否 | S(1) | `1` 禁止、`0` 不禁止 | C3：只允许 `0/1` |
| `attach/reqReserved` | 否 | S(128) | 保留域 | C3 DTO 字段存在，长度待沙箱/官方错误样本确认 |
| `mktInfo` | 否 | JSON | 当前仅支持交易商户承担营销金额 | 未启用 |
| `riskInfo` | 否/条件必填 | JSON | 微信/支付宝必传 | C3：微信/支付宝 payCode 下强制 `riskInfo.clientIp` |
| `riskInfo.clientIp` | 是 | S(64) | 付款用户 IP | C3：负向测试覆盖 |
| `riskInfo.locationPoint` | 否 | S(128) | 经度,纬度 | 字段存在，未使用 |

`payExtend` 微信 JSAPI 字段：

| 字段 | 必填 | 类型 | 说明 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `sub_appid` | 是 | S(128) | 小程序或公众号 appid | C3：条件必填校验 |
| `sub_openid` | 是 | S(128) | 用户在 `sub_appid` 下的 openid | C3：条件必填校验 |
| `body` | 是 | S(128) | 商品/订单展示名 | C3：条件必填校验 |

响应字段：`resultCode/errCode/errMsg/merId/terId/outTradeNo/txnState/tradeNo/reqChlNo/payCode/chlRetParam`；`chlRetParam.wc_pay_data` 是小程序支付参数。

本地缺口：

- 已有真实 HTTP client、官方 endpoint profile、公共 envelope 本地测试和 API runtime Baofu facade wiring；不再回退普通服务商/平台收付通。
- `UnifiedOrderRequest.Validate()` 已校验首版必填字段、金额关系、`txnTime` 格式、`SHARING/orderType=7/WECHAT_JSAPI` 枚举、微信 `subMchId/payExtend/riskInfo.clientIp` 条件必填、`pageUrl=https` 和 `forbidCredit` 枚举。`PaymentQueryRequest`、`ShareQueryRequest`、`RefundQueryRequest`、`OrderCloseRequest` 均在本地校验 `merId/terId` 和交易引用，缺失不会 POST 到宝付。
- 响应验签、数字信封完整性和测试地址联调仍未完成。
- 未做测试地址联调：`https://mch-juhe.baofoo.com/api`。

### 8.2 支付订单查询

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm13po92jq

用途：支付通知缺失或处理中时查询。请求通常用 `tradeNo` 或 `outTradeNo` 定位。响应返回 `txnState`、`tradeNo`、金额、支付方式、渠道返回等。

本地覆盖：`PaymentQueryRequest` 有 `merId/terId/tradeNo/outTradeNo`，Validate 强制 `merId/terId` 与一个交易引用；aggregatepay client 已实现 `order_query`，scheduler 可查询并落 query fact。完整响应字段和测试地址联调仍待补。

### 8.3 支付结果通知

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm4ujg50cv

用途：支付异步结果。当前本地 parser 可把支付通知落 fact，并通过 fact application 更新本地支付单。

缺口：已有本地 parser/callback/fact application，但未核对官方通知公共验签/ACK 完整要求；未做宝付真实通知联调；通知 `feeAmt`、渠道原始字段等未完整进入契约。

### 8.4 交易关闭

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm0flcca1k

用途：支付单超时、上游创建后本地失败、用户取消支付时关闭订单。

当前覆盖：C2/C3。`order_close` DTO/client 已补齐；宝付统一下单成功但本地支付参数解析失败时，会先调用宝付关单再关闭本地支付单。统一下单调用本身失败且不确定上游是否建单的歧义恢复，仍需后续按查询/关单 worker 收敛。

## 9. 分账详细核对

### 9.1 确认分账 `share_after_pay`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qlvu1em0tb

用途：支付接口 `prodType=SHARING` 且 `orderType=7` 支付完成后分账。同一支付订单不支持确认分账和申请退款同时发起。订单支付成功后必须在 365 天内完成分账。

字段结论：官方 `share_after_pay` 请求字段未包含 `subMchId`、`authType`、`authContent` 或微信小程序 appid。分账接收方只在 `sharingDetails[].sharingMerId` 表达，来源必须是宝财通开户返回并同步到本地的 `sharing_mer_id`。因此异主体申报/绑定授权目录不决定资金实际分给谁；它只影响微信小程序渠道支付能否在统一下单阶段合规拉起。

请求字段：

| 字段 | 必填 | 类型 | 条件/说明 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `agentMerId/agentTerId` | 否 | S(16) | 代理场景 | C3 DTO 字段存在，首版不主动填 |
| `merId/terId` | 是 | S(16) | 收款商户号/终端号 | C3：Validate 必填 |
| `originTradeNo` | 否 | S(32) | 与 `originOutTradeNo` 二选一，推荐 | C3：与 `originOutTradeNo` 二选一校验 |
| `originOutTradeNo` | 否 | S(64) | 二选一 | C3：与 `originTradeNo` 二选一校验 |
| `txnTime` | 是 | T | `yyyyMMddHHmmss` | C3：统一下单、分账、退款均校验格式 |
| `outTradeNo` | 是 | S(50) | 商户分账订单号 | C3：Validate 必填 |
| `notifyUrl` | 否 | S(128) | 分账通知地址 | C3 DTO 字段存在，运行时使用配置回调 URL |
| `sharingDetails` | 是 | JSON array | 分账明细 | C3：数组非空校验 |
| `sharingDetails[].sharingMerId` | 是 | S(64) | 宝付分配商户号/二级商户号 | C3：每个接收方必填；业务层强制来自 `sharing_mer_id` |
| `sharingDetails[].sharingAmt` | 是 | I | 分账金额，分 | C3：正数校验 |

响应字段：`resultCode/errCode/errMsg/merId/terId/tradeNo/outTradeNo/txnState/finishTime/succAmt/clearingDate`。

本地覆盖：DTO、Validate、真实 HTTP client、分账 worker、查询恢复和 fact application 已有本地测试；真实通知验签和测试地址联调仍待补。

### 9.2 分账订单查询与分账通知

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm1m0u1s68 与 https://doc.mandao.com/docs/bct/bct-1f9qm58emskkg

用途：分账处理中恢复和终态通知。状态以 `txnState` 判断，当前本地映射：`SUCCESS -> success`，`PROCESSING -> processing`，`CANCELED -> failed`，`ABNORMAL -> unknown`。

缺口：通知页内容较短，需用沙箱确认完整 payload、ACK、签名字段和 `resultCode`/`txnState` 组合；未做真实联调。

## 10. 退款详细核对

### 10.1 申请退款 `order_refund`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm06dmb1a9

用途：首版只允许分账前退款。宝付文档支持分账退款/垫资相关字段，但 LocalLife 首版不启用分账后退款。

本地覆盖：C3。已新增 `order_refund` DTO/Validate/HTTP client，并在业务层只允许分账前退款；`sharingRefundInfo` 和 `advanceAmt` 在首版请求中显式拒绝，已进入分账流程继续返回 `订单已进入结算分账流程，不支持退款`。

必须补齐的首版契约：原支付单引用、退款订单号、退款金额、退款原因/通知地址、`sharingRefundInfo`/`advanceAmt` 等分账后退款字段禁止出现在首版请求。

### 10.2 退款查询与退款通知

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm246c6cp8 与 https://doc.mandao.com/docs/bct/bct-1f9qm5hspcd9v

用途：退款处理中恢复、回调终态。

本地覆盖：C3。`refund_query` DTO/client、退款通知 parser、API callback route、订单退款 fact application 和退款查询恢复 scheduler 已补齐；真实验签/数字信封、沙箱回调证据和宝付测试地址查询证据仍需补齐。

## 11. 枚举和错误码核对

| 文档 | 地址 | 本地覆盖 | 缺口 |
| --- | --- | --- | --- |
| 账户错误码 | https://doc.mandao.com/docs/bct/bct-1fjpm4fpns79f | C3：已把官方页列出的开户参数错误、业务异常错误码、开户异步通知错误码按安全前端语义归类，并对 union-gw `retCode` 业务失败做 `ProviderError` fail-closed | 资料字段级修复指引和沙箱错误样例仍待补；C4 前不得宣称真实错误码已验证。 |
| 聚合支付错误码 | https://doc.mandao.com/docs/bct/bct-1f9qrfsj2fcbu | C3：已把官方页列出的 `INVALID_PARAMETER/SYSTEM_BUSY/UNOPENED_PRODUCT/ORDER_EXIST/MERCHANT_NOT_REPORT/RISK_REFUSED` 等错误码归类，并对聚合支付/报备 public envelope `resultCode != SUCCESS` 做 `ProviderError` fail-closed | 沙箱错误样例、响应验签/数字信封和真实渠道错误组合仍待补。 |
| 产品类型 | https://doc.mandao.com/docs/bct/bct-1f9qrdjnaqra5 | C1：`SHARING` | 未限制其他产品类型。 |
| 支付方式 | https://doc.mandao.com/docs/bct/bct-1f9qrdro3gtv1 | C1：`WECHAT_JSAPI` | 未建完整支付方式枚举和条件必填矩阵。 |
| 订单状态 | https://doc.mandao.com/docs/bct/bct-1f9qre51sa7dg | C2：支付/分账/退款状态局部 | 已覆盖退款 `SUCCESS/REFUND/REFUND_ERROR/ABNORMAL` 映射；关闭状态仍主要依赖支付状态，未知组合进入 `unknown`。 |
| 支付属性 | https://doc.mandao.com/docs/bct/bct-1f9qrefjkin2b | C1：微信 JSAPI 三字段 | 未覆盖公共参数禁传条件、支付宝/云闪付等非首版支付方式。 |
| 聚合商户报备附录 | https://doc.mandao.com/docs/bct/bct-1f9o6qi1pf2r8 | C3：报备类型、报备状态、终端设备、操作标识、设备状态、微信/支付宝服务类型、联系人业务标识、微信/支付宝证件类型、支付宝联系人类型、授权类型、站点类型、间连等级、商户状态、交易控制位、认证订单状态、商户认证状态和 110 项微信经营类目 allowlist 已建 | 仍需把长尾枚举接入未来新增字段的 DTO 校验；当前首版未使用字段必须保持不进入请求体。 |

### 11.1 聚合商户报备附录枚举覆盖状态

结论：首版微信小程序路径已把关键报备类型、报备状态、微信服务类型、微信证件类型、授权类型和微信经营类目 allowlist 落入项目契约层；本地已补齐当前审计识别出的终端/联系人/认证/商户状态等附录枚举 typed constants 和 allowlist 测试。生产前仍需确保未来新增字段全部复用这些校验，未使用字段继续禁止进入请求体。

| 附录/下载文件 | 来源 | 已识别内容 | 当前项目覆盖 | 生产前要求 |
| --- | --- | --- | --- | --- |
| 报备附录枚举 | `bct-1f9o6qi1pf2r8` | 签名类型、报备类型、报备状态、终端设备类型、操作标识、设备状态、微信服务类型、支付宝服务类型、联系人业务标识、微信证件类型、支付宝证件类型、联系人类型、授权类型、站点类型、间连等级、商户状态、交易控制位、认证订单状态、商户认证状态 | C3，已覆盖 `WECHAT`、报备状态、`APPLET`、微信证件类型、授权类型以及当前审计识别出的终端设备、联系人业务标识、商户状态、交易控制位、认证状态等长尾枚举 | 未来新增字段必须复用 allowlist；未使用字段进入请求前必须 fail-closed。 |
| 经营类目/MCC | `/home/sam/文档/分账/宝付/经营类目&MCC.xlsx`，SHA256 `c521b7b15397a5aa63be9a3d8297c8a8c207e68e7d7fea7a26f8450945b4793f` | `微信经营类目` sheet：110 条，字段为 `类目值/类目名称`；`支付宝MCC` sheet：368 条，字段为 `MCC/经营类目一级/经营类目二级/经营类目三级/特殊资质` | C3：微信经营类目已生成 allowlist 并用 hash/行数/非法值测试锁定；支付宝 MCC 暂缓 | xlsx 更新时必须更新 hash 和生成物；如启用支付宝报备，再抽取支付宝 MCC。 |
| 聚合支付枚举 | `产品类型`、`支付方式`、`订单状态`、`支付属性` 附录 | `SHARING`、`WECHAT_JSAPI`、支付/退款/分账订单状态、微信 JSAPI 支付属性等 | C2/C3，首版常量和状态映射已覆盖支付/分账/退款主状态 | 已显式拒绝非 `SHARING`、非 `orderType=7`、非 `WECHAT_JSAPI`；支付/分账/退款状态分别映射，未知值进入 `unknown`，错误码细分仍留 Task 10。 |
| 错误码 | 账户错误码、聚合支付错误码、报备错误码 | 参数错误、身份/银行卡核验失败、系统繁忙、商户未报备、分账配置不存在、订单重复、交易未知、风控拒绝等 | C3，本地 typed classification 已覆盖官方账户错误码页和聚合支付错误码页，报备接口按 public envelope `resultCode/errCode/errMsg` 统一进入分类器 | 已区分资料需修改、平台配置错误、可重试/可查询处理中、渠道/宝付异常需人工；沙箱错误样例和真实渠道组合仍待补。 |


### 11.2 聚合商户报备附录枚举明细

以下枚举来自 `bct-1f9o6qi1pf2r8`，当前项目没有完整 typed constants。首版契约实现时，不允许只写散落字符串；必须按用途分组建常量、校验函数和表驱动测试。

| 枚举组 | 官方枚举值 | 首版处理要求 |
| --- | --- | --- |
| 签名类型 | `SM2`、`RSA` | 公共报文层必须显式配置并校验，不允许空值或隐式默认漂移。 |
| 报备类型 | `WECHAT`、`ALIPAY` | 首版微信小程序只允许 `WECHAT`；如保留支付宝 DTO，必须同样建校验但不得误接入主支付路径。 |
| 报备状态 | `PROCESSING`、`SUCCESS`、`FAIL` | 映射为本地 processing/succeeded/failed；未知值进入 `unknown/needs_manual_review`。 |
| 终端设备类型 | `01`、`02`、`03`、`04`、`05`、`06`、`07`、`08`、`09`、`10`、`11`、`12`、`13` | 若报备请求涉及终端信息，必须从 allowlist 选择；首版未使用时也要禁止随意传值。 |
| 操作标识 | `00` 新增、`01` 修改、`02` 注销 | 报备新增、后续修改/注销接口不能混用。 |
| 设备状态 | `00` 启用、`01` 注销 | 设备上报时校验。 |
| 微信服务类型 | `JSAPI`、`APPLET`、`MICROPAY` | 小程序支付首版应使用/允许 `APPLET`，如文档要求 `service_codes` 同时含 `JSAPI` 需以沙箱验证为准固化测试。 |
| 支付宝服务类型 | `F2F` | 首版不启用支付宝主路径时不得影响微信校验。 |
| 联系人业务标识 | `02`、`06`、`08`、`11` | 报备联系人数组必须校验业务标识；普通用户错误提示只能说资料不完整/需补充。 |
| 微信证件类型 | `NATIONAL_LEGAL`、`NATIONAL_LEGAL_MERGE`、`INST_RGST_CTF`、`IDENTITY_CARD`、`OTHERS` | 机构/个体/个人报备资料映射必须显式；不能复用微信进件旧枚举名而不转换。 |
| 支付宝证件类型 | `NATIONAL_LEGAL`、`NATIONAL_LEGAL_MERGE`、`INST_RGST_CTF` | 同上，若支付宝暂缓则保持 C0/C1 不接生产。 |
| 支付宝联系人类型 | `LEGAL_PERSON`、`CONTROLLER`、`AGENT`、`OTHER` | 支付宝暂缓时仅记录，不能误作为微信联系人类型。 |
| 授权类型 | `AUTH`、`JSAPI`、`APPLET` | 本项目使用 `APPLET` + LocalLife 小程序 appid，宝付已确认支持异主体报备；该状态只 gate 统一下单，不参与分账。 |
| 站点类型 | `01`、`02`、`03`、`04`、`05`、`06` | 只有报备字段需要时启用 allowlist。 |
| 间连等级 | `INDIRECT_LEVEL_M1`、`INDIRECT_LEVEL_M2`、`INDIRECT_LEVEL_M3`、`INDIRECT_LEVEL_M4` | 如宝付要求代理/间连等级，必须配置化并测试。 |
| 商户状态 | `00` 启用、`01` 注销 | 报备查询返回注销时必须阻断支付 readiness。 |
| 交易控制位 | `00`、`01` | 未明确业务需要前不应由业务层随意设置。 |
| 认证订单状态 | `AUDITING`、`CONTACT_CONFIRM`、`LEGAL_CONFIRM`、`AUDIT_PASS`、`AUDIT_REJECT`、`AUDIT_FREEZE`、`CANCELED`、`CONTACT_PROCESSING` | 这些状态解释了“可能需要联系人/法人确认”的流程，但文档未给出拓展码生成接口；需要宝付确认扫码/确认动作来源。 |
| 商户认证状态 | `AUTHORIZED`、`UNAUTHORIZED`、`CLOSED`、`SMID_NOT_EXIST` | 支付 readiness 必须 fail-closed；`UNAUTHORIZED` 不能视为可支付。 |

### 11.3 经营类目/MCC 文件防漂移要求

`经营类目&MCC.xlsx` 不适合手工复制到代码里维护。生产前建议生成项目内来源可追踪的 allowlist，例如 `locallife/baofu/merchantreport/contracts/categories_generated.go` 或只读数据表，并在测试中固定来源 hash。当前本地文件核验结果：

| 文件 | SHA256 | Sheet | 行数 | 字段 | 首版要求 |
| --- | --- | --- | --- | --- | --- |
| `/home/sam/文档/分账/宝付/经营类目&MCC.xlsx` | `c521b7b15397a5aa63be9a3d8297c8a8c207e68e7d7fea7a26f8450945b4793f` | `微信经营类目` | 110 | `类目值`、`类目名称` | 微信报备 `business` 必须来自该 allowlist；xlsx 更新时必须更新 hash 和生成物。 |
| 同上 | 同上 | `支付宝MCC` | 368 | `MCC`、`经营类目一级`、`经营类目二级`、`经营类目三级`、`特殊资质` | 支付宝首版不启用时可暂缓生成，但不能把支付宝 MCC 当微信经营类目使用。 |

漂移门禁：生成物测试必须校验行数、hash、关键样例值、空值、重复值；报备契约测试必须验证非法经营类目被拒绝。


## 12. 当前项目代码覆盖清单

| 路径 | 现状 | 主要缺口 |
| --- | --- | --- |
| `locallife/baofu/config.go` | 区分收款/支付商户号；已拆官方三组 endpoint profile 并拒绝 `https://api.baofoo.com` 占位地址 | 仍需用真实宝付测试地址证明每个 client 命中正确 endpoint。 |
| `locallife/baofu/uniongw.go` | 账户请求 union-gw `verifyType=1` 官方 URL 参数、`content` 密文、`header/body` envelope 和响应 `sysRespCode` 校验；账户通知复用 RSA/base64 解码规则 | `verifyType=2`、真实沙箱和完整系统错误码仍待补。 |
| `locallife/baofu/crypto/uniongw.go` | 早期本地 AES-GCM envelope 草稿 | 不再接入运行时账户通知；不得作为账户请求或账户通知官方 envelope。 |
| `locallife/baofu/aggregatepay/contracts/types.go` | 统一下单、查询、分账、退款、关单 DTO | 退款/关单已补 DTO 和首版互斥；错误码分类、沙箱 fixture 仍待补。 |
| `locallife/baofu/account/contracts` | 业务归一化 DTO + 官方开户/查询/余额/提现 DTO | 企业/个体完整资料映射、资质附件、账户错误码表和沙箱样例仍待补。 |
| `locallife/baofu/aggregatepay/client.go` | interface + concrete HTTP client | 已覆盖支付/查询/分账/退款/关单；未做沙箱验证和响应验签/数字信封完整验证。 |
| `locallife/baofu/account/client.go` | concrete account client，覆盖开户/查询/余额/提现/提现查询，并已切到 union-gw `verifyType=1` 请求/响应 envelope | `verifyType=2`、沙箱证据和真实错误码仍待补。 |
| `locallife/baofu/merchantreport/**` | 报备/查询/APPLET 绑定 contracts + concrete client + tests | 当前审计识别的附录枚举和报备恢复 worker 已补；真实资料来源映射和沙箱证据仍待补。 |
| `locallife/logic/baofu_payment_service.go` | 服务层可组统一下单并记录 command；主业务 API runtime 可切宝付 concrete aggregate client | 宝付合单支付已 fail-closed；沙箱证据仍待补。 |
| `locallife/api/logic_adapters.go` | 已按 `BAOFU_MAIN_BUSINESS_ENABLED` 构造宝付主业务 facade | 主业务支付可切宝付；宝付启用时合单支付已明确 fail-closed。 |
| `locallife/api/baofu_callback.go` | 支付/分账/退款/开户回调落 fact 草稿 | ACK、验签、payload 完整性需沙箱确认。 |
| `locallife/worker/*baofu*` | 分账创建、支付/分账/退款查询恢复、提现查询恢复、提现 fact application、聚合商户报备查询恢复等本地 worker 边界 | aggregatepay/account/merchantreport 生产 client wiring 已接入任务处理器和 Baofu/refund/withdrawal/merchant-report recovery scheduler；沙箱证据仍待补。 |


### 12.1 契约漂移审计 Findings

| 编号 | 严重度 | 证据 | 风险 | 整改要求 |
| --- | --- | --- | --- | --- |
| C-001 | 已本地修复，待真实 client 使用验证 | `locallife/baofu/config.go` 已拆 `AccountGatewayBaseURL`、`AggregatePayBaseURL`、`MerchantReportBaseURL` 并拒绝 `https://api.baofoo.com` | 配置层已防止三组入口混用；真实 client 仍必须逐接口读取对应 endpoint | Task 8 接真实 HTTP 时验证每个 client 使用正确 endpoint。 |
| C-002 | C3 本地修复，待沙箱 | 已新增官方开户、查询、余额、提现 DTO 与金额元/分转换测试；account client 已将业务抽象转换为官方 DTO 后放入 union-gw `body` | 原 `OpenAccountRequest` 业务抽象仍保留给现有 service；企业/个体完整资料映射和沙箱样例仍待补 | Task 12/C4 用真实测试地址证明字段和错误码。 |
| C-003 | C3 本地修复，待沙箱验证 | `locallife/baofu/account/notification/notification.go` 已按官方开户/提现通知字段解析并覆盖提现密文 envelope；`locallife/api/baofu_callback.go` 开户/提现 ACK 均为纯文本 `OK` | 本地 parser/ACK/提现入队已防止明显字段漂移，但还未使用宝付测试环境真实通知验证 URL query、密文 envelope、重放 ACK 行为 | 用宝付测试通知样例或沙箱回调验证开户/提现通知，并补回真实样例摘要。 |
| C-004 | C3 本地修复，部分负向沙箱 smoke | 已新增聚合/报备 `PublicRequestEnvelope`；账户 API 已拆到独立 union-gw `verifyType=1` envelope；`order_query` 负向 smoke 已证明聚合请求 `bizContent` 与响应 `dataContent` 解析可通过宝付测试地址 | 聚合/报备响应验签、数字信封加密、账户 `verifyType=2`、真实业务成功 `dataContent`、真实错误码和回调仍未验证 | C4 补真实成功请求/响应、回调和错误样例；不得因负向 smoke 宣称支付/分账已联调完成。 |
| C-005 | 已本地修复，待沙箱验证 | `locallife/baofu/aggregatepay/contracts/types.go` 已新增 `unified_order` 表驱动校验，覆盖 M/C 字段、枚举、`subMchId` 条件必填、https `pageUrl`、金额关系 | 本地契约已 fail-closed；仍未用宝付测试地址验证真实错误码和字段长度边界 | Task 8/沙箱测试补真实请求、参数错误和错误码样例。 |
| C-006 | C3 本地修复，待联调 | 已新增 `locallife/baofu/merchantreport/contracts`、`baofu_merchant_reports`、sqlc query、报备 service、APPLET 授权 readiness、`merchantreport.Client` 和 `baofu-merchant-report-recovery` | 本地契约/持久化/服务/HTTP client/recovery 已能防止 `bctMerId/subMchId/authContent` 漂移，但还没有沙箱证据和真实资料映射验收 | Task 8/沙箱联调验证真实报备、报备查询与授权目录请求。 |
| C-007 | 已本地修复，待联调 | `locallife/logic/baofu_payment_order_route.go` 已通过 `merchantBaofuReadinessForPayment` 取商户报备 `sub_mch_id`；`CreateBaofuWechatJSAPIOrderInput` 字段已改 `MerchantSubMchID` | 旧普通服务商 `txResult.SubMchID` 来源已移除，API/logic readiness 不再读取 `baofu_account_bindings.wechat_sub_mch_id`；真实支付仍待宝付聚合商户报备沙箱验证 | Task 8/沙箱测试验证 `unified_order.subMchId` 使用报备返回值。 |
| C-008 | C3 本地修复，待沙箱 | 已新增 `order_refund`、`refund_query`、`order_close` DTO/client、退款通知 parser、退款状态映射、API callback、退款查询恢复 scheduler 和分账前退款业务接入 | 首版“分账前退款、分账后不退款”已在本地 fail-closed；仍缺真实验签/数字信封、宝付测试地址退款查询/回调证据 | Task 12/C4 补沙箱证据；响应验签/数字信封在 C-004/C-011 跟进。 |
| C-009 | 部分修复，待 service/client 切换 | 已新增 `YuanStringToFen` / `FenToYuanString` 并覆盖 2 位小数校验 | 转换 helper 已在契约包，余额/提现真实 client 仍需集中调用 | Task 8/提现服务切换时禁止业务层散落金额转换。 |
| C-010 | C3 本地修复，待沙箱样例 | `locallife/baofu/errors.go` 已覆盖官方账户错误码页和聚合支付错误码页；`locallife/baofu/client.go` 已在账户 `retCode` 失败、聚合支付/报备 `resultCode != SUCCESS` 时返回 `ProviderError`，并保留上游原文只在 provider error/log 边界 | 前端可获得安全中文语义，不暴露上游原文、证件、银行卡、手机号、`contractNo`、`sharingMerId`、`subMchId` 或 raw payload；沙箱错误样例和真实渠道组合仍待补 | Task 12/C4 补真实错误样例、API handler 边界日志验证和 evidence。 |
| C-011 | C3 局部，部分负向沙箱 smoke | `locallife/baofu/account/client.go` 已用 union-gw 官方 URL/query/encrypted content；`aggregatepay`、`merchantreport` concrete HTTP client 已有；账户查询和聚合 `order_query` 已打到测试地址并解析脱敏响应；主业务 API runtime/worker/scheduler 已防止宝付启用时回退普通服务商 | 账户开户/余额/提现、聚合商户报备/授权目录、统一下单/分账/退款/回调的正向沙箱证据仍缺；聚合/报备响应验签和数字信封完整验证仍待补 | C4 前继续补正向沙箱证据、回调证据和真实错误样例。 |
| C-012 | 已本地修复，待产品验收 | `merchantBaofuReadinessForPayment` 已返回“商户微信支付通道待开通，暂不能创建微信生态支付订单”，API runtime 切换测试覆盖主业务宝付与直连支付边界 | 旧“微信特约商户进件”语义已移除；仍需前端/运营最终文案验收 | 沙箱联调和上线前检查中继续确认用户侧只看到产品语义，不暴露 report/auth/subMchId 内部细节。 |

### 12.2 防漂移实现门禁

以下门禁用于后续代码实现和 review，不满足时不能把接口标记为 C3/C4：

1. 每个官方接口必须有独立 `Method`/报文编号常量、endpoint profile、官方字段级请求/响应 DTO、字段校验、状态映射、错误码分类和表驱动测试。
2. 所有条件必填必须以测试锁住：微信/支付宝 `riskInfo.clientIp`、微信/支付宝 `subMchId`、`payExtend` 按 `payCode` 的字段矩阵、`share_after_pay` 的原支付单二选一、企业/个体开户资料差异。
3. 公共报文 envelope 与业务 `bizContent` 分层实现；业务层不得直接拼接 `method`、签名串、数字信封或上游原始 JSON。
4. `sharing_mer_id` 只能由开户/查询/通知解析层的“宝付二级商户号”字段写入；任何 `subMchId/openid/collect merchant id/contract_no` 写入都必须有失败测试。
5. `subMchId` 只能由聚合商户报备成功结果写入支付通道 readiness；支付创建只读取“最终选定 subMchId”，不得从普通服务商 `txResult.SubMchID` 继承。
6. 回调 parser 必须用官方字段名和 ACK 形态测试；重复通知必须先落 fact 再幂等应用。
7. 错误日志只在一个边界记录上游代码、接口名、脱敏流水号和内部原因；前端只收到安全中文语义，不暴露 `reportNo/bctMerId/subMchId/sharingMerId/contractNo` 原值、证件、银行卡、手机号、签名或 raw payload。
8. 沙箱联调完成后把请求摘要、响应摘要、回调样例、查询恢复样例、错误样例和测试流水号补回本审计文档；没有证据仍标记“未做宝付测试地址联调”。

## 13. 测试地址联调状态

| 能力 | 测试地址 | 是否已联调 | 证据 |
| --- | --- | --- | --- |
| 账户 union-gw | `https://vgw.baofoo.com/union-gw/api/{报文编号}/transReq.do` | 部分 | `T-1001-013-03` 账户查询已有负向 smoke，证明测试地址可达和 union-gw 响应可解析；开户/余额/提现/提现查询正向证据仍缺。 |
| 聚合商户报备 | `https://mch-juhe.baofoo.com/mch-service/api` | 否 | 已有 contracts/client/service/httptest；尚未打宝付测试地址。 |
| 聚合支付/分账/退款 | `https://mch-juhe.baofoo.com/api` | 部分 | `order_query` 负向 smoke 已证明 public envelope、S(10) serial、上海时间戳和响应 `dataContent` 解析；统一下单、支付回调、分账、退款正向证据仍缺。 |
| 回调通知 | 本平台 webhook URL | 否 | 仅本地 fake parser/callback 测试。 |

生产前必须形成沙箱证据：请求报文摘要、响应摘要、回调样例、查询补偿样例、错误样例、对应测试订单/流水号、测试时间、测试账号、代码 commit。

## 14. 高优先级整改任务

1. 补 union-gw 官方 envelope/数字信封/响应验签：确认账户接口是否必须使用 `verifyType/content/veryfyString` 形态，不能把聚合公共 envelope 误用到宝财通账户 API。
2. 宝付合单支付首版已选择 fail-closed：`BAOFU_MAIN_BUSINESS_ENABLED=true` 时创建合单支付返回 `宝付合单支付暂未开通，请分开支付`，不得回退普通服务商/平台收付通；后续如需合单再按宝付官方合单/多单契约新增。
3. 用宝付测试地址补退款 callback / `refund_query` 证据，并继续保持“分账前退款、分账后不退款”互斥。
4. 补沙箱错误样例：账户、聚合支付、聚合商户报备至少各保留一条参数/配置/处理中或渠道异常样例，验证本地分类器和 API 安全文案不漂移。
5. 补聚合商户报备真实资料来源映射；附录枚举和报备查询恢复 worker 已本地完成，未覆盖字段仍需在请求进入 client 前 fail-closed。
6. 每个必用接口至少跑一次宝付测试地址正向、参数错误、重复请求/幂等、查询恢复、回调重复投递，并把脱敏证据写入 `baofu-sandbox-evidence.md`。
7. 向宝付确认拓展码/扫码确认的接口归属、字段、扫码主体和状态查询方式，并把结果补入开户/报备契约。


## 15. 本次审计使用的官方页面

- 账户请求入口：https://doc.mandao.com/docs/bct/unionGw
- 个人/机构开户：https://doc.mandao.com/docs/bct/openAcc
- 个人开户二要素：https://doc.mandao.com/docs/bct/bct-1gj4ccsdha6d8
- 开户查询：https://doc.mandao.com/docs/bct/queryAcc
- 开户结果通知：https://doc.mandao.com/docs/bct/openAccNotify
- 余额查询：https://doc.mandao.com/docs/bct/queryBalace
- 提现：https://doc.mandao.com/docs/bct/accWithdrawal
- 提现查询：https://doc.mandao.com/docs/bct/queryWithdrawal
- 提现通知：https://doc.mandao.com/docs/bct/withdrawNotify
- 聚合商户报备入口：https://doc.mandao.com/docs/bct/bct-1f9o5s1lqlean
- 报备认证：https://doc.mandao.com/docs/bct/bct-1f9o62bulbiqd
- 报备信息查询：https://doc.mandao.com/docs/bct/bct-1f9o63b6ufii5
- 聚合支付入口：https://doc.mandao.com/docs/bct/bct-1f9qhakcna6te
- 统一下单：https://doc.mandao.com/docs/bct/bct-1f9qlvjef634j
- 支付属性：https://doc.mandao.com/docs/bct/bct-1f9qrefjkin2b
- 支付查询：https://doc.mandao.com/docs/bct/bct-1f9qm13po92jq
- 支付通知：https://doc.mandao.com/docs/bct/bct-1f9qm4ujg50cv
- 确认分账：https://doc.mandao.com/docs/bct/bct-1f9qlvu1em0tb
- 分账查询：https://doc.mandao.com/docs/bct/bct-1f9qm1m0u1s68
- 分账通知：https://doc.mandao.com/docs/bct/bct-1f9qm58emskkg
- 申请退款：https://doc.mandao.com/docs/bct/bct-1f9qm06dmb1a9
- 退款查询：https://doc.mandao.com/docs/bct/bct-1f9qm246c6cp8
- 退款通知：https://doc.mandao.com/docs/bct/bct-1f9qm5hspcd9v
- 交易关闭：https://doc.mandao.com/docs/bct/bct-1f9qm0flcca1k
- 聚合支付错误码：https://doc.mandao.com/docs/bct/bct-1f9qrfsj2fcbu
- 账户错误码：https://doc.mandao.com/docs/bct/bct-1fjpm4fpns79f
