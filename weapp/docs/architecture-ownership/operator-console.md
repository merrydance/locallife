# operator-console ownership note

## 当前 owner 边界

- 受保护超级 service：weapp/miniprogram/services/operator-console.ts
- 当前允许页面 owner：无。allowlist 已收缩到 0。

## 当前状态

- 原 dashboard owner 已迁移到 `weapp/miniprogram/services/operator-workbench.ts`。
- 原 analytics owner 已迁移到 `weapp/miniprogram/services/operator-analytics-dashboard.ts`。
- 原共享 region owner 已迁移到 `weapp/miniprogram/services/operator-regions.ts`。
- `operator-console.ts` 仅保留为受保护占位文件，阻止旧导入路径被悄悄复活。

## 收口原则

- 新增运营能力不得重新挂回该文件；应直接落到新的任务域 service 或页面组 owner。
- 若有人试图恢复 `services/operator-console.ts` 页面导入，super-service gate 应直接阻断。
- 若必须改动该占位文件或重新定义边界，提交中必须同步更新本说明。

## 当前风险

- 历史超级 service 路径已经收口，但真正的风险还在于新任务域 service 未来是否再次被无边界吸收。
- 该 note 的目标从“阻止继续膨胀”升级为“阻止旧路径复活”。

## 后续拆分

- 具体拆分顺序与目标 owner 见 `weapp/docs/architecture-ownership/operator-console-split-plan.md`。