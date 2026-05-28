# 飞鹅云打印结果回调开发计划

> **For agentic workers:** REQUIRED ROUTING: follow root `AGENTS.md`, `.github/copilot-instructions.md`, `.github/README.md`, `.github/instructions/backend-locallife.instructions.md`, and `locallife/AGENTS.md`. This is backend external-provider callback work; treat as `G2`.

**Goal:** 支持飞鹅云打印结果回调，让下单后的自动打印状态以飞鹅云实际打印结果为准，并为后续失败补救留下可靠的持久化入口。

**Architecture:** 飞鹅云调用仍由 `cloudprint/` 作为 provider 边界封装；订单打印 worker 只依赖内部 `cloudprint.Client` 语义。新增公开 webhook 接收飞鹅云 `application/x-www-form-urlencoded` 回调，验签成功后通过 `vendor_order_id` 定位 `print_logs` 并更新打印状态。启用回调后，`Open_printMsg` 同步返回只代表“飞鹅云已接收任务”，本地日志保持 `pending`，回调成功才标记 `success`。

**Tech Stack:** Go, Gin, sqlc/pgx, gomock, Feieyun China Open API, RSA SHA256 signature verification.

---

## 1. 已确认的飞鹅云契约

来源：飞鹅云中国站开放平台文档 `https://help.feieyun.com/#/home/doc/zh;nav=1-8`，章节“打印状态回调”；中国站 API 地址 `https://api.feieyun.cn/Api/Open/`；回调验签公钥 `https://help.feieyun.com/assets/public.key`；用户提供的回调地址配置页说明；用户提供的验证文件 `/home/sam/下载/feieyun_verify_lPcw9LunHs8pqSvB.txt`。

### 打印结果回调

- 回调方式：`HTTPS POST`
- Content-Type：`application/x-www-form-urlencoded`
- 必填字段：
  - `orderId`：订单 ID，由 `Open_printMsg` 返回。
  - `status`：订单状态。官方文档当前只明确 `1` 表示打印成功。
  - `stime`：状态变更 UNIX 时间戳，10 位秒级。
  - `sign`：数字签名。
- 验签规则：
  - 取所有 POST 参数，排除 `sign`。
  - 排除空值参数和字节/文件字段。
  - 按参数名 ASCII 升序排序。
  - 拼接为 `key=value&key=value`。
  - 对 `sign` 做 base64 解码。
  - 使用飞鹅云公钥做 `SHA256WithRSA` 验签。
- 响应要求：5 秒内返回纯文本 `SUCCESS`，否则飞鹅云会重推。

### 回调地址配置

- 飞鹅云后台配置回调地址时，不支持 IP、端口号、短链域名。
- 域名需通过 ICP 备案验证。
- 需要先让飞鹅云验证文件可访问。
- 用户提供的验证文件：
  - 文件名：`feieyun_verify_lPcw9LunHs8pqSvB.txt`
  - 文件内容：`lPcw9LunHs8pqSvB`
  - 该文件由站点根目录或 Nginx/网关静态资源托管，不进入后端配置。
- 打印接口调用时还需要附加已配置的回调地址参数 `backurl`。

### 重要边界

- 飞鹅云 API 请求公共参数里的 `sig` 是 `SHA1(user + UKEY + stime)`，用于我们调用飞鹅云接口。
- 打印结果回调里的 `sign` 是另一套字段，文档要求用飞鹅云公钥做 `SHA256WithRSA` 验签。
- `FEIEYUN_UKEY` 是飞鹅云 API 调用签名密钥，不是回调 RSA 公钥。
- 飞鹅云回调公钥不是 TLS 证书，也不是域名验证 txt 文件。
- 回调验签需要新增独立配置：
  - `FEIEYUN_CALLBACK_PUBLIC_KEY_PEM`
  - 或 `FEIEYUN_CALLBACK_PUBLIC_KEY_PATH`
- 官方文档当前只明确成功枚举 `status=1`，未确认失败枚举。实现时不猜测失败状态；未知状态先记录日志并 ACK，避免飞鹅云无限重推造成噪音。失败自动补救可在后续基于官方失败枚举或查询接口 `Open_queryOrderState` 做恢复任务。

---

## 2. 当前代码差距

### 已有路径

- 飞鹅云 provider：`locallife/cloudprint/feieyun.go`
  - `Print(ctx, PrintInput)` 调用 `/Api/Open/printMsg`。
  - 当前参数包含 `sn`、`content`、`times`、可选 `expired`。
  - 当前未传 `backurl`。
- 打印 worker：`locallife/worker/task_print_order.go`
  - 创建 `print_logs.status=pending`。
  - 调用 `printerClient.Print`。
  - 只要同步调用无错误，就立即更新为 `success`。
  - 这会把“飞鹅云接收成功”误当成“打印机实际打印成功”。
- SQL：`locallife/db/query/print_log.sql`
  - 有按 ID、task key、订单和打印机查询。
  - 没有按 `vendor_order_id` 查找打印日志。
- 路由：`locallife/api/server.go`
  - `/v1/webhooks/*` 已绕过统一 JSON 响应信封，适合返回飞鹅云要求的纯文本 `SUCCESS`。

### 需要补齐

- 飞鹅云打印请求附加 `backurl`。
- 新增回调验签 helper。
- 新增飞鹅云打印结果 webhook。
- 新增按 `vendor_order_id` 查找 `print_logs` 的 sqlc 查询。
- 修改 worker 状态语义：回调模式下同步提交成功后保持 `pending`。
- 补齐配置、示例 env、测试和 provider 契约文档。

---

## 3. 目标行为

### 未配置回调 URL

- 保持现有行为：
  - `Open_printMsg` 调用成功后，`print_logs.status=success`。
  - 调用失败后，`print_logs.status=failed`。
- 这样本地开发或未启用飞鹅云回调的环境不受影响。

### 已配置回调 URL

- 同时配置 `FEIEYUN_PRINT_CALLBACK_URL` 和飞鹅云回调公钥后，`Open_printMsg` 请求包含 `backurl=<FEIEYUN_PRINT_CALLBACK_URL>`。
- `Open_printMsg` 调用成功：
  - 保存飞鹅云返回的 `vendor_order_id`。
  - `print_logs.status` 保持 `pending`。
  - 日志文案表达为 “print job accepted by feieyun”，不再表达为已打印成功。
- 飞鹅云回调 `status=1` 且验签通过：
  - 按 `orderId` 查找 `print_logs.vendor_order_id`。
  - 将该日志更新为 `success`，并写入 `printed_at=now()`。
  - 返回纯文本 `SUCCESS`。
- 验签失败：
  - 返回非 `SUCCESS`，建议 `401 FAIL`。
  - 不更新数据库。
- 回调缺少必填字段或字段类型错误：
  - 返回非 `SUCCESS`，建议 `400 FAIL`。
  - 不更新数据库。
- 找不到对应 `vendor_order_id`：
  - 记录结构化告警。
  - 返回非 `SUCCESS`，让飞鹅云稍后重推，避免回调早于 worker 保存 `vendor_order_id` 时被永久吞掉。

---

## 4. 文件计划

### Provider 边界

- Modify: `locallife/cloudprint/feieyun.go`
  - `FeieyunClient` 增加 `printCallbackURL string`。
  - `NewFeieyunClientFromConfig` 从 `config.FeieyunPrintCallbackURL` 注入。
  - `Print` 在 callback URL 非空时附加 `backurl`。
  - `Client` interface 增加 `PrintResultCallbackEnabled() bool`，用于 worker 判断同步提交后是否保持 pending。

- Create: `locallife/cloudprint/feieyun_callback.go`
  - `BuildFeieyunCallbackCanonicalString(values url.Values) string`
  - `VerifyFeieyunCallbackSignature(values url.Values, publicKeyPEM string) error`
  - 支持飞鹅云 RSA 公钥 PEM，例如 `BEGIN PUBLIC KEY` 或 `BEGIN RSA PUBLIC KEY`。

- Test: `locallife/cloudprint/feieyun_test.go`
  - 断言 `Print` 在配置 callback URL 时传 `backurl`。
  - 断言未配置 callback URL 时不传 `backurl`。
  - 断言验签 canonical string 排除 `sign` 和空值并按 ASCII 排序。
  - 用测试 RSA key 验证签名成功和失败分支。

### 配置

- Modify: `locallife/util/config.go`
  - 新增：
    - `FeieyunPrintCallbackURL string`
    - `FeieyunCallbackPublicKeyPEM string`
    - `FeieyunCallbackPublicKeyPath string`
  - 支持从 `FEIEYUN_CALLBACK_PUBLIC_KEY_PATH` 读取公钥。
  - 支持 env 中 PEM 的 `\n` 转真实换行。

- Modify: `locallife/app.env.example`
  - 增加：
    - `FEIEYUN_PRINT_CALLBACK_URL=https://yourdomain.com/v1/webhooks/feieyun/print-result`
    - `FEIEYUN_CALLBACK_PUBLIC_KEY_PEM=`
    - `FEIEYUN_CALLBACK_PUBLIC_KEY_PATH=`

- Test: `locallife/util/config_test.go`
  - 覆盖 env 加载 callback URL、公钥 PEM、公钥 path。

### SQL / 持久化

- Modify: `locallife/db/query/print_log.sql`
  - 新增：

```sql
-- name: GetPrintLogByVendorOrderID :one
SELECT id, order_id, printer_id, print_content, status, error_message, printed_at, created_at, vendor_order_id, task_key
FROM print_logs
WHERE vendor_order_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;
```

- Regenerate:
  - Run from `locallife/`: `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make sqlc`
  - This updates `locallife/db/sqlc/print_log.sql.go`, `locallife/db/sqlc/querier.go`, and `locallife/db/mock/store.go`.

### API / 回调

- Create: `locallife/api/feieyun_callback.go`
  - `handleFeieyunPrintResultNotify`
    - `POST /v1/webhooks/feieyun/print-result`
    - `ParseForm` 后校验 `orderId`、`status`、`stime`、`sign`。
    - 用配置中的飞鹅云公钥验签。
    - `status=1` 时按 `vendor_order_id` 更新 `print_logs.status=success`。
    - 未知 status：结构化日志记录并返回 `SUCCESS`，不更新本地状态。
    - 所有响应都用 `ctx.String(...)` 返回纯文本，不走 JSON envelope。

- Modify: `locallife/api/server.go`
  - 在 `webhooksGroup` 注册 `POST /feieyun/print-result`。

- Test: `locallife/api/feieyun_callback_test.go`
  - 合法签名 + `status=1` 更新对应 print log 为 `success` 并返回 `SUCCESS`。
  - 非法签名返回非 `SUCCESS`，且不更新数据库。
  - 缺少公钥配置返回非 `SUCCESS`，且不更新数据库。
  - 未知 `vendor_order_id` 返回非 `SUCCESS`，让飞鹅云后续重推，避免回调早于本地保存 `vendor_order_id` 时被永久吞掉。
  - 未知 `status` 返回 `SUCCESS` 且不更新状态。

### Worker / 自动打印状态

- Modify: `locallife/worker/task_print_order.go`
  - 调用 `processor.printerClient.PrintResultCallbackEnabled()` 判断是否启用异步结果确认。
  - 未启用回调：保持当前成功/失败更新逻辑。
  - 启用回调：
    - 同步调用失败：更新 `failed`。
    - 同步调用成功：保存 `vendor_order_id`，状态保持 `pending`。
    - 如果同步调用成功但没有返回 `vendor_order_id`，记录 error 并将日志标记 `failed`，因为后续无法通过回调关联。

- Test: `locallife/worker/task_print_order_test.go`
  - 更新 test stub 实现 `PrintResultCallbackEnabled() bool`。
  - 新增测试：
    - 回调未启用时同步成功仍更新 `success`。
    - 回调启用时同步成功更新时 status 仍为 `pending` 并保存 `vendor_order_id`。
    - 回调启用且无 `vendor_order_id` 时标记 `failed`。

### 契约文档

- Create: `.github/standards/domains/cloud-print/README.md`
  - 记录飞鹅云打印接口、回调接口、字段矩阵、签名规则、响应规则。
  - 明确官方当前只给出 `status=1` 成功枚举，失败枚举未知，不在代码中猜测。

---

## 5. 实施任务

### Task 1: 写 provider 和验签测试

- [ ] 在 `locallife/cloudprint/feieyun_test.go` 添加 `backurl` 请求参数测试。
- [ ] 在 `locallife/cloudprint/feieyun_callback_test.go` 添加 canonical string 和 RSA 验签测试。
- [ ] Run: `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" go test ./cloudprint`
- [ ] Expected: 新测试先失败，失败原因是字段、方法或 helper 不存在。

### Task 2: 实现 provider callback URL 和签名 helper

- [ ] 修改 `locallife/cloudprint/feieyun.go`，保存 callback URL 并在 `Print` 时传 `backurl`。
- [ ] 新增 `PrintResultCallbackEnabled() bool`。
- [ ] 新增 `locallife/cloudprint/feieyun_callback.go` 实现 canonical string 和 RSA 验签。
- [ ] Run: `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" go test ./cloudprint`
- [ ] Expected: PASS。

### Task 3: 写配置测试并实现配置加载

- [ ] 在 `locallife/util/config_test.go` 添加飞鹅云回调配置测试。
- [ ] 修改 `locallife/util/config.go` 添加配置字段、默认值、公钥 path 读取和 PEM 换行处理。
- [ ] 修改 `locallife/app.env.example`。
- [ ] Run: `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" go test ./util -run Feieyun`
- [ ] Expected: PASS。

### Task 4: 增加 SQL 查询并生成 sqlc/mock

- [ ] 修改 `locallife/db/query/print_log.sql` 添加 `GetPrintLogByVendorOrderID`。
- [ ] Run from `locallife/`: `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make sqlc`
- [ ] 确认生成物包含：
  - `db/sqlc/print_log.sql.go`
  - `db/sqlc/querier.go`
  - `db/mock/store.go`

### Task 5: 写 API 回调测试

- [ ] 新增 `locallife/api/feieyun_callback_test.go`。
- [ ] 覆盖合法回调、非法签名、缺公钥、未知 vendor order 可重试、未知 status。
- [ ] Run: `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" go test ./api -run Feieyun`
- [ ] Expected: 新测试先失败，失败原因是路由或 handler 不存在。

### Task 6: 实现 API 回调路由

- [ ] 新增 `locallife/api/feieyun_callback.go`。
- [ ] 修改 `locallife/api/server.go` 注册：
  - `POST /v1/webhooks/feieyun/print-result`
- [ ] Run: `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" go test ./api -run Feieyun`
- [ ] Expected: PASS。

### Task 7: 修改 worker 状态语义

- [ ] 在 `locallife/worker/task_print_order_test.go` 添加回调模式测试。
- [ ] 修改 `locallife/worker/task_print_order.go`，回调模式下同步成功保持 `pending`。
- [ ] Run: `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" go test ./worker -run 'PrintOrder'`
- [ ] Expected: PASS。

### Task 8: 更新云打印 provider 契约文档

- [ ] 新增 `.github/standards/domains/cloud-print/README.md`。
- [ ] 写入字段矩阵、回调验签规则、响应 `SUCCESS` 规则、已知状态枚举。
- [ ] 明确 `FEIEYUN_UKEY` 与回调 RSA 公钥用途不同。

### Task 9: 生成与最终验证

- [ ] Run from `locallife/`: `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make check-generated`
- [ ] Run from `locallife/`: `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make test-safety`
- [ ] Focused tests:
  - `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" go test ./cloudprint`
  - `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" go test ./util -run Feieyun`
  - `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" go test ./api -run Feieyun`
  - `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" go test ./worker -run 'PrintOrder'`
- [ ] If route annotations are added, run `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make swagger`; otherwise state Swagger regeneration was not required.

---

## 6. 上线配置

实现完成后，飞鹅云后台应配置：

- 验证文件 URL：`https://<your-domain>/feieyun_verify_lPcw9LunHs8pqSvB.txt`
- 打印结果回调 URL：`https://<your-domain>/v1/webhooks/feieyun/print-result`

`app.env` 应新增或确认：

```env
FEIEYUN_ENABLED=true
FEIEYUN_PRINT_CALLBACK_URL=https://<your-domain>/v1/webhooks/feieyun/print-result
FEIEYUN_CALLBACK_PUBLIC_KEY_PATH=/path/to/feieyun_public.key
# 或：
# FEIEYUN_CALLBACK_PUBLIC_KEY_PEM="-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----"
```

说明：

- 如果生产环境已有 `FEIEYUN_ENABLED=true`，仍必须配置 `FEIEYUN_PRINT_CALLBACK_URL` 才会启用异步结果确认。
- 实际启用异步结果确认还需要同时配置飞鹅云回调公钥；只配置 URL 时不会传 `backurl`，避免回调到达后无法验签。
- 如果不配置 `FEIEYUN_PRINT_CALLBACK_URL`，保持当前同步成功即成功的兼容行为。
- 回调公钥应使用飞鹅云文档“打印状态回调”章节提供的飞鹅云公钥，不使用 `FEIEYUN_UKEY`。
- 中国站当前公钥下载路径为 `https://help.feieyun.com/assets/public.key`。
- 验证文件 `/home/sam/下载/feieyun_verify_lPcw9LunHs8pqSvB.txt` 需要放到生产站点根目录或由 Nginx/网关托管，后端不提供该文件路由。

---

## 7. 补派打印与失败补救策略

本轮实现的补救基础：

- 所有打印任务都会有 `print_logs` 持久化记录。
- 飞鹅云返回的 `orderId` 会保存到 `print_logs.vendor_order_id`。
- 回调成功会将对应记录标记为 `success`。
- 未成功回调的记录会停留在 `pending`，现有异常列表和补打入口可以继续识别。

本轮不直接实现自动失败重打，原因：

- 官方文档当前只确认 `status=1` 表示打印成功，未确认失败枚举。
- 如果猜测失败状态并自动重打，可能造成重复打印或错误状态推进。

后续可选增强：

- 新增 scheduler 查询长时间 `pending` 且有 `vendor_order_id` 的打印日志。
- 调用 `Open_queryOrderState` 做二次确认。
- 对确认未打印成功或超时未确认的记录标记 `failed`，并进入现有补打流程。
- 如果飞鹅云后续提供失败状态枚举，再在回调 handler 中显式处理失败状态。

---

## 8. 风险与验收标准

### 风险等级

- `G2`：第三方 provider callback、异步状态、worker 状态语义、sqlc 查询和公开 webhook。
- 未升级到 `G3`：该路径不直接处理资金、身份、授权或敏感证件，但仍需要严格验签和重复投递处理。

### 验收标准

- 飞鹅云后台可以访问验证文件。
- 打印请求在配置 callback URL 后带 `backurl`。
- 回调验签失败不会更新本地打印状态。
- 合法 `status=1` 回调能把对应 `print_logs` 更新为 `success`。
- 启用回调后，同步 `Open_printMsg` 成功不会提前标记本地打印成功。
- 未配置回调 URL 时现有自动打印行为不回归。
- 重复回调保持幂等：重复设置 `success` 不应造成业务副作用。
- 所有新增配置出现在 `app.env.example`。
- `make sqlc` 和相关测试通过，生成物一致。
