---
title: 预支付类交易
slug: bct-1flg461i2c8u8
source_url: https://doc.mandao.com/docs/bct/bct-1flg461i2c8u8
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: befa78a53037caa994345246c6ea6e36bda48d05251bff5937ca4e207954f028
doc_version: 1767750806
---

# 预支付类交易
#### 交易URL

- 测试环境地址：
 [https://vgw.baofoo.com/cutpayment/protocol/backTransRequest](https://vgw.baofoo.com/cutpayment/protocol/backTransRequest)
- 正式环境地址：
 [https://public.baofoo.com/cutpayment/protocol/backTransRequest](https://public.baofoo.com/cutpayment/protocol/backTransRequest)

#### 请求报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1. | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2. | 报文流水号 | msg_id | M | Max32Text | 商户流水号 |
| 3. | 报文编号/版本号 | version | M | Max7Text | 4.0.0.0 |
| 4. | 终端号 | terminal_id | M | Max11Numeric | |
| 5. | 交易类型 | txn_type | M | code | 固定值：159(见附录：交易类型枚举) |
| 6. | 商户号 | member_id | M | Max11Numeric | 宝付提供给商户的唯一编号 |
| 7. | 商户订单号 | trans_id | M | Max50Text | 唯一订单号，8-50 位字母和数字,未支付成功的订单号可重复提交，重复提交时交易参数不得发生变化 |
| 8. | 数字信封 | dgtl_envlp | M | Max512Text | 格式：01\|对称密钥，01代表AES / 加密方式：Base64转码后使用宝付的公钥加密 |
| 9. | 用户ID | user_id | O | Max50Text | 用户在商户平台唯一ID |
| 10. | 订单金额 | order_amt | M | Max12Numeric | 单位：分,例：1元则提交100 / 订单金额=交易金额+营销金额 |
| 11. | 交易金额 | txn_amt | M | Max12Numeric | 单位：分,例：1元则提交100 / 银行卡扣款金额 |
| 12. | 营销账户信息 | union_info | O | 不限制 | 金额单位(分),格式:商户1,金额1;商户2,金额2… / 例如 100000363,5;100000364,5; |
| 13. | 签约协议号 | protocol_no | M | Max126Text | 加密方式：Base64转码后，使用数字信封指定的方式和密钥加密 |
| 14. | 风控参数 | risk_item | M | 不限制 | Json格式，详细参数见风控参数字段说明（通用参数、电商、互金消金、航旅、酒店、宝信、游戏、大宗） |
| 15. | 交易成功通知地址 | return_url | O | Max500Text | 最多填写三个地址,不同的地址用‘\|’连接 |
| 16. | 平台商户号 | platform_no | O | Max32Text | 宝付提供给商户的唯一编号，2.0是收单商户号 |
| 17. | 上级平台编号 | sub_merchant_no | O | Max32Text | 已升级为特约的二级平台编号 |
| 18 | 平台交易类型 | payment_type | O | Max32Text | 示例值：1、2、3 ，传值1或不传 代表银行交易回单展示sub_merchant_no信息，传值2代表特定支通道需收集sub_merchant_no信息 ，传值3代表同时实现1跟2的功能 |
| 19. | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 20. | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 21. | 系统保留域1 | additional_info1 | O | Max255Text | |
| 22. | 系统保留域2 | additional_info2 | O | Max255Text | |
| 23. | 系统保留域3 | additional_info3 | O | Max255Text | |
| 24. | 签名域 | signature | M | Max512Text | |

#### 返回报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1. | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2. | 应答报文流水号 | msg_id | M | Max32Text | |
| 3. | 报文编号/版本号 | version | M | Max7Text | 4.0.0.0 |
| 4. | 应答码 | resp_code | M | Max16Text | 具体参见附录5：商户接口应答码 |
| 5. | 终端号 | terminal_id | R | Max11Numeric | |
| 6. | 交易类型 | txn_type | R | Max11Text | |
| 7. | 商户号 | member_id | R | Max11Numeric | 宝付提供给商户的唯一编号 |
| 8. | 业务返回码 | biz_resp_code | M | | 具体参见附录1：业务应答码 |
| 9. | 业务返回说明 | biz_resp_msg | M | | |
| 10. | 数字信封 | dgtl_envlp | M | Max512Text | 格式：01\|对称密钥，01代表AES / 加密方式：Base64转码后使用商户的公钥加密 |
| 11. | 预支付唯一码 | unique_code | C | Max126Text | 预支付成功后返回 / 加密方式：Base64转码后，使用数字信封指定的方式和密钥加密 |
| 12. | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 13. | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 14. | 系统保留域1 | additional_info1 | O | Max255Text | |
| 15. | 系统保留域2 | additional_info2 | O | Max255Text | |
| 16. | 签名域 | signature | M | Max512Text | |

文档更新时间: 2026-01-07 01:53 作者：超级管理员
