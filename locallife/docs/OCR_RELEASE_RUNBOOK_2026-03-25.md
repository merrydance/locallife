# OCR 生产发布步骤 Runbook

## 1. 目的

本文对应执行计划 T50，用于在统一 OCR 主链路正式上线时，按固定顺序完成发布，避免出现“路由已切换但 worker 未就绪”或“文档已切换但权限仍保留旧路径”等半切换状态。

本文只回答生产发布顺序和发布窗口内的检查动作。

以下内容仍由后续任务补齐：

- T51：回滚步骤与不允许回滚的数据边界
- T52：上线后验收 checklist

## 2. 适用前提

执行本 Runbook 前，必须满足：

1. `docs/OCR_UNIFIED_API_CUTOVER_CHECKLIST_2026-03-25.md` 中的切换前硬性前置条件已满足。
2. `docs/OCR_TEST_ENV_E2E_CHECKLIST_2026-03-25.md` 已在测试环境完成至少一轮闭环联调。
3. 当前待发布版本已包含统一 OCR API、worker、观测、告警和最新文档。
4. 发布窗口内不再混入新的 OCR handler 或 worker 结构性改动。

## 3. 角色分工建议

建议至少明确以下责任人：

1. 发布负责人
2. 后端负责人
3. worker/任务系统负责人
4. 基础设施负责人
5. 验收负责人

每一步必须有明确拍板人，不接受“大家一起看”的模糊责任。

## 4. 发布前检查

### 4.1 配置检查

必须确认生产环境已具备：

1. `ALIYUN_OCR_ENABLED`
2. Aliyun OCR endpoint、region、凭证或 STS 配置
3. Redis 地址与认证配置
4. 数据库连接配置
5. 对象存储 public/private bucket 与读写权限配置

### 4.2 制品检查

必须确认：

1. 后端制品包含统一 `/v1/ocr/jobs*` 接口族。
2. worker 制品包含统一 `ocr.Service` 执行逻辑与最新重试/告警逻辑。
3. Swagger 产物与对外文档已更新到统一 OCR 接口族版本。
4. 权限配置准备好移除旧 OCR 路径授权。

### 4.3 数据与运行时检查

必须确认：

1. `ocr_jobs` 相关 migration 已在目标环境准备就绪。
2. Redis 队列、数据库和对象存储连接都已在目标环境冒烟验证。
3. Prometheus 可以抓取 OCR 指标。
4. 平台告警订阅链路可用。

## 5. 固定发布顺序

推荐发布顺序固定为：

1. 配置预检查
2. 数据库 migration
3. 后端 API 发布
4. worker 发布
5. Swagger 与文档产物发布
6. 权限配置切换
7. 首轮生产冒烟

不得颠倒为：

1. 先切权限或文档，再发后端和 worker
2. 先下旧路由授权，再确认统一 OCR API 可用
3. 只发 API 不发 worker

## 6. 详细发布步骤

### 6.1 配置预检查

发布前先确认：

1. OCR provider 配置完整且与目标环境一致。
2. Redis 不会退化为不可用或 Noop 分发模式。
3. 对象存储 private 读路径可由服务端访问。
4. 生产环境没有继续依赖本地 uploads 路径作为 OCR 主读取方式。

### 6.2 数据库 migration

按现有 Makefile 流程执行：

```bash
make migrateup
```

migration 完成后必须确认：

1. `ocr_jobs` 相关表和字段存在。
2. migration 状态无 dirty 标记。
3. 当前后端和 worker 所需查询与数据库结构一致。

### 6.3 后端 API 发布

API 发布后立刻确认：

1. `/health` 与 `/ready` 正常。
2. `/v1/ocr/jobs*` 路由可访问。
3. 没有 provider 初始化失败、对象存储初始化失败或 Redis 初始化失败日志。

### 6.4 worker 发布

worker 发布后立刻确认：

1. OCR 任务处理器正常注册。
2. worker 能消费新版本 `ocr_job_id` 任务。
3. 没有出现因旧 payload 或旧路径依赖导致的启动或消费错误。

### 6.5 文档与权限切换

确认并执行：

1. 发布最新 Swagger 产物。
2. 清理旧 OCR 路径授权。
3. 对内发布说明中明确统一 OCR 接口族已成为唯一对外接口。

## 7. 首轮生产冒烟

发布完成后，至少执行以下冒烟动作：

1. 创建一条 OCR job，确认能进入待执行状态。
2. 观察 worker 消费并成功回写一条业务 OCR 结果。
3. 检查 `ocr_jobs_total` 与 `ocr_job_duration_seconds` 有正常增量。
4. 人工核对一条业务 OCR JSON 中的 `ocr_job_id`、`status`、结果字段。

## 8. 立即停止继续放量的条件

出现以下任一情况，立即停止继续切换，等待 T51 回滚文档执行：

1. 统一 OCR API 创建 job 失败率持续上升。
2. worker 无法消费新 job 或大量卡在 `queued`。
3. private 证件读取出现异常回退或权限错误。
4. OCR 失败告警触发后无法定位具体 job。
5. 旧路径授权已删除，但统一接口族无法支撑业务流转。

## 9. 关联文档

- `docs/OCR_REFACTOR_EXECUTION_PLAN_2026-03-25.md`
- `docs/OCR_UNIFIED_API_CUTOVER_CHECKLIST_2026-03-25.md`
- `docs/OCR_TEST_ENV_E2E_CHECKLIST_2026-03-25.md`
- `docs/OCR_OPERATIONS_RUNBOOK_2026-03-25.md`