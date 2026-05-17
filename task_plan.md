# 小程序端用户体验整改计划

## 目标

针对本轮 `weapp/` 用户视角审查发现的问题，制定一份可直接进入实施的整改计划：先修复会让用户卡住或点击无效的真实缺陷，再清理会误导后续开发和用户预期的历史债务，最后做低风险体验一致性改进。

## 范围

- 目标目录：`weapp/`
- 任务类型：整改计划，暂不实施业务代码
- 主要依据：
  - `.github/instructions/weapp-mini-program.instructions.md`
  - `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
  - `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`
  - `.github/standards/weapp/REVIEW_CHECKLIST.md`
- 已有审查证据：`findings.md`
- 进度记录：`progress.md`

## 非目标

- 不在小程序端臆造后端未支持的能力、字段、状态或优惠选择规则。
- 不顺手重构未被审查命中的页面和共享服务。
- 不处理当前工作区既有 Baofoo 结算账户改动；这些改动与本整改计划无关。
- 不把一次性整改计划沉淀到 `.github/prompts/` 或新增可复用提示模板。

## 优先级说明

- P0：用户可见功能断链，点击无效或导致流程卡死，优先修。
- P1：当前不一定阻断用户，但会误导后续开发、引入坏入口或在被复用时形成明显缺陷。
- P2：符合长期移动应用预期的体验一致性改进，低风险、可批量处理。

## 整改任务

### P0-1 堂食结账：收口未实现的就餐人数与优惠券交互

**用户问题**

堂食/预订结算页展示了可编辑“就餐人数”和优惠券选择弹窗，但页面逻辑没有对应状态和 handler。用户看到的是移动应用里熟悉的可操作控件，却无法真实完成动作，属于典型“半实现”功能。

**涉及文件**

- 修改：`weapp/miniprogram/pages/dine-in/checkout/checkout.wxml`
- 可能修改：`weapp/miniprogram/pages/dine-in/checkout/checkout.ts`
- 可能修改：`weapp/miniprogram/pages/dine-in/checkout/checkout.wxss`
- 仅用于核对合同：`weapp/miniprogram/services/dine-in-checkout.ts`
- 仅用于核对合同：`weapp/miniprogram/api/cart.ts`
- 仅用于核对合同：`weapp/miniprogram/api/order.ts`
- 仅用于核对合同：`locallife/api/cart.go`
- 仅用于核对合同：`locallife/api/order.go`

**实施边界**

- 优惠券选择必须先确认后端合同是否支持结算阶段传入并锁定 `voucher_id` 或 `user_voucher_id`。
- 若现有堂食下单路径不支持用户选择优惠券，则移除本地优惠券弹窗、`voucherVisible` 等陈旧引用，以及 `bind:claimVoucher="onClaimVoucher"` 这类未实现绑定；保留后端 `calculateCheckoutCart` 返回的 `voucher_trials` 只读展示。
- 若后端合同已支持用户选择优惠券，则必须完整打通：加载用户可用券、选择券、重新试算、下单透传、失败回滚、关闭弹窗和空态。不得只补空 handler。
- 就餐人数如果只是桌台/预约上下文的只读信息，则把 stepper 改成只读展示；如果后端下单或预约合同支持变更，则补齐 `onGuestCountChange` 并把人数传入后端支持的真实字段。

**不做**

- 不新增前端本地“选中优惠券即生效”的假状态。
- 不把 `voucher_trials` 包装成已选择优惠券。
- 不扩展结账页到完整优惠券中心；领券仍由 `merchant-promos` 或既有优惠中心承担。

**验收标准**

- 页面不存在未定义的 `onGuestCountChange`、`onClaimVoucher`、`onVoucherPopupChange`、`closeVoucherPopup`、`onClearVoucher`、`onSelectVoucher` 绑定。
- 页面不存在未定义的 `voucherVisible`、`voucherLoading`、`selectedVoucher`、`vouchers` 渲染依赖，除非它们已被真实状态和服务调用闭环支持。
- 结算金额仍以 `calculateCheckoutCart` 的后端结果为准。
- 用户能明确分辨“优惠已自动计算/券试算仅供参考”和“可主动选择优惠券”两种状态，不出现看似可点但无效的控件。
- 弱网失败时显示可恢复状态或 Toast，不把失败渲染成空态。

**验证**

- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run lint`
- 若实现了真实优惠券选择，再补跑相关手工路径：进入堂食结账、领券后刷新、选择/取消优惠券、提交订单、支付取消后返回。

**残余风险**

- 若后端当前仅提供 `voucher_trials` 试算而不支持结算页选择券，前端只能收掉假入口；“用户主动选券”需要单独后端合同任务。

### P0-2 外卖购物车：实现已下架商品移除闭环

**用户问题**

购物车提示“部分商品已下架，请移除后再结算”，同时每个已下架商品旁边有“移除”按钮，但 `onRemoveUnavailable` 没有实现。用户会被阻止下单，又无法按界面提示解除阻塞。

**涉及文件**

- 修改：`weapp/miniprogram/pages/takeout/cart/index.ts`
- 核对：`weapp/miniprogram/pages/takeout/cart/index.wxml`
- 复用：`weapp/miniprogram/api/cart.ts`
- 复用：`weapp/miniprogram/utils/takeout-cart-view.ts`

**实施边界**

- 新增 `onRemoveUnavailable`，使用现有 `CartAPI.removeFromCart(itemId, { loading: false })` 和 `removeLocalItem(itemId)`。
- 删除成功后复用现有本地重算逻辑：商户小计、已选商户、结算金额、全局购物车状态。
- 删除失败时给出明确 Toast，例如“移除失败，请重试”。
- 可增加一个轻量进行中集合或按钮禁用状态，防止重复点击同一个下架项造成重复请求；范围仅限该页面。

**不做**

- 不改变购物车分组、代取费计算、拆单支付规则。
- 不把“下架商品”重新变成可结算商品。
- 不新增整车刷新作为唯一成功路径，除非本地移除后数据不一致才触发兜底刷新。

**验收标准**

- 点击已下架商品“移除”会调用删除接口并从页面消失。
- 移除最后一个商品时，对应商户分组消失；移除最后一个商户分组时出现空购物车状态。
- 移除后 `onCheckout` 不再因为已移除商品阻塞。
- 连点“移除”不会出现重复 Toast、金额错乱或残留选中商户。

**验证**

- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run lint`
- 手工路径：构造含 `isAvailable=false` 的购物车项，点击移除，检查分组、金额、底部结算栏和空态。

**残余风险**

- 如果后端删除已下架商品接口对历史购物车项返回特殊错误，需要在页面按真实错误语义补充提示；不得吞掉失败。

### P1-1 平台工作台：清理未使用模板里的未注册入口

**用户问题**

`platform/dashboard/templates/pc-content.wxml` 中存在未注册路由 `/pages/platform/merchants/applications` 和 `/pages/platform/riders/applications`。当前主 `dashboard.wxml` 未引用这些模板，因此不是即时用户缺陷，但它会污染路由审查，也可能在后续响应式模板复用时重新暴露死入口。

**涉及文件**

- 修改或删除：`weapp/miniprogram/pages/platform/dashboard/templates/pc-content.wxml`
- 可能删除：`weapp/miniprogram/pages/platform/dashboard/templates/pc-content-full.wxml`
- 核对：`weapp/miniprogram/pages/platform/dashboard/dashboard.wxml`
- 核对：`weapp/miniprogram/app.json`

**实施边界**

- 先确认 `templates/` 下这些文件是否被任何 WXML import/template 引用。
- 若确认未引用，删除或移到明确的历史归档位置；优先删除死模板，减少未来误用。
- 若确认仍有构建期或运行期引用，则把入口改为已注册、真实存在的审核页，或移除对应 grid item。

**不做**

- 不为模板里的旧链接新增空页面。
- 不把平台工作台重新设计为多端响应式页面。
- 不改变当前可见 `dashboard.wxml` 的业务入口，除非证明确实引用了旧模板。

**验收标准**

- 静态搜索不到指向未注册页面的 `/pages/platform/merchants/applications` 和 `/pages/platform/riders/applications`。
- 平台 dashboard 当前可见入口仍能跳转到已注册页面。
- 路由审查不再因死模板报告这两个字面链接。

**验证**

- 在仓库根目录运行：`rg -n "merchants/applications|riders/applications" weapp/miniprogram`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`
- 若删除模板，再运行：`PATH="$HOME/.local/bin:$PATH" npm run quality:check`

**残余风险**

- 如果某个非标准构建脚本动态拼接模板路径，纯文本搜索可能漏掉；删除前需要确认页面 JSON、WXML import 和构建脚本。

### P1-2 地图组件：去掉演示坐标和不存在的 marker 资源

**用户问题**

地图组件包含北京演示坐标和不存在的 marker 图片路径。用户一旦在配送或订单页看到该组件，会产生“定位错了”或“图标裂了”的强烈不信任感。

**涉及文件**

- 修改或删除：`weapp/miniprogram/components/map-view/index.ts`
- 修改或删除：`weapp/miniprogram/components/map-view/index.wxml`
- 修改或删除：`weapp/miniprogram/components/map-view/index.wxss`
- 修改：`weapp/miniprogram/components/delivery-map/index.ts`
- 核对：`weapp/miniprogram/components/delivery-map/index.wxml`
- 核对资源：`weapp/miniprogram/assets/merchant.png`
- 核对资源：`weapp/miniprogram/assets/customer.png`
- 核对资源：`weapp/miniprogram/assets/rider.png`

**实施边界**

- `map-view` 如果没有真实调用方，删除该演示组件及其组件声明；若有调用方，改成纯 props 驱动，不允许默认北京坐标作为业务位置。
- `delivery-map` 保留真实 props 驱动，但 marker 使用存在的资源或不传 `iconPath` 走小程序默认 marker。
- 当 merchant/customer/rider 坐标缺失时，显示明确空态或不渲染对应 marker；不得用北京坐标兜底。
- 地图中心点必须来自有效坐标中的优先级：骑手位置、商户位置、用户位置，而不是硬编码地理点。

**不做**

- 不新增地图路线规划服务。
- 不改变后端配送位置合同。
- 不把坐标缺失伪装成默认地点。

**验收标准**

- 搜索不到 `/assets/marker_shop.png`、`/assets/marker_user.png`、`/assets/images/marker-merchant.png`、`/assets/images/marker-customer.png`、`/assets/images/marker-rider.png` 这些不存在资源路径。
- 没有真实坐标时不会出现北京演示点。
- 有商户、用户、骑手坐标时 marker 和连线按真实数据显示。

**验证**

- 在仓库根目录运行：`rg -n "marker_shop|marker_user|marker-merchant|marker-customer|marker-rider|39\\.98|116\\.31|39\\.909|116\\.397" weapp/miniprogram/components`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`
- 组件被页面使用时，手工检查有坐标、部分坐标缺失、全缺失三种状态。

**残余风险**

- 如果调用方本身传入的经纬度来自旧 mock 数据，组件修复只能阻止组件层造假；调用方还需要单独追踪数据来源。

### P1-3 增加 WXML 事件绑定静态审查脚本

**用户问题**

本次两个 P0 问题都属于 WXML 绑定了不存在的页面动作。单靠 TypeScript compile 没有抓住这类缺陷，需要一个轻量审查门禁，至少覆盖同页 Page handler 的显性遗漏。

**涉及文件**

- 新增：`weapp/scripts/check-wxml-handler-bindings.js`
- 修改：`weapp/package.json`

**实施边界**

- 脚本扫描 `miniprogram/pages/**/*.wxml` 的 `bindtap`、`bind:tap`、`bindchange`、`bind:change`、`catchtap` 等常见事件绑定。
- 对同路径 `.ts` 或 `.js` 中 Page 对象内的同名方法做静态匹配。
- 对已确认由 behaviors 或 runtime method spread 提供的页面建立白名单，白名单必须写清来源文件。
- 先作为 `npm run check:wxml-handlers` 独立脚本；确认误报可控后再并入 `quality:check` 或 `gate:weapp`。

**不做**

- 不试图一次性构建完整小程序模板编译器。
- 不扫描 `miniprogram_npm`。
- 不把组件内部事件误判为页面 handler；共享组件可作为后续增强。

**验收标准**

- 脚本能抓出本轮已确认的两类缺陷，如果代码尚未修复。
- 修复 P0 后脚本通过，或只剩有白名单解释的行为注入项。
- 白名单不允许裸写页面名，必须包含“handler 来源模块”。

**验证**

- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run check:wxml-handlers`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run quality:check`

**残余风险**

- WXML 模板、behavior、运行时 spread 会产生静态分析误报；第一版目标是防止明显断链，不承担完整类型系统责任。

### P2-1 消费者空态：降低 B 端入驻入口权重

**用户问题**

外卖和预订消费者页面在空态中突出展示“商户入驻”“运营商入驻”。普通消费者进入页面时预期是找店、订座或理解附近暂无服务，而不是被引导到 B 端入驻流程。

**涉及文件**

- 修改：`weapp/miniprogram/pages/takeout/index.wxml`
- 可能修改：`weapp/miniprogram/pages/takeout/index.wxss`
- 修改：`weapp/miniprogram/pages/reservation/index.wxml`
- 可能修改：`weapp/miniprogram/pages/reservation/index.wxss`
- 核对：对应页面 `.ts` 中的 `onMerchantRegister`、`onOperatorRegister`

**实施边界**

- 消费者空态主行动应回到用户任务：更换位置、刷新、查看附近、返回首页或稍后再试。
- B 端入口保留为低权重入口，例如“我是商家/合作入驻”，放在空态次级位置或用户中心，不抢主按钮。
- 若当前页面没有位置切换或刷新能力，则只降低入驻按钮视觉权重，不新增未支持的新主能力。

**不做**

- 不删除真实入驻能力。
- 不重做外卖首页或预订首页的信息架构。
- 不新增后端接口。

**验收标准**

- 消费者空态第一眼表达“暂无可用商家/包间”和可恢复动作，而不是 B 端招募。
- “商户入驻/运营商入驻”不再作为消费者空态的两个大号主按钮同时出现。
- 既有入驻 handler 若仍保留，入口可见但低权重且文案明确。

**验证**

- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`
- 手工检查外卖无商户和预订无商户/无包间空态。

**残余风险**

- 如果产品当前刻意把消费者空态当作招商入口，需要产品确认权重；技术侧建议按消费者任务优先。

### P2-2 预订表单：清理 placeholder-as-label 文案漂移

**用户问题**

预订创建和确认表单里，`label="姓名"` 搭配 `placeholder="请输入姓名"`、`label="手机号"` 搭配 `placeholder="请输入手机号"`。这违反小程序表单基线：标签已经说明字段意义，placeholder 应该提供格式、约束或示例，而不是重复标签。

**涉及文件**

- 修改：`weapp/miniprogram/pages/reservation/create/index.wxml`
- 修改：`weapp/miniprogram/pages/reservation/confirm/index.wxml`

**实施边界**

- 联系人 placeholder 改为示例或约束，例如“例：张三”。
- 手机号 placeholder 改为格式提示，例如“11 位手机号”。
- 日期和时间 cell 的 `请选择日期/请选择时间` 属于选择状态提示，可保留；本任务只处理普通输入框的重复标签。

**不做**

- 不改变表单字段、校验规则、提交接口。
- 不新增 required 语义，除非现有后端合同和 TDesign 版本都明确支持并渲染。

**验收标准**

- 目标文件中不再出现 `placeholder="请输入姓名"` 和 `placeholder="请输入手机号"`。
- 输入框 label 与 placeholder 各司其职：label 表达字段，placeholder 表达示例、格式或约束。

**验证**

- 在仓库根目录运行：`rg -n "placeholder=\"请输入姓名\"|placeholder=\"请输入手机号\"" weapp/miniprogram/pages/reservation`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`

**残余风险**

- 全小程序范围可能还有类似文案，本任务只覆盖本次审查确认的预订路径；全量文案治理可另开任务。

## 推荐实施顺序

1. 执行 P0-1 和 P0-2，先消除用户点击无效和流程卡死。
2. 执行 P1-3，把本轮断链类型纳入脚本审查，避免回归。
3. 执行 P1-1 和 P1-2，清理死入口、演示坐标和不存在资源。
4. 执行 P2-1 和 P2-2，统一消费者任务预期与表单文案。
5. 最后从 `weapp/` 跑 `PATH="$HOME/.local/bin:$PATH" npm run quality:check`，并手工走 P0 涉及的结账与购物车路径。

## 总体验收口径

- 所有用户可见按钮和控件都有真实 handler、真实状态和真实服务或路由支撑。
- 结算、购物车、优惠、地图等状态不由前端 mock 或演示兜底制造“看起来可用”的假象。
- 消费者页面优先服务消费者任务，B 端入口不抢普通用户主路径。
- 表单文案符合移动应用长期形成的预期：控件含义靠 label，placeholder 只补充格式、约束或示例。
- 自动验证至少包括 `npm run compile`；跨多个小程序文件的整改完成后必须跑 `npm run quality:check`，若不能运行需在交付说明中写明原因和残余风险。

## 已完成的审查阶段

1. [complete] 映射 app 结构、角色、路由和主要用户旅程。
2. [complete] 检查页面实现中的用户可见状态和死/半实现交互。
3. [complete] 追踪关键动作和高风险流程背后的 service/API wiring。
4. [complete] 按严重程度记录审查发现、证据和残余风险。
5. [complete] 基于审查发现制定边界清晰的整改计划。

## 执行状态

- 当前状态：计划已制定，尚未实施业务代码。
- 下一步：等待用户确认后按推荐顺序执行整改。

## 已遇到的问题

| 问题 | 尝试 | 处理 |
| --- | --- | --- |
| `node: command not found` while running local audit scripts | Used default shell PATH from repo root | Re-run Node-based checks with `PATH="$HOME/.local/bin:$PATH"` per Mini Program toolchain guidance. |
| Handler audit script syntax error | Regex script had one extra closing brace in the heredoc | Fix script and re-run instead of using the failed result. |
