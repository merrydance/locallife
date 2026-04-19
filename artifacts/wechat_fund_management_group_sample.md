# 账户资金管理组文档样例

本文件基于微信支付服务商账户资金管理组官方文档摘录，保留 contracts、errorcodes 与 alignment audit 所需的接口、字段、枚举和语义约束。

来源页面：

- 查询二级商户账户实时余额：https://pay.weixin.qq.com/doc/v3/partner/4012476690.md
- 查询二级商户账户日终余额：https://pay.weixin.qq.com/doc/v3/partner/4012476693.md
- 查询平台账户实时余额：https://pay.weixin.qq.com/doc/v3/partner/4012476700.md
- 查询平台账户日终余额：https://pay.weixin.qq.com/doc/v3/partner/4012476702.md
- 二级商户预约提现：https://pay.weixin.qq.com/doc/v3/partner/4012476652.md
- 二级商户查询预约提现状态（根据商户预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476656.md
- 二级商户查询预约提现状态（根据微信支付预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476665.md
- 平台预约提现：https://pay.weixin.qq.com/doc/v3/partner/4012476670.md
- 平台查询预约提现状态（根据商户预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476672.md
- 平台查询预约提现状态（根据微信支付预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476674.md
- 二级商户按日终余额预约提现：https://pay.weixin.qq.com/doc/v3/partner/4013328143.md
- 查询二级商户按日终余额预约提现状态：https://pay.weixin.qq.com/doc/v3/partner/4013328163.md
- 按日下载提现异常文件：https://pay.weixin.qq.com/doc/v3/partner/4012476678.md
- 商户提现状态变更通知：https://pay.weixin.qq.com/doc/v3/partner/4013049135.md

## 账户资金管理-查询二级商户账户实时余额

### 接口说明

请求方式：【GET】
`/v3/ecommerce/fund/balance/{sub_mchid}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| account_type | string | false | 二级商户账户类型，枚举值：BASIC FEES OPERATION DEPOSIT |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |
| available_amount | integer | true | 可用余额，单位为分 |
| pending_amount | integer | false | 不可用余额，单位为分 |
| account_type | string | false | 二级商户账户类型，枚举值：BASIC FEES OPERATION DEPOSIT |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 输入账户类型未开通 | 请检查 account_type 并重新调用 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | NO_AUTH | 无接口权限 | 请确认权限和受理关系 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 账户资金管理-查询二级商户账户日终余额

### 接口说明

请求方式：【GET】
`/v3/ecommerce/fund/enddaybalance/{sub_mchid}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| date | string(10) | true | 日期，格式 YYYY-MM-DD |
| account_type | string | false | 账户类型，枚举值：BASIC DEPOSIT |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |
| available_amount | integer | true | 可用余额，单位为分 |
| pending_amount | integer | false | 不可用余额，单位为分 |
| account_type | string | false | 账户类型，枚举值：BASIC DEPOSIT |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 该账户在指定时间不存在 | 请检查 date 输入是否正确 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | NO_AUTH | 无接口权限 | 请确认权限和受理关系 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 账户资金管理-查询平台账户实时余额

### 接口说明

请求方式：【GET】
`/v3/merchant/fund/balance/{account_type}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| account_type | string | true | 账户类型，枚举值：BASIC OPERATION FEES |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| available_amount | integer | true | 可用余额，单位为分 |
| pending_amount | integer | false | 不可用余额，单位为分 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 输入账户类型未开通 | 请检查 account_type 并重新调用 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | NO_AUTH | 当前商户号没有使用该接口的权限 | 请确认是否已经开通相关权限 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 账户资金管理-查询平台账户日终余额

### 接口说明

请求方式：【GET】
`/v3/merchant/fund/dayendbalance/{account_type}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| account_type | string | true | 账户类型，枚举值：BASIC OPERATION FEES |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| date | string(10) | false | 日期，格式 YYYY-MM-DD |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| available_amount | integer | true | 可用余额，单位为分 |
| pending_amount | integer | false | 不可用余额，单位为分 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 该账户在指定时间不存在 | 请检查 account_type 和 date 输入是否正确 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | NO_AUTH | 当前商户号没有使用该接口的权限 | 请确认是否已经开通相关权限 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 账户资金管理-二级商户预约提现

### 接口说明

请求方式：【POST】
`/v3/ecommerce/fund/withdraw`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |
| out_request_no | string(32) | true | 商户预约提现单号 |
| amount | integer | true | 提现金额，单位为分，不能超过 8 亿元 |
| remark | string(56) | false | 提现备注 |
| bank_memo | string(32) | false | 银行附言 |
| account_type | string | false | 出款账户类型，枚举值：BASIC FEES OPERATION |
| notify_url | string(256) | false | 提现结果通知地址，必须为公网可访问的安全回调地址，且不能携带 URL 参数 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |
| withdraw_id | string(128) | true | 微信支付预约提现单号 |
| out_request_no | string(32) | true | 商户预约提现单号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求不合法或权限关系不满足 | 请检查请求参数、账户类型和受理关系 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | ACCOUNT_ERROR | 二级商户未绑卡 | 绑定结算银行卡后重试 |
| 403 | ACCOUNT_NOT_VERIFIED | 二级商户下行打款未成功 | 修改结算银行卡信息后重试 |
| 403 | CONTRACT_NOT_CONFIRMED | 二级商户未开启预约提现权限 | 确认已开通预约提现权限 |
| 403 | NO_AUTH | 无接口使用权限 | 请开通商户号相关权限 |
| 403 | NOT_ENOUGH | 二级商户号账户可用余额不足 | 请确认余额后重试 |
| 403 | REQUEST_BLOCKED | 二级商户预约提现权限被冻结 | 请联系微信支付处理 |
| 404 | ORDER_NOT_EXIST | 预约提现单号不存在 | 请检查订单号是否正确 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统错误 | 请使用相同参数稍后重试 |

## 账户资金管理-二级商户查询预约提现状态（商户预约提现单号）

### 接口说明

请求方式：【GET】
`/v3/ecommerce/fund/withdraw/out-request-no/{out_request_no}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| out_request_no | string(32) | true | 商户预约提现单号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string | true | 二级商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_mchid | string | true | 收付通平台商户号 |
| sub_mchid | string | true | 二级商户号 |
| status | string | true | 预约提现单状态，枚举值：CREATE_SUCCESS SUCCESS FAIL REFUND CLOSE INIT |
| withdraw_id | string(128) | true | 微信支付预约提现单号 |
| out_request_no | string(32) | true | 商户预约提现单号 |
| amount | integer | true | 提现金额，单位为分 |
| create_time | string | true | 提交预约时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| update_time | string | true | 提现状态更新时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| reason | string(255) | true | 失败原因，仅在提现失败、退票、关单时有值 |
| remark | string(255) | true | 提现备注 |
| bank_memo | string(32) | true | 银行附言 |
| account_type | string | true | 出款账户类型，枚举值：BASIC FEES OPERATION |
| account_number | string(4) | true | 入账银行账号后四位 |
| account_bank | string(10) | true | 入账银行 |
| bank_name | string(128) | false | 入账银行全称（含支行） |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求不合法或权限关系不满足 | 请检查请求参数、账户类型和受理关系 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | ACCOUNT_ERROR | 二级商户未绑卡 | 绑定结算银行卡后重试 |
| 403 | ACCOUNT_NOT_VERIFIED | 二级商户下行打款未成功 | 修改结算银行卡信息后重试 |
| 403 | CONTRACT_NOT_CONFIRMED | 二级商户未开启预约提现权限 | 确认已开通预约提现权限 |
| 403 | NO_AUTH | 无接口使用权限 | 请开通商户号相关权限 |
| 403 | NOT_ENOUGH | 二级商户号账户可用余额不足 | 请确认余额后重试 |
| 403 | REQUEST_BLOCKED | 二级商户预约提现权限被冻结 | 请联系微信支付处理 |
| 404 | ORDER_NOT_EXIST | 预约提现单号不存在 | 请检查预约提现单号是否正确 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统错误 | 请使用相同参数稍后重试 |

## 账户资金管理-二级商户查询预约提现状态（微信支付预约提现单号）

### 接口说明

请求方式：【GET】
`/v3/ecommerce/fund/withdraw/{withdraw_id}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| withdraw_id | string(128) | true | 微信支付预约提现单号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string | true | 二级商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_mchid | string | true | 收付通平台商户号 |
| sub_mchid | string | true | 二级商户号 |
| status | string | true | 预约提现单状态，枚举值：CREATE_SUCCESS SUCCESS FAIL REFUND CLOSE INIT |
| withdraw_id | string(128) | true | 微信支付预约提现单号 |
| out_request_no | string(32) | true | 商户预约提现单号 |
| amount | integer | true | 提现金额，单位为分 |
| create_time | string | true | 提交预约时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| update_time | string | true | 提现状态更新时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| reason | string(255) | true | 失败原因，仅在提现失败、退票、关单时有值 |
| remark | string(255) | true | 提现备注 |
| bank_memo | string(32) | true | 银行附言 |
| account_type | string | true | 出款账户类型，枚举值：BASIC FEES OPERATION |
| account_number | string(4) | true | 入账银行账号后四位 |
| account_bank | string(10) | true | 入账银行 |
| bank_name | string(128) | false | 入账银行全称（含支行） |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求不合法或权限关系不满足 | 请检查请求参数、账户类型和受理关系 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | ACCOUNT_ERROR | 二级商户未绑卡 | 绑定结算银行卡后重试 |
| 403 | ACCOUNT_NOT_VERIFIED | 二级商户下行打款未成功 | 修改结算银行卡信息后重试 |
| 403 | CONTRACT_NOT_CONFIRMED | 二级商户未开启预约提现权限 | 确认已开通预约提现权限 |
| 403 | NO_AUTH | 无接口使用权限 | 请开通商户号相关权限 |
| 403 | NOT_ENOUGH | 二级商户号账户可用余额不足 | 请确认余额后重试 |
| 403 | REQUEST_BLOCKED | 二级商户预约提现权限被冻结 | 请联系微信支付处理 |
| 404 | ORDER_NOT_EXIST | 预约提现单号不存在 | 请检查预约提现单号是否正确 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统错误 | 请使用相同参数稍后重试 |

## 账户资金管理-平台预约提现

### 接口说明

请求方式：【POST】
`/v3/merchant/fund/withdraw`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| out_request_no | string(32) | true | 商户预约提现单号 |
| amount | integer | true | 提现金额，单位为分，不能超过 8 亿元 |
| remark | string(56) | false | 提现备注 |
| bank_memo | string(32) | false | 银行附言 |
| account_type | string | true | 出款账户类型，枚举值：BASIC FEES OPERATION |
| notify_url | string(256) | false | 提现结果通知地址，必须为公网可访问的安全回调地址，且不能携带 URL 参数 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| withdraw_id | string(128) | true | 微信支付预约提现单号 |
| out_request_no | string(32) | true | 商户预约提现单号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求不合法或权限关系不满足 | 请检查请求参数、账户类型和受理关系 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | ACCOUNT_ERROR | 二级商户未绑卡 | 绑定结算银行卡后重试 |
| 403 | ACCOUNT_NOT_VERIFIED | 二级商户下行打款未成功 | 修改结算银行卡信息后重试 |
| 403 | CONTRACT_NOT_CONFIRMED | 二级商户未开启预约提现权限 | 确认已开通预约提现权限 |
| 403 | NO_AUTH | 无接口使用权限 | 请开通商户号相关权限 |
| 403 | NOT_ENOUGH | 二级商户号账户可用余额不足 | 请确认余额后重试 |
| 403 | REQUEST_BLOCKED | 二级商户预约提现权限被冻结 | 请联系微信支付处理 |
| 404 | ORDER_NOT_EXIST | 预约提现单号不存在 | 请检查预约提现单号是否正确 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统错误 | 请使用相同参数稍后重试 |

## 账户资金管理-平台查询预约提现状态（商户预约提现单号）

### 接口说明

请求方式：【GET】
`/v3/merchant/fund/withdraw/out-request-no/{out_request_no}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| out_request_no | string | true | 商户预约提现单号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| status | string | true | 预约提现单状态，枚举值：CREATE_SUCCESS SUCCESS FAIL REFUND CLOSE INIT |
| withdraw_id | string(128) | true | 微信支付预约提现单号 |
| out_request_no | string(32) | true | 商户预约提现单号 |
| amount | integer | true | 提现金额，单位为分 |
| create_time | string | true | 提交预约时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| update_time | string | true | 提现状态更新时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| reason | string(255) | true | 失败原因，仅在提现失败、退票、关单时有值 |
| remark | string(255) | true | 提现备注 |
| bank_memo | string(32) | true | 银行附言 |
| account_type | string | true | 出款账户类型，枚举值：BASIC FEES OPERATION |
| solution | string(255) | true | 提现失败解决方案，仅在提现失败、退票、关单时有值 |
| account_number | string(4) | true | 入账银行账号后四位 |
| account_bank | string(10) | true | 入账银行 |
| bank_name | string(128) | false | 入账银行全称（含支行） |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求不合法或权限关系不满足 | 请检查请求参数、账户类型和受理关系 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | ACCOUNT_ERROR | 二级商户未绑卡 | 绑定结算银行卡后重试 |
| 403 | ACCOUNT_NOT_VERIFIED | 二级商户下行打款未成功 | 修改结算银行卡信息后重试 |
| 403 | CONTRACT_NOT_CONFIRMED | 二级商户未开启预约提现权限 | 确认已开通预约提现权限 |
| 403 | NO_AUTH | 无接口使用权限 | 请开通商户号相关权限 |
| 403 | NOT_ENOUGH | 二级商户号账户可用余额不足 | 请确认余额后重试 |
| 403 | REQUEST_BLOCKED | 二级商户预约提现权限被冻结 | 请联系微信支付处理 |
| 404 | ORDER_NOT_EXIST | 预约提现单号不存在 | 请检查预约提现单号是否正确 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统错误 | 请使用相同参数稍后重试 |

## 账户资金管理-平台查询预约提现状态（微信支付预约提现单号）

### 接口说明

请求方式：【GET】
`/v3/merchant/fund/withdraw/withdraw-id/{withdraw_id}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| withdraw_id | string(128) | true | 微信支付预约提现单号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| status | string | true | 预约提现单状态，枚举值：CREATE_SUCCESS SUCCESS FAIL REFUND CLOSE INIT |
| withdraw_id | string(128) | true | 微信支付预约提现单号 |
| out_request_no | string(32) | true | 商户预约提现单号 |
| amount | integer | true | 提现金额，单位为分 |
| create_time | string | true | 提交预约时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| update_time | string | true | 提现状态更新时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| reason | string(255) | true | 失败原因，仅在提现失败、退票、关单时有值 |
| remark | string(255) | true | 提现备注 |
| bank_memo | string(32) | true | 银行附言 |
| account_type | string | true | 出款账户类型，枚举值：BASIC FEES OPERATION |
| solution | string(255) | true | 提现失败解决方案，仅在提现失败、退票、关单时有值 |
| account_number | string(4) | true | 入账银行账号后四位 |
| account_bank | string(10) | true | 入账银行 |
| bank_name | string(128) | false | 入账银行全称（含支行） |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求不合法或权限关系不满足 | 请检查请求参数、账户类型和受理关系 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | ACCOUNT_ERROR | 二级商户未绑卡 | 绑定结算银行卡后重试 |
| 403 | ACCOUNT_NOT_VERIFIED | 二级商户下行打款未成功 | 修改结算银行卡信息后重试 |
| 403 | CONTRACT_NOT_CONFIRMED | 二级商户未开启预约提现权限 | 确认已开通预约提现权限 |
| 403 | NO_AUTH | 无接口使用权限 | 请开通商户号相关权限 |
| 403 | NOT_ENOUGH | 二级商户号账户可用余额不足 | 请确认余额后重试 |
| 403 | REQUEST_BLOCKED | 二级商户预约提现权限被冻结 | 请联系微信支付处理 |
| 404 | ORDER_NOT_EXIST | 预约提现单号不存在 | 请检查预约提现单号是否正确 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统错误 | 请使用相同参数稍后重试 |

## 账户资金管理-二级商户按日终余额预约提现

### 接口说明

请求方式：【POST】
`/v3/platsolution/ecommerce/withdraw/day-end-balance-withdraw`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |
| out_request_no | string(32) | true | 商户提现单号 |
| calculate_amount_type | string | true | 计算提现金额方式，枚举值：ONLY_DAY_END_BALANCE ALLOW_CURRENT_BALANCE |
| remark | string(56) | false | 提现备注 |
| bank_memo | string(32) | false | 银行附言 |
| notify_url | string(256) | false | 提现结果通知地址，必须为公网可访问的安全回调地址，且不能携带 URL 参数 |
| reserve_amount | integer | false | 留存额，单位为分 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_mchid | string | true | 收付通平台商户号 |
| sub_mchid | string | true | 二级商户号 |
| status | string | true | 单据状态，枚举值：CREATED PROCESSING FINISHED ABNORMAL |
| withdraw_id | string(64) | true | 微信支付提现单号 |
| out_request_no | string(32) | true | 商户提现单号 |
| total_amount | integer | true | 提现金额，单位为分 |
| success_amount | integer | false | 提现成功金额，单位为分 |
| fail_amount | integer | false | 提现失败金额，单位为分 |
| refund_amount | integer | false | 提现退票金额，单位为分 |
| create_time | string | true | 提交预约时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| update_time | string | true | 状态更新时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| reason | string(255) | false | 失败原因 |
| remark | string(255) | false | 提现备注 |
| bank_memo | string(32) | false | 银行附言 |
| account_type | string | true | 出款账户类型，枚举值：BASIC FEES OPERATION |
| account_number | string(4) | true | 入账银行账号后四位 |
| account_bank | string(10) | true | 入账银行 |
| bank_name | string(128) | false | 入账银行全称（含支行） |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求不合法或权限关系不满足 | 请检查请求参数、账户类型和受理关系 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | ACCOUNT_ERROR | 二级商户未绑卡 | 绑定结算银行卡后重试 |
| 403 | ACCOUNT_NOT_VERIFIED | 二级商户下行打款未成功 | 修改结算银行卡信息后重试 |
| 403 | CONTRACT_NOT_CONFIRMED | 二级商户未开启预约提现权限 | 确认已开通预约提现权限 |
| 403 | NO_AUTH | 无接口使用权限 | 请开通商户号相关权限 |
| 403 | NOT_ENOUGH | 二级商户号账户可用余额不足 | 请确认余额后重试 |
| 403 | REQUEST_BLOCKED | 二级商户预约提现权限被冻结 | 请联系微信支付处理 |
| 404 | ORDER_NOT_EXIST | 预约提现单号不存在 | 请检查订单号是否正确 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统错误 | 请使用相同参数稍后重试 |

## 账户资金管理-查询二级商户按日终余额预约提现状态

### 接口说明

请求方式：【GET】
`/v3/platsolution/ecommerce/withdraw/day-end-balance-withdraw/out-request-no/{out_request_no}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| out_request_no | string | true | 商户提现单号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string | true | 二级商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_mchid | string | true | 收付通平台商户号 |
| sub_mchid | string | true | 二级商户号 |
| status | string | true | 单据状态，枚举值：CREATED PROCESSING FINISHED ABNORMAL |
| withdraw_id | string(64) | true | 微信支付提现单号 |
| out_request_no | string(32) | true | 商户提现单号 |
| total_amount | integer | true | 提现金额，单位为分 |
| success_amount | integer | false | 提现成功金额，单位为分 |
| fail_amount | integer | false | 提现失败金额，单位为分 |
| refund_amount | integer | false | 提现退票金额，单位为分 |
| create_time | string | true | 提交预约时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| update_time | string | true | 状态更新时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| reason | string(255) | false | 失败原因 |
| remark | string(255) | false | 提现备注 |
| bank_memo | string(32) | false | 银行附言 |
| account_type | string | true | 出款账户类型，枚举值：BASIC FEES OPERATION |
| account_number | string(4) | true | 入账银行账号后四位 |
| account_bank | string(10) | true | 入账银行 |
| bank_name | string(128) | false | 入账银行全称（含支行） |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求不合法或权限关系不满足 | 请检查请求参数、账户类型和受理关系 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | ACCOUNT_ERROR | 二级商户未绑卡 | 绑定结算银行卡后重试 |
| 403 | ACCOUNT_NOT_VERIFIED | 二级商户下行打款未成功 | 修改结算银行卡信息后重试 |
| 403 | CONTRACT_NOT_CONFIRMED | 二级商户未开启预约提现权限 | 确认已开通预约提现权限 |
| 403 | NO_AUTH | 无接口使用权限 | 请开通商户号相关权限 |
| 403 | NOT_ENOUGH | 二级商户号账户可用余额不足 | 请确认余额后重试 |
| 403 | REQUEST_BLOCKED | 二级商户预约提现权限被冻结 | 请联系微信支付处理 |
| 404 | ORDER_NOT_EXIST | 预约提现单号不存在 | 请检查预约提现单号是否正确 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统错误 | 请使用相同参数稍后重试 |

## 账户资金管理-按日下载提现异常文件

### 接口说明

请求方式：【GET】
`/v3/merchant/fund/withdraw/bill-type/{bill_type}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| bill_type | string | true | 账单类型，枚举值：NO_SUCC |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| bill_date | string(10) | true | 账单日期，格式 YYYY-MM-DD |
| tar_type | string | false | 压缩格式，枚举值：GZIP |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| hash_type | string | true | 哈希类型，枚举值：SHA1 |
| hash_value | string | true | 哈希值 |
| download_url | string | true | 下载地址 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求的账单日期已过期 | 请检查 bill_date 并重新调用 |
| 400 | NO_STATEMENT_EXIST | 请求的账单文件不存在 | 请检查指定日期是否有资金操作 |
| 400 | STATEMENT_CREATING | 请求的账单正在生成中 | 请在 T+1 上午 10 点后重试 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 403 | NO_AUTH | 当前商户号没有使用该接口的权限 | 请确认是否已经开通相关权限 |
| 500 | SYSTEM_ERROR | 系统错误 | 请使用相同参数稍后重新调用 |

## 账户资金管理-商户提现状态变更通知

### 接口说明

请求方式：【POST】
`/v1/webhooks/wechat-ecommerce/withdraw-notify`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_mchid | string | false | 收付通平台商户号 |
| sub_mchid | string | false | 二级商户号 |
| status | string | true | 提现状态，枚举值：CREATE_SUCCESS SUCCESS FAIL REFUND CLOSE INIT CREATED PROCESSING FINISHED ABNORMAL |
| withdraw_id | string | true | 微信支付提现单号 |
| out_request_no | string | true | 商户提现单号 |
| amount | integer | false | 提现金额，单位为分 |
| total_amount | integer | false | 提现金额，单位为分 |
| success_amount | integer | false | 提现成功金额，单位为分 |
| fail_amount | integer | false | 提现失败金额，单位为分 |
| refund_amount | integer | false | 提现退票金额，单位为分 |
| create_time | string | true | 提交预约时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| update_time | string | true | 状态更新时间，时间格式为 yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| reason | string | false | 失败原因 |
| remark | string | false | 提现备注 |
| bank_memo | string | false | 银行附言 |
| account_type | string | true | 出款账户类型，枚举值：BASIC FEES OPERATION |
| account_number | string | true | 入账银行账号后四位 |
| account_bank | string | true | 入账银行 |
| bank_name | string | false | 入账银行全称（含支行） |
