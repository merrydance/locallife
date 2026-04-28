# 运营商通知中心与待接单监控任务计划

## 目标

这份计划用于把“3 分钟未接单提醒运营商”需求收敛成可执行任务，避免在实现时重新漂移回：

- 旧的 20 分钟商户告警语义
- 平台 alerts / platform ws 复用
- operator 订单详情页扩张
- dashboard 大而全聚合

本计划覆盖：

- 后端事件链路
- operator 通知读模型
- operator 小程序通知中心
- dashboard 摘要入口
- 二期待接单大厅预留

## 已确认边界

### 业务语义

- 本需求关注的是“配送单进入 pending 后，3 分钟内没有骑手接单”。
- 不是“订单长时间未送达”。
- 不是“骑手履约超时”。
- 不是“商户不处理导致顾客取消”的补救链路。

### 运营商职责

- 运营商收到的是区域派单异常提醒。
- 运营商不需要看到实际订单详情。
- 运营商不需要在一期里直接操作改单、改派、抢单或进入单订单处理页。

### 页面边界

- 一期只做 operator 通知列表页和通知详情页。
- dashboard 只做通知摘要入口。
- 不做 operator order detail。
- 不做 dispatch hall 页面。
- dispatch hall 放到二期。

### 历史实现处理

- 当前调度器里的 20 分钟商户告警和 60 分钟自动取消只作为旧实现参考。
- 新需求不得直接挂靠这套历史语义继续扩写。
- 若后续替换旧实现，必须单独说明替换范围与回归验证。

## 风险等级

- 该任务按 G2 推进。
- 原因：涉及异步扫描、去重真值、RBAC、角色专属读模型、weapp 页面组扩展和摘要入口变更。

## 范围拆分

## 一期必须完成

### 后端

- 新增 3 分钟未接单 operator 告警事件链路。
- 新增 operator 通知读模型接口：列表、摘要、详情、已读操作。
- 新增按区域查 active operator 收件人的查询。
- 新增去重真值，保证同一 delivery 的同一阈值只发一次。
- 补齐 Casbin 与 Swagger。

### 小程序

- 新增 operator 通知列表页。
- 新增 operator 通知详情页。
- operator dashboard 增加未读角标和最近一条提醒摘要。

## 二期再做

### 后端

- operator 区域待接单 summary 接口。
- operator 区域待接单 list 接口。

### 小程序

- dispatch hall 页面。
- 通知详情跳转 dispatch hall。

## 一期明确不做

- 不做 operator 订单详情页。
- 不做 operator 直接改单。
- 不做 operator 改派骑手。
- 不做 operator 抢单入口。
- 不做平台 alerts / platform ws 复用。
- 不做实时 websocket 读模型。
- 不做 dashboard 内嵌通知流。
- 不做 dashboard 内嵌待接单列表。

## 架构 owner

## 后端 owner

### 1. delivery pending timeout monitor

职责：

- 扫描 pending 且超过 3 分钟未被接单的 delivery
- 负责去重真值
- 只产出 operator 告警事件

不负责：

- 订单详情聚合
- operator 页面 DTO
- dispatch hall 展示结构

### 2. operator notification read model

职责：

- 将 notifications 投影成 operator 专属列表/详情/摘要 DTO
- 做 operator 用户的读取权限校验
- 输出前端需要的 category、level、region summary、deep link 语义

不负责：

- 调度扫描
- 告警触发判断
- 页面 setData 逻辑

### 3. operator dispatch monitor

职责：

- 二期承接按区域读取待接单 summary 与列表

一期状态：

- 只保留 owner 位置，不交付接口

## 前端 owner

### 1. operator notification center

职责：

- 通知列表
- 通知详情
- 已读状态承接
- 下拉刷新与 onShow 回读

### 2. operator dashboard summary

职责：

- 展示未读数量
- 展示最近一条提醒摘要
- 提供通知中心入口

### 3. operator dispatch hall

职责：

- 二期承接区域待接单监控

一期状态：

- 不实现页面，只保留计划位置

## 后端任务拆解

## P1-BE-01 新增去重真值

目标：

- 为“delivery 超过 3 分钟未接单”的 operator 告警建立唯一真值。

建议实现：

- 新增表：`delivery_timeout_alerts`

建议字段：

- `id`
- `delivery_id`
- `region_id`
- `threshold_key`
- `triggered_at`
- `created_at`

唯一约束：

- `unique(delivery_id, threshold_key)`

说明：

- 不复用 `deliveries.is_delayed`。
- `is_delayed` 已经承载旧的 20 分钟链路语义，不能继续混写。

## P1-BE-02 新增 operator 收件人查询

目标：

- 从 region 找到 active operator 的 `user_id`，用于通知 fan-out。

建议落点：

- `locallife/db/query/operator_region.sql`

建议新增查询：

- 按 `region_id` 列 active operator + user_id

输出至少包含：

- `operator_id`
- `user_id`
- `region_id`

## P1-BE-03 新增 3 分钟 pending 扫描链路

目标：

- 扫描 `pending` 且创建时间超过 3 分钟的 delivery
- 为每条 delivery 最多触发一次 operator 告警事件

要求：

- 不继续往 `cleanupStaleDeliveries` 塞新语义
- 单独抽 owner 文件
- 先写去重真值，再发异步任务

建议结构：

- scheduler 负责扫描并写 ledger
- worker 负责收件人 fan-out 和通知落库

## P1-BE-04 新增 operator 通知读接口

建议路径：

- `GET /v1/operators/me/notifications`
- `GET /v1/operators/me/notifications/summary`
- `GET /v1/operators/me/notifications/:id`
- `PUT /v1/operators/me/notifications/:id/read`
- `PUT /v1/operators/me/notifications/read-all`

说明：

- 读接口走 operator 专属模型。
- 底层通知记录仍可复用 `notifications` 表。
- 不直接把顾客侧 `/v1/notifications` 暴露给 operator 页面。

### 列表 DTO

- `id`
- `category`
- `level`
- `title`
- `summary`
- `is_read`
- `created_at`
- `region_id`
- `region_name`
- `deep_link_type`
- `deep_link_target`

### summary DTO

- `unread_total`
- `latest_actionable_notification`

### detail DTO

- 列表字段
- `content`
- `business_snapshot`

`business_snapshot` 一期只放提醒所需摘要，例如：

- `region_name`
- `wait_seconds`
- `threshold_label`

明确不放：

- 订单详情对象
- 商户完整详情
- 配送单完整详情

## P1-BE-05 权限与生成物

必须同步：

- `locallife/casbin/policy.csv`
- Swagger 注解
- `make sqlc`
- `make swagger`

## 小程序任务拆解

## P1-WEAPP-01 新增 operator 通知 API owner

新增：

- `weapp/miniprogram/api/operator-notification.ts`

职责：

- 对接 operator notifications list/summary/detail/read 接口
- 只做请求与 DTO 声明

## P1-WEAPP-02 新增 operator 通知中心 service owner

新增：

- `weapp/miniprogram/services/operator-notification-center.ts`

职责：

- 列表项 view model 投影
- 分类映射
- level 文案映射
- 详情页摘要结构映射

明确不做：

- 页面 setData
- 全局路由编排

## P1-WEAPP-03 新增通知列表页

新增页面：

- `pages/operator/notifications/index`

页面职责：

- 未读数展示
- 分类 tabs
- 通知卡片列表
- 分页加载
- 下拉刷新

首屏结构：

- 顶部导航
- 未读摘要行
- 分类 tabs
- 通知列表
- 空态 / 失败态

明确禁止：

- 顶部解释性大卡片
- dashboard 式多模块拼盘

## P1-WEAPP-04 新增通知详情页

新增页面：

- `pages/operator/notifications/detail`

页面职责：

- 展示完整通知内容
- 展示区域级业务摘要
- 承接已读状态

主按钮策略：

- 一期不跳订单详情
- 一期可不显示按钮
- 或显示“返回运营中心”/“返回通知列表”

## P1-WEAPP-05 更新 operator dashboard 摘要入口

更新：

- `weapp/miniprogram/services/operator-workbench.ts`
- `weapp/miniprogram/pages/operator/dashboard/index.ts`
- 对应 WXML/WXSS

只新增：

- 未读数量
- 最近一条提醒摘要
- 通知中心入口

明确禁止：

- 内嵌通知流
- 内嵌待接单列表
- 回写 `services/operator-console.ts`

## 二期任务预留

## P2-BE-01 operator 区域待接单 summary 接口

建议路径：

- `GET /v1/operator/regions/:region_id/delivery-pool/summary`

字段：

- `region_id`
- `pending_total`
- `timeout_over_3m_total`
- `oldest_wait_seconds`
- `latest_refresh_at`

## P2-BE-02 operator 区域待接单列表接口

建议路径：

- `GET /v1/operator/regions/:region_id/delivery-pool`

字段：

- `order_no`
- `merchant_name`
- `region_name`
- `wait_seconds`
- `delivery_fee`
- `expected_pickup_at`
- `is_timeout_over_3m`

说明：

- 二期页面是区域监控页，不是订单详情页。

## P2-WEAPP-01 dispatch hall 页面

新增：

- `weapp/miniprogram/api/operator-dispatch-monitor.ts`
- `weapp/miniprogram/services/operator-dispatch-monitor.ts`
- `pages/operator/dispatch-hall/index`

二期通知详情页再跳到 dispatch hall。

## 验收标准

## 一期后端验收

- delivery 超过 3 分钟未接单会生成 operator 告警。
- 同一 delivery 不会重复发同一阈值告警。
- operator 只能读到自己的通知。
- 读接口不暴露订单详情对象。
- Casbin 和 Swagger 同步完成。

## 一期前端验收

- operator dashboard 能看到通知摘要入口。
- operator 能进入通知列表页与通知详情页。
- 列表页支持分页和刷新。
- 详情页只展示提醒摘要，不跳订单详情。
- 新能力没有重新挂回 `operator-console.ts`。

## 验证清单

## 后端

- `make sqlc`
- `make swagger`
- `make test-unit`

重点测试：

- 3 分钟 pending 扫描
- 重复扫描幂等
- 区域 fan-out
- operator 通知详情越权读取

## 小程序

- `npm run compile`
- `npm run quality:check`

重点检查：

- dashboard 摘要入口没有膨胀成通知流
- 新 service owner 没有回漂到 super-service
- 通知详情页没有引入订单详情依赖

## 实施顺序

1. 后端先做去重真值和 operator 收件人查询。
2. 后端完成 3 分钟 pending 扫描链路。
3. 后端完成 operator 通知读接口、Casbin、Swagger。
4. 小程序完成 operator 通知 API owner 与 service owner。
5. 小程序完成通知列表页和详情页。
6. 小程序完成 dashboard 摘要入口。
7. 二期再补 dispatch hall。

## 跑偏警报

实现中若出现下面任一情况，必须停下来回看本计划：

- 想把新能力继续加到 `cleanupStaleDeliveries` 的旧 20 分钟逻辑里
- 想直接复用 `/v1/notifications` 给 operator 页面
- 想补 operator 订单详情页作为一期依赖
- 想把通知详情页做成订单工作台
- 想把通知流塞回 operator dashboard
- 想把新 owner 挂回 `services/operator-console.ts`