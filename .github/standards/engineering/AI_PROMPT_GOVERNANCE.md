# AI Prompt Governance

本文件定义 LocalLife 仓库内 AI Prompt、Agent、Instruction 与相关门禁的长期治理规则。

它解决四类问题：

- Prompt 资产如何分层，而不是平铺堆积。
- 实现 Prompt 与 Review Prompt 如何复用同一套推进项和禁止项。
- 哪些规则应留在 standards，哪些应镜像到 instructions、prompts、agents 或 workflows。
- Prompt 库本身如何像生产代码一样接受 lint 与 CI 门禁。

## 1. Layer Model

### Layer 0: Standards

位置：`.github/standards/`

职责：

- 保存长期有效的治理规则、技术栈规范、业务域规则与验证口径。
- 作为 implementation、review、instructions、agents 与 workflows 的共同来源。

要求：

- 长期规则先改 standards，再同步镜像层。
- 不要把长期规则只写在 prompt 中。

### Layer 1: Instructions

位置：`.github/instructions/` 与 `copilot-instructions.md`

职责：

- 解决默认行为、路径匹配、Read First 热路径与执行约束。
- 只保留高频执行规则，不复写完整标准正文。

要求：

- 说明当前路径默认应读哪些标准。
- 说明当前路径的强约束与高风险反模式。

### Layer 2: Protocol Prompts

位置：`.github/prompts/general-*.prompt.md`

职责：

- 定义任务协议，而不是技术栈细节。
- 统一实现、审查、事故回灌、任务闭环这类工作流输入与输出结构。

当前协议层模板：

- `general-implementation.prompt.md`
- `general-review.prompt.md`
- `general-incident-followup.prompt.md`
- `general-task-loop.prompt.md`

要求：

- 协议层 Prompt 只定义任务协议、风险分级、验证口径、交付边界。
- 不要在协议层 Prompt 中堆入过多栈内细节或业务域例外。
- `general-implementation.prompt.md` 与 `general-review.prompt.md` 必须保留 `Use only when` 边界，避免吞掉技术栈 Prompt 的路由空间。

### Layer 3: Stack And Domain Prompts

位置：`.github/prompts/<stack-or-domain>-*.prompt.md`

职责：

- 在协议层之上补充技术栈差异或业务域专项规则。
- 只补本栈或本域独有的推进项、禁止项、输入上下文与验收要点。

技术栈层模板：

- Backend: `backend-implementation.prompt.md`, `backend-review-closure.prompt.md`
- Web: `web-implementation.prompt.md`, `web-review.prompt.md`
- Weapp: `weapp-implementation.prompt.md`, `weapp-review.prompt.md`

业务域层模板：

- `backend-payment-runbook.prompt.md`
- `backend-sql-review.prompt.md`
- `backend-integration-test.prompt.md`
- `backend-task-card-implementation.prompt.md`
- `backend-phase-batch-implementation.prompt.md`
- `weapp-payment-flow.prompt.md`
- `weapp-page-refactor-blueprint.prompt.md`
- `business-flow-mermaid.prompt.md`

要求：

- 技术栈层 Prompt 不要重复定义通用风险分级与发布口径，应复用协议层和工程治理标准。
- 业务域层 Prompt 只在该域存在明显独特失败模式时新增，不要为了“更专业”滥建新文件。

### Layer 4: Agents And Gates

位置：`.github/agents/`, `.github/workflows/`, `.github/scripts/`

职责：

- 处理真正需要工具边界的模式，如只读审计或多 Agent 编排。
- 把可自动化校验的规则做成脚本和 CI 门禁。

要求：

- 能用 Prompt 表达的，不新增 Agent。
- 能用 lint 与 workflow 阻断的，不只写成文档提醒。

## 2. Prompt Gate Requirements

Prompt 库必须接受和代码同等级的基础门禁。

### 2.1 Required Lint Checks

- Frontmatter 完整：每个 `.prompt.md` 必须有 `name` 与 `description`。
- Routing hints 完整：每个 `.prompt.md` 必须有 `routing-hints`，用于可执行路由断言。
- 名称唯一：Prompt `name` 不得重复。
- 描述唯一：Prompt `description` 不得重复，避免路由冲突。
- Trigger phrases 明确：可路由 Prompt 的 `description` 必须声明 `Trigger phrases:`。
- 协议边界明确：`general-implementation.prompt.md` 与 `general-review.prompt.md` 必须保留 `Use only when` 边界。
- 索引一致：`.github/prompts/README.md` 中 `Current Templates` 必须与实际 Prompt 文件一一对应。
- Agent 引用有效：Prompt frontmatter 中的 `agent` 必须指向真实存在的 Agent 名称。
- README 一致：`.github/agents/README.md` 必须能覆盖当前 Agent 文件集合。
- 仓库内引用可达：AI-facing 资产中的仓库路径引用不得悬空。

### 2.2 CI Rule

- Prompt lint 失败应阻断 PR。
- Prompt routing tests 失败应阻断 PR。
- Prompt 相关 README、instructions、agents、standards 变更必须同时接受相同 lint。
- 如果代码路径变更会打断 AI-facing 资产中的仓库内链接，Prompt lint 也应触发。

### 2.3 Required Branch Checks

默认主分支应至少要求以下检查通过：

- `Prompt Governance Lint`
- `Prompt Routing Tests`

如果当前环境无法直接改仓库保护规则，也要至少把这两个 job name 固化在 workflow 中，避免后续仓库设置与 CI 名称漂移。

## 3. Shared Implementation And Review Matrix

以下矩阵是 implementation prompt 与 review prompt 的共同来源。

### 3.1 Backend

| Area | Implementation Must Push | Implementation Must Not | Review Must Check |
| --- | --- | --- | --- |
| Backend | 闭环打通 handler / logic / store / route / DTO / tests；状态常量复用 `db/sqlc/constants.go`；识别并执行 `make sqlc` / `make mock` / `make swagger`；说明验证范围与残余风险 | 不要把业务逻辑塞进 handler；不要新增魔法状态字符串；不要只改 SQL、DTO 或 handler 而不做全链路传播；不要跳过生成步骤判断 | 传播是否断层；新增逻辑是否真正可达；生成物是否提交；回调、异步、支付、上传、OCR、鉴权等高风险路径是否真实验证 |

### 3.2 Web

| Area | Implementation Must Push | Implementation Must Not | Review Must Check |
| --- | --- | --- | --- |
| Web | 新字段与状态贯通类型、API、页面状态、渲染与文案；复用共享 UI；补齐 loading / empty / error / validation；明确危险动作和敏感字段行为 | 不要在页面本地发明后端没有的状态语义；不要复制共享组件形成分叉；不要硬编码一次性视觉 token；不要把页面数据逻辑回塞进展示组件 | 状态传播是否完整；共享 UI 是否被绕开；危险动作、敏感字段与失败态是否处理完整；视觉系统是否出现新漂移 |

### 3.3 Mini Program

| Area | Implementation Must Push | Implementation Must Not | Review Must Check |
| --- | --- | --- | --- |
| Weapp | 以后端契约为唯一真相；service / state / handlers / WXML / WXSS / feedback 一起改；显式处理 loading / success / empty / error / retry / re-entry；说明弱网、重复点击与冷启动恢复 | 不要猜后端字段、状态、权限或统计语义；不要只改 WXML / WXSS；不要把金额语义、角色边界或结果语义混掉；不要把重写 TDesign 内部样式当常规方案 | setData 粒度、状态恢复、弱网与重入、支付结果、角色边界、页面壳一致性、反馈通道是否正确 |

### 3.4 Cross-Stack Rules

| Area | Implementation Must Push | Implementation Must Not | Review Must Check |
| --- | --- | --- | --- |
| Cross-stack | 风险分级；最小充分验证；未验证路径与残余风险；必要时说明发布与回滚口径 | 不要把高风险改动当 routine patch；不要只报“已完成”不报验证边界；不要只修表层症状不修根因 | 端到端路径是否完整；失败路径与恢复路径是否被验证；文档、门禁与生成物是否需要同步 |

## 4. Prompt Addition Rules

- 新 Prompt 必须先回答“为什么现有 Prompt 无法承载”。
- 如果不能为新 Prompt 增加新的 routing test case，它通常不该存在。
- 新 Prompt 必须补齐 `routing-hints`，否则不能进入可执行路由断言体系。
- 如果新 Prompt 只是补充技术栈或业务域细节，应优先更新现有技术栈层或业务域层 Prompt。
- 如果新规则是长期有效的，应先落到 standards，再镜像到 Prompt。

## 5. Maintenance Rules

- 当 `.github/prompts/README.md` 的 `Current Templates`、`Prompt Boundaries` 或 `Routing Order` 变化时，必须运行 Prompt lint。
- 当新增或移除 `.agent.md` 文件时，必须同步更新 `.github/agents/README.md`。
- 当 `.github/standards/engineering/` 中的跨层治理规则变化时，必须重新检查协议层 Prompt 是否仍与治理基线一致。
- 当技术栈新增反复出现的 defect pattern 时，优先更新本文件矩阵或 area-specific instructions，再决定是否新增 Prompt。