# 商户侧配置中心闭环交付任务清单

日期：2026-04-04

适用范围：`weapp/miniprogram/pages/merchant/config/**` 及其从配置中心进入的商户侧一到四级页面。

目标：把“整个 weapp 商户侧配置中心不满意”收敛成一个可由闭环工作流逐项执行的任务列表，按 开发 -> 审查 -> 修复 -> 复审 -> 文档同步 的顺序推进，直到任务清单完成。

## 1. 本轮范围

纳入本轮的页面与入口：

- `merchant/config`
- `merchant/settings/profile`
- `merchant/profile-images`
- `merchant/merchant-categories`
- `merchant/settings/business-hours`
- `merchant/settings/membership`
- `merchant/settings/recharge-rules`
- `merchant/settings/packaging-policy`
- `merchant/settings/application`
- `merchant/settings/applyment`
- `merchant/settings/applyment/completed`
- `merchant/finance`
- `merchant/settings/display-config`
- `merchant/printers`
- `merchant/tables`
- `merchant/discount-rules`
- `merchant/delivery-promotions`
- `merchant/vouchers`
- `merchant/staff`

与配置中心信息架构直接相关、但是否保留入口需在任务中判定的页面：

- `merchant/orders/print-anomalies`
- `merchant/inventory`
- `merchant/group/application`
- `merchant/group/join`
- `merchant/reviews`

## 2. 本轮不直接追求的目标

- 不在本轮顺手重构整个商户端工作台、订单、后厨或统计域。
- 不在本轮为了配置中心而改动无关角色页面。
- 不在本轮大规模重写所有 shared components，除非配置中心问题无法通过局部共享能力收口。

## 3. 全局验收标准

整组任务完成后，应同时满足：

1. 配置中心入口与下游页面的信息架构清晰，页面边界不再混乱。
2. 同类页面的顶部间距、底部安全区、弹层底部按钮、底部主操作样式一致。
3. TDesign 组件使用一致，不再存在无必要魔改和风格漂移。
4. 聚合页、入口页、设置页、规则页的请求预算和预加载策略可解释、可回归。
5. 所有保存类、提交流、上传流页面的结果承接一致，不再出现提示混乱或重复提示。
6. 页面说明文案简洁，用户能快速理解当前状态和下一步动作。
7. 页面能力与后端真实契约对齐，不再出现前端实现了页面但没与后端能力闭环的情况。

## 4. 任务顺序

### Task 1: 配置中心信息架构与入口边界重整（已接受）

目标：

- 收口 `merchant/config` 作为配置导航页的角色。
- 清理入口分类、页面归属、缺失入口和错误入口。

重点路径：

- `weapp/miniprogram/pages/merchant/config/**`
- 相关跳转页的入口映射

必须解决：

- 配置中心入口标题、副标题、区块层级不清晰。
- 店铺设置、主体与结算、设备与展示、营销与拉新、协作入口边界混乱。
- 缺失库存管理入口。
- `print-anomalies` 是否应继续作为配置中心入口需要明确结论。
- 平台设置、商户设置、独立页面边界不清晰的问题要给出最终收口方案。

验收要点：

- 配置中心成为清晰的设置导航页，而不是杂糅入口页。
- 所有入口都能回答“为什么在这里”。
- 不再把异常任务、低相关页面、跨域页面强塞进配置中心。

当前结论（2026-04-04）：

- `merchant/config` 已收口为清晰的配置导航页。
- 已补齐 `merchant/inventory` 的配置中心入口。
- `merchant/orders/print-anomalies` 不再作为配置中心主入口。
- `merchant/group/application` 与 `merchant/group/join` 归入配置中心的协作边界。
- 商户工作台不再承载集团静态目录。

### Task 2: 配置中心共享规范收口包（已接受）

目标：

- 把配置中心多页反复出现的布局与交互问题收口为统一模式。

重点路径：

- `merchant/config`
- `merchant/settings/*`
- `merchant/tables`
- `merchant/discount-rules`
- `merchant/delivery-promotions`
- `merchant/vouchers`

必须解决：

- 顶部间距不统一。
- 页面底部安全区不统一。
- 弹层底部按钮大小、样式、留白不统一。
- 规则类页面新增入口形态不统一。
- 按钮、标签、说明文字层级不统一。

验收要点：

- 同类页面一眼能看出是同一个系统。
- 弹层底部按钮不再小、挤、被截断。
- 右下角新增按钮规则在所有适用页面统一生效。

当前结论（2026-04-04）：

- merchant config 相关 sibling 页顶部间距已统一。
- 页面底部安全区已统一处理。
- 弹层底部双按钮已收口为统一模式。
- 规则类页面右下角新增入口已统一补齐。
- 按钮层级已收口为统一模式。

### Task 3: 配置中心提示系统与页面说明文案统一（已接受）

目标：

- 收口配置中心范围内的反馈通道、提示节奏和说明文案。

重点路径：

- 全部配置中心页面，优先保存类、提交类、上传类、状态页

必须解决：

- 一闪而过但用户不理解的提示。
- 应该有提示却没有提示。
- 应该有结果页或页面承接结果，却只弹一下提示。
- 同一页面同时存在顶部横幅、页内横幅、Toast、Modal 重复提示。
- 页面说明、标题、副标题、提示区重复解释，导致越看越糊涂。

验收要点：

- 短时动作默认 Toast，确认和解释默认 Modal，长期结果默认页面状态或结果区。
- 不再出现同一结果多通道重复表达。
- 页面说明文字明显更短、更直接、更像任务说明而不是堆文案。

当前结论（2026-04-04）：

- 主体申请页提交结果只由状态区承接，OCR 完成和定位回写不再额外叠加成功提示。
- business-hours、membership、packaging-policy、display-config 四个保存类设置页的成功结果已统一为页内 notice，文案更短、更直接。
- 配置中心范围内提示通道已进一步收口，重复表达已减少。

### Task 4: 请求策略、预加载与聚合页性能治理（已接受）

目标：

- 统一配置中心及其下游页面的请求时机、`onShow` 策略、预加载边界和主任务预算。

重点路径：

- `merchant/config`
- 入口页、聚合页、资料页、设置首页、资金页、设备页

必须解决：

- 页面打开就全量拉取无关数据。
- 不该预加载却预加载子页面或其他角色接口。
- `onShow` 无条件重拉重型数据。
- 聚合页主任务还没稳定，就先拉多个低频区块。

验收要点：

- 配置中心与相关入口页有明确主任务请求预算。
- 预加载都有明确收益，不再基于“也许会用到”。
- 弱网下可以安全退化，不会因为预热失败破坏当前任务。

当前结论（2026-04-04）：

- dashboard 的主任务刷新与经营洞察刷新已拆开，`onShow` 改为 freshness gate + 轻量营业状态同步。
- `merchant/settings/packaging-policy` 首屏不再把策略和全量菜品候选绑成同一主链路，失败态不再伪装成空态。
- `merchant/settings/application`、`merchant/settings/business-hours`、`merchant/settings/membership`、`merchant/settings/display-config`、`merchant/printers` 的 `onShow` 自动刷新已改为有门槛策略，不再每次返回都整页重拉。
- `merchant/printers` 已去掉当前页面未消费的对账任务默认请求。

### Task 5: 店铺资料与图资链路修复包（已接受）

目标：

- 收口店铺资料、图片管理和经营类目相关体验与契约问题。

重点路径：

- `merchant/settings/profile`
- `merchant/profile-images`
- `merchant/merchant-categories`

必须解决：

- 店铺地址调用能力错误。
- 经纬度不应手工输入，应通过定位或明确辅助流程获取。
- 上传图片导致整页刷新。
- 上传后无即时预览。
- 页面说明、字段命名、保存反馈需要统一。

验收要点：

- 上传后即时预览可见，且不触发整页刷新。
- 资料页字段与后端契约对齐，输入方式符合真实业务。
- 保存后的结果能被页面承接，而不是只靠闪一下的提示。

当前结论（2026-04-05）：

- 店铺资料页已改为通过定位辅助流程维护经营位置，不再手工输入经纬度。
- 店铺资料保存结果已由页内结果承接。
- `merchant/profile-images` 不再依赖回页整刷；Logo 删除已形成前后端持久化闭环；慢地址图片会保留 pending 预览并继续 finalize。
- `merchant/merchant-categories` 文案已收回到真实平台类目边界。

### Task 6: 营业与会员规则页修复包（已接受）

目标：

- 收口营业时间、会员设置、充值规则、包装费策略四类页面。

重点路径：

- `merchant/settings/business-hours`
- `merchant/settings/membership`
- `merchant/settings/recharge-rules`
- `merchant/settings/packaging-policy`

必须解决：

- 保存反馈不足。
- 页面状态回写不清晰。
- 包装费策略参数错误。
- 规则类页面新增入口、弹层按钮、表单状态不统一。

验收要点：

- 保存类页面都具备 提交中 -> 成功/失败 -> 页面回写 的清晰闭环。
- 包装费策略与后端能力对齐，不再存在参数错误。
- 规则页体验与新增入口模式统一。

当前结论（2026-04-05）：

- 本轮实际收口集中在 `merchant/settings/recharge-rules`。
- `merchant/settings/recharge-rules` 首屏失败已改为页内 `initialError` + 重试入口，不再以 Toast-only 失败伪装空态。
- `merchant/settings/recharge-rules` 的 create / edit / toggle / delete 成功后会先本地回写，再静默重同步；后续同步失败只在页内显示 `refreshErrorMessage`，不再误报为动作失败。

### Task 7: 主体申请、收付通与资金账户闭环包（已接受）

目标：

- 收口主体申请、进件、签约、银行资料重提与资金账户页面。

重点路径：

- `merchant/settings/application`
- `merchant/settings/applyment`
- `merchant/settings/applyment/completed`
- `merchant/finance`

必须解决：

- 主体申请状态只有灰块、缺少可理解文案。
- 主体申请和进件流的结果承接、重提、草稿、回流状态要统一。
- 资金账户页面底部按钮和安全区问题。
- 资金账户页面设计混乱、导航返回缺失。

验收要点：

- 状态不再只靠色块表达。
- 进件和主体申请与后端契约严格对齐。
- 资金账户页面可读、可返回、可理解、可操作。

当前结论（2026-04-05）：

- `merchant/finance` 已按真实后端状态消费 `not_applied`、`rejected_sign`、`frozen` 等状态，不再把“无进件记录”误判成已有进件。
- `merchant/finance` 各财务分区在失败时不再伪装为空态或零值；已有可信数据时会保留上次结果并在页内提示刷新失败，无可信数据时按分区显示错误承接。
- `merchant/settings/applyment/completed` 返回 `merchant/settings/applyment` 时不再自循环，已支持回看进件详情。
- `merchant/settings/applyment` 在 `onShow`、下拉刷新和手动刷新失败时会保留当前上下文，并落到页内 `refreshErrorMessage`，不再只靠瞬时提示承接。
- 本任务范围内的 outline 按钮/标签与设计漂移已收口，`merchant/settings/applyment`、`merchant/settings/application`、`merchant/finance` 与 `components/applyment-bank-form` 已对齐当前配置中心交互规范。

### Task 8: 设备与展示设置收口包（已接受）

目标：

- 收口显示与打印设置、打印机管理、桌台管理三类页面。

重点路径：

- `merchant/settings/display-config`
- `merchant/printers`
- `merchant/tables`

必须解决：

- 显示与打印设置缺少页面级反馈。
- KDS 地址字段含义不清。
- 打印机管理边界不清，不应混入异常对账任务入口。
- 桌台管理位置不合理、弹层按钮太小、图片可上传不可预览、标签无法取消、二维码能力缺失。

验收要点：

- 设备设置边界清晰。
- 显示与打印设置的说明文案可理解。
- 桌台管理具备图片预览、二维码与合理的交互结构。

当前结论（2026-04-05）：

- `merchant/tables` 已区分首屏加载失败与已有数据后的刷新失败；当页面已持有可信列表时，失败会落到页内 `refreshErrorMessage`，不再把刷新失败伪装成首屏失败。
- `merchant/tables` 在 tab 切换同步失败时会回退到上次已加载 tab，不再出现“新 tab + 旧列表”错配。
- `merchant/printers` 主页面已移除 `print-anomalies` 入口，设备页边界只保留打印机配置与状态相关操作。
- `merchant/printers` 被 review 点名的 outline 漂移已收口，页面操作与场景标签已回到当前配置中心交互规范。

### Task 9: 营销配置页收口包（已接受）

目标：

- 收口满减规则、配送优惠、代金券管理三类营销配置页。

重点路径：

- `merchant/discount-rules`
- `merchant/delivery-promotions`
- `merchant/vouchers`

必须解决：

- 新建入口形态不统一。
- 弹层底部按钮太小、间距不足。
- 配送优惠缺删除和停用能力。
- 代金券适用订单类型无法选择。

验收要点：

- 规则管理类页面的新增、编辑、删除、启停交互统一。
- 代金券与配送优惠的能力和后端契约闭环。

当前结论（2026-04-05）：

- `merchant/discount-rules`、`merchant/delivery-promotions`、`merchant/vouchers` 三页都已补齐“首屏失败”和“已有结果后的刷新失败”分流；首屏失败在页内可见并可重试，刷新失败会保留当前结果并在页内提示。
- create / edit / toggle / delete 成功后，页面会先本地写回，再做静默重同步；后续同步失败不再回滚刚完成的动作，也不再误报为动作失败。
- `merchant/discount-rules` 与 `merchant/vouchers` 已接入可管理的分页与继续加载；针对错误 `total` 契约和整页倍数末页场景，已加入 lookahead / 缓存兜底，并在弱网下安全降级。
- `merchant/delivery-promotions` 的弹层底部动作已收口为显式等宽动作行，避免按钮过小或底部承接不清。
- `merchant/vouchers` 的 0 门槛文案已修正，页面说明与真实业务语义对齐。

### Task 10: 员工与协作入口收口包（已接受）

目标：

- 收口员工管理及与配置中心相关的协作入口表达。

重点路径：

- `merchant/staff`
- `merchant/config`
- `user/bind-merchant`
- 如有必要再覆盖 `merchant/group/application`、`merchant/group/join`

必须解决：

- 邀请店员不应直接展示字符串，应提供二维码或清晰可用的邀请承接。
- 配置中心中协作入口边界和文案需要更清晰。

验收要点：

- 员工邀请方式符合移动端实际使用场景。
- 协作入口不会和店铺配置、资金配置混成一类。

当前结论（2026-04-05）：

- `merchant/staff` 邀请弹层已改为二维码主路径，明确引导店员从“我的”页头像卡右上角扫码入口加入；邀请码退为备用承接。
- `merchant/staff` 已接入商户权限校验分流，权限拒绝、校验失败、首屏加载失败和已有数据后的刷新失败已分层承接；页内按钮与弹层底部动作已收口到当前配置中心按钮系统。
- `merchant/config` 的“人员与协作”已拆清为门店成员协作与品牌 / 集团合作两层，不再把门店员工与品牌合作入口混成同一类配置项。
- `user/bind-merchant` 成功页岗位文案已产品化；扫码协议优先级已与用户中心扫码入口对齐，并补齐 Web 登录确认所需的 `code` / `sig` / `ts` 透传。

### Task 11: 最终一致性复核与配置中心验收（已接受）

目标：

- 对配置中心全域做一次最后的交互、契约、性能、提示系统和设计一致性复核。

必须解决：

- 同类问题是否已在所有相关页面收口，而不是只修了一个样板页。
- 是否还存在跨页风格漂移、提示混乱、入口边界混乱、请求策略失控。

验收要点：

- 配置中心作为一个系统被统一，而不是若干单页各自修过一遍。

当前结论（2026-04-05）：

- 配置中心全域最终一致性 sweep 已完成；前序 Task 1 至 Task 10 的交互、契约、提示系统、请求策略与设计收口点已完成最终复核，当前范围内未再发现新的系统性回退项。
- `merchant/profile-images` 的弱网删图、失败重试与 4xx 收敛链已在最终 sweep 中复核通过，图片管理的结果承接与异常回收链路已闭环。
- `merchant/settings/applyment/completed` 在 `completed=false` 的兜底态现仅保留一个返回动作，结果页不再保留并列主动作造成的回流分叉。
- 配置中心作为一个系统的最终验收已通过，本轮关键收口点已与前序 Task 9 / Task 10 的 accepted 记录口径保持一致。
- 本轮状态已更新为 accepted / docs synced。

## 5. 建议验证策略

每个任务默认遵循：

- 最小相关代码校验
- 相关页面路径人工走查
- 审查阶段明确指出残余风险
- 文档同步仅在行为、边界、契约或流程确有变化时执行

建议默认验证范围：

- `npm run quality:check` 为优先
- 若任务只影响少量页面，可使用更小范围的 lint 或 compile 验证

## 6. 建议交给闭环工作流的输入模板

可直接把以下内容交给 `任务清单闭环执行`：

```text
Target area: weapp

Ordered task list:
1. 配置中心信息架构与入口边界重整
2. 配置中心共享规范收口包
3. 配置中心提示系统与页面说明文案统一
4. 请求策略、预加载与聚合页性能治理
5. 店铺资料与图资链路修复包
6. 营业与会员规则页修复包
7. 主体申请、收付通与资金账户闭环包
8. 设备与展示设置收口包
9. 营销配置页收口包
10. 员工与协作入口收口包
11. 最终一致性复核与配置中心验收

Acceptance criteria:
- 配置中心全域完成统一收口，满足本文第 3 节的全局验收标准

Validation scope or budget:
- 优先最小相关校验；涉及多页联动时使用 npm run quality:check

Documentation expectations:
- 仅在行为、页面边界、长期规则或流程变化时同步文档

Stop conditions:
- 同一任务连续两轮 修复 -> 复审 后仍无法通过，则停止并明确 blocker
```

## 7. 备注

- 本清单默认先收口配置中心作为“系统”的一致性，再深入各域页面。
- 如果执行过程中发现某一任务过大，可在闭环流程中继续拆成子任务，但应保持当前任务顺序不被打乱。