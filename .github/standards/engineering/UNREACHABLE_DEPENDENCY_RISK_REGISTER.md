# 不可达依赖风险台账

> 作用：为安全扫描中出现的 `required module not called`、`modules you require but your code doesn't appear to call these vulnerabilities` 等非可触达依赖风险提供统一登记、复核和关闭口径。

本文件不替代可触达漏洞修复流程。它只收口“当前代码路径未命中漏洞符号，但依赖图中仍存在带漏洞模块”的场景。

## 1. 适用范围

适用于以下发现：

1. `govulncheck` 的 module result，且当前代码为 0 reachable vulnerabilities。
2. 其他依赖安全扫描工具明确标注为未调用、未导入、未触达的漏洞结果。
3. 已确认当前交付不构成立即可利用路径，但依赖面上仍存在需要后续复核的漏洞模块。

不适用于：

1. 已可触达的高危或关键漏洞。
2. 仅凭主观判断“应该没调用到”的风险。
3. 没有扫描证据、没有版本信息、没有关闭条件的模糊记录。

## 2. 使用原则

### 2.1 不可达不等于可忽略

- 不可达发现不按“当前可利用漏洞”处理，但也不能直接当作无事发生。
- 这类发现至少要有一条台账记录，说明为什么当前判定为不可达，以及什么变化会让它变成需要立即修复的问题。

### 2.2 可触达漏洞不能只进台账

- 若扫描已经确认代码路径可触达，默认要求修复、升级、替换或给出有时限的风险接受说明。
- 禁止把可触达漏洞降格写进本台账后继续当作 routine 变更交付。

### 2.3 台账必须可复核

每条记录至少应包含：

1. 漏洞 ID。
2. 模块名。
3. 当前版本与修复版本。
4. 扫描命令与日期。
5. 当前为何判定为不可达。
6. 哪些代码或依赖变化会触发重新评估。
7. 计划动作与关闭条件。

## 3. 交付与审查口径

- 当安全扫描只剩不可达模块风险时，交付说明应明确引用本台账，而不是口头写“还有 1 个但没关系”。
- review 应确认该发现确实属于不可达模块结果，且已有台账条目、触发条件和后续动作。
- release readiness 应说明安全基线是否仍有未关闭项，以及这些未关闭项是否全部已纳入本台账管理。
- `security-baseline` backend `govulncheck` 工作流若发现新的 module-only finding 但本台账没有对应记录，应视为治理缺口并直接失败，而不是仅保留 warning。

## 4. 关闭条件

以下任一条件满足时，可关闭台账条目：

1. 依赖已升级到修复版本或移除受影响模块。
2. 依赖关系已变化，重新扫描后该 module result 消失。
3. 代码路径新增或变化后，该风险已转为可触达漏洞，并转入正式修复流程。

关闭时应记录关闭日期和依据，不要直接删掉历史记录。

## 5. 当前条目

### Entry 2026-04-05-01

- 漏洞 ID：`GO-2026-4815`
- 模块：`golang.org/x/image`
- 当前版本：`v0.33.0`
- 修复版本：`v0.38.0`
- 扫描命令：`"$HOME/go/bin/govulncheck" -show verbose ./...`
- 扫描日期：`2026-04-05`
- 扫描结果：`Your code is affected by 0 vulnerabilities ... 1 vulnerability in modules you require, but your code doesn't appear to call these vulnerabilities.`
- 当前判定：`不可达模块风险`
- 判定依据：`govulncheck` verbose output 仅在 module result 中报告 `golang.org/x/image/tiff` 的 OOM 风险，当前仓库没有对应 reachable symbol result 或 package result。
- 暴露触发条件：
  - 新增或引入对 `golang.org/x/image/tiff` 的解析路径。
  - 新增图片解码链路把不可信 TIFF 输入接入当前服务。
  - 未来依赖升级或代码改动导致该漏洞从 module result 升级为 package/symbol result。
- 计划动作：
  - 后续依赖收敛或相关图像链路变更时优先升级到 `golang.org/x/image@v0.38.0` 或更高版本。
  - 每次安全基线复跑后确认该条目是否仍为 module-only result。
- 状态：`open-monitored`

## 6. 维护规则

- 新增条目时，优先补充现有台账，而不是另建一次性说明文档。
- `security-baseline` 的 backend `govulncheck` 工作流可产出 report/template artifact，帮助整理 module-only finding；artifact 只是录入辅助，不替代正式台账更新。
- 若某条目长期存在，应在相关 domain runbook 或 release note 中说明它为什么仍保留。
- 若台账内容影响 review、release 或 incident follow-up 口径，应同步更新对应 instructions、prompts 或 workflow 说明。