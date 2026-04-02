# LocalLife 批次 A 修复执行跟踪

## 说明

- 本文用于跟踪批次 A 的实际推进，后续每一次进展都继续追加到本文，不再只停留在对话中。
- 批次 A 的来源文档：
  - [production-robustness-review-report.md](./production-robustness-review-report.md)
  - [production-robustness-p0-remediation-review.md](./production-robustness-p0-remediation-review.md)
- 本批次只覆盖问题 1、5、9。

## 批次目标

- 尽快止血新增越权下单风险。
- 尽快止血新增骑手押金错账风险。
- 尽快止血新增退款超额风险。

## 状态总览

| 任务 | 对应问题 | 当前状态 | 下一步 |
| --- | --- | --- | --- |
| A-1 | 问题 1：外卖建单地址归属校验缺失 | 已完成 | 观察是否还存在其他非建单路径的地址归属旁路 |
| A-2 | 问题 5：自动确认送达固定解冻金额 | 已完成 | 收尾时复查是否还有其他自动履约入口使用硬编码金额 |
| A-3 | 问题 9：退款累计额度未占用 pending/processing | 已完成 | 收尾时评估是否需要历史长挂退款单清理配套 |

## 固定实施顺序

1. A-1：先封住越权建单入口。
2. A-2：再修自动送达押金解冻口径。
3. A-3：最后修通用退款额度占用口径。

说明：A-3 涉及 SQL 与 sqlc 生成面，改动外溢性高于 A-1、A-2，因此保持在本批次最后处理。

## 进展记录

### 2026-04-02 第 1 次推进

完成内容：

- 已确认批次 A 的执行边界固定为问题 1、5、9，不混入其他 P0 项。
- 已从风险报告和 P0 修复评审稿中抽出这三项的目标、最小修复面、涉及模块和回归验证点。
- 已定位当前可直接承接的测试入口：
  - 问题 1：`api/order_test.go` 中已存在 `TakeoutAddressBelongsToOtherUser` 场景。
  - 问题 5：`api/rider_location_events_test.go` 中已存在 `TestMaybeAutoConfirmDelivery`。
  - 问题 9：`db/sqlc/refund_order_test.go` 中已存在 `TestGetTotalRefundedByPaymentOrder`，`api/payment_order_test.go` 已覆盖退款入口基本路径。

当前判断：

- A-1 和 A-2 适合直接进入代码修复。
- A-3 在进入代码修复前，需要先明确“哪些退款状态占用额度、哪些状态释放额度”，否则容易误伤合法分批退款。

### 2026-04-02 第 2 次推进

完成内容：

- 已在 `logic/order_service.go` 的外卖建单路径补上地址归属校验。
- 当前用户若传入不属于自己的 `address_id`，建单流程现在会直接返回 403，不再继续进入运费计算和建单事务。
- 已确认 `api/order_test.go` 中原有 `TakeoutAddressBelongsToOtherUser` 场景能够直接承接此次修复，无需额外补一条重复测试。

验证结果：

- 已运行 `TestCreateOrderAPI`，结果通过。
- 本轮未修改 SQL、生成代码和其他链路逻辑。

遗留观察：

- 报价相关链路中还存在其他按 `address_id` 读取地址的路径，但当前发现的 create-order 风险点已经封住。
- 是否要统一报价链路的报错语义，留待批次 A 收尾时再评估，不在本次最小修复面内扩大处理。

### 2026-04-02 第 3 次推进

完成内容：

- 已把自动送达解冻金额的计算从 `api/rider_location_events.go` 外部常量传参，收敛到 `logic.AutoConfirmDelivery` 内部统一按订单口径计算。
- 自动送达现在会在加载订单后直接使用 `OrderFreezeAmount(order)`，不再依赖固定值 `5000`。
- 已同步收窄 `AutoConfirmDelivery` 函数签名，避免后续调用方再次传入任意解冻金额。

验证结果：

- 已增强 `logic/delivery_geofence_test.go` 与 `api/rider_location_events_test.go`，显式断言非 `5000` 金额场景下传入 `CompleteDeliveryTx` 的 `UnfreezeAmount` 与订单金额一致。
- 已运行自动送达相关定向测试，结果通过。

遗留观察：

- 本轮修复只覆盖围栏自动确认送达。
- 其他自动履约入口是否存在类似硬编码金额问题，留待批次 A 收尾时统一复查，不在本轮最小修复面内扩大改动。

### 2026-04-02 第 4 次推进

完成内容：

- 已把 `GetTotalRefundedByPaymentOrder` 的统计口径从仅统计 `success`，调整为统计 `pending`、`processing`、`success` 三种会占用退款额度的状态。
- 已同步保留 `failed`、`closed` 不占用额度的语义，避免把终态失败单错误算入可退款占用。
- 已补事务级测试，直接验证 `CreateRefundOrderTx` 在存在 pending/processing 退款时会拒绝超额新退款。
- 已按仓库约束执行 `make sqlc`，同步更新生成代码与 mock。

验证结果：

- `db/sqlc/refund_order_test.go` 已覆盖 pending、processing、success 混合口径。
- `db/sqlc/tx_refund_test.go` 已新增事务级超额拦截验证。
- 退款相关定向测试通过。

遗留观察：

- 新规则会让历史长期挂起的 pending/processing 退款单真实占住退款额度。
- 是否需要上线前增加一次历史退款单排查与运营确认，作为批次 A 收尾项记录，不在本次代码最小修复面内扩大处理。

### 2026-04-02 第 5 次推进

完成内容：

- 已对批次 A 相关测试文件执行一次合并回归，覆盖问题 1、5、9 对应的 API、logic、db/sqlc 层测试。
- 批次 A 当前三项代码修复均已落地，且自动化验证已通过。

验证结果：

- 批次 A 定向测试合计通过，未发现当前实现层面的新增回归。

下一步建议：

- 批次 A 可以进入代码评审与历史坏数据排查准备阶段。
- 后续若继续推进，应新建批次 B 跟踪文档，并保持与本文相同的“每一步都落文档”节奏。

## 任务拆解

### A-1 外卖建单地址归属校验

目标：

- 保证用户只能用自己的地址完成外卖建单。
- 禁止其他用户地址通过 `address_id` 被挂到当前订单上。

候选改动文件：

- `logic/order_service.go`
- `api/order_test.go`
- 如有必要，补充共享校验辅助逻辑所在文件

主要实施点：

- 确认外卖建单链路读取地址后的统一校验位置。
- 在该位置加入 `address.UserID == currentUserID` 的强校验。
- 排查是否存在其他通过同一建单路径复用的 takeout 地址读取分支，避免只修主入口。

测试与验证：

- 保持“地址不存在”场景继续返回现有预期错误。
- 保持“地址属于当前用户”场景仍可正常建单。
- 保证“地址属于其他用户”场景稳定返回拒绝，沿用或增强 `api/order_test.go` 中的现有用例。

进入实施前的注意点：

- 先确认是否存在后台代客下单或运营代下单入口复用同一逻辑，避免误把特权路径也锁死。

### A-2 自动确认送达解冻金额统一

目标：

- 让自动送达与人工送达使用同一套押金解冻金额来源。
- 消除固定值 `5000` 导致的押金账实漂移。

候选改动文件：

- `api/rider_location_events.go`
- `logic/delivery_status.go`
- `api/rider_location_events_test.go`
- 如有必要，补充相关履约或事务测试

主要实施点：

- 确认自动送达路径能否直接复用现有 `OrderFreezeAmount(order)` 口径。
- 检查自动送达路径是否已具备所需订单上下文；若缺失，需要先明确最小补数方式。
- 排查是否还有其他自动履约路径使用硬编码冻结/解冻金额。

测试与验证：

- 补强 `TestMaybeAutoConfirmDelivery`，使其覆盖非 `5000` 分订单。
- 验证自动送达调用 `CompleteDeliveryTx` 时使用的金额与人工送达一致。
- 确认修复后不会影响正常的订单状态日志和送达完成路径。

进入实施前的注意点：

- 先确认历史上是否已经存在因固定金额造成的冻结余额漂移；该问题可能需要单独的数据清理动作，但不属于本次代码修复本体。

### A-3 退款累计额度占用口径统一

目标：

- 防止第一笔退款进入 `pending` / `processing` 后，后续退款仍继续被放行。
- 把退款额度校验收敛为真实已占用额度口径。

候选改动文件：

- `db/query/refund_order.sql`
- `db/sqlc/refund_order_test.go`
- `db/sqlc/tx_refund.go`
- `api/payment_order_test.go`
- 如有必要，补充 `logic/refund_service.go` 相关回归测试

主要实施点：

- 先定义退款额度占用状态集合，至少覆盖 `pending`、`processing` 与 `success` 的取舍关系。
- 确认状态释放规则，避免失败单或关闭单仍持续占用额度。
- 修改 SQL 后同步评估是否需要重新生成 sqlc 代码以及是否影响 mock 或调用方断言。

测试与验证：

- 扩展 `TestGetTotalRefundedByPaymentOrder`，使其覆盖 `pending`、`processing`、`success` 混合场景。
- 验证第一笔退款未完成时，第二笔退款会按剩余额度被正确拒绝或放行。
- 验证正常单笔退款与合法分批退款场景不被误伤。

进入实施前的注意点：

- 需要先确认线上是否已经存在长时间悬挂的 `pending` / `processing` 退款单，否则新规则可能导致历史支付单短期内无法继续退款。

## 本批次完成标准

- A-1、A-2、A-3 的代码修复均已落地。
- 每项至少有一条直接对应原风险的自动化回归验证。
- 变更后形成新的实施记录，继续追加到本文“进展记录”。
- 若发现历史坏数据需要单独处置，必须在本文追加记录，不得只在对话中说明。