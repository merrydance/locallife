# Weapp 宝付开户页面体验与状态承接修复计划

日期：2026-05-10

风险等级：G3 - 资金、身份、银行账户、支付确认、异步开户状态恢复。

## 1. 背景

本计划承接对小程序端宝付开户页面的审查，范围覆盖平台、商户、运营商、骑手四类角色的宝付二级户开户界面。

当前页面共同存在一个根因：把“状态承接页”“资料提交页”“支付/开户等待态”“最终结果展示”压在同一个页面里，导致 UI 和状态语义同时漂移。

已确认的问题包括：

1. 资料提交页面支持下拉刷新，且 `onShow` 会静默重拉账户状态，可能覆盖用户正在填写的本地草稿。
2. 页面 UI 过度依赖文字 cell 和 input label，长 label 在小屏中换行，表单像字段面板。
3. 提交页顶部展示“开户状态/核验费”区块，信息价值低，且状态应由专门状态页承接。
4. `applyment-bank-form` 在已有 page gutter 内又使用 card 样式，形成额外边距和双重卡片感。
5. 企业户银行省市/支行选择被 `requireBankBranch=true` 强制常显，没有按银行目录返回的 `need_bank_branch` 按需展示。
6. 底部按钮出现多个中号按钮上下排列，提交、继续支付、刷新状态同级出现，不符合非顾客侧表单页动作层级。
7. 长耗时动作依赖 loading Toast 和结果 Toast；页面 finally 又会清掉同一个 toast selector，造成结果提示闪断或被隐藏。
8. 最终状态缺少专门承接页面/组件，处理中、成功、失败、未知结果没有形成统一的后端真值驱动表达。

本计划的目标是按后端唯一真值重塑页面边界和组件边界，而不是局部修样式。

## 2. 后端真值原则

小程序只消费后端返回的开户事实，不发明状态、阶段或成功结论。

后端契约入口：

- `weapp/miniprogram/api/baofu-account.ts`
- `GET /v1/merchant/settlement-account`
- `POST /v1/merchant/settlement-account`
- `GET /v1/operators/me/settlement-account`
- `POST /v1/operators/me/settlement-account`
- `GET /v1/platform/finance/settlement-account`
- `POST /v1/platform/finance/settlement-account`
- `GET /v1/rider/settlement-account`
- `POST /v1/rider/settlement-account`

前端状态只来自：

- `status`
- `state`
- `status_desc`
- `open_state`
- `profile_status`
- `missing_fields`
- `flow_id`
- `flow_state`
- `verify_fee_amount`
- `payment`
- `payment_ready`
- `profile_defaults`
- `submitted_at`
- `updated_at`

硬约束：

- 不以 `wx.requestPayment` 成功回调作为业务成功。
- 不以提交 API 返回作为最终开户成功，除非后端返回的 `status` 本身已经是终态。
- 不用本地 `setData` 假装资料已提交、支付已成功、开户已开通。
- 支付、提交、恢复、刷新后的最终页面状态必须通过后端回查或后端返回账户对象构建。
- 敏感资料只按后端 `profile_defaults` 的 `has_*` 与 mask 字段展示，不在页面构造未返回的敏感真值。

## 3. 目标页面架构

### 3.1 页面职责

每个角色统一拆成三个职责层：

| 职责 | 页面/组件 | 能力 |
| --- | --- | --- |
| 状态页 | 现有 `settlement-account/index` | 首屏状态、终态展示、下一步动作、刷新状态、恢复 pending workflow |
| 提交页 | 新增 `settlement-account/submit/index` 或等价路径 | 只填写资料并提交，不显示顶部状态摘要，不支持下拉刷新 |
| 等待/结果承接 | 新增共享组件或页面 | 提交后、支付后、恢复中、轮询中、未知结果、最终结果 |

状态页保留原入口路径，避免破坏现有跳转：

- `pages/platform/finance/settlement-account/index`
- `pages/merchant/finance/settlement-account/index`
- `pages/operator/finance/settlement-account/index`
- `pages/rider/settlement-account/index`

新增提交页建议路径：

- `pages/platform/finance/settlement-account/submit/index`
- `pages/merchant/finance/settlement-account/submit/index`
- `pages/operator/finance/settlement-account/submit/index`
- `pages/rider/settlement-account/submit/index`

如实现时发现小程序分包路径或构建约束不适合嵌套目录，可以选择 `settlement-account-submit/index`，但必须在计划执行记录中明确实际路径，并更新 `app.json`。

### 3.2 角色差异

| 角色 | 账户类型 | 核验费 | 提交页字段 | 银行表单 |
| --- | --- | --- | --- | --- |
| platform | business | 平台承担，不走用户支付 | 主体资料、法人、联系人、企业银行账户 | 复用 `applyment-bank-form` |
| merchant | business | 平台承担，不走用户支付 | 主体资料、法人、联系人、企业银行账户 | 复用 `applyment-bank-form` |
| operator | personal | 用户侧先直连支付核验费 | 姓名、身份证、银行卡号、预留手机号、开户银行可选 | 使用个人户表单组件/页面 |
| rider | personal | 用户侧先直连支付核验费 | 姓名、身份证、银行卡号、预留手机号、开户银行可选 | 使用个人户表单组件/页面 |

### 3.3 状态页目标结构

状态页使用 TDesign-first 结构：

1. 首屏 loading/error/empty 由 `t-loading` / `t-empty` 承接。
2. 主体状态使用 `t-result` 或 `t-cell-group + t-tag + t-steps`，具体由状态密度决定。
3. 主操作只有一个：
   - `profile_pending` / 可提交：进入提交页。
   - `verify_fee_pending`：继续支付核验费。
   - `processing`：刷新状态或等待。
   - `ready`：查看账户信息。
   - `failed`：进入提交页修正资料或联系平台，取决于后端能力。
4. 刷新状态只在状态页出现，不放在提交页底部。
5. 已有可信状态时刷新失败，保留旧状态并展示局部错误。

### 3.4 提交页目标结构

提交页直接进入任务：

1. 无顶部“开户状态/核验费”摘要区。
2. 无下拉刷新。
3. 无状态刷新按钮。
4. 底部内容流只保留一个全宽主提交按钮。
5. 长 label 收短或使用 `layout="vertical"`。
6. 表单错误贴近字段或用一条主反馈，不和 Toast 重复。
7. 提交中禁用所有输入与主按钮，防止重复提交。
8. 提交成功后进入等待承接，不用 Toast 作为最终结果。

### 3.5 等待与结果承接

长耗时过程包括：

- 提交开户资料后宝付返回处理中。
- 骑手/运营商支付核验费后等待支付确认和开户状态推进。
- 重新进入页面时恢复 pending workflow。
- 手动刷新时后端暂未同步终态。

目标组件或页面必须表达：

| 状态 | 展示 | 动作 |
| --- | --- | --- |
| submitting | 页面/组件内 loading，不用短 Toast 停留 | 禁用重复提交 |
| payment_confirming | 支付结果确认中 | 等待、回状态页 |
| opening_processing | 开户处理中 | 轮询/刷新 |
| pending_confirmation | 状态同步中，结果未知 | 稍后刷新、返回状态页 |
| ready | 结算账户已开通 | 返回状态页展示终态 |
| failed | 后端失败原因 | 重新提交或联系平台 |

## 4. 文件结构计划

### 4.1 共享 API 与服务

修改：

- `weapp/miniprogram/api/baofu-account.ts`
  - 保持后端 DTO 为唯一字段来源。
  - 如需新增展示字段，只允许来自后端响应，不添加前端伪字段。
- `weapp/miniprogram/services/baofu-account-role-page.ts`
  - 收敛四角色状态页 view model。
  - 新增明确的 status page action model。
- `weapp/miniprogram/services/baofu-account-onboarding.ts`
  - 保留支付/提交/轮询工作流 owner。
  - 输出适合等待组件消费的 workflow result。
  - 去除页面需要重复解释的结果文案分支。

建议新增：

- `weapp/miniprogram/services/baofu-account-status-view.ts`
  - 负责把 `BaofuSettlementAccountResponse` 转成状态页 view model。
- `weapp/miniprogram/services/baofu-account-submit-view.ts`
  - 负责四角色提交页默认资料、字段显示、mask 与表单初始态。

如果实现时能在 `baofu-account-role-page.ts` 内保持文件清晰，也可以不新增两个文件；但不得继续在四个页面内复制状态解释。

### 4.2 共享组件

修改：

- `weapp/miniprogram/components/applyment-bank-form/index.*`
  - 增加嵌入式样式模式。
  - 增加全宽主按钮布局。
  - 默认按 `form.need_bank_branch` 显示省市支行。
  - 不再让业务页通过 `requireBankBranch=true` 强制支行常显。

建议新增：

- `weapp/miniprogram/components/baofu-onboarding-status/index.*`
  - 状态页主体承接组件，展示 status/result/steps/actions。
- `weapp/miniprogram/components/baofu-onboarding-wait/index.*`
  - 等待/处理中/未知结果承接组件。

组件边界：

- 组件只接收 view model 和事件。
- 组件不直接请求接口。
- 组件不做路由跳转。
- Page 或 workflow owner 负责请求、轮询、恢复、跳转。

### 4.3 页面

状态页改造：

- `weapp/miniprogram/pages/platform/finance/settlement-account/index.*`
- `weapp/miniprogram/pages/merchant/finance/settlement-account/index.*`
- `weapp/miniprogram/pages/operator/finance/settlement-account/index.*`
- `weapp/miniprogram/pages/rider/settlement-account/index.*`

提交页新增：

- `weapp/miniprogram/pages/platform/finance/settlement-account/submit/index.*`
- `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.*`
- `weapp/miniprogram/pages/operator/finance/settlement-account/submit/index.*`
- `weapp/miniprogram/pages/rider/settlement-account/submit/index.*`

路由更新：

- `weapp/miniprogram/app.json`

样式：

- `weapp/miniprogram/styles/baofu-settlement-account.wxss`
  - 仅保留共享 page shell 和状态/等待局部样式。
  - 不放业务页特有的 card margin hack。

## 5. 任务卡

每个任务卡执行顺序固定：实现 -> 最小验证 -> review -> 修复 review finding -> 再验证。未完成验证不能标记完成。

### BAOFU-WEAPP-01 契约矩阵与状态枚举收口

风险：G3。

目标：冻结前端允许识别的后端状态和页面动作，避免后续页面继续猜状态。

范围：

- `weapp/miniprogram/api/baofu-account.ts`
- `weapp/miniprogram/services/baofu-account-role-page.ts`
- 必要时新增 `weapp/miniprogram/services/baofu-account-status-view.ts`

实施要求：

1. 列出 `BaofuSettlementAccountStatus` 到页面状态的映射：
   - `profile_pending` -> 可提交资料。
   - `verify_fee_pending` -> 可继续支付。
   - `verify_fee_processing` -> 支付确认中。
   - `opening_processing` -> 开户处理中。
   - `merchant_report_processing` -> 商户报备中。
   - `applet_auth_pending` -> 授权目录绑定中。
   - `ready` -> 开通成功。
   - `failed` -> 开通失败。
   - `voided` -> 流程作废。
   - unknown -> 同步中，不能展示成功或失败。
2. 输出统一 view model：
   - `statusText`
   - `statusDesc`
   - `tagTheme`
   - `resultTheme`
   - `primaryAction`
   - `secondaryAction`
   - `canSubmitProfile`
   - `canContinuePayment`
   - `canRefreshStatus`
   - `isTerminal`
   - `isWaiting`
3. 页面 action 只由后端状态和 workflow result 决定。
4. 不新增后端没有的业务阶段。

验收：

- 四角色状态页不再各自解释同一个后端状态。
- `ready/failed/voided/processing/profile_pending/verify_fee_pending` 均有明确页面动作。
- unknown 状态只允许显示同步中和刷新动作。

验证：

- `PATH="$HOME/.local/bin:$PATH" npm run compile`
- `PATH="$HOME/.local/bin:$PATH" npm run lint:all`

Review checkpoint：

- 检查是否有页面本地 switch 再解释 `status`。
- 检查是否有前端新增伪状态。

### BAOFU-WEAPP-02 银行表单组件修复

风险：G2/G3。组件承接银行账户和敏感资料输入。

目标：修复银行表单的卡片边距、按钮层级、省市支行按需展示和 label 密度。

范围：

- `weapp/miniprogram/components/applyment-bank-form/index.wxml`
- `weapp/miniprogram/components/applyment-bank-form/index.ts`
- `weapp/miniprogram/components/applyment-bank-form/index.wxss`
- 所有使用方：
  - `weapp/miniprogram/pages/merchant/settings/applyment/submit/index.wxml`
  - `weapp/miniprogram/pages/merchant/settings/applyment/settlement-account/index.wxml`
  - `weapp/miniprogram/pages/merchant/finance/settlement-account/index.wxml`
  - `weapp/miniprogram/pages/platform/finance/settlement-account/index.wxml`

实施要求：

1. 新增组件属性：
   - `embedded`: 默认 `false`。为 `true` 时不使用额外 card 视觉壳，跟随父页面内容节奏。
   - `submitBlock`: 默认 `true`。主提交按钮默认全宽。
   - `showSubmitActions`: 默认 `true`。状态页只展示只读信息时可关闭。
2. 保留 `requireBankBranch` 作为少数后端强制场景的显式例外，但平台/商户宝付开户地址不得传 `true`。
3. 省市/支行显示条件收敛为：
   - 银行目录返回 `need_bank_branch=true` 后显示；
   - 或修改结算账户后端明确要求补支行时显示；
   - 其他情况下不显示。
4. 业务页不得用 `requireBankBranch=true` 让支行常显来掩盖表单逻辑。
5. 长 label 处理：
   - `开户地址省份` -> `开户省份`
   - `开户地址城市` -> `开户城市`
   - `开户支行` 保持。
   - 主体页长字段使用更短 label 或 `layout="vertical"`。
6. 组件底部只渲染一个主提交按钮和一个可选取消按钮；主按钮 `block`。
7. `form-actions` 不再 `justify-content:flex-end` 造成中号按钮右对齐。

验收：

- 平台/商户宝付提交页未选需支行银行前不显示省市支行。
- 需要支行的银行被选中后才显示省市支行。
- 银行表单不再产生额外双重 card/gutter。
- 主提交按钮全宽且只有一个主操作。

验证：

- `PATH="$HOME/.local/bin:$PATH" npm run compile`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:tdesign-boundary`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:non-consumer-ui-patterns`

Review checkpoint：

- 检查是否覆盖 TDesign 内部类。
- 检查是否仍有平台/商户宝付页传 `requireBankBranch="{{true}}"`。

### BAOFU-WEAPP-03 等待与结果承接组件

风险：G3。

目标：把长耗时等待、未知结果、终态结果从 Toast 中移出，形成可见、可恢复的页面结构。

范围：

- 新增 `weapp/miniprogram/components/baofu-onboarding-wait/index.json`
- 新增 `weapp/miniprogram/components/baofu-onboarding-wait/index.wxml`
- 新增 `weapp/miniprogram/components/baofu-onboarding-wait/index.ts`
- 新增 `weapp/miniprogram/components/baofu-onboarding-wait/index.wxss`
- 修改 `weapp/miniprogram/services/baofu-account-onboarding.ts`

组件输入：

- `state`: `submitting | payment_confirming | opening_processing | pending_confirmation | ready | failed | error`
- `title`
- `description`
- `primaryActionText`
- `secondaryActionText`
- `loading`

组件事件：

- `primary`
- `secondary`

实施要求：

1. 组件用 `t-loading`、`t-result`、`t-button` 承接，不新增本地装饰壳。
2. 组件不发请求、不轮询、不跳转。
3. Page 负责调用 `startBaofuAccountOnboarding`、`continueBaofuAccountPayment`、`pollBaofuSettlementAccountStatus`。
4. 服务层保留 loading toast 的地方需要逐步替换为组件状态；若保留短期兼容，页面不得再 show/hide 同一个 result toast。
5. `pending_confirmation` 明确表示未知结果，不是失败。

验收：

- 提交后不以 Toast 作为唯一等待反馈。
- 支付后等待状态可见。
- 超时/未知结果有“返回状态页/刷新状态”的下一步。
- 失败状态显示后端映射后的中文原因。

验证：

- `PATH="$HOME/.local/bin:$PATH" npm run compile`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:tdesign-component-declarations`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:component-policy`

Review checkpoint：

- 检查组件是否吸收了请求或路由副作用。
- 检查 unknown result 是否被误写成 failed。

### BAOFU-WEAPP-04 平台企业户页面拆分

风险：G3。

目标：平台 `settlement-account/index` 成为状态页；新增提交页承接平台主体资料和企业银行账户。

范围：

- 修改 `weapp/miniprogram/pages/platform/finance/settlement-account/index.*`
- 新增 `weapp/miniprogram/pages/platform/finance/settlement-account/submit/index.*`
- 修改 `weapp/miniprogram/app.json`

状态页要求：

1. 保留原入口路径。
2. 显示后端开户状态、后端 `status_desc`、下一步动作。
3. 只在状态页支持刷新状态。
4. 状态页可以支持下拉刷新，但刷新失败必须保留上次可信状态。
5. `profile_pending/failed` 主操作进入提交页。
6. `processing` 主操作为刷新状态或进入等待承接。
7. `ready` 显示成功结果和已提交账户摘要。

提交页要求：

1. 不配置 `enablePullDownRefresh`。
2. 不显示顶部“开户状态/核验费”区块。
3. 不在 `onShow` 静默刷新覆盖草稿。
4. 使用 `applyment-bank-form embedded submitBlock`。
5. 不传 `requireBankBranch=true`。
6. 主按钮只有一个：提交开户资料。
7. 提交成功后进入等待组件或返回状态页并触发后端状态刷新。
8. 表单草稿只在首次加载时用 `profile_defaults` 初始化；用户编辑后不得被静默覆盖。

验收：

- 平台提交页没有下拉刷新。
- 平台提交页底部只有一个全宽主按钮。
- 平台提交页无顶部状态摘要区。
- 平台企业银行支行按需显示。
- 提交后状态来自后端返回或后端回查。

验证：

- `PATH="$HOME/.local/bin:$PATH" npm run compile`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:page-shell`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:wxml-expression-safety`

Review checkpoint：

- 检查 `onShow` 是否覆盖草稿。
- 检查状态页和提交页职责是否混回去。

### BAOFU-WEAPP-05 商户企业户页面拆分

风险：G3。

目标：商户宝付结算账户页面按平台同一模式拆分，同时保留商户权限校验。

范围：

- 修改 `weapp/miniprogram/pages/merchant/finance/settlement-account/index.*`
- 新增 `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.*`
- 修改 `weapp/miniprogram/app.json`
- 不改动微信支付原进件页面组，除非是复用银行表单组件所需兼容。

实施要求：

1. 状态页保留 `ensureMerchantApplymentAccess()` 权限校验。
2. 权限失败、首屏失败、已有可信状态刷新失败分开承接。
3. 提交页无下拉刷新，无顶部状态区。
4. 提交页使用共享企业户表单逻辑，不复制平台页状态解释。
5. 企业户省市支行按需显示。
6. 与原微信支付开户 `pages/merchant/settings/applyment/submit/index` 的提交页体验对齐：直接进入资料任务，底部一个主提交按钮。

验收：

- 商户宝付状态页与微信支付开户状态页一样承担状态承接。
- 商户宝付提交页不再像状态页。
- 权限未通过时不能进入提交页执行提交。

验证：

- `PATH="$HOME/.local/bin:$PATH" npm run compile`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:role-contract`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:payment-workflow-boundary`

Review checkpoint：

- 检查商户权限边界是否只作为体验，不替代后端授权。
- 检查微信支付原进件页面是否被意外改坏。

### BAOFU-WEAPP-06 运营商个人户页面拆分

风险：G3。

目标：运营商页面拆成状态页和个人户提交页，支付/恢复/刷新只在状态页承接。

范围：

- 修改 `weapp/miniprogram/pages/operator/finance/settlement-account/index.*`
- 新增 `weapp/miniprogram/pages/operator/finance/settlement-account/submit/index.*`
- 修改 `weapp/miniprogram/app.json`

状态页要求：

1. 保留 pending workflow 恢复。
2. `verify_fee_pending` 主操作是继续支付核验费。
3. `verify_fee_processing/opening_processing` 进入等待或刷新状态。
4. `profile_pending/failed` 主操作进入提交页。
5. 刷新失败保留上次可信状态。

提交页要求：

1. 不支持下拉刷新。
2. 只展示个人资料输入。
3. 长 label 收短：
   - `银行预留手机号` 可改为 `预留手机号`，字段 note 或 placeholder 说明银行预留。
   - `开户银行（可选）` 可改为 `开户银行`，必要时用 placeholder 表示可选。
4. 底部只有一个全宽主按钮：提交资料并支付核验费。
5. 提交后进入支付/等待 workflow，不直接 Toast 成功。

验收：

- 运营商提交页没有“刷新状态”按钮。
- 继续支付核验费只出现在状态页。
- 返回重入恢复 pending 支付不会覆盖提交页草稿。

验证：

- `PATH="$HOME/.local/bin:$PATH" npm run compile`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:payment-workflow-boundary`

Review checkpoint：

- 检查支付取消、未知、失败是否都能回到状态页真实状态。
- 检查提交页是否还展示状态区。

### BAOFU-WEAPP-07 骑手个人户页面拆分

风险：G3。

目标：骑手页面按运营商同一模式拆分，保留骑手入口路径与收入页跳转。

范围：

- 修改 `weapp/miniprogram/pages/rider/settlement-account/index.*`
- 新增 `weapp/miniprogram/pages/rider/settlement-account/submit/index.*`
- 修改 `weapp/miniprogram/app.json`

实施要求：

1. 状态页保留 `/pages/rider/settlement-account/index`。
2. `pages/rider/income/index.ts` 和其他入口不改为提交页；入口进入状态页，由状态页决定下一步。
3. 提交页字段和运营商一致，但文案按骑手角色调整。
4. 提交页无顶部状态区、无刷新按钮、无下拉刷新。
5. 支付、恢复、刷新在状态页和等待组件承接。

验收：

- 骑手从收入页进入后先看到状态页，不直接进入表单。
- 骑手提交页只有一个主提交按钮。
- 支付后未知结果可回状态页刷新。

验证：

- `PATH="$HOME/.local/bin:$PATH" npm run compile`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:payment-workflow-boundary`

Review checkpoint：

- 检查骑手押金支付 workflow 没有被共享支付改动影响。
- 检查 `baofu_account_verify_fee` 和 rider deposit 的 pending context 不混淆。

### BAOFU-WEAPP-08 路由、入口与回退策略统一

风险：G2/G3。

目标：所有角色入口进入状态页；状态页决定进入提交、支付、等待或终态，避免外部入口绕过状态 owner。

范围：

- `weapp/miniprogram/app.json`
- `weapp/miniprogram/pages/platform/dashboard/dashboard.ts`
- `weapp/miniprogram/pages/merchant/finance/index.ts`
- `weapp/miniprogram/pages/merchant/config/index.ts`
- `weapp/miniprogram/utils/merchant-dashboard-view.ts`
- `weapp/miniprogram/pages/operator/finance/withdraw/index.ts`
- `weapp/miniprogram/pages/rider/income/index.ts`
- `weapp/miniprogram/utils/rider-dashboard-runtime.ts`

实施要求：

1. 外部入口全部指向状态页。
2. 提交页只由状态页或明确的编辑动作进入。
3. 等待完成后返回状态页，并触发后端回查。
4. `navigateBack` 不可靠时 fallback 到状态页。
5. 任何页面不得直接根据本地表单判断终态。

验收：

- 四角色入口路径可达。
- 新增 submit 页面已注册。
- 支付/提交后回退不会掉到过期表单页。

验证：

- `PATH="$HOME/.local/bin:$PATH" npm run compile`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:page-responsibility`

Review checkpoint：

- 检查入口是否绕过状态页。
- 检查 app.json 分包路径是否正确。

### BAOFU-WEAPP-09 清理 Toast 反馈冲突

风险：G2/G3。

目标：取消长耗时流程的 Toast 叠加和 result toast 被 finally 清掉的问题。

范围：

- `weapp/miniprogram/services/baofu-account-onboarding.ts`
- 四角色状态页与提交页 `index.ts`
- 等待组件使用方

实施要求：

1. 提交、支付、轮询、恢复的长耗时状态进入等待组件或页内 loading。
2. 短校验错误只使用一个反馈通道：
   - 字段错误优先贴近表单；
   - 非字段错误可以用页内状态或 Toast，不能重复。
3. 不在页面 finally 中无差别 `hideToast` 清掉刚显示的结果 Toast。
4. `getBaofuOnboardingFeedbackMessage` 只作为页面状态文案来源，不强制 Toast 展示。

验收：

- 提交成功/失败/未知不会出现 Toast 闪断。
- 同一动作不会同时出现 Toast 和页内同文案提示。
- 首屏失败不是 Toast-only。

验证：

- `PATH="$HOME/.local/bin:$PATH" npm run compile`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:request-boundary`
- `PATH="$HOME/.local/bin:$PATH" npm run gate:payment-workflow-boundary`

Review checkpoint：

- 检查是否还有 `showResultToast` 后 immediately `hidePageToast`。
- 检查长耗时路径是否只靠 Toast。

### BAOFU-WEAPP-10 全量质量门禁与人工 G3 验证

风险：G3。

目标：完成自动化门禁、人工路径验证和残余风险记录。

自动化验证：

从 `weapp/` 执行：

1. `PATH="$HOME/.local/bin:$PATH" npm run lint:all`
2. `PATH="$HOME/.local/bin:$PATH" npm run compile`
3. `PATH="$HOME/.local/bin:$PATH" npm run gate:weapp`
4. `PATH="$HOME/.local/bin:$PATH" npm run quality:check`

人工验证矩阵：

企业户：

1. 平台 `profile_pending`：状态页 -> 提交页 -> 填资料 -> 支行按需显示 -> 提交 -> 等待 -> 状态页。
2. 商户 `profile_pending`：权限通过后同平台路径。
3. 企业户支行不需要：省市支行不显示。
4. 企业户支行需要：选银行后显示省市支行并必填。
5. 企业户失败终态：显示后端 `status_desc`，可重新进入提交页。

个人户：

1. 运营商 `profile_pending`：状态页 -> 提交页 -> 提交资料并支付核验费。
2. 骑手 `profile_pending`：同运营商。
3. 支付取消：回状态页显示可继续支付。
4. 支付结果未知：等待组件显示同步中，不报失败。
5. 支付完成但开户处理中：状态页显示处理中并可刷新。

通用：

1. 提交页下拉刷新不可用。
2. 提交页 `onShow` 不覆盖用户草稿。
3. 提交页底部只有一个全宽主按钮。
4. 状态刷新只在状态页出现。
5. 首屏失败有页内重试。
6. 已有可信状态刷新失败保留旧状态。
7. 返回重入恢复 pending workflow。
8. 不暴露原始后端、SQL、provider 或英文诊断。

交付说明必须包含：

- 风险等级和依据。
- 自动化命令结果。
- 人工验证路径。
- 未验证路径。
- 残余风险。
- 是否涉及后端 contract 缺口。

## 6. 非目标

本计划不做以下事情：

1. 不修改后端开户状态机，除非实施中发现后端缺少必要状态字段。
2. 不改变宝付开户接口字段或业务规则。
3. 不把微信支付原进件页面重构为宝付页面的一部分。
4. 不新增前端自定义视觉系统。
5. 不把状态页做成营销页或解释性大卡片页。
6. 不用本地缓存作为资金/开户终态真值。

## 7. 实施顺序

推荐顺序：

1. BAOFU-WEAPP-01
2. BAOFU-WEAPP-02
3. BAOFU-WEAPP-03
4. BAOFU-WEAPP-04
5. BAOFU-WEAPP-05
6. BAOFU-WEAPP-06
7. BAOFU-WEAPP-07
8. BAOFU-WEAPP-08
9. BAOFU-WEAPP-09
10. BAOFU-WEAPP-10

不要先局部改某个页面样式。先统一状态真值和共享表单/等待组件，再迁移各角色页面。

## 8. Review 总检查清单

每个任务卡 review 时至少确认：

1. 后端状态是唯一真值。
2. 状态页和提交页职责没有重新混在一起。
3. 提交页没有下拉刷新，没有状态刷新按钮。
4. 提交页底部只有一个全宽主动作。
5. 支行选择按后端/银行目录要求显示。
6. 长耗时状态不是 Toast-only。
7. unknown result 没被当成失败或成功。
8. 失败终态显示后端映射后的中文说明和下一步。
9. 入口跳转不绕过状态页。
10. 自动化验证和人工 G3 路径都有记录。

## 9. 残余风险记录模板

执行完成后在本节追加：

```text
日期：
执行任务卡：
自动化验证：
人工验证：
未验证路径：
残余风险：
后续处理：
```

## 10. 执行记录

日期：2026-05-10

执行任务卡：

- BAOFU-WEAPP-01 至 BAOFU-WEAPP-10 已按本计划落地。
- 实际新增提交页路径采用计划建议的嵌套路径，并已注册到 `weapp/miniprogram/app.json`：
  - `pages/platform/finance/settlement-account/submit/index`
  - `pages/merchant/finance/settlement-account/submit/index`
  - `pages/operator/finance/settlement-account/submit/index`
  - `pages/rider/settlement-account/submit/index`

自动化验证：

- `PATH="$HOME/.local/bin:$PATH" npm run compile`：通过。
- `PATH="$HOME/.local/bin:$PATH" npm run quality:check`：通过，包含 lint、compile、`gate:weapp` 全量门禁。

人工验证：

- 未在微信开发者工具或真机上执行完整 G3 人工路径。

未验证路径：

- 企业户支行不需要 / 支行需要的银行目录真实返回组合。
- 运营商、骑手核验费支付取消、支付未知、支付成功但开户仍处理中。
- 弱网、离开小程序后重入、后端回调延迟、pending workflow 本地恢复。

残余风险：

- 页面已按后端状态和支付 workflow 承接未知结果，但宝付 / 微信支付回调延迟、银行目录数据差异、弱网下的真实小程序渲染仍需在开发者工具和至少一台真机上走查。
- 前端没有新增后端状态语义；若后端后续新增开户状态，状态页会按未知 / 同步中保守展示，需要后端契约同步后再补显式文案。

后续处理：

- 发布前按 BAOFU-WEAPP-10 的人工验证矩阵补齐真机证据。
