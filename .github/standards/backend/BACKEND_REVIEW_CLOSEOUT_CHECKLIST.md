# Backend Review Closeout Checklist

> 作用：把正式 backend review、subsystem audit 或高风险路径审查的收口要求标准化，避免高价值 finding 只停留在一次对话里。

与具体写回规则配合使用：`.github/standards/backend/FORMAL_REVIEW_DURABILITY.md`

在完成正式 backend 审查后，至少过一遍本清单。

## 1. Findings First

- 审查输出是否先列 findings，而不是先写总结？
- 每条 finding 是否包含具体文件路径、问题、运行或维护后果？
- 如果没有明确缺陷，是否明确写出“未发现明确缺陷”，并补充残余风险或验证空白？

## 2. High-Risk Coverage

- 若范围涉及资金、支付、退款、分账、配送、预订、callback、worker、scheduler、OCR、上传下载或 authz，是否明确检查了高风险路径？
- 若高风险路径没有实际验证证据，是否明确点名未验证的是哪个 callback、retry、recovery、duplicate-delivery 或 authz 分支？
- 是否检查了 `make sqlc`、`make mock`、`make swagger`、`make check-generated`、`make test-safety` 等必要生成或验证步骤？
- 若改动触及错误处理、空值回退、5xx 返回、上游失败映射或前端提示语义，是否明确检查了 unexpected error 是否到达唯一结构化日志边界、业务错误与基础设施错误是否正确分流、以及对前端是否保持稳定且不泄漏内部细节的语义？
- 是否明确检查了 `nil`、0-row conditional update、空 DTO、silent early return 或 fallback/no-op 分支没有把真实失败伪装成成功？
- 若改动触及外部 API / provider 契约，是否按 `.github/standards/backend/EXTERNAL_API_CONTRACT_STANDARDS.md` 检查官方文档、source matrix、field matrix、样例夹具、字段类型、必填性、枚举、错误码、降级规则和漂移复核？

## 3. Durable Knowledge

- 如果 finding 暴露的是重复 bug class、事务边界漂移、权限检查漏项、worker/scheduler 被遗忘等系统性问题，是否把它反馈到标准、instructions、prompts、workflow、测试或 runbook？
- 如果本仓库已有风险地图、review backlog、审计日志或 domain audit 机制，是否把高价值 finding 写入对应 artifact，而不是只留在聊天记录里？
- 如果现有治理资产缺少该 bug class 的默认约束，是否明确指出应该补到哪里？
- 如果 finding 属于静默吞错、5xx 跳过日志、raw upstream error 透传、或 caller-facing error semantics 含糊这类重复问题，是否检查 `.github/standards/backend/ERROR_HANDLING.md`、`.github/instructions/backend-locallife.instructions.md`、相关 backend prompt，以及 `.github/scripts/backend_go_guard.sh` 是否需要同步补门禁？
- 如果 finding 属于盲猜 provider 字段、外部 API 契约未沉淀、字段矩阵漂移、未知枚举成功、隐式降级或 provider 错误映射含糊，是否检查 `.github/standards/backend/EXTERNAL_API_CONTRACT_STANDARDS.md`、匹配 domain README、backend instructions、backend prompt 和相关合同矩阵是否需要同步补强？

## 4. Scope And Remaining Work

- 本轮审查是否明确写出实际覆盖的 scope，而不是暗示“全都看过”？
- 是否写出仍未审查或未验证的剩余范围？
- 对建议修复项，是否说明至少一个应运行的验证命令或回归测试？

## 5. Closeout Rule

正式 backend 审查只有在以下问题都能回答时，才算真正收口：

1. 找到的缺陷或残余风险是否已经结构化记录
2. 重复问题是否被回灌到组织级默认约束
3. 审查范围、未覆盖区域和下一步行动是否对后续接手者清晰可见

如果这些问题答不清，该 review 只能算一次临时意见交换，不能算 durable closeout。
