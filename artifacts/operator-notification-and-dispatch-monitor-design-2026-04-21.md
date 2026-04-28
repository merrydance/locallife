# 运营商通知中心与区域待接单监控设计

## 目标

这份文档用于设计运营商小程序新增的通知组合能力，覆盖：

- 分类通知列表
- 通知详情
- 运营商区域内待接单监控
- 订单超过 3 分钟无人接单的提醒机制

同时明确一个工程原则：后续运营商小程序新增能力，统一按高内聚、低耦合方式落地，不把新能力继续塞进现有 dashboard 聚合代码里。

## 先说结论

我建议把这次需求拆成两个相邻但独立的任务域，而不是混成一个超级 dashboard：

### 任务域一：运营商通知中心

- 页面一：通知列表
- 页面二：通知详情
- 负责“看见什么异常、为什么收到、应该去哪里处理”

### 任务域二：区域待接单监控

- 页面一：区域待接单大厅
- 负责“看当前区域还有哪些单没人接、哪些已经超过 3 分钟、应该去看哪张订单”

两者关系是：

- 通知中心负责事件入口和提醒承接
- 待接单大厅负责同类订单的集中查看
- 通知详情页可以跳转到单个订单详情，或者跳转到某个区域的待接单大厅

这比把“通知流 + 区域监控 + 平台告警 + dashboard 摘要”全部塞回一个页面更稳，也更符合高内聚低耦合。

## 当前后端真实情况

先把真相列清楚，避免设计建立在错误前提上。

### 已有能力

#### 1. 通用通知中心已存在

当前后端已经开放了通用通知接口：

- `GET /v1/notifications`
- `GET /v1/notifications/unread/count`
- `PUT /v1/notifications/:id/read`
- `PUT /v1/notifications/read-all`
- `DELETE /v1/notifications/:id`
- `GET /v1/notifications/preferences`
- `PUT /v1/notifications/preferences`

这说明“通知列表、未读数、已读状态、偏好设置”已经有基础能力，不需要从零设计通知存储模型。

#### 2. 通知数据库层支持单条读取

数据库生成层已经有 `GetNotification`，说明单条通知详情在存储层是可读的。

但当前 API 没有开放：

- `GET /v1/notifications/:id`

所以“通知详情页”现在不能按正式能力闭环上线，至少需要后端补一个 detail 读取接口。

#### 3. 配送池和超时未接单数据层已存在

数据库层已有两个关键查询：

- `ListDeliveryPool`：列出配送池待接单
- `ListPendingDeliveriesBefore`：查询指定时间点之前仍然 pending 的配送单

这说明“区域待接单大厅”和“超过阈值未接单提醒”在数据层不是空白。

#### 4. 骑手侧已有抢单大厅能力

当前已有骑手侧配送能力：

- `GET /v1/delivery/recommend`
- `POST /v1/delivery/grab/:order_id`

这条链路证明“待接单池”在业务上真实存在。

### 当前缺口

#### 1. 运营商没有自己的通知读接口组合

当前只有通用 `/v1/notifications`，没有运营商专属读模型，例如：

- `GET /v1/operators/me/notifications`
- `GET /v1/operators/me/notifications/:id`
- `GET /v1/operators/me/notifications/summary`

这意味着如果直接复用当前通用通知页，会把运营商和普通用户通知模型耦在一起。

#### 2. 运营商没有自己的区域待接单大厅接口

当前没有真实开放的 operator 路由，例如：

- `GET /v1/operator/regions/:region_id/delivery-pool`
- `GET /v1/operator/regions/:region_id/delivery-pool/summary`

所以运营商“看自己区域抢单大厅”目前不能只靠前端拼装实现。

#### 3. 平台 alerts 不能拿来替代运营商通知中心

当前存在：

- `GET /v1/platform/alerts`
- `GET /v1/platform/ws`

这是平台侧能力，不应该继续被运营商页面混用。

#### 4. 当前未接单告警阈值不是 3 分钟

现有调度逻辑里：

- 20 分钟未接单：做延迟告警处理
- 60 分钟未接单：自动取消并退款

也就是说，“3 分钟无人接单通知运营商”是一个新的业务规则，不是当前后端已有能力。

而且现实现更偏向商户通知，不是成熟的运营商通知闭环。

## 推荐产品方案

我建议分两期做。

### 第一期：先上通知中心 + 3 分钟超时提醒

这是最小可落地方案。

包含：

- 运营商通知列表页
- 运营商通知详情页
- 订单超过 3 分钟无人接单时，给对应区域运营商发通知
- dashboard 只显示“未读通知摘要”，不内嵌长通知流

不包含：

- 运营商在小程序里直接参与抢单
- 运营商直接改派骑手
- 运营商直接操作配送池

### 第二期：补区域待接单大厅

在通知中心跑稳之后，再加：

- 运营商区域待接单大厅页
- 按区域查看当前待接单列表
- 明确区分“全部待接单”和“超过 3 分钟未接单”

这样做的好处是：

- 一期先解决“运营商能感知异常”
- 二期再解决“运营商能集中查看区域问题”
- 不会因为大厅接口未补齐而卡住通知中心上线

## 为什么不建议直接塞回 dashboard

dashboard 当前已经承担：

- 区域概览
- 商户摘要
- 骑手摘要
- 财务概览

如果再把下面这些都堆进去：

- 分类通知流
- 区域待接单大厅
- 3 分钟超时告警
- 实时订阅

页面会退化成一个大而全的工作台，最终变成：

- 首页过重
- 首屏请求过多
- 失败态互相污染
- 一块数据失败带崩全页
- 模块边界越来越差

这不符合高内聚低耦合。

正确做法是：

- dashboard 只保留摘要和跳转入口
- 通知中心单独成页
- 待接单大厅单独成页

## 页面与模块设计

## 一、运营商通知中心

### 页面结构

建议新增两页：

- `pages/operator/notifications/index`
- `pages/operator/notifications/detail/index`

### 列表页职责

列表页只做四件事：

- 看未读数
- 按分类筛选
- 看通知摘要
- 进入通知详情

不要在列表页里直接承载复杂处理动作。

### 详情页职责

详情页只做三件事：

- 展示完整通知内容
- 展示关联业务上下文
- 提供一个明确跳转动作

例如：

- 跳订单详情
- 跳区域待接单大厅
- 跳食安案件详情

详情页不做“处理台”。

### 推荐分类

运营商通知分类不要沿用普通用户的消费端心智，建议按运营商任务域分：

- 全部
- 待接单监控
- 配送异常
- 风险与协同
- 系统通知

第一期最少先落两类：

- 待接单监控
- 系统通知

### 第一期待支持的通知类型

建议先支持以下 operator category：

- `dispatch_timeout_3m`：订单超过 3 分钟无人接单
- `dispatch_hall_digest`：区域待接单概览提醒
- `system`：系统通知

如果后续要扩充，再加：

- `food_safety_case`
- `merchant_risk`
- `rider_capacity_risk`

## 二、区域待接单大厅

### 页面结构

建议新增一页：

- `pages/operator/dispatch-hall/index`

### 页面职责

这个页面只负责“看当前区域哪些单没人接”。

不负责：

- 抢单
- 改派
- 调度决策提交

因为这些能力当前后端没有开放给运营商。

### 页面信息结构

建议按下面结构组织：

#### 1. 区域选择

- 如果运营商只有一个区域，直接固定展示
- 如果有多个区域，使用顶部区域切换
- 因为运营商区域通常不多，不需要复杂分页或复杂筛选展开区

#### 2. 概览摘要

- 当前待接单总数
- 超过 3 分钟未接单数
- 最早待接单已等待多久

#### 3. 列表分组

- 超过 3 分钟未接单
- 其他待接单

这样首屏先看风险订单，再看普通待接单。

### 单卡片字段建议

每张待接单卡片展示：

- 订单号
- 商户名
- 所属区域
- 下单时间
- 当前等待时长
- 配送费
- 预计取货时间
- 是否超过 3 分钟

可跳转动作只建议保留：

- 查看订单详情
- 查看通知详情

不要伪装成可操作大厅。

## 高内聚低耦合的前端架构建议

这部分是核心。

## 一、不要继续把新能力塞进 `services/operator-console.ts`

`operator-console.ts` 现在已经承担 dashboard 聚合。

通知中心和区域待接单监控不要再挂进去。

否则会继续扩大耦合：

- dashboard 和通知中心耦合
- dashboard 和区域监控耦合
- 首页聚合与详情页逻辑耦合

建议新开独立领域模块。

## 二、推荐目录边界

建议按任务域拆分：

### 1. API 层

- `weapp/miniprogram/api/operator-notification.ts`
- `weapp/miniprogram/api/operator-dispatch-monitor.ts`

职责：

- 只负责请求接口
- 只声明入参与返回结构
- 不做页面业务判断

### 2. Service 层

- `weapp/miniprogram/services/operator-notification-center.ts`
- `weapp/miniprogram/services/operator-dispatch-monitor.ts`

职责：

- 组织页面需要的数据
- 做 view model 适配
- 处理分类映射、状态文案、跳转目标映射

不要把页面 setData 逻辑塞进 service。

### 3. 页面层

- `weapp/miniprogram/pages/operator/notifications/index.*`
- `weapp/miniprogram/pages/operator/notifications/detail/index.*`
- `weapp/miniprogram/pages/operator/dispatch-hall/index.*`

职责：

- 页面状态管理
- 事件绑定
- 首屏加载、分页、重试、重入恢复

### 4. 组件层

如果页面复杂度升高，再拆领域组件：

- `components/operator-notification-list/`
- `components/operator-notification-card/`
- `components/operator-dispatch-summary/`
- `components/operator-dispatch-order-card/`

要求：

- 组件只接数据和事件
- 不在共享组件里藏 API 调用
- 不在共享组件里藏 route jump

这就是低耦合。

## 三、dashboard 只保留摘要入口

dashboard 新增的内容应该只有：

- 未读通知数
- 最近一条高优先级提醒摘要
- 一个“查看通知”入口
- 一个“查看待接单大厅”入口

不要把完整通知列表和完整待接单列表直接挂在首页。

## 推荐后端接口设计

为了保持角色边界清晰，我不建议继续直接把 operator 页面绑死到通用 `/v1/notifications`。

更干净的方式是：

## 一、读接口走 operator 专属读模型

建议新增：

- `GET /v1/operators/me/notifications`
- `GET /v1/operators/me/notifications/summary`
- `GET /v1/operators/me/notifications/:id`

说明：

- 底层存储仍然可以复用通用 `notifications` 表
- 但 operator 读接口自己负责筛选 operator 可见类别和结构
- 这样普通用户通知中心和运营商通知中心不会互相绑死

### 列表接口建议字段

- `id`
- `category`
- `level`
- `title`
- `summary`
- `is_read`
- `created_at`
- `region_id`
- `region_name`
- `related_type`
- `related_id`
- `deep_link_type`
- `extra_data`

### summary 接口建议字段

- `unread_total`
- `unread_by_category`
- `highest_level_unread`
- `latest_actionable_notification`

### detail 接口建议字段

- 列表摘要字段全部保留
- `content`
- `action_label`
- `action_target`
- `business_snapshot`

## 二、区域待接单大厅走 operator 专属区域接口

建议新增：

- `GET /v1/operator/regions/:region_id/delivery-pool/summary`
- `GET /v1/operator/regions/:region_id/delivery-pool`

### summary 字段建议

- `region_id`
- `pending_total`
- `timeout_over_3m_total`
- `oldest_wait_seconds`
- `latest_refresh_at`

### 列表字段建议

- `order_id`
- `order_no`
- `merchant_id`
- `merchant_name`
- `region_id`
- `region_name`
- `delivery_id`
- `delivery_fee`
- `created_at`
- `wait_seconds`
- `is_timeout_over_3m`
- `expected_pickup_at`
- `status`

## 三、3 分钟提醒走新的事件规则，不复用 20 分钟规则

建议新增单独业务规则：

- 当配送单进入 `pending` 后，首次超过 3 分钟仍未接单
- 给所属区域运营商发一条通知
- 同一配送单同一阈值只发一次

推荐去重键：

- `delivery_id + event_type + threshold`

例如：

- `delivery_123_timeout_3m`

这样可以避免轮询任务重复发通知。

## 推荐通知载荷设计

通知 payload 建议最少带这些字段：

- `type`: `delivery` 或 `system`
- `category`: `dispatch_timeout_3m`
- `level`: `warning`
- `region_id`
- `region_name`
- `order_id`
- `order_no`
- `delivery_id`
- `merchant_id`
- `merchant_name`
- `wait_seconds`
- `deep_link_type`
- `deep_link_target`

其中：

- `deep_link_type = order_detail`
- 或 `deep_link_type = dispatch_hall`

这样前端详情页不需要写死跳转规则。

## 页面交互建议

## 1. 通知列表页

首屏结构建议：

- 顶部导航
- 未读摘要行
- 分类 tabs
- 通知列表
- 首屏失败态 / 空态

不要放顶部解释性大卡片。

## 2. 通知详情页

建议结构：

- 标题
- 事件状态标签
- 完整正文
- 业务摘要块
- 一个主按钮

主按钮示例：

- 查看订单
- 查看区域待接单大厅

## 3. 待接单大厅页

建议结构：

- 区域切换
- 摘要块
- 超时未接单分组
- 其他待接单分组

首屏重点永远是超时组。

## 异步与恢复策略

这块必须提前收住，不然后面容易出假状态。

### 第一阶段建议

- 列表页与大厅页先用下拉刷新 + `onShow` 回读
- dashboard 只拉 unread summary
- 先不上 operator websocket

原因：

- 当前 operator 还没有自己的 websocket 读模型
- 如果强行复用平台 websocket，会再次产生角色边界污染

### 第二阶段再考虑实时

等 operator 通知模型稳定后，再考虑：

- operator 专属 websocket
- 或 operator 专属长轮询 summary

## 风险与边界

### 1. 不要把平台 alerts 当 operator 通知中心

这是角色边界错误。

### 2. 不要把 rider 抢单大厅当 operator 大厅直接复用

骑手大厅是“可接可抢”。

运营商大厅应该是“区域监控只读”。

二者不是一个任务域。

### 3. 不要在前端伪造 3 分钟告警

3 分钟超时提醒属于业务事件，不是前端自己本地定时算一下就可以。

必须以后端统一规则为准。

### 4. 不要在通知详情页硬编码所有跳转

最好由后端返回跳转目标类型，前端只负责映射。

## 推荐落地顺序

### P1. 先补后端最小读能力

- operator 通知列表
- operator 通知 summary
- operator 通知 detail
- 3 分钟未接单通知事件

### P2. 再上小程序通知中心

- operator 通知列表页
- operator 通知详情页
- dashboard 未读摘要入口

### P3. 最后补区域待接单大厅

- 区域待接单 summary
- 区域待接单列表
- 通知详情跳转大厅

## 最小实施建议

如果这轮只想先落最小可用版本，我建议这样收口：

### 本轮必须做

- 新增运营商通知列表页
- 新增运营商通知详情页
- 后端新增 operator 通知 detail 接口
- 后端新增 3 分钟无人接单通知规则
- dashboard 只新增一个通知入口和未读角标

### 本轮不要做

- 不做 operator 直接参与抢单
- 不做 operator 改派
- 不做平台 alerts 迁移复用
- 不做复杂实时 websocket

### 下一轮再做

- 区域待接单大厅页
- 大厅摘要接口
- 大厅和通知详情互跳

## 适合后续继续讨论的点

后面如果继续细化，建议直接按下面方式点题：

- “把 operator 通知列表页拆成具体前端任务”
- “把 operator 通知 detail 接口字段定一下”
- “把 3 分钟未接单通知的后端规则拆成事件流”
- “把区域待接单大厅页的信息架构展开”
- “把 dashboard 里的通知入口怎么放讲细一点”