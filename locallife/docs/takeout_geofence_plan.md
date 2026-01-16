# 围栏/定位事件实施计划（外卖配送）

目的：把“围栏/定位事件触发 picked 或 rider_delivered”的实现步骤固化为可执行清单，避免遗忘。

## 范围
- 配送位置上报入口
- 围栏判断与驻留策略
- 事件落库与幂等
- 状态触发策略（默认不自动推进）
- 通知与日志联动
- 测试与 swagger 更新

## 现状结论
- 目前仅有位置服务接口与手动配送状态流转，未发现围栏/驻留触发实现。

## 实施步骤（可回滚）

### 1. 位置上报入口
- 新增骑手位置上报接口（建议新增 api/rider_location.go）。
- 支持批量上报字段：delivery_id、lat、lng、accuracy、speed、timestamp、source。
- 鉴权：骑手身份 + 仅允许更新自己配送中的单。

### 2. 事件落库
- 新建表 delivery_location_events（或 rider_location_events）。
- 字段建议：
  - id (bigint)
  - delivery_id
  - order_id
  - rider_id
  - lat/lng
  - accuracy
  - speed
  - event_type (arrive_pickup / arrive_dropoff / dwell_pickup / dwell_dropoff / raw)
  - source
  - created_at
  - raw (jsonb, 可选)
- 索引：
  - (delivery_id, created_at)
  - (rider_id, created_at)
- 幂等键：event_hash 或 (delivery_id, event_type, date_trunc('hour', created_at))

### 3. 围栏判断与驻留
- 围栏半径可配置（例如 50m-120m）。
- 驻留判断：连续 N 次在围栏内且持续 >= T 秒。
- 对精度低的上报设置最低精度门槛（例如 accuracy <= 80m）。

## 实时上报与数据存储策略（新增）
- 需要实时上报用于 ETA、围栏/驻留、异常风控与轨迹追溯。
- 原始轨迹不做长期全量存储：
  - 在线计算/短期缓存（分钟级）
  - 事件落库（到店/离店/送达/驻留）
  - 轨迹采样 + TTL（例如 5–10 秒一条，保留 1–7 天）
  - 长期仅保留聚合统计（按订单/骑手）

## WebSocket 与本次开发的关系（新增）
- 当前 WebSocket 仅用于通知推送，不承载位置上报。
- 若决定走 WebSocket 上报，需要新增消息类型与服务端处理逻辑，并与 HTTP 上报保持同一套落库与围栏计算。
- 默认方案仍为 HTTP 批量上报，WebSocket 作为可选增强路径。

### 4. 触发策略（与既定决策一致）
- 默认不自动推进状态，只记录事件 + 作为按钮解锁条件。
- 可选特性开关：允许自动推进。
  - 到店驻留 -> UpdateDeliveryToPickupTx（同步订单至 courier_accepted）。
  - 到达收货点驻留 -> 仅记录“到达”事件，rider_delivered 仍需骑手确认。

### 5. 事务与幂等
- 事件写入与状态变更分离：先写事件，再尝试状态变更。
- 同一 delivery_id + event_type 只触发一次状态更新（幂等判断）。

### 6. 通知与日志
- 触发状态变更时复用现有通知逻辑。
- 状态日志：写入 from/to 状态与操作人（rider）。

### 7. 测试与验证
- 单测：围栏判断、驻留判定、幂等触发。
- 集成：位置上报 -> 事件落库 -> 状态变更（开关开启/关闭两种路径）。
- 更新 swagger。

## 配置建议（默认值）
- geofence_radius_m = 80
- dwell_min_seconds = 60
- dwell_min_samples = 3
- min_accuracy_m = 80

## 风险与回滚
- 风险：误判导致状态误变。
- 方案：默认不开自动推进，仅用于按钮解锁；出现问题可关闭 feature flag。

## TODO（实施时勾选）
- [ ] 新增位置上报 API
- [ ] 新增事件表与索引
- [ ] 实现围栏与驻留计算
- [ ] 完成事件落库与幂等
- [ ] 增加实时上报与存储策略（采样+TTL+事件化落库）
- [ ] 明确 WebSocket 是否承担位置上报（若是，新增消息协议与处理）
- [ ] 接入可选自动推进开关
- [ ] 通知与日志联动
- [ ] 单测/集成测/Swagger
