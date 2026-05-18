---
title: 账户提现接口
slug: accWithdrawal
source_url: https://doc.mandao.com/docs/bct/accWithdrawal
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 75f3447107c35bc2eaf3c2d19f352fe81f023847bec5fd10fdfffc415448c992
doc_version: 1748517741
---

# 账户提现接口
账户可用余额可进行提现，资金提现到绑定银行卡账户，接口只返回受理结果，并不代理提现结果，提现结果需使用异步通知或者查询接口中的state 状态字段判断

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-14

#### 接口版本：

| 版本号 | 修订日期 | 修订内容 |
| --- | --- | --- |
| 4.0.0 | 2024-03-01 | 初始版本 |
| 4.2.0 | 2024-12-27 | 此版本提现结果通知接口新增返回订单状态3:提现退回 |

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.2.0 |
| contractNo | String | 32 | M | 客户账户号 |
| directPlatformNo | String | 32 | C | 上级客户账户号 |
| transSerialNo | String | 50 | M | 商户订单号 |
| dealAmount | BigDecimal | 10,2 | M | 提现金额,单位：元 |
| returnUrl | String | 256 | M | 提现结果异步通知地址，通知参数详见[提现结果通知](https://doc.mandao.com/docs/bct/withdrawNotify) |
| feeMemberId | String | 32 | C | 用户自己承担手续费必传，与客户号contractNo一致需用户承担手续费时要提前和商务申请配置 |
| reqReserved | String | 512 | O | 原样返回保留字段 |
| transAbstract | String | 255 | C | 摘要 |

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
| contractNo | String | 32 | M | 客户账户号 |
| state | int | 4 | M | 订单状态 1受理成功 2受理失败 |
| transRemark | String | 512 | C | 订单备注（受理失败的具体原因） |

#### 示例

    {
        "contractNo": "CM690000000000000348",
        "retCode": 1,
        "state": 1,
        "transRemark": "",
        "transSerialNo": "TSN162116291523594396928312"
    }

文档更新时间: 2025-09-16 09:09   作者：超级管理员
