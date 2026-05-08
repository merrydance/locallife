---
title: 开户查询接口
slug: queryAcc
source_url: https://doc.mandao.com/docs/bct/queryAcc
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 5a7e7da48c7655ec915ab04b99685deaadde740e08274bb41c151b1e2ca515e5
doc_version: 1768269672
---

# 开户查询接口
查询开户的客户号等相关信息

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-03

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.0.0 |
| certificateNo | String | 64 | C | 证件号码（社会信用代码） |
| certificateType | String | 16 | C | 证件类型 只能取”ID”或”LICENSE” |
| platformNo | String | 32 | C | 平台号(主商户号) |
| loginNo | String | 128 | C | 登录号(传此参数以上三个参数必填) |
| contractNo | String | 32 | C | 客户账户号(传此参数以上四个参数可以不填) |
| accType | int | 1 | M | 账户类型:1个人,2商户 |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| retCode | int | 4 | M | 返回码 1 成功 0 失败 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| [result](#result) | Object |  | C | 返回数据列表，当retCode=1时有值 |

##### result数据列表

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| contractNo | String | 32 | M | 客户账户号 |
| contractName | String | 20 | O | 客户账户名 |
| customerName | String | 20 | O | 客户名 |
| customerNo | String | 20 | O | 此字段暂用不到 |
| customerType | String | 16 | O | 客户类型 |
| certificateType | String | 16 | O | 证件类型 只能取”ID”或”LICENSE” |
| certificateNo | String | 64 | O | 证件号码（社会信用代码） |
| platformNo | String | 32 | O | 平台号 |
| bindMobile | String | 64 | O | 绑定手机号 |
| email | String | 128 | O | 邮箱 |

#### 示例

    {
        "errorCode": "SUCCESS",
        "errorMsg": "成功",
        "result": {
          "certificateNo": "420101196002294741",
          "certificateType": "ID",
          "contractName": "张宝",
          "contractNo": "CP690000000000004278",
          "customerName": "张宝",
          "customerNo": "51290000000000001198",
          "customerType": "1",
          "platformNo": "100030218"
        },
        "retCode": 1
    }

文档更新时间: 2026-01-13 02:01   作者：超级管理员
