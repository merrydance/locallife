# OCR 主链路运行手册

## 1. 适用范围

本文覆盖统一 OCR 主链路的日常巡检、失败定位与人工介入流程，适用对象包括：

- 商户入驻 OCR
- 运营商入驻 OCR
- 骑手入驻 OCR
- 集团入驻 OCR

当前统一 OCR 主链路的关键组成如下：

- 统一任务表 `ocr_jobs`
- 统一创建/查询接口 `/v1/ocr/jobs*`
- 统一执行服务 `ocr.Service`
- 服务端媒体读取 `ReadMediaAsset`
- Provider 路由：阿里云优先，微信作为兼容 provider
- 异步 worker 回写业务 OCR JSON

本文重点回答三个运维问题：

1. OCR 失败后系统会不会自动重试
2. 哪些失败会自动停下并发出平台告警
3. 发生问题后应该先看哪里、怎么人工处理

## 2. 自动恢复与告警矩阵

### 2.1 自动重试规则

当前 OCR 任务默认启用最小自动重试闭环：

- asynq 侧默认 `MaxRetry=2`
- `ocr_jobs.max_attempts` 默认值为 `3`
- worker 在领取任务时会允许重拿超过 15 分钟未续租的 `processing` lease，避免进程崩溃后任务永久卡死
- Worker 会根据错误类型区分可重试与不可重试失败

可重试失败示例：

- 阿里云限流
- 阿里云服务不可用
- 网络超时、连接重置、EOF
- 微信 OCR 次数限流或临时不可用

不可重试失败示例：

- 证件图片过大
- 媒体文件不存在
- Provider 鉴权失败或权限不足
- 明确的请求参数错误、图片格式错误

### 2.2 平台告警规则

当前 OCR 失败在以下两类场景会发平台告警：

- `OCR_JOB_FAILED`
  - 不可重试失败
  - 例如图片问题、权限问题、介质缺失
- `OCR_RETRY_EXHAUSTED`
  - 本次失败本身可重试，但任务已经达到 `max_attempts`

告警会通过 Redis Pub/Sub 发布到：

- `notification:platform:alerts`

告警 extra 当前至少包含：

- `ocr_job_id`
- `owner_type`
- `owner_id`
- `document_type`
- `provider`
- `media_asset_id`
- `side`
- `attempt_count`
- `max_attempts`
- `error_code`
- `retryable`
- `reason`

### 2.3 业务 OCR JSON 可见字段

当 OCR 失败时，业务表内对应的 OCR JSON 会补充：

- `status`
- `error`
- `error_code`
- `ocr_job_id`
- `queued_at`
- `started_at`
- `alert_emitted_at`

这些字段的目的不是替代 `ocr_jobs` 主表，而是帮助业务查询页、客服和运营后台快速定位失败上下文。

## 3. 监控点说明

当前 OCR 主链路已接入以下 Prometheus 指标：

- `ocr_jobs_total`
  - labels: `owner_type`, `document_type`, `provider`, `status`, `error_code`
- `ocr_job_duration_seconds`
  - labels: `owner_type`, `document_type`, `provider`, `status`
- `ocr_alerts_total`
  - labels: `alert_type`, `level`, `owner_type`, `document_type`
- `ocr_retry_suppressed_total`
  - labels: `owner_type`, `document_type`, `provider`, `reason`

运维侧重点看三类信号：

1. `status=failed` 是否持续上升
2. `ocr_provider_unavailable` / `ocr_rate_limited` 是否成批出现
3. `ocr_retry_suppressed_total{reason="attempts_exhausted"}` 是否出现明显堆积

## 4. 日常巡检顺序

建议每天按以下顺序巡检：

### 4.1 先看平台告警

优先关注：

- `OCR_JOB_FAILED`
- `OCR_RETRY_EXHAUSTED`

若同一 `document_type` 在短时间内连续出现，优先判断是：

- 单个用户上传问题
- 某类证件整体识别质量下降
- Provider 配置、权限或限流问题

### 4.2 再看 Prometheus 指标

重点观察：

1. `ocr_jobs_total` 中 `status=failed` 的增长斜率
2. `ocr_job_duration_seconds` 的 P95/P99 是否显著抬升
3. `ocr_retry_suppressed_total` 是否集中出现在某个 provider 或 document type
4. `ocr_alerts_total` 是否在某个 owner type 上突然增长

### 4.3 最后到业务与任务明细定位

需要定位单条问题时，顺序建议为：

1. 通过告警 extra 获取 `ocr_job_id`
2. 查询 `ocr_jobs` 当前状态、错误码、尝试次数、provider
3. 查看对应业务表里的 OCR JSON
4. 检查关联 `media_asset_id` 是否仍可读取

## 5. 典型异常处理步骤

### 5.1 `ocr_provider_unavailable`

说明：Provider 服务临时不可用或网络超时。

处理顺序：

1. 先确认是否只是短时抖动。
2. 查看 `ocr_rate_limited` 是否同时升高，避免把限流误判为服务异常。
3. 若同时间大量出现，检查阿里云 endpoint、网络出站、DNS 与证书状态。
4. 若任务已进入 `OCR_RETRY_EXHAUSTED`，按 `ocr_job_id` 抽样人工重试。

### 5.2 `ocr_rate_limited`

说明：Provider 达到限流阈值。

处理顺序：

1. 观察 `ocr_jobs_total{error_code="ocr_rate_limited"}` 是否只在单一 `document_type` 上升。
2. 判断是否为突发批量导入或异常重试风暴。
3. 若为持续限流，优先降并发、错峰或申请更高配额。
4. 对已经重试耗尽的任务，人工按批次重新创建 OCR job。

### 5.3 `ocr_provider_unauthorized` / `ocr_provider_forbidden`

说明：Provider 凭证或 RAM 权限配置异常。

处理顺序：

1. 先确认环境变量与当前部署实例加载的凭证是否一致。
2. 检查 STS/RAM 角色策略是否包含 OCR 所需最小权限。
3. 这类错误通常不会自动恢复，必须优先修配置。
4. 配置恢复后，再按 `ocr_job_id` 或 owner 维度人工补发任务。

补充约束：

- 当前版本的生产推荐方案是“专用 RAM 用户最小权限 AK/SK”。
- `ALIYUN_OCR_STS_ENABLED` 当前仅完成配置位与启动校验，运行时尚未实现，不应在生产开启。
- 详细约束见 `docs/OCR_ALIYUN_RAM_STS_MIN_PERMISSION_2026-03-25.md`。

### 5.4 `ocr_bad_request`

说明：图片或请求内容本身不满足 OCR 条件。

常见原因：

- 图片过大
- 图片格式异常
- 证件内容缺失或拍摄质量极差

处理顺序：

1. 先在业务 OCR JSON 中确认 `error_code` 与 `alert_emitted_at`。
2. 再确认关联 `media_asset_id` 对应的原图是否可访问。
3. 这类问题通常不建议后台直接重试，应让用户重新上传。

### 5.5 `ocr_media_not_found`

说明：OCR 任务执行时无法读取关联媒体。

处理顺序：

1. 检查 `media_asset_id` 是否已被误删或 object key 已失效。
2. 若是存储配置问题，先修复 bucket / object 访问能力。
3. 若媒体确实不存在，按业务侧提示重新上传，不做盲重试。

## 6. 人工处理原则

发生 OCR 异常时，遵守以下原则：

1. 不直接伪造业务 OCR JSON 为成功状态。
2. 不直接跳过 OCR，除非有人工审核兜底且有明确审批记录。
3. 优先修复 provider、媒体或权限问题，再重建 OCR job。
4. 若属于用户上传质量问题，应要求重新上传，而不是后台反复重试。

## 7. 人工重试建议步骤

当需要人工处理单条失败任务时，建议按以下顺序执行：

1. 先通过 `GET /v1/ocr/jobs/dead-letter` 拉取待人工介入任务列表，再用 `ocr_job_id` 确认失败原因、错误码和尝试次数。
2. 确认 `media_asset_id` 对应文件存在且内容正确。
3. 确认 provider 配置已恢复。
4. 通过统一 OCR 接口重新创建任务，避免直接改旧任务状态。
5. 回查业务表 OCR JSON 是否已更新为 `done`。

## 8. 发布后观察点

OCR 主链路上线或大改后，至少观察以下窗口：

### 8.1 首小时

- `ocr_jobs_total{status="failed"}` 是否异常抬升
- `ocr_alerts_total` 是否持续增长
- `ocr_job_duration_seconds` 是否明显高于基线

### 8.2 首日

- 各 owner type 是否都能正常回写 OCR JSON
- 是否出现集中在某个 document type 的失败
- `OCR_RETRY_EXHAUSTED` 是否持续出现

### 8.3 首周

- Provider 限流是否成为常态
- 是否需要调整重试策略或配额
- 是否存在特定证件类型识别质量长期偏低

## 9. 联调与验收建议

测试环境至少覆盖以下场景：

1. 正常创建 OCR job 并成功回写
2. Provider 临时失败后自动重试
3. 不可重试错误直接停机并产出平台告警
4. `error_code` 与 `alert_emitted_at` 能正确回写到业务 OCR JSON
5. `/metrics` 可见 OCR 相关指标

验收通过标准：

1. 失败场景能区分自动恢复与人工介入边界
2. 平台告警携带足够排障上下文
3. 指标足以观察失败率、耗时和重试耗尽情况