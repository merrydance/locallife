# 微信支付 Domain

## 1. 平台事实

本平台基于微信小程序构建，全局只有一个 appid，在配置文件中统一配置。

本平台是一个外卖和到店服务平台。

支付能力分为三类边界：

- 普通服务商特约商户：迁移目标，用于餐饮、零售等商户主业务支付、退款、分账、进件、账单和上线后商户管理。
- 平台收付通：原商户主业务能力来源。迁移完成后不承载新链路、不设计老商户兼容，仅作为冷备代码和文档能力保留，以便微信支付政策再次变化时评估。
- 直连支付：用于骑手保证金缴纳与赎回，以及商户、骑手追偿向平台付款。

迁移目标：如果微信支付不允许继续使用平台收付通，餐饮、零售等商户主业务支付应切换到普通服务商特约商户模式。直连支付能力保持不变，不纳入普通服务商迁移范围。

当前分账接收方固定为三个：

- 骑手：获取全额代取运费，不按比例分账。
- 运营商：获取订单金额的 3%。
- 平台：获取订单金额的 2%。

所有支付功能开发、审查和前端流程设计，都必须以能力组中的官方文档为唯一真值来源，不能再依赖仓库内旧快照或历史审计稿。

## 2. 项目内支付开发规则

### 2.1 后端规则

- 先抽象微信支付客户端，明确区分普通服务商特约商户、直连支付与平台收付通客户端。
- 普通服务商特约商户按独立 bounded module 开发，模块根包使用 `locallife/wechat/ordinaryserviceprovider`，对外 Go 类型使用 `OrdinaryServiceProvider` 前缀；模块外文件若必须表达该能力，使用 `ordinary_service_provider` 前缀。
- 普通服务商交易数据通道命名必须使用 `ordinary_service_provider`，不使用 `partner` 或复用 `ecommerce`。
- 普通服务商模块必须高内聚拥有自己的 client、contracts、errorcodes、notification 解密、validation、error mapping 和测试，不把这些真值散落进共享的 `wechat/contracts`、`wechat/errorcodes` 或既有支付 client 中。
- 普通服务商模块不得依赖直连支付或平台收付通实现作为行为模板；实现依据只来自官方普通服务商文档、本 README 和后端工程标准。若复用代码，只允许复用不含支付能力语义的 API v3 签名、证书、HTTP transport 等基础设施，并且边界必须有单元测试保护。
- 普通服务商模块对业务层只暴露最小能力接口，业务层不得直接依赖微信官方请求/响应结构、错误码字符串或模块内部 helper。
- 默认不 fork `wechatpay-go`。普通服务商模块可以引入官方 `github.com/wechatpay-apiv3/wechatpay-go` 作为模块私有 adapter 依赖，复用其 `core.Client`、签名、验签、证书/公钥、`core/notify`、敏感字段加解密、`APIError` 和已覆盖的 `services/*`，但不得让 SDK DTO、`core.APIResult` 或 `core.APIError` 穿透到 `logic/`、`api/`、`worker/` 或 `db/` 层。
- `wechatpay-go` 已覆盖的普通服务商支付、退款、分账能力，应在 `ordinaryserviceprovider` 内包装为本项目自己的 capability 方法；SDK 未覆盖且官方明确支持【普通服务商】的特约商户进件、合单支付、商户管控查询、商户平台处置通知等能力，应使用官方 `core.Client.Request` 或等价中性 transport 加本模块自有 contracts/errorcodes 实现。
- 本项目当前只接入【普通服务商】身份，不是【渠道商】、【从业机构（银行）】、【从业机构（支付机构）】或【平台商户】。官方文档 `支持商户` 未包含【普通服务商】的接口，不得放入 `ordinaryserviceprovider` 生产主链路；若未来获得其他身份，必须先新增独立 bounded module 和独立标准。
- 只有当普通服务商缺口能力膨胀到需要长期维护通用 SDK、并准备向官方仓库 upstream PR 或内部正式维护 SDK 时，才重新评估 fork `wechatpay-go`；在此之前不得用 fork 替换官方 SDK 依赖。
- 按能力组组织支付文档，而不是按零散接口组织。
- 每个能力组中的官方文档，都是该能力组请求结构、响应结构、条件必填、状态、枚举、错误码的唯一真值来源。
- 进件只允许个体工商户和企业，这是业务设计，不是错误。
- 代码实现时，要把能力组中的请求结构、响应结构、错误码分别映射到对应包中。
- 映射完成后，项目内所有后续调用都只能依赖这些包中的结构与错误码定义，不能再从 handler、logic、worker 或前端随意重写一套。
- 若官方文档变更，应优先更新对应 capability group 的结构包和错误码包，再更新调用链。

### 2.2 前端规则

- 前端页面和流程设计，不直接从后端已有页面反推。
- 前端必须以能力组中的开发指引、前置说明、产品介绍、常见问题等官方文档来组合页面流程和用户提示。
- 前端不拥有支付协议真值；前端消费的是后端已经按能力组固化后的项目内结构与状态语义。

## 3. 当前支付能力分工

### 3.1 平台收付通

使用微信支付平台收付通工具箱，原计划为餐饮、零售等客户提供：

- 预订、堂食、自取、外卖等商户业务支付
- 商户进件
- 普通支付与合单支付
- 分账与补差
- 退款与异常退款
- 账户资金管理与提现
- 账单下载与对账
- 消费者投诉 2.0
- 商户注销提现

平台收付通下线后，不作为老商户兼容链路继续开发；已有代码可冷备保留，但新商户主业务不得再新增依赖。

### 3.2 直连支付

使用微信支付直连支付，处理：

- 个人骑手保证金缴纳
- 个人骑手保证金赎回
- 商户追偿向平台付款
- 骑手追偿向平台付款
- 与这些直连支付主链路配套的查单、关单、退款、异常退款、通知

### 3.3 普通服务商（迁移目标）

使用普通服务商特约商户模式替代餐饮、零售等商户主业务的平台收付通交易能力，覆盖：

- 特约商户进件与结算账户查询、修改
- 商户平台处置通知
- 商户被管控能力及原因查询
- 普通小程序支付
- 小程序合单支付
- 交易退款与退款通知
- 分账、分账回退、剩余资金解冻、分账接收方管理
- 交易账单、资金账单、分账账单

普通服务商迁移不承接以下平台收付通能力：

- 平台收付通 180 天账期
- 补差、补差回退、取消补差
- 平台收付通垫付退款与垫付回补
- 二级商户余额查询、平台账户余额查询
- 二级商户预约提现、平台预约提现
- 注销后余额提现

商户资金出口应以普通服务商特约商户的结算银行账户、微信支付商户平台或微信支付商家助手为准。平台小程序不再承诺代商户发起提现或提供平台资金管理能力。

普通服务商分账接收方按具体支付单与特约商户号同步。平台收付通时代的全局分账接收方预热、修复和 worker lifecycle 仅可作为历史/cold-reserve 排障入口保留；普通服务商启用时不得作为新商户分账前置入口，API/UI 必须给出“按支付单自动同步、必要时做商户管控诊断”的明确指引。

本系统不存在需要继续兼容的平台收付通老商户；迁移实现只需要建设新的普通服务商特约商户模块，并把相关业务链路切换到该模块。平台收付通代码不必完全删除，但不得作为支付、退款、分账、进件或商户管理的新路径。

## 4. 能力组文档索引

### 4.1 平台收付通产品入口

- 平台收付通产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012086891.md
- API V3概述：https://pay.weixin.qq.com/doc/v3/partner/4012081673.md
- 如何签名:
  如何构造接口请求签名-请求参数里带Path参数（路径参数），如何计算签名:https://pay.weixin.qq.com/doc/v3/partner/4012365862.md
  如何构造接口请求签名-请求参数里带Body参数(包体参数），如何计算签名:https://pay.weixin.qq.com/doc/v3/partner/4012365864.md
  如何构造接口请求签名-请求参数里有Query（查询参数），如何计算签名:https://pay.weixin.qq.com/doc/v3/partner/4012365865.md
  如何构造接口请求签名-图片上传接口，如何计算签名:https://pay.weixin.qq.com/doc/v3/partner/4012365863.md
- 如何构造调起支付签名-小程序调起支付签名:https://pay.weixin.qq.com/doc/v3/partner/4012365869.md 
- 如何验签-如何使用微信支付公钥验签:https://pay.weixin.qq.com/doc/v3/partner/4013059017.md
- 如何使用微信支付公钥加密敏感字段:https://pay.weixin.qq.com/doc/v3/partner/4013059044.md
- 如何解密回调报文和平台证书:https://pay.weixin.qq.com/doc/v3/partner/4012082320.md
- 开户银行全称对照表:https://pay.weixin.qq.com/doc/v3/partner/4012082812.md
- 开户银行对照表:https://pay.weixin.qq.com/doc/v3/partner/4012082813.md
- 银行类型对照表:https://pay.weixin.qq.com/doc/v3/partner/4012082814.md
- 省市区编号对照表:https://pay.weixin.qq.com/doc/v3/partner/4012082815.md

### 4.2 商户进件组

- 商户进件-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012087137.md
- 商户进件-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4012525423.md
- 商户进件-提交申请单：https://pay.weixin.qq.com/doc/v3/partner/4012713017.md
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
- 补差-业务示例代码:https://pay.weixin.qq.com/doc/v3/partner/4015593692.md
- 补差-请求补差:https://pay.weixin.qq.com/doc/v3/partner/4012477631.md
- 补差-请求补差回退:https://pay.weixin.qq.com/doc/v3/partner/4012477636.md
- 补差-取消补差:https://pay.weixin.qq.com/doc/v3/partner/4012477639.md
- 补差-常见问题:https://pay.weixin.qq.com/doc/v3/partner/4015942503.md

### 4.6 交易退款组

- 交易退款-业务示例代码：https://pay.weixin.qq.com/doc/v3/partner/4015217874.md
- 交易退款-申请退款：https://pay.weixin.qq.com/doc/v3/partner/4012476892.md
- 交易退款-查询单笔退款（按微信支付退款单号）：https://pay.weixin.qq.com/doc/v3/partner/4012476908.md
- 交易退款-查询单笔退款（按商户退款单号）：https://pay.weixin.qq.com/doc/v3/partner/4012476911.md
- 交易退款-退款结果通知：https://pay.weixin.qq.com/doc/v3/partner/4012124635.md
- 交易退款-查询垫付回补通知：https://pay.weixin.qq.com/doc/v3/partner/4012476916.md（当前业务模式不使用）
- 交易退款-垫付退款回补：https://pay.weixin.qq.com/doc/v3/partner/4012476927.md（当前业务模式不使用）
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

- 账单下载-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4013080592.md
- 账单下载-交易账单详细说明：https://pay.weixin.qq.com/doc/v3/partner/4013080599.md
- 账单下载-资金账单详细说明：https://pay.weixin.qq.com/doc/v3/partner/4013080600.md
- 账单下载-平台下载账单操作指引：https://pay.weixin.qq.com/doc/v3/partner/4013080601.md
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

### 4.10 普通服务商迁移能力组

#### 4.10.1 普通服务商产品与安全基础

- 合作伙伴平台文档中心：https://pay.weixin.qq.com/doc/v3/partner/llms.txt
- 微信支付 AI Agent Skills 知识库：https://github.com/wechatpay-apiv3/wechatpay-skills（仅作为官方示例、流程、排障和质量检查参考，不作为生产依赖）
- 微信支付 API v3 Go SDK：https://github.com/wechatpay-apiv3/wechatpay-go（仅作为普通服务商模块私有 adapter 依赖或结构参考，不作为业务层 contract）
- 微信支付接入模式：https://pay.weixin.qq.com/doc/v3/partner/4012081931.md（用于区分普通服务商、渠道商、从业机构、平台商户等身份边界）
- API V3 概述：https://pay.weixin.qq.com/doc/v3/partner/4012081673.md
- API V3 接口规则：https://pay.weixin.qq.com/doc/v3/partner/4012081726.md
- 如何构造接口请求签名-Path 参数：https://pay.weixin.qq.com/doc/v3/partner/4012365862.md
- 如何构造接口请求签名-Body 参数：https://pay.weixin.qq.com/doc/v3/partner/4012365864.md
- 如何构造接口请求签名-Query 参数：https://pay.weixin.qq.com/doc/v3/partner/4012365865.md
- 小程序调起支付签名：https://pay.weixin.qq.com/doc/v3/partner/4012365869.md
- 如何使用微信支付公钥验签：https://pay.weixin.qq.com/doc/v3/partner/4013059017.md
- 如何使用微信支付公钥加密敏感字段：https://pay.weixin.qq.com/doc/v3/partner/4013059044.md
- 如何解密回调报文和平台证书：https://pay.weixin.qq.com/doc/v3/partner/4012082320.md

普通服务商模块的 `contracts`、`errorcodes` 和 endpoint path 必须以本节列出的官方 endpoint 文档为唯一结构真值；不得从平台收付通、直连支付、历史 artifact 或本地业务 DTO 反推字段名、错误码或嵌套结构。每次修改 `locallife/wechat/ordinaryserviceprovider/**` 时，必须复核对应官方页面的请求参数、应答参数、回调解密后资源结构和错误码，并用模块测试固定官方字段名，避免以“能编译/能反序列化”为准。生产代码必须在普通服务商模块内维护可执行的能力组契约映射：按能力组列出 endpoint ID、method/path、operation、请求/响应 contract 类型、状态归属和 request validation 入口；这不是官方 URL 文档目录，官方页面 URL 仍只保留在本 README、snapshot 和测试夹具中。`errorcodes` 必须按能力组和 endpoint 维护 DocumentedCodes 集合，保留官方原始错误码拼写及 canonical alias 匹配，让主业务层只调用普通服务商模块能力，不直接拼微信字段、猜错误码或依赖旧平台收付通/直连 DTO。

当前普通服务商上线测试收口范围以商户进件、结算账户、商户管控与处置通知、小程序支付、合单支付、退款、分账/回退/解冻/剩余待分金额/接收方/通知为主。交易账单、资金账单、分账账单申请与下载，以及分账最大比例查询暂不作为本轮上线测试阻塞项；后续启用这些能力时，必须先按本节同样规则补齐 `contracts`、`errorcodes`、client runtime coverage、后端 API/任务入口和前端业务任务说明。`商户开户意愿确认` 与 `不活跃商户身份核实` 当前不得计入普通服务商上线范围，除非官方接口页的 `支持商户` 明确包含【普通服务商】并完成本 README 复核更新。

#### 4.10.2 普通服务商商户管理、特约商户进件与结算账户

普通服务商商户管理能力中，特约商户进件属于新商户准入主链路；商户被管控能力及原因查询、商户平台处置通知属于上线后的风控运营与异常恢复主链路。`商户开户意愿确认` 是渠道商/从业机构能力组，不是当前普通服务商能力；`不活跃商户身份核实` API 当前标注为【平台商户】，也不是当前普通服务商能力。4.10 中凡标注“非普通服务商”的文档只能作为角色边界参考，不得生成 `ordinaryserviceprovider` contracts、errorcodes、API、worker 或前端入口。

普通服务商开户流程的项目内完成态规则：

- 提交申请单使用特约商户进件 `applyment4sub`，`contact_info.contact_email` 必须由商户填写超级管理员邮箱；后端和前端都不得使用平台管理员邮箱、配置邮箱或服务商人员邮箱兜底。
- `APPLYMENT_STATE_TO_BE_CONFIRMED` 在本项目普通服务商链路中解释为待账户验证，前端应引导法人扫码验证或账户汇款验证，不得展示“开户意愿确认”。
- `APPLYMENT_STATE_TO_BE_SIGNED` 和签约链接表示待商户签约；`APPLYMENT_STATE_SIGNING` 表示微信侧开通权限中；这些都是微信侧待办或处理中状态，不是本地开户完成。
- 开户完成点是微信申请单进入 `APPLYMENT_STATE_FINISHED` 且返回 `sub_mchid`，并且本地商户支付配置激活成功；普通服务商完成态不得再查询或等待 `AUTHORIZE_STATE_AUTHORIZED`。
- `apply4subject`、`商户开户意愿确认`、`AUTHORIZE_STATE_*` 不属于当前普通服务商进件完成门槛；不得在 `ordinaryserviceprovider` 生产 contracts/client/errorcodes、worker 恢复、API 状态聚合或小程序页面中重新引入。

商户小程序开户用户流程摘要：

1. 商户在提交页填写结算账户资料和超级管理员邮箱，提交后页面保持短时间长 loading，并自动轮询申请单状态。
2. 若微信返回签约、法人扫码验证或账户验证待办，小程序自动进入微信待办页，展示二维码、汇款账号/备注等必要动作，并提供“完成后刷新状态”。
3. 若微信仍在审核或开通中，小程序回到开户首页展示处理中状态，不要求用户用“手动刷新”作为正常路径；页面重入和返回时按状态刷新策略恢复。
4. 当状态进入 `APPLYMENT_STATE_FINISHED + sub_mchid` 且本地激活成功，开户首页展示“微信支付已开通”、微信支付商户号、结算账户入口和后续说明。
5. 终态页只说明平台内订单流水/结算记录查看路径；商户余额、提现和微信商户平台操作以微信支付商户平台或微信支付商家助手为准，平台小程序不得承诺代商户提现。

- 特约商户进件-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012062365.md
- 特约商户进件-接入前准备：https://pay.weixin.qq.com/doc/v3/partner/4012062375.md
- 特约商户进件-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012062379.md
- 特约商户进件-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4016058480.md
- 特约商户进件-提交申请单：https://pay.weixin.qq.com/doc/v3/partner/4012719997.md
- 特约商户进件-申请单号查询申请状态：https://pay.weixin.qq.com/doc/v3/partner/4012697052.md
- 特约商户进件-业务申请编号查询申请状态：https://pay.weixin.qq.com/doc/v3/partner/4012697168.md
- 特约商户进件-修改结算账户：https://pay.weixin.qq.com/doc/v3/partner/4012761102.md
- 特约商户进件-查询结算账户：https://pay.weixin.qq.com/doc/v3/partner/4012761113.md
- 特约商户进件-查询结算账户修改申请状态：https://pay.weixin.qq.com/doc/v3/partner/4012761120.md
- 特约商户进件-图片上传：https://pay.weixin.qq.com/doc/v3/partner/4012760490.md
- 特约商户进件-视频上传：https://pay.weixin.qq.com/doc/v3/partner/4012761084.md
- 商户开户意愿确认-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012064820.md（非普通服务商：产品主体为渠道商）
- 商户开户意愿确认-流程指引：https://pay.weixin.qq.com/doc/v3/partner/4012064824.md（非普通服务商：流程主体为渠道商）
- 商户开户意愿确认-接入前准备：https://pay.weixin.qq.com/doc/v3/partner/4012064828.md（非普通服务商主链路：属于开户意愿确认能力组，仅作角色边界参考）
- 商户开户意愿确认-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012064832.md（非普通服务商：开发主体为渠道商/从业机构）
- 商户开户意愿确认-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4016644196.md（非普通服务商：开户意愿确认能力组参考）
- 商户开户意愿确认-提交申请单：https://pay.weixin.qq.com/doc/v3/partner/4012722388.md（非普通服务商接口：支持商户为【渠道商】【从业机构（银行）】【从业机构（支付机构）】）
- 商户开户意愿确认-撤销申请单：https://pay.weixin.qq.com/doc/v3/partner/4012697627.md（非普通服务商接口：支持商户为【渠道商】【从业机构（银行）】【从业机构（支付机构）】）
- 商户开户意愿确认-查询申请单审核结果：https://pay.weixin.qq.com/doc/v3/partner/4012697715.md（非普通服务商接口：支持商户为【渠道商】【从业机构（银行）】【从业机构（支付机构）】）
- 商户开户意愿确认-获取商户开户意愿确认状态：https://pay.weixin.qq.com/doc/v3/partner/4012467549.md（非普通服务商接口：支持商户为【渠道商】【从业机构（银行）】【从业机构（支付机构）】）
- 商户开户意愿确认-图片上传：https://pay.weixin.qq.com/doc/v3/partner/4012760509.md（注意：该上传接口页标注支持【普通服务商】，但其所在开户意愿确认能力组不适用于当前身份；普通服务商进件图片上传使用“特约商户进件-图片上传”）
- 商户平台处置通知-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012064844.md
- 商户平台处置通知-接入前准备：https://pay.weixin.qq.com/doc/v3/partner/4012064851.md
- 商户平台处置通知-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012064853.md
- 商户平台处置通知-业务示例代码：https://pay.weixin.qq.com/doc/v3/partner/4015949382.md
- 商户平台处置通知-查询商户违规通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012471327.md
- 商户平台处置通知-修改商户违规通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012471330.md
- 商户平台处置通知-创建商户违规通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012471333.md
- 商户平台处置通知-删除商户违规通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012471334.md
- 商户平台处置通知-商户平台处置记录回调通知：https://pay.weixin.qq.com/doc/v3/partner/4012079216.md
- 商户被管控能力及原因查询-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012165270.md
- 商户被管控能力及原因查询-查询子商户管控情况：https://pay.weixin.qq.com/doc/v3/partner/4012803072.md
- 不活跃商户身份核实-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012064898.md（非当前普通服务商主链路：API 页未标注【普通服务商】）
- 不活跃商户身份核实-接入前准备：https://pay.weixin.qq.com/doc/v3/partner/4012064902.md（非当前普通服务商主链路：API 页未标注【普通服务商】）
- 不活跃商户身份核实-关键概念：https://pay.weixin.qq.com/doc/v3/partner/4012064904.md（非当前普通服务商主链路：API 页未标注【普通服务商】）
- 不活跃商户身份核实-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012064909.md（非当前普通服务商主链路：API 页未标注【普通服务商】）
- 不活跃商户身份核实-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4012064915.md（非当前普通服务商主链路：API 页未标注【普通服务商】）
- 不活跃商户身份核实-发起不活跃商户身份核实：https://pay.weixin.qq.com/doc/v3/partner/4012471357.md（非普通服务商接口：支持商户为【平台商户】）
- 不活跃商户身份核实-查询不活跃商户身份核实结果：https://pay.weixin.qq.com/doc/v3/partner/4012471359.md（非普通服务商接口：支持商户为【平台商户】）

#### 4.10.3 普通服务商小程序支付

- 小程序支付-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012085810.md
- 小程序支付-权限申请：https://pay.weixin.qq.com/doc/v3/partner/4012076731.md
- 小程序支付-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012076732.md
- 小程序支付-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4013352071.md
- 小程序支付-JSAPI/小程序下单：https://pay.weixin.qq.com/doc/v3/partner/4012759974.md
- 小程序支付-小程序调起支付：https://pay.weixin.qq.com/doc/v3/partner/4012085827.md
- 小程序支付-支付成功回调通知：https://pay.weixin.qq.com/doc/v3/partner/4012085801.md
- 小程序支付-微信支付订单号查询订单：https://pay.weixin.qq.com/doc/v3/partner/4012738973.md
- 小程序支付-商户订单号查询订单：https://pay.weixin.qq.com/doc/v3/partner/4012760115.md
- 小程序支付-关闭订单：https://pay.weixin.qq.com/doc/v3/partner/4012760108.md
- 小程序支付-申请退款：https://pay.weixin.qq.com/doc/v3/partner/4012760121.md
- 小程序支付-查询单笔退款（通过商户退款单号）：https://pay.weixin.qq.com/doc/v3/partner/4012760128.md
- 小程序支付-发起异常退款：https://pay.weixin.qq.com/doc/v3/partner/4013352278.md
- 小程序支付-退款结果回调通知：https://pay.weixin.qq.com/doc/v3/partner/4012085802.md
- 小程序支付-申请所有/单个子商户交易账单：https://pay.weixin.qq.com/doc/v3/partner/4012760132.md
- 小程序支付-申请服务商资金账单：https://pay.weixin.qq.com/doc/v3/partner/4012760136.md
- 小程序支付-下载账单：https://pay.weixin.qq.com/doc/v3/partner/4012085803.md

#### 4.10.4 普通服务商小程序合单支付

- 合单支付-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012079378.md
- 合单支付-权限申请：https://pay.weixin.qq.com/doc/v3/partner/4013461849.md
- 小程序合单支付-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012079334.md
- 小程序合单支付-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012166836.md
- 小程序合单支付-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4013462619.md
- 小程序合单支付-小程序合单下单：https://pay.weixin.qq.com/doc/v3/partner/4012758246.md
- 小程序合单支付-小程序调起支付：https://pay.weixin.qq.com/doc/v3/partner/4012166847.md
- 小程序合单支付-查询合单订单：https://pay.weixin.qq.com/doc/v3/partner/4013462520.md
- 小程序合单支付-关闭合单订单：https://pay.weixin.qq.com/doc/v3/partner/4013462566.md
- 小程序合单支付-合单订单支付成功回调通知：https://pay.weixin.qq.com/doc/v3/partner/4013462574.md
- 小程序合单支付-申请退款：https://pay.weixin.qq.com/doc/v3/partner/4013462579.md
- 小程序合单支付-查询单笔退款（按商户退款单号）：https://pay.weixin.qq.com/doc/v3/partner/4013462581.md
- 小程序合单支付-发起异常退款：https://pay.weixin.qq.com/doc/v3/partner/4013462582.md
- 小程序合单支付-退款结果回调通知：https://pay.weixin.qq.com/doc/v3/partner/4013462586.md
- 小程序合单支付-申请所有/单个子商户交易账单：https://pay.weixin.qq.com/doc/v3/partner/4013462604.md
- 小程序合单支付-申请服务商资金账单：https://pay.weixin.qq.com/doc/v3/partner/4013462607.md
- 小程序合单支付-下载账单：https://pay.weixin.qq.com/doc/v3/partner/4013462614.md
- 合单支付-商户号绑定 AppID 操作说明：https://pay.weixin.qq.com/doc/v3/partner/4013462628.md

#### 4.10.5 普通服务商订单退款

- 订单退款-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4013080622.md
- 订单退款-权限申请/解除：https://pay.weixin.qq.com/doc/v3/partner/4013080630.md
- 订单退款-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4013080623.md
- 订单退款-业务示例代码：https://pay.weixin.qq.com/doc/v3/partner/4015217325.md
- 订单退款-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4013080629.md
- 订单退款-申请退款：https://pay.weixin.qq.com/doc/v3/partner/4013080625.md
- 订单退款-查询单笔退款（通过商户退款单号）：https://pay.weixin.qq.com/doc/v3/partner/4013080626.md
- 订单退款-发起异常退款：https://pay.weixin.qq.com/doc/v3/partner/4013080627.md
- 订单退款-退款结果通知：https://pay.weixin.qq.com/doc/v3/partner/4013080628.md
- 订单退款-退款操作指引：https://pay.weixin.qq.com/doc/v3/partner/4013080632.md
- 订单退款-微信支付退款最佳实践：https://pay.weixin.qq.com/doc/v3/partner/4014960215.md

#### 4.10.6 普通服务商分账

- 分账-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4012072582.md
- 分账-接入前准备：https://pay.weixin.qq.com/doc/v3/partner/4012072589.md
- 分账-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012072601.md
- 分账-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4014547107.md
- 分账-请求分账：https://pay.weixin.qq.com/doc/v3/partner/4012690683.md
- 分账-查询分账结果：https://pay.weixin.qq.com/doc/v3/partner/4012466850.md
- 分账-请求分账回退：https://pay.weixin.qq.com/doc/v3/partner/4012466854.md
- 分账-查询分账回退结果：https://pay.weixin.qq.com/doc/v3/partner/4012466858.md
- 分账-解冻剩余资金：https://pay.weixin.qq.com/doc/v3/partner/4012466860.md
- 分账-查询剩余待分金额：https://pay.weixin.qq.com/doc/v3/partner/4012457927.md
- 分账-查询最大分账比例：https://pay.weixin.qq.com/doc/v3/partner/4012466864.md
- 分账-添加分账接收方：https://pay.weixin.qq.com/doc/v3/partner/4012690944.md
- 分账-删除分账接收方：https://pay.weixin.qq.com/doc/v3/partner/4012466868.md
- 分账-分账动账通知：https://pay.weixin.qq.com/doc/v3/partner/4012075216.md
- 分账-申请分账账单：https://pay.weixin.qq.com/doc/v3/partner/4012761140.md
- 分账-下载账单：https://pay.weixin.qq.com/doc/v3/partner/4012075366.md
- 分账-分账失败处理指引：https://pay.weixin.qq.com/doc/v3/partner/4015504885.md

#### 4.10.7 普通服务商账单下载

- 下载账单-产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4013080592.md
- 下载账单-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4013080593.md
- 下载账单-业务示例代码：https://pay.weixin.qq.com/doc/v3/partner/4015988147.md
- 下载账单-常见问题：https://pay.weixin.qq.com/doc/v3/partner/4013080602.md
- 下载账单-申请所有/单个特约商户交易账单：https://pay.weixin.qq.com/doc/v3/partner/4013080595.md
- 下载账单-申请服务商资金账单：https://pay.weixin.qq.com/doc/v3/partner/4013080596.md
- 下载账单-下载账单：https://pay.weixin.qq.com/doc/v3/partner/4013080597.md
- 下载账单-交易账单详细说明：https://pay.weixin.qq.com/doc/v3/partner/4013080599.md
- 下载账单-资金账单详细说明：https://pay.weixin.qq.com/doc/v3/partner/4013080600.md
- 下载账单-平台下载账单操作指引：https://pay.weixin.qq.com/doc/v3/partner/4013080601.md

### 4.11 直连支付组

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

- 直连支付-商家转账
- 产品介绍：https://pay.weixin.qq.com/doc/v3/merchant/4012711988.md
- 开发指引：https://pay.weixin.qq.com/doc/v3/merchant/4012715211.md
- 发起转账：https://pay.weixin.qq.com/doc/v3/merchant/4012716434.md
- JSAPI调起用户确认收款：https://pay.weixin.qq.com/doc/v3/merchant/4012716430.md
- 撤销转账：https://pay.weixin.qq.com/doc/v3/merchant/4012716458.md
- 商户单号查询转账单：https://pay.weixin.qq.com/doc/v3/merchant/4012716437.md
- 微信单号查询转账单：https://pay.weixin.qq.com/doc/v3/merchant/4012716457.md
- 商家转账回调通知：https://pay.weixin.qq.com/doc/v3/merchant/4012712115.md
- 商户单号申请电子回单:https://pay.weixin.qq.com/doc/v3/merchant/4012716452.md
- 商户单号查询电子回单:https://pay.weixin.qq.com/doc/v3/merchant/4012716436.md
- 微信单号申请电子回单：https://pay.weixin.qq.com/doc/v3/merchant/4012716456.md
- 微信单号查询电子回单：https://pay.weixin.qq.com/doc/v3/merchant/4012716455.md
- 下载电子回单：https://pay.weixin.qq.com/doc/v3/merchant/4013866774.md
- 常见问题:https://pay.weixin.qq.com/doc/v3/merchant/4013778940.md
- 设置接口安全IP:https://pay.weixin.qq.com/doc/v3/merchant/4013751010.md
- 订单失败原因说明:https://pay.weixin.qq.com/doc/v3/merchant/4013774966.md
- 转账场景报备信息字段传参说明-企业赔付:https://pay.weixin.qq.com/doc/v3/merchant/4013774589.md

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
