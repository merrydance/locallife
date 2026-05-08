---
title: 账户余额查询接口
slug: queryBalace
source_url: https://doc.mandao.com/docs/bct/queryBalace
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 9588ec7769e6b83cc1f22c2f4616e1e17b040f612f87bc3b356731021003c432
doc_version: 1736330912
---

# 账户余额查询接口
查询账户二级账户余额：账簿余额、可用余额、在途余额

> **账簿余额=可用余额+在途余额**  
> 在途余额和可用余额关系：支付订单分账到二级户后先进入在途余额，待资金结算（注：结算周期可与商务确认）后；在途余额资金清算到可用余额

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-06

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.0.0 |
| contractNo | String | 32 | M | 客户号 |
| accType | int | 4 | M | 账户类型:1个人,2商户 |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| retCode | int | 4 | M | 返回码 1 成功 0 失败 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| back1 | String | 64 | O | 备用字段 |
| back2 | String | 64 | O | 备用字段 |
| back3 | String | 100 | O | 备用字段 |
| availableBal | BigDecimal | 10,2 | O | 账簿可用余额,单位：元;可用于提现 |
| pendingBal | BigDecimal | 10,2 | O | 在途资金余额,单位：元 |
| currBal | BigDecimal | 10,2 | O | 账簿余额,单位：元; / 账簿余额=可用余额(availableBal)+在途余额(pendingBal)+冻结金额 |

说明：  
A）“冻结金额”  
发生退款时，可能瞬时产生“冻结金额”。

#### 示例

    {
        "availableBal": 0,
        "currBal": 0,
        "errorCode": "SUCCESS",
        "errorMsg": "成功",
        "freezeBal": 0,
        "retCode": 1
    }

文档更新时间: 2025-09-16 09:09   作者：超级管理员
