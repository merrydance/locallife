# merchant-console ownership note

## 当前 owner 边界

- 受保护超级 service：weapp/miniprogram/services/merchant-console.ts
- 当前允许页面 owner：无。allowlist 已收缩到 0。

## 当前状态

- dashboard 聚合 owner 已迁移到 `weapp/miniprogram/services/merchant-dashboard.ts`。
- storefront status owner 已迁移到 `weapp/miniprogram/services/merchant-open-status.ts`。
- applyment owner 已迁移到 `weapp/miniprogram/services/merchant-applyment-console.ts`。
- app bind owner 已迁移到 `weapp/miniprogram/services/merchant-app-bind.ts`。
- `merchant-console.ts` 仅保留为受保护占位文件，阻止旧导入路径被悄悄复活。

## 收口原则

- 新增商户能力不得重新挂回该文件；应直接落到新的任务域 service 或页面组 owner。
- 若有人试图恢复 `services/merchant-console.ts` 页面导入，super-service gate 应直接阻断。
- 若必须改动该占位文件或重新定义边界，提交中必须同步更新本说明。

## 当前风险

- 历史超级 service 路径已经收口，但真正的风险还在于新 owner 文件未来是否再次被无边界吸收。
- 该 note 的目标从“防膨胀”升级为“阻止旧路径复活”。