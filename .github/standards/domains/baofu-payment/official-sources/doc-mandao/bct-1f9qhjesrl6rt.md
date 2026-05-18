---
title: 预绑卡
slug: bct-1f9qhjesrl6rt
source_url: https://doc.mandao.com/docs/bct/bct-1f9qhjesrl6rt
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 3516e283ddbae1632bc61acc761b38c48eb110e2068b20e0e4afb9449e8a7d12
doc_version: 1704422784
---

# 预绑卡
绑定银行卡是指经过持卡人授权将个人银行卡和商户建立绑定关系，支付时不再需要输入银行卡信息。商户需先进行预绑卡，宝付（或银行）会发送短信验证码（可根据银行要求来配置是否发送）给持卡人，商户再使用确认绑卡接口将短信验证码回传给宝付完成绑卡。

------------------------------------------------------------------------

#### 交易URL

- 测试环境地址：
 [https://vgw.baofoo.com/cutpayment/protocol/backTransRequest](https://vgw.baofoo.com/cutpayment/protocol/backTransRequest)
- 正式环境地址：
 [https://public.baofoo.com/cutpayment/protocol/backTransRequest](https://public.baofoo.com/cutpayment/protocol/backTransRequest)

#### 请求报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1 | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2 | 报文流水号 | msg_id | M | Max32Text | 商户流水号 |
| 3 | 报文编号/版本号 | version | M | Max7Text | 4.0.0.0 |
| 4 | 终端号 | terminal_id | M | Max11Numeric | |
| 5 | 交易类型 | txn_type | M | code | 固定值：11(见附录：交易类型枚举) |
| 6 | 商户号 | member_id | M | Max11Numeric | 宝付提供给商户的唯一编号 |
| 7 | 数字信封 | dgtl_envlp | M | Max512Text | 格式：01\|对称密钥，01代表AES;加密方式：Base64转码后使用宝付的公钥加密 |
| 8 | 用户ID | user_id | O | Max50Text | 用户在商户平台唯一ID |
| 9 | 卡类型 | card_type | M | code | 见附录：卡类型 |
| 10 | 证件类型 | id_card_type | M | code | 见附录：证件类型 |
| 11 | 账户信息 | acc_info | M | ISODateTime | 格式：***银行卡号\|持卡人姓名\|证件号\|手机号\|银行卡安全码\|银行卡有效（yymm）***, / 加密方式：Base64转码后，使用数字信封指定的方式和密钥加密 |
| 12 | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 13 | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 14 | 系统保留域1 | additional_info1 | O | Max255Text | |
| 15 | 系统保留域2 | additional_info2 | O | Max255Text | |
| 16 | 签名域 | signature | M | Max255Text | |

#### 返回报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1 | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2 | 应答报文流水号 | msg_id | M | Max32Text | |
| 3 | 报文编号/版本号 | version | R | Max7Text | 4.0.0.0 |
| 4 | 应答码 | resp_code | M | Max11Numeric | 具体参见附录：商户接口应答码 |
| 5 | 终端号 | terminal_id | R | Max11Numeric | |
| 6 | 交易类型 | txn_type | R | code | |
| 7 | 商户号 | member_id | R | Max11Numeric | 宝付提供给商户的唯一编号 |
| 8 | 业务返回码 | biz_resp_code | M | Max50Text | 具体参见附录：业务应答码 |
| 9 | 业务返回说明 | biz_resp_msg | M | Max50Text | |
| 10 | 预签约唯一码 | unique_code | C | Max126Text | 加密方式：Base64转码后，使用数字信封指定的方式和密钥加密 |
| 11 | 数字信封 | dgtl_envlp | M | Max512Text | 格式：01\|对称密钥，01代表AES;加密方式：Base64转码后使用商户的公钥加密 |
| 12 | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 13 | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 14 | 系统保留域1 | additional_info1 | O | Max255Text | |
| 15 | 系统保留域2 | additional_info2 | O | Max255Text | |
| 16 | 签名域 | signature | M | Max512Text | |

文档更新时间: 2024-06-05 05:22 作者：超级管理员
