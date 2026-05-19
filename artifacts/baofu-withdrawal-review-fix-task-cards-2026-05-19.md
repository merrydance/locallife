# 宝付提现 Review Findings 修复任务卡

日期：2026-05-19
范围：`locallife/` 宝付宝财通提现创建、审计命令、API 错误映射；`weapp/` 多角色提现列表页局部失败反馈。
风险等级：G3。原因：涉及真实资金提现、宝付受理/查询状态、重复提交、外部支付命令审计、异步恢复和前端资金状态承接。
执行方式：每个 finding 对应一张独立任务卡。执行任一卡前必须先完整阅读本文件对应任务卡和“全局不变式”。

---

## 全局不变式

这些规则适用于三张任务卡，执行任一卡都不得破坏：

- 宝付 `accWithdrawal` 创建接口只表示“受理结果”，不是提现终态。官方文档 `.github/standards/domains/baofu-payment/official-sources/doc-mandao/accWithdrawal.md` 明确写明最终提现结果必须通过异步通知或查询接口 `state` 判断。
- `accWithdrawal.response.state=1` 只表示受理成功，本地订单继续是 `processing`，等待回调或恢复查询推进到 `succeeded`、`failed`、`returned`。
- `accWithdrawal.response.state=2` 表示受理失败，不能保存为本地 `processing`，不能让恢复 worker 后续继续查询一个未被受理的提现申请。
- 本地已经创建 `baofu_withdrawal_orders` 后，如果 provider/network 返回未知结果，前端必须看到“结果确认中，请勿重复提交”语义，不能看到“稍后重试”从而诱导重复提现。
- 后端所有 provider、数据库、审计命令、状态更新失败都必须进入一个明确的结构化日志边界。日志必须包含可定位字段，不得包含完整 `contractNo`、银行卡号、证件号、手机号、密钥、证书、完整 provider 原始报文或未脱敏生产回调。
- 前端可展示的错误文案必须是中文产品语义，不能泄露 SQL、Go driver、provider 原始错误、英文诊断串或堆栈。
- 小程序不能从本地累计收入、历史记录或旧接口推断可提现余额；余额以 `getBaofuWithdrawalBalance(role)` 为准。
- 回调和恢复查询仍是提现终态的唯一来源。本批修复不新增“同步创建即成功”的路径。

推荐执行顺序：

1. 先执行 `TASK-BAOFU-WD-FIX-001`，修正明确受理失败。
2. 再执行 `TASK-BAOFU-WD-FIX-002`，修正 provider 结果未知的语义和审计命令状态。
3. 最后执行 `TASK-BAOFU-WD-FIX-003`，让小程序承接后端语义并拆分余额与记录局部失败。

---

## TASK-BAOFU-WD-FIX-001：同步受理失败不得落为 processing

### 目标

当宝付提现创建接口同步返回 `state=2` 时，本地 `baofu_withdrawal_orders` 必须更新为 `failed`，审计命令必须记录为 `rejected`，API 必须返回明确的业务反馈：“提现申请未被受理，请刷新余额后重试”。

### 当前证据

- 官方契约：`.github/standards/domains/baofu-payment/official-sources/doc-mandao/accWithdrawal.md` 说明创建接口只返回受理结果，`state=1` 为受理成功，`state=2` 为受理失败。
- Provider 解析：`locallife/baofu/account/contracts/types.go` 中 `WithdrawAcceptanceStatusFromUpstream("2")` 已能转换为 `failed`。
- Provider 校验：`locallife/baofu/account/client.go` 中 `validateWithdrawAcceptanceResponse()` 已只接受 `"1"` 和 `"2"`。
- 业务缺陷：`locallife/logic/baofu_withdraw_service.go` 当前在 provider 返回后忽略 `upstream.Status`，始终调用 `UpdateBaofuWithdrawalOrderToProcessing`。
- API 缺陷：`locallife/api/baofu_withdrawal.go` 当前创建成功一律返回 `201 Created`。

### 修改范围

后端文件：

- 修改：`locallife/logic/baofu_withdraw_service.go`
- 修改：`locallife/api/baofu_withdrawal.go`
- 修改：`locallife/logic/baofu_withdraw_service_test.go`
- 修改：`locallife/api/baofu_withdrawal_contract_test.go`
- 如 mock 接口因 store interface 变化不匹配：重新生成 `locallife/db/mock/**`

可选文件：

- 如果实现需要复用审计服务常量，修改 `locallife/db/sqlc/constants.go` 和 `locallife/logic/payment_command_service.go`，见本卡“审计命令要求”。

### 不在本卡处理

- 不处理 provider/network 错误后的未知结果语义，该问题属于 `TASK-BAOFU-WD-FIX-002`。
- 不修改提现回调、恢复 worker、fact application 的终态推进规则。
- 不改小程序页面结构。
- 不把 `state=1` 解释成提现成功。

### 实现步骤

- [ ] 在 `locallife/logic/baofu_withdraw_service.go` 增加 typed error：

```go
var (
	ErrBaofuWithdrawServiceNotConfigured = errors.New("baofu withdraw service is not configured")
	ErrBaofuWithdrawAccountNotReady      = errors.New("宝付结算账户未开通，暂不能提现")
	ErrBaofuWithdrawContractNoRequired   = errors.New("宝付结算账户缺少提现账户标识，暂不能提现")
	ErrBaofuWithdrawBalanceUnavailable   = errors.New("baofu withdraw balance unavailable")
	ErrBaofuWithdrawInsufficientBalance  = errors.New("可提现金额不足")
	ErrBaofuWithdrawCreateRejected       = errors.New("baofu withdraw create rejected")
)
```

- [ ] 扩展 `baofuWithdrawStore`，确保服务层能把本地订单从 `processing` 更新为 `failed`：

```go
type baofuWithdrawStore interface {
	GetBaofuAccountBindingByOwner(ctx context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error)
	CreateBaofuWithdrawalOrder(ctx context.Context, arg db.CreateBaofuWithdrawalOrderParams) (db.BaofuWithdrawalOrder, error)
	UpdateBaofuWithdrawalOrderToProcessing(ctx context.Context, arg db.UpdateBaofuWithdrawalOrderToProcessingParams) (db.BaofuWithdrawalOrder, error)
	UpdateBaofuWithdrawalOrderStatus(ctx context.Context, arg db.UpdateBaofuWithdrawalOrderStatusParams) (db.BaofuWithdrawalOrder, error)
	CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error)
}
```

- [ ] 在 `CreateWithdrawal` 处理 provider 返回后，先标准化 raw snapshot，再分支处理 `upstream.Status`：

```go
raw := []byte(upstream.Raw)
if len(raw) == 0 || !json.Valid(raw) {
	raw = []byte(`{}`)
}

switch upstream.Status {
case db.BaofuWithdrawalStatusFailed:
	updated, updateErr := s.store.UpdateBaofuWithdrawalOrderStatus(ctx, db.UpdateBaofuWithdrawalOrderStatusParams{
		ID:     withdrawalOrder.ID,
		Status: db.BaofuWithdrawalStatusFailed,
		BaofuWithdrawNo: pgtype.Text{
			String: strings.TrimSpace(upstream.BaofuWithdrawNo),
			Valid:  strings.TrimSpace(upstream.BaofuWithdrawNo) != "",
		},
		RawSnapshot: raw,
	})
	if updateErr != nil {
		return result, fmt.Errorf("mark baofu withdrawal create rejected: %w", updateErr)
	}
	result.WithdrawalOrder = updated
	return result, ErrBaofuWithdrawCreateRejected
case db.BaofuWithdrawalStatusProcessing:
	updated, updateErr := s.store.UpdateBaofuWithdrawalOrderToProcessing(ctx, db.UpdateBaofuWithdrawalOrderToProcessingParams{
		ID: withdrawalOrder.ID,
		BaofuWithdrawNo: pgtype.Text{
			String: strings.TrimSpace(upstream.BaofuWithdrawNo),
			Valid:  strings.TrimSpace(upstream.BaofuWithdrawNo) != "",
		},
		RawSnapshot: raw,
	})
	if updateErr != nil {
		return result, updateErr
	}
	result.WithdrawalOrder = updated
	return result, nil
default:
	return result, fmt.Errorf("unsupported baofu withdraw acceptance status %q", upstream.Status)
}
```

- [ ] 在创建审计命令时，确保受理失败记录为 `rejected`。如果继续直接调用 `CreateExternalPaymentCommand`，则在 provider 返回后再创建命令，不要在 provider 调用前固定写 `submitted`。受理失败参数必须包含：

```go
CommandStatus:    db.ExternalPaymentCommandStatusRejected,
SubmittedAt:      s.now().UTC(),
RejectedAt:       pgtype.Timestamptz{Time: s.now().UTC(), Valid: true},
LastErrorCode:    pgtype.Text{String: "baofu_acceptance_rejected", Valid: true},
LastErrorMessage: pgtype.Text{String: strings.TrimSpace(upstream.Remark), Valid: strings.TrimSpace(upstream.Remark) != ""},
```

当前 `WithdrawResult` 没有独立错误码字段，`Remark` 对应宝付 `transRemark`。如果 `Remark` 为空，`LastErrorMessage.Valid` 保持 `false`；不要为了补错误摘要把完整 provider 原文塞进前端响应。

- [ ] 如果改为调用 `logic.PaymentCommandService.RecordExternalPaymentCommand`，必须先把多角色资金 owner 常量补齐：

```go
const (
	ExternalPaymentBusinessOwnerMerchantFunds  = "merchant_finance"
	ExternalPaymentBusinessOwnerRiderIncome    = "rider_income"
	ExternalPaymentBusinessOwnerOperatorFunds  = "operator_finance"
	ExternalPaymentBusinessOwnerPlatformFunds  = "platform_finance"
)
```

同时在 `logic/payment_command_service.go` 的 `isExternalPaymentBusinessOwner` 中加入三类新增 owner。若本卡只沿用直接 store 插入，也必须至少把 `businessOwnerForBaofuWithdrawal` 的 `"rider"`、`"operator"`、`"platform"` 魔法字符串替换成常量，避免审计 owner 再次漂移。

- [ ] 在 `locallife/api/baofu_withdrawal.go` 中让 create handler 能拿到 typed error 时的 result。推荐新增私有 helper：

```go
func (server *Server) respondBaofuWithdrawalCreateResultError(ctx *gin.Context, result logic.BaofuCreateWithdrawalResult, err error) bool {
	if errors.Is(err, logic.ErrBaofuWithdrawCreateRejected) {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("提现申请未被受理，请刷新余额后重试")))
		return true
	}
	return false
}
```

调用位置：

```go
result, err := server.baofuWithdrawService.CreateWithdrawal(...)
if err != nil {
	if server.respondBaofuWithdrawalCreateResultError(ctx, result, err) {
		return
	}
	server.respondBaofuWithdrawalCreateError(ctx, err)
	return
}
```

### 日志要求

本卡必须让受理失败和相关内部错误可观测：

- Provider 明确拒绝时记录 `Warn`，字段包含：
  - `owner_type`
  - `owner_id`
  - `baofu_withdrawal_order_id`
  - `out_request_no`
  - `upstream_state`
  - `provider_error_code`，仅当有稳定错误码
  - `provider_error_message`，仅保存稳定摘要，不保存完整 raw
- 状态更新失败、命令记录失败走 `Error`，字段包含上述定位字段和 `err`。
- 不记录完整 `contractNo`、银行卡号、证件号、手机号、证书、密钥、完整 provider raw。

如果服务层已有统一“不打日志、由 API 打一次”的约定，provider 明确拒绝仍可在服务层打 `Warn`，因为它是 provider 业务事实；内部错误继续由 `internalError` 或 `loggedServerError` 兜底。不要让同一个内部错误在服务层和 API 层重复以 `Error` 级别记录。

### 前端反馈语义

本卡后端必须返回稳定中文：

- HTTP：`409 Conflict`
- Message：`提现申请未被受理，请刷新余额后重试`

小程序即使尚未改动，也应能通过 `getErrorUserMessage(error, fallback)` 显示上述语义。若当前 `409` 被 `mapErrorByStatusCode` 覆盖为泛化文案，则在 `TASK-BAOFU-WD-FIX-003` 修复前端安全文案放行。

### 测试要求

- [ ] 在 `locallife/logic/baofu_withdraw_service_test.go` 新增测试：provider 返回 `UpstreamState:"2"`、`Status: db.BaofuWithdrawalStatusFailed` 时：
  - 已创建本地订单后调用 `UpdateBaofuWithdrawalOrderStatus`
  - `Status` 参数为 `failed`
  - 不调用 `UpdateBaofuWithdrawalOrderToProcessing`
  - `CreateWithdrawal` 返回 `ErrBaofuWithdrawCreateRejected`
  - result 内带有更新后的 failed 订单
  - 审计命令为 `rejected`

- [ ] 在 `locallife/api/baofu_withdrawal_contract_test.go` 新增测试：四类角色任选一条 create 路由即可覆盖 shared handler，provider 返回受理失败时：
  - HTTP 状态为 `409`
  - response message 为 `提现申请未被受理，请刷新余额后重试`
  - response body 不包含 provider raw、`contractNo` 或英文诊断串

### 验证命令

从 `locallife/` 执行：

```bash
go test ./logic -run 'TestBaofuWithdrawServiceCreateWithdrawal.*Rejected' -count=1
go test ./api -run 'TestCreateBaofuWithdrawal.*Rejected|TestBaofuWithdrawal.*Rejected' -count=1
make check-baofu-contract
```

如果修改了 sqlc query、mock-backed interface 或 Swagger 注解，再执行：

```bash
make sqlc
make mock
make swagger
make check-generated
```

### 验收标准

- `state=2` 不再留下 `processing` 订单。
- 受理失败订单 `finished_at` 被设置，后续 recovery 不再扫描该订单。
- 外部支付命令能区分 `accepted` 与 `rejected`。
- API 返回稳定中文业务文案，前端不会看到 provider/raw/internal 错误。
- 日志能通过 `out_request_no` 和本地订单 ID 关联 provider 拒绝、状态更新和命令记录。

### 建议提交

```bash
git add locallife/logic/baofu_withdraw_service.go locallife/api/baofu_withdrawal.go locallife/logic/baofu_withdraw_service_test.go locallife/api/baofu_withdrawal_contract_test.go
git commit -m "fix(baofu): reject failed withdrawal acceptance"
```

如果包含生成文件，把生成文件一并加入同一个提交。

---

## TASK-BAOFU-WD-FIX-002：provider 结果未知时返回“确认中”而不是“重试”

### 目标

当本地 `baofu_withdrawal_orders` 已创建，但 `CreateWithdraw` 因网络、超时、provider 5xx、空响应或解析外错误返回未知结果时，API 必须返回“提现申请已提交，结果正在确认，请勿重复提交”语义，并带回本地提现单。前端随后进入详情页或列表刷新，不能提示用户立即重试提交。

### 当前证据

- `locallife/logic/baofu_withdraw_service.go` 当前先创建本地 `processing` 订单，再调用 provider。
- provider 调用失败或返回 nil 后，当前直接返回错误，本地 `processing` 订单保留。
- `locallife/worker/baofu_withdrawal_recovery_scheduler.go` 会扫描 `processing` 订单并通过 `out_request_no` 查询宝付最终状态。
- `locallife/api/baofu_withdrawal.go` 当前默认把 create 错误映射为 `502` + `提现申请暂不可提交，请稍后重试`，这会让用户重复发起另一笔提现。

### 修改范围

后端文件：

- 修改：`locallife/logic/baofu_withdraw_service.go`
- 修改：`locallife/api/baofu_withdrawal.go`
- 修改：`locallife/logic/baofu_withdraw_service_test.go`
- 修改：`locallife/api/baofu_withdrawal_contract_test.go`
- 可能修改：`locallife/db/sqlc/constants.go`
- 可能修改：`locallife/logic/payment_command_service.go`

小程序文件：

- 修改：四个 create 页，让 `202 Accepted` 或 `201 Created` 且带 `withdrawal` 的响应都进入详情页：
  - `weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.ts`
  - `weapp/miniprogram/pages/operator/finance/withdrawals/create/index.ts`
  - `weapp/miniprogram/pages/platform/finance/withdrawals/create/index.ts`
  - `weapp/miniprogram/pages/rider/income/withdrawals/create/index.ts`
- 修改：`weapp/miniprogram/api/baofu-withdrawal.ts`，如当前类型没有 `message` 字段。
- 修改：`weapp/scripts/check-baofu-withdrawal-workflow.js`

### 不在本卡处理

- 不新增客户端自定义幂等键。
- 不改变 recovery 查询的批次策略、间隔和终态 fact application。
- 不做“再次点击按钮合并到旧单”的完整产品设计。
- 不处理 provider 明确 `state=2` 的拒绝分支，该问题属于 `TASK-BAOFU-WD-FIX-001`。

### 后端实现步骤

- [ ] 在 `locallife/logic/baofu_withdraw_service.go` 增加 typed error：

```go
var (
	ErrBaofuWithdrawCreateResultUnknown = errors.New("baofu withdraw create result unknown")
)
```

与 `ErrBaofuWithdrawCreateRejected` 并列，不要用 `err.Error()` 字符串匹配。

- [ ] 扩展 `BaofuCreateWithdrawalResult`，让未知结果能把本地订单带回 handler：

```go
type BaofuCreateWithdrawalResult struct {
	WithdrawalOrder db.BaofuWithdrawalOrder
	SyncState       string
	UserMessage     string
}
```

约定：

- `SyncState="accepted"`：provider 明确受理成功，本地订单 `processing`。
- `SyncState="rejected"`：provider 明确受理失败，本地订单 `failed`。
- `SyncState="unknown"`：本地订单已创建，provider 同步结果未知，本地订单 `processing`，等待恢复查询。

- [ ] 在本地订单创建后立刻赋值，保证 provider 调用失败时 result 仍携带 durable anchor：

```go
withdrawalOrder, err := s.store.CreateBaofuWithdrawalOrder(...)
if err != nil {
	return result, err
}
result.WithdrawalOrder = withdrawalOrder
```

- [ ] provider 调用失败时，不返回泛化 provider 错误。必须记录未知语义并返回 typed error：

```go
upstream, err := s.client.CreateWithdraw(ctx, baofucontracts.WithdrawRequest{...})
if err != nil {
	result.SyncState = "unknown"
	result.UserMessage = "提现申请已提交，结果正在确认，请勿重复提交"
	return result, fmt.Errorf("%w: %w", ErrBaofuWithdrawCreateResultUnknown, err)
}
if upstream == nil {
	result.SyncState = "unknown"
	result.UserMessage = "提现申请已提交，结果正在确认，请勿重复提交"
	return result, fmt.Errorf("%w: empty provider result", ErrBaofuWithdrawCreateResultUnknown)
}
```

- [ ] 外部支付命令不能在 provider 调用前固定写 `submitted` 后不再修正。推荐把命令写入移动到 provider outcome 分支：
  - provider `state=1`：写 `accepted`
  - provider `state=2`：写 `rejected`
  - provider error/nil：写 `unknown`

未知结果命令参数必须包含：

```go
CommandStatus:      db.ExternalPaymentCommandStatusUnknown,
SubmittedAt:        s.now().UTC(),
LastErrorCode:      pgtype.Text{String: "create_withdraw_unknown", Valid: true},
LastErrorMessage:   pgtype.Text{String: "provider create result unknown; recovery will query by out_request_no", Valid: true},
ResponseSnapshot:   baofuWithdrawCommandSnapshot(result.WithdrawalOrder),
ExternalObjectKey:  outRequestNo,
BusinessObjectType: pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
BusinessObjectID:   pgtype.Int8{Int64: result.WithdrawalOrder.ID, Valid: true},
```

若 `ExternalPaymentCommandStatusUnknown` 常量已存在，直接复用。若不存在，先确认 schema 枚举/约束是否允许 `unknown`，再补常量和测试；不要临时写入未被 schema 接受的字符串。

- [ ] 审计命令记录失败时必须打日志。用户语义以本地订单和 provider outcome 为准：
  - provider outcome 已知或未知且本地订单已持久化时，命令审计失败不能把响应改成“请重试提交”。
  - 日志必须标记为 `Error`，并包含 `baofu_withdrawal_order_id`、`out_request_no`、`command_status`。
  - 如果团队要求审计命令是强一致前置条件，则必须先补 outbox/repair 设计；不能在本卡里无声回滚或诱导重复提交。

- [ ] 在 `locallife/api/baofu_withdrawal.go` 扩展 create response：

```go
type baofuWithdrawalCreateResponse struct {
	Withdrawal baofuWithdrawalItem `json:"withdrawal"`
	Message    string              `json:"message,omitempty"`
}
```

- [ ] 创建 handler 遇到未知结果时返回 `202 Accepted`：

```go
if errors.Is(err, logic.ErrBaofuWithdrawCreateResultUnknown) && result.WithdrawalOrder.ID != 0 {
	item := newBaofuWithdrawalItem(result.WithdrawalOrder)
	item.SyncState = "unknown"
	item.SyncMessage = "提现申请已提交，结果正在确认，请勿重复提交"
	ctx.JSON(http.StatusAccepted, baofuWithdrawalCreateResponse{
		Withdrawal: item,
		Message:    "提现申请已提交，结果正在确认，请勿重复提交",
	})
	return
}
```

- [ ] 保持其他 provider 创建前失败的原语义：
  - 余额查询失败：`502` + `提现账户余额暂不可确认，请稍后刷新`
  - 账户未开通：`409` + `结算账户未开通，暂不能提现`
  - 金额不足：`409` + `可提现金额不足`

### 日志要求

未知结果必须有一条结构化日志，建议在服务层记录 provider outcome，并避免 API 层重复输出 provider 原文：

字段：

- `owner_type`
- `owner_id`
- `baofu_withdrawal_order_id`
- `out_request_no`
- `amount_fen`
- `sync_state="unknown"`
- `provider="baofu"`
- `capability="withdraw"`
- `err`

日志消息固定为：

```text
baofu withdrawal create result unknown after local order persisted
```

禁止日志字段：

- 完整 `contractNo`
- 完整 provider request/response raw
- 银行卡、证件号、手机号
- 密钥、证书、token

### 前端反馈语义

后端返回：

- HTTP：`202 Accepted`
- Body：

```json
{
  "withdrawal": {
    "id": 91,
    "out_request_no": "RBW...",
    "amount": 1200,
    "status": "processing",
    "status_text": "提现处理中",
    "sync_state": "unknown",
    "sync_message": "提现申请已提交，结果正在确认，请勿重复提交",
    "created_at": "2026-05-19T12:00:00Z",
    "updated_at": "2026-05-19T12:00:00Z"
  },
  "message": "提现申请已提交，结果正在确认，请勿重复提交"
}
```

小程序 create 页必须满足：

- `createBaofuWithdrawal(role, { amount })` resolve 且 response 带 `withdrawal.id` 时，无论底层 HTTP 是 `201` 还是 `202`，都进入详情页。
- toast 或页面提示使用 `response.message || response.withdrawal.sync_message || '提现申请已提交，请等待处理结果'`。
- 不显示“稍后重试提交”。
- 提交按钮在 request pending 期间保持禁用，避免重复点击。

### 测试要求

- [ ] 在 `locallife/logic/baofu_withdraw_service_test.go` 新增测试：provider `CreateWithdraw` 返回 error：
  - 本地订单已创建
  - result 带 `WithdrawalOrder`
  - 返回 `ErrBaofuWithdrawCreateResultUnknown`
  - 审计命令记录 `unknown`
  - 不调用 `UpdateBaofuWithdrawalOrderToProcessing`
  - 不调用 `UpdateBaofuWithdrawalOrderStatus`

- [ ] 新增测试：provider `CreateWithdraw` 返回 nil：
  - 返回 `ErrBaofuWithdrawCreateResultUnknown`
  - result 带本地 `processing` 订单
  - 审计命令记录 `unknown`

- [ ] 在 `locallife/api/baofu_withdrawal_contract_test.go` 新增测试：create provider error after local order：
  - HTTP 状态为 `202`
  - body 有 `withdrawal.id`
  - `withdrawal.status` 为 `processing`
  - `withdrawal.sync_state` 为 `unknown`
  - message 为 `提现申请已提交，结果正在确认，请勿重复提交`
  - body 不包含 provider 原始错误文本

- [ ] 在 `weapp/scripts/check-baofu-withdrawal-workflow.js` 增加断言：
  - 四个 create 页成功分支只依赖 `withdrawal.id` 跳详情，不把 `202` 当失败。
  - 检查源码包含 `sync_message` 或 `message` 的用户提示读取。

### 验证命令

后端从 `locallife/` 执行：

```bash
go test ./logic -run 'TestBaofuWithdrawServiceCreateWithdrawal.*Unknown' -count=1
go test ./api -run 'TestCreateBaofuWithdrawal.*Unknown|TestBaofuWithdrawal.*Unknown' -count=1
go test ./worker -run 'TestBaofuWithdrawal|TestProcessTaskBaofuWithdrawal' -count=1
make check-baofu-contract
```

小程序从 `weapp/` 执行：

```bash
npm run check:baofu-withdrawal-workflow
npm run compile
```

如果本卡修改了共享小程序 API 类型或多页行为，执行：

```bash
npm run quality:check
```

### 验收标准

- 本地订单已创建后，provider 同步结果未知不会返回“稍后重试提交”。
- API 返回 `202`，并携带可跳转详情页的 `withdrawal.id`。
- recovery 仍能基于 `processing` 订单和 `out_request_no` 查询终态。
- 外部支付命令能记录 `unknown`，并能通过本地订单 ID 和 `out_request_no` 关联。
- 日志能定位 provider 未知结果和命令记录失败。
- 前端不会泄露 provider/internal 错误，也不会诱导用户重复发起新提现。

### 建议提交

```bash
git add locallife/logic/baofu_withdraw_service.go locallife/api/baofu_withdrawal.go locallife/logic/baofu_withdraw_service_test.go locallife/api/baofu_withdrawal_contract_test.go weapp/miniprogram/api/baofu-withdrawal.ts weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.ts weapp/miniprogram/pages/operator/finance/withdrawals/create/index.ts weapp/miniprogram/pages/platform/finance/withdrawals/create/index.ts weapp/miniprogram/pages/rider/income/withdrawals/create/index.ts weapp/scripts/check-baofu-withdrawal-workflow.js
git commit -m "fix(baofu): return pending confirmation for unknown withdrawal create"
```

---

## TASK-BAOFU-WD-FIX-003：小程序提现列表拆分余额失败和记录失败

### 目标

四类小程序提现列表页必须把“宝付实时余额查询失败”和“本地提现记录查询失败”拆开处理。余额失败时仍展示本地提现记录，并禁用发起提现入口；记录失败时仍可展示余额区域和明确的记录加载失败提示。

### 当前证据

以下四个页面当前使用同一个 `Promise.all` 同时加载余额和记录：

- `weapp/miniprogram/pages/merchant/finance/withdrawals/index.ts`
- `weapp/miniprogram/pages/operator/finance/withdrawals/index.ts`
- `weapp/miniprogram/pages/platform/finance/withdrawals/index.ts`
- `weapp/miniprogram/pages/rider/income/withdrawals/index.ts`

现状问题：

- `getBaofuWithdrawalBalance(role)` 失败会导致 `listBaofuWithdrawals(role)` 的成功结果被丢弃。
- 用户在 provider 余额接口抖动时无法查看本地提现历史。
- create 入口只依赖 `balanceView.canSubmit`，但页面没有独立的 `balanceErrorMessage` 语义。
- `weapp/miniprogram/utils/user-facing.ts` 对 5xx 默认返回 `服务暂时不可用，请稍后再试`，可能吞掉后端安全的提现文案。

### 修改范围

小程序文件：

- 修改：`weapp/miniprogram/pages/merchant/finance/withdrawals/index.ts`
- 修改：`weapp/miniprogram/pages/merchant/finance/withdrawals/index.wxml`
- 修改：`weapp/miniprogram/pages/operator/finance/withdrawals/index.ts`
- 修改：`weapp/miniprogram/pages/operator/finance/withdrawals/index.wxml`
- 修改：`weapp/miniprogram/pages/platform/finance/withdrawals/index.ts`
- 修改：`weapp/miniprogram/pages/platform/finance/withdrawals/index.wxml`
- 修改：`weapp/miniprogram/pages/rider/income/withdrawals/index.ts`
- 修改：`weapp/miniprogram/pages/rider/income/withdrawals/index.wxml`
- 修改：`weapp/miniprogram/utils/user-facing.ts`
- 修改：`weapp/scripts/check-baofu-withdrawal-workflow.js`

可选文件：

- 如果四页重复代码过多，可在 `weapp/miniprogram/services/baofu-withdrawal-workflow.ts` 新增纯函数，例如 `buildBaofuWithdrawalBalanceUnavailableView(message)`。该函数不得发起请求，不得做路由跳转。

### 不在本卡处理

- 不新增新的提现 API。
- 不改后端 balance/list/detail/create contract。
- 不重做四个页面的视觉设计。
- 不把 provider 余额查询失败降级为可发起提现。
- 不用本地历史记录、收入累计或缓存余额推断可提现金额。

### 页面状态设计

每个列表页都要新增或等价表达这些状态字段：

```ts
balanceErrorMessage: string
recordsErrorMessage: string
balanceLoading: boolean
recordsLoading: boolean
```

状态语义：

- `balanceErrorMessage` 非空：余额不可确认，禁用创建提现入口，页面文案显示 `可提现余额暂不可确认，提现申请已暂停，提现记录可继续查看`。
- `recordsErrorMessage` 非空且无可信 rows：记录区域展示加载失败，不把整页渲染成空状态。
- 余额失败但记录成功：显示记录，余额区域显示不可确认提示，提现按钮禁用。
- 记录失败但余额成功：显示余额，记录区域显示失败提示，提现按钮是否可用仍由真实余额决定。
- 两者都失败且没有可信数据：首屏错误显示 `提现信息加载失败，请稍后重试`，同时保留刷新按钮。

### 实现步骤

- [ ] 把四个页面的 `fetchWithdrawals(page)` 从 `Promise.all` 改成 `Promise.allSettled`：

```ts
async fetchWithdrawals(page: number): Promise<MerchantWithdrawalFetchResult> {
  const [balanceResult, recordsResult] = await Promise.allSettled([
    getBaofuWithdrawalBalance('merchant'),
    listBaofuWithdrawals('merchant', { page, limit: PAGE_SIZE })
  ])

  return buildMerchantWithdrawalFetchResult(page, balanceResult, recordsResult)
}
```

每个角色替换 role：

- 商户：`merchant`
- 运营商：`operator`
- 平台：`platform`
- 骑手收入：`rider`

- [ ] 在每个页面本地实现明确的 result builder。以商户页为例：

```ts
function buildMerchantWithdrawalFetchResult(
  page: number,
  balanceResult: PromiseSettledResult<BaofuWithdrawalBalance>,
  recordsResult: PromiseSettledResult<BaofuWithdrawalsResponse>
): MerchantWithdrawalFetchResult {
  const balance =
    balanceResult.status === 'fulfilled' ? balanceResult.value : undefined
  const records =
    recordsResult.status === 'fulfilled' ? recordsResult.value : undefined

  if (!balance && !records) {
    throw new Error('提现信息加载失败，请稍后重试')
  }

  return {
    balance,
    balanceErrorMessage:
      balanceResult.status === 'rejected'
        ? getErrorUserMessage(
            balanceResult.reason,
            '可提现余额暂不可确认，提现申请已暂停，提现记录可继续查看'
          )
        : '',
    withdrawals: records?.withdrawals || [],
    recordsErrorMessage:
      recordsResult.status === 'rejected'
        ? getErrorUserMessage(recordsResult.reason, '提现记录加载失败，请稍后刷新')
        : '',
    page: records?.page || page,
    totalPages: records?.total_pages || 0
  }
}
```

如果当前 `MerchantWithdrawalFetchResult` 类型不允许 `balance` 可空，改成：

```ts
interface MerchantWithdrawalFetchResult {
  balance?: BaofuWithdrawalBalance
  balanceErrorMessage: string
  withdrawals: BaofuWithdrawal[]
  recordsErrorMessage: string
  page: number
  totalPages: number
}
```

其他三页使用各自类型名，不共享页面私有类型。

- [ ] `loadWithdrawals` setData 时必须分别处理：

```ts
const nextBalanceView = result.balance
  ? buildBaofuWithdrawalBalanceView(result.balance)
  : {
      availableAmountText: '--',
      pendingAmountText: '--',
      ledgerAmountText: '--',
      frozenAmountText: '--',
      canSubmit: false,
      disabledReason: result.balanceErrorMessage || '可提现余额暂不可确认'
    }

this.setData({
  initialLoading: false,
  initialError: false,
  initialErrorMessage: '',
  refreshErrorMessage: result.recordsErrorMessage,
  balanceErrorMessage: result.balanceErrorMessage,
  recordsErrorMessage: result.recordsErrorMessage,
  loadingWithdrawals: false,
  balanceView: nextBalanceView,
  rows,
  page: result.page,
  totalPages: result.totalPages,
  hasMore: result.page < result.totalPages
})
```

若当前 `balanceView` 字段名不同，保持原字段名，只保证“余额不可确认时创建按钮禁用”。

- [ ] `onOpenCreate` 增加余额错误优先提示：

```ts
if (this.data.balanceErrorMessage) {
  wx.showToast({
    title: this.data.balanceErrorMessage,
    icon: 'none'
  })
  return
}
```

然后再执行原有 `balanceView.canSubmit` 判断。

- [ ] WXML 中余额区域新增局部提示：

```xml
<t-notice-bar
  wx:if="{{balanceErrorMessage}}"
  theme="warning"
  content="{{balanceErrorMessage}}"
/>
```

记录区域新增局部失败提示：

```xml
<t-empty
  wx:if="{{recordsErrorMessage && !rows.length}}"
  description="{{recordsErrorMessage}}"
/>
```

如果当前页面不用 `t-empty`，使用现有空态组件，但文案必须来自 `recordsErrorMessage`。

- [ ] 修改 `weapp/miniprogram/utils/user-facing.ts`，放行提现相关安全中文文案。`SAFE_COPY_PREFIXES` 增加：

```ts
'提现',
'可提现',
'结算账户',
'余额'
```

同时调整 `getErrorUserMessage` 的调用顺序，确保后端返回的安全中文 message 优先于 `mapErrorByStatusCode(500+)` 的泛化文案。最终行为必须满足：

- 后端 message `提现账户余额暂不可确认，请稍后刷新` 被原样展示。
- 后端 message `提现申请已提交，结果正在确认，请勿重复提交` 被原样展示。
- 英文、SQL、provider raw、stack、timeout diagnostic 仍被映射成安全文案。

- [ ] 更新 `weapp/scripts/check-baofu-withdrawal-workflow.js`，增加硬性断言：

```js
for (const [role, group] of Object.entries(rolePageGroups)) {
  const listPage = read(group.listPagePath)
  assert(!listPage.includes('Promise.all(['), `${group.label} withdrawal list must not couple balance and records with Promise.all`)
  assert(listPage.includes('Promise.allSettled'), `${group.label} withdrawal list must isolate balance and records failures`)
  assert(listPage.includes('balanceErrorMessage'), `${group.label} withdrawal list must expose balance error state`)
  assert(listPage.includes('recordsErrorMessage'), `${group.label} withdrawal list must expose records error state`)
}

const userFacingSource = read('miniprogram/utils/user-facing.ts')
assert(userFacingSource.includes("'提现'"), 'User-facing mapper must allow safe withdrawal copy')
assert(userFacingSource.includes("'可提现'"), 'User-facing mapper must allow safe withdrawable-balance copy')
```

如果脚本当前的 `rolePageGroups` 没有 `listPagePath`，补成显式结构，避免从拼接 source 中无法定位列表页。

### 日志要求

小程序侧失败必须进入现有 `logger.warn`，并区分失败来源：

- 余额失败：`logger.warn('<Role> baofu withdrawal balance load failed', error)`
- 记录失败：`logger.warn('<Role> baofu withdrawals records load failed', error)`
- 两者都失败：允许再记录一条 `logger.warn('<Role> baofu withdrawals load failed', error)`，但不要吞掉上面两个来源。

前端日志不得包含用户银行卡、证件号、手机号或 provider raw。页面只记录 error object 交给现有 logger 处理，不新增手写 stringify raw payload。

### 前端反馈语义

页面必须展示这些稳定中文：

- 余额失败但记录成功：`可提现余额暂不可确认，提现申请已暂停，提现记录可继续查看`
- 记录失败但余额成功：`提现记录加载失败，请稍后刷新`
- 两者都失败且无可信数据：`提现信息加载失败，请稍后重试`
- 后端返回未知创建结果：`提现申请已提交，结果正在确认，请勿重复提交`
- 后端返回受理失败：`提现申请未被受理，请刷新余额后重试`

这些文案要么来自后端安全 message，要么来自页面 fallback。不能显示 `bad gateway`、`internal server error`、`baofu`、`SQL`、`timeout`、`request:fail` 等诊断串。

### 测试要求

- [ ] 更新 `weapp/scripts/check-baofu-withdrawal-workflow.js` 覆盖四个角色列表页：
  - 不允许使用 `Promise.all([getBaofuWithdrawalBalance, listBaofuWithdrawals])` 绑定成同成同败。
  - 必须存在 `Promise.allSettled` 或等价的独立 try/catch。
  - 必须存在 `balanceErrorMessage`。
  - 必须存在 `recordsErrorMessage`。
  - 必须存在余额失败禁用创建入口逻辑。

- [ ] 如果仓库已有小程序单元测试框架，补 `user-facing` mapper 测试：
  - `提现账户余额暂不可确认，请稍后刷新` 原样返回。
  - `提现申请已提交，结果正在确认，请勿重复提交` 原样返回。
  - `bad gateway` 映射为 `服务暂时不可用，请稍后再试`。

### 验证命令

从 `weapp/` 执行：

```bash
npm run check:baofu-withdrawal-workflow
npm run compile
npm run quality:check
```

如本地 `node`、`npm`、`npx` 找不到，按仓库指令用本地工具路径重试：

```bash
PATH="$HOME/.local/bin:$PATH" npm run check:baofu-withdrawal-workflow
PATH="$HOME/.local/bin:$PATH" npm run compile
PATH="$HOME/.local/bin:$PATH" npm run quality:check
```

### 验收标准

- 四个角色列表页在余额失败时仍能展示本地提现记录。
- 四个角色列表页在余额失败时禁用创建提现入口。
- 四个角色列表页在记录失败时不隐藏余额区域。
- 后端安全中文提现文案能透传到用户，不被 5xx/409 泛化文案覆盖。
- 小程序日志能区分余额失败和记录失败。
- 质量脚本能防止未来再把余额与记录用 `Promise.all` 绑定。

### 建议提交

```bash
git add weapp/miniprogram/pages/merchant/finance/withdrawals/index.ts weapp/miniprogram/pages/merchant/finance/withdrawals/index.wxml weapp/miniprogram/pages/operator/finance/withdrawals/index.ts weapp/miniprogram/pages/operator/finance/withdrawals/index.wxml weapp/miniprogram/pages/platform/finance/withdrawals/index.ts weapp/miniprogram/pages/platform/finance/withdrawals/index.wxml weapp/miniprogram/pages/rider/income/withdrawals/index.ts weapp/miniprogram/pages/rider/income/withdrawals/index.wxml weapp/miniprogram/utils/user-facing.ts weapp/scripts/check-baofu-withdrawal-workflow.js
git commit -m "fix(weapp): isolate baofu withdrawal balance failures"
```

---

## 批次验证建议

完成三张卡后，至少执行：

```bash
cd locallife
go test ./logic -run 'TestBaofuWithdrawServiceCreateWithdrawal.*(Rejected|Unknown|RecordsCommand)' -count=1
go test ./api -run 'TestCreateBaofuWithdrawal.*(Rejected|Unknown)|TestBaofuWithdrawal.*(Rejected|Unknown)' -count=1
go test ./worker -run 'TestBaofuWithdrawal|TestProcessTaskBaofuWithdrawal' -count=1
make check-baofu-contract
```

```bash
cd weapp
npm run check:baofu-withdrawal-workflow
npm run compile
npm run quality:check
```

如果改动 SQL、sqlc query、mock interface 或 Swagger，再执行：

```bash
cd locallife
make sqlc
make mock
make swagger
make check-generated
```

如果当前环境缺少 `go` 或 `npm`，不能宣称验证通过；交付时必须写明“未运行，原因是本地工具链不可用”，并列出上面的待执行命令。

---

## 批次完成定义

三张卡全部完成后，提现创建链路必须满足：

- 同步受理成功：API `201`，本地 `processing`，前端进入详情页等待结果。
- 同步受理失败：API `409`，本地 `failed`，前端提示刷新余额后重试。
- 同步结果未知但本地订单已创建：API `202`，本地 `processing`，前端进入详情页并提示结果确认中，请勿重复提交。
- 创建前余额不可确认：API `502`，前端提示余额暂不可确认，不创建本地提现单。
- 列表页余额查询失败：提现记录仍可查看，创建入口禁用。
- 列表页记录查询失败：余额区域仍可查看，记录区域显示局部失败。
- 所有后端异常都能通过结构化日志定位到 owner、订单、out_request_no 和 provider outcome；所有前端可见反馈都是中文产品语义。
