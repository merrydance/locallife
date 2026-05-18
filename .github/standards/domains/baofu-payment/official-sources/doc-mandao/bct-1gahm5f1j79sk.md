---
title: 账户升级接口
slug: bct-1gahm5f1j79sk
source_url: https://doc.mandao.com/docs/bct/bct-1gahm5f1j79sk
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 2e928c1053cfdf122645595fab4a42028735156e07b241aef50814a9fa5b94f6
doc_version: 1745825546
---

# 账户升级接口
接口用于宝付账簿二级户升级为宝付特约商户。此接口申请提交后，同步返回受理结果。具体是否升级成功需要结合绑定关系查询接口查得。

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-20

当header的 sysRespCode 为S_0000时，body的retCode如下：  
1.retCode=1 说明接口调用成功。具体业务是否成功。看具体的参数字段。  
2.retCode=0 说明接口调用失败。异常或者参数校验失败。  
3.retCode=2 说明接口调用处理中。需要调用查询接口查询状态。

当header的 sysRespCode 为非S_0000时，系统异常或者校验失败。和具体业务无关联。

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.1.0 |
| contractNo | String | [1,32] | M | 平台号 |
| qualificationTransSerialNo | String | [1,32] | M | 资质文件请求流水号 / 上传文件类型必须有： / 101 企业营业执照 / 102 银行开户许可证 / 104 法人身份证(正) / 111 法人身份证(反) |
| province | String | [1,32] | M | 公司地址-省份 |
| city | String | [1,32] | M | 公司地址-城市 |
| district | String | [1,32] | M | 公司地址-区 |
| detailedAddress | String | [1,512] | M | 公司地址-详细地址 |
| businessAddress | String | [1,512] | M | 经营地址 |
| registeredAddress | String | [1,512] | M | 注册地址 |
| legalPersonIdValidityPeriod | String | [1,8] | M | 法人证件有效期 yyyyMMdd |
| businessScope | String | [1,1500] | M | 企业经营范围 |
| establishmentDate | String | [1,8] | M | 成立时间 yyyyMMdd |
| registeredCapital | String | [1,20] | M | 注册资本 单位万元 |
| businessExecutionValidityPeriod | String | [1,8] | M | 营业期限 yyyyMMdd,长期99991231 |
| requestNo | String | [1,32] | M | 请求流水号 |
| contactMobile | String | [1,32] | O | 联系人手机号,用于接收合作协议电子签验证码。开户时未传此字段必填 |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| retCode | int | 4 | M | 返回码 1 成功 0 失败 2 处理中 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| back1 | String | 64 | O | 备用字段 |
| back2 | String | 64 | O | 备用字段 |
| back3 | String | 100 | O | 备用字段 |

文档更新时间: 2025-09-16 09:09   作者：超级管理员
