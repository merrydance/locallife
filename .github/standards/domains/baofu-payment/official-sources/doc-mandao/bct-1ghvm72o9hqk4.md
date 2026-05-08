---
title: 支付查询接口
slug: bct-1ghvm72o9hqk4
source_url: https://doc.mandao.com/docs/bct/bct-1ghvm72o9hqk4
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 7bfffad739251db0a53b99e8f168234179ff2df3575226411520952a49003b6b
doc_version: 1750669739
---

# 支付查询接口
## 3.2 查询接口

### 3.2.1 接口请求参数

接口请求参数是指商户通过HTTPS 请求，采用POST的方式请求宝付的接口地址，并按 照接口参数定义传送数据。宝付支付网关将以纯文本方式返回查询结果。 `注： 通过订单查询接口对账及时发现和处理异常的交易，此接口没有查询页面，商户如果无法使用查询接口，也可登录商户前台直接查询。 如果参数传入有误将直接返回提示，该接口将不受理频繁的查询请求，最好每次查询时间间隔 1 分钟。`

- **测试接收地址：[https://vgw.baofoo.com/neptune/order/certpayquery](https://vgw.baofoo.com/neptune/order/certpayquery)**
- **正式接收地址：[https://gw.baofoo.com/neptune/order/certpayquery](https://gw.baofoo.com/neptune/order/certpayquery)**

**请求报文**

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户 ID | member_id | M | 平台或商户号,测试商户号：100000178 |
| 02 | 终端 ID | terminal_id | M | 商户签约后通过邮件或者其他方式给予商户 测试终端号：10000001 |
| 03 | 数据类型 | data_type | M | xml 或 json |
| 04 | 加密数据 | data_content | M | 具体参数如下加密数据 / `注意：加密之前,先将组装的数据（请参照数据模版组装）进行Base64编码转化（UTF-8），然后再进行证书加密。` |

**加密数据**

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户ID | member_id | M | 平台或商户号,测试商户号：100000178 |
| 02 | 终端 ID | terminal_id | M | 商户签约后通过邮件或者其他方式给予商户 测试终端号：10000001 |
| 03 | 订单日期 | trade_date | M | 14 位定长。格式：年年年年月月日日时时分分秒秒 |
| 04 | 订单号 | trans_id | M | 唯一订单号，8-20 位字母和数字,宝付将以此作为结算的唯一凭证 |

**应答报文（宝付返回报文）**

```
data_content=261a490a45070b3dd58ca5efa058b35973f39ebbdff5356d942a36cfe364726d7092a4305b22e758c3d95d66967bff9de548353adc265b906cebc0e942bf3f077e9f92344197d5a710ac925f43f8aa0c89df16b4fcef412ee5f56a36d8dfbdda39389146d0a6ae760acf94618e14b4f0bf37b77fbb3481c7739a7b421f3037ff90d5b1dadab1ed480683f02b9c5ad718f7602de55b8db0733c7632113c7b3635aeb1f06ab852567508f1b8ade380b071eeefdba4584c785aeb6f9f8b5c488b4ef8dbec978ef33d4ead8f4950d5d10186484842c73d064c49097b0d3e2dc13b891cace19c4766c7e73f0f6241090ec14cb273d1e655d895da21aafc159058765367dfa7983ba1c60a9f752032e25fab9324562c6bfdf5c4ab921f6b99b07b36205c819564260bd5e096d6a613114f4a64331c795e27b4d4d5f138e4602ed2a45cac28a57dd93723de0324360574ba0f0b0732c3e5f423eb05b1e3518dcbef9018b5440e497f9981b0c12dceb27e7c951cdc3a3236f26c2fffc60f739013f674c096aa6ba969ad2291d1f1ca41e15054193c8c8c15d4702afdda3acf0b8aab30274d2f96b24db5addb57c0709146cb3058aa1f35b54bc544376f4b1d4791e71a5bbc7d2e37e4ae7b56ce77926c4c3cb6b36515e2d2ed4e3c4cfffeb57c2efc1becc0cf421f8f864e7b0284809fcac7863fe8201f47d8bd48ac0879796ffba9b64c8fd001a6a83bfb989671ece19ffd50c6fd0760efa8eea0788ab9f4d10df697ed1cb5e58c91c90d72ffe3d86447c19628d2efa86518e572b00c436f17671fca1beb8c77c74e9690ac111197b14f526d88e02da145dd629a7b5a6a7038b548680830b9c9fdb7aebfe927f3889e7f083f27d20ca7ad4ee698fdd2047f77d5bab199
```

通知结果解密后的报文格式是根据请求的`data_type`来决定，输出内容包括

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户ID | member_id | M | 等同于商户提交的商户号 |
| 02 | 终端 ID | terminal_id | M | 等同于商户提交的终端号 |
| 03 | 订单日期 | trade_date | M | 等同于商户提交的订单日期 |
| 04 | 订单号 | trans_id | M | 等同于商户提交的订单号 |
| 05 | 应答信息 | pay_result | M | 订单的支付状态 / Y：成功 F：失败 P：处理中 N：没有订单 / 若参数校验失败返回具体描述 |
| 06 | 实际成功金额 | succ_money | O | 单位（分），实际支付成功的金额 |
| 07 | 支付完成时间 | succ_time | O | 订单支付完成时间 格式：年年年年月月日日时时分分秒秒 |
| 08 | 功能ID | bank_id | O | 参照:产品功能 |
| 09 | 产品类别 | product_type | O | 对公:b2b,对私:b2c |
| 10 | 卡号 | card_no | O | 对公:保留后四,对私:保留前六后四 |
| 11 | 户名 | card_name | O | 对公:长度10,保留前五后五,对私:掩去第一位 |

文档更新时间: 2025-06-23 09:09 作者：超级管理员
