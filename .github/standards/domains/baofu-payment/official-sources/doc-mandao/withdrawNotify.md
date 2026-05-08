---
title: 提现结果通知
slug: withdrawNotify
source_url: https://doc.mandao.com/docs/bct/withdrawNotify
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 1fad8ea386b25a1588932fc5706b3248d38fa93f3715031f4c2593413d9e9fc0
doc_version: 1744786210
---

# 提现结果通知
------------------------------------------------------------------------

#### 接口说明:

1.付款订单最终交易状态以代付state为准  
2.商户接收到通知后务必在接收通知后返回大写OK  
3.宝付系统在未确认商户接收通知成功后将会通过重发机制通知商户（重发次数10次，请以第一次收到的付款成功的消息为准，避免进行多次确认）通知发给商户。  
4.该接口除了订单成功、失败结果通知外，退款的结果也一并通知。  
5.通知参数格式：<http://URL?member_id=1&terminal_id=2&data_type=JSON&data_content=密文>  
**异步通知规则**  
1、地址通且有返回OK则后续不再通知  
2、24小时通知10次，分别为：0，2，5，10，30，60，90，180，240，820 单位是分钟

#### 通知参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| member_id | String | 10 | M | 商户号 |
| terminal_id | String | 10 | M | 终端号 |
| data_type | String | 5 | M | JSON,data_content的明文组装格式 |
| data_content | String | 128 | M | 加密数据，需使用宝付公钥解密后再做base64解码 |

#### data_content解密参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| contractNo | String | 32 | M | 提现子商户号 |
| orderId | String | 20 | M | 提现订单号 |
| transSerialNo | String | 50 | M | 商户订单号 |
| transMoney | BigDecimal | 10,2 | M | 转账金额,单位：元 |
| transFee | BigDecimal | 10,2 | M | 费用,单位：元 |
| transferTotalAmount | BigDecimal | 10,2 | M | 转账交易时金额,单位：元 |
| state | String | 4 | M | 状态枚举 / 0:失败 / 1:成功 / 2:处理中 / 3:提现退回 注：此状态只在提现时上送版本号4.2.0返回 |
| transRemark | String | 128 | M | 失败原因 |
| reqReserved | String | 512 | M | 保留域 |

##### data_content示例

    {
        "contractNo": "CP690000000000000258",
        "orderId": "21201148",
        "reqReserved": "",
        "state": "1",
        "transFee": "1.00",
        "transMoney": "10.01",
        "transRemark": "",
        "transSerialNo": "TID277461406486212443164402",
        "transferTotalAmount": "10.01"
    }

文档更新时间: 2025-09-16 09:09   作者：超级管理员
