# 宝付宝财通 2.0 字段级契约矩阵

Updated: 2026-05-05
Risk class: G3 - funds/payment/refund/share/withdraw/callback contract boundary.

This matrix is the field-level build checklist for the LocalLife Baofoo BaoCaiTong contract layer. It is intentionally verbose: every production-used field must be traceable to an official page row, a local DTO/parser, a validator, and a test. Official Baofoo documentation remains the only source of truth; this file is a project-side extraction and implementation checklist.

## Reading Rules

- `M/是` = mandatory; `C` = conditional mandatory; `O/否` = optional; `R` = returned/original value.
- `Path` is the project contract path to implement or parse. For public-envelope rows it is the wrapper path; for business rows it is inside `bizContent` or `dataContent`.
- `契约实现要求` is derived from official requiredness/type/description and must become constants, validators, parsers, or table-driven tests before a field can be considered C3.
- Sandbox/demo behavior may add parser tolerance only. It cannot change a field type, requiredness, enum, unit, or ID ownership unless Baofoo confirms it and the source matrix is updated.
- First-version production path is BaoCaiTong account + WECHAT merchant report + APPLET binding + WECHAT_JSAPI aggregate payment + share-after-pay + pre-share refund + withdrawal. Deferred tables are captured so they are not accidentally implemented by analogy.

## 1. Account Union-GW Common Envelope

### 账户请求入口 (`unionGw`)

Source: `https://doc.mandao.com/docs/bct/unionGw`; updated: `2025-05-14 06:48`.


#### 1.4.1符号含义

| 序号 | 符号缩写 | 符号性质 | 符号说明 |
| --- | --- | --- | --- |
| 1 | M | 强制域(Mandatory) | 必须填写的属性，否则会被认为格式错误 |
| 2 | C | 条件域(Conditional) | 某条件成立时必须填写的属性 |
| 3 | O | 选用域(Optional) | 选填属性 |
| 4 | R | 原样返回域(Returned) | 必须与先前报文中对应域的值相同的域 |

#### 请求参数

| Path | 字段名/域名 | 变量名 | 类型 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `unionGw.urlQuery.memberId` | 商户号 | `memberId` |  | M |  | 宝付提供给商户的唯一编号 | Validate required; add missing-field test |
| `unionGw.urlQuery.terminalId` | 终端号 | `terminalId` |  | M |  | 终端号 | Validate required; add missing-field test |
| `unionGw.urlQuery.verifyType` | 签名类型 | `verifyType` |  | M |  | 类型值：1、2；参考verifyType传值说明 | Validate required; add missing-field test |
| `unionGw.urlQuery.content` | 加密密文 | `content` |  | M |  | 字段按content说明组装后按verifyType类型加密，组装示例：{“body”: {“amount”: 1.00,”cardName”: “zhangsan”,”cardNo”: “123”},<br>“header”: {“memberId”: “001”,”serviceTp”: “T-1001-001-01”,”terminalId”: “002”,”verifyType”: “1”}} | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |
| `unionGw.urlQuery.veryfyString` | 验签密文 | `veryfyString` |  | C |  | 按verifyType类型加密 | Encode condition from notes; add positive/negative conditional tests; Security boundary: sign/decrypt/verify before business parsing |

#### 请求参数

| verifyType值 | content加密方式 | verifyString加密方式 |
| --- | --- | --- |
| 1 | 1.先明文base64编码<br>2.然后RSA私钥加密生成16进制字符串 | 空 |
| 2 | 1.先明文base64编码<br>2.然后3des加密(key联系支持)生成16进制字符串 | 1.content基础上先sha-1签名，生成16进制小写字符串<br>2.然后RSA私钥签名生成16进制字符串 |

#### header部分

| Path | 字段名/域名 | 变量名 | 类型 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `unionGw.request.header.memberId` | 商户号 | `memberId` |  | M |  | 宝付提供给商户的唯一编号 | Validate required; add missing-field test |
| `unionGw.request.header.terminalId` | 终端号 | `terminalId` |  | M |  | 终端号 | Validate required; add missing-field test |
| `unionGw.request.header.serviceTp` | 报文编号 | `serviceTp` |  | M |  | 见附录；报文编号应与请求地址中一致 | Validate required; add missing-field test |
| `unionGw.request.header.verifyType` | 加密方式 | `verifyType` |  | M |  | 参考2.1.1 | Validate required; add missing-field test |

#### 2.3 同步响应报文

| verifyType值 | 返回内容 |
| --- | --- |
| 1 | base64和RSA私钥加密之后的内容 |
| 2 | content=数据内容&verifyString=签名内容 |

#### header部分

| Path | 字段名/域名 | 变量名 | 类型 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `unionGw.response.header.memberId` | 商户号 | `memberId` |  | R |  | 宝付提供给商户的唯一编号 | Returned/original value: compare with request context |
| `unionGw.response.header.terminalId` | 终端号 | `terminalId` |  | R |  | 终端号 | Returned/original value: compare with request context |
| `unionGw.response.header.serviceTp` | 报文编号 | `serviceTp` |  | R |  | 报文编号 | Returned/original value: compare with request context |
| `unionGw.response.header.sysRespCode` | 系统返回码 | `sysRespCode` |  | M |  | 见附录 | Validate required; add missing-field test |
| `unionGw.response.header.sysRespDesc` | 系统返回信息 | `sysRespDesc` |  | M |  |  | Validate required; add missing-field test |

#### 1.系统响应吗

| Code | 含义/描述 | 解决/请求状态 | 契约实现要求 |
| --- | --- | --- | --- |
| `S_0000` | 请求正常 | 正常(请查看body中的相关信息) | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_0001` | 明文参数格式不正确,%s | 失败 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_0002` | 明文参数解析失败,%s | 失败 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_0003` | 商户信息不存在或状态不正常 | 失败 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_0004` | 商户与终端号不匹配 | 失败 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_0005` | ip未绑定，请联系宝付 | 失败 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_0006` | 密文解密失败 | 失败 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_0007` | 密文参数解析失败 | 失败 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_0008` | 头参数格式不正确,%s | 失败 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_0009` | 头参数和文明参数不一致,%s | 失败 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_0010` | 接口服务报文不支持 | 失败 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_9001` | 请求受理失败 | 失败 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `S_E_9002` | 请求受理结果未知 | 未知 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |

#### 2.报文编号

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `T-1001-013-01` | 宝财通开户 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `T-1001-013-02` | 修改绑定卡信息 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `T-1001-013-03` | 开户信息查询 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `T-1001-013-08` | 绑卡查询 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `T-1001-013-06` | 余额查询 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `T-1001-013-11` | 收支明细查询 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `T-1001-013-14` | 账户提现 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `T-1001-013-15` | 账户提现查询 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `T-1001-013-13` | 账户间转账 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `T-1001-013-10` | 账户间转账查询 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

## 2. Account API Field Matrix - First-Version Production Scope

### 个人/机构开户接口 (`openAcc`)

Source: `https://doc.mandao.com/docs/bct/openAcc`; updated: `2026-01-12 07:58`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `openAcc.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.1.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.body.accType` | `accType` | int | 1 | M | length=1 | 账户类型:1-个人,2-企业/个体 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.body.accInfo` | `accInfo` | Object |  | M |  | 开户具体信息根据类型不同,信息不同,现只支持单笔开户 | Validate required; add missing-field test |
| `openAcc.request.body.noticeUrl` | `noticeUrl` | String | 256 | M | length=256 | 开户结果通知地址，通知参数详见开户结果通知 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.body.businessType` | `businessType` | String | 32 | M | length=32 | 宝财通2.0: BCT2.0 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `openAcc.request.accInfo.personalFourFactor.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.personalFourFactor.loginNo` | `loginNo` | String | 32 | M | length=32 | 登录号,商户自定义要求全局唯一，长度11位以上 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.personalFourFactor.customerName` | `customerName` | String | 32 | M | length=32 | 客户名称与持卡人姓名一致 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.personalFourFactor.certificateType` | `certificateType` | String | 16 | M | length=16 | 证件类型,身份证: ID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.personalFourFactor.certificateNo` | `certificateNo` | String | 64 | M | length=64 | 身份证号码 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.personalFourFactor.cardNo` | `cardNo` | String | 128 | M | length=128 | 卡号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.personalFourFactor.mobileNo` | `mobileNo` | String | 64 | M | length=64 | 银行预留手机号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.personalFourFactor.cardUserName` | `cardUserName` | String | 20 | M | length=20 | 持卡人姓名 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.personalFourFactor.needUploadFile` | `needUploadFile` | boolean |  | M |  | 是否需要上传附件,true/false | Validate required; add missing-field test |
| `openAcc.request.accInfo.personalFourFactor.platformNo` | `platformNo` | String | 32 | C | length=32 | 平台号 (代理模式下此处为业务方商户号非代理商商户号) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAcc.request.accInfo.personalFourFactor.platformTerminalId` | `platformTerminalId` | String | 32 | C | conditional rule in description; length=32 | 终端号(代理模式必传) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAcc.request.accInfo.personalFourFactor.qualificationTransSerialNo` | `qualificationTransSerialNo` | String | 128 | C | length=128 | 资质文件流水,是否上传请咨询业务对接人 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |

#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `openAcc.request.accInfo.business.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.loginNo` | `loginNo` | String | 32 | M | length=32 | 登录号,商户自定义要求全局唯一，长度11位以上 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.email` | `email` | String | 32 | M | length=32 | 邮箱 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.selfEmployed` | `selfEmployed` | boolean | 1 | C | conditional rule in description; length=1 | 是否个体户 企业为false，不传默认为false | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.customerName` | `customerName` | String | 64 | M | length=64 | 商户名称（营业执照上的名称） | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.aliasName` | `aliasName` | String | 64 | O | length=64 | 商户名称别名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.certificateNo` | `certificateNo` | String | 64 | M | length=64 | 证件号码 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.certificateType` | `certificateType` | String | 16 | M | length=16 | 证件类型 营业执照:LICENSE | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.corporateName` | `corporateName` | String | 20 | M | length=20 | 法人姓名 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.corporateCertType` | `corporateCertType` | String | 10 | M | length=10 | 法人证件类型：身份证-ID，<br>港澳通行证-HONG_KONG_AND_MACAO_PASS<br>台湾同胞来往内地通行证-TAIWAN_TRAVEL_PERMIT<br>护照-PASSPORT | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.corporateCertId` | `corporateCertId` | String | 64 | M | length=64 | 法人身份证号码/港澳通行证/台湾同胞来往内地通行证 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.corporateMobile` | `corporateMobile` | String | 64 | C | conditional rule in description; length=64 | 法人手机号<br>当开个体户(selfEmployed=true)且绑定对私卡时必传 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.industryId` | `industryId` | String | 11 | M | length=11 | 公司所属行业 见附录 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.contactName` | `contactName` | String | 20 | O | length=20 | 联系人姓名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.contactMobile` | `contactMobile` | String | 64 | O | length=64 | 联系人手机号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.cardNo` | `cardNo` | String | 128 | M | length=128 | 卡号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.bankName` | `bankName` | String | 20 | M | length=20 | 银行名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.depositBankProvince` | `depositBankProvince` | String | 20 | M | length=20 | 开户行省份 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.depositBankCity` | `depositBankCity` | String | 20 | M | length=20 | 开户行城市 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.depositBankName` | `depositBankName` | String | 64 | M | length=64 | 开户支行名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.registerCapital` | `registerCapital` | String | 64 | C | length=64 | 注册资本 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.cardUserName` | `cardUserName` | String | 64 | O | conditional rule in description; length=64 | 持卡人姓名<br>当开个体户且绑定对私卡时需传此字段，长度为20；若不是对私卡传输需要与企业绑定对公卡名称保持一致，不传则默认绑定对公卡 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.platformNo` | `platformNo` | String | 32 | C | conditional rule in description; length=32 | 平台号(主商户号) (代理模式必传) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.platformTerminalId` | `platformTerminalId` | String | 32 | C | conditional rule in description; length=32 | 终端号(代理模式必传) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAcc.request.accInfo.business.qualificationTransSerialNo` | `qualificationTransSerialNo` | String | 128 | C | conditional rule in description; length=128 | 资质文件流水,businessType为宝财通2.0非必填 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `openAcc.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAcc.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAcc.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `openAcc.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `openAcc.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `openAcc.response.body.result` | `result` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `openAcc.response.result[].state` | `state` | String | 4 | M | length=4 | 状态 1 成功 0 失败 -1 异常 2开户处理中 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.response.result[].errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAcc.response.result[].errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAcc.response.result[].transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.response.result[].loginNo` | `loginNo` | String | 128 | M | length=128 | 登录号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.response.result[].customerName` | `customerName` | String | 64 | M | length=64 | 商户名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAcc.response.result[].contractNo` | `contractNo` | String | 64 | C | length=64 | 商户客户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |

### 个人开户接口(二要素) (`bct-1gj4ccsdha6d8`)

Source: `https://doc.mandao.com/docs/bct/bct-1gj4ccsdha6d8`; updated: `2025-09-16 09:09`.

Note: Runtime first version rejects personal two-factor; fields remain here for drift detection.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `openAccTwoFactor.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.1.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.request.body.accType` | `accType` | int | 1 | M | length=1 | 账户类型:1-个人,2-企业/个体 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.request.body.accInfo` | `accInfo` | Object |  | M |  | 开户具体信息根据类型不同,信息不同,现只支持单笔开户 | Validate required; add missing-field test |
| `openAccTwoFactor.request.body.noticeUrl` | `noticeUrl` | String | 256 | M | length=256 | 开户结果通知地址，通知参数详见开户结果通知 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.request.body.businessType` | `businessType` | String | 32 | M | length=32 | 宝财通2.0: BCT2.0 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `openAccTwoFactor.request.accInfo.personalTwoFactor.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.request.accInfo.personalTwoFactor.loginNo` | `loginNo` | String | 32 | M | length=32 | 登录号,商户自定义要求全局唯一，长度11位以上 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.request.accInfo.personalTwoFactor.customerName` | `customerName` | String | 32 | M | length=32 | 客户名称与持卡人姓名一致 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.request.accInfo.personalTwoFactor.certificateType` | `certificateType` | String | 16 | M | length=16 | 证件类型,身份证: ID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.request.accInfo.personalTwoFactor.certificateNo` | `certificateNo` | String | 64 | M | length=64 | 身份证号码 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.request.accInfo.personalTwoFactor.cardUserName` | `cardUserName` | String | 20 | M | length=20 | 持卡人姓名 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.request.accInfo.personalTwoFactor.needUploadFile` | `needUploadFile` | boolean |  | M |  | 是否需要上传附件,true/false | Validate required; add missing-field test |
| `openAccTwoFactor.request.accInfo.personalTwoFactor.platformNo` | `platformNo` | String | 32 | C | length=32 | 平台号 (代理模式下此处为业务方商户号非代理商商户号) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAccTwoFactor.request.accInfo.personalTwoFactor.platformTerminalId` | `platformTerminalId` | String | 32 | C | conditional rule in description; length=32 | 终端号(代理模式必传) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAccTwoFactor.request.accInfo.personalTwoFactor.qualificationTransSerialNo` | `qualificationTransSerialNo` | String | 128 | C | length=128 | 资质文件流水,是否上传请咨询业务对接人 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `openAccTwoFactor.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAccTwoFactor.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAccTwoFactor.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `openAccTwoFactor.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `openAccTwoFactor.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `openAccTwoFactor.response.body.result` | `result` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `openAccTwoFactor.response.result[].state` | `state` | String | 4 | M | length=4 | 状态 1 成功 0 失败 -1 异常 2开户处理中 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.response.result[].errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAccTwoFactor.response.result[].errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAccTwoFactor.response.result[].transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.response.result[].loginNo` | `loginNo` | String | 128 | M | length=128 | 登录号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.response.result[].customerName` | `customerName` | String | 64 | M | length=64 | 商户名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccTwoFactor.response.result[].contractNo` | `contractNo` | String | 64 | C | length=64 | 商户客户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |

### 开户查询接口 (`queryAcc`)

Source: `https://doc.mandao.com/docs/bct/queryAcc`; updated: `2026-01-13 02:01`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `queryAcc.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.0.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryAcc.request.body.certificateNo` | `certificateNo` | String | 64 | C | length=64 | 证件号码（社会信用代码） | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryAcc.request.body.certificateType` | `certificateType` | String | 16 | C | enum/allowlist; length=16 | 证件类型 只能取”ID”或”LICENSE” | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `queryAcc.request.body.platformNo` | `platformNo` | String | 32 | C | length=32 | 平台号(主商户号) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryAcc.request.body.loginNo` | `loginNo` | String | 128 | C | conditional rule in description; length=128 | 登录号(传此参数以上三个参数必填) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryAcc.request.body.contractNo` | `contractNo` | String | 32 | C | length=32 | 客户账户号(传此参数以上四个参数可以不填) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `queryAcc.request.body.accType` | `accType` | int | 1 | M | length=1 | 账户类型:1个人,2商户 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `queryAcc.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryAcc.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryAcc.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryAcc.response.body.result` | `result` | Object |  | C |  | 返回数据列表，当retCode=1时有值 | Encode condition from notes; add positive/negative conditional tests |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `queryAcc.response.result.contractNo` | `contractNo` | String | 32 | M | length=32 | 客户账户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `queryAcc.response.result.contractName` | `contractName` | String | 20 | O | length=20 | 客户账户名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryAcc.response.result.customerName` | `customerName` | String | 20 | O | length=20 | 客户名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryAcc.response.result.customerNo` | `customerNo` | String | 20 | O | length=20 | 此字段暂用不到 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryAcc.response.result.customerType` | `customerType` | String | 16 | O | length=16 | 客户类型 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryAcc.response.result.certificateType` | `certificateType` | String | 16 | O | enum/allowlist; length=16 | 证件类型 只能取”ID”或”LICENSE” | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `queryAcc.response.result.certificateNo` | `certificateNo` | String | 64 | O | length=64 | 证件号码（社会信用代码） | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryAcc.response.result.platformNo` | `platformNo` | String | 32 | O | length=32 | 平台号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryAcc.response.result.bindMobile` | `bindMobile` | String | 64 | O | length=64 | 绑定手机号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryAcc.response.result.email` | `email` | String | 128 | O | length=128 | 邮箱 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 账户余额查询接口 (`queryBalace`)

Source: `https://doc.mandao.com/docs/bct/queryBalace`; updated: `2025-09-16 09:09`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `queryBalance.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.0.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryBalance.request.body.contractNo` | `contractNo` | String | 32 | M | length=32 | 客户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `queryBalance.request.body.accType` | `accType` | int | 4 | M | length=4 | 账户类型:1个人,2商户 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `queryBalance.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryBalance.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryBalance.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryBalance.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryBalance.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryBalance.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryBalance.response.body.availableBal` | `availableBal` | BigDecimal | 10,2 | O | unit=yuan; length=10,2 | 账簿可用余额,单位：元;可用于提现 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `queryBalance.response.body.pendingBal` | `pendingBal` | BigDecimal | 10,2 | O | unit=yuan; length=10,2 | 在途资金余额,单位：元 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `queryBalance.response.body.currBal` | `currBal` | BigDecimal | 10,2 | O | unit=yuan; length=10,2 | 账簿余额,单位：元;<br>账簿余额=可用余额(availableBal)+在途余额(pendingBal)+冻结金额 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |

### 账户提现接口 (`accWithdrawal`)

Source: `https://doc.mandao.com/docs/bct/accWithdrawal`; updated: `2025-09-16 09:09`.


#### 接口版本：

| 版本号 | 修订日期 | 修订内容 |
| --- | --- | --- |
| 4.0.0 | 2024-03-01 | 初始版本 |
| 4.2.0 | 2024-12-27 | 此版本提现结果通知接口新增返回订单状态3:提现退回 |

#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `withdraw.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.2.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdraw.request.body.contractNo` | `contractNo` | String | 32 | M | length=32 | 客户账户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `withdraw.request.body.directPlatformNo` | `directPlatformNo` | String | 32 | C | length=32 | 上级客户账户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `withdraw.request.body.transSerialNo` | `transSerialNo` | String | 50 | M | length=50 | 商户订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdraw.request.body.dealAmount` | `dealAmount` | BigDecimal | 10,2 | M | unit=yuan; length=10,2 | 提现金额,单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `withdraw.request.body.returnUrl` | `returnUrl` | String | 256 | M | length=256 | 提现结果异步通知地址，通知参数详见提现结果通知 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdraw.request.body.feeMemberId` | `feeMemberId` | String | 32 | C | conditional rule in description; length=32 | 用户自己承担手续费必传，与客户号contractNo一致需用户承担手续费时要提前和商务申请配置 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `withdraw.request.body.reqReserved` | `reqReserved` | String | 512 | O | length=512 | 原样返回保留字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `withdraw.request.body.transAbstract` | `transAbstract` | String | 255 | C | length=255 | 摘要 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `withdraw.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdraw.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `withdraw.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `withdraw.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `withdraw.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `withdraw.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `withdraw.response.body.transSerialNo` | `transSerialNo` | String | 50 | M | length=50 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdraw.response.body.contractNo` | `contractNo` | String | 32 | M | length=32 | 客户账户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `withdraw.response.body.state` | `state` | int | 4 | M | length=4 | 订单状态 1受理成功 2受理失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdraw.response.body.transRemark` | `transRemark` | String | 512 | C | length=512 | 订单备注（受理失败的具体原因） | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |

### 提现查询接口 (`queryWithdrawal`)

Source: `https://doc.mandao.com/docs/bct/queryWithdrawal`; updated: `2025-09-16 09:09`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `withdrawQuery.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.2.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdrawQuery.request.body.transSerialNo` | `transSerialNo` | String | 50 | M | length=50 | 商户订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdrawQuery.request.body.tradeTime` | `tradeTime` | String | 10 | M | date/time format; length=10 | 交易时间 yyyy-MM-dd | Validate required; add missing-field test; Preserve official length/type at boundary; Validate official time/date format |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `withdrawQuery.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdrawQuery.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `withdrawQuery.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `withdrawQuery.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `withdrawQuery.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `withdrawQuery.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `withdrawQuery.response.body.memberId` | `memberId` | String | 32 | M | length=32 | 商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdrawQuery.response.body.transSerialNo` | `transSerialNo` | String | 50 | M | length=50 | 商户订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdrawQuery.response.body.state` | `state` | int | 4 | M | length=4 | 订单状态 1成功 0失败 2处理中 3提现退回 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdrawQuery.response.body.orderId` | `orderId` | long | 20 | O | length=20 | 订单号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `withdrawQuery.response.body.successTime` | `successTime` | String | 19 | O | date/time format; length=19 | 成功时间，格式：yyyy-MM-dd HH:mm:ss | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Validate official time/date format |
| `withdrawQuery.response.body.contractNo` | `contractNo` | String | 32 | M | length=32 | 商户客户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `withdrawQuery.response.body.transMoney` | `transMoney` | BigDecimal | 10,2 | M | unit=yuan; length=10,2 | 转账金额,单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `withdrawQuery.response.body.transFee` | `transFee` | BigDecimal | 10,2 | M | unit=yuan; length=10,2 | 转账手续费,单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `withdrawQuery.response.body.transferTotalAmount` | `transferTotalAmount` | BigDecimal | 10,2 | M | unit=yuan; length=10,2 | 转账交易时金额,单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `withdrawQuery.response.body.transRemark` | `transRemark` | String | 200 | O | length=200 | 失败原因 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 开户结果通知 (`openAccNotify`)

Source: `https://doc.mandao.com/docs/bct/openAccNotify`; updated: `2026-04-22 02:12`.


#### 通知参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `openAccNotify.transport.member_id` | `member_id` | String | 10 | M | length=10 | 商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccNotify.transport.terminal_id` | `terminal_id` | String | 10 | M | length=10 | 终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccNotify.transport.data_type` | `data_type` | String | 5 | M | length=5 | JSON,data_content的明文组装格式 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccNotify.transport.data_content` | `data_content` | String | 128 | M | length=128 | 加密数据，需使用宝付公钥解密后再做base64解码 | Validate required; add missing-field test; Preserve official length/type at boundary; Security boundary: sign/decrypt/verify before business parsing |

#### 开户通知参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `openAccNotify.data_content.member_id` | `member_id` | String | 10 | M | length=10 | 商户号 | Validate required; accept official-example `memberId` alias inbound; require plaintext identity to match transport identity; Preserve official length/type at boundary |
| `openAccNotify.data_content.terminal_id` | `terminal_id` | String | 10 | M | length=10 | 终端号 | Validate required; accept official-example `terminalId` alias inbound; require plaintext identity to match transport identity; Preserve official length/type at boundary |
| `openAccNotify.data_content.memberType` | `memberType` | int | 1 | M | length=1 | 类型:类型:1-个人,2-企业,3-个体工商户 | Validate required; accept official-example quoted numeric strings and table-declared JSON numbers inbound; add missing-field/type test; Preserve official length/type at boundary |
| `openAccNotify.data_content.state` | `state` | String | 4 | M | length=4 | 状态 1 成功 0 失败 -1 异常 2开户处理中 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccNotify.data_content.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAccNotify.data_content.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `openAccNotify.data_content.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccNotify.data_content.loginNo` | `loginNo` | String | 128 | M | length=128 | 登录号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccNotify.data_content.customerName` | `customerName` | String | 64 | M | length=64 | 商户名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `openAccNotify.data_content.contractNo` | `contractNo` | String | 64 | C | required when `state=1`; official failure example sends empty string | 商户客户号 | Validate required only for successful/active state; tolerate empty for failed/abnormal/processing states so callback can be ACKed and persisted; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `openAccNotify.data_content.noticeType` | `noticeType` | String | 32 | M | length=32 |  | Official table marks required, but official success/failure examples and production callback evidence can omit it; tolerate absent inbound; Preserve official length/type at boundary |

### 提现结果通知 (`withdrawNotify`)

Source: `https://doc.mandao.com/docs/bct/withdrawNotify`; updated: `2025-09-16 09:09`.


#### 通知参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `withdrawNotify.transport.member_id` | `member_id` | String | 10 | M | length=10 | 商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdrawNotify.transport.terminal_id` | `terminal_id` | String | 10 | M | length=10 | 终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdrawNotify.transport.data_type` | `data_type` | String | 5 | M | length=5 | JSON,data_content的明文组装格式 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdrawNotify.transport.data_content` | `data_content` | String | 128 | M | length=128 | 加密数据，需使用宝付公钥解密后再做base64解码 | Validate required; add missing-field test; Preserve official length/type at boundary; Security boundary: sign/decrypt/verify before business parsing |

#### data_content解密参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `withdrawNotify.data_content.contractNo` | `contractNo` | String | 32 | M | length=32 | 提现子商户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `withdrawNotify.data_content.orderId` | `orderId` | String | 20 | M | length=20 | 提现订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdrawNotify.data_content.transSerialNo` | `transSerialNo` | String | 50 | M | length=50 | 商户订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `withdrawNotify.data_content.transMoney` | `transMoney` | BigDecimal | 10,2 | M | unit=yuan; length=10,2 | 转账金额,单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `withdrawNotify.data_content.transFee` | `transFee` | BigDecimal | 10,2 | M | unit=yuan; length=10,2 | 费用,单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `withdrawNotify.data_content.transferTotalAmount` | `transferTotalAmount` | BigDecimal | 10,2 | M | unit=yuan; length=10,2 | 转账交易时金额,单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `withdrawNotify.data_content.state` | `state` | String | 4 | M | enum/allowlist; conditional rule in description; length=4 | 状态枚举<br>0:失败<br>1:成功<br>2:处理中<br>3:提现退回 注：此状态只在提现时上送版本号4.2.0返回 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `withdrawNotify.data_content.transRemark` | `transRemark` | String | 128 | C | official success example sends empty string; meaningful mainly for failure/return reason | 失败原因 | Tolerate absent or empty inbound for success/processing callbacks; preserve when supplied for failure/returned callbacks; Preserve official length/type at boundary |
| `withdrawNotify.data_content.reqReserved` | `reqReserved` | String | 512 | C | official success example sends empty string | 保留域 | Tolerate absent or empty inbound; preserve when supplied; Preserve official length/type at boundary |

## 3. Merchant Report Field Matrix

### 请求入口 (`bct-1f9o5s1lqlean`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9o5s1lqlean`; updated: `2024-04-24 07:16`.


#### 请求参数格式约定如下：

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchantReport.publicRequest.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchantReport.publicRequest.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchantReport.publicRequest.method` | 接口名称 | `method` | 是 | S(64) | merchant_report | S(64) | 对应接口的名称，参考各接口说明 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchantReport.publicRequest.charset` | 字符集 | `charset` | 是 | S(16) | UTF-8 | fixed value; S(16) | 字符集编码，固定值UTF-8 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchantReport.publicRequest.version` | 接口版本号 | `version` | 是 | S(8) | 1.0 | fixed value; S(8) | 对应接口的版本号，固定值1.0 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchantReport.publicRequest.format` | 格式化类型 | `format` | 是 | S(8) | json | fixed value; S(8) | 各个接口业务参数的格式化类型，固定值json | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchantReport.publicRequest.timestamp` | 时间戳 | `timestamp` | 是 | T | 20210315155012 | date/time format | 时间戳与宝付支付系统时间误差不超过10分钟，格式为yyyyMMddHHmmss，如：2021年3月15日15点50分12秒表示为：20210315155012 | Validate required; add missing-field test; Validate official time/date format |
| `merchantReport.publicRequest.signType` | 加密签名类型 | `signType` | 是 | E | SM2 | enum/allowlist | 商户生成签名和加密字符串使用的算法类型，见附录【签名类型】 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `merchantReport.publicRequest.signSn` | 签名证书序列号 | `signSn` | 是 | S(10) | 1 | S(10) | 发送方公钥证书序列号，用于接收方验签证书选择 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchantReport.publicRequest.ncrptnSn` | 加密证书序列号 | `ncrptnSn` | 是 | S(10) | 1 | S(10) | 接收方公钥证书序列号，用于接收方解密证书选择 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchantReport.publicRequest.dgtlEnvlp` | 数字信封 | `dgtlEnvlp` | 否 | S |  |  | 使用接收方公钥加密的对称密钥，并做16进制转码，是否需要传值，详见各个业务接口说明 | Optional: omit empty outbound; tolerate absent inbound; Security boundary: sign/decrypt/verify before business parsing |
| `merchantReport.publicRequest.signStr` | 签名串 | `signStr` | 是 | S |  |  | 使用发送方私钥签名的非对称密钥，并做16进制转码 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |
| `merchantReport.publicRequest.bizContent` | 业务参数 | `bizContent` | 是 | S |  |  | 业务数据报文，JSON格式，具体见各业务接口定义 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |

#### 返回参数格式约定如下:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchantReport.publicResponse.transport.returnCode` | 返回状态码 | `returnCode` | 是 | S(16) | SUCCESS | S(16) | 成功：SUCCESS 失败：FAIL 通信标识，非交易标识 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchantReport.publicResponse.transport.returnMsg` | 返回信息 | `returnMsg` | 是 | S(128) | OK | S(128) | 当returnCode返回FAIL时返回错误信息，如：验签失败 解密失败 商户号不存在 参数格式验证错误等 检查报文重新发起请求 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数格式约定如下:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchantReport.publicResponse.success.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchantReport.publicResponse.success.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchantReport.publicResponse.success.charset` | 字符集 | `charset` | 是 | S(16) | UTF-8 | fixed value; S(16) | 字符集编码，固定值UTF-8 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchantReport.publicResponse.success.version` | 接口版本号 | `version` | 是 | S(8) | 1.0 | fixed value; S(8) | 对应接口的版本号，固定值1.0 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchantReport.publicResponse.success.format` | 格式化类型 | `format` | 是 | S(8) | json | fixed value; S(8) | 各个接口业务参数的格式化类型，固定值json | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchantReport.publicResponse.success.signType` | 加密签名类型 | `signType` | 是 | E | SM2 | enum/allowlist | 与商户原请求签名加密类型一致，如：商户请求采用国密类型，则对应返回也采用国密类型 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `merchantReport.publicResponse.success.signSn` | 签名证书序列号 | `signSn` | 是 | S(10) | 1 | fixed value; S(10) | 发送方公钥证书序列号，用于接收方验签证书选择，固定值1 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchantReport.publicResponse.success.ncrptnSn` | 加密证书序列号 | `ncrptnSn` | 是 | S(10) | 1 | fixed value; S(10) | 接收方公钥证书序列号，用于接收方解密证书选择，固定值1 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchantReport.publicResponse.success.dgtlEnvlp` | 数字信封 | `dgtlEnvlp` | 否 | S |  |  | 使用接收方公钥加密的对称密钥，并做16进制转码，返回是否有值，详见各个业务接口返回参数说明 | Optional: omit empty outbound; tolerate absent inbound; Security boundary: sign/decrypt/verify before business parsing |
| `merchantReport.publicResponse.success.signStr` | 签名串 | `signStr` | 是 | S |  |  | 使用发送方私钥签名的非对称密钥，并做16进制转码 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |
| `merchantReport.publicResponse.success.dataContent` | 返回参数 | `dataContent` | 是 | S |  |  | 业务数据报文，JSON格式，具体见各业务接口返回参数说明 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |

### 报备认证 (`bct-1f9o62bulbiqd`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9o62bulbiqd`; updated: `2026-04-27 06:20`.

Note: First-version posts WECHAT only. Alipay rows are captured as deferred fields and must not leak into WECHAT requests.


#### 接口说明

| 接口名称 | merchant_report |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

#### 请求参数：

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report.request.bizContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.bizContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.bizContent.merId` | 交易商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.bizContent.terId` | 交易终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.bizContent.reportType` | 报备类型 | `reportType` | 是 | E | WECHAT | enum/allowlist | 详见附录：《报备类型》 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `merchant_report.request.bizContent.reportNo` | 报备编号 | `reportNo` | 是 | S(64) | 20211220120030798 | S(64) | 每次请求报备接口唯一编号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.bizContent.reportInfo` | 报备信息 | `reportInfo` | 是 | C |  |  | JSON格式，根据传入报备类型，查看相应的报备信息说明<br>详见：报备信息参数：reportInfo | Validate required; add missing-field test |
| `merchant_report.request.bizContent.bctMerId` | 宝财通二级商户号 | `bctMerId` | 是 | S(64) | CM1234567890 | S(64) | 宝财通二级用户客户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |

#### 微信参数:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report.request.reportInfo.wechat.merchant_name` | 商户名称 | `merchant_name` | 是 | S(50) | 商户名称 | S(50) | 该名称是公司主体全称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.merchant_shortname` | 商户简称 | `merchant_shortname` | 是 | S(20) | 商户简称 | S(20) | 该名称是显示给消费者看的商户名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.service_phone` | 客服电话 | `service_phone` | 是 | S(20) | 075586010000 | S(20) | 方便银联在必要时能联系上商家 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.contact` | 联系人 | `contact` | 否 | S(10) | 联系人 | S(10) | 同上 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.contact_phone` | 联系电话 | `contact_phone` | 否 | S(11) | 13000000000 | S(11) | 同上 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.contact_email` | 联系邮箱 | `contact_email` | 否 | S(30) | test@test.com | S(30) | 同上 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.channel_id` | 渠道商商户号 | `channel_id` | 是 | S(32) | 10100000 | S(32) | 收单机构为其渠道商申请,测试环境传：148717784 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.channel_name` | 渠道商商户名称 | `channel_name` | 是 | S(50) | 渠道商商户名称 | S(50) | 渠道商商户名称，测试环境传：宝财通收单商户有限公司 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.business` | 经营类目 | `business` | 是 | S(10) | 101 | S(10) | 行业类目，请填写对应的ID,测试环境传:758-2 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.contact_wechatid_type` | 联系人微信账号类型 | `contact_wechatid_type` | 否 | S(32) | type_wechatid | S(32) | 如传微信号，值为 type_wechatid | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.contact_wechatid` | 联系人微信帐号 | `contact_wechatid` | 否 | S(32) | OPENID_01231232 | S(32) | 微信号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.service_codes` | 申请服务 | `service_codes` | 是 | C | [“JSAPI”,”APPLET”] |  | 申请服务，可传送所有需要开通的服务，详见《微信服务类型》，JSON数组格式 | Validate required; add missing-field test |
| `merchant_report.request.reportInfo.wechat.address_info` | 地址信息 | `address_info` | 是 | C | - |  | 地址信息，JSON格式<br>详见:地址信息：address_info | Validate required; add missing-field test |
| `merchant_report.request.reportInfo.wechat.business_license` | 商户证件编号 | `business_license` | 是 | S(20) | 100000011234561 | S(20) | 商户证件编号（企业或者个体工商户提供营业执照，事业单位提供事证号，小微商户提供身份证号） | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.business_license_type` | 商户证件类型 | `business_license_type` | 是 | S(32) | NATIONAL_LEGAL | enum/allowlist; S(32) | 商户证件类型，取值范围详见《微信证件类型》枚举 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchant_report.request.reportInfo.wechat.bankcard_info` | 银行结算卡信息 | `bankcard_info` | 是 | C |  |  | 商户对应银行所开立的结算卡信息，JSON格式<br>详见：银行卡信息：bankcard_info | Validate required; add missing-field test |

#### 微信参数:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report.request.reportInfo.wechat.address_info.city_code` | 城市编码 | `city_code` | 是 | S(6) | 510800 | S(6) | 城市编码是与国家统计局一致，请查询 :https://dmfw.mca.gov.cn/XzqhVersionPublish.html | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.address_info.district_code` | 区县编码 | `district_code` | 是 | S(1) | 510812 | S(1) | 区县编码是与国家统计局一致，请查询 : https://dmfw.mca.gov.cn/XzqhVersionPublish.html | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.address_info.province_code` | 省份编码 | `province_code` | 是 | S(10) | 510000 | S(10) | 省份编码是与国家统计局一致，请查询 : https://dmfw.mca.gov.cn/XzqhVersionPublish.html | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.address_info.address` | 详细地址 | `address` | 是 | S(64) |  | S(64) | 商户详细经营地址或人员所在地点 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.address_info.longitude` | 经度 | `longitude` | 否 | S(11) | - | S(11) | 浮点型, 小数点后最多保留 6 位。如需要录入经纬度，请以高德坐标系为准，录入时请确保经纬度参数准确。高德经纬度查询 http://lbs.amap.com/console/show/picker | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.address_info.latitude` | 纬度 | `latitude` | 否 | S(10) | - | S(10) | 同上 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.address_info.type` | 地址类型 | `type` | 否 | S(32) |  | enum/allowlist; S(32) | 地址类型，取值范围：BUSINESS_ADDRESS：经营地址（默认） | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |

#### 微信参数:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report.request.reportInfo.wechat.bankcard_info.card_no` | 银行卡号 | `card_no` | 是 | S(48) | 6228480402637874213 | S(48) | 银行卡号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.bankcard_info.card_name` | 银行卡持卡人姓名 | `card_name` | 是 | S(32) | 张三 | S(32) | 银行卡持卡人姓名 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.wechat.bankcard_info.bank_branch_name` | 银行开户行名称 | `bank_branch_name` | 否 | S(32) | 招商银行杭州高新支行 | S(32) | 银行开户行名称 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 支付宝参数:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report.request.reportInfo.alipay.name` | 商户名称 | `name` | 是 | S(128) | 商户名称 | S(128) | 该名称是公司主体全称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.alias_name` | 商户简称 | `alias_name` | 是 | S(64) | 商户简称 | S(64) | 商户简称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.service_phone` | 客服电话 | `service_phone` | 是 | S(64) | 075586010000 | S(64) | 商户客服电话 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.category_id` | 商户经营类目 | `category_id` | 否 | S(32) | 2015050700000000 | S(32) | 商户经营类目 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.mcc` | 标准商户类别码 | `mcc` | 是 | S(4) | 5976 | S(4) | 类别码,例如5976表示“专业销售-药品医疗-康复和身体辅助用品”,测试环境传:5411 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.source` | 支付宝pid | `source` | 是 | S(20) | 2088100020003000 | S(20) | 支付宝pid。测试环境传：2088100020003000 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.source_name` | 支付宝pid名称 | `source_name` | 是 | S(64) | 支付宝pid对应的名称 | S(64) | 支付宝pid对应的名称，测试环境传：宝财通收单商户有限公司 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.business_license` | 商户证件编号 | `business_license` | 否 | S(20) | 100000011234561 | conditional rule in description; S(20) | 商户证件编号（企业或者个体工商户提供营业执照，事业单位提供事证号）注：business_license 与 contact_info.id_card_no 两字段至少上送一个。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.business_license_type` | 商户证件类型 | `business_license_type` | 否 | S(32) | NATIONAL_LEGAL | enum/allowlist; S(32) | 商户证件类型，与商户证件编号（business_license）同时出现，商户证件类型，取值范围详见《证件类型》枚举 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchant_report.request.reportInfo.alipay.contact_info` | 商户联系人信息 | `contact_info` | 是 | C | - |  | 商户联系人信息，JSON格式<br>详见：商户联系人信息：contact_info | Validate required; add missing-field test |
| `merchant_report.request.reportInfo.alipay.address_info` | 地址信息 | `address_info` | 是 | C | - |  | 地址信息,JSON格式<br>详见：地址信息：address_info | Validate required; add missing-field test |
| `merchant_report.request.reportInfo.alipay.bankcard_info` | 银行结算卡信息 | `bankcard_info` | 是 | C |  |  | 商户对应银行所开立的结算卡信息，JSON格式<br>详见：银行卡信息：bankcard_info | Validate required; add missing-field test |
| `merchant_report.request.reportInfo.alipay.pay_code_info` | 支付二维码信息 | `pay_code_info` | 否 | C | - |  | 商户的支付二维码中信息，用于营销活动，JSON数组 | Optional: omit empty outbound; tolerate absent inbound |
| `merchant_report.request.reportInfo.alipay.logon_id` | 支付宝账号 | `logon_id` | 否 | C | [“user@domain.com”] |  | 商户的支付宝账号，JSON数组 | Optional: omit empty outbound; tolerate absent inbound |
| `merchant_report.request.reportInfo.alipay.memo` | 备注信息 | `memo` | 否 | S(512) | - | S(512) | 商户备注信息 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.service_codes` | 申请服务 | `service_codes` | 否 | C | [“F2F”] |  | 申请服务，默认情况下开通F2F服务，可传送所有需要开通的服务。允许同时申请多个服务，各服务的准入验证相互独立，服务申请实时生效；详见《支付宝服务类型》,JSON数组 | Optional: omit empty outbound; tolerate absent inbound |
| `merchant_report.request.reportInfo.alipay.site_info` | 网站信息 | `site_info` | 否 | C | - | conditional rule in description | service_codes包含PC和APP时必填,PC必传 site_url，APP必传 site_name, JSON格式<br>详见：网站信息：site_info | Optional: omit empty outbound; tolerate absent inbound |
| `merchant_report.request.reportInfo.alipay.indirect_level` | 间连商户等级 | `indirect_level` | 是 | S(32) | - | enum/allowlist; S(32) | 详见《间连等级》枚举 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |

#### 支付宝参数:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report.request.reportInfo.alipay.contact_info.name` | 姓名 | `name` | 是 | S(128) | 张三 | S(128) | 联系人名字 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.contact_info.phone` | 电话 | `phone` | 否 | S(20) | 0571-85022088 | fixed value; S(20) | 固定电话 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.contact_info.mobile` | 手机号 | `mobile` | 否 | S(20) | 13888888888 | S(20) | 手机号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.contact_info.email` | 邮箱 | `email` | 否 | S(128) | test@test.com | S(128) | 邮箱 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.contact_info.tag` | 联系人业务标识 | `tag` | 是 | C | [“06”,”08”] | enum/allowlist | 表示商户联系人的职责。详《联系人业务标识枚举》，JSON数组 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `merchant_report.request.reportInfo.alipay.contact_info.type` | 联系人类型 | `type` | 是 | S(20) | AGENT | enum/allowlist; S(20) | 商户联系人业务标识枚举，表示商户联系人的职责。详《联系人类型》枚举 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchant_report.request.reportInfo.alipay.contact_info.id_card_no` | 身份证号 | `id_card_no` | 否 | S(32) | 110000199001011234 | conditional rule in description; S(32) | business_license 与 contact_info.id_card_no两字段至少上送一个 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 支付宝参数:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report.request.reportInfo.alipay.address_info.city_code` | 城市编码 | `city_code` | 是 | S(10) | - | S(10) | 城市编码是与国家统计局一致，请查询： ::https://www.mca.gov.cn/n156/n186/index.html | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.address_info.district_code` | 区县编码 | `district_code` | 是 | S(10) | - | S(10) | 区县编码是与国家统计局一致，请查询 : :https://www.mca.gov.cn/n156/n186/index.html | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.address_info.address` | 地址 | `address` | 是 | S(256) |  | S(256) | 商户详细经营地址或人员所在地点 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.address_info.province_code` | 省份编码 | `province_code` | 是 | S(10) |  | S(10) | 省份编码是与国家统计局一致，请查询 : :https://www.mca.gov.cn/n156/n186/index.html | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.address_info.longitude` | 经度 | `longitude` | 否 | S(11) | - | S(11) | 浮点型, 小数点后最多保留 6 位。如需要录入经纬度，请以高德坐标系为准，录入时请确保经纬度参数准确。高德经纬度查询 http://lbs.amap.com/console/show/picker | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.address_info.latitude` | 纬度 | `latitude` | 否 | S(10) | - | S(10) | 同上 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.address_info.type` | 地址类型 | `type` | 否 | S(32) |  | enum/allowlist; S(32) | 地址类型，取值范围：BUSINESS_ADDRESS：经营地址（默认） | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |

#### 支付宝参数:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report.request.reportInfo.alipay.bankcard_info.card_no` | 银行卡号 | `card_no` | 是 | S(48) | 6228480402637874213 | S(48) | 银行卡号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.bankcard_info.card_name` | 银行卡持卡人姓名 | `card_name` | 是 | S(128) | 张三 | S(128) | 银行卡持卡人姓名 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.bankcard_info.bank_branch_name` | 银行开户行名称 | `bank_branch_name` | 否 | S(512) | 招商银行杭州高新支行 | S(512) | 银行开户行名称 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 支付宝参数:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report.request.reportInfo.alipay.site_info.site_type` | 站点类型 | `site_type` | 是 | S(32) | - | enum/allowlist; S(32) | 详见《站点类型》枚举 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `merchant_report.request.reportInfo.alipay.site_info.site_url` | 站点地址 | `site_url` | 否 | S(256) | - | S(256) | 站点地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.site_info.site_name` | 站点名称 | `site_name` | 否 | S(512) | - | S(512) | 站点名称 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.site_info.account` | 账号 | `account` | 否 | S(128) | - | S(128) | 账号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.request.reportInfo.alipay.site_info.password` | 密码 | `password` | 否 | S(128) | - | S(128) | 密码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 返回参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report.response.dataContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.response.dataContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.response.dataContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.response.dataContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.response.dataContent.reportType` | 报备类型 | `reportType` | 是 | E | WECHAT | enum/allowlist | 详见附录：《报备类型》 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `merchant_report.response.dataContent.reportNo` | 报备编号 | `reportNo` | 是 | S(64) | 20211220120030798 | S(64) | 每次请求报备接口唯一编号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.response.dataContent.reportState` | 报备状态 | `reportState` | 否 | S(16) | SUCCESS | S(16) | 详见《报备状态》 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.response.dataContent.subMchId` | 商户识别码 | `subMchId` | 否 | S(30) |  | S(30) | 微信/支付宝分配的商户识别码 resultCode=SUCCESS时有值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Channel merchant id only; never use as share receiver |
| `merchant_report.response.dataContent.platformBizNo` | 业务号 | `platformBizNo` | 否 | S(30) | API2121122548678654 | S(30) | resultCode=SUCCESS时有值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.response.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | 业务处理结果 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report.response.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) |  | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report.response.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) |  | S(128) | 当业务结果为FAIL时，返回错误描述 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 报备信息查询 (`bct-1f9o63b6ufii5`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9o63b6ufii5`; updated: `2024-01-04 05:31`.


#### 接口说明

| 接口名称 | merchant_report_query |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

#### 请求参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report_query.request.bizContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report_query.request.bizContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report_query.request.bizContent.merId` | 交易商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report_query.request.bizContent.terId` | 交易终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report_query.request.bizContent.reportType` | 报备类型 | `reportType` | 是 | E | WECHAT | enum/allowlist | 详见附录：《报备类型》 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `merchant_report_query.request.bizContent.reportNo` | 报备编号 | `reportNo` | 是 | S(64) | 20211220120030798 | S(64) | 每次请求报备接口唯一编号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report_query.response.dataContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report_query.response.dataContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report_query.response.dataContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report_query.response.dataContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report_query.response.dataContent.reportType` | 报备类型 | `reportType` | 是 | E | WECHAT | enum/allowlist | 详见附录：《报备类型》 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `merchant_report_query.response.dataContent.reportNo` | 报备编号 | `reportNo` | 是 | S(64) | 20211220120030798 | S(64) | 每次请求报备接口唯一编号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report_query.response.dataContent.reportState` | 报备状态 | `reportState` | 否 | S(16) | SUCCESS | S(16) | 详见《报备状态》 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report_query.response.dataContent.channelRetParam` | 渠道参数 | `channelRetParam` | 否 | C |  |  | 查询报备接口渠道返回参数，JSON格式<br>详见：渠道参数：channelRetParam | Optional: omit empty outbound; tolerate absent inbound |
| `merchant_report_query.response.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | 业务处理结果 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `merchant_report_query.response.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) |  | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `merchant_report_query.response.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) |  | S(128) | 当业务结果为FAIL时，返回错误描述 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 微信返回：

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report_query.response.channelRetParam.wechat.sub_mch_id` | 商户识别码 | `sub_mch_id` | 否 | S(30) | - | S(30) | 微信分配的商户识别码 resultCode=SUCCESS时有值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Channel merchant id only; never use as share receiver |

#### 支付宝返回：

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `merchant_report_query.response.channelRetParam.alipay.sub_mch_id` | 商户识别码 | `sub_mch_id` | 否 | S(30) | - | S(30) | 微信分配的商户识别码 resultCode=SUCCESS时有值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Channel merchant id only; never use as share receiver |
| `merchant_report_query.response.channelRetParam.alipay.indirect_level` | 间连等级 | `indirect_level` | 否 | S(16) | M1 | enum/allowlist; S(16) | 间连商户等级，详见《间连等级枚举》 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |

### 绑定授权目录 (`bct-1f9o63qmkndkc`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9o63qmkndkc`; updated: `2024-04-03 03:22`.


#### 接口说明

| 接口名称 | bind_sub_config |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

#### 请求参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bind_sub_config.request.bizContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bind_sub_config.request.bizContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bind_sub_config.request.bizContent.merId` | 交易商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bind_sub_config.request.bizContent.terId` | 交易终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bind_sub_config.request.bizContent.subMchId` | 商户识别码 | `subMchId` | 是 | S(30) |  | S(30) | 微信/支付宝分配的商户识别码 | Validate required; add missing-field test; Preserve official length/type at boundary; Channel merchant id only; never use as share receiver |
| `bind_sub_config.request.bizContent.authType` | 授权类型 | `authType` | 是 | E | AUTH | enum/allowlist | 详见附录：《授权类型》 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `bind_sub_config.request.bizContent.authContent` | 授权内容 | `authContent` | 是 | S(256) | http://www.qq.com/wechat | S(256) | 1 授权类型为AUTH填写支付路径，要求符合URI格式规范，每次添加一个支付目录，最多5个<br>2 授权类型为JSAPI需填写公众号appid<br>3 授权类型为APPLET需填写小程序appid | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bind_sub_config.request.bizContent.remark` | 备注 | `remark` | 是 | S(128) | JSAPI授权目录 | S(128) | 备注信息 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bind_sub_config.response.dataContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bind_sub_config.response.dataContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bind_sub_config.response.dataContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bind_sub_config.response.dataContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bind_sub_config.response.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | 业务处理结果 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bind_sub_config.response.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) |  | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bind_sub_config.response.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) |  | S(128) | 当业务结果为FAIL时，返回错误描述 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

## 4. Aggregate Payment / Share / Refund Field Matrix

### 接口请求入口 (`bct-1f9qhakcna6te`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qhakcna6te`; updated: `2024-12-30 03:13`.


#### 请求参数格式约定如下

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `aggregate.publicRequest.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `aggregate.publicRequest.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `aggregate.publicRequest.method` | 接口名称 | `method` | 是 | S(64) | unified_order | S(64) | 对应接口的名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `aggregate.publicRequest.charset` | 字符集 | `charset` | 是 | S(16) | UTF-8 | fixed value; S(16) | 字符集编码，固定值UTF-8 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicRequest.version` | 接口版本号 | `version` | 是 | S(8) | 1.0 | fixed value; S(8) | 对应接口的版本号，固定值1.0 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicRequest.format` | 格式化类型 | `format` | 是 | S(8) | json | fixed value; S(8) | 各个接口业务参数的格式化类型，固定值json | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicRequest.timestamp` | 时间戳 | `timestamp` | 是 | T | 20210315155012 | date/time format | 时间戳与宝付支付系统时间误差不超过10分钟，格式为yyyyMMddHHmmss，如：2021年3月15日15点50分12秒表示为：20210315155012 | Validate required; add missing-field test; Validate official time/date format |
| `aggregate.publicRequest.signType` | 加密签名类型 | `signType` | 是 | E | SM2 | enum/allowlist | 商户生成签名和加密字符串使用的算法类型，见附录【签名类型】 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicRequest.signSn` | 签名证书序列号 | `signSn` | 是 | S(10) | 1 | S(10) | 发送方公钥证书序列号，用于接收方验签证书选择 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `aggregate.publicRequest.ncrptnSn` | 加密证书序列号 | `ncrptnSn` | 是 | S(10) | 1 | S(10) | 接收方公钥证书序列号，用于接收方解密证书选择 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `aggregate.publicRequest.dgtlEnvlp` | 数字信封 | `dgtlEnvlp` | 否 | S |  |  | 使用接收方公钥加密的对称密钥，并做16进制转码，是否需要传值，详见各个业务接口说明 | Optional: omit empty outbound; tolerate absent inbound; Security boundary: sign/decrypt/verify before business parsing |
| `aggregate.publicRequest.signStr` | 签名串 | `signStr` | 是 | S |  |  | 使用发送方私钥签名的非对称密钥，并做16进制转码 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |
| `aggregate.publicRequest.bizContent` | 业务参数 | `bizContent` | 是 | S |  |  | 业务数据报文，JSON格式，具体见各业务接口定义 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |

#### 返回参数格式约定如下:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `aggregate.publicResponse.transport.returnCode` | 返回状态码 | `returnCode` | 是 | S(16) | SUCCESS | S(16) | 成功：SUCCESS 失败：FAIL 通信标识，非交易标识 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `aggregate.publicResponse.transport.returnMsg` | 返回信息 | `returnMsg` | 是 | S(128) | OK | S(128) | 当returnCode返回FAIL时返回错误信息，如：验签失败 解密失败 商户号不存在 参数格式验证错误等 检查报文重新发起请求 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数格式约定如下:

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `aggregate.publicResponse.success.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `aggregate.publicResponse.success.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `aggregate.publicResponse.success.charset` | 字符集 | `charset` | 是 | S(16) | UTF-8 | fixed value; S(16) | 字符集编码，固定值UTF-8 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicResponse.success.version` | 接口版本号 | `version` | 是 | S(8) | 1.0 | fixed value; S(8) | 对应接口的版本号，固定值1.0 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicResponse.success.format` | 格式化类型 | `format` | 是 | S(8) | json | fixed value; S(8) | 各个接口业务参数的格式化类型，固定值json | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicResponse.success.signType` | 加密签名类型 | `signType` | 是 | E | SM2 | enum/allowlist | 与商户原请求签名加密类型一致，如：商户请求采用国密类型，则对应返回也采用国密类型 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicResponse.success.signSn` | 签名证书序列号 | `signSn` | 是 | S(10) | 1 | fixed value; S(10) | 发送方公钥证书序列号，用于接收方验签证书选择，固定值1 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicResponse.success.ncrptnSn` | 加密证书序列号 | `ncrptnSn` | 是 | S(10) | 1 | fixed value; S(10) | 接收方公钥证书序列号，用于接收方解密证书选择，固定值1 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicResponse.success.dgtlEnvlp` | 数字信封 | `dgtlEnvlp` | 否 | S |  |  | 使用接收方公钥加密的对称密钥，并做16进制转码，返回是否有值，详见各个业务接口返回参数说明 | Optional: omit empty outbound; tolerate absent inbound; Security boundary: sign/decrypt/verify before business parsing |
| `aggregate.publicResponse.success.signStr` | 签名串 | `signStr` | 是 | S |  |  | 使用发送方私钥签名的非对称密钥，并做16进制转码 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |
| `aggregate.publicResponse.success.dataContent` | 返回参数 | `dataContent` | 是 | S |  |  | 业务数据报文，JSON格式，具体见各业务接口返回参数说明 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |

#### 通知参数格式约定如下：

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `aggregate.publicNotification.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `aggregate.publicNotification.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `aggregate.publicNotification.charset` | 字符集 | `charset` | 是 | S(16) | UTF-8 | fixed value; S(16) | 字符集编码，固定值UTF-8 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicNotification.version` | 接口版本号 | `version` | 是 | S(8) | 1.0 | fixed value; S(8) | 对应接口的版本号，固定值1.0 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicNotification.format` | 格式化类型 | `format` | 是 | S(8) | json | fixed value; S(8) | 各个接口业务参数的格式化类型，固定值json | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicNotification.notifyType` | 通知类型 | `notifyType` | 是 | E | PAYMENT | enum/allowlist | 本次通知的订单类型，见附录：【通知类型】 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicNotification.signType` | 加密签名类型 | `signType` | 是 | E | SM2 | enum/allowlist | 与商户原请求签名加密类型一致，如：商户请求采用国密类型，则对应返回也采用国密类型 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicNotification.signSn` | 签名证书序列号 | `signSn` | 是 | S(10) | 1 | fixed value; S(10) | 发送方公钥证书序列号，用于接收方验签证书选择，固定值1 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicNotification.ncrptnSn` | 加密证书序列号 | `ncrptnSn` | 是 | S(10) | 1 | fixed value; S(10) | 接收方公钥证书序列号，用于接收方解密证书选择，固定值1 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `aggregate.publicNotification.dgtlEnvlp` | 数字信封 | `dgtlEnvlp` | 否 | S |  |  | 使用接收方公钥加密的对称密钥，并做16进制转码，返回是否有值，详见各个业务接口返回参数说明 | Optional: omit empty outbound; tolerate absent inbound; Security boundary: sign/decrypt/verify before business parsing |
| `aggregate.publicNotification.signStr` | 签名串 | `signStr` | 是 | S |  |  | 使用发送方私钥签名的非对称密钥，并做16进制转码 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |
| `aggregate.publicNotification.dataContent` | 返回参数 | `dataContent` | 是 | S |  |  | 业务数据报文，JSON格式，具体见各业务接口返回参数说明 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |

### 统一下单交易创建 (`bct-1f9qlvjef634j`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qlvjef634j`; updated: `2024-12-23 10:33`.


#### 接口说明

| 接口名称 | unified_order |
| --- | --- |
| 是否幂等 | 是 |
| 接口模式 | 直连 |
| 异步通知 | 是 |

#### 请求参数：

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `unified_order.request.bizContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.request.bizContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.request.bizContent.merId` | 交易商户号 | `merId` | 是 | S(16) |  | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `unified_order.request.bizContent.terId` | 交易终端号 | `terId` | 是 | S(16) |  | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `unified_order.request.bizContent.outTradeNo` | 交易商户订单号 | `outTradeNo` | 是 | S(32) | 20210315155012 | S(32) | 商户系统内部订单号，同一个商户号下唯一 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `unified_order.request.bizContent.txnAmt` | 用户实际付款金额 | `txnAmt` | 是 | I | 100 | unit=fen | 交易金额，单位：分，如：1元则传入100 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |
| `unified_order.request.bizContent.txnTime` | 交易时间 | `txnTime` | 是 | T | 20210315155012 | date/time format | 订单交易时间 | Validate required; add missing-field test; Validate official time/date format |
| `unified_order.request.bizContent.totalAmt` | 订单总金额 | `totalAmt` | 是 | I | 100 |  | 如包含营销信息，则订单总金额=用户实际付款金额+营销总金额，反之订单总金额=用户实际付款金额 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |
| `unified_order.request.bizContent.timeExpire` | 订单有效时间 | `timeExpire` | 否 | I | 72460 | conditional rule in description; unit=fen | 订单支付的有效时间，单位：分钟，不传此参数则宝付支付默认有效时间30分钟，允许最大时效7天 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer; Validate official time/date format |
| `unified_order.request.bizContent.prodType` | 产品类型 | `prodType` | 是 | E | SHARING | enum/allowlist | 详见附录：产品类型 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `unified_order.request.bizContent.orderType` | 订单类型 | `orderType` | 是 | S | 7 | conditional rule in description | 宝财通2.0模式必传。传值:7 | Validate required; add missing-field test |
| `unified_order.request.bizContent.payCode` | 支付方式 | `payCode` | 是 | E | WECHAT_JSAPI | enum/allowlist | 详见附录：【支付方式】 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `unified_order.request.bizContent.payExtend` | 支付方式属性 | `payExtend` | 是 | C | 微信公众号为例：{“sub_openid”:”1231231231”,”sub_appid”:”1231231123”,”body”:”特价手机”} | enum/allowlist | 根据传入的支付方式选择相应的支付属性。<br>详见附录：【支付属性】 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `unified_order.request.bizContent.subMchId` | 聚合交易商户号 | `subMchId` | 否 | S(64) |  | conditional rule in description; S(64) | 微信/支付宝必传，在微信/支付宝报备的二级商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Channel merchant id only; never use as share receiver |
| `unified_order.request.bizContent.notifyUrl` | 服务端通知地址 | `notifyUrl` | 否 | S(128) | https://www.example.com/return_url | S(128) | 付款成功后请求商户侧服务端地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.request.bizContent.pageUrl` | 页面端跳转地址 | `pageUrl` | 否 | S(128) | https://www.example.com/caallback_url | S(128) | 支付完成后跳转的地址：必须是https协议 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.request.bizContent.forbidCredit` | 禁止贷记卡支付 | `forbidCredit` | 否 | S(1) | 0 | conditional rule in description; S(1) | 1：禁止0：不禁止不传默认为0 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.request.bizContent.attach` | 附加字段 | `attach` | 否 | S(128) |  | S(128) | 预留字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.request.bizContent.reqReserved` | 请求方保留域 | `reqReserved` | 否 | S(128) |  | S(128) | 预留字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.request.bizContent.mktInfo` | 营销信息 | `mktInfo` | 否 | S | {“mktAmt”:100,”mktMerId”:”100000”} | conditional rule in description | JSON格式，目前仅支持交易商户承担营销金额<br>[无营销金额时该字段不上送]<br>详见：营销信息：mktInfo | Optional: omit empty outbound; tolerate absent inbound |
| `unified_order.request.bizContent.riskInfo` | 风控信息 | `riskInfo` | 否 | S | {“clientIp”:”XXX”,”locationPoint”:”XXX”} | conditional rule in description | 微信/支付宝必传，JSON格式 | Optional: omit empty outbound; tolerate absent inbound |

#### 营销信息：mktInfo

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `unified_order.request.mktInfo.mktMerId` | 商户号 | `mktMerId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `unified_order.request.mktInfo.mktAmt` | 营销金额 | `mktAmt` | 是 | I | 100 | unit=fen | 营销金额，单位：分，如：1元则传入100 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |

#### 风控信息：riskInfo

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `unified_order.request.riskInfo.clientIp` | 用户ip地址 | `clientIp` | 是 | S(64) | 100000 | S(64) | 付款用户ip地址 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `unified_order.request.riskInfo.locationPoint` | 交易商户终端经纬度 | `locationPoint` | 否 | S(128) | 100,100 | S(128) | 包含经度和纬度，英文逗号分隔 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 返回参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `unified_order.response.dataContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.response.dataContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.response.dataContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `unified_order.response.dataContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `unified_order.response.dataContent.outTradeNo` | 商户订单号 | `outTradeNo` | 是 | S(64) | 20210315155012 | S(64) | 商户系统内部订单号，同一个商户号下唯一 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `unified_order.response.dataContent.txnState` | 订单状态 | `txnState` | 否 | E | WAIT_PAYING | enum/allowlist | 订单状态，详见附录 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `unified_order.response.dataContent.tradeNo` | 宝付交易号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户订单号对应的宝付侧唯一交易号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.response.dataContent.reqChlNo` | 请求渠道订单号 | `reqChlNo` | 否 | S(64) |  | S(64) | 宝付请求渠道订单号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.response.dataContent.payCode` | 支付方式 | `payCode` | 是 | E |  | enum/allowlist | 原样返回 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `unified_order.response.dataContent.chlRetParam` | 渠道返回参数 | `chlRetParam` | 否 | C |  | enum/allowlist | 根据不同的支付方式返回相应的业务参数，作为商户侧唤起支付，详见附录：【统一下单渠道返回参数】 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `unified_order.response.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | 业务处理结果 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `unified_order.response.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) |  | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `unified_order.response.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) |  | S(128) | 当业务结果为FAIL时，返回错误描述 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 支付订单查询 (`bct-1f9qm13po92jq`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qm13po92jq`; updated: `2024-04-02 00:56`.


#### 接口说明

| 接口名称 | order_query |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

#### 请求参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `order_query.request.bizContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_query.request.bizContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_query.request.bizContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_query.request.bizContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_query.request.bizContent.tradeNo` | 宝付交易号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户订单号对应的宝付侧唯一订单号，推荐传入此值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_query.request.bizContent.outTradeNo` | 商户订单号 | `outTradeNo` | 否 | S(32) | 20210315155012 | S(32) | 商户系统内部订单号，同一个商户号下唯一 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 返回参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `order_query.response.dataContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_query.response.dataContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_query.response.dataContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_query.response.dataContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_query.response.dataContent.tradeNo` | 宝付交易号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户订单号对应的宝付侧唯一订单号，推荐传入此值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_query.response.dataContent.outTradeNo` | 商户订单号 | `outTradeNo` | 否 | S(32) | 20210315155012 | S(32) | 商户系统内部订单号，同一个商户号下唯一 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_query.response.dataContent.txnState` | 订单状态 | `txnState` | 否 | E | REFUND | enum/allowlist | 详见附录订单状态 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `order_query.response.dataContent.finishTime` | 完成时间 | `finishTime` | 否 | T | 20210315155012 | date/time format | 订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `order_query.response.dataContent.succAmt` | 成功金额 | `succAmt` | 否 | I | 100 | unit=fen | 单位：分，订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `order_query.response.dataContent.feeAmt` | 支付手续费 | `feeAmt` | 否 | I | 100 | unit=fen | 单位：分，订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `order_query.response.dataContent.instFeeAmt` | 分期手续费 | `instFeeAmt` | 否 | I | 100 | unit=fen | 单位：分，商户使用分期产品支付时，订单状态为成功时有值 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `order_query.response.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | SUCCESS：成功 FAIL：失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_query.response.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) | SUCCESS | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_query.response.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) | SUCCESS | S(128) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_query.response.dataContent.reqChlNo` | 请求渠道订单号 | `reqChlNo` | 否 | S | SUCCESS |  | 支付成功时返回 | Optional: omit empty outbound; tolerate absent inbound |
| `order_query.response.dataContent.payCode` | 支付方式 | `payCode` | 否 | E | SUCCESS | enum/allowlist | 支付成功时返回 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `order_query.response.dataContent.chlRetParam` | 渠道返回参数 | `chlRetParam` | 否 | C | SUCCESS |  | 根据不同的支付方式返回相应的业务参数详见：渠道返回参数 | Optional: omit empty outbound; tolerate absent inbound |
| `order_query.response.dataContent.clearingDate` | 清算日期 | `clearingDate` | 否 | D | 20210501 | date/time format |  | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |

#### 支付宝

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `order_query.response.chlRetParam.alipay.buyerId` | 买家编号 | `buyerId` | 否 | S(128) |  | S(128) | 买家支付宝用户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_query.response.chlRetParam.alipay.accountNo` | 客户账户号 | `accountNo` | 否 | S(128) |  | S(128) | 支付宝会返回 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 微信

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `order_query.response.chlRetParam.wechat.openId` | 微信用户标识 | `openId` | 否 | S(128) |  | S(128) | 用户在服务商公众号appid下的唯一标识 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_query.response.chlRetParam.wechat.subOpenid` | 买家编号 | `subOpenid` | 否 | S(128) |  | S(128) | 微信平台的sub_openid | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 交易关闭 (`bct-1f9qm0flcca1k`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qm0flcca1k`; updated: `2024-01-05 05:41`.


#### 接口说明

| 接口名称 | order_close |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 是 |

#### 请求参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `order_close.request.bizContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_close.request.bizContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_close.request.bizContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_close.request.bizContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_close.request.bizContent.tradeNo` | 宝付订单号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户订单号对应的宝付侧唯一订单号，推荐传入此值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_close.request.bizContent.outTradeNo` | 商户订单号 | `outTradeNo` | 否 | S(32) | 20210315155012 | S(32) | 商户系统内部订单号，同一个商户号下唯一 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 返回参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `order_close.response.dataContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_close.response.dataContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_close.response.dataContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_close.response.dataContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_close.response.dataContent.tradeNo` | 宝付订单号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户订单号对应的宝付侧唯一订单号，推荐传入此值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_close.response.dataContent.outTradeNo` | 商户订单号 | `outTradeNo` | 否 | S(32) | 20210315155012 | S(32) | 商户系统内部订单号，同一个商户号下唯一 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_close.response.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | SUCCESS：成功，FAIL：失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_close.response.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) |  | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_close.response.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) |  | S(128) | 当业务结果为FAIL时，返回错误描述 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 支付结果通知 (`bct-1f9qm4ujg50cv`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qm4ujg50cv`; updated: `2024-09-23 08:13`.


#### 通知参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `paymentNotify.dataContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `paymentNotify.dataContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `paymentNotify.dataContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `paymentNotify.dataContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `paymentNotify.dataContent.tradeNo` | 宝付订单号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户订单号对应的宝付侧唯一订单号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `paymentNotify.dataContent.outTradeNo` | 商户订单号 | `outTradeNo` | 否 | S(32) | 20210315155012 | S(32) | 商户系统内部订单号，同一个商户号下唯一 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `paymentNotify.dataContent.txnState` | 订单状态 | `txnState` | 否 | E | REFUND | enum/allowlist | 详见附录订单状态 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `paymentNotify.dataContent.finishTime` | 完成时间 | `finishTime` | 否 | T | 20210315155012 | date/time format | 订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `paymentNotify.dataContent.succAmt` | 成功金额 | `succAmt` | 否 | I | 100 | unit=fen | 单位：分，订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `paymentNotify.dataContent.feeAmt` | 支付手续费 | `feeAmt` | 否 | I | 100 | unit=fen | 单位：分，订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `paymentNotify.dataContent.instFeeAmt` | 分期手续费 | `instFeeAmt` | 否 | I | 100 | unit=fen | 单位：分，商户使用分期产品支付时，订单状态为成功时有值 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `paymentNotify.dataContent.resultCode` | 业务结果 | `resultCode` | 否 | S(16) | SUCCESS | S(16) | SUCCESS：成功 FAIL：失败 ，注：关单场景不返回 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `paymentNotify.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) | SUCCESS | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `paymentNotify.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) | SUCCESS | S(128) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `paymentNotify.dataContent.reqChlNo` | 请求渠道订单号 | `reqChlNo` | 否 | S | SUCCESS |  | 支付成功时返回 | Optional: omit empty outbound; tolerate absent inbound |
| `paymentNotify.dataContent.payCode` | 支付方式 | `payCode` | 是 |  |  |  |  | Validate required; add missing-field test |
| `paymentNotify.dataContent.chlRetParam` | 渠道返回参数 | `chlRetParam` | 否 | C |  |  | 根据不同的支付方式返回相应的业务参数详见：渠道返回参数 | Optional: omit empty outbound; tolerate absent inbound |
| `paymentNotify.dataContent.clearingDate` | 清算日期 | `clearingDate` | 否 | D | 20210501 | date/time format |  | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |

### 确认分账 (`bct-1f9qlvu1em0tb`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qlvu1em0tb`; updated: `2026-01-13 02:03`.


#### 接口说明

| 接口名称 | share_after_pay |
| --- | --- |
| 是否幂等 | 是 |
| 接口模式 | 直连 |
| 异步通知 | 是 |

#### 请求参数：

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `share_after_pay.request.bizContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_after_pay.request.bizContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_after_pay.request.bizContent.merId` | 交易商户号 | `merId` | 是 | S(16) |  | S(16) |  | Validate required; add missing-field test; Preserve official length/type at boundary |
| `share_after_pay.request.bizContent.terId` | 交易终端号 | `terId` | 是 | S(16) |  | S(16) |  | Validate required; add missing-field test; Preserve official length/type at boundary |
| `share_after_pay.request.bizContent.originTradeNo` | 原支付订单宝付订单号 | `originTradeNo` | 否 | S(32) | 12312312312 | conditional rule in description; S(32) | 与商户订单号对应的宝付侧唯一订单号，推荐传入此值。originTradeNo和originOutTradeNo必须二选一上送 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_after_pay.request.bizContent.originOutTradeNo` | 原支付订单商户订单号 | `originOutTradeNo` | 否 | S(64) | 20210315155012 | conditional rule in description; S(64) | 商户系统内部订单号，同一个商户号下唯一。originTradeNo和originOutTradeNo必须二选一上送 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_after_pay.request.bizContent.txnTime` | 交易时间 | `txnTime` | 是 | T | 20210315155012 | date/time format | 发起分账交易时间 | Validate required; add missing-field test; Validate official time/date format |
| `share_after_pay.request.bizContent.outTradeNo` | 分账订单号 | `outTradeNo` | 是 | S(50) | 100000 | S(50) | 商户分账订单号，查询分账订单 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `share_after_pay.request.bizContent.notifyUrl` | 分账结果通知地址 | `notifyUrl` | 否 | S(128) | http://www.example.com/notify | conditional rule in description; S(128) | 宝付分账完成通知商户侧接收地址，不传入此值则不通知 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_after_pay.request.bizContent.sharingDetails` | 分账信息 | `sharingDetails` | 是 | C | “sharingDetails”:[{“sharingAmt”:100,”sharingMerId”:”100000”},{“sharingAmt”:200,”sharingMerId”:”100001”}] |  | JSON数组 | Validate required; add missing-field test |
| `share_after_pay.request.bizContent.sharingMerId` | -商户号 | `sharingMerId` | 是 | S(64) | 100000 | S(64) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `share_after_pay.request.bizContent.sharingAmt` | -分账金额 | `sharingAmt` | 是 | I | 100 | unit=fen | 分账金额，单位：分，如：1元则传入100 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |

#### 返回参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `share_after_pay.response.dataContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_after_pay.response.dataContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_after_pay.response.dataContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `share_after_pay.response.dataContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `share_after_pay.response.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | 业务处理结果 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `share_after_pay.response.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) |  | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_after_pay.response.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) |  | S(128) | 当业务结果为FAIL时，返回错误描述 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_after_pay.response.dataContent.tradeNo` | 宝付订单号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户订单号对应的宝付侧唯一订单号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_after_pay.response.dataContent.txnState` | 订单状态 | `txnState` | 是 | E | REFUND | enum/allowlist | 详见附录订单状态 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `share_after_pay.response.dataContent.finishTime` | 完成时间 | `finishTime` | 否 | T | 20210315155012 | date/time format | 订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `share_after_pay.response.dataContent.succAmt` | 分账成功金额 | `succAmt` |  | I |  | unit=fen | 单位：分 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `share_after_pay.response.dataContent.clearingDate` | 清算日期 | `clearingDate` | 否 | D |  | date/time format |  | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |

### 分账订单查询 (`bct-1f9qm1m0u1s68`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qm1m0u1s68`; updated: `2024-01-05 05:51`.


#### 接口说明

| 接口名称 | share_query |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

#### 请求参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `share_query.request.bizContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_query.request.bizContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_query.request.bizContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `share_query.request.bizContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `share_query.request.bizContent.tradeNo` | 宝付分账交易号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户分账订单号对应的宝付侧唯一分账交易号，推荐传入此值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_query.request.bizContent.outTradeNo` | 商户分账订单号 | `outTradeNo` | 否 | S(50) | 20210315155012 | S(50) | 商户系统内部订单号，同一个商户号下唯一 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 返回参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `share_query.response.dataContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_query.response.dataContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_query.response.dataContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `share_query.response.dataContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `share_query.response.dataContent.tradeNo` | 宝付分账交易号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户分账订单号对应的宝付侧唯一分账交易号，推荐传入此值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_query.response.dataContent.outTradeNo` | 商户分账订单号 | `outTradeNo` | 否 | S(32) | 20210315155012 | S(32) | 商户系统内部订单号，同一个商户号下唯一 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_query.response.dataContent.txnState` | 订单状态 | `txnState` | 是 | E | REFUND | enum/allowlist | 详见附录订单状态 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `share_query.response.dataContent.finishTime` | 完成时间 | `finishTime` | 否 | T | 20210315155012 | date/time format | 订单状态为成功时才有值格式为yyyyMMddHHmmss，如：2021年3月15日15点50分12秒表示为：20210315155012 | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `share_query.response.dataContent.succAmt` | 分账成功金额 | `succAmt` |  | I |  | unit=fen | 单位：分 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `share_query.response.dataContent.clearingDate` | 清算日期 | `clearingDate` | 否 | D |  | date/time format |  | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `share_query.response.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | SUCCESS：成功 FAIL：失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `share_query.response.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) | SUCCESS | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `share_query.response.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) | SUCCESS | S(128) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 分账结果通知 (`bct-1f9qm58emskkg`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qm58emskkg`; updated: `2024-01-05 03:40`.


#### 通知参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `shareNotify.dataContent.tradeNo` | 宝付交易号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户订单号对应的宝付侧唯一订单号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `shareNotify.dataContent.outTradeNo` | 商户订单号 | `outTradeNo` | 否 | S(32) | 20210315155012 | S(32) | 商户系统内部订单号，同一个商户号下唯一 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `shareNotify.dataContent.txnState` | 订单状态 | `txnState` | 是 | E | REFUND | enum/allowlist | 详见附录订单状态 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `shareNotify.dataContent.finishTime` | 完成时间 | `finishTime` | 否 | T | 20210315155012 | date/time format | 订单状态为成功时才有值格式为yyyyMMddHHmmss，如：2021年3月15日15点50分12秒表示为：20210315155012 | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `shareNotify.dataContent.succAmt` | 成功金额 | `succAmt` | 否 | I | 100 | unit=fen | 单位：分，订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `shareNotify.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | SUCCESS：成功 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `shareNotify.dataContent.clearingDate` | 清算日期 | `clearingDate` | 否 | D | 20210501 | date/time format |  | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |

### 申请退款 (`bct-1f9qm06dmb1a9`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qm06dmb1a9`; updated: `2025-07-15 06:42`.


#### 接口说明

| 接口名称 | order_refund |
| --- | --- |
| 是否幂等 | 是 |
| 接口模式 | 直连 |
| 异步通知 | 是 |

#### 请求参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `order_refund.request.bizContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.request.bizContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.request.bizContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_refund.request.bizContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_refund.request.bizContent.merchantName` | 商户名称 | `merchantName` | 否 | S(128) | 商户名称 | S(128) | 商户名称 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.request.bizContent.originTradeNo` | 原支付订单宝付交易号 | `originTradeNo` | 否 | S(32) | 12312312312 | S(32) | 原支付订单宝付交易号，推荐传入此值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.request.bizContent.originOutTradeNo` | 原支付订单商户订单号 | `originOutTradeNo` | 否 | S(32) | 20210315155012 | conditional rule in description; S(32) | 原支付订单商户订单号，与原支付订单宝付交易号二选一比传 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.request.bizContent.outTradeNo` | 退款订单号 | `outTradeNo` | 是 | S(50) | 20210315155013 | S(50) | 商户系统内部退款订单号，同一个商户号下唯一 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_refund.request.bizContent.notifyUrl` | 服务端通知地址 | `notifyUrl` | 否 | S(128) | https://www.example.com/return_url | S(128) | 退款处理完成后请求商户侧服务端地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.request.bizContent.refundAmt` | 退款金额 | `refundAmt` | 是 | I | 100 | unit=fen | 单位：分，退款金额不得大于用户实际付款金额 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |
| `order_refund.request.bizContent.totalAmt` | 退款总金额 | `totalAmt` | 是 | I | 200 |  | 如包含营销信息，则退款总金额=退款金额+营销退款总金额，反之退款总金额=退款金额 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |
| `order_refund.request.bizContent.txnTime` | 交易时间 | `txnTime` | 是 | T | 20210315155012 | date/time format | 退款发起时间 | Validate required; add missing-field test; Validate official time/date format |
| `order_refund.request.bizContent.attach` | 附加字段 | `attach` | 否 | S(128) |  | S(128) | 预留字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.request.bizContent.reqReserved` | 请求方保留域 | `reqReserved` | 否 | S(128) |  | S(128) | 预留字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.request.bizContent.sharingRefundInfo` | 分账退款信息 | `sharingRefundInfo` | 否 | C | [{“sharingAmt”:100,”sharingMerId”:”100000”},{“sharingAmt”:200,”sharingMerId”:”100001”}] |  | JSON数组 | Optional: omit empty outbound; tolerate absent inbound |
| `order_refund.request.bizContent.sharingMerId` | -商户号 | `sharingMerId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `order_refund.request.bizContent.sharingAmt` | -分账金额 | `sharingAmt` | 是 | I | 100 | unit=fen | 分账金额，单位：分，如1元则传入100 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |
| `order_refund.request.bizContent.mktRefundInfo` | 营销退款信息 | `mktRefundInfo` | 否 | C | {“mktAmt”:100,”mktMerId”:”100000”} |  | JSON格式，目前仅支持交易商户承担营销金额 | Optional: omit empty outbound; tolerate absent inbound |
| `order_refund.request.bizContent.mktMerId` | -商户号 | `mktMerId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_refund.request.bizContent.mktAmt` | -营销金额 | `mktAmt` | 是 | I | 100 | unit=fen | 营销金额，单位：分，如：1元则传入100 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |
| `order_refund.request.bizContent.advanceAmt` | 垫资金额 | `advanceAmt` | 否 | I | 100 |  | 如果需要垫资传入<br>注：垫资可能导致资金损失，请商户确认需要后谨慎选择 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `order_refund.request.bizContent.refundReason` | 退款原因 | `refundReason` | 是 | s(128) |  |  |  | Validate required; add missing-field test |

#### 返回参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `order_refund.response.dataContent.originTradeNo` | 原支付订单宝付交易号 | `originTradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户订单号对应的宝付侧唯一订单号，推荐传入此值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.response.dataContent.originOutTradeNo` | 原支付订单商户订单号 | `originOutTradeNo` | 否 | S(32) | 20210315155012 | S(32) | 商户系统内部订单号，同一个商户号下唯一 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.response.dataContent.outTradeNo` | 商户退款订单号 | `outTradeNo` | 是 | S(32) | 20210315155013 | S(32) | 商户系统内部退款订单号，同一个商户号下唯一 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_refund.response.dataContent.tradeNo` | 宝付退款交易号 | `tradeNo` | 是 | S(32) | 12312312312 | S(32) | 与商户退款订单号对应的宝付侧唯一退款订单号， | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_refund.response.dataContent.refundAmt` | 退款金额 | `refundAmt` | 是 | I | 100 | unit=fen | 单位：分，退款金额不得大于用户实际付款金额 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |
| `order_refund.response.dataContent.totalAmt` | 退款总金额 | `totalAmt` | 是 | I | 200 |  | 如包含营销信息，则退款总金额=退款金额+营销退款总金额，反之退款总金额=退款金额 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |
| `order_refund.response.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | SUCCESS：业务受理成功，FAIL：业务受理失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `order_refund.response.dataContent.refundState` | 订单状态 | `refundState` | 否 | E | REFUND | enum/allowlist | 详见附录订单状态 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `order_refund.response.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) |  | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.response.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) |  | S(128) | 当业务结果为FAIL时，返回错误描述 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `order_refund.response.dataContent.reqReserved` | 请求方保留域 | `reqReserved` | 否 | S(128) |  | S(128) | 预留字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 退款订单查询 (`bct-1f9qm246c6cp8`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qm246c6cp8`; updated: `2024-01-05 05:52`.


#### 接口说明

| 接口名称 | refund_query |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

#### 请求参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `refund_query.request.bizContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `refund_query.request.bizContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `refund_query.request.bizContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `refund_query.request.bizContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `refund_query.request.bizContent.outTradeNo` | 退款订单号 | `outTradeNo` | 否 | S(50) | 20210315155012 | S(50) | 商户系统内部退款订单号，同一个商户号下唯一 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `refund_query.request.bizContent.tradeNo` | 宝付订单号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户退款订单号对应的宝付侧唯一退款订单号， | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 返回参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `refund_query.response.dataContent.tradeNo` | 宝付订单号 | `tradeNo` | 否 | S(32) | 12312312312 | S(32) | 与商户订单号对应的宝付侧唯一订单号，推荐传入此值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `refund_query.response.dataContent.outTradeNo` | 商户订单号 | `outTradeNo` | 否 | S(32) | 20210315155012 | S(32) | 商户系统内部订单号，同一个商户号下唯一 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `refund_query.response.dataContent.refundState` | 订单状态 | `refundState` | 否 | E | REFUND | enum/allowlist | 详见附录订单状态 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `refund_query.response.dataContent.finishTime` | 完成时间 | `finishTime` | 否 | T | 20210315155012 | date/time format | 订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `refund_query.response.dataContent.succAmt` | 成功金额 | `succAmt` | 否 | I | 100 | unit=fen | 单位：分，订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `refund_query.response.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | SUCCESS：成功 FAIL：失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `refund_query.response.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) | SUCCESS | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `refund_query.response.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) | SUCCESS | S(128) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 退款结果通知 (`bct-1f9qm5hspcd9v`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qm5hspcd9v`; updated: `2024-01-05 05:59`.


#### 通知参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `refundNotify.dataContent.agentMerId` | 代理商商户号 | `agentMerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `refundNotify.dataContent.agentTerId` | 代理商终端号 | `agentTerId` | 否 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `refundNotify.dataContent.merId` | 商户号 | `merId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的商户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `refundNotify.dataContent.terId` | 终端号 | `terId` | 是 | S(16) | 100000 | S(16) | 宝付支付分配的终端号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `refundNotify.dataContent.tradeNo` | 宝付退款交易号 | `tradeNo` | 是 | S(32) | 12312312312 | S(32) | 与商户退款订单号对应的宝付侧唯一退款订单号， | Validate required; add missing-field test; Preserve official length/type at boundary |
| `refundNotify.dataContent.outTradeNo` | 退款订单号 | `outTradeNo` | 是 | S(50) | 20210315155012 | S(50) | 商户系统内部退款订单号，同一个商户号下唯一 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `refundNotify.dataContent.refundState` | 订单状态 | `refundState` | 否 | E | REFUND | enum/allowlist | 详见附录订单状态 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `refundNotify.dataContent.finishTime` | 完成时间 | `finishTime` | 否 | T | 20210315155012 | date/time format | 订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `refundNotify.dataContent.succAmt` | 成功金额 | `succAmt` | 否 | I | 100 | unit=fen | 单位：分，订单状态为成功时才有值 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `refundNotify.dataContent.resultCode` | 业务结果 | `resultCode` | 是 | S(16) | SUCCESS | S(16) | SUCCESS：成功 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `refundNotify.dataContent.txnTime` | 交易时间 | `txnTime` | 是 | T | 20210315155012 | date/time format | 订单交易时间 | Validate required; add missing-field test; Validate official time/date format |
| `refundNotify.dataContent.errCode` | 错误代码 | `errCode` | 否 | S(32) | SUCCESS | S(32) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `refundNotify.dataContent.errMsg` | 错误描述 | `errMsg` | 否 | S(128) | SUCCESS | S(128) | 当业务结果FAIL时，返回错误代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

## 5. Payment Attribute And Channel Return Appendices

These tables define nested `payExtend` and `chlRetParam` fields. First-version LocalLife uses `WECHAT_JSAPI`; other pay methods are captured to prevent accidental reuse.


### 支付属性 (`bct-1f9qrefjkin2b`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qrefjkin2b`; updated: `2026-03-23 03:47`.


#### 微信公共参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table1.name` | 姓名 | `name` | 否 | S(32) | 张三 | S(32) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table1.number` | 证件号 | `number` | 否 | S(32) | 330000000000000000 | S(32) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table1.type` | 证件类型 | `type` | 否 | S(32) | IDCARD | S(32) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table1.body` | 商品名称 | `body` | 是 | S(128) |  | S(128) |  | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table1.area_info` | 地区信息 | `area_info` | 是 | S(6) | 510812 | enum/allowlist; S(6) | 商户所在地地区信息，6 位定长，精确到区县编码维度，与国家统计局一致。注：取值范围可参考《省市区结构说明》，详见地址：https://www.mca.gov.cn/n156/n186/index.html | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table1.terminal_info` | 终端信息 | `terminal_info` | 否 | C | json格式 | conditional rule in description | 商户侧受理终端信息<br>支付方式为：WECHAT_MICROPAY，该字段必填<br>支付方式为：ALIPAY_NATIVE/ALIPAY_JSAPI/WECHAT_JSAPI，该字段不能上送<br>详见：微信终端信息：terminal_info | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table1.scene_info` | 场景信息 | `scene_info` | 否 | C | {“store_info”:{ “id”: “SZTX001”,”name”:”腾大餐厅”,”area_code”:”440305”, “address”: “科技园中 一路腾讯大厦”}} |  | JSON格式<br>详见：微信场景信息：scene_info | Optional: omit empty outbound; tolerate absent inbound |

#### 微信终端信息：terminal_info

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table2.location` | 时经纬度信息 | `location` | 否 | s(32) | +37.12/-121.213 |  | 受理终端设备实时经纬度信息，格式为纬度/经度，+表示北纬、东经，-表示南+37.12/-121.213 2纬、西经。 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table2.network_license` | 终端产品入网认证编号 | `network_license` | 否 | S(5) | P3100 | S(5) | 银行卡受理终端产品入网认证编号。该编号由“中国银联标识产品企业资质认证办公室”为通过入网认证的终端进行分配 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.device_type` | 商户端设备类型 | `device_type` | 是 | S(2) |  | enum/allowlist; S(2) | 终端设备类型，受理方可参考终端注册时的设备类型填写，取值详见《终端设备类型枚举》 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table2.serial_num` | 设备的硬件序列号 | `serial_num` | 否 | s(50) | 123456798989 |  | 终端设备的硬件序列号 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table2.device_id` | 终端设备号 | `device_id` | 是 | S(8) | 12345679 | S(8) | 终端设备号，收单机构为商户终端分配的唯一编号。 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.encrypt_rand_num` | 加密随机因子 | `encrypt_rand_num` | 否 | S(10) |  | S(10) | 仅在被扫支付类交易报文中出现：若付款码为 19 位数字，则取后 6 位；若付款码码为 EMV 二维码，则取其 tag 57 的卡号/token 号的后 6 位 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.secret_text` | 密文数据 | `secret_text` | 否 | S(16) |  | S(16) | 仅在被扫支付类交易报文中出现：64bit的密文数据，对终端硬件序列号和加密随机因子加密后的结果。本子域取值为：64bit密文数据进行base64编码后的结果。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table2.app_version` | 应用程序的版本号 | `app_version` | 否 | S(8) |  | S(8) | 应用程序变更应保证版本号不重复。当长度不足时，右补空格。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.device_ip` | 商户端终端设备 IP 地址 | `device_ip` | 否 | S(40) |  | conditional rule in description; S(40) | 商户端终端设备 IP 地址。注：如经、维度信息未上送，该字段必送。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.mobile_country_cd` | 移动国家代码 | `mobile_country_cd` | 否 | S(3) | 460 | S(3) | 基站信息，移动国家代码，由国际电联(ITU) 统 一 分 配 的 移 动 国 家 代 码（MCC），中国为 460 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.mobile_net_num` | 移动网络号码 | `mobile_net_num` | 否 | S(2) | 01 | S(2) | 基站信息，移动网络号码，由国际电联(ITU) 统 一 分 配 的 移 动 网 络 号 码（MNC）移动：00、02、04、07；联通：01、06、09；电信：03、05、11 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.icc_id` | ICCID | `icc_id` | 否 | S(20) |  | S(20) | ICCID，SIM 卡卡号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.location_cd1` | 位置区域码1 | `location_cd1` | 否 | S(4) |  | S(4) | LAC(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.lbs_num1` | 基站编号1 | `lbs_num1` | 否 | S(12) |  | S(12) | LAC(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.lbs_signal1` | 基站信号1 | `lbs_signal1` | 否 | S(4) |  | S(4) | SIG(移动、联通)，16 进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.location_cd2` | 位置区域码2 | `location_cd2` | 否 | S(4) |  | S(4) | 位置区域码2LAC(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.lbs_num2` | 基站编号2 | `lbs_num2` | 否 | S(12) |  | S(12) | CID(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.bs_signal2` | 基站信号2 | `bs_signal2` | 否 | S(4) |  | S(4) | SIG(移动、联通)，16 进 制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.location_cd3` | 位置区域码3 | `location_cd3` | 否 | S(4) |  | S(4) | LAC(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.lbs_num3` | 基站编号3 | `lbs_num3` | 否 | S(12) |  | S(12) | CID(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.lbs_signal3` | 基站信号3 | `lbs_signal3` | 否 | S(4) |  | S(4) | SIG(移动、联通)，16 进 制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.telecom_sys_id` | 电信系统识别码 | `telecom_sys_id` | 否 | S(4) |  | S(4) | SID（电信），电信系统识别码,每个地级市只有一个 SID | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.telecom_net_id` | 电信网络识别码 | `telecom_net_id` | 否 | S(4) |  | S(4) | NID（电信），电信网络识别码,由电信各由地级分公司分配。每个地级市可能有 1 到 3 个 NID | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.telecom_lbs` | 基站信号3 | `telecom_lbs` | 否 | S(4) |  | S(4) | BID（电信），电信网络中的小区识别码，等效于基站 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table2.telecom_lbs_signal` | 电信基站信号 | `telecom_lbs_signal` | 否 | S(4) |  | S(4) | SIG(移动、联通)，16 进 制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 微信场景信息：scene_info

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table3.id` | 门店 编号 | `id` | 是 | S(32) |  | S(32) | 门店唯一标识 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table3.name` | 门店名称 | `name` | 否 | S(256) |  | S(256) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table3.area_code` | 门店行政区划码 | `area_code` | 否 | S(32) |  | S(32) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table3.address` | 门店详细地址 | `address` | 否 | S(512) |  | S(512) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 微信JSAPI：WECHAT_JSAPI

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table4.sub_appid` | 应用号 | `sub_appid` | 是 | S(128) | 1231231231 | S(128) | 小程序或公众号的appid | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table4.sub_openid` | 用户标识 | `sub_openid` | 是 | S(128) | 1231231123 | S(128) | 用户在公众号sub_appid下的唯一标识 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 微信JSAPI：WECHAT_APP

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table5.sub_appid` | 应用号 | `sub_appid` | 是 | S(128) | 1231231231 | S(128) | 商户APP在微信平台的appid | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 微信扫码：WECHAT_MICROPAY

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table6.auth_code` | 付款码 | `auth_code` | 是 | S(128) | 1231231231 | S(128) | 微信支付付款码 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 支付宝公共参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table7.subject` | 商品名称 | `subject` | 是 | S(128) | 1231231231 | S(128) | 商品标题 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table7.area_info` | 地区信息 | `area_info` | 否 | S(6) | 510812 | enum/allowlist; S(6) | 商户所在地地区信息，6位定长，精确到区县编码维度，与国家统计局一致。注：取值范围可参考《省市区结构说明》，详见地址：https://www.mca.gov.cn/n156/n186/index.html | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table7.terminal_info` | 终端信息 | `terminal_info` | 否 | C | json格式 | conditional rule in description | 商户侧受理终端信息，支付方式为：ALIPAY_MICROPAY，该字段必填<br>详见：支付宝终端信息：terminal_info | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table7.store_id` | 商户门店编号 | `store_id` | 否 |  | NJ_001 |  |  | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table7.hb_fq_num` | 分期数 | `hb_fq_num` | 否 | S(5) | 12 | S(5) | 使用花呗分期比传 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table7.hb_fq_seller_percent` | 手续费比例 | `hb_fq_seller_percent` | 否 | I | 99 | conditional rule in description | 使用花呗分期需要卖家承担的 手续费 比例的百分值，间联仅支持送入0；使用花呗分期必传 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table7.enable_pay_channels` | 可用渠道 | `enable_pay_channels` | 否 | S(64) | pcredit,moneyFund,debitCardExpress | enum/allowlist; S(64) | 可用渠道,用户只能在指定渠道范围内支付，多个渠道以逗号分割 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table7.disable_pay_channels` | 禁用渠道 | `disable_pay_channels` | 否 | S(64) | pcredit,moneyFund,debitCardExpress | S(64) | 禁用渠道,用户不可用指定渠道支付，多个渠道以逗号分割 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table7.ext_user_info` | 外部买家信息 | `ext_user_info` | 否 | C |  | conditional rule in description | 指定外部买家信息，基于上送信息进行校验，不上送则不校验。<br>详见：支付宝外部买家信息：ext_user_info | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table7.goods_detail` | 订单详情信息 | `goods_detail` | 否 | C |  |  | 订单包含的商品列表信息。<br>详见：支付宝订单详情信息:goods_detail | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table7.business_params` | 业务信息 | `business_params` | 否 | C | {“charge_type”:”ebike_charging”} |  | 商户传入由支付宝约定的业务信息，以用于安全，营销等参数直传场景；格式为json（最外层无双引号） | Optional: omit empty outbound; tolerate absent inbound |

#### 支付宝终端信息：terminal_info

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table8.location` | 时经纬度信息 | `location` | 否 | S(32) | +37.12/-121.213 | S(32) | 受理终端设备实时经纬度信息，格式为纬度/经度，+表示北纬、东经，-表示南+37.12/-121.213 2纬、西经。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.network_license` | 终端产品入网认证编号 | `network_license` | 否 | S(5) | P3100 | S(5) | 银行卡受理终端产品入网认证编号。该编号由“中国银联标识产品企业资质认证办公室”为通过入网认证的终端进行分配 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.terminal_type` | 终端设备类型 | `terminal_type` | 是 | S(2) |  | enum/allowlist; S(2) | 终端设备类型，受理方可参考终端注册时的设备类型填写，取值详见《终端设备类型枚举》 | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table8.serial_num` | 终端设备的硬件序列号 | `serial_num` | 否 | S(50) | 123456798989 | S(50) | 终端设备的硬件序列号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.terminal_id` | 终端设备编号 | `terminal_id` | 是 | S(8) | 89048765 | S(8) | 终端设备的硬件序列号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.encrypt_rand_num` | 加密随机数 | `encrypt_rand_num` | 否 | S(10) |  | S(10) | 仅在被扫支付类交易报文中出现：若付款码为 19 位数字，则取后 6 位；若付款码码为 EMV 二维码，则取其 tag 57 的卡号/token 号的后 6 位 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.secret_text` | 密文数据 | `secret_text` | 否 | S(16) |  | S(16) | 仅在被扫支付类交易报文中出现：64bit的密文数据，对终端硬件序列号和加密随机因子加密后的结果。本子域取值为：64bit 密文数据进行base64 编码后的结果。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table8.app_version` | 终端应用程序的版本号 | `app_version` | 否 | S(8) |  | S(8) | 应用程序变更应保证版本号不重复。当长度不足时，右补空格。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.terminal_ip` | 商户端终端设备 IP 地址 | `terminal_ip` | 否 | S(64) |  | conditional rule in description; S(64) | 商户端终端设备 IP 地址。注：如经、维度信息未上送，该字段必送。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.mobile_country_cd` | 基站国家信息 | `mobile_country_cd` | 否 | S(3) | 460 | S(3) | 基站信息，移动国家代码，由国际电联(ITU) 统一 分 配 的 移 动 国 家 代 码（MCC），中国为 460 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.mobile_net_num` | 基站信息 | `mobile_net_num` | 否 | S(2) | 01 | S(2) | 基站信息，移动网络号码，由国际电联(ITU) 统 一 分 配 的 移 动 网 络 号 码（MNC）移动：00、02、04、07；联通：01、06、09；电信：03、05、11 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.icc_id` | ICCID | `icc_id` | 否 | S(20) |  | S(20) | ICCID，SIM 卡卡号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.location_cd1` | 位置区域码1 | `location_cd1` | 否 | S(4) |  | S(4) | LAC(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.lbs_num1` | 基站编号1 | `lbs_num1` | 否 | S(12) |  | S(12) | CID(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.lbs_signal1` | 基站信号1 | `lbs_signal1` | 否 | S(4) |  | S(4) | CID(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.location_cd2` | 位置区域码2 | `location_cd2` | 否 | S(4) |  | S(4) | 位置区域码2 LAC(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.lbs_num2` | 基站编号2 | `lbs_num2` | 否 | S(12) |  | S(12) | CID(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.lbs_signal2` | 基站信号2 | `lbs_signal2` | 否 | S(4) |  | S(4) | SIG(移动、联通)，16 进 制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.location_cd3` | 位置区域码3 | `location_cd3` | 否 | S(4) |  | S(4) | LAC(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.lbs_num3` | 基站编号3 | `lbs_num3` | 否 | S(12) |  | S(12) | CID(移动、联通)，16进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.lbs_signal3` | 基站信号3 | `lbs_signal3` | 否 | S(4) |  | S(4) | SIG(移动、联通)，16 进 制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.telecom_sys_id` | 电信系统识别码 | `telecom_sys_id` | 否 | S(4) |  | S(4) | SID（电信），电信系统识别码,每个地级市只有一个 SID | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.telecom_net_id` | 电信网络识别码 | `telecom_net_id` | 否 | S(4) |  | S(4) | NID（电信），电信网络识别码,由电信各由地级分公司分配。每个地级市可能有 1 到 3 个 NID | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.telecom_lbs` | 电信基站 | `telecom_lbs` | 否 | S(4) |  | S(4) | BID（电信），电信网络中的小区识别码，等效于基站 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table8.telecom_lbs_signal` | 电信基站信号 | `telecom_lbs_signal` | 否 | S(4) |  | S(4) | SIG（电信），16 进制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 支付宝外部买家信息：ext_user_info

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table9.name` | 姓名 | `name` | 否 | S(16) |  | S(16) | need_check_info=T时生效 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table9.mobile` | 手机号码 | `mobile` | 否 | S(20) |  | S(20) | 手机号码（该字段暂不参与校验 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table9.cert_type` | 证件类型 | `cert_type` | 否 | S(32) |  | S(32) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table9.cert_no` | 证件号 | `cert_no` | 否 | S(64) |  | S(64) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table9.min_age` | 允许支付的最小买家年龄 | `min_age` | 否 | S(3) |  | S(3) | 买家年龄必须大于等于所传数值need_check_info=T时生效，且年龄为大于0的整数 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table9.need_check_info` | 是否强制校验身份信息 | `need_check_info` | 否 | S(1) | F | S(1) | 是否强制校验身份信息：T：强制校验，F：不强制 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 支付宝订单详情信息:goods_detail

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table10.goods_id` | 商品的编号 | `goods_id` | 是 | S(32) | apple-01 | S(32) | 商品的编号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table10.alipay_goods_id` | 支付宝定义的统一商品编号 | `alipay_goods_id` | 否 | S(32) | 20010001 | S(32) | 支付宝定义的统一商品编号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table10.goods_name` | 商品名称 | `goods_name` | 是 | S(256) | ipad | S(256) | 商品名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table10.quantity` | 商品数量 | `quantity` | 是 | I | 2 |  | 商品数量 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table10.price` | 商品单价 | `price` | 是 | I | 2000 |  | 商品单价，单位为元 | Validate required; add missing-field test; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table10.goods_category` | 商品类目 | `goods_category` | 否 | S(24) | 34543238 | S(24) | 商品类目 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table10.categories_tree` | 商品类目树 | `categories_tree` | 否 | S(128) | 124868003\|126232002\|126252004 | S(128) | 商品类目树,从商品类目根节点到叶子节点的类目id 组成，类目id值使用\|分割 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table10.body` | 商品描述信息 | `body` | 否 | S(1000) | 特价手机 | S(1000) | 商品描述信息 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table10.show_url` | 商品的展示地址 | `show_url` | 否 | S(400) | http://www.alipay.com/xoxxjpg | S(400) | 商品的展示地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 支付宝生活号：ALIPAY_JSAPI

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table11.buyer_id` | 用户标识 | `buyer_id` | buyer_id | S(128) | 123123123 | S(128) | 通过生活号授权获取 | Preserve official length/type at boundary |

#### 支付宝扫码：ALIPAY_MICROPAY

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table12.auth_code` | 付款码 | `auth_code` | 是 | S(128) | 123123123 | S(128) | 支付宝付款码 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 云闪付主扫：QUICK_PASS_NATIVE

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table13.areaInfo` | 区域信息 | `areaInfo` | 是 | S(7) | 1560001 | fixed value; S(7) | 固定7位 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 云闪付主扫js：QUICK_PASS_NATIVE_JS

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table14.userId` | 用户标识 | `userId` | 是 | S(128) | 123123123 | S(128) |  | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table14.areaInfo` | 区域信息 | `areaInfo` | 是 | S(7) | 1560001 | fixed value; S(7) | 固定7位 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table14.customerIp` | 付款人IP | `customerIp` | 是 | S(16) | 127.0.0.1 | S(16) | 付款人IP | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table14.orderDesc` | 订单描述 | `orderDesc` | 是 | S(64) |  | S(64) | 描述性文字，用于向付款人展示。 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 云闪付APP：QUICK_PASS_APP

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table15.subMerId` | 二级商户代码 | `subMerId` | 否 | S(30) | 123123123 | conditional rule in description; S(30) | 商户类型为平台类商户接入时必须上送 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table15.subMerName` | 二级商户全称 | `subMerName` | 否 | S(40) | 上海XX公司 | conditional rule in description; S(40) | 商户类型为平台类商户接入时必须上送 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table15.subMerAbbr` | 二级商户简称 | `subMerAbbr` | 否 | S(16) | 上海XX | conditional rule in description; S(16) | 商户类型为平台类商户接入时必须上送 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table15.reserved` | 保留域 | `reserved` | 否 |  |  |  |  | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table15.ebankEnAbbr` | - 统一收银台直通模式的银行标识 | `ebankEnAbbr` | 否 | S(2) | BCCB | conditional rule in description; S(2) | 该值为统一收银台直通模式业务下，直通银行的英文简称。如果商该值为统一收银台直通模式业务下，直通银行的英文简称。如果商 户 选 择 直 接 拉 起 特 定 银 行APP，必须上送此要素。<br>支持银行详情请查询云闪付APP线上收银台直通模式说明 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table15.ebankType` | - 个人网关类别 | `ebankType` | 否 | S(2) | 01 | conditional rule in description; S(2) | 01：APP+H5 02：APP 03：H5 如果商户未上送取值，默认取值01 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |

#### 云闪付APPLET（云闪付微信小程序支付）：

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table16.merWxMpAppId` | 商户小程序id | `merWxMpAppId` | 是 | S(32) | 123123123 | S(32) | 商户小程序/公众号拉起云闪付小程序时必送，用于云闪付小程序校验来源信息 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table16.invokeScene` | 交易发起场景 | `invokeScene` | 是 | S(2) | 03 | S(2) | 03：小程序 04：公众号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table16.subMerId` | 二级商户编号 | `subMerId` | 否 | S(32) | 示例值CP690000000000004278 | S(32) | 云闪付交易回单需展示二级商户全称时传值 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 聚合码支付、H5支付

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table17.goods_name` | 商品名称 | `goods_name` | 否 | S(256) | 123123123 | S(256) | 商品标题 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table17.merchant_name` | 商户名称 | `merchant_name` | 是 | S(64) | 商户简称测试 | S(64) | 商户名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table17.area_info` | 地区信息 | `area_info` | 是 | S(7) | 1560210 | enum/allowlist; S(7) | 商户所在地地区信息，6 位定长，精确到区县编码维度，与国家统计局一致。注：取值范围可参考《省市区结构说明》，详见地址：https://www.mca.gov.cn/n156/n186/index.html | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table17.terminal_info` | 终端信息 | `terminal_info` | 否 | C | json格式 | conditional rule in description | 商户侧受理终端信息<br>支付方式为：WECHAT_MICROPAY，该字段必填<br>支付方式为：ALIPAY_NATIVE/ALIPAY_JSAPI/WECHAT_JSAPI，该字段不能上送，详见微信、支付宝公共参数字段 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table17.alipay_sub_member_id` | 支付宝smid | `alipay_sub_member_id` | 否 | S(64) | 123456 | conditional rule in description; S(64) | 云闪付支付：该字段可以不填<br>支付宝支付：该字段必填<br>未指定支付方式：支付宝smid、微信smid 两者任选一项必填 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table17.wechat_sub_member_id` | 微信smid | `wechat_sub_member_id` | 否 | S(64) | 123456 | conditional rule in description; S(64) | 云闪付支付：该字段可以不填<br>微信支付：该字段必填<br>未指定支付方式：支付宝smid、微信smid 两者任选一项必填 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 云闪付无感支付公共参数

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table18.subMerId` | 二级商户代码 | `subMerId` | 否 | S(15) | SUB7897979879 | S(15) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table18.subMerName` | 二级商户全称 | `subMerName` | 否 | S(40) | 白云科技有限公司 | S(40) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table18.subMerAbbr` | 二级商户简称 | `subMerAbbr` | 否 | S(16) | 白云科技 | S(16) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 云闪付无感支付：签约支付

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table19.planId` | 签约模板 ID | `planId` | 是 | S(32) | 123123123 | S(32) | 签约模板 ID，与接入产品对应 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table19.contractId` | 签约协议号 | `contractId` | 是 | S(32) | 123123123 | S(32) | 云闪付侧的签约协议号，由银联生成，前序签约接口返回的 token | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 云闪付无感支付：支付并签约

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table20.orderDesc` | 订单描述 | `orderDesc` | 否 | S(32) | 描述订单信息 | S(32) | 描述订单信息，显示在银联支付控件或客户端支付界面中 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table20.acqAddnData` | 订单详细信息（支持单品） | `acqAddnData` | 否 | C | {“goodsInfo”:[{“id”:”1234567890”,”name”:”商品1”,”price”:”500”,”quantity”:”1”},{“id”:”1234567891”,”name”:”商品2”,”price”:”1000”,”quantity”:”2”,”category”:”类目1”,”addnInfo”: “商品图片http://www.95516.com/xxx.jpg"}]} |  | 订单详情信息，内部可包含多个子域 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table20.riskRateInfo` | 风险信息域 | `riskRateInfo` | 否 | C | {“riskRateInfo”:”000”,”shippingCountryCode”:”商品1”,”shippingProvinceCode”:”86”,”shippingCityCode”:”021”,”shippingDistrictCode”:”021”,”shippingStreet”:”陆家嘴”,”commodityName”:”商品名称”} |  | JSON格式<br>详见：云闪付无感支付风控信息域：riskRateInfo | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table20.contractScene` | 签约场景 | `contractScene` | 是 | E | 01 | enum/allowlist | 01：支付并承诺签约<br>02：支付并可选签约 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table20.invokeScene` | 交易发起场景 | `invokeScene` | 是 | E | 03 | enum/allowlist; conditional rule in description | 1、01-APP，02-H5<br>2、支付方式为：QUICK_PASS_H5_PAY_SIGN，上送：02<br>3、支付方式为：QUICK_PASS_APP_PAY_SIGN，上送：01 | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table20.planId` | 签约模板 ID | `planId` | 是 | S(32) | 1234323JKHDFE1243252 | S(32) | 签约模板 ID，与接入产品对应 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table20.backUrl` | 后台通知地址 | `backUrl` | 是 | S(256) | https://www.example.com/return_url | conditional rule in description; S(256) | 后台返回商户结果时使用，如上送，则发送商户后台签约结果通知<br>如果不上送，则默认【服务端通知地址：notifyUrl】 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 云闪付无感支付风控信息域：riskRateInfo

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table21.shippingFlag` | 商品风险类别标识 | `shippingFlag` | 否 | S(3) | 111 | S(3) | 111：虚拟高风险类（无物流、非实名登记、易变现如：游戏点卡、游戏装备、手机充值、礼品卡、虚拟账户充值）<br>110：虚拟低风险类（无物流、非实名登记、不易变现如：电影票、信息咨询）100：虚拟实名类（无物流、实名登记、不易变现如：航空售票、酒店预订、旅游产品、学费、行政费用（税费、车船使用费）、汽车、房产）<br>001：实物高风险类（有物流、易变现如：数码家电、黄金、珠宝首饰等）<br>000：实物低风险类（有物流、不易变现如：服饰、食品、日用品等） | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.shippingCountryCode` | 收货地址-国家 | `shippingCountryCode` | 否 | S(3) |  | S(3) | 国家代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.shippingProvinceCode` | 收货地址-省 | `shippingProvinceCode` | 否 | S(6) |  | S(6) | 省份代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.shippingCityCode` | 收货地址-市 | `shippingCityCode` | 否 | S(6) |  | S(6) | 市区代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.shippingDistrictCode` | 收货地址-地区 | `shippingDistrictCode` | 否 | S(6) |  | S(6) | 地区代码 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.shippingStreet` | 收货地址-详细 | `shippingStreet` | 否 | S(256) |  | S(256) | 详情地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.commodityCategory` | 商品种类 | `commodityCategory` | 否 | S(4) |  | S(4) | 区分商品类别 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.commodityName` | 商品名称 | `commodityName` | 否 | S(256) |  | S(256) | 商品名称 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.commodityUrl` | 商品URL | `commodityUrl` | 否 | S(1024) | http://test.com | S(1024) | 商品URL | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.commodityUnitPrice` | 商品单价 | `commodityUnitPrice` | 否 | I | 20 |  | 商户单价 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table21.commodityQty` | 商品数量 | `commodityQty` | 否 | I | 10 |  | 商品数量 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table21.shippingMobile` | 收货/订单手机号 | `shippingMobile` | 否 | S(20) | 18895263214 | S(20) | 收货人手机号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.addressModifyTim` | 订单地址最后修改时间 | `addressModifyTim` | 否 | T | 20230315155012 | date/time format |  | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `bct-1f9qrefjkin2b.table21.userRegisterTime` | 用户注册时间 | `userRegisterTime` | 否 | T | 20230315155012 | date/time format |  | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `bct-1f9qrefjkin2b.table21.orderNameModifyTime` | 收货（订单）姓名的最后修改时间 | `orderNameModifyTime` | 否 | T | 20230315155012 | date/time format |  | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `bct-1f9qrefjkin2b.table21.userId` | 账户ID | `userId` | 否 | S(128) | user123648999 | S(128) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.orderName` | 收货/订单姓名 | `orderName` | 否 | S(32) | 欧阳 | S(32) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.userFlag` | 优质用户标识码 | `userFlag` | 否 | E | 0 | enum/allowlist | 0普通用户，1-优质用户 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table21.mobileModifyTime` | 订单手机号最后修改时间 | `mobileModifyTime` | 否 | T | 20230315155012 | date/time format |  | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `bct-1f9qrefjkin2b.table21.riskLevel` | 风险级别 | `riskLevel` | 否 | E | 0 | enum/allowlist | 基于绑定关系的支付交易时使用：0：无风险业务1：有风险业务 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table21.merUserId` | 商户端用户ID | `merUserId` | 否 | S(64) | user12364477 | S(64) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.merUserRegDt` | 商户端用户注册时间 | `merUserRegDt` | 否 | T | 20230315 | date/time format |  | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `bct-1f9qrefjkin2b.table21.merUserEmail` | 商户端用户注册邮箱 | `merUserEmail` | 否 | S(256) | test_email@qq.com | S(256) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.diskSep` | 硬盘序列号 | `diskSep` | 否 | S(64) | 7979879899879 | S(64) | 1.持卡人支付时的存储设备的硬盘序列号<br>2.终端硬件序列号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.imei` | IMEI | `imei` | 否 | S(64) | 798789798797 | S(64) | 持卡人支付时手机设备的IMEI | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.macAddr` | MAC地址 | `macAddr` | 否 | S(17) | 00-02-0F-ED-01-03 | S(17) | 持卡人支付时使用设备的MAC地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.lbs` | LBS信息 | `lbs` | 否 | S(32) | +37.12/-121.23 | conditional rule in description; S(32) | 1.空中发卡时的位置信息，经纬度，格式为纬度/经度，+表示北纬、东经，-表示南纬、西经。举例：+37.12/-121.23或者+37/-121<br>2.sourceIP、lbs、fullDeviceNumber这三要素建议至少上送一个 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.deviceNumber` | 设备通讯号码 | `deviceNumber` | 否 | S(32) |  | S(32) | 1.终端拨号号码<br>2.单个手机号,可能包含前缀（发起交易的手机号码，不是接收验证码的手机号） | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.deviceType` | 设备类型 | `deviceType` | 否 | E | 1 | enum/allowlist | 设备类型：1.Phone<br>2. Pad<br>3. iWatch<br>4.PC | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table21.captureMethod` | 卡片信息录入方式 | `captureMethod` | 否 | E | 1 | enum/allowlist | 卡号录入方式，例如：1.camera：表示摄像头捕捉得到卡号<br>2.manual：用户手输入卡号<br>3.nfc：nfc方式读取卡号<br>4.unknow：未知的获取卡号方式。经手工修改卡号后，均应填写为manual，表示手工输入。 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table21.simCardCount` | 设备sim卡数量 | `simCardCount` | 否 | I | 100 |  |  | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table21.deviceName` | 设备名称 | `deviceName` | 否 | S(128) | POS机 | S(128) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.deviceID` | 设备标识 | `deviceID` | 否 | S(64) |  | S(64) | 移动终端设备的唯一标识 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.mobile` | 银行预留手机号 | `mobile` | 否 | S(20) | 1859638795 | S(20) | 银行卡预留手机号码仅1个，不包括+86等信息 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.accountIdHash` | 应用提供方账户ID | `accountIdHash` | 否 | S(64) |  | S(64) | 用来标识用户在智能设备上登录账号ID信息的哈希值，与用户登录账号ID是一一对应关系，为登录账号ID的替换值。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.fullDeviceNumber` | 设备SIM卡号码 | `fullDeviceNumber` | 否 | S(32) |  | conditional rule in description; S(32) | 1.持卡人用来做设备卡加载时所使用设备的号码，多个号码用逗号隔开。<br>2.sourceIP、lbs、fullDeviceNumber这三要素建议至少上送一个 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.sourceIP` | IP | `sourceIP` | 否 | S(64) | 192.168.1.1 | conditional rule in description; S(64) | 1.必送（IP、设备GPS位置、设备SIM卡号码，这三要素至少上送一个）<br>2.sourceIP、lbs、fullDeviceNumber这三要素建议至少上送一个 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.deviceLanguage` | 设备使用语言 | `deviceLanguage` | 否 | S(3) |  | S(3) | 移动支付设备所设定的使用语言，语言代码取值遵从ISO639-3标准。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table21.accountEmailLife` | 账户关键信息修改时间 | `accountEmailLife` | 否 | I | 12 |  | 设备用户重要信息修改时间，最近一次修改email距今X个月，X的数值范围：0-24，表示0-24个月，大于24个月赋值24。 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer; Validate official time/date format |
| `bct-1f9qrefjkin2b.table21.cardHolderName` | 持卡人姓名 | `cardHolderName` | 否 | S(256) | 三李 | S(256) | 持卡人姓名，名在前，姓在后。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.billingAddress` | 持卡人账单地址 | `billingAddress` | 否 | S(256) |  | S(256) | 用户账单地址信息。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.billingZip` | 持卡人邮编 | `billingZip` | 否 | S(6) |  | S(6) | 用户账单邮编信息。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.riskScore` | 总体风险评级 | `riskScore` | 否 | I | 5 |  | 风险评级, 1-5分，5分最高。 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table21.riskStandardVersion` | 风险评级版本号 | `riskStandardVersion` | 否 | S(8) |  | S(8) | 设备厂商给出加载流程风险建议时所基于的风险判断原则对应的版本。 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.deviceScore` | 设备评级 | `deviceScore` | 否 | I | 3 |  | 设备厂商给设备的评分，1-5分，5分可信度越高。 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table21.accountScore` | 账户评级 | `accountScore` | 否 | I | 9 |  | 设备厂商给用户账户的评分，取值从0到9。 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table21.phoneNumberScore` | 设备SIM卡号码评级 | `phoneNumberScore` | 否 | I | 3 |  | 设备号码评分，加载流程对应手机号信任评级级别，取值从1到5。 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table21.riskReasonCode` | 评级原因码 | `riskReasonCode` | 否 | S(100) | 否 | S(100) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.applyChannel` | 绑卡渠道 | `applyChannel` | 否 | E | 01 | enum/allowlist | 01:银行自有渠道<br>02:非银行渠道 | Optional: omit empty outbound; tolerate absent inbound; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table21.thirdPartyRiskScore` | 第三方风险总体评分 | `thirdPartyRiskScore` | 否 | I | 1025 |  | 第三方提供的细化风险总体评分，最多可支持5位数字 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table21.thirdPartyDeviceId` | 第三方设备标识 | `thirdPartyDeviceId` | 否 | S(100) |  | S(100) | 第三方提供的设备标识 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.thirdPartyAdvise` | 第三方建议 | `thirdPartyAdvise` | 否 | S(5) |  | S(5) | 第三方针对当前交易风险情况提供的处置建议 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.deviceMode` | 设备型号 | `deviceMode` | 否 | S(256) |  | S(256) | 设备型号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.safeCarrIss` | 安全载体发行方 | `safeCarrIss` | 否 | S(16) | 第三方 | S(16) | 安全载体发行方 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.seId` | 设备指纹ID | `seId` | 否 | S(64) |  | S(64) |  | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table21.csn` | CSN | `csn` | 否 | S(64) |  | S(64) | 移动支付部交易专用，由移动支付部为SD卡，SIM卡，读卡器统一分配的唯一ID号，用于识别卡片载体 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 云闪付无感支付风控信息域：riskRateInfo

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table22.frontUrl` | 前台通知地址 | `frontUrl` | 是 | S(256) | https://www.example.com/return_url | conditional rule in description; S(256) | 前台返回商户结果时使用，前台类交易需上送 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table22.frontFailUrl` | 失败交易前台跳转地址 | `frontFailUrl` | 是 | S(256) | https://www.example.com/return_url | conditional rule in description; S(256) | 前台交易若商户上送此字段，则在交易失败时，页面跳转至商户该URL（不带交易信息，仅跳转）前台类交易需上送<br>如果不上送，则默认【页面端跳转地址：pageUrl】 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 云闪付聚分期支付

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table23.accNo` | 卡号 | `accNo` | 否 | S(256) | 732CCEE5C3A4040C4AB5350AEDAC1FA0090565122D503872 | conditional rule in description; S(256) | 敏感信息，加密上送商户限定卡号支付卡号或卡号掩码仅需上送一个，若限定了卡号信息该笔订单无法更换卡号支付 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table23.maskedCardNo` | 卡号掩码 | `maskedCardNo` | 否 | S(19) | 678464* * * * * *678 | conditional rule in description; S(19) | 商户限定卡号掩码支付，需同时上送用户手机号码，仅在联合登陆场景下使用。卡号与卡号掩码仅需上送一个（按照聚分期返回的掩码原样上送） | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table23.suppBankName` | 限定银行 | `suppBankName` | 否 | S(1024) | [“ABC”,”ACBC”,”JCB”] | enum/allowlist; conditional rule in description; S(1024) | 商户指定银行分期支付，则填上该 值，用array格式上送。多个银行代码例如[“ABC”,”ACBC”,”JCB”]银行代码简称详见附录B若上送了卡号或卡号掩码无需上送改字段。若上送需与卡号对应银行保持一致 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrefjkin2b.table23.limitNum` | 限定分期期数 | `limitNum` | 否 | S(20) | [“3”,”6”,”9”] | conditional rule in description; S(20) | 商户指定分期期数，用array格式上送。支持上送多个期数 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table23.customerInfo` | 身份信息 | `customerInfo` | 否 | C | {“certifId”:”310536199006093459”,”certifTp”:”01”,”customerNm”:”姓名”} | conditional rule in description | 身 份 信 息 如{“certifTp”:“01”,“certified”:“value”,“customerNm”:“value”}，需对customerInfo整体加密，如果选择上送customerInfo域，则customerInfo里面的字段为必填 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table23.phoneNo` | 手机号 | `phoneNo` | 否 | S(1024) | AA77319E10CA007034AB84B60FA79796 | conditional rule in description; S(1024) | 联合登陆场景下上送用户手机号 手机号需加密上送 （白名单商户才能 支持联登），需加密 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table23.store` | 门店标志 | `store` | 否 | S(100) | AAAAA898989899 | S(100) | 用来标志线下商户的门店信息 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table23.storeName` | 门店名称 | `storeName` | 否 | S(15) | XXX加油站 | conditional rule in description; S(15) | 用于前端展示商户门店名称（需与 store一起上送该字段，不能单独上 送），不能超过15个汉字和字符 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table23.trxChnnel` | 渠道类型 | `trxChnnel` | 是 | S(2) | 02 | S(2) | 02：线上收银台 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table23.subMerId` | 二级商户代码 | `subMerId` | 否 | S(30) | SUB7897979879 | conditional rule in description; S(30) | 接入类型为平台商户接入时需上送 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table23.subMerName` | 二级商户全称 | `subMerName` | 否 | S(40) | 白云科技有限公司 | conditional rule in description; S(40) | 接入类型为平台商户接入时需上送 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table23.subMerAbbr` | 二级商户简称 | `subMerAbbr` | 否 | S(8) | 白云科技 | conditional rule in description; S(8) | 接入类型为平台商户接入时需上送 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 京东白条支付：

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table24.areaInfo` | 区域信息 | `areaInfo` | 是 | S(7) | 1562904 | fixed value; S(7) | 固定7位：“156+四位地区代码”。地区代码采用《银联卡跨行业务地区代码标准》，如上海市浦东新区为2904 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table24.acqAddnData` | 收款方附加信息 | `acqAddnData` | 是 | C |  |  |  | Validate required; add missing-field test |

#### 京东白条支付收款方附加数据：acqAddnData

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table25.orderInfo` | 订单信息 | `orderInfo` | 是 | C |  |  | 订单明细内容，如订单标题，订单描述 | Validate required; add missing-field test |

#### 京东白条支付订单信息：orderInfo

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table26.title` | 订单标题 | `title` | 是 | S(100) | 华为手机 | S(100) | 订单明细内容，如订单标题，订单描述 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrefjkin2b.table26.addnInfo` | 附加信息 | `addnInfo` | 是 | C |  |  | 京东白条支付附件信息 | Validate required; add missing-field test |

#### 京东白条支付附加数据：addnInfo

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table27.installmentNum` | 默认选中期数 | `installmentNum` | 否 | I | 3 |  | 默认选中期数，表示用户使用的白条支付时选择的分期数，进入京东收银台后默认选中的分期数。非预锁分期时使用。 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table27.lockplan` | 预锁分期活动 | `lockplan` | 否 | I | 6 | conditional rule in description | 和“默认选中期数”二选一，传入后用户在京东收银台不可更换分期数 | Optional: omit empty outbound; tolerate absent inbound; Fen integer: no decimal conversion in business layer |
| `bct-1f9qrefjkin2b.table27.a_o_s_c_u` | 支付成功指定跳转地址 | `a_o_s_c_u` | 否 | C |  |  | openapp 前缀链接，会跳转回商户App https 前缀链接，会在京东App打开H5跳转 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table27.riskInfo` | 白条侧策略采集字段信息 | `riskInfo` | 是 | C |  |  |  | Validate required; add missing-field test |

#### 京东白条支付附加数据：riskInfo

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrefjkin2b.table28.tradeScene` | 交易场景 | `tradeScene` | 是 | S | 01 | conditional rule in description | 商户传入，线上场景必填，01（线上）/02（线下） | Validate required; add missing-field test |
| `bct-1f9qrefjkin2b.table28.ifPickup` | 自提标记 | `ifPickup` | 否 | S | 01 |  | 是否为自提订单，01（配送）/02（自提） | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.province` | 收货省 | `province` | 否 | S |  | conditional rule in description | 用于省市聚集校验，线上场景必填 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.city` | 收货市 | `city` | 否 | S |  | conditional rule in description | 同上，线上场景必填 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.orderSource` | 下单来源 | `orderSource` | 是 | S | 京东金融 | conditional rule in description | 传中文：微信APP扫一扫、京东金融、京东、支付宝APP扫一扫，线下or自提必填 | Validate required; add missing-field test |
| `bct-1f9qrefjkin2b.table28.payCodeId` | 商户收款码ID | `payCodeId` | 否 | S | code_123 | conditional rule in description | 静态码/动态码终端设备唯一识别码 ，线下/自提必填 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.orderEid` | 下单设备号 | `orderEid` | 否 | S | device_123 |  | 商户侧唯一设备标识 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.orderIp` | 下单IP | `orderIp` | 否 | S | 116.236.217.150 | conditional rule in description | 用户下单IP地址，线上场景必填 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.orderAccount` | 下单账号 | `orderAccount` | 否 | S | UID12345678 | conditional rule in description | 用户在商户侧的唯一标识，线上场景必填 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.acctRegTime` | 注册时间 | `acctRegTime` | 否 | S | yyyy-mm-dd hh:mm:ss | conditional rule in description | 用户账号注册时间，线上场景必填 | Optional: omit empty outbound; tolerate absent inbound; Validate official time/date format |
| `bct-1f9qrefjkin2b.table28.name` | 收货人姓名 | `name` | 否 | S | 张三 | conditional rule in description | 收货人姓名 ，线上场景必填 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.address` | 收货地址 | `address` | 否 | S | xx省xx市xx街道100号 | conditional rule in description | 收货地址，线上场景必填 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.mobile` | 收货手机号 | `mobile` | 否 | S | 13823451234 | conditional rule in description | 收货人手机号，线上场景必填 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.id` | 商品编号 | `id` | 否 | S | 123456 |  |  | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.goodsName` | 商品名称 | `goodsName` | 是 | S | 手机 | conditional rule in description | 商品名称，线上场景必填 | Validate required; add missing-field test |
| `bct-1f9qrefjkin2b.table28.type` | 商品类型 | `type` | 否 | S | GT01 |  | 实物或虚拟商品标识，GT01：实物、GT02：虚拟 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.price` | 商品单价 | `price` | 否 | S | 5000（单位：分） | conditional rule in description | 单价，以分为单位；线上场景必填 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrefjkin2b.table28.num` | 商品数量 | `num` | 否 | S | 2 | conditional rule in description | 商品购买数量；线上场景必填 | Optional: omit empty outbound; tolerate absent inbound |

### 统一下单渠道返回参数 (`bct-1f9qrf159lni6`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qrf159lni6`; updated: `2025-08-20 09:33`.


#### table 1

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table1.prepay_id` | 预支付交易会话标识 | `prepay_id` | 是 | S(64) |  | S(64) |  | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table1.wc_pay_data` | 微信调用数据 | `wc_pay_data` | 是 | C |  |  | 用于执行JS或者调起APP支付 | Validate required; add missing-field test |
| `bct-1f9qrf159lni6.table1.order_id` | 宝付订单号 | `order_id` | 是 | S(32) | 1188000078909 | S(32) | 宝付订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### table 2

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table2.appId` | 公众号 id/小程序 id | `appId` | 是 | S(16) | wx8888888888888888 | S(16) | 商户注册具有支付权限的公众号成功后即可获得公众号 id；商户注册具有支付权限的小程序成功后即可获得小程序 id | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table2.timeStamp` | 时间戳 | `timeStamp` | 是 | S(32) | 1414561699 | S(32) | 当前的时间 | Validate required; add missing-field test; Preserve official length/type at boundary; Validate official time/date format |
| `bct-1f9qrf159lni6.table2.nonceStr` | 随机字符串 | `nonceStr` | 是 | S(32) | 5K8264ILTKCH16CQ2502SI8ZNMTM67VS | S(32) | 随机字符串，不长于32位。推荐随机数生成算法 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table2.package` | 订单详情扩展字符串 | `package` | 是 | S(128) | prepay_id=123456789 | S(128) | 统一下单接口返回的prepay_id参数值，提交格式如：prepay_id=123456 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table2.signType` | 签名方式 | `signType` | 是 | S(32) | RSA | S(32) | 签名类型，支持 RSA | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table2.paySign` | 签名 | `paySign` | 是 | S(64) | C380BEC2BFD727A4B6845133519F3AD6 | S(64) | 签名 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### table 3

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table3.appId` | 应用id | `appId` | 是 | S(32) | wx8888888888888888 | S(32) | 微 信 开 放 平 台 审 核 通 过 的 应 用APPID，为特约商户申请的应用APPID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table3.prepayId` | 预支付交易会话ID | `prepayId` | 是 | S(32) | WX1217752501201407033233368018 | S(32) | 微信返回的支付交易会话 ID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table3.package` | 订单详情扩展字符串 | `package` | 是 | S(128) | Sign=WXPay | fixed value; S(128) | 固定值 Sign=WXPay | Validate required; add missing-field test; Preserve official length/type at boundary; Use typed constant/allowlist; unknown fail-closed |
| `bct-1f9qrf159lni6.table3.nonceStr` | 随机字符串 | `nonceStr` | 是 | S(32) | 5K8264ILTKCH16CQ2502SI8ZNMTM67VS | S(32) | 随机字符串，不长于32位。推荐随机数生成算法 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table3.timeStamp` | 时间戳 | `timeStamp` | 是 | S(32) | 1414561699 | S(32) | 当前的时间 | Validate required; add missing-field test; Preserve official length/type at boundary; Validate official time/date format |
| `bct-1f9qrf159lni6.table3.paySign` | 签名 | `paySign` | 是 | S(64) | C380BEC2BFD727A4B6845133519F3AD6 | S(64) | 签名 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### table 4

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table4.prepay_req_body_base64` | 预下单请求头部 | `prepay_req_body_base64` | 是 | S(1048576) |  | S(1048576) | 用于应答微信支付分商户预下单通知 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table4.prepay_req_body_base64` | 预下单请求包体 | `prepay_req_body_base64` | 是 | S(1048576) |  | S(1048576) | 用于应答微信支付分商户预下单通知 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table4.prepay_resp_header_base64` | 预下单响应头部 | `prepay_resp_header_base64` | 是 | S(1048576) |  | S(1048576) | 用于应答微信支付分商户预下单通知 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table4.prepay_resp_body_base64` | 预下单响应包体 | `prepay_resp_body_base64` | 是 | S(1048576) |  | S(1048576) | 用于应答微信支付分商户预下单通知 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table4.order_id` | 宝付订单号 | `order_id` | 是 | S(32) | 1188000078909 | S(32) | 宝付订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### table 5

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table5.qr_code` | 支付链接 | `qr_code` | 是 | S |  |  |  | Validate required; add missing-field test |
| `bct-1f9qrf159lni6.table5.order_id` | 宝付订单号 | `order_id` | 是 | S(32) | 1188000078909 | S(32) | 宝付订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### table 6

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table6.trade_no` | 支付订单号 | `trade_no` | 是 | S |  |  |  | Validate required; add missing-field test |
| `bct-1f9qrf159lni6.table6.order_id` | 宝付订单号 | `order_id` | 是 | S(32) | 1188000078909 | S(32) | 宝付订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### table 7

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table7.qrCode` | 支付链接 | `qrCode` | 是 | S |  |  |  | Validate required; add missing-field test |
| `bct-1f9qrf159lni6.table7.order_id` | 宝付订单号 | `order_id` | 是 | S(32) | 1188000078909 | S(32) | 宝付订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### table 8

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table8.redirectUrl` | 支付链接 | `redirectUrl` | 是 | S |  |  |  | Validate required; add missing-field test |
| `bct-1f9qrf159lni6.table8.order_id` | 宝付订单号 | `order_id` | 是 | S(32) | 1188000078909 | S(32) | 宝付订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### table 9

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table9.tn` | 银联受理订单号 | `tn` | 是 | S(21) | 476367050082030801610 | S(21) | 注：调用银联SDK使用此银联受理订单号，非宝付订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table9.order_id` | 宝付订单号 | `order_id` | 是 | S(32) | 1188000078909 | S(32) | 宝付订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### table 10

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table10.payCardType` | 支付卡类型 | `payCardType` | 是 | S(8) | 01 | S(8) | 00-未知，01-借记卡，02-贷记卡 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table10.order_id` | 宝付订单号 | `order_id` | 是 | S(32) | 1188000078909 | S(32) | 宝付订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### table 11

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table11.order_id` | 宝付订单号 | `order_id` | 是 | S(32) | 240825130925525678 | S(32) | 宝付订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table11.qr_no` | 二维码链接 | `qr_no` | 否 | S(2048) | https://fts.unionpay.com/aps/aps1/dynamic/loading.html#loading?id=9033338bcefa5c2a314857e69700e38d4c4940193a3c898e | S(2048) | 商户可用此参数自定义去生成二维码后展示出来进行扫码支付 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table11.qr_image` | 二维码图片 | `qr_image` | 否 | S(2048) | https://fts.unionpay.com/apsmgm/oper/qrcode?token=2A70D50DE32D49C95FCA0075FA45747 | S(2048) | 此参数的值即是根据 qrNo 生成的可以扫码支付的二维码图片地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table11.req_reserved` | 请求方保留域 | `req_reserved` | 否 | S(512) |  | S(512) | 此参数的值即是根据 qrNo 生成的可以扫码支付的二维码图片地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 云闪付APPLET（云闪付微信小程序支付）

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrf159lni6.table12.cqp_mp_appid` | 云闪付小程序id | `cqp_mp_appid` | 是 | S(32) | 用于商户跳转进行支付 | S(32) |  | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table12.cqp_mp_path` | 云闪付小程序path | `cqp_mp_path` | 是 | S(1024) | 1188000078909 | S(1024) | 用于商户跳转进行支付 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table12.tn` | 银联受理订单号 | `tn` | 是 | S(21) | 476367050082030801610 | S(21) |  | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrf159lni6.table12.order_id` | 宝付订单号 | `order_id` | 是 | S(32) | 1188000078909 | S(32) | 宝付订单号 | Validate required; add missing-field test; Preserve official length/type at boundary |

### 异步通知渠道返回参数 (`bct-1f9qrfb580243`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qrfb580243`; updated: `2024-01-17 02:08`.


#### table 1

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrfb580243.table1.buyer_logon_id` | 买家支付宝账号 | `buyer_logon_id` | 是 | S |  |  |  | Validate required; add missing-field test |
| `bct-1f9qrfb580243.table1.fund_bill_list` | 支付金额信息 | `fund_bill_list` | 是 | S |  |  |  | Validate required; add missing-field test |
| `bct-1f9qrfb580243.table1.fund_channel` | - 资金渠道 | `fund_channel` | 是 | S |  |  | 交易使用的资金渠道 | Validate required; add missing-field test |
| `bct-1f9qrfb580243.table1.bank_code` | - 银行代码 | `bank_code` | 否 | S |  |  | 银行卡支付时的银行代码 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrfb580243.table1.amount` | - 支付金额 | `amount` | 是 | S |  |  | 该支付工具类型所使用的金额 | Validate required; add missing-field test |
| `bct-1f9qrfb580243.table1.real_amount` | - 渠道实际付款金额 | `real_amount` | 否 | S |  |  | 渠道实际付款金额 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrfb580243.table1.fund_type` | - 资金类型 | `fund_type` | 否 | S |  |  | 渠道所使用的资金类型,目前只在资金 渠 道 (fundChannel) 是 银 行 卡 渠 道 (BANKCARD)的情况下才返回该信息。DEBIT_CARD:借记卡 CREDIT_CARD:信用卡 MIXED_CARD:借贷合一卡 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrfb580243.table1.buyer_id` | 买家的支付宝唯一用户号 | `buyer_id` | 是 | S |  |  |  | Validate required; add missing-field test |
| `bct-1f9qrfb580243.table1.trade_no` | 支付宝交易号 | `trade_no` | 是 | S |  |  |  | Validate required; add missing-field test |

#### table 2

| 支付渠道代码 | 支付渠道 |
| --- | --- |
| COUPON | 支付宝红包 |
| ALIPAYACCOUNT | 支付宝账户 |
| POINT | 集分宝 |
| DISCOUNT | 折扣券 |
| PCARD | 预付卡 |
| MCARD | 商家储值卡 |
| MDISCOUNT | 商户优惠券 |
| MCOUPON | 商户红包 |
| BANKCARD | 银行卡 |

#### table 3

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrfb580243.table3.bank_type` | 付款银行 | `bank_type` | 是 | S |  |  | 银行类型，采用字符串类型的银行标识 xxx_DEBIT:借记卡(包含DEBIT), xxx_CREDIT:信用卡(包含CREDIT) OTHERS:其他 LQT：微信零钱通 CFT：微信零钱 | Validate required; add missing-field test |
| `bct-1f9qrfb580243.table3.transaction_id` | 微信交易号 | `transaction_id` | 是 | S |  |  |  | Validate required; add missing-field test |
| `bct-1f9qrfb580243.table3.out_order_no` | 支付分服务单号 | `out_order_no` | 否 | S |  |  | 当支付方式为微信支付分时，该字段值不为空，原样返回商户创建支付分服务单号 | Optional: omit empty outbound; tolerate absent inbound |

#### table 4

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrfb580243.table4.voucherNum` | 付款凭证号 | `voucherNum` | 是 | S | 48230223364098087021 |  | 付款凭证号 | Validate required; add missing-field test |
| `bct-1f9qrfb580243.table4.payerInfo` | 付款方信息 | `payerInfo` | 是 | C | {"cardAttr":"01"} |  | 付款方信息 该字段为JSON对象数据 cardAttr 取值01-借记卡 02-贷记卡(含准贷记卡) | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |

#### table 5

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrfb580243.table5.payCardType` | 支付卡类型 | `payCardType` | 是 | S(8) | 01 | S(8) | 00-未知，01-借记卡，02-贷记卡 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrfb580243.table5.reserved` | 保留域 | `reserved` | 否 | S(2048) | eyJkaXNjb3VudEFtdCI6IjIyNyJ9 | S(2048) | 字符串经Base64解码后为{“discountAmt”:”203”} | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrfb580243.table5.discountAmt` | - 总的立减金额 | `discountAmt` | 否 | S(12) | 47 | S(12) | 消费、账单支付的商户通知和交易状态查询返回 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrfb580243.table5.mchtDiscountAmt` | - 商户优惠金额 | `mchtDiscountAmt` | 否 | S(12) | 01 | S(12) | 商户出资金额 消费、账单支付的商户通知和交易状态查询返回 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrfb580243.table5.activityId` | - 活动编号 | `activityId` | 否 | S(40) | 34 | S(40) | 单品营销时返回，票券编号、活动编号等，格式自定义 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrfb580243.table5.activityNm` | - 活动简称 | `activityNm` | 否 | S(60) | XX营销 | S(60) | 单品营销时返回，优惠活动简称，可用于展示、打单等 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrfb580243.table5.addnPrintInfo` | - 活动打印信息 | `addnPrintInfo` | 否 | S(100) |  | S(100) | 单品营销时返回，内容自定义打印信息（营销活动需要将营销信息打印到商户的购物小票中，这个字段通过营销活动的配置进行模板的编辑，通过交易信息的回传传给商户的终端收银台进行打印） | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### table 6

| Path | 中文名 | 变量名 | 必填 | 类型 | 示例值 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1f9qrfb580243.table6.tn` | 银联受理订单号 | `tn` | 否 | S(32) | 476367050082030801610 | S(32) | 支付并签约时返回 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrfb580243.table6.payCardType` | 支付卡类型 | `payCardType` | 是 | S(8) | 01 | S(8) | 00-未知，01-借记卡，02-贷记卡 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1f9qrfb580243.table6.reserved` | 保留域 | `reserved` | 否 | S(2048) | eyJkaXNjb3VudEFtdCI6IjIyNyJ9 | S(2048) | 字符串经过Base64解码后为{“discountAmt”:”203”} | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrfb580243.table6.discountAmt` | - 总的立减金额 | `discountAmt` | 否 | S | 47 |  | 消费、账单支付的商户通知和交易状态查询返回 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrfb580243.table6.mchtDiscountAmt` | - 商户优惠金额 | `mchtDiscountAmt` | 否 | S | 01 |  | 商户出资金额 消费、账单支付的商户通知和交易状态查询返回 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1f9qrfb580243.table6.activityId` | - 活动编号 | `activityId` | 否 | S(40) | 34 | S(40) | 单品营销时返回，票券编号、活动编号等，格式自定义 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrfb580243.table6.activityNm` | - 活动简称 | `activityNm` | 否 | S(60) | XX营销 | S(60) | 单品营销时返回，优惠活动简称，可用于展示、打单等 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1f9qrfb580243.table6.addnPrintInfo` | - 活动打印信息 | `addnPrintInfo` | 否 | S(100) |  | S(100) | 单品营销时返回，内容自定义打印信息（营销活动需要将营销信息打印到商户的购物小票中，这个字段通过营销活动的配置进行模板的编辑，通过交易信息的回传传给商户的终端收银台进行打印） | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

## 6. Enum / Appendix Matrix

### 签名类型 (`bct-1f9qrd02b5bka`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qrd02b5bka`; updated: `2024-01-05 05:15`.


#### table 1

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `SM2` | 国密 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `RSA` | RSA | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

### 产品类型 (`bct-1f9qrdjnaqra5`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qrdjnaqra5`; updated: `2024-01-09 06:37`.


#### table 1

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `SHARING` | 分账产品 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

### 通知类型 (`bct-1f9qrdaere1nb`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qrdaere1nb`; updated: `2024-01-05 05:15`.


#### table 1

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `PAYMENT` | 支付结果 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `SHARING` | 分账结果 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `REFUND` | 退款结果 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `SIGN` | 签约结果 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

### 支付方式 (`bct-1f9qrdro3gtv1`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qrdro3gtv1`; updated: `2026-03-23 03:48`.


#### table 1

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `WECHAT_JSAPI` | 微信JSAPI | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `WECHAT_APP` | 微信APP | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `ALIPAY_NATIVE` | 支付宝扫码支付（主扫） | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `ALIPAY_JSAPI` | 支付宝生活号支付 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `QUICK_PASS_NATIVE` | 云闪付主扫 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `QUICK_PASS_NATIVE_JS` | 云闪付主扫JS | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `QUICK_PASS_APP` | 云闪付APP | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `QUICK_PASS_APPLET` | 云闪付APPLET（云闪付微信小程序支付） | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `QUICK_PASS_APPLET_PAY` | 云闪付无感支付微信小程序签约支付 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `QUICK_PASS_H5_PAY` | 云闪付无感支付H5签约支付（暂不支持） | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `QUICK_PASS_H5_PAY_SIGN` | 云闪付无感支付H5支付并签约 （暂不支持） | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `QUICK_PASS_APP_PAY` | 云闪付无感支付云闪付APP签约支付 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `QUICK_PASS_APP_PAY_SIGN` | 云闪付无感支付云闪付APP支付并签约（暂不支持） | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `QUICK_PASS_INSTALLMENT` | 云闪付聚分期 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `JDPAY_BT` | 京东白条 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### table 2

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `WECHAT_MICROPAY` | 微信付款码支付 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `ALIPAY_MICROPAY` | 支付宝付款码支付 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### table 3

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `WECHAT_JSAPI` | 微信JSAPI | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `ALIPAY_JSAPI` | 支付宝生活号支付 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `QUICK_PASS_NATIVE` | 云闪付主扫 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

### 订单状态 (`bct-1f9qre51sa7dg`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qre51sa7dg`; updated: `2024-03-26 07:09`.


#### table 1

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `SUCCESS` | 交易成功，支付成功的订单再次发起支付依然返回支付成功，商户侧需做幂等处理 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `CLOSED` | 已关闭 通常存在3种情况会关闭订单 1：商户侧发起的订单关闭 2：超出订单有效期还未支付成功的订单，系统自动关闭 3：被风控的订单 已关闭的订单不能再次发起支付 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `WAIT_PAYING` | 下单成功，等待用户支付中 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `PAY_ERROR` | 支付失败，同一笔订单号在有效期内可再次发起支付 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `REFUND` | 支付订单已退款 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `ABNORMAL` | 支付异常，返回此状态的支付订单，请稍后发起查询。 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### table 2

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `SUCCESS` | 退款成功 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `REFUND` | 退款受理成功 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `REFUND_ERROR` | 退款失败 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `ABNORMAL` | 退款异常，返回此状态的退款订单，请稍后发起查询。 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### table 3

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `SUCCESS` | 分账成功 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `PROCESSING` | 分账处理中 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `CANCELED` | 取消分账 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `ABNORMAL` | 分账请求异常 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### table 4

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `SUCCESS` | 成功 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `PROCESSING` | 处理中 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `CLOSED` | 已关闭 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `FAIL` | 失败 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `ABNORMAL` | 返回此状态的订单，请稍后发起查询 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

### 附录 (`bct-1f9o6qi1pf2r8`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9o6qi1pf2r8`; updated: `2024-12-13 06:15`.


#### 签名类型

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `SM2` | 国密 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `RSA` | RSA | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 报备类型

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `WECHAT` | 微信渠道 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `ALIPAY` | 支付宝渠道 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 报备状态

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `PROCESSING` | 处理中 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `SUCCESS` | 成功 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `FAIL` | 失败 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 终端设备类型

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `01` | 自动柜员机（含 ATM 和 CDM）和多媒体自助终端 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `02` | 传统 POS | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `03` | mPOS | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `04` | 智能 POS | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `05` | II 型固定电话 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `06` | 云闪付终端 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `07` | 保留使用 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `08` | 手机 POS | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `09` | 刷脸付终端 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `10` | 条码支付受理终端 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `11` | 条码支付辅助受理终端 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `12` | 行业终端（公交、地铁用于指定行业的终端） | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `13` | MIS终端 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 操作标识

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `00` | 新增 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `01` | 修改 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `02` | 注销 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 设备状态

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `00` | 启用 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `01` | 注销 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 微信服务类型

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `JSAPI` | JSAPI支付 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `APPLET` | 小程序支付 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `MICROPAY` | 付款码支付 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 支付宝服务类型

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `F2F` | 支付宝支付 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 联系人业务标识

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `02` | 异议处理接口人 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `06` | 商户关键联系人 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `08` | 服务联动接口人 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `11` | 数据反馈接口人 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 微信证件类型

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `NATIONAL_LEGAL` | 营业执照 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `NATIONAL_LEGAL_MERGE` | 营业执照(多证合一) | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `INST_RGST_CTF` | 事业单位法人证书 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `IDENTITY_CARD` | 个人身份证 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `OTHERS` | 其他 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 支付宝证件类型

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `NATIONAL_LEGAL` | 营业执照 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `NATIONAL_LEGAL_MERGE` | 营业执照(多证合一) | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `INST_RGST_CTF` | 事业单位法人证书 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 联系人类型（支付宝）

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `LEGAL_PERSON` | 法人 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `CONTROLLER` | 实际控制人 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `AGENT` | 代理人 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `OTHER` | 其他 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 授权类型

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `AUTH` | 授权目录 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `JSAPI` | 微信公众号 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `APPLET` | 微信小程序 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 站点类型

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `01` | 网站 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `02` | APP | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `03` | 服务窗 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `04` | 公众号 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `05` | 其他 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `06` | 支付宝小程序 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 间连等级

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `INDIRECT_LEVEL_M1` | M1等级 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `INDIRECT_LEVEL_M2` | M2等级 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `INDIRECT_LEVEL_M3` | M3等级 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `INDIRECT_LEVEL_M4` | M4等级 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 商户状态

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `00` | 启用（默认值） | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `01` | 注销（注销的商户不能正常交易） | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 交易控制位

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `00` | 默认值 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `01` | 标识存量商户仅更新终端信息给银联 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 认证订单状态

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `AUDITING` | 审核中 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `CONTACT_CONFIRM` | 待联系人确认 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `LEGAL_CONFIRM` | 待法人确认 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `AUDIT_PASS` | 审核通过 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `AUDIT_REJECT` | 审核失败 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `AUDIT_FREEZE` | 已冻结 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `CANCELED` | 已撤回 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `CONTACT_PROCESSING` | 联系人处理中 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

#### 商户认证状态

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `AUTHORIZED` | 已确认 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `UNAUTHORIZED` | 未确认 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `CLOSED` | 已销户 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `SMID_NOT_EXIST` | smid不存在 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

### 附录 (`appendix`)

Source: `https://doc.mandao.com/docs/bct/appendix`; updated: `2025-09-16 09:09`.


#### 1、公司所属行业

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `9902` | 数字娱乐 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9910` | 互联网金融 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9911` | 保险 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9923` | 航旅 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9926` | 消费金融 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9927` | 基金 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9928` | 大宗商品现货交易 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9929` | 收藏品交易 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9930` | 电商B2B | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9931` | 零售B2C | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9937` | 代理商 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9938` | 物流 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9940` | 金融租赁 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9941` | 票据 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9943` | 教育培训 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9946` | 公共服务及便民服务 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9980` | 跨境服务贸易 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9981` | 跨境货物贸易 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9982` | 货币代兑 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9983` | 金融机构 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9990` | 征信 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `10012` | 有偿资讯 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |
| `9999` | 其他 | Use typed constant/allowlist when used; unknown values fail closed or become explicit unknown state |

## 7. Error Code Matrix

All error-code handling must preserve the upstream code internally for operators while returning only stable, safe LocalLife messages to callers. Do not expose raw payloads, certificate data, bank cards, phones, IDs, `contractNo`, `sharingMerId`, `subMchId`, or signatures.


### 错误码 (`bct-1fjpm4fpns79f`)

Source: `https://doc.mandao.com/docs/bct/bct-1fjpm4fpns79f`; updated: `2024-05-20 10:15`.


#### 错误码汇总

| retCode | 含义 | 描述 |
| --- | --- | --- |
| 0 | 失败 | 接口调用失败，异常或者参数校验失败。 |
| 1 | 成功 | 接口调用成功，具体业务是否成功。看具体的参数字段。 |
| 2 | 处理中 | 接口调用处理中，需要调用查询接口查询状态。 |

#### 错误码汇总

| 参数名 | 描述 |
| --- | --- |
| retCode | 1 受理成功 0受理失败 |
| errorCode | 校验失败错误码 |
| errorMsg | 校验失败描述 |

#### 错误码汇总

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `BF0001` | 请求参数非法 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF0005` | 系统异常，请稍后再试 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00214` | 请求商户非法 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00062` | 开户只支持单笔 | Map to ProviderError classification; never expose raw sensitive upstream payload |

#### 错误码汇总

| 参数名 | 描述 |
| --- | --- |
| state | 状态 1 成功 0 失败 -1 异常 2开户处理中 |
| errorCode | 业务异常错误码 |
| errorMsg | 业务异常错误码描述 |

#### 错误码汇总

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `BF00077` | 商户状态不正确 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00058` | 未开通相关产品 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00059` | 未开通相关功能 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00105` | 持卡人姓名与商户名称不同 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00110` | 上传客户文件不能为空 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00108` | 文件信息错误 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00107` | 文件缺失 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00060` | 该子商户已开户，请勿重复提交 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00063` | 您输入的银行卡号有误，请重新输入 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00111` | 绑定卡只能是借记卡 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF0013` | 商户订单号已存在，请勿重复提交 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF0002` | 数据库操作失败 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00106` | 持卡人姓名与经营者名称不同 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00217` | 企业有违法信息 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00061` | 企业法人四要素验证失败 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `BF00218` | 企业信息比对比率低 | Map to ProviderError classification; never expose raw sensitive upstream payload |

#### 错误码汇总

| Code/Value | Meaning | 契约实现要求 |
| --- | --- | --- |
| `PARAMETER_VALID_NOT_PASS` | 参数校验不通过 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `PARAMETER_VALID` | 请求参数有误 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `ID_CARD_CHECK_FAILED` | 身份证号码不合法 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `EXISTED_LOGIN_NO` | 登录号已存在 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `REPEATED_REQUEST` | 重复请求 | Map to ProviderError classification; never expose raw sensitive upstream payload |
| `SYSTEM_INNER_ERROR` | 服务忙，请稍后再试 | Map to ProviderError classification; never expose raw sensitive upstream payload |

### 错误码 (`bct-1f9qrfsj2fcbu`)

Source: `https://doc.mandao.com/docs/bct/bct-1f9qrfsj2fcbu`; updated: `2024-01-05 05:13`.


#### table 1

| Code | 含义/描述 | 解决/请求状态 | 契约实现要求 |
| --- | --- | --- | --- |
| `INVALID_PARAMETER` | 参数无效 | 请检查提交的参数是否有误 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `SYSTEM_BUSY` | 系统繁忙，请稍后再试 | 请稍后再次发起 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `ORDER_CREATED_FAIL` | 订单创建失败 | 请检查提交的参数，稍后再次发起 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `UNOPENED_PRODUCT` | 商户未开通此产品 | 请联系宝付 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `TRADE_AMT_EXCEEDS_LIMIT` | 交易金额超过单笔支付限额 | 请联系宝付 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `AGENT_RELATION_NOT_EXISTS` | 代理商关系不存在 | 请检查提交的参数是否有误 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `RISK_REFUSED` | 风控拒绝 | 请联系宝付 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `MERCHANT_NOT_EXIST` | 商户号不存在 | 请检查商户号是否正确 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `TERMINAL_NOT_EXIST` | 终端号不存在 | 请检查终端号是否正确 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `MERID_TERID_NOT_MATCH` | 商户号和终端号不匹配 | 请确认商户号和终端号是否匹配 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `VERIFY_ERROR` | 验签错误 | 请检查签名参数和方法是否符合规范 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `DGTL_DEC_ERROR` | 数字信封解密失败 | 请检查签名参数和方法是否符合规范 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `ORDER_EXIST` | 订单已存在 | 请核实商户订单号是否重复提交 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `ORDER_NOT_EXIST` | 原订单不存在 | 请检查订单是否发起过交易 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `SHARE_INFO_NOT_CORRECT` | 分账信息校验不通过 | 请检查提交的参数是否有误 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `SHARE_DEPLOY_NOT_EXIST` | 分账配置不存在 | 请联系宝付 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `DEPLOY_NOT_CORRECT` | 分账配置不正确 | 请检查提交的参数是否有误 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `FEE_MER_ID_ERROR` | 扣费商户号传入错误 | 请检查提交的参数是否有误 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `PAY_CODE_ERROR` | 支付方式传入错误 | 请检查提交的参数是否有误 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `TRADE_UNCONFIRMED` | 交易结果未知，请稍后查询 | 请稍后查询 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `ORDER_NOT_SUPPORT_REFUNDS` | 订单未支付成功，无法退款 | 请核实支付订单是否支付成功 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `REFUND_AMT_EXCEEDS` | 退款金额超限 | 请检查提交的参数是否有误 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `NOT_SUPPORT_PAY_CODE` | 不支持的支付方式 | 请检查提交的参数是否有误 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `NOT_SUPPORT_CONCURRENT` | 不支持的并发操作 | 请分批次执行 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `CHANNEL_RETURN_ERR` | 渠道返回错误 | 请联系宝付 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |
| `MERCHANT_NOT_REPORT` | 商户未在渠道报备 | 请先报备之后再做交易 | Map to ProviderError; preserve upstream code internally; expose safe Chinese guidance only; retry only if classification says retryable |

## 8. Deferred Account API Field Tables

These account pages are not first-version production runtime paths unless explicitly enabled later. They are included because they are in the account API list and must not be implemented from memory or by copying another DTO. Before enabling any row here, add local DTOs, validators, tests, source-matrix rows, and sandbox evidence.


### 查询绑定卡接口 (`queryCard`)

Source: `https://doc.mandao.com/docs/bct/queryCard`; updated: `2025-09-16 09:09`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `queryCard.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.0.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryCard.request.body.loginNo` | `loginNo` | String | 128 | C | conditional rule in description; length=128 | 登录号(无商户客户号必填) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryCard.request.body.contractNo` | `contractNo` | String | 32 | C | length=32 | 客户账户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `queryCard.request.body.accType` | `accType` | int | 1 | M | length=1 | 账户类型:1个人,2商户 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryCard.request.body.platformNo` | `platformNo` | String | 32 | C | conditional rule in description; length=32 | 平台号(主商户号)(无商户客户号必填) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `queryCard.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryCard.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryCard.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryCard.response.body.bindCardInfoList` | `bindCardInfoList` | List |  | C |  | 绑卡信息,当retCode=1是有值 | Encode condition from notes; add positive/negative conditional tests |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `queryCard.response.bindCardInfoList.personal.cardUserName` | `cardUserName` | String | 32 | M | length=32 | 客户名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryCard.response.bindCardInfoList.personal.cardNo` | `cardNo` | String | 128 | M | length=128 | 卡号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryCard.response.bindCardInfoList.personal.bankName` | `bankName` | String | 20 | M | length=20 | 银行名称 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `queryCard.response.bindCardInfoList.business.cardUserName` | `cardUserName` | String | 32 | M | length=32 | 客户名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryCard.response.bindCardInfoList.business.cardNo` | `cardNo` | String | 128 | M | length=128 | 卡号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryCard.response.bindCardInfoList.business.bankName` | `bankName` | String | 20 | M | length=20 | 银行名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryCard.response.bindCardInfoList.business.depositBankProvince` | `depositBankProvince` | String | 20 | M | length=20 | 开户行省份 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryCard.response.bindCardInfoList.business.depositBankCity` | `depositBankCity` | String | 20 | M | length=20 | 开户行城市 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryCard.response.bindCardInfoList.business.depositBankName` | `depositBankName` | String | 64 | M | length=64 | 开户支行名称 | Validate required; add missing-field test; Preserve official length/type at boundary |

### 账户信息修改接口 (`updateCard`)

Source: `https://doc.mandao.com/docs/bct/updateCard`; updated: `2026-04-02 08:33`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `updateCard.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.0.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `updateCard.request.body.accType` | `accType` | int | 1 | M | length=1 | 账户类型:1个人,2商户 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `updateCard.request.body.accInfo` | `accInfo` | Object |  | M |  | 开户具体信息根据类型不同,信息不同,现只支持单笔修改 | Validate required; add missing-field test |

#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `updateCard.request.accInfo.personal.contractNo` | `contractNo` | String | 64 | M | length=64 | 客户账户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `updateCard.request.accInfo.personal.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `updateCard.request.accInfo.personal.cardNo` | `cardNo` | String | 128 | C | length=128 | 卡号影响计费 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.request.accInfo.personal.mobileNo` | `mobileNo` | String | 64 | C | length=64 | 银行预留手机号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.request.accInfo.personal.cardUserName` | `cardUserName` | String | 20 | C | length=20 | 持卡人姓名影响计费 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |

#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `updateCard.request.accInfo.business.contractNo` | `contractNo` | String | 32 | M | length=32 | 客户账户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `updateCard.request.accInfo.business.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.cardNo` | `cardNo` | String | 128 | C | conditional rule in description; length=128 | 卡号 影响计费-对私结算<br>(修改卡号时必传) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.cardUserName` | `cardUserName` | String | 60 | C | length=60 | 持卡人姓名 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.bankName` | `bankName` | String | 20 | C | conditional rule in description; length=20 | 银行名称 影响计费-对私结算<br>(修改卡号时必传) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.depositBankProvince` | `depositBankProvince` | String | 20 | C | conditional rule in description; length=20 | 开户行省份 (修改卡号时必传) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.depositBankCity` | `depositBankCity` | String | 20 | C | conditional rule in description; length=20 | 开户行城市 (修改卡号时必传) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.depositBankName` | `depositBankName` | String | 64 | C | conditional rule in description; length=64 | 开户支行 影响计费-对私结算<br>(修改卡号时必传) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.contactName` | `contactName` | String | 20 | O | length=20 | 联系人姓名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.contactMobile` | `contactMobile` | String | 64 | O | length=64 | 联系人手机号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.corporateMobile` | `corporateMobile` | String | 64 | O | conditional rule in description; length=64 | 法人手机号影响计费-对私结算<br>当开个体户且绑定对私卡时必传 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.customerName` | `customerName` | String | 60 | C | length=60 | 公司名称影响计费 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.corporateName` | `corporateName` | String | 20 | C | length=20 | 法人姓名影响计费 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.corporateCertId` | `corporateCertId` | String | 32 | C | length=32 | 法人身份证号影响计费 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.request.accInfo.business.aliasName` | `aliasName` | String | 64 | O | length=64 | 商户名称别名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `updateCard.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `updateCard.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `updateCard.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `updateCard.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `updateCard.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `updateCard.response.body.contractNo` | `contractNo` | String | 64 | M | length=64 | 商户客户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |

### 账户收支明细查询 (`accDetails`)

Source: `https://doc.mandao.com/docs/bct/accDetails`; updated: `2026-01-23 01:32`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `accDetails.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.0.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accDetails.request.body.contractNo` | `contractNo` | String | 32 | M | length=32 | 商户客户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `accDetails.request.body.accountType` | `accountType` | String | 32 | O | conditional rule in description; length=32 | BALANCE-余额户,TRANSIT-在途户,不传默认余额户 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.request.body.startTime` | `startTime` | String | 32 | O | date/time format; length=32 | 明细开始时间 yyyy-MM-dd HH：mm：ss | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Validate official time/date format |
| `accDetails.request.body.endTime` | `endTime` | String | 32 | O | date/time format; length=32 | 明细结束时间 yyyy-MM-dd HH：mm：ss查询间隔最大支持一个月 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Validate official time/date format |
| `accDetails.request.body.pageNum` | `pageNum` | int | 32 | M | length=32 | 开始页 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accDetails.request.body.pageSize` | `pageSize` | int | 32 | M | length=32 | 每页显示记录数,最大100 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `accDetails.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accDetails.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `accDetails.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `accDetails.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.body.pageNum` | `pageNum` | int | 32 | O | length=32 | 开始页 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.body.pageSize` | `pageSize` | int | 32 | O | length=32 | 每页显示记录数 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.body.pageCount` | `pageCount` | int | 32 | O | length=32 | 总页数 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.body.list` | `list` | List |  | O |  | 数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `accDetails.response.pageResultList[].transType` | `transType` | String | 40 | O | length=40 | 交易类型 :RECHARGE 入金,TRANSFER 划拨 ,WITHDRAW 出金 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.pageResultList[].drCrFlag` | `drCrFlag` | String | 10 | O | length=10 | 余额方向：CR-贷款（收入）/ DR-借款（支出） | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.pageResultList[].ccy` | `ccy` | String | 10 | O | length=10 | 币种 CNY人民币 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.pageResultList[].amount` | `amount` | BigDecimal | 10,2 | O | unit=yuan; length=10,2 | 交易金额,单位：元 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `accDetails.response.pageResultList[].afterBal` | `afterBal` | BigDecimal | 10,2 | O | unit=yuan; length=10,2 | 交易后余额,单位：元 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `accDetails.response.pageResultList[].creatTime` | `creatTime` | String | 32 | O | date/time format; length=32 | 创建时间yyyy-MM-dd HH:mm:ss | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Validate official time/date format |
| `accDetails.response.pageResultList[].orderId` | `orderId` | String | 32 | O | length=32 | 宝付订单号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.pageResultList[].transSerialNo` | `transSerialNo` | String | 200 | O | length=200 | 商户订单号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.pageResultList[].businessTadeType` | `businessTadeType` | String | 200 | O | length=200 | SHARE-分账<br>OFFSET_SHARE-差额分账<br>REFUND-退款<br>TRANSFER-转账<br>WITHDRAW-提现<br>CLEAR-资金清算<br>OTHER-其他<br>WITHDRAW_CANCEL-提现退回 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.pageResultList[].partyAcctName` | `partyAcctName` | String | 200 | O | length=200 | 对手方户名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accDetails.response.pageResultList[].partyAcctNo` | `partyAcctNo` | String | 32 | O | length=32 | 对手方账号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 转账（账户间） (`accTransfer`)

Source: `https://doc.mandao.com/docs/bct/accTransfer`; updated: `2026-02-27 09:00`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `accTransfer.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.0.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accTransfer.request.body.payerNo` | `payerNo` | String | 32 | M | length=32 | 付款方(二级子商户号) | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accTransfer.request.body.payeeNo` | `payeeNo` | String | 32 | M | length=32 | 收款方(二级子商户号) | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accTransfer.request.body.transSerialNo` | `transSerialNo` | String | 50 | M | length=50 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accTransfer.request.body.dealAmount` | `dealAmount` | BigDecimal | 10,2 | M | unit=yuan; length=10,2 | 转账金额,单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `accTransfer.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accTransfer.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `accTransfer.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `accTransfer.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accTransfer.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accTransfer.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accTransfer.response.body.transSerialNo` | `transSerialNo` | String | 50 | M | length=50 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accTransfer.response.body.businessNo` | `businessNo` | String | 255 | O | length=255 | 业务流水号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accTransfer.response.body.payerNo` | `payerNo` | String | 32 | M | length=32 | 付款方(二级子商户号) | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accTransfer.response.body.payeeNo` | `payeeNo` | String | 32 | M | length=32 | 收款方(二级子商户号) | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accTransfer.response.body.dealAmount` | `dealAmount` | BigDecimal | 10,2 | M | unit=yuan; length=10,2 | 转账金额,单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `accTransfer.response.body.feeAmount` | `feeAmount` | BigDecimal | 10,2 | O | unit=yuan; length=10,2 | 手续费金额,单位：元 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `accTransfer.response.body.state` | `state` | int | 2 | M | length=2 | 订单状态 1成功 2失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accTransfer.response.body.transRemark` | `transRemark` | String | 128 | O | length=128 | 失败原因 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 转账结果查询（账户间） (`queryTransfer`)

Source: `https://doc.mandao.com/docs/bct/queryTransfer`; updated: `2026-02-27 09:00`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `queryTransfer.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.0.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryTransfer.request.body.transSerialNo` | `transSerialNo` | String | 50 | M | length=50 | 原转账订单请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryTransfer.request.body.tradeTime` | `tradeTime` | String | 10 | M | date/time format; length=10 | 原交易时间 yyyy-MM-dd | Validate required; add missing-field test; Preserve official length/type at boundary; Validate official time/date format |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `queryTransfer.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryTransfer.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryTransfer.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `queryTransfer.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryTransfer.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryTransfer.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryTransfer.response.body.transSerialNo` | `transSerialNo` | String | 50 | M | length=50 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryTransfer.response.body.businessNo` | `businessNo` | String | 255 | O | length=255 | 业务流水号 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `queryTransfer.response.body.payerNo` | `payerNo` | String | 32 | M | length=32 | 付款方(二级子商户号) | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryTransfer.response.body.payeeNo` | `payeeNo` | String | 32 | M | length=32 | 收款方(二级子商户号) | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryTransfer.response.body.dealAmount` | `dealAmount` | BigDecimal | 10,2 | M | unit=yuan; length=10,2 | 转账金额,单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `queryTransfer.response.body.feeAmount` | `feeAmount` | BigDecimal | 10,2 | M | unit=yuan; length=10,2 | 手续费金额,单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `queryTransfer.response.body.state` | `state` | int | 2 | M | length=2 | 订单状态 1成功 2失败 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `queryTransfer.response.body.transRemark` | `transRemark` | String | 128 | O | length=128 | 失败原因 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 月终余额查询接口 (`bct-1g721ak74fap8`)

Source: `https://doc.mandao.com/docs/bct/bct-1g721ak74fap8`; updated: `2025-09-16 09:09`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `monthEndBalance.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.1.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `monthEndBalance.request.body.contractNo` | `contractNo` | String | 32 | M | length=32 | 客户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `monthEndBalance.request.body.date` | `date` | String | 6 | M | length=6 | 日期格式：YYYYMM | Validate required; add missing-field test; Preserve official length/type at boundary |
| `monthEndBalance.request.body.reqNo` | `reqNo` | String | 32 | M | length=32 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `monthEndBalance.response.body.contractNo` | `contractNo` | String | 32 | M | length=32 | 客户号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `monthEndBalance.response.body.customerName` | `customerName` | String | 32 | M | length=32 | 客户姓名 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `monthEndBalance.response.body.customerType` | `customerType` | String | - | M |  | 客户类型：1个体、2个人、3企业 | Validate required; add missing-field test |
| `monthEndBalance.response.body.beforeBalance` | `beforeBalance` | String | 16 | M | unit=yuan; length=16 | 月初余额，单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `monthEndBalance.response.body.inBalance` | `inBalance` | String | 16 | M | unit=yuan; length=16 | 入金金额，单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `monthEndBalance.response.body.inDetails` | `inDetails` | List | - | M |  | 入金明细：BusinessSummaryAmount | Validate required; add missing-field test |
| `monthEndBalance.response.body.outBalance` | `outBalance` | String | 16 | M | unit=yuan; length=16 | 出金金额，单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |
| `monthEndBalance.response.body.outDetails` | `outDetails` | List | - | M |  | 出金明细：BusinessSummaryAmount | Validate required; add missing-field test |
| `monthEndBalance.response.body.afterBalance` | `afterBalance` | String | 16 | M | unit=yuan; length=16 | 期末金额，单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `monthEndBalance.response.summaryList[].businessType` | `businessType` | String | 200 | M | length=200 | 业务类型<br>SHARE-分账<br>OFFSET_SHARE-差额分账<br>REFUND-退款<br>TRANSFER-转账<br>WITHDRAW-提现<br>CLEAR-资金清算<br>OTHER-其他<br>WITHDRAW_CANCEL-提现退回 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `monthEndBalance.response.summaryList[].businessTypeName` | `businessTypeName` | String | 200 | M | length=200 | 业务类型名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `monthEndBalance.response.summaryList[].amount` | `amount` | String | 16 | M | unit=yuan; length=16 | 业务汇总金额，单位：元 | Validate required; add missing-field test; Preserve official length/type at boundary; Yuan decimal: convert only at contract boundary |

### 账户升级接口 (`bct-1gahm5f1j79sk`)

Source: `https://doc.mandao.com/docs/bct/bct-1gahm5f1j79sk`; updated: `2025-09-16 09:09`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `accountUpgrade.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.1.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.request.body.contractNo` | `contractNo` | String | [1,32] | M | length=[1,32] | 平台号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `accountUpgrade.request.body.qualificationTransSerialNo` | `qualificationTransSerialNo` | String | [1,32] | M | length=[1,32] | 资质文件请求流水号<br>上传文件类型必须有：<br>101 企业营业执照<br>102 银行开户许可证<br>104 法人身份证(正)<br>111 法人身份证(反) | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.request.body.province` | `province` | String | [1,32] | M | length=[1,32] | 公司地址-省份 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.request.body.city` | `city` | String | [1,32] | M | length=[1,32] | 公司地址-城市 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.request.body.district` | `district` | String | [1,32] | M | length=[1,32] | 公司地址-区 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.request.body.detailedAddress` | `detailedAddress` | String | [1,512] | M | length=[1,512] | 公司地址-详细地址 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.request.body.businessAddress` | `businessAddress` | String | [1,512] | M | length=[1,512] | 经营地址 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.request.body.registeredAddress` | `registeredAddress` | String | [1,512] | M | length=[1,512] | 注册地址 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.request.body.legalPersonIdValidityPeriod` | `legalPersonIdValidityPeriod` | String | [1,8] | M | date/time format; length=[1,8] | 法人证件有效期 yyyyMMdd | Validate required; add missing-field test; Preserve official length/type at boundary; Validate official time/date format |
| `accountUpgrade.request.body.businessScope` | `businessScope` | String | [1,1500] | M | length=[1,1500] | 企业经营范围 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.request.body.establishmentDate` | `establishmentDate` | String | [1,8] | M | date/time format; length=[1,8] | 成立时间 yyyyMMdd | Validate required; add missing-field test; Preserve official length/type at boundary; Validate official time/date format |
| `accountUpgrade.request.body.registeredCapital` | `registeredCapital` | String | [1,20] | M | length=[1,20] | 注册资本 单位万元 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.request.body.businessExecutionValidityPeriod` | `businessExecutionValidityPeriod` | String | [1,8] | M | date/time format; length=[1,8] | 营业期限 yyyyMMdd,长期99991231 | Validate required; add missing-field test; Preserve official length/type at boundary; Validate official time/date format |
| `accountUpgrade.request.body.requestNo` | `requestNo` | String | [1,32] | M | length=[1,32] | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.request.body.contactMobile` | `contactMobile` | String | [1,32] | O | conditional rule in description; length=[1,32] | 联系人手机号,用于接收合作协议电子签验证码。开户时未传此字段必填 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `accountUpgrade.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 2 处理中 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountUpgrade.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `accountUpgrade.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `accountUpgrade.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accountUpgrade.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accountUpgrade.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 账户绑定新增接口 (`bct-1gahm7jsqr66u`)

Source: `https://doc.mandao.com/docs/bct/bct-1gahm7jsqr66u`; updated: `2025-09-16 09:09`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `accountBinding.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.1.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountBinding.request.body.contractNo` | `contractNo` | String | [1,32] | M | length=[1,32] | 平台编号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `accountBinding.request.body.upperContractNo` | `upperContractNo` | String | [1,32] | M | length=[1,32] | 上级平台编号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountBinding.request.body.operationType` | `operationType` | String | [1,32] | M | length=[1,32] | 01-新增、02-禁用 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountBinding.request.body.requestNo` | `requestNo` | String | [1,32] | M | length=[1,32] | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `accountBinding.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 2 处理中 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountBinding.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `accountBinding.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `accountBinding.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accountBinding.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accountBinding.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

### 账户绑定查询接口 (`bct-1gahm7u2hj1s1`)

Source: `https://doc.mandao.com/docs/bct/bct-1gahm7u2hj1s1`; updated: `2025-09-16 09:09`.


#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `accountBindingQuery.request.body.version` | `version` | String | 5 | M | length=5 | 版本号4.1.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountBindingQuery.request.body.contractNo` | `contractNo` | String | [1,32] | M | length=[1,32] | 平台编号 | Validate required; add missing-field test; Preserve official length/type at boundary; BaoCaiTong account id; never replace with channel subMchId |
| `accountBindingQuery.request.body.requestNo` | `requestNo` | String | [1,32] | M | length=[1,32] | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `accountBindingQuery.response.body.retCode` | `retCode` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 2 处理中 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `accountBindingQuery.response.body.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `accountBindingQuery.response.body.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `accountBindingQuery.response.body.back1` | `back1` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accountBindingQuery.response.body.back2` | `back2` | String | 64 | O | length=64 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accountBindingQuery.response.body.back3` | `back3` | String | 100 | O | length=100 | 备用字段 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `accountBindingQuery.response.body.list` | `list` | String |  | O |  | 绑定关系集合 json数组 | Optional: omit empty outbound; tolerate absent inbound |
| `accountBindingQuery.response.body.list.contractNo` | `list.contractNo` | String |  | O |  | 平台编号 | Optional: omit empty outbound; tolerate absent inbound |
| `accountBindingQuery.response.body.list.upperContractNo` | `list.upperContractNo` | String |  | O |  | 上级平台编号 | Optional: omit empty outbound; tolerate absent inbound |
| `accountBindingQuery.response.body.list.state` | `list.state` | String |  | O |  | 状态 OPEN(“开启”),PENDING_OPEN(“待开启”),CLOSED(“关闭”); | Optional: omit empty outbound; tolerate absent inbound |

## 9. File Upload / Qualification Deferred Tables

These are qualification/fund file upload pages related to fields such as `qualificationTransSerialNo`. They are not in the current runtime path, but account-open conditional fields must not be enabled without matching upload contract support.


### 资质文件上传接口1 (`bct-1gbbtmk773oah`)

Source: `https://doc.mandao.com/docs/bct/bct-1gbbtmk773oah`; updated: `2026-04-25 08:30`.


#### 接口须知

| 符号 | 符号性质 | 符号说明 |
| --- | --- | --- |
| M | 强制域(Mandatory) | 必须填写的域 |
| C | 条件域(Conditional) | 某条件成立时必须填写的域 |
| O | 选用域(Optional) | 选填属性（可选预） |
| R | 原样返回域(Returned) | 必须与先前报文中对应域的值相同的域 |

#### 请求报文

| Path | 字段名/域名 | 变量名 | 类型 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table2.orderType` | orderType | `orderType` | String | M |  | 类型值:0 宝财通上传文件 | Validate required; add missing-field test |
| `bct-1gbbtmk773oah.table2.memberId` | memberId | `memberId` | String | M |  | 宝付商户号 | Validate required; add missing-field test |
| `bct-1gbbtmk773oah.table2.terminalId` | terminalId | `terminalId` | String | M |  | 宝付终端号 | Validate required; add missing-field test |
| `bct-1gbbtmk773oah.table2.content` | content | `content` | String | M | conditional rule in description | 上送参数为请求下列接口请求参数，内容格式为JSON字串进行Base64转码后使用私钥RSA加密后上送 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |
| `bct-1gbbtmk773oah.table2.file` | file | `file` | Multipart | M | conditional rule in description | 上传的文件尽量文件名命名格式如下，上送的文件为ZIP压缩包。多个文件在fileNameMap中标识出来。与压缩包文件名一致。 | Validate required; add missing-field test |

#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table3.accType` | `accType` | int | 1 | M | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table3.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table3.businessParams` | `businessParams` | String | 128 | M | length=128 | Ordertype 0 :自定义唯一标识，示例：登录号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table3.noticeUrl` | `noticeUrl` | String | 200 | O | length=200 | 回调地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table3.fileNameMap` | `fileNameMap` | List |  |  |  | 文件名映射表 | Optional: omit empty outbound; tolerate absent inbound |

#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table4.fileType` | `fileType` | String | 1 | M | length=1 | 详情开户文件类型表，参看附录《开户文件类型表》 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table4.fileName` | `fileName` | String | 200 | M | length=200 | 文件类型对应文件名称（不包括扩展名） | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table5.success` | `success` | boolean | 8 | M | length=8 |  | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table5.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table5.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table5.result` | `result` | Json |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table6.memberId` | `memberId` | String | 32 | C | length=32 | 宝付商户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table6.terminalId` | `terminalId` | String | 32 | C | length=32 | 宝付终端号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table6.businessParams` | `businessParams` | String | 100 | C | length=100 | Ordertype 0 :自定义唯一标识，示例：登录号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table6.orderType` | `orderType` | String | 200 | C | length=200 | 订单类型 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table6.transSerialNo` | `transSerialNo` | int | 1 | C | length=1 | 请求流水号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table6.accType` | `accType` | int | 1 | C | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table6.state` | `state` | int | 1 | C | length=1 | 订单状态1成功,2失败 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table6.dfsFileNotifyList` | `dfsFileNotifyList` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table7.fileName` | `fileName` | String | 64 | M | length=64 | 文件名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table7.fileType` | `fileType` | String | 1 | M | length=1 | 文件类型 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table7.fileId` | `fileId` | String | 64 | M | length=64 | 文件ID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table7.dfsGroup` | `dfsGroup` | String | 32 | M | length=32 | Dfs分组 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table7.dfsFileName` | `dfsFileName` | String | 64 | O | length=64 | Dfs文件名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table7.dfsPath` | `dfsPath` | String | 100 | M | length=100 | Dfs路径 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table7.state` | `state` | int | 1 | O | length=1 | 文件上传状态 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table8.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table8.businessParams` | `businessParams` | String | 128 | M | length=128 | Ordertype 0 :自定义唯一标识，示例：登录号 | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table9.success` | `success` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 -1 异常 2处理中 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table9.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table9.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table9.result` | `result` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table10.memberId` | `memberId` | String | 32 | C | length=32 | 宝付商户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table10.terminalId` | `terminalId` | String | 32 | C | length=32 | 宝付终端号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table10.businessParams` | `businessParams` | String | 100 | C | length=100 | Ordertype 0 :自定义唯一标识，示例：登录号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table10.orderType` | `orderType` | String | 200 | C | length=200 | 订单类型 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table10.transSerialNo` | `transSerialNo` | int | 1 | C | length=1 | 请求流水号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table10.accType` | `accType` | int | 1 | C | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table10.state` | `state` | int | 1 | C | length=1 | 订单状态1成功,2失败 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table10.sync` | `sync` | int | 1 | C | length=1 | 同步状态0异步 1同步 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table10.dfsFileNotifyList` | `dfsFileNotifyList` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table11.fileName` | `fileName` | String | 64 | M | length=64 | 文件名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table11.fileType` | `fileType` | String | 1 | M | length=1 | 文件类型 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table11.fileId` | `fileId` | String | 64 | M | length=64 | 文件ID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table11.dfsGroup` | `dfsGroup` | String | 32 | M | length=32 | Dfs分组 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table11.dfsFileName` | `dfsFileName` | String | 64 | O | length=64 | Dfs文件名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table11.dfsPath` | `dfsPath` | String | 100 | M | length=100 | Dfs路径 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table11.state` | `state` | int | 1 | O | length=1 | 文件重传/上传状态 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table12.accType` | `accType` | int | 1 | M | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table12.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table12.businessParams` | `businessParams` | String | 128 | M | length=128 | Ordertype 0 :开户返回的客户账户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table12.noticeUrl` | `noticeUrl` | String | 200 | O | length=200 | 回调地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table12.fileNameMap` | `fileNameMap` | List |  |  |  | 文件名映射表 | Optional: omit empty outbound; tolerate absent inbound |

#### 请求参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table13.fileType` | `fileType` | String | 1 | M | length=1 | 详情开户文件类型表 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table13.fileName` | `fileName` | String | 200 | M | length=200 | 文件类型对应文件名称（不包括扩展名） | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table14.success` | `success` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 -1 异常 2处理中 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table14.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table14.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table14.result` | `result` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table15.memberId` | `memberId` | String | 32 | C | length=32 | 宝付商户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table15.terminalId` | `terminalId` | String | 32 | C | length=32 | 宝付终端号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table15.businessParams` | `businessParams` | String | 100 | C | length=100 | Ordertype 0 :自定义唯一标识，示例：登录号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table15.orderType` | `orderType` | String | 200 | C | length=200 | 订单类型 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table15.transSerialNo` | `transSerialNo` | int | 1 | C | length=1 | 请求流水号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table15.accType` | `accType` | int | 1 | C | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table15.dfsFileNotifyList` | `dfsFileNotifyList` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table16.fileName` | `fileName` | String | 64 | M | length=64 | 文件名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table16.fileType` | `fileType` | String | 1 | M | length=1 | 文件类型 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table16.fileId` | `fileId` | String | 64 | M | length=64 | 文件ID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table16.dfsGroup` | `dfsGroup` | String | 32 | M | length=32 | Dfs分组 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table16.dfsFileName` | `dfsFileName` | String | 64 | O | length=64 | Dfs文件名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table16.dfsPath` | `dfsPath` | String | 100 | M | length=100 | Dfs路径 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table16.state` | `state` | int | 1 | O | length=1 | 文件上传状态 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 报文参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table17.memberId` | `memberId` | String | 32 | C | length=32 | 宝付商户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table17.terminalId` | `terminalId` | String | 32 | C | length=32 | 宝付终端号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table17.businessParams` | `businessParams` | String | 100 | C | length=100 | Ordertype 0 :自定义唯一标识，示例：登录号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table17.orderType` | `orderType` | String | 200 | C | length=200 | 订单类型 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table17.transSerialNo` | `transSerialNo` | int | 1 | C | length=1 | 请求流水号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table17.accType` | `accType` | int | 1 | C | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table17.dfsFileNotifyList` | `dfsFileNotifyList` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 报文参数

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gbbtmk773oah.table18.fileName` | `fileName` | String | 64 | M | length=64 | 文件名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table18.fileType` | `fileType` | String | 1 | M | length=1 | 文件类型 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table18.fileId` | `fileId` | String | 64 | M | length=64 | 文件ID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table18.dfsGroup` | `dfsGroup` | String | 32 | M | length=32 | Dfs分组 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table18.dfsFileName` | `dfsFileName` | String | 64 | O | length=64 | Dfs文件名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table18.dfsPath` | `dfsPath` | String | 100 | M | length=100 | Dfs路径 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gbbtmk773oah.table18.state` | `state` | int | 1 | O | length=1 | 文件上传/重传状态 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 开户文件类型表

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

#### 文件参考命名格式(zip结尾): 压缩包文件名不强制规定

| 二级商户类型 | 格式 | 示例 |
| --- | --- | --- |
| 公司 | 平台商户号_二级商户户名 | 120838_zhangsan@163.com |
| 个体工商户 | 平台商户号_二级商户户名 | 120838_zhangsan@163.com |
| 自然人 | 平台商户号_手机号 | 120838_13999990001 |

### 资金文件上传接口2（暂未启用） (`bct-1gpdo5i47lrsj`)

Source: `https://doc.mandao.com/docs/bct/bct-1gpdo5i47lrsj`; updated: `2025-09-16 09:14`.


#### 接口须知

| 符号 | 符号性质 | 符号说明 |
| --- | --- | --- |
| M | 强制域(Mandatory) | 必须填写的域 |
| C | 条件域(Conditional) | 某条件成立时必须填写的域 |
| O | 选用域(Optional) | 选填属性（可选预） |
| R | 原样返回域(Returned) | 必须与先前报文中对应域的值相同的域 |

#### 请求报文

| Path | 字段名/域名 | 变量名 | 类型 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table2.orderType` | orderType | `orderType` | String | M |  | 类型值:0 宝财通上传文件 | Validate required; add missing-field test |
| `bct-1gpdo5i47lrsj.table2.memberId` | memberId | `memberId` | String | M |  | 宝付商户号 | Validate required; add missing-field test |
| `bct-1gpdo5i47lrsj.table2.terminalId` | terminalId | `terminalId` | String | M |  | 宝付终端号 | Validate required; add missing-field test |
| `bct-1gpdo5i47lrsj.table2.verifyType` | verifyType | `verifyType` | String | M | fixed value | 固定值：11（RSA国际加密） | Validate required; add missing-field test; Use typed constant/allowlist; unknown fail-closed |
| `bct-1gpdo5i47lrsj.table2.content` | content | `content` | String | M |  | 内容 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |
| `bct-1gpdo5i47lrsj.table2.file` | file | `file` | Multipart | M | conditional rule in description | 上传的文件尽量文件名命名格式如下，上送的文件为ZIP压缩包。多个文件在fileNameMap中标识出来。与压缩包文件名一致。 | Validate required; add missing-field test |
| `bct-1gpdo5i47lrsj.table2.sha256` | sha256 | `sha256` | String | M | fixed value | 固定长度64位，对文件流进行sha256运算后转16进制 | Validate required; add missing-field test |
| `bct-1gpdo5i47lrsj.table2.signature` | signature | `signature` | String | M |  | 除签名字段外全部加签。按key1=value1&key2=value2…模式将TreeMap对象转换为字符串，UTF-8编码格式下进行SHA-256计算后使用RSA私钥加签 | Validate required; add missing-field test |

#### 同步返回报文

| Path | 字段名/域名 | 变量名 | 类型 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table3.content` | content | `content` | String | M |  | 内容 | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |
| `bct-1gpdo5i47lrsj.table3.signature` | signature | `signature` | String | M |  | 除签名字段外全部加签。按key1=value1&key2=value2…模式将TreeMap对象转换为字符串，UTF-8编码格式下进行SHA-256计算后使用RSA私钥加签 | Validate required; add missing-field test |

#### 请求参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table4.accType` | `accType` | int | 1 | M | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table4.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table4.businessParams` | `businessParams` | String | 128 | M | length=128 | Ordertype 0 :自定义唯一标识，示例：登录号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table4.noticeUrl` | `noticeUrl` | String | 200 | O | length=200 | 回调地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table4.fileNameMap` | `fileNameMap` | List |  |  |  | 文件名映射表 | Optional: omit empty outbound; tolerate absent inbound |
| `bct-1gpdo5i47lrsj.table4.platformNo` | `platformNo` | String | 32 | C | conditional rule in description; length=32 | 平台号(主商户号) (代理模式必传) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |

#### 请求参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table5.fileType` | `fileType` | String | 1 | M | length=1 | 详情开户文件类型表 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table5.fileName` | `fileName` | String | 200 | M | length=200 | 文件类型对应文件名称（不包括扩展名） | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table6.success` | `success` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 -1 异常 2处理中 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table6.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table6.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table6.result` | `result` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table7.memberId` | `memberId` | String | 32 | C | length=32 | 宝付商户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table7.terminalId` | `terminalId` | String | 32 | C | length=32 | 宝付终端号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table7.businessParams` | `businessParams` | String | 100 | C | length=100 | Ordertype 0 :自定义唯一标识，示例：登录号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table7.orderType` | `orderType` | String | 200 | C | length=200 | 订单类型 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table7.transSerialNo` | `transSerialNo` | int | 1 | C | length=1 | 请求流水号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table7.accType` | `accType` | int | 1 | C | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table7.state` | `state` | int | 1 | C | length=1 | 订单状态1成功,2失败 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table7.dfsFileNotifyList` | `dfsFileNotifyList` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table8.fileName` | `fileName` | String | 64 | M | length=64 | 文件名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table8.fileType` | `fileType` | String | 1 | M | length=1 | 文件类型 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table8.fileId` | `fileId` | String | 64 | M | length=64 | 文件ID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table8.dfsGroup` | `dfsGroup` | String | 32 | M | length=32 | Dfs分组 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table8.dfsFileName` | `dfsFileName` | String | 64 | O | length=64 | Dfs文件名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table8.dfsPath` | `dfsPath` | String | 100 | M | length=100 | Dfs路径 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table8.state` | `state` | int | 1 | O | length=1 | 文件上传状态 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 请求参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table9.version` | `version` | String | 5 | M | length=5 | 版本号：1.0.0 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table9.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table9.businessParams` | `businessParams` | String | 128 | M | length=128 | Ordertype 0 :自定义唯一标识，示例：登录号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table9.platformNo` | `platformNo` | String | 32 | C | conditional rule in description; length=32 | 平台号(主商户号) (代理模式必传) | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table9.tradeTime` | `tradeTime` | String | 10 | M | length=10 | 对应流水号的交易日期，格式YYYY-MM-DD | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table10.success` | `success` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 -1 异常 2处理中 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table10.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table10.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table10.result` | `result` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table11.memberId` | `memberId` | String | 32 | C | length=32 | 宝付商户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table11.terminalId` | `terminalId` | String | 32 | C | length=32 | 宝付终端号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table11.businessParams` | `businessParams` | String | 100 | C | length=100 | Ordertype 0 :自定义唯一标识，示例：登录号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table11.orderType` | `orderType` | String | 200 | C | length=200 | 订单类型 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table11.transSerialNo` | `transSerialNo` | int | 1 | C | length=1 | 请求流水号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table11.accType` | `accType` | int | 1 | C | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table11.state` | `state` | int | 1 | C | length=1 | 订单状态1成功,2失败 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table11.sync` | `sync` | int | 1 | C | length=1 | 同步状态0异步 1同步 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table11.dfsFileNotifyList` | `dfsFileNotifyList` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table12.fileName` | `fileName` | String | 64 | M | length=64 | 文件名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table12.fileType` | `fileType` | String | 1 | M | length=1 | 文件类型 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table12.fileId` | `fileId` | String | 64 | M | length=64 | 文件ID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table12.dfsGroup` | `dfsGroup` | String | 32 | M | length=32 | Dfs分组 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table12.dfsFileName` | `dfsFileName` | String | 64 | O | length=64 | Dfs文件名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table12.dfsPath` | `dfsPath` | String | 100 | M | length=100 | Dfs路径 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table12.state` | `state` | int | 1 | O | length=1 | 文件重传/上传状态 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 请求参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table13.accType` | `accType` | int | 1 | M | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table13.transSerialNo` | `transSerialNo` | String | 200 | M | length=200 | 请求流水号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table13.businessParams` | `businessParams` | String | 128 | M | length=128 | Ordertype 0 :开户返回的客户账户号 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table13.noticeUrl` | `noticeUrl` | String | 200 | O | length=200 | 回调地址 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table13.fileNameMap` | `fileNameMap` | List |  |  |  | 文件名映射表 | Optional: omit empty outbound; tolerate absent inbound |

#### 请求参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table14.fileType` | `fileType` | String | 1 | M | length=1 | 详情开户文件类型表 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table14.fileName` | `fileName` | String | 200 | M | length=200 | 文件类型对应文件名称（不包括扩展名） | Validate required; add missing-field test; Preserve official length/type at boundary |

#### 返回参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table15.success` | `success` | int | 4 | M | length=4 | 返回码 1 成功 0 失败 -1 异常 2处理中 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table15.errorCode` | `errorCode` | String | 20 | C | length=20 | 错误码 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table15.errorMsg` | `errorMsg` | String | 40 | C | length=40 | 错误原因 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table15.result` | `result` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table16.memberId` | `memberId` | String | 32 | C | length=32 | 宝付商户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table16.terminalId` | `terminalId` | String | 32 | C | length=32 | 宝付终端号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table16.businessParams` | `businessParams` | String | 100 | C | length=100 | Ordertype 0 :自定义唯一标识，示例：登录号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table16.orderType` | `orderType` | String | 200 | C | length=200 | 订单类型 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table16.transSerialNo` | `transSerialNo` | int | 1 | C | length=1 | 请求流水号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table16.accType` | `accType` | int | 1 | C | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table16.dfsFileNotifyList` | `dfsFileNotifyList` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 返回参数content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table17.fileName` | `fileName` | String | 64 | M | length=64 | 文件名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table17.fileType` | `fileType` | String | 1 | M | length=1 | 文件类型 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table17.fileId` | `fileId` | String | 64 | M | length=64 | 文件ID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table17.dfsGroup` | `dfsGroup` | String | 32 | M | length=32 | Dfs分组 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table17.dfsFileName` | `dfsFileName` | String | 64 | O | length=64 | Dfs文件名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table17.dfsPath` | `dfsPath` | String | 100 | M | length=100 | Dfs路径 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table17.state` | `state` | int | 1 | O | length=1 | 文件上传状态 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 报文头说明

| Path | 字段名/域名 | 变量名 | 类型 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table18.memberId` | memberId | `memberId` | String | M |  | 宝付商户号 | Validate required; add missing-field test |
| `bct-1gpdo5i47lrsj.table18.terminalId` | terminalId | `terminalId` | String | M |  | 宝付终端号 | Validate required; add missing-field test |
| `bct-1gpdo5i47lrsj.table18.data_type` | data_type | `data_type` | String | M |  | JSON | Validate required; add missing-field test |
| `bct-1gpdo5i47lrsj.table18.data_content` | data_content | `data_content` | String | M |  |  | Validate required; add missing-field test; Security boundary: sign/decrypt/verify before business parsing |
| `bct-1gpdo5i47lrsj.table18.signature` | signature | `signature` | String | M |  | 除签名字段外全部加签。按key1=value1&key2=value2…模式将TreeMap对象转换为字符串，UTF-8编码格式下进行SHA-256计算后使用RSA私钥加签 | Validate required; add missing-field test |

#### 报文参数data_content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table19.memberId` | `memberId` | String | 32 | C | length=32 | 宝付商户号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table19.terminalId` | `terminalId` | String | 32 | C | length=32 | 宝付终端号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table19.businessParams` | `businessParams` | String | 100 | C | length=100 | Ordertype 0 :自定义唯一标识，示例：登录号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table19.orderType` | `orderType` | String | 200 | C | length=200 | 订单类型 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table19.transSerialNo` | `transSerialNo` | int | 1 | C | length=1 | 请求流水号 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table19.accType` | `accType` | int | 1 | C | length=1 | 账户类型:1个人,2商户,3 个体工商户 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table19.state` | `state` | int | 1 | C | length=1 | 订单状态1成功,2失败 | Encode condition from notes; add positive/negative conditional tests; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table19.dfsFileNotifyList` | `dfsFileNotifyList` | List |  |  |  | 返回数据列表 | Optional: omit empty outbound; tolerate absent inbound |

#### 报文参数data_content

| Path | 字段名 | 类型 | 长度 | 必填 | 枚举/条件/格式要求 | 官方描述 | 契约实现要求 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bct-1gpdo5i47lrsj.table20.fileName` | `fileName` | String | 64 | M | length=64 | 文件名称 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table20.fileType` | `fileType` | String | 1 | M | length=1 | 文件类型 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table20.fileId` | `fileId` | String | 64 | M | length=64 | 文件ID | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table20.dfsGroup` | `dfsGroup` | String | 32 | M | length=32 | Dfs分组 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table20.dfsFileName` | `dfsFileName` | String | 64 | O | length=64 | Dfs文件名 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table20.dfsPath` | `dfsPath` | String | 100 | M | length=100 | Dfs路径 | Validate required; add missing-field test; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table20.state` | `state` | int | 1 | O | length=1 | 文件上传/重传状态 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |
| `bct-1gpdo5i47lrsj.table20.path` | `path` | String | 100 | O | length=100 | 文件路径 | Optional: omit empty outbound; tolerate absent inbound; Preserve official length/type at boundary |

#### 附录

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

#### 附录

| 二级商户类型 | 格式 | 示例 |
| --- | --- | --- |
| 公司 | 平台商户号_二级商户户名 | 120838_zhangsan@163.com |
| 个体工商户 | 平台商户号_二级商户户名 | 120838_zhangsan@163.com |
| 自然人 | 平台商户号_手机号 | 120838_13999990001 |

## 10. Implementation Closure Checklist

| Gate | Required evidence |
| --- | --- |
| DTO completeness | Every first-version `Path` has a Go DTO/parser field or an explicit unsupported/deferred decision. |
| Requiredness | Every `M/是` row has a missing-field test; every `C` row has both condition-met and condition-not-met tests. |
| Enums | Every enum/fixed-value row uses a constant or allowlist; callbacks reject unknown official-critical enums. |
| Units | Yuan `BigDecimal` and fen integer fields convert only in contract packages; business logic never guesses units. |
| ID ownership | `sharingMerId/contractNo/bctMerId` and `subMchId/sub_mch_id` have separate types or validators and cannot be used interchangeably. |
| Signature/encryption | `content/data_content/bizContent/dataContent/signStr/dgtlEnvlp` are parsed only after the correct envelope, decryption, and signature checks. |
| Async idempotency | Payment/share/refund/withdraw/open callbacks persist facts before state application and ACK only after safe local handling. |
| Error semantics | `sysRespCode`, `retCode`, `returnCode`, `resultCode`, `errCode`, `state/txnState/refundState` are interpreted at their own layer, not collapsed into one success flag. |
| C4 evidence | Sandbox/production evidence is masked and recorded in `baofu-sandbox-evidence.md`; endpoint reachability alone is not C4. |
