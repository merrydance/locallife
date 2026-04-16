# 消费者投诉 2.0 组文档样例

本文件基于微信支付服务商消费者投诉 2.0 官方文档摘录，保留 contracts、errorcodes 与 alignment audit 所需的接口、字段、枚举和语义约束。

来源页面：

- 查询投诉单列表：https://pay.weixin.qq.com/doc/v3/partner/4012691285.md
- 查询投诉单详情：https://pay.weixin.qq.com/doc/v3/partner/4012691648.md
- 查询投诉单协商历史：https://pay.weixin.qq.com/doc/v3/partner/4012691802.md
- 投诉通知回调：https://pay.weixin.qq.com/doc/v3/partner/4012076174.md
- 创建投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012458106.md
- 查询投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012459065.md
- 更新投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012459287.md
- 删除投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012460474.md
- 回复用户：https://pay.weixin.qq.com/doc/v3/partner/4012467213.md
- 反馈处理完成：https://pay.weixin.qq.com/doc/v3/partner/4012467217.md
- 更新退款审批结果：https://pay.weixin.qq.com/doc/v3/partner/4012467218.md
- 回复需要即时服务的投诉单：https://pay.weixin.qq.com/doc/v3/partner/4017151726.md
- 图片上传接口：https://pay.weixin.qq.com/doc/v3/partner/4012467222.md
- 图片请求接口：https://pay.weixin.qq.com/doc/v3/partner/4012467223.md

## 消费者投诉 2.0-查询投诉单列表

### 接口说明

请求方式：【GET】
`/v3/merchant-service/complaints-v2`

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| limit | integer | false | 分页大小，范围[1,50] |
| offset | integer | false | 分页开始位置，从0开始计数 |
| begin_date | string(10) | true | 投诉发生的开始日期，格式YYYY-MM-DD |
| end_date | string(10) | true | 投诉发生的结束日期，格式YYYY-MM-DD |
| complainted_mchid | string(64) | false | 被诉商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| data | array[object] | false | 用户投诉信息详情 |
| data.complaint_id | string(64) | true | 投诉单号 |
| data.complaint_time | string(32) | true | 投诉时间，遵循RFC3339标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| data.complaint_detail | string(300) | true | 投诉详情 |
| data.complaint_state | string(30) | true | 投诉单状态，枚举值：PENDING PROCESSING PROCESSED |
| data.complainted_mchid | string(64) | true | 被诉商户号 |
| data.payer_phone | string(512) | false | 投诉人联系方式 |
| data.complaint_order_info | array[object] | false | 投诉单关联订单信息 |
| data.complaint_order_info.transaction_id | string(64) | true | 微信订单号 |
| data.complaint_order_info.out_trade_no | string(64) | true | 商户订单号 |
| data.complaint_order_info.amount | integer | true | 订单金额，单位（分） |
| data.complaint_full_refunded | boolean | true | 投诉单是否已全额退款 |
| data.incoming_user_response | boolean | true | 是否有待回复的用户留言 |
| data.user_complaint_times | integer | true | 用户投诉次数 |
| data.complaint_media_list | array[object] | false | 投诉资料列表 |
| data.complaint_media_list.media_type | string | true | 媒体文件业务类型，枚举值：USER_COMPLAINT_IMAGE OPERATION_IMAGE |
| data.complaint_media_list.media_url | array[string(512)] | true | 媒体文件请求url |
| data.problem_description | string(256) | true | 问题描述 |
| data.problem_type | string | false | 问题类型，枚举值：REFUND SERVICE_NOT_WORK OTHERS |
| data.apply_refund_amount | integer | false | 申请退款金额，单位（分） |
| data.user_tag_list | array[string] | false | 用户标签列表，枚举值：TRUSTED HIGH_RISK |
| data.service_order_info | array[object] | false | 投诉单关联服务单信息 |
| data.service_order_info.order_id | string(128) | false | 微信支付服务订单号 |
| data.service_order_info.out_order_no | string(128) | false | 商户服务订单号 |
| data.service_order_info.state | string | false | 支付分服务单状态，枚举值：DOING REVOKED WAITPAY DONE |
| data.additional_info | object | false | 补充信息 |
| data.additional_info.type | string | false | 补充信息类型，枚举值：SHARE_POWER_TYPE |
| data.additional_info.share_power_info | object | false | 充电宝投诉相关信息 |
| data.additional_info.share_power_info.return_time | string | false | 归还时间，遵循RFC3339标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| data.additional_info.share_power_info.return_address_info | object | false | 归还地点信息 |
| data.additional_info.share_power_info.return_address_info.return_address | string(256) | false | 归还地点 |
| data.additional_info.share_power_info.return_address_info.longitude | string(32) | false | 归还地点经度 |
| data.additional_info.share_power_info.return_address_info.latitude | string(32) | false | 归还地点纬度 |
| data.additional_info.share_power_info.is_returned_to_same_machine | boolean | false | 是否归还同一柜机 |
| data.in_platform_service | boolean | false | 是否在平台协助中 |
| data.need_immediate_service | boolean | false | 是否需即时服务用户 |
| data.is_agent_mode | boolean | false | 是否是智能体投诉 |
| limit | integer | true | 分页大小 |
| offset | integer | true | 分页开始位置 |
| total_count | integer | false | 投诉总条数 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-查询投诉单详情

### 接口说明

请求方式：【GET】
`/v3/merchant-service/complaints-v2/{complaint_id}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| complaint_id | string(64) | true | 投诉单号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| complaint_id | string(64) | true | 投诉单号 |
| complaint_time | string(32) | true | 投诉时间，遵循RFC3339标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| complaint_detail | string(300) | true | 投诉详情 |
| complaint_state | string(30) | true | 投诉单状态，枚举值：PENDING PROCESSING PROCESSED |
| complainted_mchid | string(64) | true | 被诉商户号 |
| payer_phone | string(512) | false | 投诉人联系方式 |
| payer_openid | string(128) | false | 投诉人OpenID |
| complaint_order_info | array[object] | false | 投诉单关联订单信息 |
| complaint_order_info.transaction_id | string(64) | true | 微信订单号 |
| complaint_order_info.out_trade_no | string(64) | true | 商户订单号 |
| complaint_order_info.amount | integer | true | 订单金额，单位（分） |
| complaint_full_refunded | boolean | true | 投诉单是否已全额退款 |
| incoming_user_response | boolean | true | 是否有待回复的用户留言 |
| user_complaint_times | integer | true | 用户投诉次数 |
| complaint_media_list | array[object] | false | 投诉资料列表 |
| complaint_media_list.media_type | string | true | 媒体文件业务类型，枚举值：USER_COMPLAINT_IMAGE OPERATION_IMAGE |
| complaint_media_list.media_url | array[string(512)] | true | 媒体文件请求url |
| problem_description | string(256) | true | 问题描述 |
| problem_type | string | false | 问题类型，枚举值：REFUND SERVICE_NOT_WORK OTHERS |
| apply_refund_amount | integer | false | 申请退款金额，单位（分） |
| user_tag_list | array[string] | false | 用户标签列表，枚举值：TRUSTED HIGH_RISK |
| service_order_info | array[object] | false | 投诉单关联服务单信息 |
| service_order_info.order_id | string(128) | false | 微信支付服务订单号 |
| service_order_info.out_order_no | string(128) | false | 商户服务订单号 |
| service_order_info.state | string | false | 支付分服务单状态，枚举值：DOING REVOKED WAITPAY DONE |
| additional_info | object | false | 补充信息 |
| additional_info.type | string | false | 补充信息类型，枚举值：SHARE_POWER_TYPE |
| additional_info.share_power_info | object | false | 充电宝投诉相关信息 |
| additional_info.share_power_info.return_time | string | false | 归还时间，遵循RFC3339标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| additional_info.share_power_info.return_address_info | object | false | 归还地点信息 |
| additional_info.share_power_info.return_address_info.return_address | string(256) | false | 归还地点 |
| additional_info.share_power_info.return_address_info.longitude | string(32) | false | 归还地点经度 |
| additional_info.share_power_info.return_address_info.latitude | string(32) | false | 归还地点纬度 |
| additional_info.share_power_info.is_returned_to_same_machine | boolean | false | 是否归还同一柜机 |
| in_platform_service | boolean | false | 是否在平台协助中 |
| need_immediate_service | boolean | false | 是否需即时服务用户 |
| is_agent_mode | boolean | false | 是否是智能体投诉 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-查询投诉单协商历史

### 接口说明

请求方式：【GET】
`/v3/merchant-service/complaints-v2/{complaint_id}/negotiation-historys`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| complaint_id | string(64) | true | 投诉单号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| limit | integer | false | 分页大小，范围[1,300] |
| offset | integer | false | 分页开始位置，从0开始计数 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| data | array[object] | false | 投诉协商历史 |
| data.log_id | string(64) | true | 操作流水号 |
| data.operator | string(64) | true | 操作人 |
| data.operate_time | string(64) | true | 操作时间，遵循RFC3339标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| data.operate_type | string | true | 操作类型，枚举值：USER_CREATE_COMPLAINT USER_CONTINUE_COMPLAINT USER_RESPONSE PLATFORM_RESPONSE MERCHANT_RESPONSE MERCHANT_CONFIRM_COMPLETE USER_CREATE_COMPLAINT_SYSTEM_MESSAGE COMPLAINT_FULL_REFUNDED_SYSTEM_MESSAGE USER_CONTINUE_COMPLAINT_SYSTEM_MESSAGE USER_REVOKE_COMPLAINT USER_COMFIRM_COMPLAINT PLATFORM_HELP_APPLICATION USER_APPLY_PLATFORM_HELP MERCHANT_APPROVE_REFUND MERCHANT_REFUSE_RERUND USER_SUBMIT_SATISFACTION SERVICE_ORDER_CANCEL SERVICE_ORDER_COMPLETE COMPLAINT_PARTIAL_REFUNDED_SYSTEM_MESSAGE COMPLAINT_REFUND_RECEIVED_SYSTEM_MESSAGE COMPLAINT_ENTRUSTED_REFUND_SYSTEM_MESSAGE USER_APPLY_PLATFORM_SERVICE USER_CANCEL_PLATFORM_SERVICE PLATFORM_SERVICE_FINISHED USER_CLICK_RESPONSE |
| data.operate_details | string(500) | false | 操作内容 |
| data.image_list | array[string] | false | 图片凭证 |
| data.complaint_media_list | object | false | 操作资料列表 |
| data.complaint_media_list.media_type | string | true | 媒体文件业务类型，枚举值：USER_COMPLAINT_IMAGE OPERATION_IMAGE |
| data.complaint_media_list.media_url | array[string(512)] | true | 媒体文件请求url |
| data.user_appy_platform_service_reason | string | false | 用户申请平台协助原因 |
| data.user_appy_platform_service_reason_description | string | false | 用户申请平台协助原因描述 |
| data.normal_message | object | false | 普通消息内容 |
| data.normal_message.blocks | array[object] | false | 消息内容块列表 |
| data.normal_message.blocks.type | string | false | 消息块类型，枚举值：TEXT IMAGE LINK FAQ_LIST BUTTON BUTTON_GROUP |
| data.normal_message.blocks.text | object | false | 文本 |
| data.normal_message.blocks.text.text | string | false | 文字内容 |
| data.normal_message.blocks.text.color | string | false | 文本颜色，枚举值：DEFAULT SECONDARY |
| data.normal_message.blocks.text.is_bold | boolean | false | 是否粗体 |
| data.normal_message.blocks.image | object | false | 图片 |
| data.normal_message.blocks.image.media_id | string | false | 媒体ID |
| data.normal_message.blocks.image.image_style_type | string | false | 图片样式类型，枚举值：IMAGE_STYLE_TYPE_NARROW IMAGE_STYLE_TYPE_WIDE |
| data.normal_message.blocks.link | object | false | 链接 |
| data.normal_message.blocks.link.text | string | false | 链接文案 |
| data.normal_message.blocks.link.action | object | false | 动作 |
| data.normal_message.blocks.link.action.action_type | string | false | 动作类型，枚举值：ACTION_TYPE_SEND_MESSAGE ACTION_TYPE_JUMP_URL ACTION_TYPE_JUMP_MINI_PROGRAM |
| data.normal_message.blocks.link.action.jump_url | string | false | 跳转链接 |
| data.normal_message.blocks.link.action.mini_program_jump_info | object | false | 跳转的小程序 |
| data.normal_message.blocks.link.action.mini_program_jump_info.appid | string | false | 小程序appid |
| data.normal_message.blocks.link.action.mini_program_jump_info.path | string | false | 小程序path |
| data.normal_message.blocks.link.action.message_info | object | false | 回复消息内容 |
| data.normal_message.blocks.link.action.message_info.content | string | true | 回复的消息内容 |
| data.normal_message.blocks.link.action.message_info.custom_data | string | false | 自定义透传字段 |
| data.normal_message.blocks.link.action.action_id | string | false | 动作id |
| data.normal_message.blocks.link.invalid_info | object | false | 失效配置 |
| data.normal_message.blocks.link.invalid_info.expired_time | string | false | 过期时间，遵循RFC3339标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| data.normal_message.blocks.link.invalid_info.multi_clickable | boolean | false | 是否可以多次点击 |
| data.normal_message.blocks.faq_list | object | false | FAQ列表 |
| data.normal_message.blocks.faq_list.faqs | array[object] | false | FAQ列表 |
| data.normal_message.blocks.faq_list.faqs.faq_id | string | false | faq的id |
| data.normal_message.blocks.faq_list.faqs.faq_title | string | false | faq内容 |
| data.normal_message.blocks.faq_list.faqs.action | object | false | 动作 |
| data.normal_message.blocks.faq_list.faqs.action.action_type | string | false | 动作类型，枚举值：ACTION_TYPE_SEND_MESSAGE ACTION_TYPE_JUMP_URL ACTION_TYPE_JUMP_MINI_PROGRAM |
| data.normal_message.blocks.faq_list.faqs.action.jump_url | string | false | 跳转链接 |
| data.normal_message.blocks.faq_list.faqs.action.mini_program_jump_info | object | false | 跳转的小程序 |
| data.normal_message.blocks.faq_list.faqs.action.mini_program_jump_info.appid | string | false | 小程序appid |
| data.normal_message.blocks.faq_list.faqs.action.mini_program_jump_info.path | string | false | 小程序path |
| data.normal_message.blocks.faq_list.faqs.action.message_info | object | false | 回复消息内容 |
| data.normal_message.blocks.faq_list.faqs.action.message_info.content | string | true | 回复的消息内容 |
| data.normal_message.blocks.faq_list.faqs.action.message_info.custom_data | string | false | 自定义透传字段 |
| data.normal_message.blocks.faq_list.faqs.action.action_id | string | false | 动作id |
| data.normal_message.blocks.button | object | false | 按钮 |
| data.normal_message.blocks.button.text | string | false | 按钮文本 |
| data.normal_message.blocks.button.action | object | false | 动作 |
| data.normal_message.blocks.button.action.action_type | string | false | 动作类型，枚举值：ACTION_TYPE_SEND_MESSAGE ACTION_TYPE_JUMP_URL ACTION_TYPE_JUMP_MINI_PROGRAM |
| data.normal_message.blocks.button.action.jump_url | string | false | 跳转链接 |
| data.normal_message.blocks.button.action.mini_program_jump_info | object | false | 跳转的小程序 |
| data.normal_message.blocks.button.action.mini_program_jump_info.appid | string | false | 小程序appid |
| data.normal_message.blocks.button.action.mini_program_jump_info.path | string | false | 小程序path |
| data.normal_message.blocks.button.action.message_info | object | false | 回复消息内容 |
| data.normal_message.blocks.button.action.message_info.content | string | true | 回复的消息内容 |
| data.normal_message.blocks.button.action.message_info.custom_data | string | false | 自定义透传字段 |
| data.normal_message.blocks.button.action.action_id | string | false | 动作id |
| data.normal_message.blocks.button.invalid_info | object | false | 失效配置 |
| data.normal_message.blocks.button.invalid_info.expired_time | string | false | 过期时间，遵循RFC3339标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| data.normal_message.blocks.button.invalid_info.multi_clickable | boolean | false | 是否可以多次点击 |
| data.normal_message.blocks.button_group | object | false | 按钮组 |
| data.normal_message.blocks.button_group.buttons | array[object] | false | 按钮组 |
| data.normal_message.blocks.button_group.buttons.text | string | false | 按钮文本 |
| data.normal_message.blocks.button_group.buttons.action | object | false | 动作 |
| data.normal_message.blocks.button_group.buttons.action.action_type | string | false | 动作类型，枚举值：ACTION_TYPE_SEND_MESSAGE ACTION_TYPE_JUMP_URL ACTION_TYPE_JUMP_MINI_PROGRAM |
| data.normal_message.blocks.button_group.buttons.action.jump_url | string | false | 跳转链接 |
| data.normal_message.blocks.button_group.buttons.action.mini_program_jump_info | object | false | 跳转的小程序 |
| data.normal_message.blocks.button_group.buttons.action.mini_program_jump_info.appid | string | false | 小程序appid |
| data.normal_message.blocks.button_group.buttons.action.mini_program_jump_info.path | string | false | 小程序path |
| data.normal_message.blocks.button_group.buttons.action.message_info | object | false | 回复消息内容 |
| data.normal_message.blocks.button_group.buttons.action.message_info.content | string | true | 回复的消息内容 |
| data.normal_message.blocks.button_group.buttons.action.message_info.custom_data | string | false | 自定义透传字段 |
| data.normal_message.blocks.button_group.buttons.action.action_id | string | false | 动作id |
| data.normal_message.blocks.button_group.button_layout | string | false | 按钮布局方式，枚举值：LAYOUT_UNKNOWN LAYOUT_HORIZONTAL LAYOUT_VERTICAL |
| data.normal_message.blocks.button_group.invalid_info | object | false | 失效配置 |
| data.normal_message.blocks.button_group.invalid_info.expired_time | string | false | 过期时间，遵循RFC3339标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| data.normal_message.blocks.button_group.invalid_info.multi_clickable | boolean | false | 是否可以多次点击 |
| data.normal_message.sender_identity | string | false | 发送者身份类别，枚举值：UNKNOWN MANUAL MACHINE |
| data.normal_message.custom_data | string(4096) | false | 自定义透传信息 |
| data.click_message | object | false | 用户点击回复消息 |
| data.click_message.message_content | string(1024) | false | 回复的消息内容 |
| data.click_message.action_id | string(4096) | false | 动作id |
| data.click_message.clicked_log_id | string(128) | false | 被点击的操作流水号 |
| limit | integer | true | 分页大小 |
| offset | integer | true | 分页开始位置 |
| total_count | integer | false | 投诉协商历史总条数 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-投诉通知回调

### 接口说明

请求方式：【POST】
`/v1/webhooks/wechat-ecommerce/complaint-notify`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| complaint_id | string(64) | true | 投诉单号 |
| action_type | string(64) | true | 动作类型，枚举值：CREATE_COMPLAINT CONTINUE_COMPLAINT USER_RESPONSE RESPONSE_BY_PLATFORM SELLER_REFUND MERCHANT_RESPONSE MERCHANT_CONFIRM_COMPLETE USER_APPLY_PLATFORM_SERVICE USER_CANCEL_PLATFORM_SERVICE PLATFORM_SERVICE_FINISHED MERCHANT_APPROVE_REFUND MERCHANT_REJECT_REFUND REFUND_SUCCESS |

## 消费者投诉 2.0-创建投诉通知回调地址

### 接口说明

请求方式：【POST】
`/v3/merchant-service/complaint-notifications`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| url | string(255) | true | 通知地址，仅支持HTTPS |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| mchid | string(64) | true | 商户号 |
| url | string(255) | true | 通知地址 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-查询投诉通知回调地址

### 接口说明

请求方式：【GET】
`/v3/merchant-service/complaint-notifications`

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| mchid | string(64) | true | 商户号 |
| url | string(255) | true | 通知地址 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-更新投诉通知回调地址

### 接口说明

请求方式：【PUT】
`/v3/merchant-service/complaint-notifications`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| url | string(255) | true | 通知地址，仅支持HTTPS |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| mchid | string(64) | true | 商户号 |
| url | string(255) | true | 通知地址 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-删除投诉通知回调地址

### 接口说明

请求方式：【DELETE】
`/v3/merchant-service/complaint-notifications`

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-回复用户

### 接口说明

请求方式：【POST】
`/v3/merchant-service/complaints-v2/{complaint_id}/response`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| complaint_id | string(64) | true | 投诉单号 |

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| complainted_mchid | string(64) | true | 被诉商户号 |
| response_content | string(500) | true | 回复内容 |
| response_images | array[string] | false | 回复图片，最多上传4张图片凭证 |
| jump_url | string(512) | false | 跳转链接，需满足HTTPS格式 |
| jump_url_text | string(10) | false | 跳转链接文案，当传入jump_url时必填 |
| mini_program_jump_info | object | false | 跳转小程序信息 |
| mini_program_jump_info.appid | string | true | 跳转小程序APPID |
| mini_program_jump_info.path | string | true | 跳转小程序页面PATH |
| mini_program_jump_info.text | string | true | 跳转小程序页面名称 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-反馈处理完成

### 接口说明

请求方式：【POST】
`/v3/merchant-service/complaints-v2/{complaint_id}/complete`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| complaint_id | string(64) | true | 投诉单号 |

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| complainted_mchid | string(64) | true | 被诉商户号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-更新退款审批结果

### 接口说明

请求方式：【POST】
`/v3/merchant-service/complaints-v2/{complaint_id}/update-refund-progress`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| complaint_id | string(64) | true | 投诉单号 |

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| action | string | true | 审批动作，枚举值：REJECT APPROVE |
| launch_refund_day | integer | false | 预计发起退款时间，在同意退款时返回 |
| reject_reason | string(200) | false | 拒绝退款原因 |
| reject_media_list | array[string] | false | 拒绝退款的举证图片列表 |
| remark | string(200) | false | 备注 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-回复需要即时服务的投诉单

### 接口说明

请求方式：【POST】
`/v3/merchant-service/complaints-v2/{complaint_id}/response-immediate-service`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| complaint_id | string(64) | true | 投诉单号 |

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| complainted_mchid | string(64) | true | 被诉商户号 |
| message | object | true | 消息内容 |
| message.blocks | array[object] | false | 消息内容块列表 |
| message.blocks.type | string | false | 消息块类型，枚举值：TEXT IMAGE LINK FAQ_LIST BUTTON BUTTON_GROUP |
| message.blocks.text | object | false | 文本 |
| message.blocks.text.text | string | false | 文字内容 |
| message.blocks.text.color | string | false | 文本颜色，枚举值：DEFAULT SECONDARY |
| message.blocks.text.is_bold | boolean | false | 是否粗体 |
| message.blocks.image | object | false | 图片 |
| message.blocks.image.media_id | string | false | 媒体ID |
| message.blocks.image.image_style_type | string | false | 图片样式类型，枚举值：IMAGE_STYLE_TYPE_NARROW IMAGE_STYLE_TYPE_WIDE |
| message.blocks.link | object | false | 链接 |
| message.blocks.link.text | string | false | 链接文案 |
| message.blocks.link.action | object | false | 动作 |
| message.blocks.link.action.action_type | string | false | 动作类型，枚举值：ACTION_TYPE_SEND_MESSAGE ACTION_TYPE_JUMP_URL ACTION_TYPE_JUMP_MINI_PROGRAM |
| message.blocks.link.action.jump_url | string | false | 跳转链接 |
| message.blocks.link.action.mini_program_jump_info | object | false | 跳转的小程序 |
| message.blocks.link.action.mini_program_jump_info.appid | string | false | 小程序appid |
| message.blocks.link.action.mini_program_jump_info.path | string | false | 小程序path |
| message.blocks.link.action.message_info | object | false | 回复消息内容 |
| message.blocks.link.action.message_info.content | string | true | 回复的消息内容 |
| message.blocks.link.action.message_info.custom_data | string | false | 自定义透传字段 |
| message.blocks.link.action.action_id | string | false | 动作id |
| message.blocks.link.invalid_info | object | false | 失效配置 |
| message.blocks.link.invalid_info.expired_time | string | false | 过期时间，遵循RFC3339标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| message.blocks.link.invalid_info.multi_clickable | boolean | false | 是否可以多次点击 |
| message.blocks.faq_list | object | false | FAQ列表 |
| message.blocks.faq_list.faqs | array[object] | false | FAQ列表 |
| message.blocks.faq_list.faqs.faq_id | string | false | faq的id |
| message.blocks.faq_list.faqs.faq_title | string | false | faq内容 |
| message.blocks.faq_list.faqs.action | object | false | 动作 |
| message.blocks.faq_list.faqs.action.action_type | string | false | 动作类型，枚举值：ACTION_TYPE_SEND_MESSAGE ACTION_TYPE_JUMP_URL ACTION_TYPE_JUMP_MINI_PROGRAM |
| message.blocks.faq_list.faqs.action.jump_url | string | false | 跳转链接 |
| message.blocks.faq_list.faqs.action.mini_program_jump_info | object | false | 跳转的小程序 |
| message.blocks.faq_list.faqs.action.mini_program_jump_info.appid | string | false | 小程序appid |
| message.blocks.faq_list.faqs.action.mini_program_jump_info.path | string | false | 小程序path |
| message.blocks.faq_list.faqs.action.message_info | object | false | 回复消息内容 |
| message.blocks.faq_list.faqs.action.message_info.content | string | true | 回复的消息内容 |
| message.blocks.faq_list.faqs.action.message_info.custom_data | string | false | 自定义透传字段 |
| message.blocks.faq_list.faqs.action.action_id | string | false | 动作id |
| message.blocks.button | object | false | 按钮 |
| message.blocks.button.text | string | false | 按钮文本 |
| message.blocks.button.action | object | false | 动作 |
| message.blocks.button.action.action_type | string | false | 动作类型，枚举值：ACTION_TYPE_SEND_MESSAGE ACTION_TYPE_JUMP_URL ACTION_TYPE_JUMP_MINI_PROGRAM |
| message.blocks.button.action.jump_url | string | false | 跳转链接 |
| message.blocks.button.action.mini_program_jump_info | object | false | 跳转的小程序 |
| message.blocks.button.action.mini_program_jump_info.appid | string | false | 小程序appid |
| message.blocks.button.action.mini_program_jump_info.path | string | false | 小程序path |
| message.blocks.button.action.message_info | object | false | 回复消息内容 |
| message.blocks.button.action.message_info.content | string | true | 回复的消息内容 |
| message.blocks.button.action.message_info.custom_data | string | false | 自定义透传字段 |
| message.blocks.button.action.action_id | string | false | 动作id |
| message.blocks.button.invalid_info | object | false | 失效配置 |
| message.blocks.button.invalid_info.expired_time | string | false | 过期时间，遵循RFC3339标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| message.blocks.button.invalid_info.multi_clickable | boolean | false | 是否可以多次点击 |
| message.blocks.button_group | object | false | 按钮组 |
| message.blocks.button_group.buttons | array[object] | false | 按钮组 |
| message.blocks.button_group.buttons.text | string | false | 按钮文本 |
| message.blocks.button_group.buttons.action | object | false | 动作 |
| message.blocks.button_group.buttons.action.action_type | string | false | 动作类型，枚举值：ACTION_TYPE_SEND_MESSAGE ACTION_TYPE_JUMP_URL ACTION_TYPE_JUMP_MINI_PROGRAM |
| message.blocks.button_group.buttons.action.jump_url | string | false | 跳转链接 |
| message.blocks.button_group.buttons.action.mini_program_jump_info | object | false | 跳转的小程序 |
| message.blocks.button_group.buttons.action.mini_program_jump_info.appid | string | false | 小程序appid |
| message.blocks.button_group.buttons.action.mini_program_jump_info.path | string | false | 小程序path |
| message.blocks.button_group.buttons.action.message_info | object | false | 回复消息内容 |
| message.blocks.button_group.buttons.action.message_info.content | string | true | 回复的消息内容 |
| message.blocks.button_group.buttons.action.message_info.custom_data | string | false | 自定义透传字段 |
| message.blocks.button_group.buttons.action.action_id | string | false | 动作id |
| message.blocks.button_group.button_layout | string | false | 按钮布局方式，枚举值：LAYOUT_UNKNOWN LAYOUT_HORIZONTAL LAYOUT_VERTICAL |
| message.blocks.button_group.invalid_info | object | false | 失效配置 |
| message.blocks.button_group.invalid_info.expired_time | string | false | 过期时间，遵循RFC3339标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| message.blocks.button_group.invalid_info.multi_clickable | boolean | false | 是否可以多次点击 |
| message.sender_identity | string | false | 发送者身份类别，枚举值：UNKNOWN MANUAL MACHINE |
| message.custom_data | string(4096) | false | 自定义透传信息 |
| idempotent_id | string(128) | true | 幂等ID |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| log_id | string(64) | true | 操作流水号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-图片上传接口

### 接口说明

请求方式：【POST】
`/v3/merchant-service/images/upload`

### Form 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| file | file | true | 图片文件，只支持jpg、bmp、png格式，文件大小不能超过2M |
| meta | object | true | 媒体文件元信息 |
| meta.filename | string(128) | true | 文件名称，必须以JPG、BMP、PNG为后缀 |
| meta.sha256 | string(256) | true | 文件摘要 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| media_id | string(512) | true | 媒体文件标识Id |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 消费者投诉 2.0-图片请求接口

### 接口说明

请求方式：【GET】
`/v3/merchant-service/images/{media_id}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| media_id | string(256) | true | 媒体文件标识ID |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |