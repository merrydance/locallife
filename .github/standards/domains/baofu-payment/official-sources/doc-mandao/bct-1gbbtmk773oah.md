---
title: 资质文件上传接口1
slug: bct-1gbbtmk773oah
source_url: https://doc.mandao.com/docs/bct/bct-1gbbtmk773oah
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 3023e69b0131a1943efb74a146f335910227c20c6bb4b61c66963c15c6011082
doc_version: 1777105848
---

# 资质文件上传接口1
接口用于宝付账簿开户或账户升级上传资质文件使用。

------------------------------------------------------------------------

## 接口须知

| 符号 | 符号性质 | 符号说明 |
| --- | --- | --- |
| M | 强制域(Mandatory) | 必须填写的域 |
| C | 条件域(Conditional) | 某条件成立时必须填写的域 |
| O | 选用域(Optional) | 选填属性（可选预） |
| R | 原样返回域(Returned) | 必须与先前报文中对应域的值相同的域 |

## 报文说明

#### 请求报文

> - 请求报文格式
> key1=value1&key2=value2&key3=value3…
> 例如：
> memberId=100000749&terminalId=100000859&orderType=0&content=5a9c3ac419735d249e319727c89cfc0ce4a80d6a954980eaf3ea934316a56a121c758b0d13bf3302b877a8dd68619db72b2bd588ccdc9eb7fdb455705be1909df96540009146d7d81c96c0b90578f9344bd3fc00ded94d27c0c8040a83c02114b7a3a4698f830b7d0db60f230a5c3a4b38e7104088f2ee0139a4e765a9d79255

> - 敏感字段加密
> 加密方式：content为加密内容,加密内容格式为JSON,Base64转码后使用私钥RSA加密

| 域名 | 类型 | 出现要求 | 参数备注 |
| --- | --- | --- | --- |
| orderType | String | M | 类型值:0 宝财通上传文件 |
| memberId | String | M | 宝付商户号 |
| terminalId | String | M | 宝付终端号 |
| content | String | M | 上送参数为请求下列接口请求参数，内容格式为JSON字串进行Base64转码后使用私钥RSA加密后上送 |
| file | Multipart | M | 上传的文件尽量文件名命名格式如下，上送的文件为ZIP压缩包。多个文件在fileNameMap中标识出来。与压缩包文件名一致。 |

#### 返回参数

> - 返回报文格式
> 例如：
> 74652829c07a71983c0da582321818aec41364528626e0f90eac1c633755b9dab84593695f5a101401052e9c64d457a881e442206330215de2281d2a3ea15d79e6732e296fdc36c6e0c76d17376cf6b9fc978b50bc747a9536d93226a69aba587f9fa5227a9b2cb915d1b822753f4a86a9fa1d81bf4d106723d927cf0f6365fb

> - 解密方式：加密内容格式为JSON,使用公钥解密后Base64解码,如果商户号或终端不正确,无法解密会返回明文

## 文件同步上传接口

#### 接口URL

准生产环境地址：[https://vgw.baofoo.com/baofu-upload-trade/trade/syncUploadFile](https://vgw.baofoo.com/baofu-upload-trade/trade/syncUploadFile) 正式环境地址：[https://upload.baofoo.com/baofu-upload-trade/trade/syncUploadFile](https://upload.baofoo.com/baofu-upload-trade/trade/syncUploadFile)

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| accType | int | 1 | M | 账户类型:1个人,2商户,3 个体工商户 |
| transSerialNo | String | 200 | M | 请求流水号 |
| businessParams | String | 128 | M | Ordertype 0 :自定义唯一标识，示例：登录号 |
| noticeUrl | String | 200 | O | 回调地址 |
| fileNameMap | List | | | 文件名映射表 |

**fileNameMap(文件对应表 List)**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| fileType | String | 1 | M | 详情开户文件类型表，参看附录《开户文件类型表》 |
| fileName | String | 200 | M | 文件类型对应文件名称（不包括扩展名） |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| success | boolean | 8 | M | |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| result | Json | | | 返回数据列表 |

**列表result**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| memberId | String | 32 | C | 宝付商户号 |
| terminalId | String | 32 | C | 宝付终端号 |
| businessParams | String | 100 | C | Ordertype 0 :自定义唯一标识，示例：登录号 |
| orderType | String | 200 | C | 订单类型 |
| transSerialNo | int | 1 | C | 请求流水号 |
| accType | int | 1 | C | 账户类型:1个人,2商户,3 个体工商户 |
| state | int | 1 | C | 订单状态1成功,2失败 |
| dfsFileNotifyList | List | | | 返回数据列表 |

**列表dfsFileNotifyList**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| fileName | String | 64 | M | 文件名称 |
| fileType | String | 1 | M | 文件类型 |
| fileId | String | 64 | M | 文件ID |
| dfsGroup | String | 32 | M | Dfs分组 |
| dfsFileName | String | 64 | O | Dfs文件名 |
| dfsPath | String | 100 | M | Dfs路径 |
| state | int | 1 | O | 文件上传状态 |

## 文件查询接口

#### 接口URL

准生产环境地址：[https://vgw.baofoo.com/baofu-upload-trade/trade/queryUploadFile](https://vgw.baofoo.com/baofu-upload-trade/trade/queryUploadFile) 正式环境地址：[https://upload.baofoo.com/baofu-upload-trade/trade/queryUploadFile](https://upload.baofoo.com/baofu-upload-trade/trade/queryUploadFile)

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| transSerialNo | String | 200 | M | 请求流水号 |
| businessParams | String | 128 | M | Ordertype 0 :自定义唯一标识，示例：登录号 |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| success | int | 4 | M | 返回码 1 成功 0 失败 -1 异常 2处理中 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| result | List | | | 返回数据列表 |

**列表result**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| memberId | String | 32 | C | 宝付商户号 |
| terminalId | String | 32 | C | 宝付终端号 |
| businessParams | String | 100 | C | Ordertype 0 :自定义唯一标识，示例：登录号 |
| orderType | String | 200 | C | 订单类型 |
| transSerialNo | int | 1 | C | 请求流水号 |
| accType | int | 1 | C | 账户类型:1个人,2商户,3 个体工商户 |
| state | int | 1 | C | 订单状态1成功,2失败 |
| sync | int | 1 | C | 同步状态0异步 1同步 |
| dfsFileNotifyList | List | | | 返回数据列表 |

**列表dfsFileNotifyList**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| fileName | String | 64 | M | 文件名称 |
| fileType | String | 1 | M | 文件类型 |
| fileId | String | 64 | M | 文件ID |
| dfsGroup | String | 32 | M | Dfs分组 |
| dfsFileName | String | 64 | O | Dfs文件名 |
| dfsPath | String | 100 | M | Dfs路径 |
| state | int | 1 | O | 文件重传/上传状态 |

## 文件重新上传接口

#### 接口URL

准生产环境地址：[https://qas-upload.baofoo.com/baofu-upload-trade/trade/syncReUploadFile](https://qas-upload.baofoo.com/baofu-upload-trade/trade/syncReUploadFile) 正式环境地址：[https://upload.baofoo.com/baofu-upload-trade/trade/syncReUploadFile](https://upload.baofoo.com/baofu-upload-trade/trade/syncReUploadFile)

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| accType | int | 1 | M | 账户类型:1个人,2商户,3 个体工商户 |
| transSerialNo | String | 200 | M | 请求流水号 |
| businessParams | String | 128 | M | Ordertype 0 :开户返回的客户账户号 |
| noticeUrl | String | 200 | O | 回调地址 |
| fileNameMap | List | | | 文件名映射表 |

**fileNameMap(文件对应表 List)**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| fileType | String | 1 | M | 详情开户文件类型表 |
| fileName | String | 200 | M | 文件类型对应文件名称（不包括扩展名） |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| success | int | 4 | M | 返回码 1 成功 0 失败 -1 异常 2处理中 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| result | List | | | 返回数据列表 |

**列表result**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| memberId | String | 32 | C | 宝付商户号 |
| terminalId | String | 32 | C | 宝付终端号 |
| businessParams | String | 100 | C | Ordertype 0 :自定义唯一标识，示例：登录号 |
| orderType | String | 200 | C | 订单类型 |
| transSerialNo | int | 1 | C | 请求流水号 |
| accType | int | 1 | C | 账户类型:1个人,2商户,3 个体工商户 |
| dfsFileNotifyList | List | | | 返回数据列表 |

**列表dfsFileNotifyList**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| fileName | String | 64 | M | 文件名称 |
| fileType | String | 1 | M | 文件类型 |
| fileId | String | 64 | M | 文件ID |
| dfsGroup | String | 32 | M | Dfs分组 |
| dfsFileName | String | 64 | O | Dfs文件名 |
| dfsPath | String | 100 | M | Dfs路径 |
| state | int | 1 | O | 文件上传状态 |

## 文件完成结果通知

> - 1.宝付返回格式为JSON
> - 2.商户接收到通知后务必在接收通知页面上返回大写OK
> - 3.宝付系统在未确认商户接收通知成功后将会通过重发机制通知商户（重发次数10次，请以第一次收到的付款成功的消息为准，避免进行多次确认）通知发给商户。
> - 4.该接口除了订单成功、失败结果通知外，退款的结果也一并通知。
> - 5.商户若需要该回调接口需联系技术支持人员配置回调地址等相关信息。
> - 6.通知接口Demo
> [http://URL](http://URL)
> ? member_id=1 & terminal_id=2 & data_type=JSON & data_content=密文

#### 报文参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| memberId | String | 32 | C | 宝付商户号 |
| terminalId | String | 32 | C | 宝付终端号 |
| businessParams | String | 100 | C | Ordertype 0 :自定义唯一标识，示例：登录号 |
| orderType | String | 200 | C | 订单类型 |
| transSerialNo | int | 1 | C | 请求流水号 |
| accType | int | 1 | C | 账户类型:1个人,2商户,3 个体工商户 |
| dfsFileNotifyList | List | | | 返回数据列表 |

**列表dfsFileNotifyList**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| fileName | String | 64 | M | 文件名称 |
| fileType | String | 1 | M | 文件类型 |
| fileId | String | 64 | M | 文件ID |
| dfsGroup | String | 32 | M | Dfs分组 |
| dfsFileName | String | 64 | O | Dfs文件名 |
| dfsPath | String | 100 | M | Dfs路径 |
| state | int | 1 | O | 文件上传/重传状态 |

#### 返回报文说明

> - 格式样例:
> {“accType”:”1”,”businessParams”:”CP660000002185089108”,”dfsFileNotifyList”:[{“dfsFileName”:”CertIdBack”,”dfsGroup”:”group1”,”dfsPath”:”M00/3C/BD/CgAVhF7WDICAbooPABlMAzW0aS0255.jpg”,”fileId”:12979198,”fileName”:”CertIdBack”,”fileType”:302},{“dfsFileName”:”CertIdFront”,”dfsGroup”:”group1”,”dfsPath”:”M00/40/C1/CgAVg17WDICAc_ysABlMAzW0aS0815.jpg”,”fileId”:12979199,”fileName”:”CertIdFront”,”fileType”:301},{“dfsFileName”:”PlatformCooperateAgreement”,”dfsGroup”:”group1”,”dfsPath”:”M00/3C/BD/CgAVhF7WDIGAVO2vABlMAzW0aS0426.jpg”,”fileId”:12979200,”fileName”:”PlatformCooperateAgreement”,”fileType”:311},{“dfsFileName”:”SpecialIndustryLicense”,”dfsGroup”:”group1”,”dfsPath”:”M00/40/C1/CgAVg17WDIGAPS1RABlMAzW0aS0072.jpg”,”fileId”:12979201,”fileName”:”SpecialIndustryLicense”,”fileType”:999},{“dfsFileName”:”BankCard”,”dfsGroup”:”group1”,”dfsPath”:”M00/3C/BD/CgAVhF7WDIGAMiIaABlMAzW0aS0679.jpg”,”fileId”:12979202,”fileName”:”BankCard”,”fileType”:401}],”memberId”:100000178,”orderType”:0,”state”:1,”terminalId”:100000859,”transSerialNo”:”UPFILE1591086203360”}

#### 响应报文说明

> - 注意：接受请求之后，请必须按照要求返回”OK”。

# 附录

## 开户文件类型表

| 二级商户类型 | 附件名 | 类型ID |
| --- | --- | --- |
| 公司 | 企业营业执照 | 101 |
| 公司 | 开户许可证(或开户凭证) | 102 |
| 公司 | 法人身份证正面 | 104 |
| 公司 | 法人身份证反面 | 111 |
| 公司 | 与平台合作协议 | 401 |
| 公司 | 特殊行业许可证(酒店/旅馆) | 999 |
| 个体工商户 | 个体工商户营业执照 | 101 |
| 个体工商户 | 开户许可证/经营者本人银行卡正面照 | 102 |
| 个体工商户 | 经营者身份证正面 | 104 |
| 个体工商户 | 经营者身份证反面 | 111 |
| 个体工商户 | 与平台合作协议 | 401 |
| 个体工商户 | 特殊行业许可证(酒店/旅馆) | 999 |
| 自然人 | 个人身份证正面 | 301 |
| 自然人 | 个人身份证反面 | 302 |
| 自然人 | 本人名下状态正常银行卡 | 311 |
| 自然人 | 与平台合作协议 | 401 |
| 自然人 | 特殊行业资格证书 | 999 |

## 文件参考命名格式(zip结尾): 压缩包文件名不强制规定

| 二级商户类型 | 格式 | 示例 |
| --- | --- | --- |
| 公司 | 平台商户号_二级商户户名 | [120838_zhangsan@163.com](mailto:120838_zhangsan@163.com) |
| 个体工商户 | 平台商户号_二级商户户名 | [120838_zhangsan@163.com](mailto:120838_zhangsan@163.com) |
| 自然人 | 平台商户号_手机号 | 120838_13999990001 |

文档更新时间: 2026-04-25 08:30 作者：超级管理员
