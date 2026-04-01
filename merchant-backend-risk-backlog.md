# 商户侧后端风险与疑点 Backlog

## 目的

这份文档只收录当前后端审查中无法确认“已经完整且无明显风险实现”的商户侧业务能力。

用途：

1. 作为逐条深入审查的待办清单。
2. 每条问题在进一步确认后，再决定是否进入修复。
3. 明确区分“已知问题”“高风险假设”“验证缺口”。

说明：下方风险清单保留首次审查快照；若与“本轮已完成修复”冲突，以最新状态为准。

## 本轮已完成修复（2026-04-01）

1. RB-01 已收口：商户开户绑定银行卡与开户状态查询已改为 owner-only。
2. RB-02 已收口：提现申请已改为 owner-only；历史提现记录继续保持“谁提现谁查看”。
3. RB-03、RB-04、RB-05 已按岗位矩阵拆分：桌台、预订、设备、展示配置分别限制到 owner/manager/cashier 的允许范围，并补了缺失的商户归属校验。
4. RB-06 已落地：飞鹅云 V1 云打印已接入，测试打印、自动打印、手动补打、打印记录查询均已形成端到端链路。
5. RB-07 已收口：商户完结投诉前会校验投诉归属，且补充了对应 API 测试。
6. RB-10 已收口：下单主路径中的调试日志已移除。
7. RB-08 已补直接回归：商户代客建预订已有 handler 级自动化覆盖，包含成功创建与 cashier 允许路径。
8. RB-13 已收口：创建菜品时图片与规格组选项已并入同一事务，并补了失败回滚测试，避免残留半成品菜品。
9. RB-14 已按最小方案收口：owner 侧包装策略配置、下单硬校验、sqlc/mocks 与 API/logic 测试均已落地；当前仅剩购物车前置提示和更复杂联动规则未纳入本轮。

## 打印后续关注点

1. 当前 V1 仅支持飞鹅云；API 入参、worker 分发与 Swagger 已统一到飞鹅云-only。
2. worker 会跳过库中历史遗留的非飞鹅打印机，避免误进入自动打印链路。
3. 创建设备与删除设备都已补上远端补偿：当“本地变更失败且远端补偿也失败”时，系统会落库为商户侧打印机对账任务，可通过设备对账列表查看并手动重试，不再只能依赖日志排查。
4. 当前对账任务聚焦设备注册/删除漂移修复；若后续要进一步做运维自动化，可再补周期性扫描与自动修复策略。
5. 商户订单侧现已补上打印异常列表与按原打印记录重试能力，异常列表按“订单 + 打印机最新一次结果”聚合，重试成功后旧失败项会自然退出异常视图。

## 暂不纳入本轮风险 Backlog 的高置信链路

以下链路当前证据相对充分，暂不作为优先风险项进入本清单：

1. 外卖商户接单/拒单/出餐主状态机。
依据：[locallife/logic/merchant_order.go](locallife/logic/merchant_order.go)、[locallife/logic/merchant_order_test.go](locallife/logic/merchant_order_test.go)、[locallife/integration/takeout_journey_integration_test.go#L464](locallife/integration/takeout_journey_integration_test.go#L464)

2. 堂食会话核心链路：开台、转台、结账。
依据：[locallife/logic/dining_session.go](locallife/logic/dining_session.go)、[locallife/api/dining_session_test.go#L180](locallife/api/dining_session_test.go#L180)、[locallife/api/dining_session_test.go#L271](locallife/api/dining_session_test.go#L271)、[locallife/integration/transfer_integration_test.go#L74](locallife/integration/transfer_integration_test.go#L74)

3. 商户评价回复基础流程。
依据：[locallife/api/review.go#L456](locallife/api/review.go#L456)、[locallife/api/review_test.go#L376](locallife/api/review_test.go#L376)

## 风险分级说明

1. `高`：涉及资金、权限、租户边界、外部系统调用，或可能让错误角色直接执行敏感业务。
2. `中`：业务能力存在，但验证不足、角色边界不清、状态约束不完整，可能引发回归或灰色故障。
3. `低`：不直接破坏主链路，但会引入噪音、误导、维护风险，或体现为未完工能力。

## 本轮处理策略

本轮按“先记账、后统一修复”的方式推进，不在发现疑点后立即改代码，除非出现可被明确证实、且需要立即止血的高危缺陷。

执行原则：

1. 先把所有风险项逐条审成“已确认问题”“高风险假设”或“验证缺口”。
2. 权限模型、资金权限、商户解析语义这类共性问题，先统一规则，再批量修复，避免前后口径不一致。
3. 补测类问题先集中确认缺口，最后按链路批量补测试。
4. 明确未完工或调试残留问题，放到最后一起收尾，避免打断主审查路径。

按当前信息，后续更适合按下面三包集中处理：

1. 权限与角色规则统一包：RB-01、RB-02、RB-03、RB-04、RB-05、RB-09
2. 验证与回归保护补强包：RB-07、RB-08
3. 明确未完工与清理收尾包：RB-06、RB-10

## 已拍板口径（持续更新）

1. 开户相关能力与提现能力均为老板专属，员工岗位不应操作。
2. 商户岗位矩阵需要单独设计后，再批量收口桌台、预订、设备等经营权限。
3. 集团与品牌当前定位是服务集团老板或多店经营老板查看各店经营情况与组织管理，暂不接管菜单、价格、库存和营销；现有残留逻辑与文案应清理，避免混淆。
4. 菜品规格当前目标是“多个规格组并行、每个规格组单选”，例如大小、冷热、辣度可同时存在；不要求组内多选。
5. 提现记录继续保持“谁提现谁看”，不需要对员工开放共享视图；在 owner-only 收口后，员工也不应再产生新的提现记录。
6. 包装物继续沿用普通菜品模型，不增加专门标识；若后续要实现“免费包装/收费包装二选一”，优先采用订单级包装策略，而不是菜单级 required group，并对外卖与打包两类订单生效。
7. 投诉完结授权按调用来源拆分：商户端只允许当前商户的 owner/manager 完结本商户投诉；运营端保留全局完结能力。

## 商户岗位矩阵建议稿

按当前业务边界，建议以 `owner`、`manager`、`cashier`、`chef` 四类岗位为准，按下面口径收口：

1. `owner`：完整经营权限，包含资金、开户、结构变更、设备配置、预订管理。
2. `manager`：除资金与开户外的全店经营权限，允许做结构和配置类操作。
3. `cashier`：前台运营权限，负责桌台状态、前台订单与基础预订操作，不负责结构和配置。
4. `chef`：厨房执行权限，负责 KDS 与出餐相关使用，不负责桌台结构、预订经营动作和设备配置。

建议按资源细分如下：

1. 桌台：
owner/manager 可新增、删除、改图、改标签、生成或重置二维码。
owner/manager/cashier 可查看与修改桌台状态。
chef 默认不开放。

2. 预订：
owner/manager 可查看、代客创建、改预订、确认、完成、标记 no-show。
cashier 可查看、代客创建、确认、完成；默认不给改预订与 no-show。
chef 默认不开放。

3. 设备与展示配置：
owner/manager 可增删改打印机、测试打印、修改 KDS/语音/展示配置。
cashier/chef 默认不开放配置改动权限。

4. 资金与开户：
只给 owner，不给其他岗位。

## 订单级包装策略最小设计稿

目标：

1. 保持“包装物就是普通菜品”的现有建模，不引入菜品特殊标记。
2. 只解决“外卖/打包下单时，必须从指定包装菜品中选择 1 个”的约束，不扩展成菜单模板、套餐规格或 SKU 体系。
3. 先把规则收口到门店级配置与下单校验，避免把问题扩散到菜品模型本身。

建议的数据模型：

1. 新增门店级配置表，例如 `merchant_packaging_policies`。
2. 结构保持和 `merchant_membership_settings` 一致：每店一条记录，`merchant_id` 唯一。
3. 最小字段建议：
  `merchant_id`
  `applicable_order_types TEXT[]`，当前只允许 `takeout`、`takeaway`
  `candidate_dish_ids BIGINT[]`
  `created_at`
  `updated_at`
4. 不单独加 `enabled` 字段：当 `applicable_order_types` 为空或 `candidate_dish_ids` 为空时，视为未启用。
5. 不增加 `required_count`、`selection_mode` 等扩展字段；当前口径固定为“命中场景时恰选 1 个”。后续真有“一份餐盒/按件数联动”需求，再单独升级模型。

建议的配置口径：

1. 只有 owner 可以读写包装策略配置。
2. 配置时要求 `candidate_dish_ids` 全部属于当前商户。
3. 配置时要求候选包装菜品当前可售，至少不能是已删除或跨商户菜品；是否强制 `on_shelf` 可在实现时二选一，但不能允许失效菜品混入配置。
4. `applicable_order_types` 仅允许 `takeout`、`takeaway`；`dine_in`、`reservation` 不进入本能力范围。

建议的接口形态：

1. 增加 owner 侧读取接口：获取当前门店包装策略。
2. 增加 owner 侧保存接口：整条覆盖式 upsert 即可，不必先做复杂 patch 语义。
3. 返回体除配置本身外，建议顺带回传候选包装菜品的基础信息，避免前端再自行拼多次查询。

建议的校验落点：

1. `CreateOrder` 作为最终硬校验点，必须兜底。
2. 购物车加菜/改菜链路可同步做前置校验或预检提示，但即使购物车层先不做，创建订单层也必须拦住。
3. 校验规则建议为：
  当订单类型属于策略生效场景，且门店存在有效包装策略时，订单项中命中的包装候选菜品数量必须恰好为 1。
4. 命中数量为 0：返回明确业务错误，例如“请先选择包装方式”。
5. 命中数量大于 1：返回明确业务错误，例如“只能选择一种包装方式”。
6. 命中的包装菜品仍按普通菜品参与价格计算、优惠计算和订单明细落库，不做特殊记账分支。

建议的实现边界：

1. 先只校验“是否选了且只选了 1 个”，不做“包装数量必须跟随主商品数量”这类复杂联动。
2. 先不做“按分类自动识别包装菜品”或“全菜单自动兜底”，必须由商户显式配置候选包装菜品。
3. 先不把包装策略下沉到菜单、分类、套餐、规格组、集团模板或品牌模板。
4. 先不改变购物车与订单项现有结构，包装物继续只是普通 dish item。

建议的测试补强：

1. 新增 owner 配置接口测试：空配置、合法配置、跨商户菜品、失效菜品、非法订单类型。
2. 新增下单测试：
  `takeout` 命中策略但未选包装，应失败。
  `takeout` 选了两个包装候选，应失败。
  `takeout` 恰选一个包装候选，应成功。
  `takeaway` 同上。
  `dine_in`、`reservation` 即使未选包装，也不应触发该校验。
3. 如果购物车层也加校验，再补加菜、改菜、删菜后的即时反馈测试。

这样做的好处：

1. 与当前“包装物是普通菜品”的历史口径完全兼容。
2. 能直接复用现有“门店 owner 配置 + 逻辑层按订单类型消费配置”的实现模式。
3. 不会把一个订单约束问题错误地做成菜品模型重构。

## 风险清单

### RB-01 开户能力未收口到明确岗位权限

- 场景：商户资料维护与收款开通
- 等级：高
- 类型：权限风险
- 当前判断：已确认问题
- 现象：`/merchant/applyment/bindbank` 和 `/merchant/applyment/status` 都没有挂 `MerchantStaffMiddleware`，且处理器通过 `GetMerchantByOwner` 解析商户，而底层 SQL 已兼容 active staff。进一步细看后可确认：`status` 查询对 active staff 是可达的；`bindbank` 虽然入口同样可达，但后续还要求按当前登录用户读取 `GetUserMerchantApplication`，因此非 owner 员工是否能真正提交开户，取决于其名下是否存在商户申请记录，不能直接下结论为“任意员工都能提交开户”。
- 关键证据：
  [locallife/api/server.go#L662](locallife/api/server.go#L662)
  [locallife/api/ecommerce_applyment.go#L56](locallife/api/ecommerce_applyment.go#L56)
  [locallife/api/ecommerce_applyment.go#L419](locallife/api/ecommerce_applyment.go#L419)
  [locallife/db/query/merchant.sql#L81](locallife/db/query/merchant.sql#L81)
  [locallife/api/rbac_middleware.go#L158](locallife/api/rbac_middleware.go#L158)
- 风险影响：
  1. 非老板角色大概率可以查看开户进度，并触发开户状态同步、副商户号落库、商户状态更新等后续副作用。
  2. 绑卡开户入口的权限模型不清晰，即使最终因缺少本人申请记录而失败，也仍然体现为资金敏感能力的路由边界未收口。
  3. 权限语义与“员工协作模型”不一致。
- 待核问题：
  1. 如何把 `status` 与 `bindbank` 一并明确收口为 owner-only？
  2. 是否直接复用 owner-only 中间件，而不是继续依赖“查不到本人申请记录”来被动失败？
  3. 是否需要一并复用 `MerchantStaffMiddleware` 中的商户状态和 region 前置校验？
- 建议后续验证：
  1. 用 manager/cashier/chef 身份直接请求 `status`，确认是否能读到开户进度并触发状态同步。
  2. 用 manager/cashier/chef 身份直接请求 `bindbank`，确认当前实际表现是成功提交、400/404 失败，还是 500 异常。
  3. 检查产品定义的资金权限模型。

### RB-02 资金账户提现允许 manager 操作，需确认是否符合资金权限策略

- 场景：经营统计、财务分析与资金提现
- 等级：高
- 类型：权限策略与审计归属风险
- 当前判断：已确认问题
- 现象：商户财务路由整体允许 `owner` 和 `manager`，其中包含余额查询、提现申请、提现记录查询。提现处理器没有再做 owner-only 限制，而是沿用 staff-compatible 的 `GetMerchantByOwner` 解析商户，因此 manager 在当前实现下可直接对商户收付通余额发起提现。同时，提现记录不是按 merchant 维度归档，而是按发起用户 `user_id` 写入和查询，这意味着同一商户下 owner 与 manager 的提现记录天然彼此隔离。
- 关键证据：
  [locallife/api/server.go#L1129](locallife/api/server.go#L1129)
  [locallife/api/merchant_finance.go#L1138](locallife/api/merchant_finance.go#L1138)
  [locallife/api/merchant_finance.go#L1232](locallife/api/merchant_finance.go#L1232)
  [locallife/api/merchant_finance.go#L1294](locallife/api/merchant_finance.go#L1294)
  [locallife/api/merchant_finance.go#L1363](locallife/api/merchant_finance.go#L1363)
  [locallife/api/merchant_finance.go#L1441](locallife/api/merchant_finance.go#L1441)
  [locallife/db/query/merchant.sql#L81](locallife/db/query/merchant.sql#L81)
  [locallife/db/query/operator_finance.sql#L1](locallife/db/query/operator_finance.sql#L1)
  [locallife/db/query/operator_finance.sql#L16](locallife/db/query/operator_finance.sql#L16)
- 风险影响：
  1. 如果产品预期“仅老板可提现”，当前实现已明确放宽了资金操作权限。
  2. 即使产品允许 manager 提现，当前记录模型也会让提现历史按操作人分裂，owner 无法天然看到 manager 发起的提现，manager 也看不到 owner 的提现，不利于商户维度审计与对账。
  3. 单笔提现详情还要求 `record.UserID == authPayload.UserID`，进一步固化了“同店不同角色互不可见”的资金记录边界。
- 待核问题：
  1. 是否需要补充 owner 级别的独立审计日志或二次确认？
- 建议后续验证：
  1. 用 manager 身份发起 `/merchant/finance/account/withdraw`，确认当前真实行为。
  2. 用 owner 和 manager 分别查询 `/merchant/finance/account/withdrawals` 与详情接口，确认是否存在同店资金记录互不可见。
  3. 对照产品/财务权限要求确认设计意图。

### RB-03 桌台管理权限未收口，且部分读接口缺少商户归属校验

- 场景：商品、套餐、库存与桌台资源配置
- 等级：高
- 类型：权限与租户边界风险
- 当前判断：已确认问题
- 现象：桌台相关路由未挂 `MerchantStaffMiddleware`。大部分写接口只通过 `GetMerchantByOwner` 解析商户并校验 `table.MerchantID == merchant.ID`，因此 active staff 在当前实现下可直接创建、修改、改状态、删桌、改图、打标签、生成二维码。与此同时，`listTableTags` 和 `listTableImages` 两个读接口甚至没有商户解析和归属校验，只要知道桌台 ID，任意已登录用户就可以读取桌台标签和图片列表。
- 关键证据：
  [locallife/api/server.go#L830](locallife/api/server.go#L830)
  [locallife/api/table.go#L120](locallife/api/table.go#L120)
  [locallife/api/table.go#L534](locallife/api/table.go#L534)
  [locallife/api/table.go#L660](locallife/api/table.go#L660)
  [locallife/api/table.go#L796](locallife/api/table.go#L796)
  [locallife/api/table.go#L1175](locallife/api/table.go#L1175)
  [locallife/api/table.go#L1270](locallife/api/table.go#L1270)
  [locallife/api/table.go#L1328](locallife/api/table.go#L1328)
  [locallife/api/table.go#L975](locallife/api/table.go#L975)
  [locallife/api/scan.go#L551](locallife/api/scan.go#L551)
  [locallife/db/query/merchant.sql#L81](locallife/db/query/merchant.sql#L81)
  [locallife/api/table_test.go#L75](locallife/api/table_test.go#L75)
  [locallife/api/table_test.go#L1109](locallife/api/table_test.go#L1109)
  [locallife/api/table_test.go#L1685](locallife/api/table_test.go#L1685)
- 风险影响：
  1. 收银或后厨可能拥有不应具备的桌台结构调整权限，包括建桌、删桌、改状态、改图和生成二维码。
  2. 这些接口绕过了 `MerchantStaffMiddleware` 中对商户状态和 region 的统一校验。
  3. 任意已登录用户如果枚举到桌台 ID，还可直接读取桌台标签和图片列表，形成跨商户资源暴露面。
- 待核问题：
  1. 桌台新增/删除/二维码生成是否应仅 owner/manager 可做？
  2. 桌台状态更新是否可以单独授权给 cashier？
  3. 桌台图片和标签列表是否本来要走商户侧私有接口，还是应改成明确的 C 端公开视图接口？
- 建议后续验证：
  1. 用 cashier/chef 身份测试建桌、删桌、改状态、改图和二维码生成，确认当前真实权限面。
  2. 用非该商户用户直接请求桌台标签和图片列表，确认是否存在跨商户读取。
  3. 检查禁用商户或未完成开通商户是否仍可调用这些接口。

### RB-04 商户预订管理路由未使用岗位中间件，角色边界不清

- 场景：预订、预订点菜与到店提醒
- 等级：高
- 类型：权限风险与验证缺口
- 当前判断：部分确认
- 现象：商户侧预订列表、统计、今日预订、代客创建、修改、确认、完成、标记 no-show 全部在 `/reservations` 组下，未挂 `MerchantStaffMiddleware`。处理器和逻辑层同样沿用 staff-compatible 的 `GetMerchantByOwner` 解析商户，因此当前实现下 active staff 可以进入整套商户预订管理面。与此同时，商户代客创建、商户修改、今日预订这几条链路没有直接找到 API 测试，岗位边界和关键行为都缺少自动化保护。
- 关键证据：
  [locallife/api/server.go#L866](locallife/api/server.go#L866)
  [locallife/api/table_reservation.go#L888](locallife/api/table_reservation.go#L888)
  [locallife/api/table_reservation.go#L1320](locallife/api/table_reservation.go#L1320)
  [locallife/api/table_reservation.go#L1604](locallife/api/table_reservation.go#L1604)
  [locallife/api/table_reservation.go#L1799](locallife/api/table_reservation.go#L1799)
  [locallife/api/table_reservation.go#L1872](locallife/api/table_reservation.go#L1872)
  [locallife/logic/reservation.go#L310](locallife/logic/reservation.go#L310)
  [locallife/logic/reservation.go#L380](locallife/logic/reservation.go#L380)
  [locallife/logic/reservation.go#L423](locallife/logic/reservation.go#L423)
  [locallife/logic/reservation.go#L612](locallife/logic/reservation.go#L612)
  [locallife/logic/reservation_update.go#L27](locallife/logic/reservation_update.go#L27)
  [locallife/db/query/merchant.sql#L81](locallife/db/query/merchant.sql#L81)
  [locallife/api/table_reservation_test.go#L823](locallife/api/table_reservation_test.go#L823)
  [locallife/api/table_reservation_test.go#L594](locallife/api/table_reservation_test.go#L594)
  [locallife/api/table_reservation_test.go#L976](locallife/api/table_reservation_test.go#L976)
  [locallife/api/table_reservation_test.go#L1241](locallife/api/table_reservation_test.go#L1241)
- 风险影响：
  1. 低权限员工当前大概率可以直接查看全店预订列表与统计，并执行代客建预订、修改预订、确认、完成、标记爽约等经营动作。
  2. 这些接口同样绕过了 `MerchantStaffMiddleware` 中对商户状态和 region 的统一校验。
  3. 商户代客创建与商户修改缺少直接自动化验证，一旦字段或初始状态漂移，不容易被及时发现。
- 待核问题：
  1. 哪些预订动作应开放给 manager？哪些可以给 cashier？哪些必须 owner-only？
  2. `no-show`、完成预订、商户改预订是否需要更高权限或审计留痕？
  3. 商户代客创建是否允许所有 active staff，还是只允许前台/店长类岗位？
- 建议后续验证：
  1. 用不同 staff 角色请求 `/reservations/merchant/*` 及 `/reservations/:id/(confirm|complete|no-show|update)`，确认当前真实权限面。
  2. 检查禁用商户或未完成开通商户是否仍可使用这些商户预订接口。
  3. 后续补充商户代客创建、商户修改、今日预订的直接 API 测试。

### RB-05 设备管理与展示配置未使用岗位中间件，且绕过统一商户状态校验

- 场景：商户资料维护与收款开通、商品与经营配置
- 等级：高
- 类型：权限与准入风险
- 当前判断：已确认问题
- 现象：打印机与展示配置路由都未挂 `MerchantStaffMiddleware`。处理器统一只依赖 staff-compatible 的 `GetMerchantByOwner` 找商户，再按 `merchant.ID` 读写打印机或展示配置，没有统一复用“商户需 active/approved、region 已设置”的中间件逻辑。打印机详情、更新、删除、测试虽然做了打印机归属校验，但岗位上仍然是“任何能解析到该商户的 active staff 都可操作”；展示配置读写则完全没有岗位细分。
- 关键证据：
  [locallife/api/server.go#L1146](locallife/api/server.go#L1146)
  [locallife/api/server.go#L1157](locallife/api/server.go#L1157)
  [locallife/api/device.go#L68](locallife/api/device.go#L68)
  [locallife/api/device.go#L154](locallife/api/device.go#L154)
  [locallife/api/device.go#L285](locallife/api/device.go#L285)
  [locallife/api/device.go#L380](locallife/api/device.go#L380)
  [locallife/api/device.go#L449](locallife/api/device.go#L449)
  [locallife/api/device.go#L528](locallife/api/device.go#L528)
  [locallife/api/device.go#L620](locallife/api/device.go#L620)
  [locallife/db/query/merchant.sql#L81](locallife/db/query/merchant.sql#L81)
  [locallife/api/rbac_middleware.go#L158](locallife/api/rbac_middleware.go#L158)
  [locallife/api/device_test.go#L460](locallife/api/device_test.go#L460)
  [locallife/api/device_test.go#L598](locallife/api/device_test.go#L598)
  [locallife/api/device_test.go#L922](locallife/api/device_test.go#L922)
  [locallife/api/device_test.go#L1042](locallife/api/device_test.go#L1042)
  [locallife/api/device_test.go#L1162](locallife/api/device_test.go#L1162)
- 风险影响：
  1. 任意 active staff 当前大概率可以增删打印机、改打印开关、改 KDS/语音/展示配置。
  2. 商户状态不合规或 region 未设置时，仍可能进入这些配置接口。
  3. 设备与展示配置属于门店基础设施面，一旦低权限岗位误改，影响面会直接扩散到接单、后厨或提醒链路。
- 待核问题：
  1. 打印机和展示配置应开放给哪些岗位？
  2. 是否应补充 merchant status/region 前置校验？
- 建议后续验证：
  1. 用 chef/cashier 身份测试打印机和展示配置读写，确认当前真实权限面。
  2. 用 inactive merchant 或 region 未设置商户测试是否仍可通过。

### RB-06 打印机测试能力明确未实现（已关闭，保留历史记录）

- 场景：商户资料维护与收款开通
- 等级：中
- 类型：明确未完工
- 当前判断：已关闭
- 现象：该条目对应的风险已在本轮关闭。当前测试打印接口已接入飞鹅云在线测试打印；同时补齐了自动打印、手动补打、打印记录查询与 Swagger 文档。501 仅保留给“非飞鹅云类型或当前环境未配置云打印客户端”的暂不支持场景。
- 关键证据：
  [locallife/api/device.go#L457](locallife/api/device.go#L457)
  [locallife/api/order.go#L1579](locallife/api/order.go#L1579)
  [locallife/worker/task_print_order.go#L15](locallife/worker/task_print_order.go#L15)
  [locallife/api/device_test.go#L936](locallife/api/device_test.go#L936)
- 风险影响：
  1. 原“测试打印未实现”的问题已消除。
  2. 当前剩余约束主要是厂商范围，以及是否需要继续把少量异常场景自动化闭环，而不是功能空缺。
- 待核问题：
  1. 是否需要在后续版本接入更多云打印厂商。
  2. 是否需要在现有商户侧手动对账/重试基础上，再补充周期性远端扫描或自动修复工具，用于进一步减少人工介入。

### RB-07 商户投诉处理链路缺少自动化验证，且依赖外部微信接口

- 场景：投诉、索赔、申诉与追偿支付
- 等级：高
- 类型：权限缺陷与高风险路径验证缺口
- 当前判断：已确认问题
- 现象：商户投诉路由组本身已挂 `MerchantStaffMiddleware("owner", "manager")`，因此问题不在岗位收口；真正的问题是 `completeComplaint` 为了同时复用给运营商路由和商户路由，没有在商户路径下校验投诉是否属于当前商户。相比之下，列表、详情、回复投诉都显式做了商户归属校验。与此同时，这整条投诉 API 链路和 webhook/同步链路几乎没有自动化测试覆盖。
- 关键证据：
  [locallife/api/server.go#L670](locallife/api/server.go#L670)
  [locallife/api/server.go#L1241](locallife/api/server.go#L1241)
  [locallife/api/complaint.go#L93](locallife/api/complaint.go#L93)
  [locallife/api/complaint.go#L136](locallife/api/complaint.go#L136)
  [locallife/api/complaint.go#L172](locallife/api/complaint.go#L172)
  [locallife/api/complaint.go#L242](locallife/api/complaint.go#L242)
  [locallife/api/appeal.go#L26](locallife/api/appeal.go#L26)
  [locallife/api/complaint.go#L340](locallife/api/complaint.go#L340)
  [locallife/worker/task_sync_complaints.go#L60](locallife/worker/task_sync_complaints.go#L60)
  [locallife/db/query/wechat_complaint.sql#L1](locallife/db/query/wechat_complaint.sql#L1)
- 风险影响：
  1. 任意 owner/manager 级商户用户如果知道投诉单号，理论上可直接调用商户端完结接口结束不属于自己商户的投诉，形成跨商户合规处置风险。
  2. 投诉回复与完结都直接依赖微信接口，缺少 API 级自动化测试时，外部失败、状态漂移和回写降级行为不易被及时发现。
  3. webhook 通知与批量同步虽然已设计幂等，但同样缺少测试证据，回调与补偿协作仍属未验证高风险路径。
- 待核问题：
  1. 是否已有遗漏的集成或 provider mock 测试未被发现？
  2. 商户完结投诉是否还需要额外的审计日志或操作人留痕？
- 建议后续验证：
  1. 用 A 商户身份直接请求 B 商户的投诉完结接口，确认当前跨商户可操作性。
  2. 后续补充商户投诉列表、详情、回复、完结、webhook 通知、投诉同步任务的自动化测试。
  3. 审查完结投诉的审计与操作留痕需求。

### RB-08 商户代客建预订缺少直接自动化验证

- 场景：预订、预订点菜与到店提醒
- 等级：中
- 类型：验证缺口
- 当前判断：已收口；首次审查时的直接验证缺口已补齐。
- 现象：更新（2026-04-01）：已在 [locallife/api/table_reservation_test.go](locallife/api/table_reservation_test.go) 补上商户代客建预订的直接 API 覆盖，包含成功创建与 cashier 允许路径。下面保留的是首次审查快照，供追溯使用。
- 关键证据：
  [locallife/api/table_reservation.go#L1604](locallife/api/table_reservation.go#L1604)
  [locallife/logic/reservation.go#L310](locallife/logic/reservation.go#L310)
  [locallife/db/query/table_reservation.sql#L31](locallife/db/query/table_reservation.sql#L31)
  [locallife/db/query/table_reservation.sql#L284](locallife/db/query/table_reservation.sql#L284)
  [locallife/api/table_reservation.go#L1666](locallife/api/table_reservation.go#L1666)
  [locallife/worker/task_reservation_timeout.go#L52](locallife/worker/task_reservation_timeout.go#L52)
- 风险影响：
  1. 代客建预订涉及资源占用、初始状态设定、来源字段写入、异步提醒调度，缺少直接回归保护。
  2. 一旦 `confirmed` 初始状态、`source` 语义、或 no-show 调度时点被改动，现有测试不一定能第一时间发现行为漂移。
  3. 用户侧预订列表当前通过 `source = 'online' 或 NULL` 过滤排除了商户代客预订，这个设计语义本身也缺少直接验证。
- 待核问题：
  1. `confirmed` 初始状态、`source` 字段和 no-show 提醒调度是否已有遗漏的集成覆盖？
  2. 用户侧是否应完全看不到商户代客预订，还是未来需要独立展示入口？
- 建议后续验证：
  1. 补一条直接命中 `/v1/reservations/merchant/create` 的 API 测试，校验状态、来源和参数约束。
  2. 补一条 logic 或 integration 测试，覆盖桌台归属校验、冲突校验和 no-show 任务分发。

### RB-09 `GetMerchantByOwner` 语义已扩展为 staff 兼容，但很多 handler 命名和依赖仍带 owner 语义

- 场景：多个商户业务场景共性风险
- 等级：中
- 类型：结构耦合风险
- 当前判断：已部分收口；API 层核心 helper、中间件入口以及一批主要商户 handler 已统一到显式的“用户关联商户”语义，商户财务读接口与 owner-only 提现动作也已拆成不同 helper 语义，但仍有部分历史调用待继续清理。
- 现象：更新（2026-04-01）：`resolveMerchantForUser`、`getMerchantFromUserID`、`MerchantOwnerOnlyMiddleware`、`MerchantStaffMiddleware` 已统一复用同一商户关联解析入口，并补了 owner-only 与 staff-compatible 差异测试，避免再把 staff 兼容能力写成隐式副作用；`merchant_finance` 中 manager 可读视图与 owner-only 提现动作也已改成分离 helper，避免继续用 owner 命名承载 staff-compatible 行为。下面保留的是首次审查快照，供追溯使用。
- 关键证据：
  [locallife/db/query/merchant.sql#L81](locallife/db/query/merchant.sql#L81)
  [locallife/api/merchant.go#L71](locallife/api/merchant.go#L71)
  [locallife/api/order.go#L267](locallife/api/order.go#L267)
  [locallife/api/appeal.go#L26](locallife/api/appeal.go#L26)
  [locallife/api/rbac_middleware.go#L167](locallife/api/rbac_middleware.go#L167)
  [locallife/db/sqlc/querier.go#L746](locallife/db/sqlc/querier.go#L746)
- 风险影响：
  1. 未来若有人按命名理解为“只限 owner”，极易在局部修复或重构时改坏 staff 能力，或反过来把 owner-only 语义误放宽。
  2. 权限边界的真实来源变得不透明，审查代码时很难仅从函数名和参数名判断当前行为。
  3. `resolveMerchantForUser` 这类 helper 仍在 `GetMerchantByOwner` 失败后再回退查 staff，暴露出调用层对底层语义的认知并不一致，后续容易产生重复逻辑或错误兜底。
- 待核问题：
  1. 是否应该统一引入 `resolveMerchantForUser` 或显式 `merchant scope` helper？
  2. 是否要把 owner-only 的 helper 与 staff-compatible 的 helper 明确拆开？

### RB-10 用户下单主路径保留调试日志

- 场景：外卖订单履约上游交易入口
- 等级：低
- 类型：调试残留
- 当前判断：已确认问题
- 现象：`createOrder` 请求路径中仍保留 `[DEBUG]` 日志，且在正常流程里打出请求解析和校验失败日志。当前日志内容主要包含 `merchant_id`、`order_type`、`items_count` 和 `address_id`，没有直接把整包请求体、菜品明细或备注打出，因此更像生产路径中的低优先级日志治理问题，而不是已确认的数据泄露缺陷。
- 关键证据：
  [locallife/api/order.go#L475](locallife/api/order.go#L475)
  [locallife/api/order.go#L485](locallife/api/order.go#L485)
  [locallife/api/order.go#L490](locallife/api/order.go#L490)
- 风险影响：
  1. 生产日志信噪比下降。
  2. 容易掩盖真正重要的异常日志。
  3. 将参数错误与正常观测都以 `[DEBUG]` 文案混在正式日志流里，会误导后续排障和告警分级。
- 待核问题：
  1. 这些日志是否仍在临时排障阶段？
  2. 是否应改成结构化 debug 级别或直接删除？

### RB-11 商户与集团/品牌的现行归属模型已切到 merchants 表直挂，但旧 binding 语义仍在多处残留

- 场景：商户与集团/品牌归属关系
- 等级：中
- 类型：结构语义漂移
- 当前判断：已部分收口；核心 SQL/query/事务/API 调用与生成代码、mock 已改为 affiliation 语义，但其余历史调用命名仍待继续清理。
- 现象：更新（2026-04-01）：门店归属的源头 SQL 名称与主要调用点已从 `GetMerchantGroupBinding` / `UpdateMerchantGroupBinding` 调整为更贴近 `merchants.group_id` / `brand_id` 事实模型的 affiliation 命名，避免继续把门店归属误表述为独立 binding 关系。下面保留的是首次审查快照，供追溯使用。
- 关键证据：
  [locallife/db/migration/000093_add_group_multi_store.up.sql#L111](locallife/db/migration/000093_add_group_multi_store.up.sql#L111)
  [locallife/api/merchant.go#L137](locallife/api/merchant.go#L137)
  [locallife/api/merchant.go#L138](locallife/api/merchant.go#L138)
  [locallife/db/query/group.sql#L183](locallife/db/query/group.sql#L183)
  [locallife/db/sqlc/tx_group.go#L105](locallife/db/sqlc/tx_group.go#L105)
  [locallife/db/sqlc/tx_group.go#L131](locallife/db/sqlc/tx_group.go#L131)
  [web/src/components/providers/merchant-session-provider.tsx#L18](web/src/components/providers/merchant-session-provider.tsx#L18)
  [web/src/components/providers/merchant-session-provider.tsx#L19](web/src/components/providers/merchant-session-provider.tsx#L19)
- 风险影响：
  1. 后续维护者很容易继续围绕“绑定表/绑定对象”加新抽象，造成同一事实关系被重复建模。
  2. `merchant_group_members` 表承载的是集团成员岗位，而不是门店归属；如果和门店 `group_id` / `brand_id` 混用，权限和租户边界很容易被误判。
  3. 审查代码时难以一眼判断“门店属于哪个集团/品牌”到底以哪个模型为准，增加后续集团能力扩展的出错概率。
- 待核问题：
  1. 是否应把 `GetMerchantGroupBinding` / `UpdateMerchantGroupBinding` 统一重命名为更贴近商户表事实的 helper / query？
  2. 是否应明确区分“门店归属关系”和“集团成员岗位关系”两套概念，避免在 handler 和 UI 中继续混称为 binding？

### RB-12 集团策略与菜单模板管理面已开放，但未真正接入菜单/库存/营销主链路，且前端文案明显超前

- 场景：集团多门店统一经营
- 等级：中
- 类型：未完整落地与文案漂移
- 当前判断：已部分收口；前端超前文案已向“组织管理 + 协同偏好记录”收敛，但主链路仍未接入集团中央化执行能力。
- 现象：更新（2026-04-01）：集团页面与加入须知中原先“统一分发菜单”“统一管理菜单、库存和营销”“策略即时同步至所有门店”等表述，已调整为更贴近当前真实能力的“门店归属、经营视图与协同偏好”描述，避免继续对外形成已接管主链路的承诺。下面保留的是首次审查快照，供追溯使用。
- 关键证据：
  [locallife/api/group.go#L1519](locallife/api/group.go#L1519)
  [locallife/api/group.go#L1571](locallife/api/group.go#L1571)
  [locallife/api/group.go#L1645](locallife/api/group.go#L1645)
  [locallife/db/query/group.sql#L198](locallife/db/query/group.sql#L198)
  [locallife/db/query/group.sql#L202](locallife/db/query/group.sql#L202)
  [locallife/db/query/group.sql#L207](locallife/db/query/group.sql#L207)
  [web/src/components/merchant/group-page-client.tsx#L171](web/src/components/merchant/group-page-client.tsx#L171)
  [web/src/components/merchant/group-page-client.tsx#L551](web/src/components/merchant/group-page-client.tsx#L551)
  [web/src/components/merchant/group-page-client.tsx#L556](web/src/components/merchant/group-page-client.tsx#L556)
  [web/src/components/merchant/settings-page-client.tsx#L1170](web/src/components/merchant/settings-page-client.tsx#L1170)
  [web/src/components/merchant/settings-page-client.tsx#L1171](web/src/components/merchant/settings-page-client.tsx#L1171)
- 风险影响：
  1. 集团管理员可能以为切换 `central` 后会真实接管门店菜单、库存和营销，但当前主链路大概率不会发生对应行为变化。
  2. 前端和产品文案已经对外形成“统一分发/统一管理”的能力承诺，容易导致上线后认知落差和误操作。
  3. 后续如果在部分链路零散接入 group policy，而没有统一规则，容易形成门店有的能力被集团接管、有的仍本地生效的碎片状态。
- 待核问题：
  1. 既然当前定位已确定为“多店经营视图与组织管理”，哪些中央化相关文案、交互和残留逻辑应优先移除？
  2. 后续若上线后很快开展中央化，菜单、价格、库存、营销四类策略应按什么顺序分批接入？

### RB-13 菜品多规格链路基础可用，但创建不原子且模型只支持单选必选

- 场景：菜品多规格管理
- 等级：中
- 类型：一致性风险与模型能力不足
- 当前判断：已部分收口；创建原子性问题已修复，模型能力上限仍维持当前单组选单选口径。
- 现象：更新（2026-04-01）：创建菜品时，基础菜品、图片和规格组选项已并入同一个事务入口；若规格写入失败，会整体回滚，不再残留无规格的半成品菜品。当前仍保留“多个规格组并行、每组单选”的模型约束，复杂多选能力不在本轮范围内。下面保留的是首次审查快照，供追溯使用。
- 关键证据：
  [locallife/api/dish.go#L510](locallife/api/dish.go#L510)
  [locallife/api/dish.go#L550](locallife/api/dish.go#L550)
  [locallife/api/dish.go#L1574](locallife/api/dish.go#L1574)
  [locallife/api/dish.go#L1703](locallife/api/dish.go#L1703)
  [locallife/db/sqlc/tx_dish.go#L35](locallife/db/sqlc/tx_dish.go#L35)
  [locallife/db/sqlc/tx_dish.go#L223](locallife/db/sqlc/tx_dish.go#L223)
  [locallife/api/customization_validator.go#L42](locallife/api/customization_validator.go#L42)
  [locallife/api/customization_validator.go#L152](locallife/api/customization_validator.go#L152)
- 风险影响：
  1. 若菜品创建成功但规格事务失败，接口会报错，但数据库里可能已经留下没有规格的半成品菜品，产生数据一致性问题。
  2. 当前模型已经能覆盖“大小 + 冷热 + 辣度”这类多个规格组并行、每组单选的场景，但不适合配料多选、至少选 2 个、最多选 3 个等更复杂的菜单结构。
  3. 创建菜品携带规格的 handler 级直接测试已经补上，但规格编辑链路与订单侧必选校验的直接覆盖仍然偏弱。
- 待核问题：
  1. 是否应把“创建菜品 + 设置规格”并入同一个事务入口，避免失败后残留半成品菜品？
  2. 是否应继续补齐规格编辑接口和订单必选校验的直接自动化测试？

### RB-14 当前允许将包装物作为独立菜品售卖，但不支持菜单层强制二选一这类必选品约束

- 场景：菜单编排与必选品
- 等级：中
- 类型：能力缺口
- 当前判断：已部分收口；owner 侧包装策略配置和 `CreateOrder` 终态硬校验已落地，购物车前置提示与复杂数量联动仍未纳入本轮。
- 现象：更新（2026-04-01）：已新增门店级 `merchant_packaging_policies` 配置，owner 可维护生效订单类型与候选包装菜品；`CreateOrder` 在外卖/打包场景下会读取该策略，并对订单项执行“候选包装菜品必须恰选 1 个”的后端硬校验。这样“包装物作为普通菜品售卖”与“命中策略时必须选一个包装物”已在后端最小闭环中打通。当前仍未扩展购物车即时预检，也未实现“包装数量跟随主商品数量”等复杂联动。下面保留的是首次审查快照，供追溯使用。
- 关键证据：
  [locallife/db/migration/000181_add_merchant_packaging_policies.up.sql#L1](locallife/db/migration/000181_add_merchant_packaging_policies.up.sql#L1)
  [locallife/db/query/merchant_packaging_policies.sql#L3](locallife/db/query/merchant_packaging_policies.sql#L3)
  [locallife/logic/packaging_policy.go#L17](locallife/logic/packaging_policy.go#L17)
  [locallife/logic/order_service.go#L142](locallife/logic/order_service.go#L142)
  [locallife/api/packaging_policy.go#L49](locallife/api/packaging_policy.go#L49)
  [locallife/logic/packaging_policy_test.go#L13](locallife/logic/packaging_policy_test.go#L13)
  [locallife/api/packaging_policy_test.go#L15](locallife/api/packaging_policy_test.go#L15)
- 风险影响：
  1. 当前后端已经能表达并校验“外卖/打包必须从候选包装菜品中恰选 1 个”，原始能力缺口已消除。
  2. 购物车层尚未提供即时提示；如果前端希望在提交订单前更早反馈，仍需额外接入预检或前置校验。
  3. 当前策略只解决“是否选了且只选 1 个”，还没有覆盖“包装数量跟随主商品数量”“按分类自动识别包装菜品”等更复杂规则。
- 待核问题：
  1. 是否需要在购物车层补一层预检提示，降低用户在下单提交时才收到错误的体验成本？
  2. 后续若出现“一份主商品对应一份包装”之类规则，是否要在现有订单级策略上继续扩字段，还是另起更明确的数量联动模型？

## RB-11 到 RB-14 统一修复策略稿

这一组问题不适合逐条零散修，建议先按统一准绳收口，再分两批落地。

统一准绳：

1. 集团/品牌现行事实模型，以 `merchants.group_id` 和 `merchants.brand_id` 为唯一门店归属真相；`merchant_group_members` 只表示集团人员岗位，不表示门店归属。
2. 在集团策略真正接入经营主链路前，所有“统一分发/统一管理菜单库存营销”的对外表述都应视为超前承诺，而不是已生效能力。
3. 菜品多规格当前只定义为“多个规格组并行 + 每组单选 + 可选加价 + 是否必选”，不要在现有模型上默认承诺组内多选、最少/最多选择数。
4. “菜品规格必选”和“菜单层包装物必选”必须分开建模、分开描述、分开修复，不应混用同一套术语。

### RB-11 统一修复口径

- 现行准绳：商户归属关系以 merchants 表字段为准，binding 只是历史命名残留。
- 短期止血：统一代码和文档术语，把 query / helper / 前端类型中的 binding 语义改成 merchant affiliation / merchant group relation 之类更贴近事实的命名，至少先消除“像是有独立绑定表”的误导。
- 长期收敛：把“门店归属关系”和“集团成员岗位关系”抽成两套明确 helper，避免在后续集团能力扩展时继续混用。
- 推荐批次：先做命名和注释收口，再处理依赖这些语义的权限 helper。

### RB-12 统一修复口径

- 现行准绳：集团与品牌当前只服务于多店经营视图和组织管理，不算已接管门店经营主链路。
- 短期止血：收敛前端文案与交互，并移除残留的中央化逻辑表达；未生效的地方不要继续对商户做强承诺。
- 长期收敛：按菜单、价格、库存、营销四条线分批接入 group policy；每接入一条线，再补对应模板下发、覆盖规则和回归测试，不建议一次性做成“大一统中央化”。
- 推荐批次：先收文案和交互，再决定哪一条主链路最先中央化，优先选边界最清晰的一条，不要四条同时开工。

### RB-13 统一修复口径

- 现行准绳：多规格当前能力边界是“多个规格组并行、每组单选”，不是通用 modifier engine。
- 短期止血：先把“创建菜品 + 写规格”并成一个事务入口，消除半成品菜品残留问题；同时补上规格管理接口和订单必选校验的直接测试。
- 长期收敛：如果未来确认需要组内多选、最少/最多选择数，再扩模型字段和下单校验，不要在现有 `map[group_id]option_id` 结构上硬撑。
- 推荐批次：先修事务一致性，再决定是否扩模型能力；这两步不要绑在同一个需求里。

### RB-14 统一修复口径

- 现行准绳：包装商品继续沿用普通菜品模型；“必须选一个包装”已通过订单级 packaging policy 进入 owner 配置与下单硬校验链路。
- 短期止血：已完成。当前最小闭环已覆盖商户配置、候选菜品校验、外卖/打包下单硬拦截，以及对应 API/logic 回归。
- 长期收敛：若后续需要更好的交互，可在购物车层增加预检提示；若需要“包装数量跟随主商品数量”“按分类自动匹配包装”等复杂规则，再扩展当前策略模型，而不是回退成菜单级 required group 或包装专门标识。
- 推荐批次：本轮到此为止；后续只把购物车预检和复杂数量联动视为独立增强项。

## 推荐落地顺序

1. 先做口径收口包：RB-11、RB-12。
2. 再做工程止血包：RB-13 的模型能力边界与规格编辑/下单校验补测。
3. 最后再决定是否立新能力包：集团中央化经营、多选规格模型，以及包装策略的购物车预检/数量联动增强。

## 建议审查顺序

建议按下面顺序逐条深挖，优先处理权限和资金边界：

1. RB-01 商户开户权限未收口
2. RB-02 manager 提现权限是否合理
3. RB-03 桌台管理权限漂移
4. RB-04 商户预订管理权限漂移
5. RB-05 设备与展示配置权限漂移
6. RB-07 商户投诉链路缺少自动化验证
7. RB-08 商户代客预订缺少直接验证
8. RB-06 打印机测试接口未实现
9. RB-09 `GetMerchantByOwner` 语义漂移导致的结构耦合
10. RB-11 商户与集团/品牌关系的现行模型与残留语义漂移
11. RB-12 集团中央化策略/模板未接经营主链路
12. RB-13 菜品多规格实现的原子性与模型能力问题
13. RB-14 包装策略收口余项（购物车预检 / 复杂联动）
14. RB-10 下单调试日志残留

## 逐条审查时建议固定回答的四个问题

每条风险进入详细审查时，统一回答以下问题：

1. 这是已确认问题，还是仅风险假设？
2. 影响的具体角色、业务动作和数据边界是什么？
3. 是否已有测试覆盖，覆盖到哪一层？
4. 如果确认要修，最小修复方案是什么？