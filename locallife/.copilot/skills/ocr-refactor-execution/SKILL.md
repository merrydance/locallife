---
name: ocr-refactor-execution
description: 按 .github/standards/domains/ocr/OCR_REFACTOR_EXECUTION_PLAN_2026-03-25.md 的基线，分阶段执行 Locallife OCR 改造，优先完成基础设施、媒体读取能力和阿里云主 provider 接入，并在每条业务线迁移时同步删除旧 OCR 链路。
---

# OCR Refactor Execution

## Purpose

将 OCR 改造从分散的 handler 直连实现，收敛为基于 media_asset_id、异步任务、统一 provider 抽象和统一回写模型的生产级链路。

这个技能适用于以下场景：

- 继续推进 .github/standards/domains/ocr/OCR_REFACTOR_EXECUTION_PLAN_2026-03-25.md 中的任一阶段或小任务。
- 评估某个 OCR 改动是否违反既定基线。
- 把现有业务线从旧 OCR 入口迁移到统一 ocr.Service。
- 为 OCR 改造生成明确的实施顺序、删除清单、测试清单和验收标准。

## Required Baseline

开始任何改造前，先确认以下约束没有被破坏：

1. 文档基线优先于代码。任何新增结论、路由变化、可见性变化、provider 策略变化，都必须先更新 .github/standards/domains/ocr/OCR_REFACTOR_EXECUTION_PLAN_2026-03-25.md，再改代码。
2. 统一主入口。OCR 最终只接受 media_asset_id 作为主入口，不保留 multipart 同步主链路。
3. 统一执行模型。主模型是创建任务、异步处理、回写结构化结果。
4. 统一边界。允许 handler 调用 ocr.Service；禁止 handler 直接调用 provider SDK。
5. 一次性切净。服务尚未上线，不为旧 OCR 接口、旧 worker payload、本地路径读取保留长期兼容层。
6. 隐私优先。private 证件不能通过 public URL 给 OCR 使用，必须由服务端直接读取字节流。
7. 阿里云优先。当前主 provider 是阿里云；微信只作为后续可选第二 provider。

## Default Execution Order

除非用户明确指定别的范围，否则按以下顺序推进：

1. T0: 固定文档类型到阿里云能力映射。
2. T1-T8: 建立 ocr_jobs、sqlc、ocr 包、service/router/provider 基础结构。
3. T9-T10: 建立 ReadMediaAsset 统一读取能力及测试。
4. T33A-T33F、T33I: 接入阿里云 provider、配置校验和错误映射。
5. T11-T18: 先迁移商户 food permit，作为第一条完整新链路。
6. T19-T26: 再迁移营业执照和身份证，并删除旧同步入口。
7. T27-T33H: 迁移扩展证件并清理旧 multipart、旧路径依赖和旧对外接口。
8. T34-T54: 补幂等、lease、重试、观测、审计、发布和回滚能力。

如果用户只要求某个单点任务，也要先检查它是否依赖前置基础设施；缺前置时先说明阻塞，再补基础设施。

## Input Contract

调用这个技能时，优先从用户请求中抽取以下输入：

- 目标阶段或任务编号
- 是要直接实现、先评审、还是先补计划文档
- 影响范围是基础设施、单条业务线，还是全链路收尾
- 是否要求同步删除旧代码
- 是否要求运行测试或只做结构改造

如果用户没有明确给出阶段或任务编号，默认从执行计划里的“当前最优先执行的小任务”开始，而不是自行跳到后续业务线迁移。

## Workflow

### 1. Lock The Scope

明确本轮只做哪一个阶段、哪几个小任务，以及它们的前置依赖。

输出至少包括：

- 当前阶段和任务编号
- 受影响的业务线
- 需要新增的表、接口、配置、worker 或路由
- 需要删除的旧链路

### 2. Verify The Baseline

对照执行计划检查本轮改动是否会触碰以下敏感规则：

- 证件 visibility
- public URL 门禁
- private 证件访问边界
- provider 主次关系
- OCR 入口是否仍走 media_asset_id

如果会改变这些规则，先改文档，再继续代码实现。

### 3. Build Or Reuse Shared Infrastructure First

如果当前任务依赖公共基础设施，则优先补齐：

- ocr_jobs migration 与 sqlc
- ocr/types.go、provider.go、router.go、service.go
- 媒体二进制读取能力
- 阿里云 provider 封装与配置校验

不要在业务 handler 里临时塞入 provider 逻辑来“先跑通”。

### 3A. Choose The Right Branch

按当前请求所属类型，选择执行分支：

- 如果请求是“开始执行计划”或“继续推进”，默认走基础设施优先分支，从 T0、T1-T10、T33A-T33F 开始。
- 如果请求是“迁移某条业务线”，先检查 T0-T10 和 provider 封装是否已满足，否则先补前置。
- 如果请求是“review”或“检查方案”，重点输出违反基线、未删旧链路、测试缺失和隐私边界风险，不直接把总结当结果。
- 如果请求是“只更新文档”，只更新计划与约束，不顺手改代码。
- 如果请求是“只做 provider 接入”，禁止顺手在 handler 内落阿里云 SDK 调用。

### 4. Migrate One Business Slice End To End

以一条业务链路为单位完成闭环：

1. handler 改成调用 ocr.Service 创建任务。
2. worker payload 收敛为最小标识符，优先使用 ocr_job_id。
3. worker 内加载 job，读取 media asset 字节流。
4. 通过 router 选择 provider 执行 OCR。
5. 解析 raw result，生成 normalized result。
6. 通过 projector 回写业务字段。
7. 删除这条业务线对应的旧 OCR 入口、旧 payload、旧路径读取代码。

不要在主分支长期保留新旧两套主链路并存。

### 5. Apply Production Gates

每完成一轮实现，都检查以下门槛：

- 是否存在幂等键，避免重复创建任务
- 是否具备 lease 或等价并发保护
- 是否区分可重试与不可重试失败
- 是否保留 raw_result 和 normalized_result
- 是否对敏感证件限制访问、定义留存期和脱敏策略
- 是否增加日志、metrics、审计或至少为后续接入预留结构

如果本轮范围不包含这些能力，也要明确哪些还没补，不能默认视为已完成。

### 6. Validate Before Declaring Done

至少完成与本轮改动直接相关的验证：

- 单元测试
- handler/worker 集成测试
- 状态流转测试
- 幂等或重试测试（如果本轮引入了对应能力）
- 文档更新与任务勾选

如果测试无法运行，明确说明原因和未验证风险。

### 7. Close The Loop In The Plan

完成后回写执行计划文档，更新：

- 已完成的小任务复选框
- 新增的决定或边界
- 风险变化
- 下一步最合理的任务序列

不要只改代码不更新计划，否则执行基线会再次漂移。

## Required Output Shape

每次按该技能响应时，优先使用下面的输出骨架：

1. 当前推进的阶段和任务编号。
2. 前置依赖是否满足；若不满足，先列阻塞项。
3. 本轮准备新增或修改的文件、表、接口、worker、配置。
4. 本轮准备删除的旧 OCR 入口、旧 payload、旧路径读取依赖。
5. 本轮验证方式，包括测试、lint、状态机或集成验证。
6. 完成后如何更新执行计划文档。

如果是 review 模式，改为：

1. Findings，按严重度排序。
2. 影响的阶段或任务。
3. 缺失的测试与验收项。
4. 可接受的下一步修复顺序。

## Decision Points

### When To Update The Plan First

出现以下任一情况，先更新执行计划：

- 新增 document type
- 修改 provider 路由原则
- 修改 public/private 可见性
- 修改 OCR API 对外模型
- 修改 worker 重试、lease 或保留策略

### When To Delete Old Code Immediately

出现以下任一情况，本轮应同步删除旧实现：

- 某业务线已切到 ocr.Service
- worker 已切到 ocr_job_id
- 图片已可通过统一媒体读取能力读取
- 旧接口只是未上线兼容壳层，没有真实生产依赖

### When To Block A Shortcut

直接判定为不应接受的捷径：

- 在 handler 中直接调用阿里云或微信 SDK
- 用 mediaAssetLocalPath 或 uploads 本地路径作为长期 OCR 主读取方式
- 为 private 证件生成 public URL 供 provider 拉取
- 为了“先跑通”继续保留 multipart 同步 OCR 主入口
- 在业务线里复制一套 provider 错误映射或解析逻辑

### When To Stop And Clarify

出现以下情况时，先向用户确认，而不是直接落代码：

- 用户要求一次跨越多个阶段，但没有说明优先级
- 当前代码状态与执行计划文档发生明显冲突
- 删除旧接口可能影响未识别到的调用方
- 用户要求“继续”，但没有说明是继续强化技能还是继续实际 OCR 编码

## Completion Checklist

一次改造在满足以下条件前，不算完成：

1. 代码实现符合本轮阶段目标。
2. 旧链路已按计划删除，而不是只新增新链路。
3. 测试覆盖了新增行为和关键失败路径。
4. 执行计划文档已同步更新。
5. 最终说明里明确：完成了什么、还缺什么、下一步是什么。

如果本轮只是创建技能或更新方法论，也应明确：

1. 技能文件是否已保存到仓库。
2. 技能解决的是哪类 OCR 改造任务。
3. 还有哪些边界需要用户确认。

## Response Template

执行该技能时，优先按以下结构输出：

1. 当前要推进的阶段与任务编号。
2. 前置依赖是否满足。
3. 将新增哪些基础设施或迁移哪条业务链路。
4. 将同步删除哪些旧入口或旧依赖。
5. 将运行哪些测试或验证。
6. 完成后如何回写执行计划。

## Example Prompts

- 按 OCR 执行计划开始阶段 B，先完成 T0 到 T8。
- 按 OCR 技能推进商户 food permit 迁移，要求迁移完成后直接删旧 worker payload。
- 用 OCR 改造技能检查我这次变更是否违反了 private 证件读取边界。
- 按执行计划完成阿里云 provider 的第一轮接入，只做配置、client 封装和错误映射测试。
- 继续 OCR 计划，默认从当前最优先的小任务开始，不要跳阶段。