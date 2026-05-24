# 宝付宝财通分账接入实施草案

## 1. 背景与目标

LocalLife 尚未正式开展主业务交易，不存在需要迁移的存量商户或存量支付单。本次接入目标是将商户主业务支付、分账、账户余额和提现全量切换到宝付宝财通/聚合支付体系，避免微信普通服务商分账比例最高 30% 对平台经营模型的限制。

已确认业务事实：

- 宝付是支付牌照持牌机构，平台已与宝付签订支付合作协议；本方案不再把合规资质作为待确认阻塞项。
- 宝付商务说明：即使使用宝付支付，微信生态仍不可绕开。微信在本链路中更像支付渠道/银行侧账户生态，平台和商户仍要完成微信侧账户/报备，否则宝付无法识别微信渠道下的交易主体和资金归属。
- LocalLife 在宝付和微信支付生态关系中按宝付的二级服务商/业务平台接入，由宝付承接微信渠道收单、分账和清结算能力。
- 已向宝付技术支持确认：LocalLife 不再需要保留项目内微信支付特约商户进件流程；宝付支持异主体报备。宝付开户成功后，主业务商户继续走聚合商户报备取得微信渠道 `subMchId`，再通过 `bind_sub_config(authType=APPLET, authContent=<LocalLife 小程序 appid>)` 把该商户 `subMchId` 绑定到 LocalLife 平台小程序。商户、骑手、运营商仍通过宝付个人/机构进件和宝财通二级户承接分账。
- 宝付已提供两个平台级商户号：为避免和微信支付二级商户号混淆，文档和代码注释统一称为 `宝付收单一级商户号` 与 `宝付代付一级商户号`。宝付收单一级商户号用于开户、转账、支付、分账等交易；宝付代付一级商户号用于提现和提现查询。系统必须显式区分二者，不能混用。
- 宝付费率为交易金额的 0.3%。个人用户开户验证费和企业用户开户验证费均为 1 元/户。
- 开户验证费由平台承担，平台需先预存到宝付代付一级商户号；支付手续费由商户承担，首版即按此规则落地，不设计后续“平台承担/商户承担”切换。
- 本业务模型约定分账后不退款。退款只允许发生在分账前；分账触发点必须晚于订单不可退款终态或售后窗口关闭。

## 2. 方案边界

### 2.1 纳入首版

- 商户宝财通企业/个体账户开户、查询、开户通知。
- 骑手宝财通个人账户开户、查询、开户通知。
- 运营商、平台佣金收款账户配置或开户。
- 宝付聚合商户报备：`merchant_report`、报备信息查询、微信渠道 `subMchId` 入库；已确认采用商户逐户异主体报备，并为每个商户 `subMchId` 绑定 LocalLife 平台小程序 appid。
- 宝付聚合支付微信小程序支付：`unified_order`、支付通知、支付订单查询。
- 宝付确认分账：`share_after_pay`、分账通知、分账订单查询。
- 宝财通余额查询、提现、提现通知。
- 支付手续费本地成本与商户承担分账公式。
- 分账前退款互斥控制；分账后不退款的业务拦截与用户提示。
- 回调验签/解密、幂等、外部命令记录、外部事实记录、领域 outbox。

### 2.2 暂不纳入首版

- 存量微信普通服务商商户兼容。
- 新旧支付通道灰度并行。
- 商户承担/平台承担手续费的动态配置。
- 替换微信直连支付路径：骑手保证金缴纳/赎回、商户追偿向平台付款、骑手追偿向平台付款及其查询、退款、通知继续走微信直连支付，不由宝付替换。
- 平台内部复杂补贴、营销金额、多费率、多区域费用策略。

## 3. 关键结论

### 3.0 术语与支付机构边界

为避免宝付、微信两个体系的“商户号/二级商户号”混淆，本文档后续统一使用以下术语：

| 术语 | 含义 | 是否作为分账接收方 |
| --- | --- | --- |
| 宝付收单一级商户号 | 平台在宝付开通的收单/交易商户号，用于宝财通开户、转账、聚合支付、确认分账等交易发起 | 否 |
| 宝付代付一级商户号 | 平台在宝付开通的代付/提现商户号，用于宝财通提现和提现查询，并承接平台预存的开户验证费 | 否 |
| 宝付二级商户号 | 商户、骑手、运营商、平台自身通过宝财通开户得到的二级户标识；本地规范字段为 `sharing_mer_id` | 是 |
| 微信渠道二级商户号 / `subMchId` | 宝付聚合商户报备返回的微信渠道交易主体标识；按商户逐户保存，用于 `unified_order.subMchId` | 否 |
| 付款人 `sub_openid` | 用户在 LocalLife 小程序 appid 下的付款身份 | 否 |

资金链路口径：顾客通过微信生态付款后，宝付聚合支付以宝付收单一级商户号和微信渠道 `subMchId` 发起交易；主交易资金进入宝付收单一级商户号对应的待结算/资金管理体系。订单达到分账条件后，LocalLife 向宝付发送确认分账指令，宝付按 `sharingDetails[].sharingMerId` 将资金分入商户、骑手、运营商和平台自己的宝付二级户。二级户资金先体现为在途余额，结算后进入可用余额，可用余额才能提现。

微信 `subMchId` 不是“无意义标记”：它是微信渠道交易主体和合规报备身份，宝付统一下单在微信/支付宝场景要求必传；但它不承载宝付分账收款，不能写入 `sharing_mer_id`，也不能作为分账接收方兜底。

项目内原微信普通服务商特约商户进件流程不再进入宝付主链路；宝付个人/机构进件与宝财通二级户开户替代分账接收方开通，宝付聚合商户报备替代微信小程序支付所需渠道主体开通。

宝付接入只替换主业务订单中原平台收付通/普通服务商方向承载的支付、分账、退款互斥、账户余额和提现能力；不替换微信直连支付能力。

### 3.0.1 运行时标识来源真值表

| 运行时需要 | 正确来源 | 绝不能使用的错误来源 | 落地约束 |
| --- | --- | --- | --- |
| 分账接收方 `sharingMerId` | 宝付开户/查询返回的宝付二级商户号，本地规范保存为 `baofu_account_bindings.sharing_mer_id` | 宝付收单一级商户号、宝付代付一级商户号、微信渠道 `subMchId`、付款人 `openid/sub_openid`、`contract_no` 兜底值 | 开户/查询同步层必须把宝付二级商户号写入 `sharing_mer_id`；分账 receiver 创建只读取 `sharing_mer_id`，为空时 fail-closed，不再回退 `contract_no`。 |
| 宝付 `unified_order.subMchId` | 商户逐户聚合商户报备成功后保存的 `baofu_merchant_reports.sub_mch_id` | 微信普通服务商进件结果、平台统一 `subMchId`、宝付二级商户号、宝付一级商户号、`baofu_account_bindings.wechat_sub_mch_id` 历史字段 | 创建微信 JSAPI 支付前必须读取商户报备记录，且 `report_state=succeeded`、`sub_mch_id` 非空、`applet_auth_state=succeeded`。 |
| APPLET 授权目录 `authContent` | 平台配置 `WECHAT_MINI_APP_ID` / 宝付配置中的 LocalLife 小程序 appid | 商户自行填写的 appid、报备响应中的随机字段、空占位符、微信普通服务商 appid | 每个商户报备返回的 `subMchId` 都要调用 `bind_sub_config(authType=APPLET, authContent=<LocalLife 小程序 appid>)`；授权未成功时阻断支付 readiness。 |
| 付款人 `sub_openid` | 用户在 LocalLife 小程序 appid 下的微信 openid | 商户/骑手宝付二级户、`sharing_mer_id`、微信渠道 `subMchId` | 只用于 `unified_order.payExtend.sub_openid` 拉起微信支付，不得进入分账 receiver 和对账展示。 |
| 宝付交易发起 `merId/terId` | 对应能力配置的宝付一级商户号和终端号：支付/分账/报备用收单一级商户号，提现/提现查询用代付一级商户号 | 宝付二级商户号、微信 `subMchId`、付款人 openid | 配置层拆分收单与代付；业务 DTO 只在对应接口边界填入一级商户号。 |

`baofu_account_bindings.wechat_sub_mch_id` 只保留为早期设计遗留/历史对账字段，不再作为宝付主链路 payment readiness 或统一下单来源。后续如需迁移清理，应以 `baofu_merchant_reports.sub_mch_id` 为唯一迁移目标。

### 3.1 openid 不能替代分账接收方账户

宝付聚合支付的微信 JSAPI 支付属性中，`sub_appid` 和 `sub_openid` 是用户支付调起所需字段，用于标识付款用户在小程序/公众号下的微信身份。它解决的是“谁在微信里付款”的问题。

宝付确认分账接口 `share_after_pay` 的分账明细字段是 `sharingDetails`，其中每个接收方使用 `sharingMerId` 和 `sharingAmt` 表达。它解决的是“钱分给哪个宝付侧收款主体”的问题。

因此：

- 用户 `openid` 只用于支付调起和付款人身份，不是分账接收方标识。
- 支付下单中的 `subMchId` 是微信/支付宝报备的二级商户号，用于渠道交易主体识别，也不是骑手分账接收方；已确认来源为商户逐户聚合商户报备结果，并通过异主体授权目录绑定 LocalLife 平台小程序。
- 骑手要直接拿代取费全额分账，必须在宝付侧存在可收款主体。宝财通个人账户开户成功后返回的二级商户号必须同步到本地 `sharing_mer_id`，分账到账后先进入在途余额，结算后进入可用余额，再提现到绑定银行卡。
- 分账是分到宝付体系下的平台二级户/宝财通二级户，不是分到微信支付二级户。微信侧二级商户号只用于渠道交易主体识别。
- 已向宝付确认：分账接口直接上送开户接口返回的二级商户号。系统以 `sharing_mer_id` 作为本地规范字段保存这个二级商户号；开户/查询解析层必须把宝付返回的二级商户号写入 `sharing_mer_id`，后续分账只从 `sharing_mer_id` 取值。`contract_no` 只保留上游开户/查询字段和对账留痕，不作为分账创建兜底字段。`openid`、微信 `subMchId`、平台在宝付的收款商户号都不能作为分账接收方。

首版按“骑手需要宝财通个人账户/宝付二级户”落地。本地必须保存骑手与宝付二级户接收方标识的映射，不能只保存 openid 或微信支付二级商户标识。

### 3.2 支付手续费由商户承担

平台从订单中分得 2 个点。如果平台再承担 0.3% 支付手续费，实际平台毛收入会被明显压缩。因此首版固定为商户承担宝付支付手续费：

- 开户验证费：平台承担，计入平台获客/入驻成本。
- 支付手续费：商户承担，从商户应得货款中扣减。
- 骑手代取费：不承担支付手续费，仍按代取费全额分账给骑手。
- 运营商佣金：不承担支付手续费，仍按订单金额 3% 分账给运营商。
- 平台佣金：不承担支付手续费，仍按订单金额 2% 分账给平台。
- 商户净收入：订单金额扣除骑手代取费、运营商佣金、平台佣金、支付手续费后的剩余金额。

首版不提供手续费承担方配置，避免产生运营口径和对账口径分叉。

### 3.3 核心问题的文档复核与业务结论

本轮复核了宝付在线接口文档和本地资料 `/home/sam/文档/分账/宝付`，并叠加宝付沟通结论。结论如下：

| 问题 | 当前结论 | 实施口径 |
| --- | --- | --- |
| `sharingMerId` 精确定义 | 已确认：分账接口使用开户接口返回的二级商户号。它不是 `openid`，不是微信报备返回的 `subMchId`，也不是平台在宝付的收款商户号。协议支付确认分账示例使用 `CM...`、`CP...` 这类宝财通二级户号；账户查询的 `contractNo` 是上游账户/查询字段，不能作为本地分账兜底字段。 | 本地以 `sharing_mer_id` 作为分账接收方规范字段。开户/查询解析层必须把宝付返回的二级商户号同步成 `sharing_mer_id`；分账明细只读取 `sharing_mer_id`。`contract_no` 仅保留上游字段和对账留痕。不得使用微信 `subMchId`、`openid` 或平台宝付收款商户号作为分账接收方。 |
| 两个宝付一级商户号边界 | 已确认：宝付收单一级商户号用于开户、转账、支付、分账等交易；宝付代付一级商户号用于提现和提现查询。 | 后端配置拆成 `baofu_collect_merchant` 和 `baofu_payout_merchant`。账户开户、账户转账、聚合支付、确认分账走宝付收单一级商户号；提现、提现查询走宝付代付一级商户号。 |
| 开户费与手续费 | 已确认：开户手续费由平台支付，平台需要先预存到宝付代付一级商户号；支付手续费由商户承担。 | 开户验证费写平台成本台账；支付手续费从商户应得货款扣减，不影响平台 2% 和运营商 3% 佣金。 |
| 分账后退款 | 宝付文档支持分账退款和垫资字段，但 LocalLife 业务模型约定分账后不退款。 | 系统只允许分账前退款；分账后退款入口关闭。分账必须在订单不可退款终态或售后窗口关闭后触发。 |
| 骑手个人账户 | 已确认：骑手也要开通宝付二级户才能实现分账。 | 骑手代取费分账接收方使用宝付二级户；分账不是分到微信支付二级户。骑手未开通宝付二级户时，不允许进入可分账接单或分账队列。 |
| 聚合商户报备与 `subMchId` | 已确认：不再需要项目内微信支付特约商户进件；宝付 `merchant_report` 可开通微信/支付宝渠道二级商户并返回 `subMchId`，且支持异主体绑定 LocalLife 平台小程序。由于分账不需要 `subMchId`，该字段只影响微信小程序支付主体和展示主体。 | 商户宝财通二级户 active 且 `sharing_mer_id` 已同步后逐户报备，并把商户 `subMchId` 作为宝付 `unified_order.subMchId` 来源；随后调用 `bind_sub_config(authType=APPLET, authContent=<LocalLife 小程序 appid>)`。`subMchId` 不得作为分账接收方。 |
| 拓展码/用户扫码确认 | 当前已抓取的宝付接口文档中未看到明确字段名、接口名或返回参数；报备附录存在 `CONTACT_CONFIRM`、`LEGAL_CONFIRM`、`AUTHORIZED`、`UNAUTHORIZED` 等确认/认证状态，宝财通协议也提到可通过商户前台页面或接口提交资质并取得授权。 | 设计上预留 `pending_user_confirm` / `pending_legal_confirm` 类状态，但不得假设完全无感。上线前必须向宝付确认拓展码属于开户、报备还是电子签/认证步骤，确认其生成接口、返回字段、扫码主体、有效期和查询接口。 |

## 4. 账户模型

### 4.1 平台级账户

平台配置两个宝付一级商户号：

- 宝付收单一级商户号：承接开户、账户转账、聚合支付和确认分账等交易。它是交易发起/资金管理主体，不是分账接收方。
- 宝付代付一级商户号：用于宝财通提现和提现查询。平台承担的开户验证费需先预存到该商户号。它不能用于开户、支付或确认分账。

代码实现时必须把宝付收单一级商户号配置和宝付代付一级商户号配置拆开，避免用一个“宝付商户号”字段混用所有能力。

### 4.2 业务主体账户

| 主体 | 是否需要宝财通账户 | 账户类型 | 用途 |
| --- | --- | --- | --- |
| 商户 | 是 | 企业/个体 | 接收扣除佣金、代取费、手续费后的商户货款；提现 |
| 骑手 | 是 | 个人 | 接收代取费；提现 |
| 运营商 | 是 | 企业/个体或平台配置账户 | 接收 3% 运营佣金 |
| 平台 | 是/已有 | 平台收款或收入账户 | 接收 2% 平台佣金 |

平台佣金接收方在本地作为单例宝付账户绑定保存：`owner_type=platform`，`owner_id=0`。该记录只用于平台 2% 佣金分账接收方 readiness 和后续提现/对账能力，不代表普通用户或商户主体。

### 4.3 微信生态标识

- 小程序全局 appid 仍是用户支付入口。
- `sub_openid` 是付款用户在小程序 appid 下的 openid，由前端/后端支付下单链路提供。
- 微信支付场景下，项目不再走微信普通服务商特约商户进件。宝付聚合支付下单时微信/支付宝场景需要传 `subMchId`；因分账接口不读取 `subMchId`，该字段只影响统一下单和展示主体。已确认采用商户逐户报备并异主体绑定平台小程序。
- `subMchId` 是微信渠道交易主体和支付展示主体，不用于骑手、商户、运营商或平台分账。
- 宝付确认分账 `share_after_pay` 不要求 `subMchId`，分账明细只传 `sharingDetails[].sharingMerId`。所以“异主体申报/绑定授权目录”不影响资金分给哪个宝付二级户；它只影响微信小程序支付能否拉起，以及顾客看到平台名还是具体商户名。
- 分账接收方归属宝付二级户体系，不归属微信支付二级户体系。

## 5. 资金与费用公式

### 5.1 名词

- `total_amount`：订单用户实际支付金额，单位分。
- `delivery_fee`：代取费，单位分。
- `platform_rate`：平台佣金比例，首版 2%。
- `operator_rate`：运营商佣金比例，首版 3%。
- `payment_fee_rate`：宝付支付手续费率，首版 0.3%。
- `platform_commission`：平台佣金。
- `operator_commission`：运营商佣金。
- `rider_amount`：骑手分账金额。
- `payment_fee`：商户承担的支付手续费。
- `merchant_amount`：商户净分账金额。

### 5.2 首版公式

```text
rider_amount = delivery_fee
platform_commission = round(total_amount * 2%)
operator_commission = round(total_amount * 3%)
payment_fee = round(total_amount * 0.3%)
merchant_amount = total_amount - rider_amount - platform_commission - operator_commission - payment_fee
```

取整规则必须与财务口径一致。建议首版使用“按分向上取整”或按宝付账单实际手续费回填；如果下单前必须给出确定分账金额，则先使用本地确定性取整规则，并在对账时核验宝付实际手续费。

### 5.3 示例

订单用户支付 100.00 元，代取费 5.00 元：

```text
total_amount = 10000
rider_amount = 500
platform_commission = 200
operator_commission = 300
payment_fee = 30
merchant_amount = 10000 - 500 - 200 - 300 - 30 = 8970
```

分账明细：

- 骑手：5.00 元。
- 平台：2.00 元。
- 运营商：3.00 元。
- 商户：89.70 元。
- 宝付手续费：0.30 元，由商户承担，不进入平台收入。

### 5.4 对账要求

每笔订单必须能追溯：

```text
用户支付金额
  -> 宝付手续费
  -> 骑手代取费分账
  -> 平台佣金分账
  -> 运营商佣金分账
  -> 商户净收入分账
  -> 各主体在途余额/可用余额
  -> 提现流水
```

平台侧提供宝付每日对账聚合视图 `GET /v1/platform/stats/baofu/reconciliation/daily`，按 provider/channel 输出支付金额、宝付手续费、商户/骑手/平台/运营商分账金额、提现成功/处理中金额、失败事实数、未知命令数和手续费台账不一致数。该视图只输出聚合金额和计数，不输出 `contract_no`、`sharing_mer_id`、银行卡、身份证、手机号、签名材料或宝付原始载荷。

平台告警口径覆盖五类首版必须盯盘的异常：宝付支付回调超过 SLA 未到达、宝付分账处理中超过 SLA、宝付提现处理中超过 SLA、`external_payment_facts.processing_status=failed`、`baofu_fee_ledger.amount` 与 `profit_sharing_orders.payment_fee` 不一致。告警 payload 固定带 provider/channel 标签，并过滤分账接收方、合同号和上游原始数据。

## 6. 主链路流程

### 6.1 宝财通三段式业务流程

宝付给出的业务流程可以抽象为三段：

```text
支付用户
  -> 选择银行卡/微信/支付宝等支付方式
  -> 支付完成后按分账指令进入宝付二级户
  -> 二级户资金提现到绑定银行卡
```

三段对应的系统能力：

1. 开户：对接宝财通开户接口，为企业、个体、个人开通宝付二级户。商户、运营商、平台佣金接收方走企业/个体二级户；骑手走个人二级户。
2. 支付分账：对接宝付相关支付产品。支付方式可以是银行卡、微信、支付宝等；支付完成后，资金按分账指令进入宝付二级户。
3. 提现：对接宝财通提现接口，将已分账到二级户且结算为可用余额的资金提现到绑定银行卡。

这张流程图也明确了三个边界：

- 支付方式不等于分账账户。微信、支付宝、银行卡只是支付用户侧的付款方式。
- 分账接收方是宝付二级户，不是微信支付二级户。微信 `sub_openid` 和渠道 `subMchId` 只服务于支付调起和渠道交易主体识别。
- 提现从宝付二级户到银行卡，必须依赖二级户开户和绑定银行卡信息。

### 6.2 开户流程

1. 商户提交营业执照、法人、银行卡、邮箱等信息。
2. 后端调用宝财通开户接口，`businessType=BCT2.0`。
3. 宝付同步返回开户受理结果，本地记录开户申请流水。
4. 宝付异步通知开户结果，本地验签/解密、写入事实表、更新账户映射。
5. 开户成功后保存开户返回的二级商户号，并同步到本地 `sharing_mer_id` 作为后续分账唯一读取字段；`contract_no` 仅同时保留上游开户/查询字段，便于查询、对账和审计。
6. 进入宝付聚合商户报备流程：以商户宝财通二级商户号 `sharing_mer_id` 作为 `bctMerId` 逐户报备。
7. 报备受理后查询/同步报备结果；成功后按商户保存支付下单所需 `subMchId`。
8. 调用 `bind_sub_config`，`authType=APPLET`，`authContent=LocalLife 小程序 appid`，为每个商户 `subMchId` 绑定平台小程序。该步骤是支付通道配置，不是分账接收方配置；失败时应阻断微信小程序支付 readiness，而不是阻断已满足条件的宝付二级户分账建模。

开业/接单前置约束：

- 商户开业前必须满足宝付企业/个体二级户已开通、分账接收方 `sharing_mer_id` 已同步、商户聚合商户报备成功、商户 `subMchId` 已绑定 LocalLife 平台小程序。
- 骑手直接接收代取费，走个人账户开户：身份证、银行卡、银行预留手机号等字段必须齐全。骑手未开户或开户未成功时，不允许其上线或承接需要直接分账的订单。
- 商户端、骑手端和运营商申请状态接口返回统一的 `settlement_account` readiness 结构，只包含 `state`、`label`、`payment_ready`，用于展示 `资料待提交`、`宝付开户处理中`、`微信支付通道待开通`、`结算账户可用`、`开通失败`。
- 对普通用户和商户端只展示产品语义，例如“商户结算账户未开通”“微信支付通道待开通”“骑手结算账户未开通”；不暴露 `contractNo`、`sharingMerId`、银行卡、身份证、手机号或宝付原始错误。

### 6.3 支付流程

1. 用户在微信小程序提交订单。
2. 后端创建本地 `payment_orders` 或合单子支付单前先做 readiness 校验：商户宝付二级户必须已开通并具备分账接收方标识，微信支付通道必须已取得商户报备返回的 `subMchId` 并完成 APPLET 授权目录绑定；不满足时直接拒绝创建支付单，不调用上游支付接口。
   - 单订单支付：在单笔支付创建事务前校验订单所属商户的宝付 readiness。
   - 合单支付：先按订单 ID 解析并去重商户 ID，再逐个校验商户宝付 readiness；校验失败时不进入合单事务，不创建本地合单、子支付单或上游合单支付请求。
3. 后端创建本地 `payment_orders`，通道为宝付聚合支付。
4. 后端调用宝付 `unified_order`：
   - `prodType=SHARING`。
   - `orderType=7`。
   - `payCode=WECHAT_JSAPI`。
   - `payExtend.sub_appid` 为本平台小程序 appid。
   - `payExtend.sub_openid` 为付款用户 openid。
   - `subMchId` 为商户通过宝付聚合商户报备取得并已绑定平台小程序的微信渠道识别码。
   - `riskInfo.clientIp` 为付款用户 IP。宝付文档标注 `riskInfo` 为“微信/支付宝必传”，其中 `clientIp` 必填；`locationPoint` 是交易商户终端经纬度，首版没有稳定来源时不传。
5. 宝付返回 `chlRetParam.wc_pay_data`，后端转给小程序调起支付。
6. 宝付支付通知到后端，后端验签、写入 `external_payment_facts`、幂等更新支付单。
7. 支付成功后发布 outbox 事件，由 worker 发起分账。

### 6.4 分账流程

1. 分账 worker 获取支付成功且需要分账的订单。
2. 在事务中锁定支付单和分账单，防止退款并发。
3. 按首版公式计算骑手、平台、运营商、商户、手续费。
4. 创建本地 `profit_sharing_orders`，记录分账明细快照。
5. 调用宝付 `share_after_pay`：
   - 使用原支付单 `originTradeNo` 或 `originOutTradeNo`。
   - 使用本地唯一分账单号 `outTradeNo`。
   - `sharingDetails` 填宝付侧分账接收方编号和金额。
6. 同步返回 `SUCCESS + PROCESSING` 时只表示受理成功，状态置为 `processing`。
7. 分账通知或 `share_query` 返回成功后，状态置为 `finished`，写入收入统计和 outbox。
8. 分账异常进入查询重试；超过阈值进入人工处理。

### 6.5 提现流程

1. 商户/骑手/运营商查看可用余额。
2. 后端调用宝财通余额查询接口展示在途余额和可用余额。
3. 用户发起提现时，后端使用代付商户号/终端号调用账户提现接口。
4. 同步返回仅代表受理结果，最终以提现通知或提现查询为准。
5. 提现成功、失败、处理中、退回均写入事实表和本地提现状态。

## 7. 状态映射

### 7.1 支付状态

| 宝付状态 | 本地状态 | 处理 |
| --- | --- | --- |
| WAIT_PAYING | pending | 等待用户支付 |
| SUCCESS | paid/success | 触发分账 outbox |
| CLOSED | closed | 释放订单或标记关闭 |
| PAY_ERROR | failed | 允许在订单有效期内重新发起 |
| REFUND | refunded | 同步退款结果 |
| ABNORMAL | unknown/query_required | 定时查询，禁止直接终态化 |

### 7.2 分账状态

| 宝付状态 | 本地状态 | 处理 |
| --- | --- | --- |
| PROCESSING | processing | 等待通知或主动查询 |
| SUCCESS | finished | 更新收入和资金明细 |
| CANCELED | failed/canceled | 标记失败并进入补偿判断 |
| ABNORMAL | failed/query_required | 定时查询，必要时人工处理 |

### 7.3 开户状态

| 宝付状态 | 本地状态 | 处理 |
| --- | --- | --- |
| 1 成功 | active | 保存开户返回二级商户号到 `sharing_mer_id`，并按需保留 `contract_no` 查询/对账留痕；允许分账/提现 |
| 0 失败 | failed | 展示失败原因，允许重新提交 |
| -1 异常 | abnormal | 查询或人工处理 |
| 2 开户处理中 | processing | 等待通知或查询 |

### 7.4 提现状态

| 宝付状态 | 本地状态 | 处理 |
| --- | --- | --- |
| 1 成功 | succeeded | 完成提现单 |
| 0 失败 | failed | 展示原因，可重试或人工处理 |
| 2 处理中 | processing | 等待通知/查询 |
| 3 提现退回 | returned | 进入补偿或人工处理 |

## 8. 后端架构

### 8.1 新增宝付 bounded module

建议新增模块根包：`locallife/baofu`。

内部按能力分组：

```text
locallife/baofu/
  client.go
  config.go
  crypto/
  account/
    contracts/
    errorcodes/
    notification/
  aggregatepay/
    contracts/
    errorcodes/
    notification/
  mock/
```

模块原则：

- 宝付账户 API 和聚合支付 API 使用不同协议，不能混成一个万能 client。
- 业务层不得直接依赖宝付原始 DTO、错误码字符串、加解密 helper。
- 所有请求/响应结构、枚举、错误码先在 `contracts` 中固化，再被 logic/worker 调用。

- 契约层必须按宝付官方字段级 DTO 落地，不能只保留业务抽象字段；公共报文 envelope、业务 `bizContent`、回调通知 payload 要分层建模。
- 官方附录枚举、错误码、经营类目/MCC 文件要有 typed constants 或生成 allowlist，并用来源 hash、行数、关键样例和非法值测试防止漂移。
- `subMchId` 只能作为支付通道 readiness 字段，来源于聚合商户报备成功结果；代码不得固化为平台统一下单；首版固定采用商户逐户异主体授权。
- 回调验签/解密只在模块边界内完成，对外输出项目内 notification payload。

### 8.2 业务层接口

建议对业务层暴露最小接口：

```go
type BaofuPaymentClient interface {
    CreatePayment(ctx context.Context, req CreatePaymentRequest) (*CreatePaymentResponse, error)
    QueryPayment(ctx context.Context, req QueryPaymentRequest) (*PaymentResult, error)
    CreateProfitSharing(ctx context.Context, req CreateProfitSharingRequest) (*ProfitSharingResult, error)
    QueryProfitSharing(ctx context.Context, req QueryProfitSharingRequest) (*ProfitSharingResult, error)
}

type BaofuAccountClient interface {
    OpenAccount(ctx context.Context, req OpenAccountRequest) (*OpenAccountResult, error)
    QueryAccount(ctx context.Context, req QueryAccountRequest) (*AccountResult, error)
    QueryBalance(ctx context.Context, req QueryBalanceRequest) (*BalanceResult, error)
    CreateWithdraw(ctx context.Context, req CreateWithdrawRequest) (*WithdrawResult, error)
    QueryWithdraw(ctx context.Context, req QueryWithdrawRequest) (*WithdrawResult, error)
}
```

### 8.3 事实与命令模型

复用现有支付域解耦方向：

- 外部调用先写 `external_payment_commands`，记录 provider、channel、capability、command_type、外部对象 key、请求指纹和响应快照。
- 回调与主动查询写 `external_payment_facts`，记录 provider、channel、fact_source、upstream_state、terminal_status、raw_resource、dedupe_key。
- 业务状态推进由 fact application/worker 完成，避免在回调 handler 内做复杂业务。
- 支付成功、分账完成、退款异常、提现异常等通过 `payment_domain_outbox` 发布领域事件。
- 宝付分账回调入口为 `POST /v1/webhooks/baofu/share`。回调只做验签/解析、加载本地分账单、写入 `external_payment_facts` 和创建 fact application；真正的 `profit_sharing_orders` 状态推进复用支付域 fact application 单写者。
- 宝付分账事实映射固定为：`PROCESSING -> processing`、`SUCCESS -> success`、`CANCELED -> failed`、`ABNORMAL -> unknown`。只有 `SUCCESS` 允许把本地分账单从 `processing` 推进到 `finished`；`PROCESSING` 只落事实并等待查询恢复，不做业务状态突变。
- 宝付确认分账 worker 只处理已落库的 Baofu `profit_sharing_orders`。它从 `sharing_detail_snapshot` 构造 `share_after_pay` 的 `sharingDetails`，先写 `external_payment_commands`，再调用宝付；命令快照只记录订单 ID、外部单号和接收方数量，不记录明文 `sharing_mer_id` 列表。

## 9. 数据库调整建议

### 9.1 常量与约束

新增常量：

```text
ExternalPaymentProviderBaofu = "baofu"
PaymentChannelBaofuAggregate = "baofu_aggregate"
ExternalPaymentCapabilityBaofuAccount = "baofu_account"
ExternalPaymentCapabilityBaofuPayment = "baofu_payment"
ExternalPaymentCapabilityBaofuProfitSharing = "baofu_profit_sharing"
ExternalPaymentCapabilityBaofuWithdraw = "baofu_withdraw"
```

需要扩展 `external_payment_commands` 和 `external_payment_facts` 的 channel check，允许宝付通道。

### 9.2 新表建议

`baofu_account_bindings`：业务主体与宝付账户映射。

关键字段：

- `owner_type`：merchant/rider/operator/platform。
- `owner_id`：本地主体 id；平台单例账户固定使用 `0`。
- `account_type`：personal/business/platform。
- `contract_no`：宝财通客户账户号/开户回包字段留痕。
- `sharing_mer_id`：分账接收方规范字段，保存开户接口返回的二级商户号；分账接口 `sharingDetails[].sharingMerId` 只读取该字段。
- `login_no`：宝付开户登录号。
- `open_state`：processing/active/failed/abnormal。
- `wechat_sub_mch_id`：早期设计遗留字段，仅可作为历史留痕；首版宝付主链路不得从该字段读取 `unified_order.subMchId`，支付 readiness 只读取 `baofu_merchant_reports.sub_mch_id` 与 APPLET 授权状态。
- `last_open_trans_serial_no`：最近开户流水。
- `raw_snapshot`：宝付返回摘要。

`baofu_merchant_reports`：聚合渠道报备生命周期。首版按商户逐户报备建模；每条成功报备都要进入 APPLET 授权目录绑定流程。

关键字段：

- `merchant_id`：本地商户 id。
- `baofu_account_binding_id`：关联商户宝财通二级户绑定。
- `report_type`：WECHAT/ALIPAY，首版微信小程序支付至少落 WECHAT。
- `report_no`：宝付报备请求唯一编号。
- `bct_mer_id`：报备请求中的宝财通二级商户号，取自 `sharing_mer_id`。
- `sub_mch_id`：报备成功后返回的微信/支付宝渠道商户识别码；同步到商户支付通道 readiness。
- `report_state`：processing/succeeded/failed。
- `platform_biz_no`：宝付/渠道业务号。
- `fail_code`、`fail_message`：报备失败的产品可解释原因，普通前端只展示语义化文案。
- `raw_snapshot`：脱敏后的宝付报备摘要。

`baofu_fee_ledger`：宝付支付手续费和开户验证费台账。

- `CreateBaofuProfitSharingOrderTx`：创建宝付分账订单和商户支付手续费台账必须在同一数据库事务内完成；不能出现有分账订单无手续费台账，或有手续费台账无分账订单的状态。
- `MarkBaofuAccountBindingActiveWithFeeLedgerTx`：开户成功更新账户为 active 与记录平台承担的开户验证费必须在同一数据库事务内完成；不能出现账户已激活但开户费台账缺失的状态。

关键字段：

- `fee_type`：payment_fee/account_open_verify_fee。
- `payer_type`：merchant/platform。
- `payer_id`。
- `business_object_type`：payment_order/baofu_account_binding。
- `business_object_id`。
- `amount`。
- `fee_rate`。
- `provider_bill_no`。
- `status`。

开户验证费的 `payer_type` 固定为 platform，并记录宝付代付一级商户号的预存/扣费流水；支付手续费的 `payer_type` 固定为 merchant，并关联对应 `payment_order` 和商户净收入扣减明细。

## 10. 前端与产品表达

### 10.1 商户开户

商户端不再表达为“微信支付服务商开户”，应拆成：

- 宝付支付开通。
- 宝付聚合商户报备。
- 宝财通结算账户。

商户看到的是业务状态，不看到宝付接口术语：

- 待提交资料。
- 开通处理中。
- 微信支付通道待开通。
- 宝付支付已开通。
- 结算账户可用。
- 开通失败，请修改资料。

运营商申请状态页在申请通过且运营商账号已建立后，也展示同一 `settlement_account` readiness 结构。运营商作为分账接收方不要求微信渠道标识，首版只校验宝付二级户开户状态和接收方标识；响应不暴露 `contractNo`、`sharingMerId` 或上游原始数据。

### 10.2 骑手开户

骑手直接接收代取费，骑手端或运营后台必须收集个人开户资料：姓名、身份证、银行卡、银行预留手机号等。骑手未开户成功时：

- 不允许接可产生分账的代取单；或
- 订单进入不可派给该骑手的状态，直到骑手宝付二级户开通完成。

首版采用“不允许接可产生分账的代取单”，资金链路更简单。

### 10.3 资金页

商户/骑手/运营商资金页应区分：

- 待结算/在途余额。
- 可提现余额。
- 提现中。
- 已提现。
- 手续费。

商户资金页需明确：支付手续费由商户承担，并在订单收入明细中展示扣减金额。当前 Web 商户财务页已在概览、日报、收入明细和服务费/扣减页展示 `支付手续费`，并把待收入口径统一为 `待结算金额`。

### 10.4 已落地前端表述

- 小程序支付调用只消费后端返回的微信调起参数，不在前端构造 nonce、package、signType 或 paySign。
- 骑手工作台、接单大厅和代取费结算页在结算账户不可用时展示：`结算账户未开通，暂不能接收代取费分账订单`。
- 商户 Flutter App 设置页新增 `结算账户` 说明页，覆盖 `宝付支付开通`、`微信支付通道`、`结算账户可用`、`支付手续费`、`待结算金额`、`可提现金额`、`提现中`、`提现成功`、`提现退回`。
- 平台 Web 对账页展示宝付每日聚合对账、提现成功/处理中金额、异常计数，并展示平台佣金接收方的脱敏 `contract_no` / `sharing_mer_id`。
- 运营商 Web 资金指引已改为宝付/宝财通资金模型，不再提示普通服务商模式下去微信商户平台处理余额。
- 所有普通用户、商户、骑手、运营商前端均不展示原始 `contractNo`、`sharingMerId`、银行卡、身份证、手机号、签名材料或上游原始 payload。

## 11. 退款与互斥规则

宝付确认分账文档明确：同一笔支付订单，确认分账接口不支持和申请退款接口同时发起。LocalLife 业务模型进一步约定：分账后不退款。

首版规则：

- 支付成功未分账：允许直接发起普通退款，禁止同时创建分账单；退款命令只能走宝付 `order_refund` 的原支付单退款字段，不能携带分账退款或垫资字段。
- 分账触发点：只能发生在订单不可退款终态或售后窗口关闭后；不能在仍允许用户退款/售后的状态发起分账。
- 分账单已创建：本地 `profit_sharing_orders.status IN ('pending','processing','finished')` 即视为订单已进入结算分账流程，所有退款入口返回 `订单已进入结算分账流程，不支持退款`。
- 分账成功后：关闭退款入口，不调用宝付分账退款或垫资退款能力。
- 垫资退款：宝付退款接口存在 `advanceAmt` 或垫资标识，但本业务模型不启用分账后退款，因此首版不实现垫资退款。
- 支付/分账/退款 worker 必须在数据库层对同一 `payment_order_id` 加锁或使用条件更新，防止并发资金命令。

保留说明：

- 宝付文档支持 `sharingRefundInfo`、`part_share_refund_info`、`advanceAmt` 等分账退款/垫资能力，但这些能力不进入首版业务路径。
- 如果未来业务改为允许分账后退款，必须重新设计退款扣回、余额不足、垫资和手续费退回规则，不能复用首版“分账后不退款”状态机。

## 12. 实施阶段

### 阶段 0：宝付规则固化

- 固化宝付一级商户号边界：宝付收单一级商户号用于开户、转账、支付、分账；宝付代付一级商户号用于提现、提现查询和平台预存开户验证费。
- 固化费用口径：开户验证费由平台支付并从宝付代付一级商户号预存余额扣除；支付手续费由商户承担并从商户净收入扣减。
- 固化分账接收方口径：商户、骑手、运营商、平台佣金接收方均使用宝付二级户接收方标识，不使用微信支付二级户作为分账接收方。
- 平台佣金接收方也必须在宝付侧为平台自己开一个平台名下二级户；不能用平台宝付收款商户号替代平台佣金接收方。
- 固化退款口径：分账后不退款；分账触发点必须晚于不可退款终态或售后窗口关闭。
- 已确认项目内不再保留微信支付特约商户进件流程；宝付支持异主体报备，首版由宝付聚合商户报备逐户取得微信小程序统一下单渠道 `subMchId`。
- 已确认分账使用开户返回的二级商户号；实现上统一同步到 `sharing_mer_id` 后用于分账。
- 确认支付手续费实际扣收的舍入规则和账单字段。

### 阶段 1：宝付基础模块

- 新增 `locallife/baofu` 模块。
- 实现账户 API union-gw 加解密、签名、验签。
- 实现聚合支付 API 签名、验签、请求封装。
- 固化 contracts、errorcodes、notification payload。
- 增加契约测试和 crypto 测试。

### 阶段 2：账户开户与绑定

- 新增宝付账户绑定表和开户流水。
- 实现商户企业/个体开户。
- 实现骑手个人开户。
- 实现开户通知、开户查询、状态同步。
- 前端改造商户/骑手开户状态展示。

### 阶段 2A：聚合商户报备

- 新增宝付聚合商户报备接口边界：`merchant_report`、报备信息查询。
- `bctMerId` 来源固定为商户 `sharing_mer_id`。
- 报备失败原因进入运营/商户可理解的配置修复提示，不暴露上游原始 payload。
- 报备成功后保存商户 `subMchId`，作为 `unified_order.subMchId` 的唯一来源。
- 微信小程序授权目录绑定纳入支付 readiness：每个商户 `subMchId` 都需要完成 `bind_sub_config(authType=APPLET, authContent=<平台小程序 appid>)`。该状态不写入 `sharing_mer_id`，也不影响 `share_after_pay` 的分账接收方计算。
- 商户支付 readiness 必须满足商户宝财通二级户 active、`sharing_mer_id` 存在、商户 `subMchId` 存在且 APPLET 授权目录绑定成功。

### 阶段 3：聚合支付

- 新增宝付支付通道配置。
- 创建支付单时调用 `unified_order`。
- 小程序使用宝付返回的微信支付参数调起支付。
- 实现支付通知、支付查询、异常查询 worker。
- 支付成功后写 outbox。

### 阶段 4：确认分账

- 实现手续费由商户承担的分账计算。
- 实现分账单创建、分账明细快照。
- 分账创建入口必须先通过本地退款互斥门禁：只扫描宝付已支付、订单已完成、退款窗口已关闭、无 `pending/processing/success` 退款、且未创建过分账单的支付单。
- 调用 `share_after_pay`。
- 实现分账通知、分账查询 worker；分账处理中和支付待回调都必须由查询结果先落 `external_payment_facts`，再交给单写者状态机推进本地状态。
- 更新商户、骑手、运营商、平台收入视图。

### 阶段 5：提现与余额

- 实现余额查询。
- 实现提现申请、提现通知、提现查询。
- 资金页展示在途、可用、提现中、已提现。
- 开户验证费和支付手续费入账。

### 阶段 6：退款与对账

- 实现退款与分账互斥。
- 固化分账后不退款：退款只允许发生在分账创建前，分账 `pending/processing/finished` 后所有退款入口返回明确业务提示。
- 建立交易、分账、提现、手续费对账报表。
- 支持通知重放、查询补偿、人工处理入口。

## 13. 验证要求

- 单元测试：签名、验签、加解密、状态映射、手续费计算、分账金额计算。
- 契约测试：宝付接口字段名、必填条件、状态枚举、通知 ACK。
- DB 测试：同一支付单退款/分账互斥、外部命令幂等、外部事实去重。
- Worker 测试：支付成功触发分账、分账处理中查询、通知重复投递、提现退回。
- 集成测试：沙箱下单、支付通知、分账通知、余额查询、提现通知。
- 生产演练：首单人工盯盘，逐项核对支付金额、手续费、分账金额、各账户余额。

## 14. 工作量估算

| 模块 | 估算 |
| --- | --- |
| 宝付基础 SDK、配置、加解密 | 1-1.5 周 |
| 开户与账户映射 | 1.5-2 周 |
| 宝付聚合商户报备与微信渠道二级商户状态同步 | 商户逐户异主体授权约 0.5-1 周 |
| 聚合支付主链路 | 1.5-2 周 |
| 分账主链路 | 1.5-2 周 |
| 退款、提现、余额、费用台账 | 2-3 周 |
| 小程序/商户端/骑手端状态页调整 | 1-2 周 |
| 对账、异常恢复、生产演练 | 1-1.5 周 |

总计约 8.5-13 人周。由于没有存量业务，可省去灰度兼容成本；但这是全量主支付链路切换，沙箱验证、回调重放、对账和生产首单盯盘不能压缩。

## 15. 参考文档

- 本项目接口契约覆盖审计：`artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`
- 宝财通产品简介：https://doc.mandao.com/docs/bct/Introduction
- 宝财通接口须知：https://doc.mandao.com/docs/bct/notice
- 账户请求入口：https://doc.mandao.com/docs/bct/unionGw
- 个人/机构开户接口：https://doc.mandao.com/docs/bct/openAcc
- 开户查询接口：https://doc.mandao.com/docs/bct/queryAcc
- 账户余额查询接口：https://doc.mandao.com/docs/bct/queryBalace
- 账户提现接口：https://doc.mandao.com/docs/bct/accWithdrawal
- 聚合商户报备入口：https://doc.mandao.com/docs/bct/bct-1f9o5s1lqlean
- 报备认证：https://doc.mandao.com/docs/bct/bct-1f9o62bulbiqd
- 报备信息查询：https://doc.mandao.com/docs/bct/bct-1f9o63b6ufii5
- 聚合支付接口请求入口：https://doc.mandao.com/docs/bct/bct-1f9qhakcna6te
- 统一下单交易创建：https://doc.mandao.com/docs/bct/bct-1f9qlvjef634j
- 支付属性：https://doc.mandao.com/docs/bct/bct-1f9qrefjkin2b
- 统一下单渠道返回参数：https://doc.mandao.com/docs/bct/bct-1f9qrf159lni6
- 确认分账：https://doc.mandao.com/docs/bct/bct-1f9qlvu1em0tb
- 分账订单查询：https://doc.mandao.com/docs/bct/bct-1f9qm1m0u1s68
- 支付结果通知：https://doc.mandao.com/docs/bct/bct-1f9qm4ujg50cv
- 分账结果通知：https://doc.mandao.com/docs/bct/bct-1f9qm58emskkg
