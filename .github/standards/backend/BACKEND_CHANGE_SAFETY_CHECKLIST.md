# Backend Change Safety Checklist

> 作用：把后端改动在交付前必须再确认的高频安全项，变成固定收口步骤，减少“看起来改完了，实际上链路没闭合”的情况。

在关闭 backend 实现、修复或重构任务前，至少过一遍本清单。

## 1. Path Coverage

- 我是否读过真实入口和真实写边界，而不是只看局部函数？
- 如果链路包含 callback、worker、scheduler、timeout、recovery，我是否把这些路径一起看过？
- 如果变更涉及资金、状态机、排他性或幂等，我是否检查了 `db/sqlc/` 事务代码？

## 2. Invariants And Trust Boundaries

- 对象归属、权限和角色边界是否由服务端可信上下文验证，而不是依赖客户端字段？
- 关键唯一性、互斥性或状态前置条件是否落在事务/数据库边界，而不是只在 handler 或普通 logic 前置检查？
- 这次改动会不会造成“本地记录成功、外部副作用失败”或相反方向的半成功状态？
- 如果改动涉及 `Idempotency-Key`、`idempotency_key`、`out_*_no`、callback notification id、worker/scheduler 重复执行或自然去重查询，是否已按 `.github/standards/backend/IDEMPOTENCY_STANDARDS.md` 分类，并说明为什么接入或不接入 request-level guard？

## 3. Cross-Layer Completeness

- 新增或修改的请求字段，是否真的贯穿到了 logic、store、response、Swagger 和测试？
- 新增或修改的 SQL、事务、store 接口、worker 入口、scheduler 入口，是否存在真实调用方？
- 是否留下了看似已计算、实则没有持久化、没有返回、也没有影响行为的死逻辑？

## 4. Generated Outputs

- 如果改了 `db/query/`、事务签名、store 接口或相关生成依赖，是否需要 `make sqlc` / `make mock`？
- 如果改了路由、注释或 API 契约，是否需要 `make swagger`？
- 对 SQL 或 Swagger 源文件改动，是否需要 `make check-generated` 来确认生成物没有漂移？

## 5. Validation

- 我是否补了最接近风险边界的回归测试，而不是只测一个遥远的 happy path？
- 我是否运行了最小相关命令，例如包级测试、`make test-unit`、`make test-safety` 或 `make test-integration`？
- 如果跳过了测试或生成步骤，我是否在交付里明确说明，而不是默认装作已完成？

## 6. Handoff

交付说明至少要写清楚：

1. 风险等级及依据
2. 改到了哪些层
3. 运行了哪些命令
4. 哪些关键分支未验证
5. 残余风险落在什么具体 callback / retry / recovery / authz / state-transition 路径

## 7. Workspace Safety

- 我是否避免覆盖或回滚工作区中与当前任务无关的用户改动？
- 我是否把改动控制在 backend 范围内，而不是顺手扩成无关重构？

