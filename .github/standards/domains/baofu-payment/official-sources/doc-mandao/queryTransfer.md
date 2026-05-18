---
title: 转账结果查询（账户间）
slug: queryTransfer
source_url: https://doc.mandao.com/docs/bct/queryTransfer
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 03b05590bb0bc363923963aa91dfe71dff0cd3d32888baacbf67a2b184749232
doc_version: 1772182855
---

# 转账结果查询（账户间）
转账结果查询，转账结果可使用state 状态字段判断

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-10

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.0.0 |
| transSerialNo | String | 50 | M | 原转账订单请求流水号 |
| tradeTime | String | 10 | M | 原交易时间 yyyy-MM-dd |

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
| feeAmount | BigDecimal | 10,2 | M | 手续费金额,单位：元 |
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
        "transSerialNo": "TSN770577582499782631561319"
    }

文档更新时间: 2026-02-27 09:00   作者：超级管理员
