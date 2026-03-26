# 统一 OCR 接口族一次性切换清单

## 1. 目的

本文对应执行计划 T48，用于在统一 OCR 接口族正式切换前，逐项确认代码、配置、客户端、文档和运维面已经满足一次性切换条件。

本清单默认前提：

- 当前服务尚未正式上线
- 不保留旧 OCR multipart 主入口与旧壳层接口的长期兼容
- 统一主入口固定为 `/v1/ocr/jobs*`

## 2. 切换范围

本次一次性切换覆盖：

- 商户入驻 OCR
- 运营商入驻 OCR
- 骑手入驻 OCR
- 集团入驻 OCR

切换后的唯一对外 OCR 接口族：

- `POST /v1/ocr/jobs`
- `GET /v1/ocr/jobs/:id`
- `GET /v1/ocr/jobs/:id/result`
- `POST /v1/ocr/jobs/:id/retry`
- `POST /v1/ocr/jobs/batch-query`

## 3. 切换前硬性前置条件

以下任一项未满足时，不进入切换执行：

- `ocr_jobs` 表、SQLC 查询、统一 `ocr.Service` 已稳定可用
- `ReadMediaAsset` 已覆盖 local / OSS 服务端字节流读取
- 核心证件链路都已按 `media_asset_id` / `ocr_job_id` 主链运行
- OCR worker 已具备失败分类、自动重试、失败停机与告警能力
- Prometheus 指标和平台告警已接入并可观测
- 业务 handler 中不存在直接调用 provider SDK 的路径
- private 证件不通过 public URL 供 OCR 使用
- 旧 OCR handler、旧 multipart 主入口、旧 worker payload 已从运行时代码删除

## 4. 切换前检查项

### 4.1 代码与路由

- [x] `api/server.go` 中只保留 `/v1/ocr/jobs*` 统一接口族
- [x] `api/merchant_application.go`、`api/operator_application.go`、`api/rider_application.go`、`api/group.go` 中不再存在旧 `upload*OCR` handler
- [x] OCR worker payload 仅保留 `ocr_job_id` / `media_asset_id` 等最小标识，不再依赖 `image_path`
- [x] 代码搜索确认 `mediaAssetLocalPath` 不再被 OCR 主链引用

### 4.2 权限与文档产物

- [x] `casbin/policy.csv` 中不再保留 merchant/operator/rider/group 旧 OCR 路径授权
- [x] `docs/swagger.json`、`docs/swagger.yaml`、`docs/docs.go` 已重新生成，不再暴露旧 OCR 路径
- [ ] 前端或小程序侧不再请求旧 OCR 路径

### 4.3 数据与任务系统

- [ ] `ocr_jobs` 的幂等键、最大重试次数和状态字段已与当前实现一致
- [ ] asynq worker 已部署新版本，消费逻辑与 `ocr_jobs` 状态机匹配
- [ ] Redis、数据库和对象存储连接在预发环境完成过闭环验证

### 4.4 观测与告警

- [ ] `ocr_jobs_total`、`ocr_job_duration_seconds`、`ocr_alerts_total`、`ocr_retry_suppressed_total` 可被正常抓取
- [ ] `OCR_JOB_FAILED` 与 `OCR_RETRY_EXHAUSTED` 告警在预发环境验证过
- [ ] 值班人知道告警查看入口和人工排障路径

## 5. 一次性切换执行顺序

1. 冻结 OCR 相关 handler / worker / swagger 变更，避免切换窗口内继续漂移。
2. 合并并部署包含统一 OCR 路由、worker 和观测能力的版本。
3. 重新生成并发布 Swagger 文档，确认外部文档只暴露 `/v1/ocr/jobs*`。
4. 更新权限配置，移除旧 OCR 路径授权。
5. 对商户、运营商、骑手、集团四条链路各执行至少一条预发闭环样本。
6. 检查 `ocr_jobs` 状态流转、业务表 OCR JSON 回写、Prometheus 指标和 Redis 平台告警。
7. 通过后才允许前端或运营侧开始只依赖统一 OCR 接口族。

## 6. 切换后最小冒烟清单

- [ ] 商户 food permit 创建 job、执行成功、结果回写成功
- [ ] 商户身份证 front/back 创建 job、执行成功、结果回写成功
- [ ] 运营商身份证或营业执照链路闭环成功
- [ ] 骑手身份证或健康证链路闭环成功
- [ ] 集团营业执照链路闭环成功
- [ ] 失败样本至少验证一次不可重试错误和一次重试耗尽告警

## 7. 立即停止切换的条件

出现以下任一情况，立即停止继续放量，并回到执行计划 T49-T52 的测试/发布/回滚文档处理：

- 统一 OCR 接口创建 job 失败率明显上升
- `ocr_jobs` 出现大面积卡在 `queued` 或 `processing`
- private 证件读取出现 public URL 回退
- 业务表 OCR JSON 未回写或回写错 owner/document/side
- 平台告警已触发但无法关联到具体 job / owner / document

## 8. 关联文档

- `.github/standards/domains/ocr/OCR_REFACTOR_EXECUTION_PLAN_2026-03-25.md`
- `docs/OCR_OPERATIONS_RUNBOOK_2026-03-25.md`

本清单只回答“能不能切、切之前检查什么”。

以下内容仍由后续任务补齐：

- T49：测试环境端到端联调清单
- T50：生产发布步骤
- T51：回滚步骤与不允许回滚的数据边界
- T52：上线后验收 checklist