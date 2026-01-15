# 外卖订单生命周期（与堂食/预约共存）

## 目的
- 重新锚定已实现的外卖业务流，确保与堂食、预约共存且不冲突。
- 向后端提供状态机、字段契约和业务不变式，便于前后端、商户端、骑手端、运营侧对齐。

## 实体与标识
- `payment_order_id`（微信合单支付）：多商户购物车的父单。
- `takeout_order_id`（每个商户一张子外卖单）：本文核心对象。
- `dispatch_order_id`（配送单/代取单，投放到市场或推荐流）。
- `flow_id`（给在线骑手的推荐流标识）。
- `pay_transaction_id`（微信子商户交易号）。
- `pickup_code`（取餐码，骑手出示，商户核销）。

## 状态机（takeout_order.status）
状态单值，迁移记录时间戳，迁移需幂等。

1) `paid`（入口）
   - 合单支付成功、子单生成即进入。
   - 无商户确认，自动推进到 `preparing`。

2) `preparing`
   - 厨房自动开做；进入本状态前允许退款/取消。
   - 商户完成出餐后转 `ready`。

3) `ready`
   - 出餐完成，待取餐。
   - 可先到 `courier_accepted`（若未接单）或直接到 `picked`（若骑手已接单并到店）。

4) `courier_accepted`
   - 配送/代取单被骑手领取。
   - 取餐确认后转 `picked`。

5) `picked`
   - 取餐码核销确认；系统也可用围栏+停留佐证。
   - 骑手离店后转 `delivering`（通常同一时刻）。

6) `delivering`
   - 配送中。
   - 骑手确认或围栏+停留到达收货点时转 `rider_delivered`。

7) `rider_delivered`
   - 骑手侧已送达。
   - 用户确认或超时自动（如+2h）转 `user_delivered`。

8) `user_delivered`
   - 用户侧送达，用于结算闭环。

9) `cancelled`
   - 仅允许在 `preparing` 前（已支付但未开做）。

10) 异常通道（正交）
    - 索赔/投诉：`exception_investigating`, `exception_resolved`。
    - 退款：`refund_requested`, `refunding`, `refund_failed`, `refunded`（只在 `preparing` 前有效；之后走异常通道）。

允许的主路径：
- `paid` -> `preparing` -> `ready` ->（可选 `courier_accepted`）-> `picked` -> `delivering` -> `rider_delivered` -> `user_delivered`
- 自动升级：`rider_delivered` 在宽限后自动到 `user_delivered`。

## 每单核心字段
- 标识：`takeout_order_id`, `merchant_id`, `store_id`, `payment_order_id`, `pay_transaction_id`（子商户）。
- 订单类型：`order_type=takeout`（堂食/自取/预约使用各自值）。
- 金额：`total`, `items_total`, `delivery_fee`, `discounts`, `pay_method`, `combine_pay=true`, `currency`。

- 商品：`[{name, count, unit_price, total_price, thumb_url}]`，足够渲染列表预览。
- 状态：`status`（见上），`status_hint`（提示栏文案），`badges[]`（如“立即送达”“支付方式”“优惠”），`allow_refund`（仅 `preparing` 前为 true）。
- 取餐/配送：`pickup_code`, `eta`, `promised_delivery_time`, `overtime`（bool）, `courier_id`（可空）, `dispatch_order_id`, `flow_id`, `courier_location`（可选快照）, `geofence_version`。

- 时间戳：`paid_at`, `prep_start_at`, `ready_at`, `courier_accept_at`, `picked_at`, `rider_delivered_at`, `user_delivered_at`, `auto_user_delivered_at`, `cancelled_at`。
- 异常/索赔：`exception_state`, `exception_reason`, `claim_channel`（餐厅/骑手）, `complaint_options`（可由服务端下发，客户端可缓存）。
- UX 控制：`actions`（服务端计算的按钮权限：退款/投诉/确认收货等），`pickup_code_masked`（非必要场景用掩码）。

## 业务规则与不变式
- 无商户确认：`paid` 立即进 `preparing`。
- 退款/取消：仅 `preparing` 前允许；进入 `preparing` 后走异常通道。
- 取餐确认：以取餐码核销为主，围栏+停留为辅，均可触发 `picked`。

- 送达确认：骑手操作或围栏+停留触发 `rider_delivered`；用户操作或超时触发 `user_delivered`。
- 幂等：重复或过期事件必须无副作用。
- 与堂食/预约隔离：用 `order_type` 区分，支付/投诉引擎可复用，状态语义不混用。

## 事件源与驱动
- 支付回调：设置 `paid` 并拆子单。
- 厨房事件：标记 `ready`。
- 调度服务：驱动 `courier_accepted`，下发 `dispatch_order_id`/`flow_id`。

- 商户取餐操作：取餐码提交驱动 `picked`。
- 位置服务：围栏+停留驱动门店侧 `picked` 或收货侧 `rider_delivered`（需置信度门槛）。
- 骑手端操作：显式取餐/送达确认（若有）补强状态。

- 定时任务：宽限后自动 `user_delivered`；超时标记 `overtime`。
- 用户操作：确认收货、提交投诉、提交索赔。

## 投诉与索赔
- 投诉选项应含食品安全症状（头晕、恶心、腹泻等）、异物、漏送、严重超时及“其他”。
- 索赔率路：餐厅责任（食品质量/安全）vs 骑手责任（漏送/超时），均进入异常通道并由 `exception_state` 跟踪。

- 取证：文本、图片；与 `takeout_order_id` 关联存储。

## 列表/摘要数据契约（客户端）
- `status`, `status_hint`, `badges[]`, `items[]`（可裁剪至 2-3 条）, `item_count`, `total`, `eta` 或 `promised_delivery_time`, `pickup_code_masked`, `actions`（退款/确认收货/投诉）, `overtime`。
- 前端映射：状态胶囊取自 `status`，提示栏用 `status_hint`，徽标 pills 用 `badges`，菜品预览用 `items`。

## 与堂食/预约的共存
- 订单表可共用，靠 `order_type` 区分；状态语义不得混淆。
- 可复用模块：支付（含合单）、投诉/索赔、异常处理、SLA 监控。
- 差异规则：外卖 `preparing` 后禁退；堂食可有不同窗口；预约可能有预检。服务需按 `order_type` 分支。

## 待确认点（下一版）
- `rider_delivered` 到 `user_delivered` 的自动宽限时长（当前假设：+2h）。
- `courier_accepted` 是否保留独立状态，或折叠为 `ready`+派单元数据。
- 围栏+停留触发 `picked` / `rider_delivered` 的置信度阈值。

- `complaint_options` 的下发/本地化方式（服务端 vs 客户端配置）。
- 各状态的动作权限矩阵（按钮展示规则）。
