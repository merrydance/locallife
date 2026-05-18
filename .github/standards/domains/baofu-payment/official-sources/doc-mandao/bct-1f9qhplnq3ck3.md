---
title: 交易结果异步通知
slug: bct-1f9qhplnq3ck3
source_url: https://doc.mandao.com/docs/bct/bct-1f9qhplnq3ck3
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 735f39b898c5be5081c6842868521afc6291bc9b2f014fad23209e4692fc86b5
doc_version: 1737524141
---

# 交易结果异步通知
> 如果直接支付类交易或预绑卡支付时上传了异步通知地址（return_url字段），当订单成功或失败时候会收到宝付的异步通知。  
> 异步通知以GET和POST方式发送到商户配置的接收地址，商户接收到支付结果，并且进行相应处理之后，需要商户接收通知的地址在页面上输出 OK 表示接收成功\<除了 OK 无任何其他内容\>，告诉宝付已经成功接收并处理完毕，宝付系统在未得到商户接收通知成功的反馈时，将通过重发机制再次通知商户（重发次数 2~10 次，请以第一次收到的支付成功的消息为准，避免进行多次充值或支付），直到商户接收成功或达到最大重发次数为止。  
> 例如：biz_resp_code=0000&biz_resp_msg=交易成功& member_id=100000749 &resp_code=S& trans_id =201803221785&signature=8ab74c7869632dc395cc945adcc388e6afceb759e4d406c3bb6e0e8002ec422f1615f2a43966d7337dcc57963f18877a959fe9f67b082da2cd95217ba003cc81f07962d665f576509ebc1a38f7ddf2a423775a794b262b7ffc4af615da3ba6bd05d0672c004d7cf80be3ed236f268078bb5c700d4b0a6ae9a0e58f2c782bd6ef&terminal_id=100000949& order_id =58752185& succ_amt =585& succ_time =2018-01-24 13:25:33

#### 返回报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1 | 应答码 | resp_code | M | Max16Text | 具体参见附录5：商户接口应答码 |
| 2 | 终端号 | terminal_id | R | Max11Numeric |  |
| 3 | 商户号 | member_id | R | Max11Numeric | 宝付提供给商户的唯一编号 |
| 4 | 业务返回码 | biz_resp_code | M | Max16Text | 具体参见附录1：业务应答码 |
| 5 | 业务返回说明 | biz_resp_msg | M |  |  |
| 6 | 宝付订单号 | order_id | M | Max32Numeric |  |
| 7 | 商户原始订单号 | trans_id | M | Max50Text | 商户支付时上传的订单号 |
| 8 | 成功金额 | succ_amt | C | Max12Numeric | 单位：分。例：1元则100，订单成功时返回 |
| 9 | 成功时间 | succ_time | C | ISODateTime | 支付成功时间，订单成功时返回 |
| 10 | 签名域 | signature | M | Max512Text |  |
| 11 | 交易金额 | trade_amt | C | Max12Numeric | 单位：分。 |
| 12 | 营销金额 | market amt | C | Max12Numeric | 单位：分。 |

文档更新时间: 2025-01-22 05:35   作者：超级管理员
