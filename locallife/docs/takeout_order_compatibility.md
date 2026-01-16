# 外卖订单状态/字段扩展兼容性设计草案

目标：在不中断现网堂食/预约/外卖流量的前提下，引入细粒度状态与新字段（取餐码、派单流、异常/投诉、前端提示字段等），支持文档中的生命周期。

## 范围
- 订单主表 `orders`
- 配送表 `deliveries`
- 状态日志 `order_status_logs`
- API/服务层（订单、配送、结算、催单/确认收货）

## 状态与字段扩展（建议一步到位但分阶段启用）
- 订单状态新增：`courier_accepted`, `picked`, `rider_delivered`, `user_delivered`。
- 订单新增列：
  - `pickup_code` (varchar, nullable, 长度<=32)
  - `dispatch_order_id` (bigint/uuid，nullable)
  - `flow_id` (bigint/uuid，nullable)
  - `status_hint` (varchar, nullable，短文案)
  - `badges` (text[] 或 jsonb，默认空数组)
  - `exception_state` (varchar, nullable)
  - `claim_channel` (varchar, nullable)
  - `overtime` (bool, default false)
- 订单计时列补充（若缺）：`prep_start_at`, `ready_at`, `courier_accept_at`, `picked_at`, `rider_delivered_at`, `user_delivered_at`, `auto_user_delivered_at`。
- 配送表（deliveries）对齐：保留现有 `picking/picked/delivering/delivered`，补充 `rider_delivered_at`（若仅有 delivered_at 则重用）、`pickup_code`（可冗余存放方便骑手端）。
- 状态日志：无需 schema 变更，但需接受新枚举值。

## 迁移策略
1) **添加列与扩展枚举（非破坏）**
   - 通过 DDL 添加列，设置默认值/nullable，避免锁表时间过长（PG 可用 `ADD COLUMN ... NULL`）。
   - 枚举若使用 check/enum，需要先放宽约束或新增允许值。
2) **回填阶段（幂等批处理）**
   - 对历史订单：
     - 将 `delivering` 且有 delivered_at 的，回填 `rider_delivered_at=delivered_at`，`status` 可保留或待灰度。
     - `completed` 的，若无 rider_delivered_at/user_delivered_at，则设 `user_delivered_at=completed_at`。
     - 生成 `status_hint` 为空字符串，`badges` 为空数组。
   - 配送表：如 delivered_at 存在，回填 rider_delivered_at。
3) **双写/灰度**
   - 服务层在保持旧状态流的同时，开始写入新时间戳/字段（pickup_code、flow_id 等），但对外接口初期仍返回旧字段，避免前端突变。
   - 状态推进时若进入 `delivering`，可同步写 `rider_delivered_at`/`user_delivered_at` 为空，预留字段。
4) **读路径兼容**
   - API 读：优先取新字段，缺失时用旧字段推导（如：如果 status 是 delivering 且 delivered_at 存在，可推导 rider_delivered_at）。
5) **切换状态枚举**
   - 灰度调整订单状态推进：
     - ready -> courier_accepted（可选） -> picked -> delivering -> rider_delivered -> user_delivered。
   - 与配送表状态对齐：picked/delivering/delivered 对应。
6) **回滚策略**
   - 若功能需回滚，可保持旧状态值兼容；新增列可保留不删。状态写入回滚时，停用新状态推进逻辑即可。

## API/逻辑兼容要点
- 确认收货接口：允许 `delivering` 或 `rider_delivered` 进入 `user_delivered`；旧前端仍可用 delivering 流程。
- 催单接口：增加对 `courier_accepted/picked/rider_delivered` 的判定。
- 列表/详情：返回新字段但保持旧字段不变，前端按需取用。
- 取消/退款：在服务层新增“preparing 前可退/取消”的闸门，preparing 及之后进入异常通道。
- 计费/结算：`user_delivered` 作为最终结算点；缺失时兼容使用 `completed`/`delivered_at`。

## 阶段性计划（高层）
1) DDL：加列、扩展状态约束；发布无业务变更。
2) 回填：批处理写入时间戳/默认值，幂等可重放。
3) 双写：服务层写新字段；状态推进保持旧值兼容。
4) 前端/接口切换：逐步消费 `status_hint/badges/pickup_code`，展示新状态文案。
5) 状态流切换：灰度启用 `courier_accepted/picked/rider_delivered/user_delivered`，并与配送表联动。
6) 收敛：完成后去除对旧推导路径的依赖（可选）。

## 待决策
- 是否保留 `courier_accepted` 独立状态（ vs. ready+派单元数据）。
- `pickup_code` 存储位置（仅订单 vs 订单+配送冗余）。
- `badges/status_hint` 存储类型（text[] vs jsonb，是否需要多语言）。
- 自动确认宽限时间（当前假设 +2h）。
- geofence+dwell 触发阈值与可信度写入何处（delivery 侧还是事件表）。

---

# 外卖状态/字段扩展实施清单（开发期可直接变更，完成请打勾）

## 阶段 1：Schema & 迁移
- [x] orders 表新增列：pickup_code, dispatch_order_id, flow_id, status_hint, badges(jsonb/text[]), exception_state, claim_channel, overtime(bool default false), prep_start_at, ready_at, courier_accept_at, picked_at, rider_delivered_at, user_delivered_at, auto_user_delivered_at。
- [x] 扩展订单状态约束/enum：courier_accepted, picked, rider_delivered, user_delivered。
- [x] deliveries 如需对齐：新增 rider_delivered_at（若仅有 delivered_at）、可选 pickup_code 冗余列。
- [x] 编写/执行迁移脚本；本地验证；重新生成 sqlc 和 mocks。

## 阶段 2：回填脚本（幂等，本地批处理）
- [x] delivering 且 delivered_at 有值 → 回填 rider_delivered_at=delivered_at（通过 join deliveries）。
- [x] completed 且无 user_delivered_at → 回填 user_delivered_at=completed_at。
- [x] 初始化 status_hint 为空串，badges 为空数组，overtime=false。（新增列默认已覆盖）
- [x] deliveries 回填 rider_delivered_at（如 delivered_at 存在）。
- [x] 批处理加分页/并发控制，验证幂等与耗时。（本地一次性批处理完成）

## 阶段 3：服务层状态机与事务
- [x] 引入新状态流：ready→courier_accepted→picked→delivering→rider_delivered→user_delivered，状态推进与日志同事务，必要时联动 deliveries。（已在骑手接单/取餐/配送/送达/用户确认收货路径双写订单状态与时间）
- [x] 写入时间戳：prep_start_at、ready_at、courier_accept_at、picked_at、rider_delivered_at、user_delivered_at。（配送流程双写对应时间，prep/ready 待厨房侧接入）
 - [x] 生成/存储 pickup_code（外卖/自取下单时生成）；dispatch_order_id/flow_id 待对接。
 - [x] 取消/退款闸门：preparing 前允许；之后进入异常通道（exception_state/claim_channel）。
- [x] 状态日志接受新枚举，记录新状态。（骑手接单/取餐/配送/送达路径已落日志）

## 阶段 4：API 契约与读路径兼容
- [x] 列表/详情响应补充：status_hint、badges、pickup_code_masked、actions、overtime；派单/流信息待后续对接。
- [x] 确认收货接口：允许 delivering/rider_delivered → user_delivered，写 user_delivered_at。
- [x] 催单接口：覆盖 courier_accepted/picked/rider_delivered；按状态决定通知对象。
- [x] 结算/统计：以 user_delivered 为最终交付点，缺失时回落 completed/delivered_at。

## 阶段 5：配送表联动
- [x] 将 deliveries 的 picking/picked/delivering/delivered 与订单新状态映射；在 Tx 中同步两表，幂等重试。
- [x] 围栏/定位事件触发 picked 或 rider_delivered 时，落地事务更新（默认关闭，通过开关启用）。

## 阶段 6：前端/客户端协作
- [x] 提供字段/状态文档给小程序/商户/骑手端；前端兼容读取 status_hint/badges/items/pickup_code。
- [x] 按 actions 权限控制按钮（退款/投诉/确认收货）。
   - 参考：[docs/takeout_frontend_changes.md](docs/takeout_frontend_changes.md)

## 阶段 7：测试与验证
- [x] 单测覆盖：围栏/驻留与自动推进分支（聚焦用例已通过）。
- [x] 更新 swagger（swag init）暴露新增字段/状态。
- [ ] 手工造流验证前后端与回填结果。

手工验证建议：
- 外卖完整流：courier_accepted -> picked -> delivering -> rider_delivered -> user_delivered。
- 取消闸门与异常通道（preparing 前/后）。
- actions 权限按钮与 status_hint/badges 展示一致。

## 阶段 8：收敛与清理（可选）
- [x] 新读写稳定后，移除旧状态推导分支（订单统计查询中剔除 legacy delivered）。

## 决策结果 → 同步到执行
- [x] courier_accepted 保留独立状态，用于区分“骑手已接单未到店”场景，状态机保留该节点，催单/派单可更精准。
- [x] badges/status_hint 存储使用 jsonb，便于结构化/多语言（如 {text, type, locale?}），API DTO 采用结构体映射。
- [x] pickup_code 存储：以 orders 为唯一来源，不在 deliveries 冗余；骑手端需通过订单读取或由服务层联查暴露，避免双写同步问题。
- [x] 自动确认与 geofence：用户侧自动确认默认 rider_delivered 后 +2h（可配置）；geofence+dwell 仅作佐证，不自动驱动送达，跑腿需显式确认送达；围栏到店后方可让跑腿执行送达确认。
