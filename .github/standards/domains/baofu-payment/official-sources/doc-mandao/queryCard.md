---
title: 查询绑定卡接口
slug: queryCard
source_url: https://doc.mandao.com/docs/bct/queryCard
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: d5492a1ebe02e578ec943fb130af64d323090880c2bf026789cbd2fc129ede78
doc_version: 1712459016
---

# 查询绑定卡接口
查询账户绑定卡信息

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-08

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.0.0 |
| loginNo | String | 128 | C | 登录号(无商户客户号必填) |
| contractNo | String | 32 | C | 客户账户号 |
| accType | int | 1 | M | 账户类型:1个人,2商户 |
| platformNo | String | 32 | C | 平台号(主商户号)(无商户客户号必填) |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| retCode | int | 4 | M | 返回码 1 成功 0 失败 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| bindCardInfoList | List | | C | 绑卡信息,当retCode=1是有值 |

**bindCardInfoList (accType=1)**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| cardUserName | String | 32 | M | 客户名称 |
| cardNo | String | 128 | M | 卡号 |
| bankName | String | 20 | M | 银行名称 |

**bindCardInfoList (accType=2)**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| cardUserName | String | 32 | M | 客户名称 |
| cardNo | String | 128 | M | 卡号 |
| bankName | String | 20 | M | 银行名称 |
| depositBankProvince | String | 20 | M | 开户行省份 |
| depositBankCity | String | 20 | M | 开户行城市 |
| depositBankName | String | 64 | M | 开户支行名称 |

#### 示例

```
 {
 "bindCardInfoList": [
 {
 "bankName": "工商银行",
 "cardNo": "6222026764734682050",
 "cardUserName": "张宝"
 }
 ],
 "retCode": 1
}
```

文档更新时间: 2025-09-16 09:09 作者：超级管理员
