---
title: 支付接口
slug: bct-1fdiul8ggbs5f
source_url: https://doc.mandao.com/docs/bct/bct-1fdiul8ggbs5f
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 692f95044ae7b9b4a40b0bed1413f5f1b8cabe082a0fd240a8b6b8a45315cdec
doc_version: 1743642551
---

# 支付接口
## 3.1 支付接口

宝付支付网关支付是指用户从商户应用先经过宝付收银台，进行相关信息的填写，再由宝付提交至银行处理的支付方式。 注： 请勿在测试环境使用大额支付，测试金额不予返还，建议使用1分钱进行测试； 上线时，请及时更换正式商户号、终端号和密钥，并提交至正式环境。

### 3.1.1 支付请求报文

支付请求报文，是指商户通过HTTP或HTTPS请求，采用POST方式提交并经过证书加密提交订单信息至宝付接收地址，宝付将获取参数进行加密信息比对校验，校验一致则把订单信息及结果通知给商户。

- 测试接收地址
 [https://vgw.baofoo.com/neptune/unionpayindex](https://vgw.baofoo.com/neptune/unionpayindex)
- 正式接收地址
 [https://gw.baofoo.com/neptune/unionpayindex](https://gw.baofoo.com/neptune/unionpayindex)

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 版本号 | interfaceVersion | M | 4.12:加密数据类型为json |
| 02 | 终端号 | terminalId | M | 由宝付分配 |
| 03 | 商户号 | memberId | M | 宝付提供给商户的唯一编号 |
| 04 | 加密数据 | dataContent | M | 具体参数如下**加密数据** / **注意：** / 加密之前,先将组装的数据（请参照数据模版组装）进行Base64编码转化，然后再进行证书加密。 |

**加密数据：**

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 功能ID | payId | O | 参考《附录：产品功能》 / 注：若选择全部银行则为空字符串，选择全部银行即跳转宝付收银台选择银行 |
| 02 | 终端号 | terminalId | M | |
| 03 | 商户号 | memberId | M | 宝付提供给商户的唯一编号 |
| 04 | 商户订单号 | transId | M | 唯一订单号，8位系统当前日期格式为yyyyMMdd+字母或数字组成,宝付将以此作为结算的唯一凭证 |
| 05 | 交易日期 | tradeDate | M | 14 位定长。 / 格式：年年年年月月日日时时分分秒秒 |
| 06 | 交易金额 | orderMoney | M | 单位：分 / 例：1元则提交100 |
| 07 | 商品名称 | productName | O | |
| 08 | 商品数量 | amount | O | 默认为1 |
| 09 | 用户名称 | userName | O | 长度不超过64位 |
| 10 | 附加字段 | additionalInfo | O | 长度不超过64位 |
| 11 | 通知类型 | noticeType | M | 固定数字：1 |
| 12 | 页面返回地址 | pageUrl | M | 该地址通知服务不可以用作交易处理 |
| 13 | 交易通知地址 | returnUrl | M | 服务器通知地址：[http://www.baofoo.com/demo/return_url](http://www.baofoo.com/demo/return_url) |
| 14 | 营销信息 | unionInfo | O | 单位(分),营销商户只能是主商户 / 格式:商户1,金额1; / 例如:100000363,10; |
| 15 | 请求方保留域 | reqReserved | O | |
| 16 | 交易模式 | model | O | 用来控制宝付收银台界面展示的产品。 / 可选填：b2b，b2c / b2b：宝付收银台只展示b2b产品 / b2c：宝付收银台只展示b2c相关产品 |
| 17 | 总金额 | totalMoney | M | 总金额，单位：分 / 例：1元则提交100 |

**加密数据格式**

- XML
 ```
 
 
 3001
 100000916
 100000178
 1236546587
 20170428145222
 500
 土豪金
 1
 土豪
 http://www.baofoo.com/demo/page_url
 http://www.baofoo.com/demo/return_url
 100000363,500;
 1
 aa
 bb
 1000
 
 ```
- JSON
 ```
 {
 "payId":"3001",
 "terminalId":"100000916",
 "memberId":"100000178",
 "transId":"1236546587",
 "tradeDate":"201412171132",
 "orderMoney":"500",
 "productName":"土豪金",
 "amount":"1",
 "userName":"土豪",
 "pageUrl":"http://www.baofoo.com/demo/page_url",
 "returnUrl":"http://www.baofoo.com/demo/return_url",
 "unionInfo":"100000363,500;",
 "noticeType":"1",
 "additionalInfo":"aa",
 "reqReserved":"bb",
 "totalMoney":"1000"
 }
 ```

文档更新时间: 2025-06-11 10:33 作者：超级管理员
