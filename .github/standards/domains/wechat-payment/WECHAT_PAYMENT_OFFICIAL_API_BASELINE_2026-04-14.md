# 微信支付官方文档与接口对齐基线

## 1. 目的

本文是 LocalLife 在微信支付与平台收付通相关开发、审查、排障中的长期有效基线。

它约束两件事：

1. 官方文档是微信支付接口行为的唯一事实来源。
2. 代码、日志、错误映射、状态处理、回调处理、运行手册必须与官方文档和当前系统持久化真相保持一致。

本文不是运行手册；生产恢复、告警与人工介入流程继续以运行手册为准。本文重点约束“开发和审查时必须先核实什么，必须严格对齐什么”。

## 2. 不可降级规则

所有微信支付、平台收付通、投诉、分账、退款、提现、进件、结算账户、账单、图片上传相关接口，必须遵守以下规则：

- 不允许凭记忆、感觉、旧代码样例、历史截图或模糊印象实现接口；必须先查对应官方文档。
- 必须核实接口用途，确认当前场景应该调用哪一个接口，而不是“看起来差不多就复用”。
- 请求结构与响应结构必须与官方文档百分百匹配。
- 必填字段、条件必填字段、字段类型、字段含义、枚举值、状态值、状态流转、错误码都必须逐项核对，不允许省略或降级处理。
- 不允许把“文档写了条件必填”实现成“本地默认补值”或“静默跳过”。
- 不允许把未知状态、未知枚举、未知错误码当成成功或无影响分支吞掉。
- 所有微信支付错误必须记录结构化日志，并保留足够的上下文标识用于排障。
- 所有对上层调用方或前端返回的错误，必须提供语义清晰的失败说明或下一步引导，不能只透传模糊原文或空泛失败信息。
- 若本地状态、持久化记录、异步任务推进结果与微信返回语义存在冲突，不得乐观报成功，必须先厘清最终状态。

建议开发和审查时逐项核对：

- 接口适用产品与调用方身份
- 请求路径、HTTP 方法、鉴权与签名要求
- 必填字段、条件必填字段、字段类型、长度和格式约束
- 响应字段、状态枚举、终态与中间态语义
- 错误码、重试条件、不可重试条件
- 回调验签、重复回调幂等、乱序与补偿语义
- 本地持久化字段、任务入队、恢复调度、审计记录是否同步更新

## 3. 平台收付通范围

官方产品介绍：<https://pay.weixin.qq.com/doc/v3/partner/4012086891.md>

当前与 LocalLife 相关的主要能力组包括：

- 商户进件
- 商户注销
- 普通支付与合单支付
- 分账、分账回退、解冻剩余资金、补差
- 交易退款、异常退款、垫付回补
- 二级商户与平台账户资金管理、预约提现、提现结果通知
- 交易账单、资金账单、分账账单下载
- 消费者投诉 2.0

## 4. 商户进件关键业务规则

商户进件官方开发指引：<https://pay.weixin.qq.com/doc/v3/partner/4012087137.md>

### 4.1 接口使用前提

- 仅支持平台使用进件接口，且平台必须已开通电商工具箱。
- 进件相关实现必须明确区分“平台侧能力”与“二级商户侧能力”，不要把平台专属接口误当成普通商户接口。

### 4.2 支持的主体类型

| 主体类型 | 定义 | 核心资料 |
| :-- | :-- | :-- |
| 小微 | 无营业执照、免办理工商注册登记的商户 | 小微经营者个人身份证 |
| 个人卖家 | 无营业执照，持续从事电子商务经营活动满 6 个月且期间经营收入累计超过 20 万元 | 个人卖家个人身份证 |
| 个体工商户 | 营业执照主体类型一般为个体户、个体工商户、个体经营 | 营业执照、经营者证件 |
| 企业 | 营业执照主体类型一般为有限公司、有限责任公司 | 营业执照、法人证件、组织机构代码证（未三证合一时） |
| 党政、机关及事业单位 | 国内各级、各类政府机构、事业单位等 | 登记证书、法人证件、组织机构代码证（未三证合一时） |
| 其他组织 | 社会团体、民办非企业、基金会等已办理组织机构代码证的组织 | 登记证书、法人证件、组织机构代码证（未三证合一时） |

### 4.3 官方额度提示

- 小微主体：正常日收款额度 10 万；交易良好可自动提升至 20-30 万/日；异常时可下降至 5 万以下/日；信用卡单日不超过 1000 元、单月不超过 1 万元。
- 个人卖家：正常日收款额度 200 万；交易良好可自动提升；异常时会下降；无信用卡收款额度限制。
- 其他主体类型：当前无收款额度限制。

如果产品、运营、商户侧说明文案涉及进件能力或收款额度，必须以官方当前规则为准，不允许编造或延用过期说法。

## 5. 商户进件主流程约束

官方文档指出，审核、签约、账户验证是三个并行流程。实现和审查时必须避免把它们错误串成单线程流程。

### 5.1 提交申请单

- 使用二级商户进件 API 创建申请单。
- 创建后，系统会在约 3 秒内完成基本资料校验。
- 创建申请单后，应通过查询申请状态 API，根据 `applyment_state` 和 `sign_state` 做后续处理。

### 5.2 签约链接与签约判断

- 推荐优先根据 `sign_state` 判断是否需要引导签约，而不是只盯 `applyment_state`。
- 在申请单状态为待账户验证、审核中、待签约时，签约状态都可能是未签约或已签约。
- 若 `sign_state` 为未签约，可获取签约链接，引导超级管理员完成签约。
- 若 `sign_state` 为已签约，说明签约已完成；待账户验证和审核也完成后，申请单会转为已完成。
- 若已签约后又被驳回，且修改了商户主体名称、法人名称、主体类型、超级管理员姓名、超级管理员证件号码，则需要重新签约，`sign_state` 会回到未签约。
- 若 `sign_state` 为不可签约，则不返回签约链接。
- 当申请单处于机器校验中、已驳回、已冻结时，签约状态为不可签约。

### 5.3 账户验证判断

必须按主体类型和超级管理员身份判断是否需要账户验证，不允许凭经验简化：

| 主体类型 | 超级管理员身份 | 账户验证 |
| :-- | :-- | :-- |
| 小微 / 个人卖家 | 法人 / 经营者 | 无需验证，直接进入审核中 |
| 个体工商户 / 企业 | 法人 / 经营者 | 无需验证，直接进入审核中 |
| 个体工商户 / 企业 | 负责人 | 需要账户验证，进入待账户验证 |
| 党政、机关及事业单位 / 其他组织 | 法人 / 经营者 | 需要账户验证，进入待账户验证 |
| 党政、机关及事业单位 / 其他组织 | 负责人 | 需要账户验证，进入待账户验证 |

### 5.4 账户验证方式与时效

- 个体工商户 / 企业，且超级管理员为负责人时：支持汇款验证或法人扫码二选一；两者均为 30 天有效期，其中法人扫码需系统校验通过才支持。
- 党政、机关及事业单位 / 其他组织：当前为汇款验证。
- 账户验证有效期需要通过查询申请状态 API 中的 `account_validation.deadline` 判断。
- 30 天内未完成账户验证，申请单会自动驳回。
- 若未超过 30 天且申请单被审核驳回，二级商户仍可继续完成验证；再次提交申请单后直接进入审核中。
- 商户汇款后，需要等待 1-2 天，以银行到账时间为准；汇款金额会原路退回到汇款银行账户。

### 5.5 入驻完成与结算账户验证

- 当申请单状态为已完成时，表示入驻完成；平台可通过查询申请状态接口获取二级商户号。
- 若申请单填写了结算银行信息，则商户发起第一笔交易后，微信支付会汇款 0.01 元校验结算卡信息。
- 平台可通过查询结算账户接口确认汇款验证结果；若失败，应引导二级商户修改结算银行卡。
- 当前只在验证失败时返回结果，建议最多每隔 1 天查询一次，连续查询 3 天。

## 6. 交易、退款与结算主语义

### 6.1 交易收款与退款

- 平台收付通同时提供合单支付和普通支付。
- 合单支付适用于购物车等跨 1-50 个二级商户订单合并支付场景。
- 普通支付适用于电商 SaaS 类服务商，每个二级商户拥有独立小店、无跨多店合并支付的场景。
- 当交易在一年内因买家或卖家原因需要退款时，可由平台发起订单退款并查询退款状态。

### 6.2 二级商户结算、分账与补差

- 平台收付通提供分账能力以满足账期、平台抽佣和分润诉求。
- 二级商户发起交易时，可授权平台传入需分账标识；资金进入二级商户账户并默认冻结 180 天。
- 平台发起分账指令后，微信支付按指令分账并解冻资金。
- 默认最高分账比例为 30%。
- 支持一笔订单多次分账，并支持分账回退。
- 对于平台营销补贴场景，支持平台补贴资金先转入二级商户账户，再统一分账。

## 7. 官方文档分组索引

以下链接是当前应优先核对的官方文档集合。开发和审查时，请先定位所属能力组，再进入具体接口文档。

### 7.1 商户进件组

- 商户进件-开发指引：<https://pay.weixin.qq.com/doc/v3/partner/4012087137.md>
- 商户进件-常见问题：<https://pay.weixin.qq.com/doc/v3/partner/4012525423.md>
- 商户进件-提交申请单：<https://pay.weixin.qq.com/doc/v3/partner/4012713017>
- 商户进件-通过业务申请编号查询申请状态：<https://pay.weixin.qq.com/doc/v3/partner/4012691376.md>
- 商户进件-通过申请单 ID 查询申请状态：<https://pay.weixin.qq.com/doc/v3/partner/4012691469.md>
- 商户进件-修改结算账户：<https://pay.weixin.qq.com/doc/v3/partner/4012761138.md>
- 商户进件-查询结算账户：<https://pay.weixin.qq.com/doc/v3/partner/4012761142.md>
- 商户进件-查询结算账户修改申请状态：<https://pay.weixin.qq.com/doc/v3/partner/4012761169.md>
- 商户进件-图片上传：<https://pay.weixin.qq.com/doc/v3/partner/4012760432.md>

### 7.2 商户注销组

- 产品介绍：<https://pay.weixin.qq.com/doc/v3/partner/4018153750.md>
- 注销预校验-商户注销资格校验：<https://pay.weixin.qq.com/doc/v3/partner/4016420099.md>
- 注销提现-提交注销提现申请：<https://pay.weixin.qq.com/doc/v3/partner/4013892756.md>
- 注销提现-商户申请单号查询申请单状态：<https://pay.weixin.qq.com/doc/v3/partner/4013892759.md>
- 注销提现-微信支付申请单号查询申请单状态：<https://pay.weixin.qq.com/doc/v3/partner/4013892765.md>

### 7.3 支付下单组

- 普通支付-开发指引：<https://pay.weixin.qq.com/doc/v3/partner/4012088031.md>
- 普通支付-小程序下单：<https://pay.weixin.qq.com/doc/v3/partner/4012714911.md>
- 普通支付-小程序调起支付：<https://pay.weixin.qq.com/doc/v3/partner/4012090181.md>
- 合单支付-开发指引：<https://pay.weixin.qq.com/doc/v3/partner/4012089542.md>
- 合单支付-小程序：<https://pay.weixin.qq.com/doc/v3/partner/4012760633.md>
- 合单支付-小程序调起支付：<https://pay.weixin.qq.com/doc/v3/partner/4012091236.md>
- 合单支付-查询订单：<https://pay.weixin.qq.com/doc/v3/partner/4012761049.md>
- 合单支付-关闭订单：<https://pay.weixin.qq.com/doc/v3/partner/4012761093.md>
- 合单支付-支付通知：<https://pay.weixin.qq.com/doc/v3/partner/4012237246.md>
- 合单支付-常见问题：<https://pay.weixin.qq.com/doc/v3/partner/4012525491.md>

### 7.4 补差与分账组

- 分账-开发指引：<https://pay.weixin.qq.com/doc/v3/partner/4012087888.md>
- 分账-业务示例代码：<https://pay.weixin.qq.com/doc/v3/partner/4015870957.md>
- 分账-请求分账：<https://pay.weixin.qq.com/doc/v3/partner/4012691594.md>
- 分账-查询分账结果：<https://pay.weixin.qq.com/doc/v3/partner/4012477734.md>
- 分账-请求分账回退：<https://pay.weixin.qq.com/doc/v3/partner/4012477737.md>
- 分账-查询分账回退结果：<https://pay.weixin.qq.com/doc/v3/partner/4012477740.md>
- 分账-解冻剩余资金：<https://pay.weixin.qq.com/doc/v3/partner/4012477745.md>
- 分账-查询订单剩余分账金额：<https://pay.weixin.qq.com/doc/v3/partner/4012477751.md>
- 分账-添加分账接收方：<https://pay.weixin.qq.com/doc/v3/partner/4012477758.md>
- 分账-删除分账接收方：<https://pay.weixin.qq.com/doc/v3/partner/4012477759.md>
- 分账-分账动账通知：<https://pay.weixin.qq.com/doc/v3/partner/4012116672.md>
- 分账-分账常见问题：<https://pay.weixin.qq.com/doc/v3/partner/4012525463.md>
- 分账-分账失败处理指引：<https://pay.weixin.qq.com/doc/v3/partner/4015504955.md>

### 7.5 交易退款组

- 交易退款-业务示例代码：<https://pay.weixin.qq.com/doc/v3/partner/4015217874.md>
- 交易退款-申请退款：<https://pay.weixin.qq.com/doc/v3/partner/4012476892.md>
- 交易退款-查询单笔退款（按微信支付退款单号）：<https://pay.weixin.qq.com/doc/v3/partner/4012476908.md>
- 交易退款-查询单笔退款（按商户退款单号）：<https://pay.weixin.qq.com/doc/v3/partner/4012476911.md>
- 交易退款-退款结果通知：<https://pay.weixin.qq.com/doc/v3/partner/4012124635.md>
- 交易退款-查询垫付回补通知：<https://pay.weixin.qq.com/doc/v3/partner/4012476916.md>
- 交易退款-垫付退款回补：<https://pay.weixin.qq.com/doc/v3/partner/4012476927.md>
- 交易退款-发起异常退款：<https://pay.weixin.qq.com/doc/v3/partner/4015181616.md>

### 7.6 账户资金管理组

- 账户资金管理-余额查询-查询二级商户账户实时余额：<https://pay.weixin.qq.com/doc/v3/partner/4012476690.md>
- 账户资金管理-余额查询-查询二级商户账户日终余额：<https://pay.weixin.qq.com/doc/v3/partner/4012476693.md>
- 账户资金管理-余额查询-查询平台账户实时余额：<https://pay.weixin.qq.com/doc/v3/partner/4012476700.md>
- 账户资金管理-余额查询-查询平台账户日终余额：<https://pay.weixin.qq.com/doc/v3/partner/4012476702.md>
- 账户资金管理-余额查询-常见问题：<https://pay.weixin.qq.com/doc/v3/partner/4016644075.md>
- 账户资金管理-商户提现-二级商户预约提现：<https://pay.weixin.qq.com/doc/v3/partner/4012476652.md>
- 账户资金管理-商户提现-二级商户查询预约提现状态（根据商户预约提现单号查询）：<https://pay.weixin.qq.com/doc/v3/partner/4012476656.md>
- 账户资金管理-商户提现-二级商户查询预约提现状态（根据微信支付预约提现单号查询）：<https://pay.weixin.qq.com/doc/v3/partner/4012476665.md>
- 账户资金管理-商户提现-平台预约提现：<https://pay.weixin.qq.com/doc/v3/partner/4012476670.md>
- 账户资金管理-商户提现-平台查询预约提现状态（根据商户预约提现单号查询）：<https://pay.weixin.qq.com/doc/v3/partner/4012476672.md>
- 账户资金管理-商户提现-平台查询预约提现状态（根据微信支付预约提现单号查询）：<https://pay.weixin.qq.com/doc/v3/partner/4012476674.md>
- 账户资金管理-商户提现-二级商户按日终余额预约提现：<https://pay.weixin.qq.com/doc/v3/partner/4013328143.md>
- 账户资金管理-商户提现-查询二级商户按日终余额预约提现状态：<https://pay.weixin.qq.com/doc/v3/partner/4013328163.md>
- 账户资金管理-商户提现-按日下载提现异常文件：<https://pay.weixin.qq.com/doc/v3/partner/4012476678.md>
- 账户资金管理-商户提现-商户提现状态变更通知：<https://pay.weixin.qq.com/doc/v3/partner/4013049135.md>
- 账户资金管理-商户提现-常见问题：<https://pay.weixin.qq.com/doc/v3/partner/4014075940.md>

### 7.7 账单下载组

- 账单下载-业务示例代码：<https://pay.weixin.qq.com/doc/v3/partner/4016062108.md>
- 账单下载-申请交易账单：<https://pay.weixin.qq.com/doc/v3/partner/4012760667.md>
- 账单下载-申请资金账单：<https://pay.weixin.qq.com/doc/v3/partner/4012760672.md>
- 账单下载-申请分账账单：<https://pay.weixin.qq.com/doc/v3/partner/4012761131.md>
- 账单下载-下载账单：<https://pay.weixin.qq.com/doc/v3/partner/4012124894.md>
- 账单下载-申请二级商户资金账单：<https://pay.weixin.qq.com/doc/v3/partner/4012760697.md>
- 账单下载-申请单个子商户资金账单：<https://pay.weixin.qq.com/doc/v3/partner/4012760249.md>
- 账单下载-下载单个子商户 / 二级商户资金账单：<https://pay.weixin.qq.com/doc/v3/partner/4014314390.md>

### 7.8 消费者投诉 2.0

- 消费者投诉-产品介绍：<https://pay.weixin.qq.com/doc/v3/partner/4012072827.md>
- 消费者投诉-接入前准备（服务商）：<https://pay.weixin.qq.com/doc/v3/partner/4012072844.md>
- 消费者投诉-开发指引：<https://pay.weixin.qq.com/doc/v3/partner/4012072858.md>
- 消费者投诉-业务示例代码：<https://pay.weixin.qq.com/doc/v3/partner/4015933338.md>
- 消费者投诉-常见问题：<https://pay.weixin.qq.com/doc/v3/partner/4016111688.md>
- 消费者投诉-主动查询投诉信息-查询投诉单列表：<https://pay.weixin.qq.com/doc/v3/partner/4012691285.md>
- 消费者投诉-主动查询投诉信息-查询投诉单详情：<https://pay.weixin.qq.com/doc/v3/partner/4012691648.md>
- 消费者投诉-主动查询投诉信息-查询投诉单协商历史：<https://pay.weixin.qq.com/doc/v3/partner/4012691802.md>
- 消费者投诉-实时获取投诉信息-投诉通知回调：<https://pay.weixin.qq.com/doc/v3/partner/4012076174.md>
- 消费者投诉-实时获取投诉信息-创建投诉通知回调地址：<https://pay.weixin.qq.com/doc/v3/partner/4012458106.md>
- 消费者投诉-实时获取投诉信息-查询投诉通知回调地址：<https://pay.weixin.qq.com/doc/v3/partner/4012459065.md>
- 消费者投诉-实时获取投诉信息-更新投诉通知回调地址：<https://pay.weixin.qq.com/doc/v3/partner/4012459287.md>
- 消费者投诉-实时获取投诉信息-删除投诉通知回调地址：<https://pay.weixin.qq.com/doc/v3/partner/4012460474.md>
- 消费者投诉-商户处理用户投诉-回复用户：<https://pay.weixin.qq.com/doc/v3/partner/4012467213.md>
- 消费者投诉-商户处理用户投诉-反馈处理完成：<https://pay.weixin.qq.com/doc/v3/partner/4012467217.md>
- 消费者投诉-商户处理用户投诉-更新退款审批结果：<https://pay.weixin.qq.com/doc/v3/partner/4012467218.md>
- 消费者投诉-商户处理用户投诉-回复需要即时服务的投诉单：<https://pay.weixin.qq.com/doc/v3/partner/4017151726.md>
- 消费者投诉-商户反馈图片-图片上传接口：<https://pay.weixin.qq.com/doc/v3/partner/4012467222.md>
- 消费者投诉-商户反馈图片-图片请求接口：<https://pay.weixin.qq.com/doc/v3/partner/4012467223.md>

## 8. 与本仓库 AI 提示系统的关系

- 微信支付实现、审查、排障时，应先读本文，再读对应的运行手册、前端协作规范或图片上传专项约束。
- 对于 `locallife/wechat/` 下的代码变更，本文应视为默认官方文档路由基线。
- 如果未来官方文档更新导致接口用途、字段、状态、错误码或业务规则变化，优先更新本文及相关专项文档，再调整代码和 Prompt 路由。