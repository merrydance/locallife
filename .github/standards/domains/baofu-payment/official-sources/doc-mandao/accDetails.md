---
title: 账户收支明细查询
slug: accDetails
source_url: https://doc.mandao.com/docs/bct/accDetails
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: f1341538f3b913ad35afa749d284ff2313182a8483e3f7a7b30a0a84395c2894
doc_version: 1769131946
---

# 账户收支明细查询
查询账户收支明细

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-11

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.0.0 |
| contractNo | String | 32 | M | 商户客户号 |
| accountType | String | 32 | O | BALANCE-余额户,TRANSIT-在途户,不传默认余额户 |
| startTime | String | 32 | O | 明细开始时间 yyyy-MM-dd HH：mm：ss |
| endTime | String | 32 | O | 明细结束时间 yyyy-MM-dd HH：mm：ss查询间隔最大支持一个月 |
| pageNum | int | 32 | M | 开始页 |
| pageSize | int | 32 | M | 每页显示记录数,最大100 |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| retCode | int | 4 | M | 返回码 1 成功 0 失败 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| back1 | String | 64 | O | 备用字段 |
| back2 | String | 64 | O | 备用字段 |
| back3 | String | 100 | O | 备用字段 |
| pageNum | int | 32 | O | 开始页 |
| pageSize | int | 32 | O | 每页显示记录数 |
| pageCount | int | 32 | O | 总页数 |
| list | List |  | O | 数据列表 |

**list数据列表**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| transType | String | 40 | O | 交易类型 :RECHARGE 入金,TRANSFER 划拨 ,WITHDRAW 出金 |
| drCrFlag | String | 10 | O | 余额方向：CR-贷款（收入）/ DR-借款（支出） |
| ccy | String | 10 | O | 币种 CNY人民币 |
| amount | BigDecimal | 10,2 | O | 交易金额,单位：元 |
| afterBal | BigDecimal | 10,2 | O | 交易后余额,单位：元 |
| creatTime | String | 32 | O | 创建时间yyyy-MM-dd HH:mm:ss |
| orderId | String | 32 | O | 宝付订单号 |
| transSerialNo | String | 200 | O | 商户订单号 |
| businessTadeType | String | 200 | O | SHARE-分账 / OFFSET_SHARE-差额分账 / REFUND-退款 / TRANSFER-转账 / WITHDRAW-提现 / CLEAR-资金清算 / OTHER-其他 / WITHDRAW_CANCEL-提现退回 |
| partyAcctName | String | 200 | O | 对手方户名 |
| partyAcctNo | String | 32 | O | 对手方账号 |

#### 示例

    {
        "list": [],
        "pageCount": 0,
        "pageNum": 1,
        "pageSize": 0,
        "retCode": 1
    }

文档更新时间: 2026-01-23 01:32   作者：超级管理员
