# 外卖状态/字段扩展实施清单（基于兼容性草案）

## 阶段 1：DDL 准备（无业务改动）
- [ ] orders 表新增列：pickup_code, dispatch_order_id, flow_id, status_hint, badges(jsonb/text[]), exception_state, claim_channel, overtime(bool default false), prep_start_at, ready_at, courier_accept_at, picked_at, rider_delivered_at, user_delivered_at, auto_user_delivered_at。
- [ ] 扩展状态约束/enum，允许新状态：courier_accepted, picked, rider_delivered, user_delivered。
- [ ] deliveries 表（可选）新增 rider_delivered_at（若仅有 delivered_at）、冗余 pickup_code。
- [ ] 生成/审核迁移脚本，确保上线无锁或低锁（PG：ADD COLUMN NULL; enum/check 先放宽）。

## 阶段 2：数据回填（幂等批处理）
- [ ] 回填已完成/配送中订单的时间戳：
  - delivering 且 delivered_at 有值：rider_delivered_at = delivered_at。
  - completed 且无 user_delivered_at：user_delivered_at = completed_at。
- [ ] 初始化 status_hint 为空字符串，badges 为空数组；overtime 默认为 false。
- [ ] deliveries 回填 rider_delivered_at（如 delivered_at 存在）。
- [ ] 验证回填幂等性与耗时，分批/并发控制。

## 阶段 3：服务双写与校验
- [ ] 订单状态推进逻辑：保留旧流同时写入新字段时间戳（courier_accept_at、picked_at 等）。
- [ ] 生成 pickup_code、填充 dispatch_order_id/flow_id，保持对旧接口兼容。
- [ ] 取消/退款闸门：preparing 前可取消/退款，之后进入异常通道（不改表结构）。
- [ ] 状态日志允许新枚举，新增 rider_delivered/user_delivered 记录。
- [ ] 单元/集成测试覆盖新状态写入与旧读路径兼容。

## 阶段 4：API / 响应兼容
- [ ] 列表/详情响应返回新增字段：status_hint、badges、pickup_code_masked、actions、overtime、补充 items 预览；缺失时用旧字段推导。
- [ ] 确认收货接口：支持 delivering 与 rider_delivered -> user_delivered，记录 user_delivered_at。
- [ ] 催单接口：支持 courier_accepted/picked/rider_delivered 判定。
- [ ] 计算/结算使用 user_delivered（缺失时回落 completed/delivered_at）。

## 阶段 5：前端渐进切换
- [ ] 消费 status_hint/badges/items/preview/pickup_code，展示新状态文案；保留对旧字段的回退。
- [ ] 按动作权限 actions 控制按钮（退款/投诉/确认收货）。

## 阶段 6：状态流灰度
- [ ] 灰度开启 ready -> courier_accepted -> picked -> delivering -> rider_delivered -> user_delivered；监控状态分布与异常。
- [ ] 与配送表状态联动（picking/picked/delivering/delivered）；确保幂等重试。
- [ ] 定时任务自动 user_delivered（+2h 可配置）。

## 阶段 7：收敛与回滚预案
- [ ] 观察期后，确认新读路径稳定，可逐步移除旧推导逻辑（可选）。
- [ ] 回滚策略：停用新状态推进/双写；保留新增列无需回退；接口兼容旧状态仍可读。

## 待决策同步到执行计划
- courier_accepted 是否独立状态。
- badges/status_hint 存储类型与多语言需求。
- pickup_code 存储位置（订单 vs 订单+配送）。
- 自动确认宽限时间与 geofence+dwell 阈值。
