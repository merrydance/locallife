# 微信支付 Domain

## 1. 平台事实

本平台基于微信小程序构建，全局只有一个 appid，在配置文件中统一配置。

本平台是一个外卖和到店服务平台。

支付能力分为两条主线：

- 平台收付通：用于为餐饮、零售等商户提供支付、退款、分账、进件、资金管理、投诉等能力。
- 直连支付：用于向独立运营的个人骑手收取保证金，以及处理商户、骑手索赔相关赔付。

当前分账接收方固定为三个：

- 骑手：获取全额代取运费，不按比例分账。
- 运营商：获取订单金额的 3%。
- 平台：获取订单金额的 2%。

所有支付功能开发、审查和前端流程设计，都必须以能力组中的官方文档为唯一真值来源，不能再依赖仓库内旧快照或历史审计稿。

## 2. 项目内支付开发规则

### 2.1 后端规则

- 先抽象微信支付客户端，明确区分直连支付客户端与平台收付通客户端。
- 按能力组组织支付文档，而不是按零散接口组织。
- 每个能力组中的官方文档，都是该能力组请求结构、响应结构、条件必填、状态、枚举、错误码的唯一真值来源。
- 代码实现时，要把能力组中的请求结构、响应结构、错误码分别映射到对应包中。
- 映射完成后，项目内所有后续调用都只能依赖这些包中的结构与错误码定义，不能再从 handler、logic、worker 或前端随意重写一套。
- 若官方文档变更，应优先更新对应 capability group 的结构包和错误码包，再更新调用链。

### 2.2 前端规则

- 前端页面和流程设计，不直接从后端已有页面反推。
- 前端必须以能力组中的开发指引、前置说明、产品介绍、常见问题等官方文档来组合页面流程和用户提示。
- 前端不拥有支付协议真值；前端消费的是后端已经按能力组固化后的项目内结构与状态语义。

## 3. 当前支付能力分工

### 3.1 平台收付通

使用微信支付平台收付通工具箱，为餐饮、零售等客户提供：

- 商户进件
- 普通支付与合单支付
- 分账与补差
- 退款与异常退款
- 账户资金管理与提现
- 账单下载与对账
- 消费者投诉 2.0
- 商户注销提现

### 3.2 直连支付

使用微信支付直连支付，处理：

- 个人骑手保证金收取
- 商户索赔赔付
- 骑手索赔赔付
- 与这些直连支付主链路配套的查单、关单、退款、异常退款、通知

## 4. 能力组文档索引

### 4.1 平台收付通产品入口

- 平台收付通产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012086891.md

### 4.2 商户进件组

- 商户进件-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012087137.md
- 商户进件-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4012525423.md
- 商户进件-提交申请单：https://pay.weixin.qq.com/doc/v3/partner/4012713017
- 商户进件-通过业务申请编号查询申请状态：https://pay.weixin.qq.com/doc/v3/partner/4012691376.md
- 商户进件-通过申请单ID查询申请状态：https://pay.weixin.qq.com/doc/v3/partner/4012691469.md
- 商户进件-修改结算账户：https://pay.weixin.qq.com/doc/v3/partner/4012761138.md
- 商户进件-查询结算账户：https://pay.weixin.qq.com/doc/v3/partner/4012761142.md
- 商户进件-查询结算账户修改申请状态：https://pay.weixin.qq.com/doc/v3/partner/4012761169.md
- 商户进件-图片上传：https://pay.weixin.qq.com/doc/v3/partner/4012760432.md

### 4.3 商户注销组

- 产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4018153750.md
- 注销预校验-商户注销资格校验：https://pay.weixin.qq.com/doc/v3/partner/4016420099.md
- 注销提现-提交注销提现申请：https://pay.weixin.qq.com/doc/v3/partner/4013892756.md
- 注销提现-商户申请单号查询申请单状态：https://pay.weixin.qq.com/doc/v3/partner/4013892759.md
- 注销提现-微信支付申请单号查询申请单状态：https://pay.weixin.qq.com/doc/v3/partner/4013892765.md

### 4.4 支付下单组

- 普通支付-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012088031.md
- 普通支付-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4012525474.md
- 普通支付-小程序下单：https://pay.weixin.qq.com/doc/v3/partner/4012714911.md
- 普通支付-小程序调起支付：https://pay.weixin.qq.com/doc/v3/partner/4012090181.md
- 普通支付-微信支付订单号查询订单：https://pay.weixin.qq.com/doc/v3/partner/4012760565.md
- 普通支付-微信支付商户订单号查询订单：https://pay.weixin.qq.com/doc/v3/partner/4012760568.md
- 普通支付-关闭订单：https://pay.weixin.qq.com/doc/v3/partner/4012760574
- 普通支付-支付结果通知：https://pay.weixin.qq.com/doc/v3/partner/4012090195
- 合单支付-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012089542.md
- 合单支付-小程序：https://pay.weixin.qq.com/doc/v3/partner/4012760633.md
- 合单支付-小程序调起支付：https://pay.weixin.qq.com/doc/v3/partner/4012091236.md
- 合单支付-合单查询订单：https://pay.weixin.qq.com/doc/v3/partner/4012761049.md
- 合单支付-合单关闭订单：https://pay.weixin.qq.com/doc/v3/partner/4012761093.md
- 合单支付-合单支付通知：https://pay.weixin.qq.com/doc/v3/partner/4012237246.md
- 合单支付-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4012525491.md

### 4.5 补差与分账组

- 分账-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012087888.md
- 分账-业务示例代码：https://pay.weixin.qq.com/doc/v3/partner/4015870957.md
- 分账-请求分账：https://pay.weixin.qq.com/doc/v3/partner/4012691594.md
- 分账-查询分账结果：https://pay.weixin.qq.com/doc/v3/partner/4012477734.md
- 分账-请求分账回退：https://pay.weixin.qq.com/doc/v3/partner/4012477737.md
- 分账-查询分账回退结果：https://pay.weixin.qq.com/doc/v3/partner/4012477740.md
- 分账-解冻剩余资金：https://pay.weixin.qq.com/doc/v3/partner/4012477745.md
- 分账-查询订单剩余分账金额：https://pay.weixin.qq.com/doc/v3/partner/4012477751.md
- 分账-添加分账接收方：https://pay.weixin.qq.com/doc/v3/partner/4012477758.md
- 分账-删除分账接收方：https://pay.weixin.qq.com/doc/v3/partner/4012477759.md
- 分账-分账动账通知：https://pay.weixin.qq.com/doc/v3/partner/4012116672.md
- 分账-分账常见问题：https://pay.weixin.qq.com/doc/v3/partner/4012525463.md
- 分账-分账失败处理指引：https://pay.weixin.qq.com/doc/v3/partner/4015504955.md

### 4.6 交易退款组

- 交易退款-业务示例代码：https://pay.weixin.qq.com/doc/v3/partner/4015217874.md
- 交易退款-申请退款：https://pay.weixin.qq.com/doc/v3/partner/4012476892.md
- 交易退款-查询单笔退款（按微信支付退款单号）：https://pay.weixin.qq.com/doc/v3/partner/4012476908.md
- 交易退款-查询单笔退款（按商户退款单号）：https://pay.weixin.qq.com/doc/v3/partner/4012476911.md
- 交易退款-退款结果通知：https://pay.weixin.qq.com/doc/v3/partner/4012124635.md
- 交易退款-查询垫付回补通知：https://pay.weixin.qq.com/doc/v3/partner/4012476916.md
- 交易退款-垫付退款回补：https://pay.weixin.qq.com/doc/v3/partner/4012476927.md
- 交易退款-发起异常退款：https://pay.weixin.qq.com/doc/v3/partner/4015181616.md

### 4.7 账户资金管理组

- 账户资金管理-余额查询-查询二级商户账户实时余额：https://pay.weixin.qq.com/doc/v3/partner/4012476690.md
- 账户资金管理-余额查询-查询二级商户账户日终余额：https://pay.weixin.qq.com/doc/v3/partner/4012476693.md
- 账户资金管理-余额查询-查询平台账户实时余额：https://pay.weixin.qq.com/doc/v3/partner/4012476700.md
- 账户资金管理-余额查询-查询平台账户日终余额：https://pay.weixin.qq.com/doc/v3/partner/4012476702.md
- 账户资金管理-余额查询-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4016644075.md
- 账户资金管理-商户提现-二级商户预约提现：https://pay.weixin.qq.com/doc/v3/partner/4012476652.md
- 账户资金管理-商户提现-二级商户查询预约提现状态（根据商户预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476656.md
- 账户资金管理-商户提现-二级商户查询预约提现状态（根据微信支付预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476665.md
- 账户资金管理-商户提现-平台预约提现：https://pay.weixin.qq.com/doc/v3/partner/4012476670.md
- 账户资金管理-商户提现-平台查询预约提现状态（根据商户预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476672.md
- 账户资金管理-商户提现-平台查询预约提现状态（根据微信支付预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476674.md
- 账户资金管理-商户提现-二级商户按日终余额预约提现：https://pay.weixin.qq.com/doc/v3/partner/4013328143.md
- 账户资金管理-商户提现-查询二级商户按日终余额预约提现状态：https://pay.weixin.qq.com/doc/v3/partner/4013328163.md
- 账户资金管理-商户提现-按日下载提现异常文件：https://pay.weixin.qq.com/doc/v3/partner/4012476678.md
- 账户资金管理-商户提现-商户提现状态变更通知：https://pay.weixin.qq.com/doc/v3/partner/4013049135.md
- 账户资金管理-商户提现-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4014075940.md

### 4.8 账单下载组

- 账单下载-业务示例代码：https://pay.weixin.qq.com/doc/v3/partner/4016062108.md
- 账单下载-申请交易账单：https://pay.weixin.qq.com/doc/v3/partner/4012760667.md
- 账单下载-申请资金账单：https://pay.weixin.qq.com/doc/v3/partner/4012760672.md
- 账单下载-申请分账账单：https://pay.weixin.qq.com/doc/v3/partner/4012761131.md
- 账单下载-下载账单：https://pay.weixin.qq.com/doc/v3/partner/4012124894.md
- 账单下载-申请二级商户资金账单：https://pay.weixin.qq.com/doc/v3/partner/4012760697.md
- 账单下载-申请单个子商户资金账单：https://pay.weixin.qq.com/doc/v3/partner/4012760249.md
- 账单下载-下载单个子商户/二级商户资金账单：https://pay.weixin.qq.com/doc/v3/partner/4014314390.md

### 4.9 消费者投诉 2.0

- 消费者投诉-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012072827.md
- 消费者投诉-接入前准备（我们是做为服务商）：https://pay.weixin.qq.com/doc/v3/partner/4012072844.md
- 消费者投诉-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012072858.md
- 消费者投诉-业务示例代码：https://pay.weixin.qq.com/doc/v3/partner/4015933338.md
- 消费者投诉-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4016111688.md
- 消费者投诉-主动查询投诉信息-查询投诉单列表：https://pay.weixin.qq.com/doc/v3/partner/4012691285.md
- 消费者投诉-主动查询投诉信息-查询投诉单详情：https://pay.weixin.qq.com/doc/v3/partner/4012691648.md
- 消费者投诉-主动查询投诉信息-查询投诉单协商历史：https://pay.weixin.qq.com/doc/v3/partner/4012691802.md
- 消费者投诉-实时获取投诉信息-投诉通知回调：https://pay.weixin.qq.com/doc/v3/partner/4012076174.md
- 消费者投诉-实时获取投诉信息-创建投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012458106.md
- 消费者投诉-实时获取投诉信息-查询投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012459065.md
- 消费者投诉-实时获取投诉信息-更新投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012459287.md
- 消费者投诉-实时获取投诉信息-删除投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012460474.md
- 消费者投诉-商户处理用户投诉-回复用户：https://pay.weixin.qq.com/doc/v3/partner/4012467213.md
- 消费者投诉-商户处理用户投诉-反馈处理完成：https://pay.weixin.qq.com/doc/v3/partner/4012467217.md
- 消费者投诉-商户处理用户投诉-更新退款审批结果：https://pay.weixin.qq.com/doc/v3/partner/4012467218.md
- 消费者投诉-商户处理用户投诉-回复需要即时服务的投诉单：https://pay.weixin.qq.com/doc/v3/partner/4017151726.md
- 消费者投诉-商户反馈图片-图片上传接口：https://pay.weixin.qq.com/doc/v3/partner/4012467222.md
- 消费者投诉-商户反馈图片-图片请求接口：https://pay.weixin.qq.com/doc/v3/partner/4012467223.md

### 4.10 直连支付组

- 小程序支付-产品介绍：https://pay.weixin.qq.com/doc/v3/merchant/4012791894.md
- 小程序支付-快速开始：https://pay.weixin.qq.com/doc/v3/merchant/4015459512.md
- 小程序支付-开发指引：https://pay.weixin.qq.com/doc/v3/merchant/4012791911.md
- 小程序支付-常见问题：https://pay.weixin.qq.com/doc/v3/merchant/4012791910.md
- 小程序支付-API列表-JSAPI/小程序下单：https://pay.weixin.qq.com/doc/v3/merchant/4012791897.md
- 小程序支付-API列表-小程序调起支付：https://pay.weixin.qq.com/doc/v3/merchant/4012791898.md
- 小程序支付-API列表-微信支付订单号查询订单：https://pay.weixin.qq.com/doc/v3/merchant/4012791899.md
- 小程序支付-API列表-商户订单号查询订单：https://pay.weixin.qq.com/doc/v3/merchant/4012791900.md
- 小程序支付-API列表-关闭订单：https://pay.weixin.qq.com/doc/v3/merchant/4012791901
- 小程序支付-API列表-支付成功回调通知：https://pay.weixin.qq.com/doc/v3/merchant/4012791902.md
- 小程序支付-API列表-退款申请：https://pay.weixin.qq.com/doc/v3/merchant/4012791903.md
- 小程序支付-API列表-查询单笔退款（通过商户退款单号）：https://pay.weixin.qq.com/doc/v3/merchant/4012791904.md
- 小程序支付-API列表-发起异常退款：https://pay.weixin.qq.com/doc/v3/merchant/4012791905.md
- 小程序支付-API列表-退款结果回调通知：https://pay.weixin.qq.com/doc/v3/merchant/4012791906.md

## 5. 默认执行顺序

1. 先在本文件中确定当前能力组属于平台收付通还是直连支付。
2. 再打开该能力组对应的官方产品介绍、开发指引、接口文档、通知文档、常见问题。
3. 先抽象客户端，再把该能力组的请求结构、响应结构、错误码映射进对应包。
4. 后续所有开发与调用，只依赖项目内已按能力组沉淀的结构与错误码。
5. 前端页面和流程，只能基于该能力组的开发指引、前置说明和常见问题来组合，不得凭旧页面猜测。

## 6. 维护规则

- 本目录不再拆分为多个 payment 基线文档。
- 本 README 是 payment domain 的唯一仓库内入口文档。
- 不再新增 dated payment 快照、审计稿、矩阵稿或 runbook 子文档。
- 若能力组列表变更、平台支付分工变更、比例规则变更或文档来源变更，直接更新本 README。
- 若具体接口结构或错误码变化，应优先更新代码中的 contracts 和 errorcodes 包，然后再回写这里的能力组链接或说明。