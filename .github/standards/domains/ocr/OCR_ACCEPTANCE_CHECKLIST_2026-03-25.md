# OCR 上线后验收 Checklist

## 1. 目的

本文对应执行计划 T52，用于在统一 OCR 主链路上线后的首轮观察窗口内，固定验收动作和通过标准。

## 2. 验收时间窗

建议按三个时间窗执行：

1. 上线后首小时
2. 上线后首日
3. 上线后首周

## 3. 首小时验收

### 3.1 接口与任务

- [ ] `POST /v1/ocr/jobs` 能正常创建任务
- [ ] `GET /v1/ocr/jobs/:id` 能查询到新任务状态
- [ ] 新建 job 能被 worker 消费，不持续卡在 `queued`

### 3.2 业务回写

- [ ] 至少一条商户 OCR 结果回写成功
- [ ] 至少一条骑手或运营商 OCR 结果回写成功
- [ ] OCR JSON 中可见 `ocr_job_id`

### 3.3 观测与告警

- [ ] `ocr_jobs_total` 有正常增量
- [ ] `ocr_job_duration_seconds` 可观察到样本
- [ ] 平台告警链路可用，没有无上下文的异常告警

## 4. 首日验收

- [ ] 四条业务线至少各完成一条成功闭环
- [ ] 不可重试失败能直接停机并发出 `OCR_JOB_FAILED`
- [ ] 可重试失败在达到阈值后发出 `OCR_RETRY_EXHAUSTED`
- [ ] 失败样本的 `error_code` 与 `alert_emitted_at` 能在业务 OCR JSON 中查到
- [ ] private 证件读取未出现 public URL 回退

## 5. 首周验收

- [ ] `status=failed` 没有持续异常抬升
- [ ] 没有单一 document type 长期失败堆积
- [ ] provider 限流没有成为常态瓶颈
- [ ] 值班同学可按 Runbook 独立完成一次 OCR 故障排查

## 6. 通过标准

满足以下条件，可认定 T52 验收通过：

1. 统一 OCR API、worker、业务回写和观测链路都已在生产环境形成闭环。
2. 失败场景能区分自动恢复与人工介入边界。
3. 平台告警和指标足以支持定位 owner、document、job 和失败原因。
4. 不再需要依赖旧 OCR 壳层接口或旧 multipart 主入口完成业务流转。

## 7. 关联文档

- `.github/standards/domains/ocr/OCR_REFACTOR_EXECUTION_PLAN_2026-03-25.md`
- `docs/OCR_RELEASE_RUNBOOK_2026-03-25.md`
- `docs/OCR_OPERATIONS_RUNBOOK_2026-03-25.md`