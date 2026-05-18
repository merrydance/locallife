---
title: 账户信息修改接口
slug: updateCard
source_url: https://doc.mandao.com/docs/bct/updateCard
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 1842b574f2a0762e1ed8ed16d911f4b9747e3af3dd779fe8a10abdd09acee4e1
doc_version: 1775118793
---

# 账户信息修改接口
修改开户绑定卡信息，目前只支持绑定一张卡，需要换卡可以调用此即可.支持修改户名和法人信息

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-02

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.0.0 |
| accType | int | 1 | M | 账户类型:1个人,2商户 |
| accInfo | Object |  | M | 开户具体信息根据类型不同,信息不同,现只支持单笔修改 |

**accType=1个人用户开户信息列表accInfo**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| contractNo | String | 64 | M | 客户账户号 |
| transSerialNo | String | 200 | M | 请求流水号 |
| cardNo | String | 128 | C | 卡号影响计费 / |
| mobileNo | String | 64 | C | 银行预留手机号 |
| cardUserName | String | 20 | C | 持卡人姓名影响计费 / |

**accType=2机构用户开户信息列表accInfo**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| contractNo | String | 32 | M | 客户账户号 |
| transSerialNo | String | 200 | M | 请求流水号 |
| cardNo | String | 128 | C | 卡号 影响计费-对私结算 / (修改卡号时必传) |
| cardUserName | String | 60 | C | 持卡人姓名 |
| bankName | String | 20 | C | 银行名称 影响计费-对私结算 / (修改卡号时必传) |
| depositBankProvince | String | 20 | C | 开户行省份 (修改卡号时必传) |
| depositBankCity | String | 20 | C | 开户行城市 (修改卡号时必传) |
| depositBankName | String | 64 | C | 开户支行 影响计费-对私结算 / (修改卡号时必传) |
| contactName | String | 20 | O | 联系人姓名 |
| contactMobile | String | 64 | O | 联系人手机号 |
| corporateMobile | String | 64 | O | 法人手机号影响计费-对私结算 / 当开个体户且绑定对私卡时必传 |
| customerName | String | 60 | C | 公司名称影响计费 |
| corporateName | String | 20 | C | 法人姓名影响计费 |
| corporateCertId | String | 32 | C | 法人身份证号影响计费 |
| aliasName | String | 64 | O | 商户名称别名 |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| retCode | int | 4 | M | 返回码 1 成功 0 失败 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| back1 | String | 64 | O | 备用字段 |
| back2 | String | 64 | O | 备用字段 |
| back3 | String | 100 | O | 备用字段 |
| contractNo | String | 64 | M | 商户客户号 |

#### 示例

    {
        "contractNo": "CP690000000000004278",
        "retCode": 1
    }

文档更新时间: 2026-04-02 08:33   作者：超级管理员
