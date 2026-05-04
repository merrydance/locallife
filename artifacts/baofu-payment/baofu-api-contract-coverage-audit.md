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

当前全局判断：项目内宝付代码最高只到 C2；没有任何接口达到 C4。


## 2.1 本次补充审计结论

- 已确认 `subMchId` 策略：采用商户逐户聚合商户报备 + 平台小程序异主体授权目录绑定。代码不得再固化为“平台统一下单”，也不得复用项目内微信普通服务商特约商户进件结果。
- `share_after_pay` 官方字段不包含 `subMchId`。分账接收方仍必须是宝财通开户返回并同步到本地 `sharing_mer_id` 的宝付二级商户号。
- 当前契约层存在明显漂移风险：多个 DTO 是业务抽象或自造通知结构，不是官方字段级 DTO；聚合支付公共报文、聚合商户报备、退款、关单、错误码和附录枚举未落入契约层。
- “防漂移”的实现标准不能只靠文档：必须把官方必填/条件必填、字段类型长度、枚举、错误码、金额单位、回调 ACK 形态、测试/生产 endpoint 都变成 typed constants、校验器和表驱动测试。

## 3. 官方入口地址

| 能力组 | 官方文档 | 请求方式 | 测试地址 | 生产地址 | 生产备用地址 | 当前项目覆盖 |
| --- | --- | --- | --- | --- | --- | --- |
| 宝财通账户 API / union-gw | https://doc.mandao.com/docs/bct/unionGw | POST | `https://vgw.baofoo.com/union-gw/api/{报文编号}/transReq.do` | `https://public.baofu.com/union-gw/api/{报文编号}/transReq.do` | 未见 | C1：`locallife/baofu/crypto` 有本地 envelope 测试；已拆出官方 endpoint profile，但 union-gw 真实 HTTP client 未完成。 |
| 宝付聚合支付 | https://doc.mandao.com/docs/bct/bct-1f9qhakcna6te | POST | `https://mch-juhe.baofoo.com/api` | `https://juhe.baofoo.com/api` | `https://juhe-backup.baofoo.com/api` | C1/C2：有 contracts/service/mock；无真实 HTTP client；API runtime 仍硬编码普通服务商 facade；微信直连支付不在宝付替换范围。 |
| 聚合商户报备 | https://doc.mandao.com/docs/bct/bct-1f9o5s1lqlean | POST | `https://mch-juhe.baofoo.com/mch-service/api` | `https://juhe.baofoo.com/mch-service/api` | 未见 | C0：已列入计划 Task 3A，尚无代码。 |

公共报文要求：

- 账户 union-gw：URL 带 `memberId`、`terminalId`、`verifyType`、`content`、条件字段 `veryfyString`；body 明文由 `header` + `body` 组成，`header.serviceTp` 必须与 URL 报文编号一致。
- 聚合支付/聚合商户报备：公共字段包括 `merId`、`terId`、`method`、`charset=UTF-8`、`version=1.0`、`format=json`、`timestamp`、`signType`、`signSn`、`ncrptnSn`、`dgtlEnvlp`、`signStr`、`bizContent`。
- 当前项目没有把三组入口拆成独立 endpoint 配置，也没有验证官方测试/生产地址。

## 4. 必用接口总览

| 能力 | 接口 | 官方方法/报文编号 | 用途 | 本地契约 | 本地运行时 | 沙箱联调 |
| --- | --- | --- | --- | --- | --- | --- |
| 账户开户 | 个人/机构开户 | `T-1001-013-01` | 商户、骑手、运营商、平台二级户开户 | C1：`OpenAccountRequest` 过于抽象，未覆盖完整 `accInfo` | C2：有 service command 记录，但无真实 client | 未做 |
| 账户开户 | 个人开户二要素 | `T-1001-013-01` 变体 | 骑手个人二要素开户候选 | C0/C1：未区分四要素/二要素 | 未接 | 未做 |
| 账户查询 | 开户查询 | `T-1001-013-03` | 同步 `contractNo`、二级商户号/状态 | C1：只有 `QueryAccountRequest` | 未完整接真实查询 | 未做 |
| 开户通知 | 开户结果通知 | 通知 URL | 开户异步结果 | C1：notification parser 有本地测试 | C2：callback 落 fact | 未做 |
| 余额 | 账户余额查询 | `T-1001-013-06` | 二级户在途/可用/冻结余额 | C1：有 `BalanceQueryRequest/Result` | C2：withdraw service 草稿 | 未做 |
| 提现 | 账户提现 | `T-1001-013-14` | 二级户可用余额提现到银行卡 | C1：有 DTO | C2：service/worker 草稿 | 未做 |
| 提现查询 | 提现查询 | `T-1001-013-15` | 提现处理中恢复/对账 | C1：有 DTO | C2：worker 应用状态 | 未做 |
| 提现通知 | 提现结果通知 | 通知 URL | 提现终态通知 | C1：notification parser 局部 | 未完整 callback route | 未做 |
| 聚合商户报备 | 报备认证 | `merchant_report` | 商户宝付开户后逐户报备微信渠道，取得该商户 `subMchId` | C0 | C0 | 未做 |
| 聚合商户报备 | 报备信息查询 | `merchant_report_query` | 查询平台或商户报备状态和 `subMchId` | C0 | C0 | 未做 |
| 聚合支付 | 统一下单交易创建 | `unified_order` | 主支付入口，微信 JSAPI 支付 | C1/C2：有 DTO/service；刚补 `riskInfo.clientIp` 校验 | C2：无真实 HTTP，API runtime 未切宝付 | 未做 |
| 聚合支付 | 支付订单查询 | `order_query` | 支付回调缺失恢复 | C1：有 `PaymentQueryRequest` | C2：scheduler 可落 query fact | 未做 |
| 聚合支付 | 支付结果通知 | 通知 URL | 支付终态回调 | C1：notification parser 有本地测试 | C2：callback 落 fact | 未做 |
| 确认分账 | 确认分账 | `share_after_pay` | 支付成功后按 `sharingMerId` 分账 | C1/C2：DTO/Validate/worker | C2：无真实 HTTP | 未做 |
| 分账查询 | 分账订单查询 | `share_query` | 分账处理中恢复 | C1/C2：DTO/scheduler | C2：无真实 HTTP | 未做 |
| 分账通知 | 分账结果通知 | 通知 URL | 分账终态回调 | C1/C2：parser/callback | C2 | 未做 |
| 退款 | 申请退款 | `order_refund` | 首版仅分账前退款 | C0：未建 DTO | C0：当前只做本地分账后禁退 | 未做 |
| 退款查询 | 退款订单查询 | `refund_query` | 退款恢复/对账 | C0 | C0 | 未做 |
| 退款通知 | 退款结果通知 | 通知 URL | 退款终态回调 | C0 | C0 | 未做 |
| 关单 | 交易关闭 | `order_close` | 上游支付失败/本地关闭时关单 | C0 | C0 | 未做 |

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
| 绑定授权目录 | https://doc.mandao.com/docs/bct/bct-1f9o63qmkndkc | 微信小程序支付上线前必须执行 | `authType=APPLET`、`authContent=<LocalLife 小程序 appid>`；每个完成聚合商户报备的商户 `subMchId` 都绑定平台小程序；不影响 `share_after_pay` 分账接收方 | C0 |

## 6. 账户 API 详细核对

### 6.1 个人/机构开户 `T-1001-013-01`

官方文档：https://doc.mandao.com/docs/bct/openAcc

用途：开通宝付账簿二级户。开户成功后存在在途户和可用余额户；分账先入在途户，结算后进入可用余额户。

请求结构摘要：

| 字段 | 必填 | 类型/长度 | 条件/枚举 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `version` | M | String(5) | 文档为 `4.1.0` | 未显式建模 |
| `accType` | M | int(1) | `1` 个人，`2` 企业/个体 | `AccountType` 抽象，不等同官方字段 |
| `accInfo` | M | Object | 根据 `accType` 不同 | 未完整建模 |
| `noticeUrl` | M | String(256) | 开户通知地址 | 未完整 DTO |
| `businessType` | M | String(32) | `BCT2.0` | 未显式校验 |
| `accInfo.transSerialNo` | M | String(200) | 请求流水号 | `OutRequestNo` 类似但名称不一致 |
| `accInfo.loginNo` | M | String(32) | 全局唯一，长度 11 位以上 | 未校验长度 |
| 个人：`customerName/certificateType/certificateNo/cardNo/mobileNo/cardUserName/needUploadFile` | M | String/boolean | `certificateType=ID`；二要素接口无 `cardNo/mobileNo` | 未完整覆盖 |
| 企业/个体：`email/selfEmployed/customerName/certificateNo/certificateType/corporateName/corporateCertType/corporateCertId/industryId/cardNo/bankName/depositBankProvince/depositBankCity/depositBankName` | M/C/O | 多类型 | `selfEmployed=true` 且绑定对私卡时 `corporateMobile` 必传；证件类型有多枚举 | 未覆盖完整字段和条件必填 |
| `platformNo/platformTerminalId/qualificationTransSerialNo` | C | String | 代理模式/上传资质时条件必填 | 未覆盖 |

响应结构摘要：`retCode`、`errorCode`、`errorMsg`、`result[]`；`result.state` 枚举 `1` 成功、`0` 失败、`-1` 异常、`2` 开户处理中；返回 `transSerialNo`、`loginNo`、`customerName`、`contractNo` 等。

本地缺口：

- `locallife/baofu/account/contracts/types.go` 的 `OpenAccountRequest` 不是官方 DTO，只是业务抽象。
- 未区分个人四要素、个人二要素、企业、个体户字段集合。
- 未把 `businessType=BCT2.0`、`version=4.1.0`、`accType`、`needUploadFile`、企业/个体条件必填落入契约校验。
- 未使用官方测试地址 `https://vgw.baofoo.com/union-gw/api/T-1001-013-01/transReq.do` 联调。

### 6.2 开户查询 `T-1001-013-03`

官方文档：https://doc.mandao.com/docs/bct/queryAcc

请求字段：`version` M；`accType` M；按条件传 `certificateNo/certificateType/platformNo/loginNo`，或传 `contractNo`。

响应字段：`retCode/errorCode/errorMsg/result`；`result.contractNo` M；其他包括 `contractName/customerName/customerNo/customerType/certificateType/certificateNo/platformNo/bindMobile/email`。

本地缺口：

- `QueryAccountRequest` 只有 `OutRequestNo/ContractNo`，未覆盖 `version`、`accType`、证件查询路径。
- 当前项目把 `contractNo` 与 `sharing_mer_id` 严格拆开是正确的，但官方查询页只明确 `contractNo`；“开户返回二级商户号写入 `sharing_mer_id`”仍需以实际开户/通知回包字段为准做沙箱验证。

### 6.3 开户结果通知

官方文档：https://doc.mandao.com/docs/bct/openAccNotify

通知要求：商户收到后返回大写 `OK`；未确认会重发。通知 URL 形如 `...?member_id=...&terminal_id=...&data_type=JSON&data_content=密文`。

通知明文字段：`member_id`、`terminal_id`、`memberType`、`state`、`errorCode`、`errorMsg`、`transSerialNo`、`loginNo`、`customerName`、`contractNo`、`noticeType`。

本地覆盖：

- `locallife/baofu/account/notification/notification.go` 有 parser；`locallife/api/baofu_callback.go` 有开户回调落 fact。
- 本地测试是自造 envelope，未使用宝付测试环境真实通知。
- 需核对 ACK 是否必须是纯文本 `OK`，当前回调 ACK 形状是否匹配宝付账户通知要求。

### 6.4 账户余额查询 `T-1001-013-06`

官方文档：https://doc.mandao.com/docs/bct/queryBalace

请求字段：`version` M，`contractNo` M，`accType` M。响应字段：`retCode/errorCode/errorMsg/availableBal/pendingBal/currBal/freezeBal`。

本地覆盖：

- `BalanceQueryRequest/BalanceResult` 有草稿，但字段使用分为单位，官方金额为元 `BigDecimal(10,2)`，需要统一转换和舍入测试。
- 未见真实 client 和测试地址联调。

### 6.5 账户提现 `T-1001-013-14`

官方文档：https://doc.mandao.com/docs/bct/accWithdrawal

请求字段：`version=4.2.0` M，`contractNo` M，`directPlatformNo` C，`transSerialNo` M，`dealAmount` M 元，`returnUrl` M，`feeMemberId` C，`reqReserved` O，`transAbstract` C。

响应字段：`retCode/errorCode/errorMsg/transSerialNo/contractNo/state/transRemark`；受理状态 `state=1` 受理成功，`2` 受理失败。

本地缺口：

- `WithdrawRequest` 使用 `AmountFen`，需要官方元金额转换校验。
- 未建模 `directPlatformNo/feeMemberId/transAbstract/reqReserved`。
- 提现应使用支付商户号/终端号；需在真实 client 中强制校验。

### 6.6 提现查询 `T-1001-013-15` 与提现通知

官方文档：https://doc.mandao.com/docs/bct/queryWithdrawal 与 https://doc.mandao.com/docs/bct/withdrawNotify

查询请求：`version=4.2.0` M，`transSerialNo` M，`tradeTime` M (`yyyy-MM-dd`)。查询响应状态：`1` 成功、`0` 失败、`2` 处理中、`3` 提现退回。

通知要求：返回大写 `OK`；通知数据包含 `contractNo/orderId/transSerialNo/transMoney/transFee/transferTotalAmount/state/transRemark/reqReserved`，`state=3` 为提现退回。

本地覆盖：

- 已有 `WithdrawStatusFromUpstream` 包含 `3 -> returned`，方向正确。
- 未完整覆盖查询请求 `tradeTime` 和通知 ACK 形态。
- 未做测试地址联调。

## 7. 聚合商户报备详细核对

### 7.1 报备认证 `merchant_report`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9o62bulbiqd

用途：微信/支付宝渠道报备；宝付技术支持已确认本项目不再需要项目内微信支付特约商户进件流程，并支持异主体报备。宝付开户成功后，主业务商户逐户调用 `merchant_report` 做微信渠道报备，成功后取得该商户用于统一下单的 `subMchId`。由于宝付确认分账不读取 `subMchId`，该接口的关键作用是提供微信小程序统一下单需要的渠道主体和顾客支付展示主体；不得把 `subMchId` 当分账接收方。

请求字段摘要：

| 字段 | 必填 | 类型 | 条件/枚举 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `agentMerId/agentTerId` | 否 | S(16) | 代理场景 | C0 |
| `merId/terId` | 是 | S(16) | 宝付交易商户号/终端号 | C0 |
| `reportType` | 是 | E | `WECHAT`/`ALIPAY`，枚举见附录 | C0 |
| `reportNo` | 是 | S(64) | 每次请求唯一 | C0 |
| `reportInfo` | 是 | C | 按报备类型传 JSON | C0 |
| `bctMerId` | 是 | S(64) | 宝财通二级商户号；主业务商户逐户报备时取商户 `sharing_mer_id` | C0 |

微信 `reportInfo` 关键字段：

| 字段 | 必填 | 类型 | 说明 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `merchant_name` | 是 | S(50) | 主体全称 | C0 |
| `merchant_shortname` | 是 | S(20) | 消费者展示名称 | C0 |
| `service_phone` | 是 | S(20) | 客服电话 | C0 |
| `contact/contact_phone/contact_email` | 否 | S | 联系信息 | C0 |
| `channel_id/channel_name` | 是 | S | 测试环境文档给固定渠道参数示例 | C0 |
| `business` | 是 | S(10) | 经营类目/MCC | C0 |
| `service_codes` | 是 | C | 如 `JSAPI`、`APPLET` | C0 |
| `address_info` | 是 | C | 省市区、地址、经纬度等 | C0 |
| `business_license/business_license_type` | 是 | S | 证照编号和类型 | C0 |
| `bankcard_info` | 是 | C | 结算卡信息 | C0 |

响应字段：`reportType/reportNo/reportState/subMchId/platformBizNo/resultCode/errCode/errMsg`；`subMchId` 在成功时有值，是微信/支付宝分配的商户识别码。该值按商户保存为商户微信渠道 `subMchId`，只作为 `unified_order.subMchId` 的来源，不写入分账接收方字段。

本地缺口：

- 无 `locallife/baofu/merchantreport` 包。
- 无报备表、报备状态机、`subMchId` 同步事务；也未在数据模型中支持商户逐户异主体报备和授权目录绑定闭环。
- 无宝付聚合商户报备资料字段映射和附件/证照处理策略。
- 未核对附录枚举：报备类型、报备状态、微信服务类型、微信证件类型、结算卡字段。
- 未做测试地址联调：`https://mch-juhe.baofoo.com/mch-service/api`。

### 7.2 报备信息查询 `merchant_report_query`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9o63b6ufii5

请求字段：`agentMerId/agentTerId` O，`merId/terId` M，`reportType` M，`reportNo` M。

响应字段：`merId/terId/reportType/reportNo/reportState`，以及渠道参数 `channelRetParam`；微信/支付宝返回中包含渠道处理结果和 `subMchId`。

本地缺口：C0。生产必须用它做报备恢复和 `subMchId` 补偿同步。

### 7.3 绑定授权目录 `bind_sub_config`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9o63qmkndkc

用途：服务商给特约子商户配置支付目录/公众号/小程序授权。宝付文档写明 `authType=APPLET` 时，`authContent` 需填写小程序 appid。本项目微信小程序支付使用 LocalLife 平台小程序 appid；宝付已确认支持异主体报备，因此每个完成聚合商户报备的商户 `subMchId` 都需要绑定 LocalLife 平台小程序 appid。

请求字段摘要：

| 字段 | 必填 | 类型 | 条件/枚举 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `agentMerId/agentTerId` | 否 | S(16) | 代理场景 | C0 |
| `merId/terId` | 是 | S(16) | 宝付交易商户号/终端号 | C0 |
| `subMchId` | 是 | S(30) | 微信/支付宝分配的商户识别码，取报备成功结果 | C0 |
| `authType` | 是 | E | `AUTH` 支付目录、`JSAPI` 公众号、`APPLET` 微信小程序 | C0 |
| `authContent` | 是 | S(256) | `APPLET` 填小程序 appid | C0 |
| `remark` | 是 | S(128) | 备注 | C0 |

边界结论：`bind_sub_config` 不进入确认分账接口，不能改变分账接收方；它只影响微信渠道支付配置。商户 `merchant_report` 成功后必须为该商户 `subMchId` 绑定 LocalLife 平台小程序 appid。若 `bind_sub_config` 失败，风险首先是 `unified_order` 后续无法在平台小程序中正常拉起微信支付或展示主体不符合预期，而不是 `share_after_pay` 无法按宝付二级户分账。

## 8. 聚合支付详细核对

### 8.1 统一下单 `unified_order`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qlvjef634j

用途：主支付入口。微信 JSAPI 场景下返回 `chlRetParam.wc_pay_data` 给小程序调起支付。`prodType=SHARING` 且 `orderType=7` 时，支付成功后可确认分账。

请求字段核对：

| 字段 | 必填 | 类型 | 条件/枚举 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `agentMerId/agentTerId` | 否 | S(16) | 代理场景 | 未覆盖 |
| `merId/terId` | 是 | S(16) | 收款商户号/终端号 | C1，已建字段 |
| `outTradeNo` | 是 | S(32) | 同商户唯一 | C1 |
| `txnAmt/totalAmt` | 是 | I | 分，`totalAmt=txnAmt+营销金额` | C1 |
| `txnTime` | 是 | T | `yyyyMMddHHmmss` | C1 |
| `timeExpire` | 否 | I | 分钟；默认 30；最大 7 天 | C1 |
| `prodType` | 是 | E | `SHARING` | C1 常量 |
| `orderType` | 是 | S | 宝财通2.0 必传 `7` | C1 常量 |
| `payCode` | 是 | E | 首版 `WECHAT_JSAPI` | C1 常量 |
| `payExtend` | 是 | C | 按支付方式选择 | C1 部分 |
| `subMchId` | 否/条件必填 | S(64) | 微信/支付宝必传，渠道报备二级商户号 | C1/C2，来源待 Task 3A |
| `notifyUrl` | 否 | S(128) | 支付成功服务端通知 | C1 |
| `pageUrl` | 否 | S(128) | https 跳转地址 | C1 未校验 https |
| `forbidCredit` | 否 | S(1) | `1` 禁止、`0` 不禁止 | 未覆盖 |
| `attach/reqReserved` | 否 | S(128) | 保留域 | C1 attach |
| `mktInfo` | 否 | JSON | 当前仅支持交易商户承担营销金额 | 未启用 |
| `riskInfo` | 否/条件必填 | JSON | 微信/支付宝必传 | C1，已补 `clientIp` 校验 |
| `riskInfo.clientIp` | 是 | S(64) | 付款用户 IP | C1，已强制 |
| `riskInfo.locationPoint` | 否 | S(128) | 经度,纬度 | 字段存在，未使用 |

`payExtend` 微信 JSAPI 字段：

| 字段 | 必填 | 类型 | 说明 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `sub_appid` | 是 | S(128) | 小程序或公众号 appid | C1 |
| `sub_openid` | 是 | S(128) | 用户在 `sub_appid` 下的 openid | C1 |
| `body` | 是 | S(128) | 商品/订单展示名 | C1 |

响应字段：`resultCode/errCode/errMsg/merId/terId/outTradeNo/txnState/tradeNo/reqChlNo/payCode/chlRetParam`；`chlRetParam.wc_pay_data` 是小程序支付参数。

本地缺口：

- 无真实 HTTP client；无 SM2/数字信封完整实现对聚合支付入口的覆盖。
- `UnifiedOrderRequest.Validate()` 目前只校验 `riskInfo.clientIp`，还未校验所有 M/C 字段、长度、枚举。
- API runtime 未切到宝付 facade。
- 未做测试地址联调：`https://mch-juhe.baofoo.com/api`。

### 8.2 支付订单查询

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm13po92jq

用途：支付通知缺失或处理中时查询。请求通常用 `tradeNo` 或 `outTradeNo` 定位。响应返回 `txnState`、`tradeNo`、金额、支付方式、渠道返回等。

本地覆盖：`PaymentQueryRequest` 有 `merId/terId/tradeNo/outTradeNo`；scheduler 可落 query fact。但无真实 client、无完整响应字段、无测试地址联调。

### 8.3 支付结果通知

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm4ujg50cv

用途：支付异步结果。当前本地 parser 可把支付通知落 fact，并通过 fact application 更新本地支付单。

缺口：未核对官方通知公共验签/ACK 完整要求；未做宝付真实通知联调；通知 `feeAmt`、渠道原始字段等未完整进入契约。

### 8.4 交易关闭

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm0flcca1k

用途：支付单超时、上游创建后本地失败、用户取消支付时关闭订单。

当前覆盖：C0。现有宝付支付创建失败只关闭本地支付单，未调宝付关单。

## 9. 分账详细核对

### 9.1 确认分账 `share_after_pay`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qlvu1em0tb

用途：支付接口 `prodType=SHARING` 且 `orderType=7` 支付完成后分账。同一支付订单不支持确认分账和申请退款同时发起。订单支付成功后必须在 365 天内完成分账。

字段结论：官方 `share_after_pay` 请求字段未包含 `subMchId`、`authType`、`authContent` 或微信小程序 appid。分账接收方只在 `sharingDetails[].sharingMerId` 表达，来源必须是宝财通开户返回并同步到本地的 `sharing_mer_id`。因此异主体申报/绑定授权目录不决定资金实际分给谁；它只影响微信小程序渠道支付能否在统一下单阶段合规拉起。

请求字段：

| 字段 | 必填 | 类型 | 条件/说明 | 本地覆盖 |
| --- | --- | --- | --- | --- |
| `agentMerId/agentTerId` | 否 | S(16) | 代理场景 | 未覆盖 |
| `merId/terId` | 是 | S(16) | 收款商户号/终端号 | C1 |
| `originTradeNo` | 否 | S(32) | 与 `originOutTradeNo` 二选一，推荐 | C1 |
| `originOutTradeNo` | 否 | S(64) | 二选一 | C1 |
| `txnTime` | 是 | T | `yyyyMMddHHmmss` | C1 |
| `outTradeNo` | 是 | S(50) | 商户分账订单号 | C1 |
| `notifyUrl` | 否 | S(128) | 分账通知地址 | C1 |
| `sharingDetails` | 是 | JSON array | 分账明细 | C1 |
| `sharingDetails[].sharingMerId` | 是 | S(64) | 宝付分配商户号/二级商户号 | C1，强制来自 `sharing_mer_id` |
| `sharingDetails[].sharingAmt` | 是 | I | 分账金额，分 | C1 |

响应字段：`resultCode/errCode/errMsg/merId/terId/tradeNo/outTradeNo/txnState/finishTime/succAmt/clearingDate`。

本地覆盖：DTO、Validate、分账 worker 和 fact application 已有草稿；无真实 HTTP client；无测试地址联调。

### 9.2 分账订单查询与分账通知

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm1m0u1s68 与 https://doc.mandao.com/docs/bct/bct-1f9qm58emskkg

用途：分账处理中恢复和终态通知。状态以 `txnState` 判断，当前本地映射：`SUCCESS -> success`，`PROCESSING -> processing`，`CANCELED -> failed`，`ABNORMAL -> unknown`。

缺口：通知页内容较短，需用沙箱确认完整 payload、ACK、签名字段和 `resultCode`/`txnState` 组合；未做真实联调。

## 10. 退款详细核对

### 10.1 申请退款 `order_refund`

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm06dmb1a9

用途：首版只允许分账前退款。宝付文档支持分账退款/垫资相关字段，但 LocalLife 首版不启用分账后退款。

本地覆盖：C0。当前只做了本地“分账单已创建后禁止退款”互斥，没有宝付退款 DTO、HTTP client、退款查询、退款通知。

必须补齐的首版契约：原支付单引用、退款订单号、退款金额、退款原因/通知地址、`sharingRefundInfo`/`advanceAmt` 等分账后退款字段禁止出现在首版请求。

### 10.2 退款查询与退款通知

官方文档：https://doc.mandao.com/docs/bct/bct-1f9qm246c6cp8 与 https://doc.mandao.com/docs/bct/bct-1f9qm5hspcd9v

用途：退款处理中恢复、回调终态。

本地覆盖：C0。生产前必须补齐，否则“分账前退款”链路不可用。

## 11. 枚举和错误码核对

| 文档 | 地址 | 本地覆盖 | 缺口 |
| --- | --- | --- | --- |
| 账户错误码 | https://doc.mandao.com/docs/bct/bct-1fjpm4fpns79f | C0/C1：只保留错误字符串 | 未建 typed error code、前端语义映射、可重试/需改资料分类。 |
| 聚合支付错误码 | https://doc.mandao.com/docs/bct/bct-1f9qrfsj2fcbu | C0 | 未建错误码集。 |
| 产品类型 | https://doc.mandao.com/docs/bct/bct-1f9qrdjnaqra5 | C1：`SHARING` | 未限制其他产品类型。 |
| 支付方式 | https://doc.mandao.com/docs/bct/bct-1f9qrdro3gtv1 | C1：`WECHAT_JSAPI` | 未建完整支付方式枚举和条件必填矩阵。 |
| 订单状态 | https://doc.mandao.com/docs/bct/bct-1f9qre51sa7dg | C1：支付/分账状态局部 | 未覆盖退款、关闭、异常组合。 |
| 支付属性 | https://doc.mandao.com/docs/bct/bct-1f9qrefjkin2b | C1：微信 JSAPI 三字段 | 未覆盖公共参数禁传条件、支付宝/云闪付等非首版支付方式。 |
| 聚合商户报备附录 | https://doc.mandao.com/docs/bct/bct-1f9o6qi1pf2r8 | C0 | 未建报备类型、报备状态、微信服务类型、证件类型、MCC/类目枚举。 |

### 11.1 聚合商户报备附录枚举覆盖状态

结论：当前审计文档只标出了“需要覆盖哪些附录”，没有把所有附录枚举值和下载文件常量完整落入项目契约层；因此不能把聚合商户报备契约视为完成。生产前至少要把首版会用到的微信渠道字段、枚举、类目常量做成项目内可校验的 typed constants 或 allowlist，并保留来源版本。

| 附录/下载文件 | 来源 | 已识别内容 | 当前项目覆盖 | 生产前要求 |
| --- | --- | --- | --- | --- |
| 报备附录枚举 | `bct-1f9o6qi1pf2r8` | 签名类型、报备类型、报备状态、终端设备类型、操作标识、设备状态、微信服务类型、支付宝服务类型、联系人业务标识、微信证件类型、支付宝证件类型、联系人类型、授权类型、站点类型、间连等级、商户状态、交易控制位、认证订单状态、商户认证状态 | C0，文档只列缺口 | 为 `merchant_report`、`merchant_report_query`、`bind_sub_config` 建枚举常量和校验；首版至少覆盖 `WECHAT`、`PROCESSING/SUCCESS/FAIL`、`APPLET`、`NATIONAL_LEGAL/NATIONAL_LEGAL_MERGE/IDENTITY_CARD`、联系人业务标识、商户状态和交易控制位。 |
| 经营类目/MCC | `/home/sam/文档/分账/宝付/经营类目&MCC.xlsx`，SHA256 `c521b7b15397a5aa63be9a3d8297c8a8c207e68e7d7fea7a26f8450945b4793f` | `微信经营类目` sheet：110 条，字段为 `类目值/类目名称`；`支付宝MCC` sheet：368 条，字段为 `MCC/经营类目一级/经营类目二级/经营类目三级/特殊资质` | C0，未生成代码常量或数据表 | 首版如果只做微信小程序支付，至少抽取微信经营类目 allowlist；如同时保留支付宝报备 DTO，再抽取支付宝 MCC。应在测试中校验报备 `business` 只能来自 allowlist，并在 xlsx 更新时显式更新 hash/版本。 |
| 聚合支付枚举 | `产品类型`、`支付方式`、`订单状态`、`支付属性` 附录 | `SHARING`、`WECHAT_JSAPI`、支付/退款/分账订单状态、微信 JSAPI 支付属性等 | C1，只有首版局部常量 | 首版契约要显式拒绝非 `SHARING`、非 `orderType=7`、非 `WECHAT_JSAPI`；支付状态、分账状态、退款状态分别建映射，未知值进入 `unknown/needs_manual_review`。 |
| 错误码 | 账户错误码、聚合支付错误码、报备错误码 | 参数错误、系统繁忙、商户未报备、分账配置不存在、风控拒绝等 | C0/C1，未分类 | 建 typed error code 与前端语义分类，不向普通用户暴露上游原文；至少区分资料需修改、平台配置错误、可重试处理中、渠道/宝付异常需人工。 |


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
| `locallife/baofu/config.go` | 区分收款/支付商户号；已拆官方三组 endpoint profile 并拒绝 `https://api.baofoo.com` 占位地址 | 后续真实 client 仍需逐接口使用正确 endpoint。 |
| `locallife/baofu/crypto/uniongw.go` | 本地 union-gw envelope 草稿 | 未按官方 `verifyType=1/2`、`content`、`veryfyString` 完整实现并联调。 |
| `locallife/baofu/aggregatepay/contracts/types.go` | 统一下单、查询、分账 DTO 草稿 | 未覆盖所有必填/条件必填/长度/枚举；无退款/关单 DTO。 |
| `locallife/baofu/account/contracts/types.go` | 账户/余额/提现 DTO 抽象 | 不是官方字段级 DTO；缺个人/企业/个体差异和条件必填。 |
| `locallife/baofu/aggregatepay/client.go` | 仅 interface | 无真实 HTTP transport。 |
| `locallife/baofu/account/client.go` | 仅 interface | 无真实 union-gw HTTP client。 |
| `locallife/baofu/merchantreport/**` | 不存在 | 需新增。 |
| `locallife/logic/baofu_payment_service.go` | 服务层可组统一下单并记录 command | 依赖 mock client；生产 API 未接入；仅部分校验。 |
| `locallife/api/logic_adapters.go` | 当前仍构造普通服务商支付 facade | 主业务未真正切宝付。 |
| `locallife/api/baofu_callback.go` | 支付/分账/开户回调落 fact 草稿 | ACK、验签、payload 完整性需沙箱确认。 |
| `locallife/worker/*baofu*` | 分账/恢复/提现 fact worker 草稿 | 无真实 client wiring；无宝付退款恢复。 |


### 12.1 契约漂移审计 Findings

| 编号 | 严重度 | 证据 | 风险 | 整改要求 |
| --- | --- | --- | --- | --- |
| C-001 | 已本地修复，待真实 client 使用验证 | `locallife/baofu/config.go` 已拆 `AccountGatewayBaseURL`、`AggregatePayBaseURL`、`MerchantReportBaseURL` 并拒绝 `https://api.baofoo.com` | 配置层已防止三组入口混用；真实 client 仍必须逐接口读取对应 endpoint | Task 8 接真实 HTTP 时验证每个 client 使用正确 endpoint。 |
| C-002 | 高 | `locallife/baofu/account/contracts/types.go:19` 的 `OpenAccountRequest` 是业务抽象 | 官方开户 `version/accType/accInfo/noticeUrl/businessType/loginNo/needUploadFile` 和个人/企业/个体条件必填未入契约，容易上线后参数缺失 | 建官方字段级 DTO；个人二要素、个人四要素、企业、个体户分开校验。 |
| C-003 | 高 | `locallife/baofu/account/notification/notification.go:45` 解析 `outRequestNo/sharingMerId/openState` | 官方开户通知字段是 `member_id/terminal_id/memberType/state/errorCode/errorMsg/transSerialNo/loginNo/customerName/contractNo/noticeType`，ACK 还需纯 `OK`；当前 parser 可能无法处理真实通知 | 按官方通知重写 parser/ACK 测试；用宝付测试通知样例或沙箱回调验证。 |
| C-004 | 部分修复，待 transport 集成 | 已新增 `locallife/baofu/envelope.go` 公共请求/响应 envelope DTO，覆盖 `merId/terId/method/charset/version/format/timestamp/signType/signSn/ncrptnSn/dgtlEnvlp/signStr/bizContent` | 业务 DTO 已可进入 `bizContent`，但真实 HTTP client 尚未把签名、验签、加密和 envelope 串起来 | Task 8 接真实 HTTP 时必须使用该 envelope 并补请求/响应 fixture。 |
| C-005 | 高 | `locallife/baofu/aggregatepay/contracts/types.go:116` 的 `Validate()` 只检查 `riskInfo.clientIp` | `unified_order` 的 M/C 字段、长度、枚举、`subMchId` 条件必填、https `pageUrl`、金额关系未被锁住 | 补全字段级校验和表驱动测试；非法枚举/缺条件字段必须 fail-closed。 |
| C-006 | 高 | `locallife/baofu/merchantreport/**` 不存在 | 无法取得宝付返回的微信渠道 `subMchId`，也无法验证商户逐户异主体小程序授权闭环 | 新增 `merchantreport` contracts/client/service/table/query，支持商户级异主体报备和 APPLET 授权目录绑定。 |
| C-007 | 高 | `locallife/logic/baofu_payment_order_route.go:98` 把 `txResult.SubMchID` 传给宝付；`locallife/logic/baofu_payment_service.go:62` 字段名仍是 `MerchantWechatSubMchID` | 来源继承普通服务商链路，未从宝付聚合商户报备的商户 `subMchId` 读取；会把旧微信进件假设带入宝付支付 | 引入商户聚合报备 `SubMchID` 解析器；按商户报备结果取 `sub_mch_id`，不再读普通服务商 txResult。 |
| C-008 | 中 | `locallife/baofu/aggregatepay/contracts/types.go` 无 `order_refund/refund_query/order_close` DTO | 首版宣称“分账前可退款/分账后不退款”，但没有宝付退款与关单契约，支付失败清理也无法关闭上游订单 | 补退款、退款查询、退款通知、关单 DTO/client/状态映射，并测试分账后禁退。 |
| C-009 | 中 | `locallife/baofu/account/contracts/types.go:36`、`:55` 用分为单位；官方余额/提现金额为元 BigDecimal | 金额单位转换若散落在业务层，容易出现提现金额放大/缩小 100 倍 | 在 contract/transport 边界集中做 fen<->yuan 转换，加入舍入和非法小数测试。 |
| C-010 | 中 | `locallife/baofu/aggregatepay/contracts/types.go:180`、`:244` 状态映射只覆盖局部状态 | 聚合支付/分账/退款/报备/认证错误码和未知状态未分类，前端语义和重试策略会漂移 | 建 typed error/status 分类：资料需修改、平台配置错误、处理中可重试、渠道异常需人工；未知状态进入人工处理。 |
| C-011 | 中 | `locallife/baofu/aggregatepay/client.go`、`locallife/baofu/account/client.go` 仍是 interface/未实现 transport | C1/C2 容易被误认为可生产；没有测试地址联调证据 | 真实 client 完成前 implementation plan 不得勾选生产任务；每个接口需 C4 沙箱证据。 |
| C-012 | 中 | `locallife/logic/baofu_payment_readiness.go:14` 仍有“商户微信渠道待报备”错误文案 | 语义混入旧微信特约商户进件，可能误导运营/商户 | 改成“微信支付通道待开通/微信渠道待配置”，内部记录具体 report/auth 状态。 |

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
| 账户 union-gw | `https://vgw.baofoo.com/union-gw/api/{报文编号}/transReq.do` | 否 | 仅本地单元测试和本地证书/加密测试。 |
| 聚合商户报备 | `https://mch-juhe.baofoo.com/mch-service/api` | 否 | 无代码。 |
| 聚合支付/分账/退款 | `https://mch-juhe.baofoo.com/api` | 否 | 仅 contracts/service mock 测试。 |
| 回调通知 | 本平台 webhook URL | 否 | 仅本地 fake parser/callback 测试。 |

生产前必须形成沙箱证据：请求报文摘要、响应摘要、回调样例、查询补偿样例、错误样例、对应测试订单/流水号、测试时间、测试账号、代码 commit。

## 14. 高优先级整改任务

1. 新增 endpoint profile：账户 union-gw、聚合支付、聚合商户报备三组测试/生产/备用地址分开配置，不再使用 `https://api.baofoo.com` 占位默认值。
2. 按已确认的 `subMchId` 口径实现：商户逐户 `merchant_report`、`merchant_report_query`、`bind_sub_config(authType=APPLET, authContent=<LocalLife 小程序 appid>)` 同步闭环；不要在代码中固化平台统一下单，也不要复用项目内微信普通服务商进件结果。
3. 实现真实 `aggregatepay.Client` HTTP transport，并把 API runtime 从普通服务商 facade 切到宝付 facade，做成 Task 4A。
4. 把 `unified_order` 的所有 M/C 字段、长度、枚举、禁传条件补进 `Validate()`，不只校验 `riskInfo.clientIp`。
5. 为账户开户建立官方字段级 DTO，区分个人二要素/个人四要素/企业/个体，并补条件必填测试；同时预留开户/报备可能需要的用户确认状态，但不得假设拓展码流程已明确。
6. 补宝付退款、退款查询、退款通知、关单契约和前置互斥，确保“分账前退款”真的可用。
7. 从 `bct-1f9o6qi1pf2r8` 和 `/home/sam/文档/分账/宝付/经营类目&MCC.xlsx` 生成/维护首版必用枚举和类目 allowlist；至少覆盖微信经营类目 110 条和报备/授权/认证状态枚举，并用 hash/行数/样例测试防漂移。
8. 建立错误码映射：账户错误码、聚合支付错误码、报备错误码至少分成“用户需改资料”“商户/平台配置错误”“宝付处理中可重试”“宝付/渠道异常需人工”。
9. 每个必用接口至少跑一次宝付测试地址正向、参数错误、重复请求/幂等、查询恢复、回调重复投递。
10. 向宝付确认拓展码/扫码确认的接口归属、字段、扫码主体和状态查询方式，并把结果补入开户/报备契约。
11. 增加主业务宝付切换边界测试，确保 `direct` 支付路径不会被宝付替换。

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
