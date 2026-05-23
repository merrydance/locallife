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

- Backend: `backend-implementation.prompt.md`, `backend-review-closure.prompt.md`, `backend-bugfix.prompt.md`, `backend-takeover.prompt.md`
- Web: `web-implementation.prompt.md`, `web-review.prompt.md`
- Weapp: `weapp-implementation.prompt.md`, `weapp-review.prompt.md`
- Flutter Merchant App: `flutter-implementation.prompt.md`, `flutter-review.prompt.md`, `flutter-bugfix.prompt.md`

业务域层模板：

- `backend-payment-domain.prompt.md`
- `backend-sql-review.prompt.md`
- `backend-integration-test.prompt.md`
- `backend-task-card-implementation.prompt.md`
- `backend-phase-batch-implementation.prompt.md`
- `business-flow-mermaid.prompt.md`

要求：

- 技术栈层 Prompt 不要重复定义通用风险分级与发布口径，应复用协议层和工程治理标准。
- 业务域层 Prompt 只在该域存在明显独特失败模式时新增，不要为了“更专业”滥建新文件。
- 对外部平台且以异步能力组为主的业务域，domain prompt 必须把任务收束到“能力组 + caller 传播 + focused validation”，而不是按单个 endpoint 组织任务。

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

### 2.0 Context Rehydration Gate

AI 任务不能依赖旧上下文里的“我记得”。以下情况必须先重新执行路由，再继续实现、审查或总结：

- 新会话开始。
- 上下文被压缩或摘要替换。
- 分叉到子 Agent、并行 Agent、接手 Agent 或新的执行者。
- 任务范围、目标路径或风险级别发生变化。
- 从计划、审查、修复、文档同步等阶段切换到另一个阶段。

重装载顺序：

1. Rerun routing from `.github/README.md` and the active `AGENTS.md` / `.github/copilot-instructions.md` entrypoint.
2. 确认目标区域和风险等级。
3. 打开匹配的 `.github/instructions/*.instructions.md`。
4. 如果任务是实现、审查、bugfix、集成测试、接手、事故回灌、任务闭环或图表工作，打开匹配的 `.github/prompts/*.prompt.md`。
5. 只在路径、风险或 prompt 指向时打开更深的 `.github/standards/**` 或 domain README。

执行要求：

- Prompt、instructions 和 AGENTS 入口必须把这条规则写成短门禁，而不是复制完整标准正文。
- Prompt governance lint 必须校验这些入口仍然引用本节或包含等价门禁。
- 交付说明中如果发生过压缩、分叉、接手或阶段切换，应说明已经重新确认了目标 area、prompt/instructions 和相关 standards。
- Do not keep relying on stale context.

### 2.1 Required Lint Checks

- Frontmatter 完整：每个 `.prompt.md` 必须有 `name` 与 `description`。
- 名称唯一：Prompt `name` 不得重复。
- 描述唯一：Prompt `description` 不得重复，避免路由冲突。
- Trigger phrases 明确：可路由 Prompt 的 `description` 必须声明 `Trigger phrases:`，并作为可执行路由断言的唯一来源。
- 不要使用不受支持的自定义 Prompt frontmatter 字段来承载路由信息，例如 `routing-hints`。
- 协议边界明确：`general-implementation.prompt.md` 与 `general-review.prompt.md` 必须保留 `Use only when` 边界。
- 索引一致：`.github/prompts/README.md` 中 `Current Templates` 必须与实际 Prompt 文件一一对应。
- Agent 引用有效：Prompt frontmatter 中的 `agent` 必须指向真实存在的 Agent 名称。
- Backend canonical-owner consistency：backend 热路径入口、legacy `.codex` wrappers 与 formal review ledger 指向不得偏离 `.github` 下的 canonical backend owners。
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
| Backend | 闭环打通 handler / logic / store / route / DTO / tests；先说明能力归属哪个模块、关键状态由谁唯一写入；状态常量复用 `db/sqlc/constants.go`；识别并执行 `make sqlc` / `make mock` / `make swagger`；说明 unexpected error 在哪里被记录、业务错误与基础设施错误如何分流、以及前端或调用方会收到什么稳定语义；对仓库级高风险链路参考 `backend/README.md` 与匹配的 domain README；外部 API/provider 变更必须先确认官方契约、样例、字段矩阵、错误码、枚举、条件必填和漂移复核；说明事务边界、副作用边界、验证范围与残余风险 | 不要把业务逻辑塞进 handler；不要新增魔法状态字符串；不要只改 SQL、DTO 或 handler 而不做全链路传播；不要让多个包同时写同一关键状态；不要为了“以后可能复用”提前抽共享层；不要跳过生成步骤判断；不要把生产 bugfix 当成表层补丁而不追真实写边界或恢复路径；不要静默吞掉 unexpected error、把 nil 或 0-row/no-op 当成功、或把内部错误细节直接透给前端；不要盲猜 provider 字段、枚举、单位、嵌套结构或把外部 API 异常隐式降级成成功 | 传播是否断层；模块所有权是否清楚；关键状态是否存在多头写入；事务与副作用边界是否分离；新增逻辑是否真正可达；生成物是否提交；是否存在静默吞错、nil-as-success、缺失结构化日志边界、或对前端语义含糊/泄漏内部细节的错误映射；外部 API/provider 是否有官方真值、字段矩阵、source/evidence ledger、显式降级规则、parser/validator/error mapper 测试和清晰调用方指引；回调、异步、支付、上传、OCR、鉴权等高风险路径是否真实验证；正式 review 是否形成 durable closeout |

### 3.2 Web

| Area | Implementation Must Push | Implementation Must Not | Review Must Check |
| --- | --- | --- | --- |
| Web | 遵循 `frontend/FRONTEND_ARCHITECTURE_BASELINE.md`，先从用户任务和 ViewState 推导页面结构；新字段与状态贯通类型、API、页面状态、渲染与文案；复用共享 UI；补齐 loading / empty / error / validation / submitting / retry；明确危险动作和敏感字段行为 | 不要按 API/DTO 形状平铺页面、卡片或组件；不要在页面本地发明后端没有的状态语义；不要复制共享组件形成分叉；不要硬编码一次性视觉 token；不要把页面数据逻辑回塞进展示组件 | 页面是否由用户任务驱动而不是 API 镜像；ViewState 是否完整；状态传播是否完整；共享 UI 是否被绕开；危险动作、敏感字段与失败态是否处理完整；视觉系统是否出现新漂移 |

### 3.3 Mini Program

| Area | Implementation Must Push | Implementation Must Not | Review Must Check |
| --- | --- | --- | --- |
| Weapp | 遵循 `frontend/FRONTEND_ARCHITECTURE_BASELINE.md`，把 API 平铺视为架构缺陷；以后端契约为唯一真相；先盘点后端能力并组合成用户任务域，再决定组件边界和一页还是一组页面；service / state / handlers / WXML / WXSS / feedback 一起改；显式处理 loading / success / empty / error / retry / re-entry；说明弱网、重复点击与冷启动恢复；默认不要主动生成说明性文案，先用结构、状态、标签、字段名和动作层级把任务讲清；只有风险告知、状态承接、字段约束或下一步动作规则在不写出来会造成理解错误时，才允许补一条最短必要说明；同一信息不要在标题、副标题、note、notice、卡片说明里重复解释；非顾客侧局部动作默认用 TDesign 图标按钮或 icon-led small button；优先用 page shell + 内容容器 + TDesign 组件表达，而不是再包本地视觉壳 | 不要猜后端字段、状态、权限或统计语义；不要按 API/DTO/handler 数量机械拆页面或铺卡片；不要把所有能力堆进同一页面；不要只改 WXML / WXSS；不要把金额语义、角色边界或结果语义混掉；不要把重写 TDesign 内部样式当常规方案；不要在单任务页首屏堆解释卡或说明块；不要为了“显得完整”补充不改变用户决策的说明文字；不要把“这里主要用于…/你可以在这里…”一类边界解释写成默认页面文案；不要默认保留文本型局部编辑/删除/测试/状态按钮；不要为了“更完整”再叠局部卡片壳和说明壳 | 页面是否由人的任务、行为和目标驱动而不是 API 镜像；能力组合是否合理；组件与页面边界是否清楚；setData 粒度、状态恢复、弱网与重入、支付结果、角色边界、页面壳一致性、反馈通道是否正确；是否出现解释卡漂移、说明文案堆叠、重复解释、文本动作漂移和本地视觉壳膨胀 |

### 3.4 Flutter Merchant App

| Area | Implementation Must Push | Implementation Must Not | Review Must Check |
| --- | --- | --- | --- |
| Flutter Merchant App | 遵循 `frontend/FRONTEND_ARCHITECTURE_BASELINE.md`，先建模商户任务、ViewState 和 state owner，再落 Widget；按 disconnected edge client 建模；说明 state owner、recovery boundary、失败模式；把 service / provider / persistence / lifecycle / UI 贯通；关键链路显式处理 duplicate、retry、cold start、re-entry；按风险说明真实验证和残余风险 | 不要把 API/entity 模型直接铺成屏幕；不要把业务逻辑和副作用塞进 Widget；不要假设单通道可靠或 exactly-once；不要为 `G2`/`G3` 路径做乐观成功；不要发明后端语义；不要省略真机、弱网、恢复路径的未验证说明 | 屏幕是否由商户任务和 ViewState 驱动；状态所有者是否清晰；去重、重试、冷启动、后台恢复是否被 deliberate 处理；push、foreground service、auth refresh、accept-order 等链路是否闭环；高风险设备或厂商路径是否仍未验证 |

### 3.5 Cross-Stack Rules

| Area | Implementation Must Push | Implementation Must Not | Review Must Check |
| --- | --- | --- | --- |
| Cross-stack | 风险分级；最小充分验证；未验证路径与残余风险；必要时说明发布与回滚口径 | 不要把高风险改动当 routine patch；不要只报“已完成”不报验证边界；不要只修表层症状不修根因 | 端到端路径是否完整；失败路径与恢复路径是否被验证；文档、门禁与生成物是否需要同步 |

### 3.5 Cross-Task Execution Baseline

以下规则适用于 implementation prompt、instructions 与相关执行门禁，是跨技术栈的默认行为基线：

- 任务必须尽量单一、边界清晰、可在一个上下文窗口内完整理解和完成；如果任务拆开后更容易丢失语义或需要反复猜测，就先拆成更小的独立任务。
- 每个任务应明确目标、输入、涉及文件、输出、验收标准与验证范围；任务描述必须足够具体，即使失去当前会话上下文也能按文件和约束继续执行，而不是靠“看着办”。
- 当一个请求同时包含多个独立能力、多个产品面、多个高风险分支或多个不共享状态的改动时，优先先拆分再执行；不要把多个可独立交付的工作包装成一个模糊的大任务。
- 对于实现或审查入口，任务边界越清楚，越应该把“写什么”“不写什么”“验证什么”“不验证什么”讲明白。
- 先显式说明会影响行为、范围或验证口径的关键假设与歧义；如果存在多个会导致不同实现的合理解释，不要静默选一个。
- 默认选择最简单、最小的可交付实现；不要为了未来可能复用而提前加抽象、配置层或扩展点。
- 默认做外科式改动；不要顺手重构相邻代码、重写无关注释或清理与当前请求无关的历史问题。若本次改动引入了新的 unused import、unused variable 或 orphan，再由实现方一并清理。
- 对 bugfix、refactor 或多步骤任务，prompt 应要求把工作转成可验证的短计划或成功标准，并在每一步之后执行最小相关验证，而不是只报告“已完成”。
- 当实现方无法确定需求边界时，应先暴露不确定性与取舍，再决定是否需要向上游提问；不要靠隐式猜测推进高影响改动。
- Instructions 层只镜像这些规则的高频执行版本；prompt 层只保留协议化输入输出要求，避免把同一段长文本复制到多个模板中。

## 3.6 Security Pattern Baseline

安全约束应写入提示词系统，但要按“高频、可执行、可审查、可门禁化”的方式分层，不做无边界漏洞清单。

### Must Cover

- 重放、重复提交、重复消费、重复回调、重复投递、竞态与时序漂移。
- 身份伪造、对象级越权、租户串扰、权限回退、签名绕过、回调伪造。
- 注入类风险：SQL、命令、模板、HTML/JS、路径、对象键、反序列化、开放重定向。
- 敏感信息泄漏：token、密钥、证书、银行卡号、身份证号、原始 provider payload、堆栈、调试痕迹、内部主键。
- 失效和降级类风险：空成功、nil-success、silent fallback、伪成功、隐藏失败、未观测失败。
- 资源与并发类风险：无界 fan-out、TOCTOU、claim 竞争、重复执行不收敛、超时和重试语义不清。

### Where To Put Them

- 长期原则放在 `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md`。
- 后端高频实现约束放在 `.github/standards/backend/AGENT.md`、`ERROR_HANDLING.md`、`IDEMPOTENCY_STANDARDS.md`、`EXTERNAL_API_CONTRACT_STANDARDS.md` 和匹配 domain README。
- 高风险审查检查项放在 `.github/instructions/review.instructions.md` 与 backend review prompt。
- 能自动识别的模式优先做成 workflow、脚本或 guard，而不是只靠人记。
- 不要把已知攻击面写成无限增长的枚举表；优先收敛成模式、边界和验证要求。

## 4. Prompt Addition Rules

- 新 Prompt 必须先回答“为什么现有 Prompt 无法承载”。
- 如果不能为新 Prompt 增加新的 routing test case，它通常不该存在。
- 新 Prompt 必须补齐 `description` 中的 `Trigger phrases:`，否则不能进入可执行路由断言体系。
- 如果新 Prompt 只是补充技术栈或业务域细节，应优先更新现有技术栈层或业务域层 Prompt。
- 如果新规则是长期有效的，应先落到 standards，再镜像到 Prompt。

## 4.1 Async Capability-Group Rule

当某个业务域满足以下特征时，应默认按“能力组治理”而不是“单接口治理”组织 Prompt 与 instructions：

- 主要依赖外部平台接口
- 一组接口共同完成一个业务能力
- 存在回调、轮询、恢复任务、状态页或异步通知
- caller 分布在 integration boundary 之外

治理要求：

- 先有官方文档基线，再有仓库内传播矩阵，再让 instructions 和 prompt 镜像执行规则。
- Domain prompt 必须要求声明当前能力组、caller 传播面、验证范围和残余风险。
- 如果没有仓库内传播矩阵，prompt 不得把任务表述成“已完整对齐”，只能推动补齐矩阵或明确标注治理缺口。

平台收付通 / 微信支付域默认属于这一类。

## 5. Maintenance Rules

- 当 `.github/prompts/README.md` 的 `Current Templates`、`Prompt Boundaries` 或 `Routing Order` 变化时，必须运行 Prompt lint。
- 当新增或移除 `.agent.md` 文件时，必须同步更新 `.github/agents/README.md`。
- 当 `.github/standards/engineering/` 中的跨层治理规则变化时，必须重新检查协议层 Prompt 是否仍与治理基线一致。
- 当技术栈新增反复出现的 defect pattern 时，优先更新本文件矩阵或 area-specific instructions，再决定是否新增 Prompt。
