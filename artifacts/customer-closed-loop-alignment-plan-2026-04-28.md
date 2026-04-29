# 顾客闭环对齐计划与任务卡

日期：2026-04-28

## 1. 标准输入与修订目标

本版重新阅读并吸收以下当前权威标准：

- `.github/instructions/weapp-mini-program.instructions.md`
- `.github/standards/weapp/README.md`
- `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`
- `.github/standards/weapp/DESIGN_SYSTEM.md`
- `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
- `.github/standards/weapp/REVIEW_CHECKLIST.md`
- `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md`
- `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`

修订目标：把顾客端能力承接从“接口对齐清单”升级为“标准驱动的任务域交付计划”。后端 API 仍是能力真值，但不再决定页面、组件、卡片、Tab 或入口数量。页面设计必须先从顾客任务、任务域 owner、ViewState、失败恢复和请求预算推导，再落到 TDesign 组件与顾客侧视觉表达。

本计划只定义架构、任务卡、验收口径和风险边界；不进入代码实现。实施需在确认后执行。

## 2. 设计工程底线

### 2.1 必须遵循的设计顺序

每个任务卡实施前必须按以下顺序完成，而不是从 API 文件或旧页面结构开始：

1. 识别顾客当前任务、成功条件和失败后果。
2. 盘点后端真实能力：实体、字段、状态、动作、权限、分页、异步结果和错误语义。
3. 把共同服务同一连续目标的能力组合成任务域。
4. 明确页面组 owner、领域组件 owner 和 workflow owner。
5. 设计 ViewState：loading、empty、error、stale data、disabled、submitting、success、unknown result、retry、re-entry。
6. 决定页面边界、组件边界和首屏信息层级。
7. 查 TDesign Miniprogram 组件能力，再决定组件组合和顾客侧视觉表达。

### 2.2 页面与组件边界

- API 文件只负责访问后端契约，不决定页面边界。
- workflow owner 负责连续任务编排、错误映射、回读策略、重入恢复、防重入和未知结果承接。
- Page 负责页面状态容器、生命周期、渲染分支、动作转发和导航，不承接整条业务流。
- 领域组件负责一个高内聚任务区域，例如优惠券选择、索赔动作、堂食会话状态、订单动作栏。
- 展示组件不得偷偷发请求、跳路由、解释后端状态或吸收页面级 orchestration。
- 同一业务状态只允许一个 owner 解释，不允许列表页、详情页、弹层各写一套条件。

### 2.3 顾客侧页面布局底线

- 首屏必须直接呈现当前任务、关键状态和主操作，不放顶部解释性大卡片。
- 默认不生成说明性文案。只有风险、状态含义、字段约束或下一步动作不写会造成理解错误时，才保留最短必要说明。
- 不做入口墙、按钮墙、卡片墙、字段面板或无意义卡片嵌套。
- 页面 shell 的导航下留白、左右 gutter、内容 padding、底部 safe area 只能由一个层级负责，禁止双重叠加。
- 列表页以筛选、状态和可操作条目为核心；详情页以关键状态、金额、时间、责任对象和主动作链为核心。
- 固定底部操作区必须处理 safe area，主操作保持强强调，次操作不抢主视觉。
- 标签、按钮、弹层、空态、骨架屏和图标动作优先使用 TDesign；官方不支持的内部类覆盖和结构依赖默认禁止。

### 2.4 状态、反馈和请求预算

- 每个数据驱动页面必须区分首屏加载、首屏成功、首屏空态、首屏失败、局部刷新中、动作处理中、静默回读失败但保留可信数据。
- 首屏失败必须页内承接并提供重试，不能只 Toast。
- 一个用户动作只能有一个主反馈通道，不得 Toast、Modal、页内提示、结果页重复解释同一结果。
- 用户可见错误必须映射为中文业务文案，不能暴露后端原始 `message`、英文诊断、SQL、provider 或内部字段名。
- 首屏只加载当前任务完成所需的最小关键数据；禁止 per-item 扇出、跨角色预热、低频子页面抢跑。
- `onShow` 不得无条件全量重拉；返回、前后台切换和重入只刷新真正可能失效的关键真值。

### 2.5 高风险路径门槛

涉及支付、退款、会员余额、索赔、食安、实时/轮询、登录恢复、重复点击、跨页状态恢复的任务至少按 G2/G3 对待：

- 必须说明真实后端真值来自哪里。
- 必须说明失败、未知结果、重复点击、重复进入、弱网、登录恢复时页面如何收住。
- 支付、退款、资金结果不得只信客户端回调，必须回查后端支付单、业务单或详情终态。
- 改后端状态的动作必须有提交中态、防重入和回读路径。
- 交付时必须说明运行了哪些验证、未验证哪些高风险分支、剩余风险落在哪条具体路径。

## 3. 顾客能力组 UI 架构

| 能力组 | 用户任务 | 页面组 owner | workflow / 领域 owner |
|---|---|---|---|
| A. 发现与决策 | 找店、看菜、看套餐、看包间、看优惠、收藏 | `takeout`, `reservation`, `restaurant-detail`, `search`, `category` | `customer-discovery-workflow`, 商户卡片、商品卡片、优惠券领取组件、收藏动作组件 |
| B. 交易与结算 | 加购物车、试算、确认订单、发起支付、支付结果恢复 | `takeout/cart`, `order-confirm`, `payment/result`, `orders` | `customer-checkout-workflow`, 购物车摘要、金额试算、支付恢复、券/余额选择组件 |
| C. 堂食与预订 | 扫码入座、开台、点餐、转台、结账、预订到店 | `dine-in`, `reservation` | `customer-dining-workflow`, 会话状态组件、账单组组件、桌台状态组件 |
| D. 履约与订单 | 订单列表、订单详情、催单、取消、确认收货、配送跟踪 | `orders/list`, `orders/detail`, `orders/tracking` | `customer-order-workflow`, 订单卡片、状态时间线、配送跟踪组件、动作栏组件 |
| E. 售后与安全 | 退款、索赔、确认继续、撤回、食安反馈、证据上传 | `service_center`, `refund-detail`, `orders/detail` | `customer-aftersales-workflow`, 索赔状态组件、食安反馈组件、证据上传组件 |
| F. 复购与资产 | 优惠券、会员、钱包、收藏、评价、地址 | `user_center`, `coupons`, `membership`, `wallet`, `favorites`, `reviews`, `addresses` | `customer-profile-workflow`, 资产摘要、会员卡、券列表、收藏列表、评价列表 |
| G. 通知与状态回流 | 通知列表、未读、偏好、跳转、订单/售后状态回流 | `notification`, 相关详情页 | `customer-notification-workflow`, 通知列表、偏好设置、跳转解析器 |

## 4. 当前问题归类

### 4.1 已知运行问题

- 用户中心进入外卖/订单列表存在报错反馈。第一轮静态审计未发现该问题，说明后续实施必须覆盖真实入口、路由参数、页面重入、编译和运行验证。

### 4.2 架构性问题

- 部分能力按 API 文件散落在 `api/coupon.ts`、`api/personal.ts`、旧 `services/coupon.ts` 等位置，缺少任务域 owner。
- 用户中心聚合优惠券、会员、钱包、收藏、评价、通知，但缺少 profile/asset workflow 边界，容易退化为入口墙。
- 售后链路分散在订单详情、客服中心、支付/退款页之间，缺少统一 aftersales workflow。
- 堂食结账、订单支付结果、会话关闭的恢复路径没有被一个 owner 收口。
- 通知列表、通知偏好、通知删除、状态跳转分散在多个 API 文件和页面中。
- 旧 service 和过期判断会误导后续开发，例如“后端暂无用户优惠券/领取优惠券接口”。

### 4.3 能力承接缺口

- 优惠券顾客可领/可用与商户管理接口边界需要重新收敛。
- 索赔详情缺少确认继续、撤回动作。
- 堂食会话 checkout 没有在前端显式承接。
- 会员加入、充值、交易记录未形成顾客端复购闭环。
- 通知删除、通知偏好未进入主要通知页面组。
- 食安反馈入口未和普通索赔区分清楚。
- 顾客履约状态刷新/实时能力没有定界。

## 5. 实施顺序

第一批稳定顾客主链和阻断项：T0、T1、T2、T3、T4。

第二批补齐复购资产和用户中心能力组：T5、T6、T7。

第三批补齐治理反馈和维护边界：T8、T9、T10。

T0 必须作为强制前置。用户中心外卖/订单列表报错是入口级缺陷，如果不先复现和收口，后续能力组页面会建立在不稳定入口上。

## 6. 任务卡

### T0 顾客入口与外卖/订单列表稳定性基线

优先级：P0  
风险级别：G2  
能力组：交易与结算、履约与订单、复购与资产  
页面组 owner：`user_center`, `orders/list`, `orders/detail`, `takeout`  
workflow owner：`customer-order-workflow`

问题：用户中心进入外卖/订单列表会报错。该问题必须先复现、定界和收口，不作为后续功能接入的附带修复。

范围：

- 盘点用户中心中订单、外卖、优惠券、会员、钱包、客服、评价入口的跳转目标和参数。
- 复现用户中心进入外卖/订单列表的报错，定位路由、页面注册、参数、接口响应、adapter、WXML 渲染或状态恢复问题。
- 为订单列表建立稳定入口协议：默认全部订单；支持 tab、selectMode、order_type 等场景；未知参数不崩溃。
- 收敛订单列表的空态、首屏失败、加载更多失败、返回恢复和筛选切换。

ViewState：首屏 loading、success、empty、error、retry、paging、paging error、filter switching、stale data、re-entry restored。

布局要求：用户中心首屏只保留身份、关键资产和近期待处理；订单入口以“最近订单/全部订单/待处理动作”组织；入口失败用就地错误态和重试承接，不做大段解释。

请求预算：首屏不为每个订单扇出详情；`onShow` 只刷新可能失效的列表真值；从支付结果或售后返回时保留筛选和最近可信数据。

验收：

- 从用户中心进入订单列表、外卖首页、最近订单详情不报错。
- 从支付结果页返回订单列表不报错。
- 从售后选择订单入口进入列表不报错。
- 订单列表在无订单、接口失败、分页结束、筛选切换时都有可信状态。
- 执行 `npm run quality:check`，或记录无法执行原因和剩余风险。

### T1 发现与优惠券能力组重构

优先级：P0  
风险级别：G2  
能力组：发现与决策、交易与结算、复购与资产  
页面组 owner：`restaurant-detail`, `coupons`, `order-confirm`, `user_center`  
workflow owner：`customer-discovery-workflow`, `customer-profile-workflow`

问题：优惠券不能继续按接口散落。顾客看到的是“可领、已领、可用、不可用原因、结算抵扣”的连续利益，而不是券接口目录。

范围：

- 明确顾客优惠券真值：商户可领取券、我的可用券、指定商户可用券、我的全部券、结算试算券。
- 建立唯一顾客券 workflow/API owner，避免页面直接拼路径。
- 统一餐厅详情、优惠券页、结算页、用户中心资产摘要的券状态 view model。
- 移除或隔离旧 `services/coupon.ts` 的误导性实现。

ViewState：loading、empty、claiming、claim success、claim failed、owned、usable、not usable with reason、expired、used、stale after refresh failure。

布局要求：餐厅详情只在点餐决策附近展示可行动券；我的优惠券按可用、已用、过期组织；结算页只展示当前订单相关券和不可用原因，不把全部券列表塞进主流程。

TDesign 约束：券状态用清晰标签和列表项承接；领取、选择、移除等动作优先用 TDesign 按钮或图标动作；不得新增本地 notice/card 壳只为解释规则。

验收：

- 顾客能在商户页看到可领取券并成功领取。
- 顾客能在我的优惠券页看到已领取、可用、已使用、过期券。
- 下单试算和结算页以后端可用券真值为准。
- 普通顾客不会调用商户员工权限的券管理接口。
- 覆盖无券、可领未领、已领取可用、最低消费不满足、已过期/已用。

### T2 交易/订单/支付恢复能力组收口

优先级：P0  
风险级别：G3  
能力组：交易与结算、履约与订单  
页面组 owner：`takeout/cart`, `order-confirm`, `payment/result`, `orders/list`, `orders/detail`  
workflow owner：`customer-checkout-workflow`, `customer-order-workflow`

问题：订单创建、支付创建、支付结果、订单回读和列表入口必须形成连续任务，不能让确认页、支付结果页、订单列表各自解释支付状态。

范围：

- 收敛购物车、订单确认、支付创建、支付回查、订单详情回读到 checkout/order workflow。
- 明确 `wx.requestPayment` 成功、失败、取消、未知结果后的后端回查策略。
- 统一订单列表与详情的状态 view model、可动作条件和错误映射。
- 验证支付结果页、用户中心、通知、售后选择订单入口回到订单列表的稳定性。

ViewState：cart loading、calculating、submitting、payment invoking、backend confirming、paid、failed、cancelled、unknown result、retry check、order stale、re-entry recovered。

布局要求：确认页核心位置放订单内容、金额和主支付动作；支付结果页展示后端回查后的可信状态；订单列表不做状态说明墙。

高风险门槛：不得把客户端支付成功回调当业务终态；重复点击支付、退出重进、支付未知必须有防重入和回查路径。

验收：

- 创建订单、发起支付、支付成功/失败/取消/未知结果均可恢复到可信订单状态。
- 支付结果页刷新或退出重进后仍以后端结果为准。
- 订单列表和详情对同一状态给出一致动作。
- 重复点击支付不会创建不可控重复支付链路。
- 交付说明必须写明未验证的支付失败或未知态分支。

### T3 售后索赔确认继续/撤回承接

优先级：P0  
风险级别：G3  
能力组：售后与安全  
页面组 owner：`service_center`, `orders/detail`, `refund-detail`  
workflow owner：`customer-aftersales-workflow`

问题：后端提供提交、列表、详情、确认继续、撤回，但 weapp 当前只承接提交/列表/详情。售后 UI 应围绕当前工单状态和下一步动作设计。

范围：

- 补齐确认继续和撤回 API，并收敛到 `customer-aftersales-workflow`。
- 基于后端状态、赔付状态、`customer_action_required`、`customer_action` 生成唯一 view model。
- 索赔详情页补齐继续处理、撤回、查看订单、查看退款/赔付状态。
- 客服中心列表只展示关键状态和下一步动作，不做长说明。

ViewState：list loading、detail loading、empty、error、submitting claim、action required、continuing、withdrawing、action success pending refresh、action conflict、stale after refresh failure。

布局要求：详情页首屏放状态、金额、责任进展和下一步动作；确认继续/撤回贴近状态或底部动作区；不使用全宽 notice bar 解释每个状态。

高风险门槛：重复点击、409/403/404、异步入队失败、后台状态已变化必须收住；不得只用本地 `setData` 提示成功。

验收：

- 待顾客确认的索赔能继续进入补偿处理。
- 待顾客确认且未进入补偿执行的索赔能撤回。
- 重复点击不会产生重复提交或错误页面状态。
- 409/403/404 错误能给出就地可理解反馈。

### T4 堂食会话 checkout 收口

优先级：P0  
风险级别：G2  
能力组：堂食与预订、交易与结算  
页面组 owner：`dine-in`, `reservation`, `payment/result`  
workflow owner：`customer-dining-workflow`, `customer-checkout-workflow`

问题：堂食和预订不仅是订单创建/支付，还涉及会话、桌台、账单组、到店和关闭状态。后端有 dining session checkout，weapp 需要明确会话收口。

范围：

- 在 `customer-dining-workflow` 中定义预检、开台、菜单、购物车、订单、支付、checkout、重入恢复。
- 补齐 dining session checkout 方法和支付成功后关闭会话策略。
- 预订页统一创建、支付、到店、加菜/改菜、取消的状态 view model。
- 处理会员余额全额支付、微信支付、支付失败/取消、重复支付回查。

ViewState：session loading、open、ordering、checkout calculating、paying、checkout confirming、closed、payment cancelled、payment unknown、re-entry recovered、session conflict。

布局要求：堂食首屏服务当前会话和主动作；预订详情首屏服务时间、人数、状态、支付/到店/取消动作；会话异常用页内状态承接。

验收：

- 堂食支付成功后会话关闭，桌台/账单组状态与后端一致。
- 支付成功但页面退出后，重新进入能通过后端状态恢复。
- 支付失败或取消不关闭会话。
- 重复点击结账不会重复关闭或重复创建支付单。

### T5 用户中心资产 IA 重整

优先级：P1  
风险级别：G1/G2  
能力组：复购与资产、通知与状态回流  
页面组 owner：`user_center`, `coupons`, `membership`, `wallet`, `favorites`, `reviews`, `addresses`  
workflow owner：`customer-profile-workflow`

问题：用户中心不能作为后端能力目录或入口墙。它应该承接顾客身份、关键资产、最近待处理事项和低频入口分层。

范围：

- 重新定义用户中心首屏 IA：身份、关键资产、最近订单/售后、低频工具。
- 统一优惠券、会员、钱包、收藏、评价、地址、通知入口权重。
- 入口全部走对应能力组 owner，不直接拼散落 API 路径。
- 与 T0 联动，确保订单/外卖入口稳定。

ViewState：profile loading、profile fallback、asset loading、asset partial failure、recent order loading、empty task、stale asset data。

布局要求：不做入口墙、卡片墙和顶部说明大卡；首屏优先展示可行动资产和待处理事项；暂停或未开放能力用禁用态或动作旁状态承接，不放假流程。

请求预算：用户中心首屏只加载身份、关键资产、最近待处理；低频入口详情不抢跑；资产局部失败不打空整页。

验收：

- 用户中心核心入口可达且不报错。
- 资产摘要与各详情页后端真值一致。
- 低频入口不挤占订单、资产和售后主任务。
- 页面 shell、gutter、safe area 只有一处负责，不卡片套卡片。

### T6 会员与钱包资金闭环

优先级：P1  
风险级别：G2/G3，取决于是否恢复线上充值  
能力组：复购与资产、交易与结算  
页面组 owner：`membership`, `wallet`, `order-confirm`, `user_center`  
workflow owner：`customer-profile-workflow`, `customer-checkout-workflow`

问题：会员能力不能分散成“会员列表页”和“钱包流水页”两个互不相干的接口承接。顾客看到的是会员资产、可用余额、充值/交易记录和结算抵扣的一组连续能力。

范围：

- 确认会员线上充值是否恢复。
- 若恢复：补齐加入会员、充值、充值支付、交易记录、充值结果恢复。
- 若继续暂停：收敛无效入口，不保留假流程。
- 钱包页统一会员余额汇总、支付账本、退款记录、会员交易记录的边界。
- 结算页使用会员余额时，和钱包/会员页展示口径一致。

ViewState：membership loading、empty、balance stale、ledger loading、ledger error with cached data、recharge disabled、recharge submitting、payment unknown、balance refresh pending。

布局要求：用户中心只展示关键资产摘要；钱包页首屏展示总资产和最近关键流水；会员页以卡片和交易记录服务复购，不放平台会员规则说明大卡。

验收：

- 顾客能看到每张会员卡余额与交易记录。
- 若启用充值，充值支付成功后余额和交易记录可回查。
- 若暂停充值，页面不会暴露无效充值流程。
- 会员余额支付与订单结算的余额展示一致。

### T7 通知中心与状态回流承接

优先级：P1  
风险级别：G1/G2  
能力组：通知与状态回流、履约与订单  
页面组 owner：`notification`, `orders/detail`, `orders/tracking`, `refund-detail`  
workflow owner：`customer-notification-workflow`, `customer-order-workflow`

问题：通知、订单状态、配送状态、支付/退款状态属于同一个“顾客知道进展”的能力组，不能让通知页只做消息列表，订单页各自解释状态。

范围：

- 补齐通知删除、偏好查询、偏好更新。
- 通知跳转以后端 `related_type`/`related_id` 为真值，经 workflow 映射到页面。
- 订单详情、配送跟踪、支付/退款详情统一状态文案与回查策略。
- 未知通知类型进入安全降级，不导致页面异常。

ViewState：notification loading、empty、error、marking read、deleting、preference loading、preference saving、unknown target、target stale after jump。

布局要求：通知中心按任务和时间组织，不展示内部字段原文；订单详情首屏展示当前状态、预计结果、主动作，不放状态枚举说明。

验收：

- 删除通知后列表和未读数一致。
- 偏好更新后刷新仍保持后端状态。
- 未知通知类型不导致页面异常。
- 弱网、退出重进、后台恢复后状态能回查。

### T8 食品安全反馈能力组

优先级：P1  
风险级别：G3  
能力组：售后与安全  
页面组 owner：`orders/detail`, `service_center`  
workflow owner：`customer-aftersales-workflow`

问题：后端有食安上报能力，但顾客端没有清晰入口。食安不能被普通退款/索赔文案吞掉，也不能以解释性页面替代可追踪反馈。

范围：

- 明确食安反馈与普通索赔关系：独立 food-safety report，还是进入 claim `food-safety` 类型。
- 若独立：补齐 API、提交页、证据上传、订单关联、提交后状态。
- 若归入索赔：扩展 claim type，并确认后端裁决、治理、食安 case 能承接。
- 从订单详情和客服中心进入。

ViewState：order selecting、form editing、evidence uploading、upload failed retryable、submitting、submitted pending, submit failed、re-entry draft restored。

布局要求：提交页核心是订单、问题类型、证据、联系方式和提交动作；风险说明只保留必要短文案，靠近字段或提交动作；提交后展示可追踪状态，不只 Toast。

验收：

- 顾客能基于订单提交食安反馈。
- 反馈不会被当成普通退款文案吞掉。
- 后端能产生可治理对象，运营侧可追踪。
- 上传失败、重复提交、退出重进有明确承接。

### T9 顾客履约状态刷新/实时能力定界

优先级：P2  
风险级别：G2  
能力组：履约与订单、通知与状态回流  
页面组 owner：`orders/list`, `orders/detail`, `orders/tracking`, `notification`  
workflow owner：`customer-order-workflow`, `customer-notification-workflow`

问题：后端存在实时能力入口，但顾客履约状态本期应采用 WebSocket、轮询还是关键节点回查，需要明确边界。边界不清会导致页面状态漂移和重复实现。

范围：

- 确认顾客端本期是否接入 WebSocket。
- 若不接入，定义订单列表、详情、配送跟踪、通知跳转后的刷新/回查策略。
- 若接入，定义连接权限、订阅范围、断线重连、后台恢复和重复消息去重。
- 与支付结果、售后详情、配送跟踪状态统一。

ViewState：subscribing、connected、disconnected with cached data、polling、manual refresh、event deduped、stale after resume、fallback to query。

布局要求：实时或刷新状态仅作为状态旁轻量提示，不做全宽解释条；手动刷新可见但不成为主任务噪音。

验收：

- 订单列表、订单详情、配送跟踪在退出重进、后台恢复、弱网后能回到可信状态。
- 未接入实时能力时不展示伪实时文案。
- 接入实时能力时重复消息不会造成重复动作或状态倒退。

### T10 旧 API/service 漂移治理

优先级：P2  
风险级别：G1  
能力组：所有顾客能力组  
页面组 owner：所有顾客侧页面组  
workflow owner：各能力组 owner

问题：旧 `services/coupon.ts`、疑似旧商品/售后路径、分散通知 API 会误导后续开发。治理目标不是把 API 文件机械合并，而是让每个能力组有唯一 owner。

范围：

- 标记并删除无人引用的旧服务，或迁移到能力组 owner。
- 对仍被页面引用但路径过期的函数逐个对齐后端运行时路由。
- 建立顾客能力组到 service/workflow owner 的索引。
- 禁止后续页面直接新增散落 API 调用，除非该能力组 owner 不存在且本任务同步建立。

工程要求：治理不得改变用户可见布局；如涉及页面重构，必须按对应能力组布局原则执行。新增或修改服务时不得继续扩大 protected super service。

验收：

- 顾客相关页面不再引用旧 coupon/product 漂移接口。
- grep 不再出现“后端暂无用户优惠券/领取优惠券接口”这类已过期判断。
- 能用静态搜索证明顾客主链只走对应能力组 owner。
- 新增能力有明确 service/workflow owner，不直接散落到页面 handler。

## 7. 统一验证矩阵

| 能力组 | 最小验证 |
|---|---|
| 用户中心与订单入口 | 用户中心进入订单列表、外卖、最近订单、支付结果回订单列表、售后选择订单入口 |
| 发现与决策 | 外卖首页、搜索、分类、餐厅详情、菜品/套餐详情、收藏动作、无位置/弱网状态 |
| 交易与结算 | 购物车、试算、创建订单、支付创建、支付回查、取消/失败/未知结果恢复、重复点击 |
| 堂食与预订 | 扫码进入、开台、点餐、支付成功 checkout、预订创建/详情/取消/到店、重入恢复 |
| 履约与订单 | 订单列表、详情、配送跟踪、催单、取消、确认收货、返回恢复、弱网刷新 |
| 售后与安全 | 索赔提交/详情/确认继续/撤回、食安反馈、证据上传失败、重复提交 |
| 复购与资产 | 优惠券、会员卡、钱包、收藏、评价、地址，且入口不形成卡片墙 |
| 通知与状态回流 | 通知列表/已读/全读/删除/偏好、通知跳转、未知类型降级、订单/售后状态回查 |
| 架构卫生 | 无旧 service 误导、无散落新增 API、能力组 owner 清晰、无超级 service 扩张 |

## 8. 交付门禁

每张任务卡实施完成前，至少需要给出以下证据：

1. 后端真值来源：字段、状态、权限、分页、异步结果来自哪些运行时路由和契约。
2. 任务域 owner：页面组、workflow、领域组件边界清楚。
3. ViewState 证据：loading、empty、error、submitting、retry、stale data、re-entry 中相关状态均被承接。
4. 页面布局证据：无顶部解释性大卡片、无入口墙/按钮墙/卡片墙、无无意义卡片嵌套、safe area 和 gutter 正确。
5. 请求预算证据：无 per-item 扇出、无低频预加载抢跑、`onShow` 刷新范围受控。
6. 反馈证据：一个动作一个主反馈通道，首屏失败页内承接，错误文案业务化。
7. 高风险证据：支付、索赔、食安、会员资金、实时/轮询等路径写清未知态、重复点击、弱网、重入恢复。
8. 自动化验证：优先从 `weapp/` 执行 `npm run quality:check`；必要时补 `npm run compile`、`npm run lint` 或专项 gate。

## 9. 实施前确认点

1. 用户中心外卖/订单列表报错的可复现路径：从哪个入口、哪个 tab、是否特定账号或特定订单类型。
2. 会员线上充值是否恢复。如果继续暂停，T6 只做只读资金闭环和无效入口收敛。
3. 食安反馈是独立 food-safety report，还是进入 claim `food-safety` 类型。
4. 顾客实时状态本期是否做 WebSocket。如果不做，本期按轮询和回查策略闭环。

## 10. 非目标

- 不把每个后端 API 做成一个页面、卡片或入口。
- 不做顾客端大而全控制台。
- 不为了“解释完整”增加顶部说明卡、全宽提示条或核心区域描述组件。
- 不在前端伪造后端缺失字段、状态、权限或终态。
- 不在确认前进入实现。
