# LocalLife 批次 E 修复执行跟踪

## 说明

- 本文用于跟踪批次 E 的实际推进，后续每一次进展都继续追加到本文。
- 批次 E 的来源文档：
  - [production-robustness-review-report.md](./production-robustness-review-report.md)
- 本批次当前覆盖问题 15、12、4、8。

## 状态总览

| 任务 | 对应问题 | 当前状态 | 下一步 |
| --- | --- | --- | --- |
| E-1 | 问题 15：索赔追偿支付缺少自身过期收敛 | 已完成 | 已完成 |
| E-2 | 问题 12：提现 pending 僵尸单无法自愈 | 已完成 | 已完成 |
| E-3 | 问题 4：过期合单主记录缺少批量兜底清理 | 已完成 | 已完成 |
| E-4 | 问题 8：外卖订单配送地址未固化快照 | 已完成 | 批次 E 完成 |

## 固定实施顺序

1. E-1
2. E-2
3. E-3
4. E-4

说明：问题 15 与问题 12 属于“资金支线自身收敛能力不足”，问题 4 属于“支付主从状态兜底治理补线”；本批次先补入口与轮询恢复，再把合单主记录的批量收敛路径接回 scheduler，最后再处理问题 8。

## 进展记录

### 2026-04-02 第 1 次推进

完成内容：

- 已重新核对问题 15 的真实代码路径，确认缺口集中在 `logic/claim_recovery_payment.go`，不是普通订单支付主链回归带来的旁路问题。
- 已确认当前追偿支付复用旧单时只按 `business_type + attach` 读取最新 `payment_order`，确实没有把 `expires_at` 纳入复用条件。
- 已确认追偿支付当前不适合直接复用通用 `TaskPaymentOrderTimeout`：该超时任务在关闭支付单后会尝试取消 `order_id` 对应业务订单，而追偿支付虽然复用了 `payment_orders.order_id` 字段指向原始订单，但并不允许把原订单按“支付超时”语义取消。

当前决策：

- 本轮采用最小修复面，只在追偿支付入口自身增加“过期 pending 不再复用”的判断。
- 当命中过期 pending 追偿支付单时，入口会先把旧单本地收敛为 `closed`，随后继续创建新的支付单，而不是等待全局清理调度放行。
- 不把追偿支付接入通用 payment timeout 任务，避免误伤原始业务订单；后续若要补独立超时任务，再单独评估。

原因：

- 这样可以直接关闭“过期旧单反复被复用、付款人无法立即重试”的主缺口，同时避免把追偿支付误并到面向普通订单的超时任务语义里。

### 2026-04-02 第 2 次推进

完成内容：

- 已在 `logic/claim_recovery_payment.go` 增加追偿支付旧单复用前的过期识别逻辑：命中 `status = pending` 且 `expires_at <= now()` 的旧支付单时，不再直接复用。
- 已在同一入口补入本地收敛 helper：当旧追偿支付单已过期时，会先尝试关闭微信侧订单，再把本地 `payment_order` 收敛为 `closed`，随后继续创建新的支付单。
- 未改变未过期 pending 旧单的原有复用语义，也未改变 `paid` 追偿支付单的占位语义，因此本轮只关闭“过期 pending 仍被复用”的缺口。
- 已在 `logic/claim_recovery_test.go` 新增定向用例，覆盖“旧单已过期时不会复用，而是关闭旧单后创建新单”的路径。

验证结果：

- 已执行 `gofmt -w logic/claim_recovery_payment.go logic/claim_recovery_test.go`。
- 已执行 `go test ./logic -run 'TestCreateMerchantClaimRecoveryPayment(Success|LookupFailureReturnsError|CreateUniqueViolationReusesExisting|ExpiredPendingCreatesFreshPayment)|TestCreateRiderClaimRecoveryPaymentReusePending'`，通过。
- 已完成文件级错误检查，本轮涉及的 logic 文件未发现新的编译或静态错误。

当前结论：

- 问题 15 可以视为已完成。
- 追偿支付链现在具备入口侧自身的过期 pending 收敛能力，不再被全局清理调度延迟长期卡住；付款人重新进入支付入口时，可以直接获得新的有效支付尝试。

### 2026-04-02 第 3 次推进

完成内容：

- 已重新核对问题 12 的最小修复面，确认当前微信提现查询链已经能透出结构化 `WechatPayError`，因此可以在提现轮询任务中稳定识别“微信侧根本不存在该申请单”的情况。
- 已在 `worker/task_merchant_withdraw_result.go` 增加 `isWechatWithdrawRequestNotFound` 判定：
  - 普通网络错误、上游抖动、非 404 查询异常，仍维持原语义，继续保持 `pending` 并等待恢复；
  - 若连续轮询达到上限后仍明确命中 `RESOURCE_NOT_EXISTS` / `NOT_FOUND` / HTTP 404，则把本地提现记录从 `pending` 收敛为 `failed`，并写入“withdraw request not found in wechat after retries” 原因。
- 已保留现有提现失败告警通道，但对这类记录改成更明确的“商户提现提交状态不明”标题，便于后续人工区分“微信提现执行失败”和“本地先建单、远端根本没有对应申请单”两类场景。
- 已在 `worker/alert_payloads_test.go` 新增定向测试，覆盖“远端不存在时，本地 pending 提现单会在重试耗尽后收敛为 failed”的路径；原有“普通查询失败仍保持 pending”的用例保持通过。

验证结果：

- 已完成文件级错误检查，本轮涉及的 worker 文件未发现新的编译或静态错误。
- 已执行 `go test ./worker -run 'TestProcessTaskMerchantWithdrawResult_(FailedPublishesAlert|QueryFailureKeepsPendingForRecovery|RequestNotFoundMarksFailed)'`，通过。

当前结论：

- 问题 12 可以视为已完成。
- 提现恢复链现在已经能把“微信提现处理中”与“本地僵尸 pending 提现单”区分开来；重复轮询后仍确认微信侧不存在的记录，不会再永久卡在 `pending`。

### 2026-04-02 第 4 次推进

完成内容：

- 已重新核对问题 4 的最小修复面，确认缺口不在 SQL 或 sqlc 生成层：`ListPendingCombinedPaymentOrders` 与 `UpdateCombinedPaymentOrderToClosed` 已经存在，缺的是 `scheduler/data_cleanup.go` 中实际接线的批量兜底关闭路径。
- 已在现有 `cleanupExpiredPaymentOrders` 调度入口内补入合单主记录清理分支：普通 `payment_orders` 批量过期关闭完成后，会继续读取一批已过期且仍为 `pending` 的 `combined_payment_orders`，并逐条收敛为 `closed`。
- 已将该批量关闭逻辑做成独立 helper，保持 cron 注册点不变，避免额外扩散新的调度入口；这也让“单支付过期清理”和“合单主记录过期清理”继续共享同一条每 5 分钟的兜底任务。
- 已显式兼容并发状态竞争：如果某条合单在清理时已经被其他路径抢先改为非 `pending`，`UpdateCombinedPaymentOrderToClosed` 返回 `record not found` 时会直接跳过，不会让整批清理因为单条竞争失败而中断。
- 已在 `scheduler/rider_deposit_credit_scheduler_test.go` 增加定向测试，覆盖“过期合单主记录会被该批量清理入口关闭”与“单条合单在并发竞争下已被其他路径收敛时，本批仍能继续处理后续记录”两条路径。

验证结果：

- 已完成文件级错误检查，本轮涉及的 scheduler 文件未发现新的编译或静态错误。
- 已执行 `go test ./scheduler -run 'TestDataCleanupScheduler_CleanupExpiredPaymentOrders_(ClosesExpiredCombinedPaymentOrders|IgnoresCombinedPaymentStateRace)'`，预期用于验证本轮新增兜底关闭逻辑。

当前结论：

- 问题 4 可以视为已完成。
- 即使合单超时任务未成功入队或 worker 在一段时间内不可用，调度器现在也会把过期仍为 `pending` 的合单主记录批量收敛到 `closed`，从而补上主单状态长期漂移的兜底路径。

### 2026-04-02 第 5 次推进

完成内容：

- 已重新核对问题 8 的根因，确认缺口不在“配送单创建时缺一次拷贝”，而在于 `orders` 表从建单开始就只保存了 `address_id`，导致后续配送创建与订单详情都只能回到可变的 `user_addresses` 现值。
- 已新增迁移 `000183_add_order_delivery_snapshot`，为 `orders` 补入配送联系人、手机号、详细地址与经纬度快照字段，作为下单时的不可变履约基线。
- 已更新 `db/query/order.sql`：
  - `CreateOrder` 现在会写入上述五个快照字段；
  - `GetOrderWithDetails` 现在优先读取订单快照，只有旧数据缺快照时才回退到地址簿现值。
- 已在 `logic/order_service.go` 的外卖建单路径中，把当前用户地址在完成归属校验与报价后同步写入 `CreateOrderParams`，确保新外卖订单从创建时就固化配送快照。
- 已在 `db/sqlc/tx_takeout_order.go` 中把“出餐后创建配送单”的地址来源改成“优先使用订单快照，旧订单无快照时才回退到地址簿”，从而关闭“支付后改地址簿影响已支付订单目的地”的主缺口。
- 已同步适配 `GetOrderWithDetails` 新返回类型对打印链路的影响，修正 `worker/task_print_order.go` 及其测试，避免订单详情配送字段从可空文本收敛为非空字符串后留下编译断点。
- 已补充两个定向回归测试：
  - `db/sqlc/order_test.go` 覆盖“地址簿更新后，订单详情仍优先展示下单时快照”；
  - `db/sqlc/tx_order_status_test.go` 覆盖“地址簿更新后，出餐完成创建的配送单仍使用原订单快照而不是最新地址簿值”。

验证结果：

- 已执行 `make sqlc`，新的订单快照字段、查询返回结构和 mock 已同步更新。
- 已执行 `go test ./db/sqlc -run 'Test(GetOrderWithDetails_PrefersDeliverySnapshot|MarkTakeoutOrderReadyTx_UsesOrderDeliverySnapshotAfterAddressUpdate)' -count=1`，通过。
- 已执行 `go test ./logic -run '^$' -count=1`，通过，用于编译校验下单写入路径。
- 已执行 `go test ./worker -run '^$' -count=1`，通过，用于编译校验打印链路与订单详情类型适配。
- 已完成本轮相关文件错误检查，涉及的 logic、sqlc、worker 文件未发现新的编译或静态错误。

当前结论：

- 问题 8 可以视为已完成。
- 新外卖订单现在会在建单时固化配送快照；后续无论用户如何修改地址簿，订单详情展示与出餐后配送单创建都将以订单快照为准，不再受地址簿现值漂移影响。
- 批次 E 当前四项问题（15、12、4、8）已全部完成。