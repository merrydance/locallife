# Go 1.26.2 升级方案

## 1. 结论

当前不建议把项目直接切到 Go 1.26.2 并和业务改动一起推进。

原因不是代码一定不兼容，而是仓库当前的 Go 基线和执行入口还没有完全统一：

1. 模块声明已经是 Go 1.25.8。
2. CI 主链大多跟随 go.mod。
3. 生成脚本和部分 workflow 仍硬编码 /usr/local/go/bin。
4. 安全文档里还写死了 GOTOOLCHAIN=go1.25.8。

所以正确顺序应当是：

1. 先统一仓库内部 Go 基线。
2. 再单独评估是否升级到 Go 1.26.2。
3. 升级后跑完整验证矩阵。

## 2. 当前版本锚点

当前仓库里的关键版本绑定点如下：

1. 模块版本声明在 [locallife/go.mod](locallife/go.mod#L3)。
2. backend safety workflow 跟随 go.mod，但仍强行注入 /usr/local/go/bin，在 [backend-safety workflow](.github/workflows/backend-safety.yml#L45) [backend-safety workflow](.github/workflows/backend-safety.yml#L71) [backend-safety workflow](.github/workflows/backend-safety.yml#L80)。
3. 生成物检查脚本直接写死了 /usr/local/go/bin，在 [locallife/scripts/check_generated.sh](locallife/scripts/check_generated.sh#L4) 和 [locallife/scripts/check_generated.sh](locallife/scripts/check_generated.sh#L10)。
4. 安全文档中的 govulncheck 命令固定为 go1.25.8，在 [UNREACHABLE_DEPENDENCY_RISK_REGISTER](.github/standards/engineering/UNREACHABLE_DEPENDENCY_RISK_REGISTER.md#L70)。

## 3. 推荐策略

推荐把升级拆成两个独立阶段。

### 阶段 A：统一现有基线

目标：先把仓库从“声明 1.25.8，部分实际跑系统 Go 或混用路径”统一到“全部按同一版本运行”。

需要处理：

1. 去掉 workflow 和脚本里对 /usr/local/go/bin 的硬编码依赖。
2. 文档中的工具链命令改为跟随 go.mod，或至少同步到同一版本。
3. 本地和 CI 都以同一版本执行测试与生成步骤。

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

建议单独建一个提交，内容只包含以下几类修改：

1. 把 [locallife/scripts/check_generated.sh](locallife/scripts/check_generated.sh#L4) 和 [locallife/scripts/check_generated.sh](locallife/scripts/check_generated.sh#L10) 改成使用当前环境里的 go，而不是固定 /usr/local/go/bin/go。
2. 把 [backend-safety workflow](.github/workflows/backend-safety.yml#L80) 中对 /usr/local/go/bin 的 PATH 注入去掉，或改成仅依赖 setup-go 提供的工具链。
3. 把 [UNREACHABLE_DEPENDENCY_RISK_REGISTER](.github/standards/engineering/UNREACHABLE_DEPENDENCY_RISK_REGISTER.md#L70) 的命令改成与当前统一基线一致。

完成后应验证：

1. 本地 clean shell 下 make test-unit 能过。
2. 本地 clean shell 下 make check-generated 能过。
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
2. 脚本里对 /usr/local/go/bin 的依赖会绕开 setup-go，重新引入“版本看起来对、实际编译链不一致”的问题。
3. 如果团队成员本地已分散到 1.25.x 和 1.26.x，多人协作期间会重新出现生成物差异和 cache 差异。

## 7. 建议的提交拆分

建议用两次提交，而不是一次完成：

1. chore: unify go toolchain baseline across repo
2. chore: upgrade backend toolchain to go 1.26.2

这样如果第二步出现兼容性问题，第一步仍然有价值，而且不会影响这次已经提交的食安业务变更。

## 8. 我的建议

如果你现在想尽快降低风险，我建议先做阶段 A，不要马上切到 1.26.2。

如果你明确希望全仓库跟你本地版本保持一致，那就按上面的两阶段走，我建议把阶段 B 当作一次独立工程变更来处理，而不是混进任何业务需求。