# Engineering Governance Standards

本目录是 LocalLife 跨产品面的工程治理权威入口，覆盖安全性、一致性、鲁棒性、验证、发布与事故闭环。

适用范围：

- `locallife/`
- `web/`
- `weapp/`
- 与上述代码路径直接相关的 `.github/` 规则、Prompt、Workflow 与交付流程

## 目录角色

- `ENGINEERING_GOVERNANCE_BASELINE.md`: 跨层治理母标准，定义长期有效的共同基线。
- `VALIDATION_AND_RELEASE_MATRIX.md`: 跨层验证矩阵与发布就绪口径。
- `AI_PROMPT_GOVERNANCE.md`: AI-facing Prompt、Instruction、Agent 与治理门禁的分层和维护规则。
- `UNREACHABLE_DEPENDENCY_RISK_REGISTER.md`: 不可达依赖风险台账，用于收口 `required module not called` 一类非可触达依赖发现。
- `INCIDENT_FEEDBACK_LOOP.md`: 事故、重大缺陷、near miss 与高价值 review finding 的规则回灌机制。

## Historical Material

- `historical/GOVERNANCE_ROADMAP.md`: 已完成的治理落地路线图，保留为历史参考，不再属于默认热路径。

## 使用方式

当任务涉及以下任一目标时，先读本目录，再进入 area-specific 标准：

- 高安全要求
- 生产级鲁棒性
- 跨 backend/web/weapp 的一致性治理
- 测试矩阵、发布就绪、事故复盘与规则回灌
- 需要判断某项要求应落在 standards、instructions、prompts 还是 workflows

## 推荐阅读顺序

1. `ENGINEERING_GOVERNANCE_BASELINE.md`
2. `VALIDATION_AND_RELEASE_MATRIX.md`
3. `AI_PROMPT_GOVERNANCE.md`（当任务涉及 `.github/` Prompt、Instruction、Agent 或治理门禁时）
4. `UNREACHABLE_DEPENDENCY_RISK_REGISTER.md`
5. `INCIDENT_FEEDBACK_LOOP.md`
6. 对应 area-specific 标准：backend、web、weapp 或 domain docs

仅在需要追溯治理建设顺序或阶段验收历史时，再查看 `historical/GOVERNANCE_ROADMAP.md`。

## 文档边界

- 本目录只放长期有效的跨层治理标准。
- area-specific 实现细节继续放在 `.github/standards/backend/`、`.github/standards/web/`、`.github/standards/weapp/` 与 domain 目录。
- 高频执行约束应镜像到 `.github/instructions/`。
- 高频任务入口应镜像到 `.github/prompts/`。
- 能自动化的门禁应优先落到 `.github/workflows/` 或脚本校验中。
- 已完成的治理路线图、一次性 rollout 计划和阶段性落地追踪，应移入 `historical/` 而不是留在默认热路径。

## 维护规则

- 当治理母标准变化影响日常实现、审查、验证或发布行为时，必须同步评估 `.github/instructions/`、`.github/prompts/`、`.github/workflows/` 和 `.github/NORMS_AUDIT.md` 是否需要更新。
- 当路线图中的某个阶段完成并不再是当前 operating baseline 时，将路线图转入历史材料，而不是继续放在默认热路径。