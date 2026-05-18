# 宝付宝财通生产首单 Checklist

> 适用场景：宝付全量切换后的第一笔生产订单。宝付已确认测试环境不支持真实下单，因此可真实调起并完成支付的 `wc_pay_data`、支付回调、后续分账/提现闭环必须在生产首单或宝付提供的真实交易验证环境中完成。首单必须在平台财务、运营、后端值守同时在线时执行；任一核对项失败即暂停新支付创建，已支付订单继续走宝付分账/提现恢复链路。

## 0. 契约真值与运行配置

- [ ] 首单前已复核 `artifacts/baofu-payment/baofu-contract-source-matrix.md`，本次使用接口的官方字段、条件必填、枚举、签名、ACK 规则均已固化到代码和测试。
- [ ] 聚合支付/分账/退款回调按宝付文档要求校验 POST 公共 envelope、`dataContent`、`signStr`，成功处理后只返回纯文本大写 `OK`；代码不依赖未写入文档的 `Content-Type` header。
- [ ] 生产 `signSn` / `ncrptnSn` 已配置为宝付发放的 S(10) 证书索引，不使用本地从 X.509 证书推导出的长序列号。
- [ ] 已确认生产首单观察项只记录脱敏字段：openid、身份证、银行卡、手机号、私钥、完整 `contractNo`、完整 `sharingMerId`、完整 `subMchId` 均不得进入文档或普通日志。

## 1. 首单前配置确认

- [ ] 宝付收款商户号、终端号、证书、加密密钥已在生产环境配置完成；开户、转账、支付、分账、余额查询均使用收款商户号。
- [ ] 宝付付款商户号、终端号已在生产环境配置完成；提现和提现查询均使用付款商户号。
- [ ] 收款商户余额和交易权限已由宝付商务/技术确认可用。
- [ ] 付款商户已预存开户验证费预算；个人/企业开户验证费由平台承担。
- [ ] 平台自己的宝付二级户已开户成功，`owner_type=platform`、`owner_id=0` 的 `sharing_mer_id` 已写入，且不是平台宝付收款商户号。

## 2. 接收方就绪确认

- [ ] 商户宝付二级户开户成功，状态为 `结算账户可用`。
- [ ] 商户微信渠道报备标识存在，允许在微信生态内通过宝付聚合支付下单；生产 `unified_order` 必须上送该商户报备返回的 `subMchId`。
- [ ] 骑手宝付个人二级户开户成功，状态为 `结算账户可用`。
- [ ] 区域运营商宝付二级户开户成功，状态为 `结算账户可用`。
- [ ] 所有分账接收方使用开户接口返回的二级商户号作为 `sharing_mer_id`；不得使用 `contract_no`、微信 `openid`、微信 `subMchId` 或平台宝付收款商户号。

## 3. 支付与回调确认

- [ ] `bind_sub_config(authType=APPLET)` 已成功至少 30 分钟后再发起首单支付；小程序发起首单支付，前端直接使用后端返回的 `wc_pay_data` 调用 `wx.requestPayment`，不在前端拼装 nonce、package、signType 或 paySign。
- [ ] 宝付支付订单本地 `payment_orders.payment_channel=baofu_aggregate`、`requires_profit_sharing=true`、`payment_type=profit_sharing`。
- [ ] `external_payment_commands` 已记录宝付支付创建命令，provider/channel/capability 分别为 `baofu`、`baofu_aggregate`、`baofu_payment`。
- [ ] 支付成功回调已验签、解密并持久化到 `external_payment_facts`，再由事实应用推进本地支付成功。
- [ ] 支付成功后不执行退款；如分账未开始且必须退款，按人工平台处理预案执行。

## 4. 分账确认

- [ ] 订单满足退款关闭条件后才创建宝付分账单。
- [ ] 分账明细包含商户、骑手、运营商、平台四类接收方；接收方均来自 `sharing_mer_id`。
- [ ] 首单金额公式核对通过：骑手拿配送费全额，平台 2%，运营商 3%，宝付 0.3% 支付手续费只从商户净额扣减。
- [ ] 分账命令 `share_after_pay` 已记录到 `external_payment_commands`。
- [ ] 分账成功回调已验签、解密并持久化到 `external_payment_facts`，再由事实应用推进分账完成。

## 5. 余额、提现与对账确认

- [ ] 商户、骑手、运营商、平台二级户余额与首单分账公式一致。
- [ ] 商户支付手续费台账 `baofu_fee_ledger` 与 `profit_sharing_orders.payment_fee` 一致。
- [ ] 发起一笔小额提现，提现请求使用宝付付款商户号。
- [ ] 提现查询返回成功或处理中，`baofu_withdrawal_orders` 状态与宝付返回一致。
- [ ] `/v1/platform/stats/baofu/reconciliation/daily` 的支付、分账、手续费、提现、异常计数已核对，无明文 `contract_no`、`sharing_mer_id`、银行卡、身份证、手机号或上游原始报文。

## 6. 首单后观察

- [ ] 宝付支付回调超时、分账处理中超时、提现处理中超时、事实应用失败、手续费台账不一致五类告警均可被平台告警通道承载。
- [ ] 后台日志中无私钥、AES key、签名材料、完整银行卡、完整身份证、完整手机号、明文分账接收方或宝付原始载荷泄露。
- [ ] `make check-generated`、宝付支付/分账/提现 focused tests、前端 lint/analyze 已在发布前通过或有明确 CI 补跑记录。
- [ ] 首单完成后 24 小时内复核宝付商户后台账单与本地对账视图。
