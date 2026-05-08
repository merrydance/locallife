---
title: 个人/机构开户接口
slug: openAcc
source_url: https://doc.mandao.com/docs/bct/openAcc
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 264aee970c2a8a1a1367ae8a714bf06f996a7e132d750d4f4f0f17d7ace641cd
doc_version: 1768204691
---

# 个人/机构开户接口
接口用于宝付账簿开户，开户成功后每个账户下面会有2个类型账户：在途户+可用余额户，分账到二级户的资金先记账在途户，资金结算后才会到可用余额户，可用余额户的资金可以提现

------------------------------------------------------------------------

#### 接口说明

1.报文编号：T-1001-013-01

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
| cardNo | String | 128 | M | 卡号 |
| mobileNo | String | 64 | M | 银行预留手机号 |
| cardUserName | String | 20 | M | 持卡人姓名 |
| needUploadFile | boolean |  | M | 是否需要上传附件,true/false |
| platformNo | String | 32 | C | 平台号 (代理模式下此处为业务方商户号非代理商商户号) |
| platformTerminalId | String | 32 | C | 终端号(代理模式必传) |
| qualificationTransSerialNo | String | 128 | C | 资质文件流水,是否上传请咨询业务对接人 |

**accType=2 企业/个体用户开户信息列表accInfo**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| transSerialNo | String | 200 | M | 请求流水号 |
| loginNo | String | 32 | M | 登录号,商户自定义要求全局唯一，长度11位以上 |
| email | String | 32 | M | 邮箱 |
| selfEmployed | boolean | 1 | C | 是否个体户 企业为false，不传默认为false |
| customerName | String | 64 | M | 商户名称（营业执照上的名称） |
| aliasName | String | 64 | O | 商户名称别名 |
| certificateNo | String | 64 | M | 证件号码 |
| certificateType | String | 16 | M | 证件类型 营业执照:LICENSE |
| corporateName | String | 20 | M | 法人姓名 |
| corporateCertType | String | 10 | M | 法人证件类型：身份证-ID， / 港澳通行证-HONG_KONG_AND_MACAO_PASS / 台湾同胞来往内地通行证-TAIWAN_TRAVEL_PERMIT / 护照-PASSPORT |
| corporateCertId | String | 64 | M | 法人身份证号码/港澳通行证/台湾同胞来往内地通行证 |
| corporateMobile | String | 64 | C | 法人手机号 / 当开个体户(selfEmployed=true)且绑定对私卡时必传 |
| industryId | String | 11 | M | 公司所属行业 见[附录](https://doc.mandao.com/docs/bct/appendix) |
| contactName | String | 20 | O | 联系人姓名 |
| contactMobile | String | 64 | O | 联系人手机号 |
| cardNo | String | 128 | M | 卡号 |
| bankName | String | 20 | M | 银行名称 |
| depositBankProvince | String | 20 | M | 开户行省份 |
| depositBankCity | String | 20 | M | 开户行城市 |
| depositBankName | String | 64 | M | 开户支行名称 |
| registerCapital | String | 64 | C | 注册资本 |
| cardUserName | String | 64 | O | 持卡人姓名 / 当开个体户且绑定对私卡时需传此字段，长度为20；若不是对私卡传输需要与企业绑定对公卡名称保持一致，不传则默认绑定对公卡 |
| platformNo | String | 32 | C | 平台号(主商户号) (代理模式必传) |
| platformTerminalId | String | 32 | C | 终端号(代理模式必传) |
| qualificationTransSerialNo | String | 128 | C | 资质文件流水,businessType为宝财通2.0非必填 |

**请求参数示例**

    {
        "noticeUrl": "http://10.0.60.55:8083/BctJavaDemo2.0/NorifyServlet/open/1",
        "accType": "1",
        "businessType": "BCT2.0",
        "version": "4.1.0",
        "accInfo": {
            "needUploadFile": false,
            "mobileNo": "13567796514",
            "loginNo": "person002",
            "certificateNo": "340101198108119852",
            "cardNo": "6217001210075124519",
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

文档更新时间: 2026-01-12 07:58   作者：超级管理员
