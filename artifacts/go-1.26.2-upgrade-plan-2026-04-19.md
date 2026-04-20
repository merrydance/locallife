# Go 1.26.2 升级方案

## 1. 结论

当前不建议把项目直接切到 Go 1.26.2 并和业务改动一起推进。

原因不是代码一定不兼容，而是虽然仓库当前的 Go 基线和执行入口已经统一，但 Go 1.26.2 升级仍然应该作为独立工程变更处理：

1. 模块声明已经是 Go 1.25.8。
2. CI 和生成检查脚本现在都已跟随当前环境与 go.mod 提供的工具链。
3. Go 1.26.2 升级仍需要单独做兼容性验证和完整回归矩阵。

所以正确顺序应当是：

1. 先保持当前 1.25.8 基线稳定。
2. 再单独评估是否升级到 Go 1.26.2。
3. 升级后跑完整验证矩阵。

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

如果阶段 A 验证稳定，再做第二个提交：

1. 修改 [locallife/go.mod](locallife/go.mod#L3) 到 1.26.2。
2. 所有 workflow 继续通过 go-version-file 跟随 go.mod，避免手工散点配置。
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

如果阶段 B 只打算先做兼容性验证、不立即切主分支，也建议至少完成前 6 项。

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

如果你现在想尽快降低风险，我建议保持阶段 A 的结果，不要马上切到 1.26.2。

如果你明确希望全仓库跟你本地版本保持一致，那就按上面的两阶段走，我建议把阶段 B 当作一次独立工程变更来处理，而不是混进任何业务需求。