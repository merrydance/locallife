# 发布前手工回归清单

日期：2026-03-27

用途：收口 CARD-09 与 CARD-12 里仍需人工确认的端到端链路，避免上线窗口临时补步骤。

## 执行记录

- 回归人：
- 回归时间：
- 回归环境：
- 构建版本 / 提交：
- 结论：通过 / 有条件通过 / 不通过

## 执行前准备

- 准备 1 个商户账号、1 个商户员工账号、2 个普通用户账号。
- 准备至少 2 张可扫码桌台，其中 1 张带有效桌码。
- 准备 1 条定金模式预约，预约时间落在当前后 10 到 30 分钟内，且已存在真实支付单。
- 准备 1 条非预约堂食开台场景，便于验证拼桌、部分支付、关台链路。
- 打开商户后台日志/告警面板，便于观察支付回调重试与告警。

## P0 主场景

### 1. 押金抵扣取已实收金额

目标：确认 reservation 订单抵扣只使用真实已支付的 reservation payment order 金额。

步骤：

1. 以顾客身份创建定金模式预约，并确认存在已支付 reservation payment order。
2. 进入到店点菜或预约加菜链路，创建基于该预约的订单。
3. 记录订单应付金额、押金抵扣金额、reservation payment order 实付金额。

预期：

- 抵扣金额等于真实已支付 reservation payment order 金额，而不是盲取 `deposit_amount` 静态字段。
- 若 reservation payment order 实付金额小于配置定金，则只发生部分抵扣。
- 未支付定金或 pending reservation 不能进入可抵扣建单链路。

证据：

- 订单详情截图或接口响应。
- 对应 reservation payment order 金额截图或数据库查询结果。

### 2. 预约确认不会提前占桌

目标：确认未来时段预约在 confirm 后不会立即把桌台写成 `reserved`。

步骤：

1. 商户确认一条未来时段预约。
2. 立刻查看桌台列表或扫码预检结果。
3. 若当前桌台已有其他堂食会话，再次确认桌台状态未被覆盖。

预期：

- 预约状态变为 `confirmed`。
- 桌台当前状态保持真实营业状态，不因为 confirm 提前变成 `reserved`。
- 后续开台、转台仍依赖时段冲突判断，而不是旧的桌台标记。

证据：

- 预约详情截图。
- 桌台列表或相关接口返回截图。

### 3. 开台 / 转台按预约时段与身份工作

目标：确认开台、预检、转台的预约冲突与身份语义一致。

步骤：

1. 预约用户扫码调用预检或进入扫码页面，检查 `is_reservation_owner` 与预约信息。
2. 商户 owner 或 staff 查看同一桌台预检结果。
3. 预约用户在签到窗口内开台，检查返回的 `billing_group.total_amount` 与 `paid_amount`。
4. 尝试转台到一张存在冲突预约的桌台。

预期：

- 预约本人看到 `is_reservation_owner=true`；商户查看同一预约时为 `false`。
- 开台响应中的账单组金额使用运行时聚合口径。
- 转台对目标桌台按真实预约时段冲突拦截，而不是依赖过期的 `reserved/current_reservation_id`。

关键接口：

- `GET /v1/dining-sessions/precheck?table_id={id}`
- `POST /v1/dining-sessions/open`
- `POST /v1/dining-sessions/{id}/transfer-table`

### 4. 支付回调重试与告警

目标：确认重复回调、认领失败、release fail 的处理与告警符合整改预期。

步骤：

1. 对同一支付单重复触发回调或回放回调报文。
2. 观察系统对重复认领、查单失败、释放失败的返回与告警。
3. 核对是否仍出现错误地回 `SUCCESS` 直接吞掉异常的情况。

预期：

- 未知重复认领状态不会错误返回 `SUCCESS`。
- 查单失败会返回失败响应并产生明确告警。
- release fail 告警包含 callback type 与失败原因，便于排查。

证据：

- 回调响应报文。
- 监控或告警截图。
- 结构化日志片段。

## P1 主场景

### 5. 账单组金额展示一致性

目标：确认拼桌、部分支付、替换或取消、关台前后的账单组金额在不同入口一致。

步骤：

1. 开台后创建默认账单组，并拉起至少 1 个额外账单组。
2. 第二位用户加入目标账单组。
3. 在同一账单组下创建多笔订单，其中至少 1 笔完成支付、1 笔保持未支付或后续取消或替换。
4. 分别查看：
   - 开台响应里的 `billing_group`
   - 账单组列表 `GET /v1/billing-groups?dining_session_id={id}`
   - 加入账单组响应 `POST /v1/billing-groups/{id}/join`
5. 关台前再次核对金额。

预期：

- 同一账单组在上述入口里 `total_amount` 一致。
- `paid_amount` 只累计已支付或进入 paid-or-later 状态的订单。
- 取消或被替换订单不再被计入聚合金额。
- 不出现“列表是 0、开台有值、加入后又变旧值”的撕裂。

建议样例：

- 订单 A：1200，已支付。
- 订单 B：480，未支付或被取消。
- 预期账单组：`total_amount=1200` 或按替换后新订单重算，`paid_amount=1200`。

### 6. 预检身份展示一致性

目标：确认小程序或 Web 页面不会把“商户可查看预约”误显示成“本人预约”。

步骤：

1. 以预约用户打开扫码预检页。
2. 以商户 owner 或 staff 查看同一桌台预检页。
3. 对照接口返回与页面文案。

预期：

- 仅预约本人显示“本人预约”或等价文案。
- 商户查看时可见预约信息，但不会显示为用户本人身份。
- 页面展示与 `is_reservation_owner` 接口语义一致。

## 回归完成判定

- P0 四项全部通过，且无阻断告警。
- P1 两项全部通过，或有明确已知问题与上线豁免说明。
- 回归记录已回填到 CARD-09 与 CARD-12。

## 回填位置

- [business_flow_task_cards_20260327/card-09-billing-group-regression.md](business_flow_task_cards_20260327/card-09-billing-group-regression.md)
- [business_flow_task_cards_20260327/card-12-release-readiness.md](business_flow_task_cards_20260327/card-12-release-readiness.md)