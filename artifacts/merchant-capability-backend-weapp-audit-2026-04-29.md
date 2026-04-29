# 商户能力后端与 Weapp 承接审计

日期：2026-04-29

## 审计目标

本文件完整审查当前后端为商户提供的能力，并按商户任务域汇总 Weapp 商户侧已经承接、部分承接、尚未承接或不应作为 Weapp 缺口处理的能力。

审计重点不是接口数量，而是能力是否形成可用任务闭环：路由、权限、handler、业务逻辑或持久化、Weapp API 封装、页面入口、页面状态与动作是否贯通。

## 判定口径

- 后端事实来源优先级：`locallife/api/server.go` 的真实路由注册 > `locallife/api/*.go` handler 与 Swagger 注解 > logic/db/sqlc 运行路径。
- Weapp 事实来源优先级：`weapp/miniprogram/app.json` 页面注册 > `weapp/miniprogram/pages/merchant/**` 页面调用 > `weapp/miniprogram/api/**` API 封装 > service 编排。
- 已闭环：后端能力、Weapp API、商户页面或页面组形成真实任务路径。
- 部分承接：后端能力已暴露，Weapp 有 API 封装但页面未消费，或页面用另一种任务模型覆盖主要场景。
- 未承接：后端商户能力已暴露，但 Weapp 商户侧未发现 API 封装或页面入口。
- 非 Weapp 缺口：接口服务 Flutter 商户 App、顾客侧、平台/集团管理、诊断运维，或当前后端未暴露为商户配置接口。
- 风险等级按工程治理口径粗分：G0 只读/展示，G1 普通经营状态，G2 状态/资金/权限/异步恢复，G3 支付、提现、回调、私密材料或高影响安全路径。

## 总览结论

| 维度 | 结论 |
| --- | --- |
| 后端商户能力域 | 24 个主域，覆盖订单、后厨、菜品、桌台、预订、财务、收付通、结算账户、员工、投诉、索赔、会员营销、集团等。 |
| Weapp 商户页面 | `app.json` 当前注册 58 个商户页面入口，包含列表、详情、编辑与结果页。 |
| 主能力承接 | 订单、后厨、菜品套餐库存、桌台包间、预订、打印设备、经营统计、财务总览、流水、结算、提现、取消提现、收付通开户、结算账户、员工、投诉、索赔申诉、评价、会员营销、集团申请/加入均已形成页面闭环。 |
| 高风险已补齐 | 取消提现已从缺口补齐为 G2 商户侧闭环：提现详情承接资格、`NOT_APPLY_WITHDRAW`、`APPLY_WITHDRAW`、收款资料、私有材料上传、提交后回读与申请详情。 |
| API 已封装但页面未消费 | 多门店列表、预订工作台属于 Weapp API 层已有但商户页未直接消费或未形成独立页面的能力。 |
| 旧矩阵需修正 | 2026-04-06 文档中“结算账户未承接”已过期；当前结算账户已注册页面并调用查询、修改、申请查询接口。包装策略当前未见后端商户配置路由，不应再列为“后端已暴露、Weapp 未接”。 |

## 后端能力域与 Weapp 承接矩阵

| 能力域 | 后端能力与主要路由 | 主要后端文件 | Weapp 承接 | 当前判断 | 风险 |
| --- | --- | --- | --- | --- | --- |
| 工作台聚合 | 订单摘要、经营概览、投诉摘要、预订统计、营业状态；`GET /v1/merchant/orders/summary`、`GET /v1/merchant/stats/overview`、`GET /v1/merchant/complaints/summary`、`GET /v1/reservations/merchant/stats`、`GET/PATCH /v1/merchants/me/status` | `locallife/api/order.go`、`merchant_stats.go`、`complaint.go`、`table_reservation.go`、`merchant.go` | `pages/merchant/dashboard/index`，通过 `merchant-dashboard`、`merchant-open-status`、`merchant-applyment-console` 聚合 | 已闭环 | G1 |
| 商户 App 绑定入口 | Weapp 生成绑定码，Flutter 商户 App 设备注册/心跳/注销；`POST /v1/auth/app-bind/code`，`POST/PUT/DELETE /v1/merchant/device*` | `locallife/api/auth.go`、`merchant_app_device.go` | dashboard 弹层调用 `services/merchant-app-bind.ts` 生成绑定码；`/v1/merchant/device/*` 为 Flutter App 运行时接口 | Weapp 承接绑定码，不承接 App 设备心跳；不是 Weapp 缺口 | G1 |
| 订单管理 | 列表、详情、接单、拒单、出餐、完结；`/v1/merchant/orders*` | `locallife/api/order.go` | `orders/list`、`orders/detail`、`order-management.ts` | 已闭环 | G1 |
| 打印任务与异常 | 订单打印任务、重试、状态、异常列表；`/v1/merchant/orders/{id}/print-jobs*`、`/v1/merchant/orders/print-anomalies` | `locallife/api/order.go` | `orders/detail`、`orders/print-anomalies` | 已闭环 | G1 |
| 后厨 KDS | 后厨订单列表、详情、开始制作、标记出餐；`/v1/kitchen/orders*` | `locallife/api/kitchen.go` | `kitchen/index`、`kitchen/detail` | 已闭环 | G1 |
| 经营统计 | 日统计、概览、热销菜品、客群、时段、来源、复购、品类；`/v1/merchant/stats/**` | `locallife/api/merchant_stats.go` | `stats/index`、`stats/customers`、`stats/customers/detail` | 已闭环 | G0 |
| 菜品分类与菜品 | 分类 CRUD、菜品 CRUD、上下架、批量上下架、定制、规格别名、推荐标签；`/v1/dishes/**` | `locallife/api/dish.go` | `dishes/index`、`dishes/edit`、`dishes/categories`、`dish.ts` | 已闭环 | G1 |
| 套餐 | 套餐 CRUD、上下线、菜品关联；`/v1/combos/**` | `locallife/api/combo.go` | `combos/index`、`combos/edit` | 已闭环 | G1 |
| 库存 | 创建/查询/更新日库存、单品库存、库存统计；`/v1/inventory/**` | `locallife/api/inventory.go` | `inventory/index` | 已闭环；`POST /inventory/check` 更偏订单内部扣减前检查，不作为商户页面主任务 | G1 |
| 桌台与包间管理 | 桌台 CRUD、状态、二维码、图片、标签；`/v1/tables/**` | `locallife/api/table.go` | `tables/index`、`tables/edit`、`table-device-management.ts` | 已闭环；商户页用 `table_type=room` 统一承接包间管理 | G1 |
| 房间专用读取 | `GET /v1/merchants/{id}/rooms`、`/rooms/all`、`GET /v1/rooms/{id}`、`/availability` | `locallife/api/table.go` | `room.ts` 有封装，但商户页未直接消费 | 主要是顾客/预订可用性读取；商户管理已通过桌台统一模型承接，不是当前商户 Weapp 缺口 | G0 |
| 预订 | 商户列表、工作台、今日、统计、可预约菜品、代客创建、确认、完结、更新、爽约、签到、起菜、加菜、改菜；`/v1/reservations/merchant*`、`/v1/reservations/{id}/*` | `locallife/api/table_reservation.go`、`table_reservation_workbench.go` | `reservations/index`、`reservations/edit`、`reservation.ts` | 主链已闭环；`getMerchantReservationWorkbench` 已封装但页面未见直接调用 | G1 |
| 打印机与展示配置 | 打印机 CRUD、状态、测试、同步恢复、设备访问、展示配置；`/v1/merchant/devices*`、`/v1/merchant/display-config` | `locallife/api/device.go`、`device_reconciliation.go` | `printers/index`、`printers/edit`、`settings/display-config` | 已闭环 | G1 |
| 财务总览与流水 | 概览、订单流水、服务费、促销流水、日汇总、结算单、结算时间线；`/v1/merchant/finance/**` | `locallife/api/merchant_finance.go` | `finance/index`、`finance/bills/index`、`finance/settlements/index`、`merchant-finance.ts`、`merchant-finance-workflow.ts` | 已闭环 | G1/G2 |
| 账户余额与提现 | 余额、提现列表、发起提现、单笔提现详情；`GET /account/balance`、`GET /account/withdrawals`、`GET /account/withdrawals/:id`、`POST /account/withdraw` | `locallife/api/merchant_finance.go` | `finance/withdrawals/index`、`finance/withdrawals/create/index`、`finance/withdrawals/detail/index` | 已闭环；提现提交后等待查询后端真值再切状态 | G2 |
| 取消提现 | 资格、申请列表、详情、提交；`GET /account/cancel-withdraw/eligibility`、`GET /account/cancel-withdraw/applications`、`GET /account/cancel-withdraw/applications/:id`、`POST /account/cancel-withdraw/applications` | `locallife/api/merchant_cancel_withdraw.go`、`merchant_cancel_withdraw_fact.go` | `finance/withdrawals/detail/index`、`finance/cancel-withdraw/detail/index`、`merchant-finance.ts`、`merchant-finance-workflow.ts` | 已闭环；`APPLY_WITHDRAW` 上游材料审查和商户确认链接需联调 | G2 |
| 结算账户资料 | 查询当前结算账户、提交修改、查询修改申请；`GET/POST /v1/merchant/finance/account/settlement-account`、`GET /applications/:application_no` | `locallife/api/settlement_account.go` | `settings/applyment/index` 展示入口；`settings/applyment/settlement-account/index` 调用 `getMerchantSettlementAccount`、`modifyMerchantSettlementAccount`、`getMerchantSettlementApplication` | 已闭环；旧矩阵“未承接”结论已过期 | G2 |
| 收付通进件与绑卡 | 开户状态、绑卡、银行/支行/省市目录；`/v1/merchant/applyment/**` | `locallife/api/ecommerce_applyment.go`、`applyment_bank_catalog.go` | `settings/applyment/index`、`action`、`submit`、`settlement-account` | 已闭环 | G2 |
| 门店资料与营业设置 | 当前商户、更新资料、门头图、营业状态、营业时间、标签、会员设置；`/v1/merchants/me/**` | `locallife/api/merchant.go`、`membership.go` | `settings/profile`、`profile-images`、`settings/business-hours`、`merchant-categories`、`settings/membership` | 已闭环 | G1 |
| 多门店列表 | `GET /v1/merchants/my` | `locallife/api/merchant.go` | `merchant.ts` 有 `listMyMerchants()`，未发现商户页面消费 | API 已封装但页面未承接；主要给集团/品牌或多门店主体使用，并非所有商户默认能力 | G0/G1 |
| 商户主体申请与材料 | 草稿、基础资料、图片、删除材料、提交、重置；`/v1/merchant/application/**` | `locallife/api/merchant_application.go` | `settings/application`、`onboarding.ts`、`ocr-jobs.ts` | 已闭环 | G2 |
| OCR 与媒体 | OCR 创建/查询/结果/重试/批量查询/死信；媒体上传会话、完成、删除；`/v1/ocr/jobs/**`、`/v1/media/**` | `locallife/api/ocr.go`、`media.go` | 申请、集团申请、门店图片等页面使用 OCR/媒体能力 | 主链已闭环；`batch-query` 与 `dead-letter` 更偏批量/诊断，未形成商户页面缺口 | G2/G0 |
| 员工管理 | 员工列表、邀请码、直接新增、改角色、移除；`/v1/merchant/staff/**` | `locallife/api/staff.go` | `staff/index`、`merchant-staff.ts` | 已闭环；页面采用邀请码模式，未承接直接新增员工，不视为缺口 | G1 |
| 投诉处理 | 列表、摘要、详情、回复、完结；`/v1/merchant/complaints/**` | `locallife/api/complaint.go` | `complaints/index`、`complaints/detail` | 已闭环；当前 Swagger 注解已覆盖详情/回复/完结 | G2 |
| 索赔、追偿、申诉 | 索赔列表/摘要/详情/判责、追偿查询/支付、追偿争议、申诉提交/列表/摘要/详情、风险用户；`/v1/merchant/claims*`、`/v1/merchant/recoveries*`、`/v1/merchant/recovery-disputes*`、`/v1/merchant/appeals*`、`/v1/merchant/risk/users/:id` | `locallife/api/claim_recovery.go`、`appeal.go`、`recovery_dispute.go`、`behavior_trace.go` | `claims/index`、`claims/detail`、`appeals/index`、`appeals/detail`、`appeals-customer-service.ts`、`claim-recovery-payment.ts` | 索赔/申诉主链已闭环；追偿争议是否面向商户页需按产品边界复核 | G2 |
| 评价 | 商户查看全部评价、评价详情、回复；`GET /v1/reviews/merchants/:id/all`、`GET /v1/reviews/:id`、`POST /v1/reviews/:id/reply` | `locallife/api/review.go` | `reviews/index`、`review.ts` | 已闭环 | G0/G1 |
| 会员与充值 | 会员设置、会员列表/详情、线下代录充值、余额调整、充值规则 CRUD、active 读口；`/v1/merchants/:id/members/**`、`/recharge-rules/**` | `locallife/api/membership.go` | `settings/membership`、`settings/members`、`settings/recharge-rules`、`settings/recharge-rules/edit` | 管理主链已闭环；active 读口未被页面消费，属于便利读口 | G2/G1 |
| 代金券 | 代金券 CRUD、active 读口；`/v1/merchants/:id/vouchers/**` | `locallife/api/voucher.go` | `vouchers/index`、`vouchers/edit`、`coupon.ts` | 管理主链已闭环；`vouchers/active` 有 API 封装但商户管理页不依赖 | G1 |
| 满减规则 | 创建、列表、详情、active、applicable、best、更新、删除；`/v1/merchants/:id/discounts/**` | `locallife/api/discount.go` | `discount-rules/index`、`discount-rules/edit`、`merchant.ts` | 管理主链已闭环；`active/applicable/best` 读口更偏下单/便利读取，商户页未消费 | G1 |
| 配送促销 | 配送优惠 CRUD；`/v1/delivery-fee/merchants/:merchant_id/promotions/**` | `locallife/api/delivery_fee.go` | `delivery-promotions/index`、`delivery-promotions/edit` | 已闭环 | G1 |
| 集团申请与加入 | 集团申请草稿/更新/删除材料/提交、搜索集团、加入集团；`/v1/groups/applications/**`、`/v1/groups`、`/v1/groups/:id/join-requests` | `locallife/api/group.go` | `group/application`、`group/join` | 已闭环 | G1/G2 |
| 集团品牌/商户管理 | 集团详情、集团商户、品牌列表/创建、政策、菜单模板；`/v1/groups/:id/**`、`/v1/brands/:id/**` | `locallife/api/group.go` | 商户页仅承接申请/加入，不承接集团治理 | 可能属于集团负责人/平台治理后续任务，不作为当前商户控制台缺口 | G1 |
| 包装策略 | 当前未见 `/v1/merchants/me/packaging-policy` 路由；后端仅见下单校验逻辑 `logic/packaging_policy.go` | `locallife/logic/packaging_policy.go` | Weapp 未见页面或 API | 旧矩阵中包装策略应降级为内部订单校验事实；若要商户配置，需先补后端接口 | G1 |

## Weapp 页面承接清单

当前 `weapp/miniprogram/app.json` 的商户分包注册 58 个页面：

| 页面组 | 页面 | 承接能力 |
| --- | --- | --- |
| 工作台 | `dashboard/index` | 工作台聚合、营业状态、开户状态、商户 App 绑定码入口 |
| 后厨 | `kitchen/index`、`kitchen/detail/index` | KDS 列表、详情、开始制作、出餐 |
| 财务 | `finance/index`、`finance/withdrawals/index`、`finance/withdrawals/create/index`、`finance/withdrawals/detail/index`、`finance/cancel-withdraw/detail/index`、`finance/bills/index`、`finance/settlements/index` | 财务总览、余额、提现、取消提现、流水、结算 |
| 经营统计 | `stats/index`、`stats/customers/index`、`stats/customers/detail/index` | 经营概览、客群、客户详情 |
| 员工 | `staff/index` | 员工列表、角色修改、移除、邀请码 |
| 设置 | `settings/profile/index`、`settings/business-hours/index`、`settings/membership/index`、`settings/members/index`、`settings/recharge-rules/index`、`settings/recharge-rules/edit/index`、`settings/application/index`、`settings/display-config/index` | 门店资料、营业时间、会员设置、会员与调账、充值规则、主体申请、展示配置 |
| 收付通 | `settings/applyment/index`、`settings/applyment/action/index`、`settings/applyment/submit/index`、`settings/applyment/settlement-account/index` | 开户状态、绑卡、开户动作、结算账户查询/修改/申请查询 |
| 集团 | `group/application/index`、`group/join/index` | 集团申请、加入集团 |
| 配置导航 | `config/index` | 商户控制台导航和路由分发，不是后端能力页 |
| 客服纠纷 | `complaints/index`、`complaints/detail/index`、`claims/index`、`claims/detail/index`、`appeals/index`、`appeals/detail/index`、`reviews/index` | 投诉、索赔、追偿支付、申诉、评价 |
| 菜品套餐库存 | `dishes/index`、`dishes/categories/index`、`dishes/edit/index`、`combos/index`、`combos/edit/index`、`inventory/index` | 菜品、分类、定制、套餐、库存 |
| 预订 | `reservations/index`、`reservations/edit/index` | 预订列表、详情编辑、代客创建和状态动作 |
| 订单 | `orders/list/index`、`orders/detail/index`、`orders/print-anomalies/index` | 订单处理、详情、打印任务、打印异常 |
| 桌台设备 | `tables/index`、`tables/edit/index`、`printers/index`、`printers/edit/index` | 桌台/包间、桌台资产、打印机、同步恢复 |
| 门店图片 | `profile-images/index` | Logo、门头图、媒体资产 |
| 营销 | `discount-rules/index`、`discount-rules/edit/index`、`delivery-promotions/index`、`delivery-promotions/edit/index`、`vouchers/index`、`vouchers/edit/index` | 满减、配送促销、代金券 |

## 缺口与边界清单

### P0 / G2：后端已暴露但 Weapp 未承接

| 能力 | 后端接口 | 当前证据 | 建议 |
| --- | --- | --- | --- |
| 当前无新增 P0 | - | 财务、提现、取消提现已在本轮补齐商户侧页面和 workflow | 剩余高风险项转为联调风险：微信上游 `APPLY_WITHDRAW` 材料审查、商户确认链接打开方式、外部回调时序 |

### P1：API 已封装但页面未消费或未形成任务入口

| 能力 | Weapp 现状 | 处理建议 |
| --- | --- | --- |
| 多门店列表 | `merchant.ts::listMyMerchants()` 调用 `/v1/merchants/my`，商户页未发现消费 | 按集团/品牌或多门店主体能力讨论，不作为所有商户默认入口 |
| 预订工作台 | `reservation.ts::getMerchantReservationWorkbench(date)` 已封装，`pages/merchant/reservations/**` 未发现调用 | 如果当前列表页已经满足任务，可不强制接；若要减少页面端聚合，应改用 workbench 作为预订主页数据源 |
| 代金券/充值/满减 active 读口 | `coupon.ts` 有 `vouchers/active`，充值/满减页面主要用列表接口和本地状态 | 管理页不必强制消费 active；若顾客下单或首页展示需要，应由对应顾客/下单任务域承接 |

### P2：不作为当前 Weapp 商户缺口

| 能力 | 原因 |
| --- | --- |
| 商户 App 设备注册/心跳/注销 | 服务 Flutter 商户 App 运行时。Weapp 只需要生成绑定码，当前 dashboard 已承接绑定码入口。 |
| 房间专用查询 | 主要服务顾客/预订可用性读取。商户侧包间管理已通过桌台模型统一承接。 |
| 直接新增员工 | 后端有 owner 写口，但商户页采用邀请码邀请员工，当前产品路径更安全，不视为未接。 |
| OCR batch-query / dead-letter | 批量查询和死信偏诊断/运维；商户页面主流程已通过单任务 OCR 查询闭环。 |
| 集团品牌、集团商户、集团政策、菜单模板 | 当前商户页只承接集团申请与加入；集团治理需要另定义集团负责人或平台任务域。 |
| 包装策略配置 | 当前后端没有商户配置路由，只有下单校验逻辑；不能把它算作 Weapp 未承接的后端商户能力。 |

## 与 2026-04-06 映射矩阵的主要差异

| 主题 | 旧矩阵结论 | 当前代码事实 | 当前处理 |
| --- | --- | --- | --- |
| 结算账户资料 | 后端已暴露，Weapp 未承接 | `app.json` 注册 `settings/applyment/settlement-account/index`；页面调用查询、修改、申请查询；入口页也展示结算账户状态 | 改为已闭环 |
| 财务页面 | 旧矩阵记为 `merchant/finance` 已闭环 | 本轮已新增 `pages/merchant/finance/**` 页面组并消费 `merchant-finance.ts` | 当前改为已闭环 |
| 取消提现 | 旧矩阵未列 | 本轮已新增 API 封装、提现详情表单和取消提现申请详情页 | 当前改为已闭环；保留上游联调风险 |
| 预订工作台 | 旧矩阵未列 | 后端有 `/v1/reservations/merchant/workbench`，Weapp API 已封装，页面未见消费 | 标为 API 已封装、页面未消费 |
| 多门店列表 | 旧矩阵认为未承接 | API 已封装，但页面未消费 | 保持页面未承接，改成“集团/品牌或多门店主体能力，非所有商户默认入口” |
| 包装策略 | 旧矩阵列为已闭环 | 当前未见 API 路由或 Weapp 页面，只见订单下单校验逻辑 | 从商户页面缺口中移除；如需配置需补后端接口 |
| 投诉 Swagger | 旧矩阵提到注解不完整 | 当前 `complaint.go` 已有列表、摘要、详情、回复、完结 `@Router` 注解 | 不再列为当前漂移 |
| 员工邀请码 Swagger | 旧矩阵提到路径漂移 | 当前 `staff.go` 注解为 `/v1/merchant/staff/invite-code`，与真实路径一致 | 不再列为当前漂移 |
| 满减 Swagger | 旧矩阵提到暴露不完整 | 当前 `discount.go` 已有创建、列表、详情、active、applicable、best、更新、删除注解 | 不再列为当前漂移 |

## 建议任务卡

### MCAP-01 取消提现 Weapp 承接

状态：已完成。

风险：G2，资金路径与微信上游状态。

目标：在商户财务提现任务域中承接取消提现，而不是单独堆一个接口页。

完成范围：

- API：新增取消提现资格、申请列表、申请详情、提交申请封装。
- 页面：从提现详情进入，符合条件时显示取消提现动作；支持不提现注销、提现后注销、材料上传和申请详情回读。
- 状态：加载、资格不可用、提交中、微信处理中、失败、未知结果、重试/回读。
- 安全：owner-only 提交动作、防重复点击、不把微信处理中当成功。

### MCAP-02 多门店切换产品边界确认

风险：G0/G1，身份与当前商户上下文。

目标：确认 Weapp 商户端是否支持一个用户管理多个商户。

若支持：只面向集团/品牌或多门店主体，需要一个门店切换 owner，统一刷新 `current_merchant`、权限、工作台、缓存和返回重入状态。

若不支持普通单店商户：在映射矩阵标注为集团/品牌能力，不进入所有商户默认页面缺口。

### MCAP-03 提现详情与取消提现合并设计

状态：已完成。

风险：G2。

目标：避免只补单笔提现详情而不承接取消提现状态。当前已把单笔提现详情作为取消提现和提现失败原因的共同承载页。

### MCAP-04 预订主页数据源收敛评估

风险：G1。

目标：评估 `getMerchantReservationWorkbench` 是否应替代页面端多请求/聚合。如果不使用，应在文档中标注为 API 预留，不作为缺口。

## 审计收口

- 本文件最初为审计底稿；本轮已继续完成财务页面组、提现、取消提现和私有材料上传承接。
- 已运行 `npm run quality:check`、`go test ./media`、`go test ./api -run 'TestValidateCreateMerchantCancelWithdrawRequest|TestCreateMerchantCancelWithdrawApplicationRecords'`。
- 剩余风险集中在外部联调：取消提现 `APPLY_WITHDRAW` 的微信材料审查、商户确认链接打开方式、外部回调时序和真实弱网/重入体验。