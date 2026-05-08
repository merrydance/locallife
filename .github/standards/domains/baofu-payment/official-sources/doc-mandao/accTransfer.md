---
title: 转账（账户间）
slug: accTransfer
source_url: https://doc.mandao.com/docs/bct/accTransfer
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: a92b8e8d12be6623ce720cee0b2579dc538edd427143779a4cda1f8aed777877
doc_version: 1772182836
---

# 转账（账户间）
支持同一商户下二级子商户可用余额可以互相转账

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-13

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.0.0 |
| payerNo | String | 32 | M | 付款方(二级子商户号) |
| payeeNo | String | 32 | M | 收款方(二级子商户号) |
| transSerialNo | String | 50 | M | 请求流水号 |
| dealAmount | BigDecimal | 10,2 | M | 转账金额,单位：元 |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| retCode | int | 4 | M | 返回码 1 成功 0 失败 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| back1 | String | 64 | O | 备用字段 |
| back2 | String | 64 | O | 备用字段 |
| back3 | String | 100 | O | 备用字段 |
| transSerialNo | String | 50 | M | 请求流水号 |
| businessNo | String | 255 | O | 业务流水号 |
| payerNo | String | 32 | M | 付款方(二级子商户号) |
| payeeNo | String | 32 | M | 收款方(二级子商户号) |
| dealAmount | BigDecimal | 10,2 | M | 转账金额,单位：元 |
| feeAmount | BigDecimal | 10,2 | O | 手续费金额,单位：元 |
| state | int | 2 | M | 订单状态 1成功 2失败 |
| transRemark | String | 128 | O | 失败原因 |

#### 示例

    {
        "businessNo": "20240108153456010333300001001020",
        "dealAmount": 10.1,
        "feeAmount": 0,
        "payeeNo": "CP690000000000011438",
        "payerNo": "CM690000000000000348",
        "retCode": 1,
        "state": 1,
        "transRemark": "",
        "transSerialNo": "TSN770577582499782631561319"
    }

文档更新时间: 2026-02-27 09:00   作者：超级管理员
