# CARD-07 确定账单组金额单一事实来源

状态：已完成

优先级：P1

所属阶段：Phase 2

## 问题目标

在修代码前先明确 billing_groups.total_amount / paid_amount 究竟是持久化聚合值，还是运行时聚合结果。

## 影响范围

- [locallife/db/query/billing_group.sql](locallife/db/query/billing_group.sql#L48)
- [locallife/api/dining_session.go](locallife/api/dining_session.go#L62)
- [locallife/api/billing_group.go](locallife/api/billing_group.go)

## 任务内容

- [x] 梳理所有对 total_amount / paid_amount 的写入点和读取点。
- [x] 选择最终方案：持久化聚合或运行时聚合。
- [x] 记录方案取舍，明确后续 CARD-08 的实施边界。

## 完成定义

- [x] 账单组金额来源方案明确。
- [x] API 层和存储层不会继续各说各话。

## 验证要求

- [x] 输出一份简短决策记录并链接到任务卡。

## 决策记录

- 最终方案：运行时聚合。
- 证据：`billing_groups.total_amount / paid_amount` 仅在创建账单组时初始化，未形成建单、支付、取消、替换、关台的完整维护链路。
- 证据：真实订单归属已经落在 `billing_group_orders`，转台等现有链路也在消费该关联表，而不是依赖 `billing_groups` 主表金额字段。
- 聚合口径：
	- `total_amount` 取账单组下“未取消且未被替换”的关联订单 `orders.total_amount` 总和。
	- `paid_amount` 取账单组下订单状态已进入 `paid/preparing/ready/courier_accepted/picked/delivering/rider_delivered/user_delivered/completed` 的关联订单 `orders.total_amount` 总和。
- 非目标：本轮不再把 `billing_groups.total_amount / paid_amount` 修成持久化账本，也不引入大面积金额回写触点。

## 完成记录

- [x] 方案确定
- [x] 评审完成