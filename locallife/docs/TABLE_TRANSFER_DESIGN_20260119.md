# 商户侧转台（换桌）功能设计方案

日期：2026-01-19

## 1. 结论（需求真伪判断）
**结论：真实需求，建议纳入商户端基础能力。**

理由：
- 堂食场景常见换桌：人数变化、菜品量变化、服务动线调整、拼桌/拆分、桌台故障等。
- 当前系统强绑定桌台与用餐会话/订单，换桌若无正式能力，会导致：
  - 桌台占用无法释放或误释放
  - 订单显示/出单台号错误
  - 预订桌台错位，导致后续预订冲突
  - 商户被迫“关台再开台”，带来对账、计费、库存/优惠异常风险

因此：这是**真实且高频**的商户场景，而非伪需求。

## 2. 现状梳理（系统已有能力）
- 桌台管理：创建/状态/二维码/标签/图片（桌台可被标记占用/空闲/停用）。
- 用餐会话：`dining_sessions` 记录桌台与用户/预订绑定，且对桌台有唯一开放会话约束。
- 预订：`table_reservations` 绑定 `table_id`，并通过 `tables.current_reservation_id` 与桌台关联。
- 订单：堂食订单包含 `table_id`，但未绑定 `dining_session_id`。

**缺口：** 未发现“转台/换桌/变更桌台”接口或事务逻辑。

## 3. 功能定义
**转台（换桌）**：将“正在进行的用餐会话”从 A 桌移至 B 桌，保证桌台状态、会话、订单、预订一致性。

**范围明确：**
- 本次只覆盖“单桌 → 单桌”的转移。
- **不包含**：并桌/拼桌/拆台/分账等（可作为后续迭代）。

## 4. 影响面分析（核心业务流程）
1. **桌台状态**
   - A 桌释放为可用（或清洁中，若引入清洁状态）
   - B 桌变更为占用

2. **用餐会话（dining_sessions）**
   - 会话 `table_id` 需要更新为 B 桌
   - 需保持 `open` 状态与唯一约束

3. **订单与出单**
   - 订单 `table_id` 需同步到 B 桌，否则厨显/出单台号错误

4. **预订**
   - 若为预订会话，需更新 `table_reservations.table_id`
   - 同时更新 `tables.current_reservation_id`（旧桌清空，新桌挂载）

5. **前端与扫码**
   - 顾客扫码新桌二维码需要“继续加入旧会话”还是新开会话？
   - 商户端需能从桌台列表中快速发起转台

6. **权限与审计**
   - 仅商户员工（老板/店长/服务员）可操作
   - 转台应可追溯（日志）

## 5. 设计方案（后端为核心）

### 5.1 新增接口（商户端）
**建议新增：**

`POST /v1/dining-sessions/{id}/transfer-table`

请求：
```json
{
  "to_table_id": 123,
  "reason": "客人要求换至包间"
}
```

响应：
```json
{
  "session": { ... },
  "from_table": { ... },
  "to_table": { ... }
}
```

**接口规则：**
- 仅 `open` 会话可转台
- `to_table_id` 必须属于同一商户
- 目标桌台必须：
  - 非 `disabled`
  - 无其他开放会话
  - 无与当前会话无关的“有效预订占用”
- 如果会话绑定预订：
  - 更新预订桌台
  - 目标桌台在当前时段不能被其他预订占用

### 5.2 事务逻辑（伪代码）
```
Begin Tx
  1) 锁定会话（for update）
  2) 校验会话状态、商户权限
  3) 锁定源桌台与目标桌台
  4) 校验目标桌台可用性（状态/预订/占用）
  5) 更新 dining_sessions.table_id = to_table_id
  6) 更新 orders.table_id（见 5.4）
  7) 若有 reservation_id：
        - 更新 table_reservations.table_id
        - 更新 tables.current_reservation_id
  8) 更新 tables.status：from -> available，to -> occupied
  9) 记录转台日志
Commit
```

### 5.3 数据表扩展（建议）
**新增转台日志表：**
```
CREATE TABLE table_transfer_logs (
  id BIGSERIAL PRIMARY KEY,
  merchant_id BIGINT NOT NULL,
  dining_session_id BIGINT NOT NULL,
  reservation_id BIGINT,
  from_table_id BIGINT NOT NULL,
  to_table_id BIGINT NOT NULL,
  operator_user_id BIGINT NOT NULL,
  reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```
用于：审计、纠纷处理、数据分析。

### 5.4 订单同步策略（生产级）
当前订单未绑定 `dining_session_id`，但**堂食/预订订单已通过账单组与会话关联**：
`billing_group_orders -> billing_groups(dining_session_id)`。

**建议采用现有关系进行订单同步（无需新增字段）**：
- 通过 `dining_session_id` 找到其账单组
- 通过 `billing_group_orders` 获取会话内订单集合
- 批量更新这些订单的 `table_id = to_table_id`

**不建议此处新增 `orders.dining_session_id`**，理由：
- 现有账单组已是生产级关联链路，新增字段会引入数据回填和一致性维护成本
- 转台仅需在会话维度同步台号，账单组路径可稳定覆盖

**仅在以下场景再评估新增字段：**
- 需要按会话直接高频检索订单且无法接受 join 成本
- 存在大量历史堂食订单没有账单组关联

### 5.5 桌台状态与预订同步
- `from_table.status` 变为 `available`
- `to_table.status` 变为 `occupied`
- `tables.current_reservation_id`
  - 仅在会话绑定预订时同步
  - 从旧桌移除，挂到新桌

> 现有桌台状态已包含 `reserved`（不新增 `cleaning/maintenance`）。

### 5.6 WebSocket 通知
沿用现有桌台状态广播机制，新增事件：
- `table_transfer`
  - payload: { session_id, from_table_id, to_table_id, operator_id }

用于前台桌台看板实时刷新。

## 6. 场景覆盖与处理策略

### 6.1 常见场景
1. **客人换到更大桌/包间**：允许，目标桌必须无冲突
2. **原桌故障/维修**：允许，同时可将原桌状态改为 `maintenance`（若未来引入）
3. **预订客户换桌**：允许，但必须通过冲突校验
4. **换桌后继续扫码点餐**：
   - 顾客扫码新桌应关联到已存在会话
   - 可以通过“桌台验证码 + 会话归属”判断

### 6.2 禁止场景
- 目标桌已有开放会话
- 目标桌被其他预订占用（当前时段内）
- 会话已关闭

### 6.3 边界场景
- **会话中已支付**：允许（不影响订单金额），但需更新厨显/台号
- **存在多个订单**：推荐绑定 `dining_session_id` 后批量更新
- **转台后退款/取消**：保持原业务逻辑即可

## 7. 前端改造建议（小程序商户端）
- 桌台列表新增“转台”操作入口
- 进入转台页面：
  - 选择目标桌台（过滤：同商户、可用、非预订冲突）
  - 填写原因（可选）
- 成功后：
  - 更新桌台列表状态
  - 展示转台记录

## 8. C端扫码入口（可选，但推荐）
当用户已开台或有有效预订时，扫码新的桌台码应弹出 Modal：
> “检测到您当前已有用餐/预订，是否切换到该桌台/包间？”

**推荐复用同一接口**：`POST /v1/dining-sessions/{id}/transfer-table`

为支持 C 端触发，需要补充以下约束：
- 允许 **会话所属用户** 调用（session.user_id == auth.user_id）
- 非预订场景必须提供新桌台的 `table_code`（与开台一致）
- 预订场景需校验预订归属（reservation.user_id == auth.user_id）

**扩展请求体：**
```json
{
  "to_table_id": 123,
  "table_code": "1234",
  "reason": "扫码换桌"
}
```

若不希望开放 C 端权限，则应新增“请求换桌”接口，由商户确认后执行转台。

## 9. 风险与补救
- **订单台号错乱**：必须同步订单或引入 `dining_session_id`
- **并发转台**：使用事务 + `SELECT FOR UPDATE` 保证一致性
- **回滚策略**：
  - 若更新订单失败则整体回滚
  - 转台日志仅在 commit 后写入

## 10. 测试要点
- 转台成功路径（无预订/有预订）
- 目标桌已占用、已预订、已停用
- 会话不存在或已关闭
- 并发转台（同一会话、同一目标桌）
- 订单台号同步正确

## 11. 里程碑建议
1. **后端接口 + 事务 + 日志表**（优先）
2. **订单与会话绑定字段**
3. **商户端 UI**
4. **数据回填与线上灰度**

---

## 附：当前系统关联点（便于落地）
- 用餐会话：`api/dining_session.go`
- 桌台管理：`api/table.go`
- 预订修改：`api/table_reservation.go`（已有修改桌台能力）
- 订单：`api/order.go`（堂食订单依赖 `table_id`）
