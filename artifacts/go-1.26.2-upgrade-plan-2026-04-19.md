# Go 1.26.2 升级方案

## 0. 执行结果

2026-04-20 已完成阶段 B 的本地执行与验证。

结果如下：

1. [locallife/go.mod](locallife/go.mod#L3) 已升级到 Go 1.26.2。
2. 已使用 Go 1.26.2 重新安装 sqlc、mockgen、swag、gosec、govulncheck。
3. 本地验证已通过：`make test-unit`、`make test-integration`、`make test-safety`、`make sqlc`、`make swagger`、`make check-generated`。
4. 本地未出现 db/sqlc、db/mock、worker/mock、wechat/mock、docs 漂移。
5. 仍未从当前会话直接触发 GitHub Actions，因此 workflow 级别验证需依赖后续推送或手动触发。

## 1. 结论

当前可以把项目切到 Go 1.26.2，但应继续把这次升级视为独立工程变更，而不是混入业务需求。

原因不是代码不兼容，而是升级虽然已经通过本地验证，CI 与协作面仍应按独立工程变更收口：

1. 模块声明已升级到 Go 1.26.2。
2. CI 和生成检查脚本都跟随 go.mod 提供的工具链。
3. 本地兼容性验证和生成链验证已经完成。
4. CI workflow 结果仍需以推送后实际执行为准。

所以正确顺序应当是：

1. 保持 Go 1.26.2 作为新的版本源头。
2. 推送后观察 CI workflow 结果。
3. 如 CI 稳定，再把升级视为完成收口。

## 2. 当前版本锚点

当前仓库里的关键版本绑定点如下：

1. 模块版本声明在 [locallife/go.mod](locallife/go.mod#L3)。
2. [backend-safety workflow](.github/workflows/backend-safety.yml) 通过 go-version-file 跟随 [locallife/go.mod](locallife/go.mod#L3)，生成物检查步骤不再手工注入 /usr/local/go/bin。
3. [locallife/scripts/check_generated.sh](locallife/scripts/check_generated.sh) 通过当前 PATH 中的 go 运行，并显式校验 go 是否存在。
4. [UNREACHABLE_DEPENDENCY_RISK_REGISTER](.github/standards/engineering/UNREACHABLE_DEPENDENCY_RISK_REGISTER.md) 中的 govulncheck 命令不再固定 GOTOOLCHAIN=go1.25.8。

## 3. 推荐策略

推荐把升级拆成两个独立阶段。

### 阶段 A：统一现有基线

状态：已完成。

目标：先把仓库从“声明 1.25.8，部分实际跑系统 Go 或混用路径”统一到“全部按同一版本运行”。

已处理：

1. 已去掉 workflow 和脚本里对 /usr/local/go/bin 的硬编码依赖。
2. 已把文档中的工具链命令改为跟随当前统一基线。
3. 已确认本地生成检查脚本可以在当前环境下直接运行。

这个阶段完成后，哪怕还不升到 1.26.2，也能消除你这次已经遇到过的“工具链混用”问题。

### 阶段 B：升级到 Go 1.26.2

在阶段 A 稳定后，再单独提交 Go 1.26.2 升级。

需要处理：

1. 更新 go.mod 的 Go 版本。
2. 确认所有 GitHub Actions 运行在同一版本。
3. 重新安装生成工具，避免旧版本缓存污染。
4. 重新跑全部后端验证。

## 4. 实施步骤

### 4.1 阶段 A：统一现有基线

这一阶段已经完成，实际修改包括：

1. 把 [locallife/scripts/check_generated.sh](locallife/scripts/check_generated.sh) 改成使用当前环境里的 go，而不是固定 /usr/local/go/bin/go。
2. 把 [backend-safety workflow](.github/workflows/backend-safety.yml) 中对 /usr/local/go/bin 的 PATH 注入去掉，改成依赖 setup-go 提供的工具链。
3. 把 [UNREACHABLE_DEPENDENCY_RISK_REGISTER](.github/standards/engineering/UNREACHABLE_DEPENDENCY_RISK_REGISTER.md) 的命令改成与当前统一基线一致。

已完成的最小验证：

1. [locallife/scripts/check_generated.sh](locallife/scripts/check_generated.sh) 已在当前环境下成功执行。
2. 生成检查后没有出现 db/sqlc、db/mock、worker/mock、wechat/mock、docs 漂移。
3. CI workflow 不再依赖系统预装 Go 路径。

### 4.2 阶段 B：升级到 Go 1.26.2

这一阶段已完成，实际执行包括：

1. 修改 [locallife/go.mod](locallife/go.mod#L3) 到 1.26.2。
2. 保持所有 workflow 通过 go-version-file 跟随 go.mod。
3. 重新安装 sqlc、mockgen、swag、gosec、govulncheck，避免旧缓存混用。

## 5. 验证矩阵

升级到 1.26.2 后，至少要完整执行以下验证：

1. make test-unit
2. make test-integration
3. make test-safety
4. make sqlc
5. make swagger
6. make check-generated
7. 触发 backend-test workflow
8. 触发 backend-safety workflow
9. 触发 security-baseline workflow

本次本地已完成前 6 项；第 7 到第 9 项仍待推送后在 GitHub Actions 中观察。

## 6. 风险点

这次升级最容易出问题的不是业务代码，而是工具链和生成链：

1. sqlc、mockgen、swag 的输出可能因新 Go 版本或新依赖解析行为发生变化。
2. 如果后续在脚本或 workflow 里重新引入硬编码 Go 路径，会重新出现“版本看起来对、实际编译链不一致”的问题。
3. 如果团队成员本地已分散到 1.25.x 和 1.26.x，多人协作期间会重新出现生成物差异和 cache 差异。

## 7. 建议的提交拆分

建议用两次提交，而不是一次完成：

1. chore: unify go toolchain baseline across repo
2. chore: upgrade backend toolchain to go 1.26.2

这样如果第二步出现兼容性问题，第一步仍然有价值，而且不会影响这次已经提交的食安业务变更。

## 8. 我的建议

如果你现在想尽快完成收口，我建议直接推送当前升级提交并观察 CI。

如果 CI 结果稳定，就可以把这次 Go 1.26.2 升级视为完成；如果 CI 暴露问题，优先按 workflow 失败点做定点修复，不要回滚阶段 A。 