# OCR 回滚步骤与数据边界说明

## 1. 目的

本文对应执行计划 T51，用于明确统一 OCR 主链路发布后的回滚优先级、触发条件，以及哪些数据边界不允许简单回滚。

本文不等于“必须回滚”，而是用于避免在故障窗口里做出更糟糕的回退动作。

## 2. 回滚原则

本轮改造属于 OCR 主链路净化，不建议回滚到“旧壳层接口 + 新 `ocr_jobs` 语义 + 半清理 worker”并存的混乱状态。

回滚优先级固定为：

1. 配置回滚
2. 后端版本回滚
3. worker 版本回滚
4. 权限或文档层回退
5. 数据库 schema 回滚仅在极端条件下评估

## 3. 可优先回滚的层

优先回滚：

1. OCR provider 配置开关
2. 后端 API 版本
3. worker 版本
4. Swagger / 文档发布版本

原因：这些层的回滚不直接破坏已经写入的 `ocr_jobs`、业务 OCR JSON 和对象存储资产。

## 4. 尽量不回滚的层

尽量不回滚：

1. `ocr_jobs` 相关 schema
2. 已写入业务表的 `ocr_job_id` 与 OCR JSON 状态字段
3. 已经生成的 `raw_result` / `normalized_result`
4. 已经创建并绑定的 `media_asset_id`

原因：这些数据已经成为统一 OCR 主链路状态机的一部分，盲目 down migration 或删除字段会直接破坏状态可追踪性。

## 5. 数据库回滚边界

除非同时满足以下条件，否则不执行数据库 schema 回滚：

1. 刚上线且尚未产生新的 OCR job 业务数据。
2. 已确认仅靠配置或代码回滚无法恢复服务。
3. 已评估 down migration 不会破坏已写入的 OCR 结果与业务关联。

数据库问题的优先处理方式应为：

1. 热修代码兼容当前 schema。
2. 增补 forward migration，而不是直接 down。
3. 通过 worker、重试或人工介入让已有 job 重新收敛。

## 6. 回滚触发条件

满足任一条件即可进入回滚评估：

1. `/v1/ocr/jobs*` 创建 job 失败率持续上升。
2. worker 无法消费新 job 或大量 job 长时间卡在 `queued` / `processing`。
3. private 证件读取大量失败，且确认不是单点 provider 限流。
4. 统一 OCR API 可用但业务表 OCR JSON 持续无法回写。
5. 平台告警持续触发，但无法定位到具体 `ocr_job_id`、owner 或 document。

## 7. 回滚后必须复查的点

1. 现存 `ocr_jobs` 是否仍可被当前版本识别和查询。
2. 业务表中的 `ocr_job_id`、`error_code`、`alert_emitted_at` 是否仍保持可解释状态。
3. 对象存储中的证件资产读路径是否仍可由服务端访问。
4. 平台告警和 Prometheus 指标是否恢复到可观测状态。

## 8. 关联文档

- `.github/standards/domains/ocr/OCR_REFACTOR_EXECUTION_PLAN_2026-03-25.md`
- `.github/standards/domains/ocr/OCR_RELEASE_RUNBOOK_2026-03-25.md`
- `.github/standards/domains/ocr/OCR_OPERATIONS_RUNBOOK_2026-03-25.md`