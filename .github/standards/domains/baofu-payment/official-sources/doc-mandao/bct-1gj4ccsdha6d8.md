---
title: 个人开户接口(二要素)
slug: bct-1gj4ccsdha6d8
source_url: https://doc.mandao.com/docs/bct/bct-1gj4ccsdha6d8
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 7c18f53350f90f1002b6dcc0747ce20c444eb92e60084225ed14eb9080f92795
doc_version: 1750999679
---

# 个人开户接口(二要素)
接口用于宝付账簿开户，开户成功后每个账户下面会有2个类型账户：在途户+可用余额户，分账到二级户的资金先记账在途户，资金结算后才会到可用余额户，可用余额户的资金可以提现

------------------------------------------------------------------------

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.1.0 |
| accType | int | 1 | M | 账户类型:1-个人,2-企业/个体 |
| accInfo | Object |  | M | 开户具体信息根据类型不同,信息不同,现只支持单笔开户 |
| noticeUrl | String | 256 | M | 开户结果通知地址，通知参数详见[开户结果通知](https://doc.mandao.com/docs/bct/openAccNotify) |
| businessType | String | 32 | M | 宝财通2.0: BCT2.0 |

**accType=1个人用户开户信息列表accInfo**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| transSerialNo | String | 200 | M | 请求流水号 |
| loginNo | String | 32 | M | 登录号,商户自定义要求全局唯一，长度11位以上 |
| customerName | String | 32 | M | 客户名称与持卡人姓名一致 |
| certificateType | String | 16 | M | 证件类型,身份证: ID |
| certificateNo | String | 64 | M | 身份证号码 |
| cardUserName | String | 20 | M | 持卡人姓名 |
| needUploadFile | boolean |  | M | 是否需要上传附件,true/false |
| platformNo | String | 32 | C | 平台号 (代理模式下此处为业务方商户号非代理商商户号) |
| platformTerminalId | String | 32 | C | 终端号(代理模式必传) |
| qualificationTransSerialNo | String | 128 | C | 资质文件流水,是否上传请咨询业务对接人 |

**请求参数示例**

    {
        "noticeUrl": "http://10.0.60.55:8083/BctJavaDemo2.0/NorifyServlet/open/1",
        "accType": "1",
        "businessType": "BCT2.0",
        "version": "4.1.0",
        "accInfo": {
            "needUploadFile": false,
            "loginNo": "person002",
            "certificateNo": "340101198108119852",
            "transSerialNo": "TSN314778753119603185643720",
            "customerName": "张宝",
            "certificateType": "ID"
        }
    }

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| retCode | int | 4 | M | 返回码 1 成功 0 失败 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| back1 | String | 64 | O | 备用字段 |
| back2 | String | 64 | O | 备用字段 |
| back3 | String | 100 | O | 备用字段 |
| [result](#result) | List |  |  | 返回数据列表 |

  
**列表result**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| state | String | 4 | M | 状态 1 成功 0 失败 -1 异常 2开户处理中 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| transSerialNo | String | 200 | M | 请求流水号 |
| loginNo | String | 128 | M | 登录号 |
| customerName | String | 64 | M | 商户名称 |
| contractNo | String | 64 | C | 商户客户号 |

**返回参数示例**

    {
        "result": [{
            "customerName": "张宝",
            "loginNo": "person002",
            "state": 2,
            "transSerialNo": "TSN314778753119603185643720"
        }],
        "retCode": 1
    }

文档更新时间: 2025-09-16 09:09   作者：超级管理员
