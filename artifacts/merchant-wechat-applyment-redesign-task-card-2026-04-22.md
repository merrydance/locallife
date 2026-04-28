# 商户微信支付开户重设计任务卡

## 目标

把 weapp 商户侧“微信支付开户”从当前的状态页 + 提交页薄封装，重构为一套按微信进件真实流程驱动的页面组与前端工作流模型。

这份任务卡不是设计说明书，而是实现拆解卡。后续执行时，完成一项勾选一项。

## 范围边界

- 只处理 weapp 商户侧微信支付开户链路。
- 首版聚焦页面组、前端工作流模型、二维码待办承接和必要的接口适配。
- 不在本轮顺手重写商户财务首页、提现卡页面或其他支付配置页。
- 不在本轮直接扩成“一个状态一个页面”的碎片化结构。
- 后端若能维持现有 `/v1/merchant/applyment/status` 与 `/v1/merchant/applyment/bindbank` 契约，则先以前端 workflow adapter 落地；只有当前端适配已明显失稳时，才追加后端 workflow contract。

## 风险等级

- 该任务按 `G3` 推进。

理由：

- 直接涉及微信支付开户、签约、法人验证、账户验证与开户结果回查。
- 涉及跨页面状态恢复、重复提交防护、弱网回查和异步结果承接。
- 若流程建模错误，会导致用户误操作、重复提交或对开户结果产生错误判断。

## 当前实现基线

- 当前已有开户状态页：[weapp/miniprogram/pages/merchant/settings/applyment/index.ts](weapp/miniprogram/pages/merchant/settings/applyment/index.ts)
- 当前已有开户提交页：[weapp/miniprogram/pages/merchant/settings/applyment/submit/index.ts](weapp/miniprogram/pages/merchant/settings/applyment/submit/index.ts)
- 当前已有进件状态 view model：[weapp/miniprogram/api/merchant-applyment.ts](weapp/miniprogram/api/merchant-applyment.ts)
- 当前已有签约二维码 / 法人验证二维码展示与保存到相册能力：[weapp/miniprogram/utils/applyment-qrcode.ts](weapp/miniprogram/utils/applyment-qrcode.ts)

当前主要问题：

- 状态模型仍是“状态码翻译器”，不是“工作流模型”。
- 状态页承接了过多待办判断，签约、确认、法人验证、汇款验证都被压成按钮分支。
- 提交页只承接绑卡提交，没有被定义成完整的“资料提交阶段”。
- 二维码能力已存在，但仍是状态页里的次级动作，不是待办阶段的一等承接能力。

## 交付目标

- 用稳定的 workflow view model 替换当前展示型 status view model。
- 把开户链路重构为“开户首页 + 资料提交页 + 待办承接页组”的页面组。
- 把签约二维码、法人验证二维码和保存到相册能力纳入主待办承接。
- 保证页面重入、返回、前后台切换后仍能回到可信状态。
- 在不先改后端契约的前提下完成第一版重构。

## 任务卡

### A. 固定工作流模型与页面边界

- [x] A1. 固定开户流程阶段枚举
  - 产出统一 workflow stage，至少覆盖：`submit_required`、`action_required`、`reviewing`、`rejected`、`opened`。
  - 明确 `action_required` 下的 task 类型至少覆盖：签约、确认、法人验证、汇款验证。
  - 验证：前端工作流设计稿与实现常量一致；页面代码不再直接散写状态码分支。

- [x] A2. 固定页面组边界
  - 开户首页只承接：当前阶段、唯一主待办、最近可信状态、次级信息。
  - 资料提交页只承接：开户资料填写、提交前校验、提交动作。
  - 待办承接页组只承接：签约、确认、法人验证、汇款验证与结果回查。
  - 验证：页面职责文档化，状态页不再内联高密度提交表单。

- [x] A3. 固定二维码承接边界
  - 待签约、待法人验证场景，二维码展示与保存到相册必须是主待办标准能力。
  - 二维码文案必须区分“超级管理员签约”和“法人验证”，避免错用微信号。
  - 验证：二维码能力不再只是首页次级按钮，而是待办页主承接的一部分。

### B. 落地前端 workflow adapter

- [x] B1. 新建 workflow adapter
  - 从现有 `merchant-applyment` status contract 派生 workflow view model。
  - 输出至少包含：`currentStage`、`primaryTask`、`secondaryTasks`、`resultState`、`reentryPolicy`。
  - 验证：页面层只消费 workflow view model，不再自己组合 `needsSign`、`needsConfirmation` 等布尔逻辑。

- [x] B2. 收口当前状态码映射
  - 把 `status`、`sign_state`、`legal_validation_url`、`account_validation`、`reject_reason` 收敛到 workflow adapter 内部统一判断。
  - 保留现有接口兼容性，不先改请求契约。
  - 验证：状态判断 owner 单一，首页、提交页、待办页不重复实现状态分支。

- [x] B3. 明确重入与回查策略
  - 提交成功返回首页、二维码待办返回首页、前后台切换回页时，统一由 workflow adapter 驱动强制回查或静默回查。
  - 验证：不需要用户手动刷新才能恢复正确状态。

### C. 重做开户首页

- [x] C1. 改造首页信息架构
  - 首屏只保留：当前阶段摘要、唯一主待办按钮、关键状态摘要。
  - 不保留“状态说明堆叠 + 多个平权按钮”的旧结构。
  - 验证：首屏只有一个主操作，低频动作下沉。

- [x] C2. 收口首页可见状态
  - 区分：首屏加载、首屏失败、已有可信数据但静默刷新失败、处理中、已开通、驳回、待处理。
  - 验证：失败时页内可见承接，不依赖 Toast-only。

- [x] C3. 收口首页动作出口
  - `submit_required` 只导向资料提交页。
  - `action_required` 只导向当前待办承接页。
  - `opened` 只导向结算账户页或开户完成摘要。
  - 验证：首页不再直接堆放签约二维码、法人验证二维码、刷新状态等并列动作。

### D. 重做资料提交页

- [x] D1. 把提交页升级为“资料提交阶段页”
  - 在现有绑卡表单基础上，明确只读主体信息、可编辑开户资料、提交前校验提示。
  - 不让用户误解为“这里只是改银行卡”。
  - 验证：页面标题、字段分组、提交动作统一指向“提交开户资料”。

- [x] D2. 收口提交前阻塞语义
  - 若当前状态不可重新提交，页面内明确承接阻塞原因，而不是只回状态页。
  - 验证：不可提交状态在提交页内可见，且不会误触发请求。

- [x] D3. 收口提交完成后的回流
  - 提交成功后强制写入回查标记并回流首页。
  - 首页进入后自动回查并跳到正确阶段。
  - 验证：提交成功后不出现“回首页仍停留旧状态”的情况。

### E. 落地待办承接页组

- [x] E1. 新建待签约承接页
  - 展示当前待办说明、签约二维码、保存到相册动作、完成后回页回查。
  - 验证：若有 `sign_url`，页面可稳定生成二维码并保存到相册。

- [x] E2. 新建法人验证承接页
  - 展示法人验证说明、法人验证二维码、保存到相册动作、完成后回页回查。
  - 验证：若有 `legal_validation_url`，页面可稳定生成二维码并保存到相册。

- [x] E3. 新建汇款验证承接页
  - 承接账户验证信息、复制汇款卡号、复制备注、验证完成后回查。
  - 验证：`account_validation` 信息不再只是首页附属卡片，而是独立待办阶段。

- [x] E4. 承接待确认阶段
  - 若当前状态是待确认，给出明确说明与返回回查路径。
  - 如果现有后端只提供状态说明而无显式跳转能力，则先以结果承接页落地，避免假动作。
  - 验证：待确认不会继续混在首页按钮堆里。

### F. 补齐共享状态恢复与导航闭环

- [x] F1. 统一 force refresh 标记与 onShow 回查策略
  - 提交页、待办承接页返回首页都复用同一套回查标记与消费逻辑。
  - 验证：不存在有的子页返回会刷新、有的不会刷新的行为漂移。

- [x] F2. 统一错误映射与页内反馈
  - 首页、提交页、待办页统一使用产品化中文文案，不泄露原始 provider 文本。
  - 验证：错误承接不重复、不混用 Toast 和页内失败态。

- [x] F3. 统一页面组路由与入口
  - 商户 dashboard、支付 readiness、配置页等入口继续指向开户首页，由首页再分发当前主待办。
  - 验证：外部入口保持稳定，不要求外部页面理解流程细节。

### G. 评估并追加后端 workflow contract（可选后置项）

- [ ] G1. 评估前端 adapter 是否已出现明显漂移风险
  - 判断标准：页面是否仍需大量猜测 status 组合；不同页面是否开始重复流程判断。
  - 验证：形成明确结论，决定是否需要后端补 contract。

- [ ] G2. 若前端 adapter 已失稳，再新增后端 workflow contract
  - 候选返回字段：`current_task`、`task_actions`、`last_synced_at`、结构化 reject details。
  - 验证：前端页面可进一步去掉 status 组合判断。

## 验证要求

- 每完成一个前端阶段，至少运行一次 `npm run compile`。
- 页面组或共享 workflow adapter 有变更时，运行 `npm run quality:check`。
- 若引入新页面路由、共享状态恢复或二维码承接变化，手工验证至少覆盖：
  - 提交成功回流首页
  - 待签约展示二维码并保存到相册
  - 待法人验证展示二维码并保存到相册
  - 汇款验证复制动作与回查
  - 前后台切换后的状态恢复

## 执行顺序

按以下顺序推进，不跳步：

1. A. 固定工作流模型与页面边界
2. B. 落地前端 workflow adapter
3. C. 重做开户首页
4. D. 重做资料提交页
5. E. 落地待办承接页组
6. F. 补齐共享状态恢复与导航闭环
7. G. 评估是否需要后端 workflow contract

## 当前状态

- 当前状态：前端首版已完成，后端 workflow contract 评估未开始
- 已验证：`cd weapp && npm run compile`、`cd weapp && npm run quality:check`
- 执行规则：完成一项即在本文件勾选，并补一行验证结果或关联提交说明。