# OCR 测试环境端到端联调清单

## 1. 目的

本文对应执行计划 T49，用于在测试环境或预发环境执行统一 OCR 主链路的一次完整闭环联调。

本文不替代运维 Runbook，也不替代切换清单：

- 切换前准入条件看 `./OCR_UNIFIED_API_CUTOVER_CHECKLIST_2026-03-25.md`
- 失败排障与告警处理看 `../OCR_OPERATIONS_RUNBOOK_2026-03-25.md`

## 2. 联调目标

本轮联调需要验证以下能力已经串成闭环：

- 统一 `/v1/ocr/jobs*` 接口族可创建和查询任务
- worker 能按 `ocr_job_id` 执行 OCR
- worker 能通过 `ReadMediaAsset` 读取字节流，不依赖本地路径
- provider 路由、错误分类、重试与告警生效
- 业务 OCR JSON 与 `ocr_jobs` 状态保持一致

## 3. 联调前检查

开始前至少确认：

1. 测试环境数据库已包含 `ocr_jobs` 相关 migration，且 migration 状态无 dirty 标记。
2. Redis、对象存储、数据库连接均可用。
3. 后端部署版本包含统一 OCR API、最新 worker 和 OCR 观测埋点。
4. `ALIYUN_OCR_ENABLED`、OCR 凭证、对象存储配置与 Redis 配置已加载到测试环境。
5. `/health`、`/ready` 正常，worker 启动日志没有 provider 初始化失败或 Redis 退化日志。

## 4. 推荐联调顺序

### 4.1 基础接口验证

1. 调用 `POST /v1/ocr/jobs` 创建一条 job。
2. 记录返回的 `ocr_job_id`、`owner_type`、`owner_id`、`document_type`、`media_asset_id`。
3. 调用 `GET /v1/ocr/jobs/:id`，确认初始状态至少包含 `queued` 或等价待执行状态。
4. 调用 `POST /v1/ocr/jobs/batch-query`，确认批量查询结果与单查一致。

### 4.2 成功闭环样本

四条业务线至少各跑一条成功样本：

1. 商户：food permit 或身份证样本
2. 运营商：营业执照或身份证样本
3. 骑手：身份证或健康证样本
4. 集团：营业执照样本

每条样本都要确认：

- `ocr_jobs` 最终进入 `succeeded` 或等价成功状态
- `raw_result` 与 `normalized_result` 可查询
- 对应业务表 OCR JSON 已回写 `ocr_job_id`
- 业务字段投影正确，例如证件号、姓名、有效期、许可证号

### 4.3 可重试失败样本

至少制造一条临时失败样本，例如：

- provider 限流
- 短暂网络失败
- 人工模拟上游临时不可用

验证点：

1. job 首次失败后不会直接终态失败。
2. asynq 与 `ocr_jobs.max_attempts` 协同工作，后续会自动重试。
3. 指标中可观察到失败、重试或重试抑制变化。

### 4.4 不可重试失败样本

至少制造一条不可重试失败样本，例如：

- 无效图片格式
- 介质不存在
- 权限/鉴权失败

验证点：

1. job 不继续自动重试。
2. 业务 OCR JSON 回写 `error_code` 与 `alert_emitted_at`。
3. 平台告警可看到 `OCR_JOB_FAILED`。

### 4.5 重试耗尽样本

至少验证一条可重试但最终耗尽样本。

验证点：

1. job 达到 `max_attempts` 后停止自动重试。
2. 平台告警出现 `OCR_RETRY_EXHAUSTED`。
3. `ocr_retry_suppressed_total{reason="attempts_exhausted"}` 有对应增量。

## 5. 联调过程中的观测点

联调过程中至少观察以下信号：

1. `ocr_jobs_total`
2. `ocr_job_duration_seconds`
3. `ocr_alerts_total`
4. `ocr_retry_suppressed_total`
5. worker 日志中的 `ocr_job_id`、`owner_type`、`document_type`、`provider`、`error_code`

## 6. 联调通过标准

满足以下条件，T49 才算可验收：

1. 四条业务线至少各有一条成功闭环样本。
2. 已验证一次可重试失败自动恢复。
3. 已验证一次不可重试失败直接停机并发告警。
4. 已验证一次重试耗尽告警。
5. `error_code`、`alert_emitted_at`、`ocr_job_id` 能在业务 OCR JSON 中查到。
6. 不需要依赖旧 multipart 主入口或旧壳层接口就能完成联调。

## 7. 联调失败时的最小排查顺序

1. 先查 `ocr_jobs` 当前状态和 attempt 计数。
2. 再查 worker 日志中相同 `ocr_job_id` 的执行记录。
3. 再查 `ocr_alerts_total` 与 Redis 平台告警消息。
4. 最后确认 media asset、provider 配置和对象存储读取是否正常。

## 8. 关联文档

- `.github/standards/domains/ocr/historical/OCR_REFACTOR_EXECUTION_PLAN_2026-03-25.md`
- `.github/standards/domains/ocr/historical/OCR_UNIFIED_API_CUTOVER_CHECKLIST_2026-03-25.md`
- `.github/standards/domains/ocr/OCR_OPERATIONS_RUNBOOK_2026-03-25.md`