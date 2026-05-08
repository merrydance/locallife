---
title: 提现查询接口
slug: queryWithdrawal
source_url: https://doc.mandao.com/docs/bct/queryWithdrawal
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 51a26b280d1182a457e361733bb9eaaac1812212ed8e86f138b2e4e4358dc005
doc_version: 1729481103
---

# 提现查询接口
提现结果查询，提现结果需使用接口中的state 状态字段判断

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-15

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.2.0 |
| transSerialNo | String | 50 | M | 商户订单号 |
| tradeTime | String | 10 | M | 交易时间 yyyy-MM-dd |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| retCode | int | 4 | M | 返回码 1 成功 0 失败 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| back1 | String | 64 | O | 备用字段 |
| back2 | String | 64 | O | 备用字段 |
| back3 | String | 100 | O | 备用字段 |
| memberId | String | 32 | M | 商户号 |
| transSerialNo | String | 50 | M | 商户订单号 |
| state | int | 4 | M | 订单状态 1成功 0失败 2处理中 3提现退回 |
| orderId | long | 20 | O | 订单号 |
| successTime | String | 19 | O | 成功时间，格式：yyyy-MM-dd HH:mm:ss |
| contractNo | String | 32 | M | 商户客户号 |
| transMoney | BigDecimal | 10,2 | M | 转账金额,单位：元 |
| transFee | BigDecimal | 10,2 | M | 转账手续费,单位：元 |
| transferTotalAmount | BigDecimal | 10,2 | M | 转账交易时金额,单位：元 |
| transRemark | String | 200 | O | 失败原因 |

#### 示例

    {
        "contractNo": "CM690000000000000348",
        "memberId": "100030220",
        "orderId": 21273130,
        "retCode": 1,
        "state": 1,
        "transFee": 1,
        "transMoney": 10.01,
        "transSerialNo": "TSN162116291523594396928312",
        "transferTotalAmount": 10.01
    }

文档更新时间: 2025-09-16 09:09   作者：超级管理员
