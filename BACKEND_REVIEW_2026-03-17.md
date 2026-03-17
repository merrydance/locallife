# 后端生产级全量审查与改进方案（2026-03-17）

## 1. 审查范围与方法
- 范围：`locallife/` 后端 Go 服务（API、调度、WebSocket、worker、db 访问层、配置与启动流程）。
- 方法：
  - 全量构建检查：`go test ./... -run '^$'`（已通过）。
  - 关键风险模式扫描：异步 goroutine、`context.Background()`、错误吞没、CORS、安全头、调度器、支付/退款链路。
  - 人工复核高风险文件：`api/`、`scheduler/`、`main.go`、`websocket/`、`util/`。
- 说明：以下问题以“生产可维护性、鲁棒性、安全性”优先，而非演示型代码风格建议。

## 2. 结论摘要
- P0（必须优先处理）：2 项
- P1（高优先级）：5 项
- P2（中优先级）：3 项

建议先完成 P0 + P1，再安排 P2 进入两周内技术债冲刺。

---

## 3. 发现列表（按严重级别）

### P0-1 CORS 策略可被误配置为“任意站点带凭证访问”
- 证据：`locallife/api/middleware_security.go:69`、`locallife/api/middleware_security.go:71`
- 现象：
  - 当 `ALLOWED_ORIGINS` 为空时，`len(allowedOrigins) == 0` 直接放行任意 `Origin`。
  - 当包含 `*` 时，同样放行任意 `Origin`，且同时设置 `Access-Control-Allow-Credentials: true`。
- 风险：跨站请求可携带凭证，存在敏感接口被跨站调用的面风险（取决于鉴权与 Cookie/Token 存储策略）。
- 建议：
  1. 生产环境禁止空白 `ALLOWED_ORIGINS` 与 `*`。
  2. 若开启 `Allow-Credentials=true`，必须使用显式白名单精确匹配。
  3. 启动时增加配置校验，非法配置直接 fail-fast。

### P0-2 退款关键链路在无 Redis 场景退化为“内存 goroutine + 无可靠重试”
- 证据：`locallife/api/risk_management.go:476`
- 现象：索赔赔付在 `taskDistributor` 不可用时，使用 goroutine + `context.Background()` 直接执行数据库事务。
- 风险：进程重启、瞬时故障、数据库抖动会导致赔付任务丢失；属于资金正确性风险。
- 建议：
  1. 退款链路禁止无队列退化；无 Redis 时应明确降级为“拒绝自动赔付并告警”。
  2. 或引入本地持久化 outbox（数据库表）+ worker 扫描，确保至少一次执行。
  3. 对失败补偿建立审计与重放机制。

### P1-1 发货上报微信链路异步 goroutine 缺乏可靠投递与超时控制
- 证据：`locallife/api/delivery.go:53`
- 现象：`uploadShippingInfoAsync` 使用 goroutine + `context.Background()`，涉及 DB 查询与外部 HTTP 调用。
- 风险：
  - 请求退出后任务与服务生命周期脱钩；
  - 无统一重试、无死信、无幂等保护策略，外部接口失败后易遗漏。
- 建议：迁移到 asynq 任务队列（与退款一致），设置指数退避、最大重试、幂等键与可观测性。

### P1-2 审计日志写入采用“每次请求开 goroutine”，缺乏背压
- 证据：`locallife/api/audit_writer.go:85`
- 现象：每条审计直接起 goroutine，虽然有 1.5s timeout，但无队列长度和并发上限。
- 风险：高并发下可能形成 goroutine 风暴和数据库突刺，影响主业务稳定性。
- 建议：改为有界队列 + 固定 worker 池；队列满时进行降级策略（采样/丢弃并计数告警）。

### P1-3 定时任务统一缺少“防重入”与“单次执行超时”
- 证据：
  - `locallife/scheduler/order_timeout.go:27`
  - `locallife/scheduler/data_cleanup.go:28`
  - `locallife/scheduler/takeout_auto_complete.go:30`
  - 多处 `context.Background()`：如 `locallife/scheduler/order_timeout.go:53`
- 现象：cron 使用默认行为，任务执行时间超过调度周期时会并发重入；且多数任务无 deadline。
- 风险：重复取消、重复扫描、锁竞争、数据库压力放大，极端情况下引发级联抖动。
- 建议：
  1. 所有 scheduler 使用 `cron.WithChain(cron.SkipIfStillRunning(...), cron.Recover(...))`。
  2. 每个 job 内使用 `context.WithTimeout`（例如 30s/60s/2m，按任务类型配置）。
  3. 为关键任务增加分布式互斥（如 Redis 锁）防多实例并发执行。

### P1-4 运营端实时统计接口存在参数解析宽松与错误吞没
- 证据：
  - `locallife/api/operator_realtime.go:35`（`fmt.Sscanf`）
  - `locallife/api/operator_realtime.go:64`
  - `locallife/api/operator_realtime.go:73`
  - `locallife/api/operator_realtime.go:82`
  - `locallife/api/operator_realtime.go:91`
- 现象：
  - `fmt.Sscanf("%d")` 对前缀数字容忍，`123abc` 可能被解析为 123。
  - 并发查询仅回填计数，不上报错误，接口可能“静默返回不可信 0 值”。
- 风险：权限边界与数据可信性受损，问题排查困难。
- 建议：
  1. 改为 `strconv.ParseInt` 严格解析并校验完整输入。
  2. 使用 `errgroup.WithContext` + 超时控制；出现部分失败时返回明确错误或降级标识。

### P1-5 业务错误通过字符串文本比较分支，稳定性差
- 证据：`locallife/api/delivery.go:209`、`locallife/api/delivery.go:455`
- 现象：基于中文错误文本“您尚未分配服务区域，请联系管理员”做业务分支。
- 风险：文案改动、i18n、包装错误链都会导致逻辑失效。
- 建议：统一使用 typed error 或稳定错误码（例如 `ErrRiderRegionUnassigned`）。

### P2-1 搜索关键词记录链路异步化但无超时、无结果观测
- 证据：`locallife/api/search.go:1495`
- 现象：goroutine 中直接 `context.Background()`，两次 DB 写入均忽略错误。
- 风险：写入失败不可见、数据统计偏差；极端阻塞可能积压 goroutine。
- 建议：使用短超时上下文 + 错误日志 + 指标计数；中长期迁移到消息队列/批处理。

### P2-2 图片删除 worker 以相对路径删除，且队列满时直接丢弃任务
- 证据：`locallife/api/upload_url.go:119`、`locallife/api/upload_url.go:130`
- 现象：
  - `os.Remove(path)` 依赖进程当前工作目录；
  - 队列满时直接丢弃删除任务。
- 风险：
  - 目录变更时可能删不到目标文件，产生垃圾文件；
  - 长期积累导致存储膨胀。
- 建议：
  1. 使用配置化绝对根目录拼接校验后删除。
  2. 队列满时落盘补偿（outbox）或延迟重试，而不是直接丢弃。

### P2-3 数据库连接池参数硬编码，缺少环境弹性
- 证据：`locallife/main.go:89`
- 现象：`MaxConns/MinConns` 等在代码中写死。
- 风险：不同环境（开发/压测/生产）无法基于容量弹性调整，易出现连接瓶颈或资源浪费。
- 建议：将连接池参数提升为配置项，并在启动日志输出最终生效值。

---

## 4. 分阶段改进计划

### 第一阶段（1-3 天，止血）
1. 修复 CORS fail-open（P0-1）。
2. 关闭退款链路的无队列退化路径（P0-2）。
3. 为 scheduler 引入防重入与超时（P1-3，先覆盖订单与资金相关任务）。

### 第二阶段（3-7 天，稳定）
1. 发货上报迁移到任务队列（P1-1）。
2. 审计日志改为有界 worker 池（P1-2）。
3. 运营实时统计接口参数与并发错误治理（P1-4）。
4. 去除字符串错误比较，改 typed error（P1-5）。

### 第三阶段（1-2 周，工程化）
1. 搜索统计、图片删除链路可靠化（P2-1、P2-2）。
2. 连接池与关键超时全面配置化（P2-3）。
3. 建立 SLO 监控：任务成功率、重试次数、队列堆积、goroutine 数量、DB 慢查询。

---

## 5. 验收标准（建议）
- 安全：生产环境 CORS 不允许空白白名单与 `* + credentials` 组合。
- 资金：赔付/退款相关任务具备持久化、重试、审计追踪能力。
- 调度：关键 cron 任务无重入执行，且每次执行具备超时边界。
- 可观测：异步链路均有错误日志和指标，失败可告警、可追踪、可补偿。
- 可维护：错误分支基于稳定错误码/类型，不依赖文本。
