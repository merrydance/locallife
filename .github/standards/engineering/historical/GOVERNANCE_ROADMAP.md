# 工程治理落地路线图

> 状态：Historical reference

本文件记录 LocalLife 曾经用于把跨层治理母标准系统性接入现有 `.github/` 文档、Prompt、Workflow 和交付流程的落地路线图。

阶段 0 到阶段 6 已全部完成，因此该文件不再属于默认热路径的 active guidance。后续如需追溯治理建设顺序、阶段验收口径或当时的落地范围，可将本文件作为历史参考。

## 1. 范围

本路线图覆盖：

- `.github/standards/`
- `.github/instructions/`
- `.github/prompts/`
- `.github/workflows/`
- Code Review、验证说明、发布就绪与事故回灌流程

## 2. 不遗漏原则

任何阶段完成时，都必须检查以下治理面是否已有落点，不能只完成其中一层：

1. Canonical standards
2. Auto-matched instructions
3. Reusable prompts
4. CI/workflow gates
5. Review checklist
6. Validation expectations
7. Release-readiness expectations
8. Incident feedback loop

## 3. 必落主题清单

以下主题在整个治理建设过程中一个都不能漏：

1. 身份、权限、对象级授权、租户边界
2. 输入校验、输出映射、敏感信息与日志脱敏
3. 状态机、事务、一致性、并发、幂等
4. 回调、Webhook、异步任务、重试、补偿
5. 上传下载、媒体、OCR、支付与外部集成信任边界
6. Web 与 Weapp 的设计系统、一致交互、弱网与失败承接
7. 测试矩阵、剩余风险表达、发布就绪与观测
8. 事故复盘到规则、工具、测试与脚手架的回灌

## 4. 分阶段计划

### Phase 0: 盘点与分类

目标：

- 把现有规则按安全、一致性、鲁棒性、验证、发布、复盘重新分类
- 标出已有标准、只有 reviewer 经验、完全缺失三类状态

本阶段产物：

- `.github/standards/engineering/` 目录
- 治理母标准入口
- 本路线图
- `.github/README.md`、`.github/standards/README.md`、`.github/NORMS_AUDIT.md` 的治理接线

完成标准：

- 团队可以从 `.github/` 索引层找到跨层治理的唯一权威入口
- `.github/NORMS_AUDIT.md` 能显式看出治理主线仍有哪些镜像层尚未落地

### Phase 1: 母标准落地

目标：

- 形成所有产品面共享的治理总基线
- 明确风险分级、最低门槛、发布口径和事故回灌要求

本阶段产物：

- `ENGINEERING_GOVERNANCE_BASELINE.md`

完成标准：

- backend、web、weapp、domain 规则都能找到跨层母线
- 不再需要通过聊天记录解释“高安全、高一致性、高鲁棒性”的共同含义

### Phase 2: Instructions 镜像

目标：

- 把会直接影响日常开发、Review、生成和校验的治理要求镜像到 auto-matched instructions

目标文件：

- `.github/instructions/backend-locallife.instructions.md`
- `.github/instructions/web-ui.instructions.md`
- `.github/instructions/weapp-mini-program.instructions.md`
- `.github/instructions/review.instructions.md`

完成标准：

- AI 或 reviewer 在对应路径工作时，会自动受到风险分级、失败模式、剩余风险表达等提醒

### Phase 3: Prompt 镜像

目标：

- 把治理要求接入高频任务入口，避免实现、审查、闭环任务默认漏掉高风险检查面

目标文件：

- `.github/prompts/general-implementation.prompt.md`
- `.github/prompts/general-review.prompt.md`
- `.github/prompts/general-task-loop.prompt.md`
- 必要时新增治理专项 prompt，但只在任务模式足够高频时新增

完成标准：

- 默认任务输入会要求风险等级、验证范围、剩余风险与文档镜像决策

### Phase 4: Workflow 与脚本门禁

目标：

- 把高频低级错误、明显安全缺口和生成物漂移尽量前移到自动化门禁

重点方向：

- backend: 生成物一致性、测试分层、focused security gates
- web: lint/build/type-check 等基础质量工作流补齐
- weapp: 质量检查与设计系统漂移、危险默认样式、状态完备性等可脚本化检查逐步增强

完成标准：

- 低级重复错误不再主要依赖人工 Review 发现

### Phase 5: 测试矩阵与发布矩阵

目标：

- 建立跨 area 的统一验证语言与发布就绪口径

完成标准：

- 高风险改动在交付时必须能清楚说明：测了什么，没测什么，上线后怎么观测与止损

### Phase 6: 事故回灌闭环

目标：

- 把线上事故、重大 review finding、重大测试逃逸转化为新的默认约束

完成标准：

- 事故复盘结论至少沉淀为标准、instructions、prompts、workflow、测试或共享基础设施中的一项

## 5. 阶段验收检查表

每一阶段结束时，统一按以下问题收口：

1. 这阶段新增的规则是否有 canonical path。
2. 哪些规则只停留在 standards，哪些已经镜像到 instructions。
3. 哪些规则已经影响 prompts 和 review 入口。
4. 哪些规则已自动化，哪些仍只能靠人工执行。
5. 哪些高风险路径仍无明确验证与发布口径。
6. 哪些文档仍是 active guidance，哪些应在后续转入 historical。

## 6. 当前阶段状态

- Phase 0：已完成
- Phase 1：已完成
- Phase 2：已完成
- Phase 3：已完成
- Phase 4：已完成
- Phase 5：已完成
- Phase 6：已完成

## 7. 与现有文档体系的整合约束

- 长期规则写入 `.github/standards/engineering/`，而不是散落到聊天记录或一次性任务文档。
- 具体 area 细则仍留在 backend、web、weapp 与 domain 标准中。
- 执行层镜像应尽量更新现有 instructions 与 prompts，而不是新增重复入口。
- 若将来新增治理专项 prompt 或 workflow，必须同步更新 `.github/README.md`、`.github/prompts/README.md` 或 `.github/NORMS_AUDIT.md`。