# 骑手押金支付重构任务卡

## 目标

把小程序骑手押金页从“页面内散写充值、提现、轮询、刷新”收敛成一条可恢复、可回查、可迭代的支付工作流。

这份任务卡不是方案说明书，而是实现拆解卡。

## 风险等级

- 该改造按小程序支付相邻链路口径应视为 `G2`

理由：

- 触及微信支付拉起与支付状态确认
- 触及退款型提现的异步处理中语义
- 触及页面重入、后台切回、弱网回查、重复点击防护
- 一旦状态承接做错，会出现“已提交但前端误报失败”或“用户重复发起”

## 当前实现结论

当前链路不是不可用，但实现边界过粗，继续堆功能会越来越脆。

已确认的事实：

- 押金页当前自己持有完整编排，见 [weapp/miniprogram/pages/rider/deposit/index.ts](weapp/miniprogram/pages/rider/deposit/index.ts)
- 充值接口仍是骑手专用入口，但响应已经以 `payment_order_id` 为真值，见 [locallife/api/rider.go](locallife/api/rider.go#L202)
- 提现接口语义是“成功返回 200，处理中返回 202”，见 [locallife/api/rider.go](locallife/api/rider.go#L351)
- 小程序请求层已经把 `202` 视为成功响应，而不是失败，见 [weapp/miniprogram/utils/request.ts](weapp/miniprogram/utils/request.ts#L416)
- 余额接口已经暴露 `withdrawal_processing_amount`，前端可以按后端真值承接处理中金额，见 [locallife/api/rider.go](locallife/api/rider.go#L425)
- 仓库已有通用支付详情页，可复用其支付单回查能力，见 [weapp/miniprogram/pages/user_center/payment-detail/index.ts](weapp/miniprogram/pages/user_center/payment-detail/index.ts)

## 必须先收紧的点

### 1. 充值支付工作流不能继续散写在页面事件里

当前充值流程从发起、拉起微信支付、轮询、提示、刷新都堆在 [weapp/miniprogram/pages/rider/deposit/index.ts](weapp/miniprogram/pages/rider/deposit/index.ts#L364)。

这会导致：

- 页面文件继续膨胀
- 支付恢复逻辑无法被其他页面复用
- 后端一旦把骑手押金彻底并入统一支付接口，页面层需要直接重写

结论：

- 必须抽出 rider deposit payment workflow owner。
- 页面只保留输入、状态渲染、动作触发和结果承接。

### 2. 充值结果不能只靠定时刷新碰运气

当前页在支付后只做了多次 `reloadPage(false)`，见 [weapp/miniprogram/pages/rider/deposit/index.ts](weapp/miniprogram/pages/rider/deposit/index.ts#L355)。

这会导致：

- 从微信支付返回后，如果轮询没命中，页面只能靠延时刷新碰结果
- 用户切后台或稍晚返回时，没有明确的待确认支付状态 owner
- “支付已提交但状态未知”与“支付失败”没有稳定区分

结论：

- 充值必须落到 `payment_order_id` 级别的恢复模型。
- 未知结果必须有“继续回查”承接，而不是只发一次提示。

### 3. 提现前端语义必须和后端退款链路对齐

提现不是简单扣余额，而是退款工作流。后端明确区分 `success` 与 `processing`，见 [locallife/logic/rider_deposit_refund_service.go](locallife/logic/rider_deposit_refund_service.go#L75)。

结论：

- 前端不能把“已提交提现申请”和“已到账”混成同一个成功结论。
- 提现后页面必须依赖 `withdrawal_processing_amount` 和账单明细回读，而不是本地乐观扣减。

## 范围边界

- 只处理小程序骑手押金账户页及其直接依赖的支付工作流。
- 本轮优先收口充值支付与提现状态承接，不顺手改骑手工作台其他模块。
- 本轮不主动修改后端契约；若发现前端无法闭环的契约缺口，再单列阻塞项。
- 本轮不做大的视觉翻新，但允许按非顾客侧 TDesign-first 规则重排信息结构。
- 本轮不新增解释性大卡片、不引入全宽长期提示条。

## 任务卡

### A. 契约与状态模型冻结

- [x] A1. 固定充值状态模型
  - 至少区分：`idle`、`creating`、`paying`、`submitted_pending_confirmation`、`paid`、`cancelled`、`failed`、`unknown`
  - 真值锚点是 `payment_order_id`
  - 验证：任务卡与实现常量命名一致，不允许页面和 workflow 各自解释一套状态

- [x] A2. 固定提现状态模型
  - 至少区分：`idle`、`submitting`、`processing`、`success`、`blocked`、`failed`
  - `processing` 明确表示“退款请求已提交但未最终完成”
  - 验证：页面文案和返回状态映射一致，不把 `processing` 渲染成到账成功

- [x] A3. 固定页面首屏真值来源
  - 余额、配送冻结、提现处理中金额以 `GET /v1/rider/deposit` 为准
  - 账单列表以 `GET /v1/rider/deposits` 为准
  - 充值中的支付状态以 `payment_order_id -> GET /v1/payments/:id` 为准
  - 验证：页面不再用本地临时金额伪装真实结果

- [x] A4. 固定未知结果承接规则
  - 微信支付拉起后未能及时确认时，必须进入“待确认”承接，而不是直接报失败
  - 允许用户稍后回页继续确认
  - 验证：实现中存在明确的 unknown/pending-confirmation 分支

### B. 边界与 owner 收口

- [x] B1. 明确 rider deposit page owner
  - `pages/rider/deposit` 只负责页面状态、弹窗输入、渲染和导航
  - 不再直接承担整条支付业务链路
  - 验证：页面文件内不再堆叠支付创建、支付恢复、支付状态解释的完整逻辑

- [x] B2. 新建 rider deposit payment workflow owner
  - 负责把骑手押金充值接口适配成统一支付工作流
  - 负责拉起微信支付、处理取消、回查支付单、返回统一结果
  - 验证：充值主链由 workflow owner 输出统一结果对象

- [x] B3. 新建 rider deposit finance view model owner
  - 负责组装余额、提现可用性、处理中提示、待确认充值提示
  - 页面不直接拼提现提示文案
  - 验证：提现/充值状态解释不散落在多个页面方法里

- [x] B4. 确认是否复用支付详情页
  - 评估 [weapp/miniprogram/pages/user_center/payment-detail/index.ts](weapp/miniprogram/pages/user_center/payment-detail/index.ts) 是否直接承接 rider deposit payment detail
  - 若可复用，明确跳转方式与入口时机
  - 若不可复用，先记录具体缺口，再决定是否局部扩展
  - 验证：不重复造第二套 payment detail 回查页

### C. API 与 workflow 实现

- [x] C1. 收口充值接口调用
  - 保留 [weapp/miniprogram/api/rider.ts](weapp/miniprogram/api/rider.ts) 的请求封装
  - 新增 workflow 层把 `payment_order_id`、`pay_params`、`expires_at` 适配成统一输出
  - 验证：页面不再直接解读专用充值响应体

- [x] C2. 复用通用支付能力
  - 尽量复用 [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts#L543) 与 [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts#L746)
  - 不再在押金页重复写一套支付状态判断
  - 验证：支付状态成功/失败/未知判断收口到统一 helper

- [x] C3. 实现充值 pending 恢复
  - 记录当前待确认的 `payment_order_id`
  - 页面重进、切后台返回、支付取消后可继续确认
  - 验证：不必重新发起支付也能继续回查既有支付单

- [x] C4. 处理用户取消支付
  - 用户取消只终止本次拉起，不应把现有 pending 支付单误标失败
  - 页面保留“稍后继续确认”的能力
  - 验证：取消支付后不会误提示“充值失败”并丢失待确认状态

- [x] C5. 收口提现提交流程
  - 保留 [weapp/miniprogram/api/rider.ts](weapp/miniprogram/api/rider.ts) 的提现请求封装
  - 把 `success` / `processing` 映射为清晰前端状态
  - 验证：HTTP 202 不进入错误分支，且页面提示与账单承接一致

- [x] C6. 禁止提现本地乐观扣减
  - 提现后只触发回读，不在页面先减可用押金
  - 所有处理中金额从后端余额接口回读
  - 验证：代码中不存在本地伪造余额结果的 `setData`

### D. 页面信息架构与交互收口

- [x] D1. 重排押金页首屏结构
  - 首屏只保留：余额总览、当前关键状态、主操作、账单入口/列表
  - 去掉重复解释性文案堆叠
  - 验证：首屏不再依赖多条说明才能理解主要任务

- [x] D2. 充值待确认状态就地承接
  - 若存在 pending recharge，在主操作附近显示短状态与继续确认入口
  - 不用长期 notice bar 占满页面
  - 验证：用户回到押金页能知道“还有一笔待确认充值”

- [x] D3. 提现处理中状态就地承接
  - 处理中金额在余额区明确可见
  - 再次提现入口按后端真值禁用
  - 验证：处理中时用户能理解为什么当前不能重复申请

- [x] D4. 收口弹窗文案
  - 充值弹窗只保留必要输入约束
  - 提现弹窗只保留必要风险提示和约束
  - 不重复解释页面边界
  - 验证：删除冗余说明后，流程仍可理解

- [x] D5. 防重复提交
  - 充值、提现按钮在提交中不可重入
  - 页面切换和返回时不自动重复提交
  - 验证：重复点击不会创建重复请求

### E. 验证与回归

- [x] E1. 编译验证
  - 运行 `npm run compile`
  - 验证：小程序编译通过

- [x] E2. 质量门禁
  - 运行 `npm run quality:check`
  - 验证：通过 weapp 质量门禁

- [ ] E3. 充值主链手工验证
  - 发起充值
  - 拉起微信支付
  - 支付成功后回页
  - 余额和账单最终对齐
  - 验证：成功、取消、未知结果三条分支都能承接

- [ ] E4. 提现主链手工验证
  - 提现成功分支
  - 提现处理中分支
  - 配送冻结 / 提现处理中 / 余额不足阻塞分支
  - 验证：页面提示与后端实际结果一致

- [ ] E5. 重入恢复验证
  - 充值拉起支付后切后台再回来
  - 支付后不立即停留在押金页，而是稍后再进
  - 验证：待确认充值仍能恢复，不需要重新发起

- [ ] E6. 回归验证
  - 骑手工作台进入押金页入口不回归
  - 账单分页不回归
  - 已有 wallet / payment detail 对 rider_deposit 展示不回归
  - 验证：相关页面可继续读取 rider_deposit 业务类型

## 推荐实现顺序

- [x] P1. 先完成 A1-A4
- [x] P2. 再完成 B1-B4
- [x] P3. 再完成 C1-C6
- [x] P4. 再完成 D1-D5
- [ ] P5. 最后完成 E1-E6

## 每阶段验证要求

- A/B 完成后：先回写任务卡与状态定义，不允许边写代码边改状态含义
- C 完成后：至少完成 `npm run compile`
- D 完成后：至少完成 `npm run quality:check`
- E 完成后：在任务卡中补齐已验证分支与未验证剩余风险

## 非目标

- 不在本轮把骑手押金充值后端直接改造成统一 `/v1/payments` 创建接口
- 不在本轮重写 rider dashboard 或 wallet 全部资金页
- 不在本轮引入新的全局 notice 壳或解释性大卡片
- 不在本轮顺手重做提现账单后端语义

## 当前状态

- 当前状态：A-D、E1-E2 与 P1-P4 已完成，剩余 E3-E6 需要真机或联调环境手工验证
- 执行规则：实现一项，勾选一项；如果某项前置未完成，不允许跳项报完成
- 若中途发现后端契约不足以支持 pending recharge 恢复，先在本文件追加阻塞项，再决定是否切后端任务