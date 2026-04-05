# CARD-11 商户侧能力缺口补齐批次

状态：进行中（核心切片代码持续推进，待能力反查与人工回归）

优先级：P1

所属阶段：Phase 3

## 问题目标

按现有矩阵文档补齐商户侧缺口能力，把商户端提升到完整经营工作台水平。

执行补充：finance、claims、appeals、risk 等高风险链路在实施前必须先核对后端真实路由、handler 响应结构和最新测试，不能只依赖前端现状或矩阵文档。

## 影响范围

- [weapp/docs/historical/pre-2026-04-05/merchant/MERCHANT_BACKEND_ALIGNMENT_MATRIX_2026-03-26.md](weapp/docs/historical/pre-2026-04-05/merchant/MERCHANT_BACKEND_ALIGNMENT_MATRIX_2026-03-26.md)
- [weapp/docs/historical/pre-2026-04-05/merchant/MERCHANT_PAGE_GAP_AND_IMPLEMENTATION_CHECKLIST_2026-03-26.md](weapp/docs/historical/pre-2026-04-05/merchant/MERCHANT_PAGE_GAP_AND_IMPLEMENTATION_CHECKLIST_2026-03-26.md)
- `weapp/miniprogram/pages/merchant/**`

## 任务内容

- [ ] 补齐 KDS、投诉、员工、统计、设置域缺口页。
- [x] 重构 config 为真正的设置导航页。
- [ ] 收口 finance、claims、appeals、reservations 的状态与动作闭环。

## 完成定义

- [ ] 商户侧主要后端能力都有明确页面落点。
- [ ] 高风险业务动作全部可用且状态可信。

## 验证要求

- [ ] 对照矩阵做一轮能力反查。
- [ ] 执行 merchant 相关最小校验与人工回归。

## 完成记录

- [x] 首个切片完成：新增 `merchant/kitchen` 后厨看板页，并接入工作台与配置中心入口
- [x] 已接通后厨订单列表、开始制作、标记出餐三条主动作
- [x] 已补齐页面级 loading、error、empty、retry 与动作中状态
- [x] 已增强 `merchant/kitchen` 的冷启动与重连恢复：进入页面即注册 websocket 监听，前后台切换后会重新绑定通知，收到订单通知时静默刷新后厨列表，避免实时页退化成只能手刷
- [x] 已增强 `merchant/kitchen` 的动作回流可信度：开始制作和标记出餐成功后会先回写当前后厨卡片，再做静默同步；若后续刷新失败，页面会保留当前后厨分组并提供页内重新同步入口
- [x] 第二个切片完成：新增 `merchant/stats` 经营统计页，并接入工作台与配置中心入口
- [x] 已接通经营概览、订单统计、热销菜、预订池概览四组真实数据
- [x] 第三个切片完成：新增 `merchant/settings/profile` 店铺资料页，并接通商户资料读取、校验、保存与配置中心入口
- [x] 已补齐店铺资料编辑态、保存中、保存成功失败反馈，以及到门店图片/经营分类/会员设置的导航
- [x] 第四个切片完成：新增 `merchant/settings/membership` 会员设置页，并接通会员叠加规则、抵扣上限、适用场景读取与保存
- [x] 已补齐会员设置 loading、error、dirty、retry、保存中状态
- [x] 已补齐 `merchant/settings/business-hours`，修复设置域入口漂移并接通营业时段读取与保存
- [x] 已新增 `merchant/staff/index`，接通员工列表、邀请码、角色分配与移除动作
- [x] 已新增 `merchant/complaints/index` 与 `merchant/complaints/detail`，接通投诉列表、详情、回复与完结动作
- [x] 已重构 `merchant/reservations`，修复日期筛选错位，补齐代客创建、编辑、取消、确认、未到店、完成动作闭环
- [x] 已增强 `merchant/dashboard`，补齐预订、投诉、员工、配置中心直达入口与待处理提醒
- [x] 已增强 `merchant/dashboard` 的状态可信度：首屏关键数据失败时改为页内 error/retry，不再把真实加载失败伪装成“0 数据”；订单流切页或刷新失败时也会提供分区级重试
- [x] 已增强 `merchant/dashboard` 的刷新可信度：经营概览、预订待办和投诉待办局部同步失败时会保留上次可信结果，并提供页内重新同步提示，避免待办数量被伪装成 0
- [x] 已新增 `merchant/settings/display-config`，接通打印、语音播报与 KDS 显示配置读取与保存
- [x] 已增强 `merchant/settings/display-config` 的配置真实性：打印/语音总开关闭合时会同步约束细分场景开关，避免继续保存前端可见但后端语义自相矛盾的配置组合
- [x] 已增强 `merchant/settings/display-config` 的编辑可信度：页面存在未保存改动时不再因回访或下拉刷新被服务端真值覆盖；无脏态时的静默刷新失败也会保留当前配置并提供页内重新同步入口
- [x] 已补齐 `merchant/appeals/detail`，并将 `merchant/appeals` 与 `merchant/claims/detail` 收口为列表、详情、异议、回款可互跳闭环
- [x] 已重构 `merchant/claims` 列表与 `merchant/finance` 页面状态壳，补齐责任/追偿摘要、收付通与余额错误态、签约链接复制与统一绑卡表单，并新增财务概览、订单收入明细、服务费明细、营销支出、财务日报、结算记录、结算流水等真实数据区块，同时修正提现记录字段映射
- [x] 已新增 `merchant/settings/applyment`，将收付通进件、签约链接复制与银行结算资料重提从资金页拆为独立设置页
- [x] 已增强 `merchant/settings/applyment` 的状态可信度：以后端 `status` 为准控制是否允许重提资料，补齐签约后显式刷新状态入口，并修正无进件记录时仍误判为“已存在进件”的问题
- [x] 已新增 `merchant/settings/application`，将主体申请草稿、必要证照 OCR 上传、提交审核与驳回重置纳入设置域，并接回配置中心与资料设置入口
- [x] 已增强 `merchant/claims/detail`，补齐用户、本店、骑手三方行为回溯摘要、顾客风险提示，并将责任判定、追偿、异议、行为摘要改为并行加载与独立重试；追偿动作已按后端新契约切换为拉起微信支付
- [x] 已按后端真实账户业务态收口 `merchant/finance` 余额与提现记录接口，使用 `account_status` / `status_desc` 驱动未激活态展示
- [x] 已将“以后端真实契约为准”写入 95+ 总计划和任务卡索引，并在 CARD-11 中明确 finance、claims、appeals、risk 需先查后端代码再实施
- [x] 已按后端真实 `appealDetailResponse` 收口 `merchant/appeals/detail`，补齐订单金额、批准金额、顾客手机号、索赔说明、发起方与赔付生效时间展示
- [x] 已增强 `merchant/appeals/detail` 的详情可信度：页面回访和下拉刷新改为静默同步，失败时保留当前详情并提供页内重新同步入口，不再只剩 toast
- [x] 已按后端真实列表字段收口 `merchant/claims` 与 `merchant/appeals` 列表，补齐顾客信息、订单金额、索赔说明，并修正追偿动作文案为支付语义
- [x] 已按后端真实提现契约收口 `merchant/finance` 提现动作，补齐创建后即时反馈、请求单号展示与单笔提现状态同步能力
- [x] 已增强 `merchant/finance` 提现动作失败反馈：提现申请失败与单笔状态同步失败会优先透传后端 `userMessage` / 错误原因，不再统一压成笼统 toast
- [x] 已增强 `merchant/claims/detail` 的弱网恢复：责任判定与追偿信息失败时会明确展示页内错误并支持独立重试，不再把加载失败伪装成“待判定”或“无追偿单”
- [x] 已增强 `merchant/claims/detail` 的动作回流可信度：异议提交与追偿支付成功后会先回写当前详情真值，再做静默同步；若后续回读失败，页面会保留当前详情并提供重新同步提示
- [x] 已增强 `merchant/complaints/detail` 的动作回流可信度：回复/完结成功后若静默回读失败，页面会保留上次同步结果并提供重新同步；若后端返回“微信成功、本地待同步”的 200 降级结果，也会显式提示商户状态稍后同步
- [x] 已增强 `merchant/complaints` 列表的刷新可信度：页面回访和下拉刷新改为静默同步，失败时不再清空已加载投诉列表，而是保留上次结果并提供页内重新同步入口
- [x] 已增强 `merchant/staff` 的动作回流可信度：改角色与移除员工成功后会先本地回写真值，再做静默列表同步；若回读失败，页面会保留当前员工列表并提供重新同步入口
- [x] 已增强 `merchant/stats` 的首屏状态可信度：经营概览、热销菜、预订池改为独立分区成功/失败语义，不再把局部请求失败伪装成 0 数据或普通空态；页面回访和下拉刷新失败时也会保留上次结果并提供分区重试
- [x] 已增强 `merchant/settings/business-hours` 的编辑可信度：页面存在未保存改动时不再因回访或下拉刷新被服务端真值静默覆盖；无脏态时的静默刷新失败也会保留当前配置并提供页内重新同步入口
- [x] 已增强 `merchant/settings/membership` 的编辑可信度：页面存在未保存改动时不再因回访或下拉刷新被服务端真值静默覆盖；无脏态时的静默刷新失败也会保留当前配置并提供页内重新同步入口
- [x] 已增强 `merchant/settings/profile` 的编辑可信度：页面存在未保存改动时不再因回访或下拉刷新被服务端真值静默覆盖；无脏态时的静默刷新失败也会保留当前配置并提供页内重新同步入口
- [x] 已增强 `merchant/reservations` 的首屏状态可信度：预订列表与备菜明细改为独立分区成功/失败语义，避免任一接口失败时把另一块结果也伪装成空态；下拉刷新失败时也会保留上次结果并提供分区重试
- [x] 已增强 `merchant/orders/list` 的列表可信度：补齐首屏 error/retry，修复“加载更多”按钮误触发重置第一页的问题，并在接单/拒单/出餐/核销成功后先本地回写真值，再静默同步最新列表
- [x] 已增强 `merchant/orders/detail` 的详情可信度：补齐首屏 error/retry，动作成功后先回写当前详情真值，再做静默同步；若后续回读失败，页面会保留当前详情并提供重新同步入口
- [x] 已增强 `merchant/printers` 的设备配置可信度：补齐首屏 error/retry，列表静默刷新失败时保留上次同步结果，并在新增、编辑、删除打印机成功后先本地回写真值再静默同步，同时为测试/删除动作补齐防重入反馈
- [x] 已完成 `merchant/finance`、`merchant/claims`、`merchant/appeals` 末轮闭环复核：finance 动作失败反馈已细化，claims 列表/详情动作链已补齐独立恢复能力，appeals 列表/详情复核后未发现新增代码缺口
- [x] 已将 `merchant/config` 重构为真正的设置导航页：按店铺资料、主体与结算、设备与展示、组织与协作分组，移除订单/商品/售后等运营型入口，避免配置中心继续承担杂项工作台职责
- [x] 已为 `merchant/appeals` 列表补齐后端真实 `compensated` 状态的汇总卡片与筛选入口，避免已赔付结果只淹没在全部列表中
- [x] 已将 `merchant/appeals` 列表切换为后端真实状态筛选与分页链路，补齐服务端 `status` 查询、真实 `has_more` 返回，以及小程序按 tab 拉取、加载更多与尾态提示，避免继续用单页本地筛选造成统计和结果失真
- [x] 已将 `merchant/claims` 列表切换为后端真实 bucket 筛选与分页链路，补齐服务端 `pending_action / appealed / closed` 过滤、列表 `recovery_status` 与真实 `has_more` 返回，并按 recovery / appeal 状态机重算列表分类与行动提示，避免继续依赖单页本地筛选和 `claim.status` 猜测结案态
- [x] 已将 `merchant/finance` 中重复的进件、签约和绑卡表单收口到 `merchant/settings/applyment`，资金页仅保留开通状态摘要、余额/提现/结算洞察与统一跳转入口，消除重复入口与重复状态源
- [x] 已按后端真实订单 DTO 收口 `merchant/orders/list` 与 `merchant/orders/detail`，补齐拒单退款动作、配送履约细态 `courier_accepted / picked / rider_delivered / user_delivered`、真实配送/联系人字段与动态时间线，移除对不存在字段的前端假设
- [x] 已补齐 `merchant/dishes/edit` 的规格组、规格选项与加价编辑链路，并将 `merchant/tables`、`merchant/printers`、`merchant/delivery-promotions` 长表单弹层切换为 `scroll-view` 滚动容器；同时修复桌台二维码相对路径与接口失败时的预览兜底，降低小屏滚动和二维码预览失效风险
- [x] 已新增商户侧全域审查执行计划 [weapp/docs/historical/pre-2026-04-05/merchant/MERCHANT_95_FULL_AUDIT_EXECUTION_PLAN_2026-03-28.md](weapp/docs/historical/pre-2026-04-05/merchant/MERCHANT_95_FULL_AUDIT_EXECUTION_PLAN_2026-03-28.md)，明确 Phase 0 到 Phase 6 的阶段边界、95 分评分标准与 P0 风险
- [x] 已新增 Phase 0 基线台账 [weapp/docs/historical/pre-2026-04-05/merchant/MERCHANT_PHASE0_BASELINE_LEDGER_2026-03-28.md](weapp/docs/historical/pre-2026-04-05/merchant/MERCHANT_PHASE0_BASELINE_LEDGER_2026-03-28.md)，将 32 个 merchant 页面与首批共享组件纳入合同核查与媒体风险跟踪
- [x] 已将 `merchant/dishes` 列表页从页面内 `API_BASE` 图片拼接切回共享 `getPublicImageUrl` 语义，避免菜品图继续沿用旧公共媒体合同
- [x] 已将 `merchant/profile-images` 的 logo、门头照、环境照回显切到共享 `getPublicImageUrl`，修复 local `/dev/uploads/...` 在页面中被吞掉的显示问题
- [x] 已补齐 `merchant/profile-images` 的上传保存闭环：门头照和环境照上传/删除改为以后端 `PATCH /v1/merchants/me/shop-images` 返回值回写真值，删除失败时不再先留下本地脏态，并补上保存中禁用与更准确的成功/失败反馈
- [x] 已补齐后端 `tables` 图片接口的公开图片 URL 返回，`GET/POST/PUT /v1/tables/{id}/images*` 现同步返回 `media_asset_id + image_url`，消除桌台图片 DTO 与小程序回显链路的核心漂移
- [x] 已修正 `merchant/tables` 的标签取消关联语义：前端移除当前桌台标签时不再误删整页 `availableTags` 可选池，只取消当前桌台的选中/关联状态，并将操作文案改为“取消选择/取消关联”以匹配后端 `DELETE /v1/tables/{id}/tags/{tag_id}` 的真实语义
- [x] 已补齐 `merchant/tables` 的图片弱网反馈：创建桌台成功但后续图片补传失败时，页面不再误报整单保存失败，而是提示“桌台已创建，部分图片添加失败，请进入编辑页重试”；编辑态图片添加、删除和设主图也补上了 loading、防重入和更准确的失败提示
- [x] 已将 `merchant/dishes/edit` 的详情图回显与预览入口切到共享 `getPublicImageUrl`，避免商品编辑页继续保留另一套公共图 helper 语义
- [x] 已为 `merchant/dishes/edit` 收口真实图片资产合同：商户菜品详情现回传 `image_asset_id`，编辑态无需重新上传也能保留既有图片资产，页面删除动作改为与后端真实能力一致的“仅支持替换，不支持删除已发布图片”语义
- [x] 已为 `merchant/dishes/edit` 补齐创建/更新成功后的 persisted 图片真值同步；当基础信息已保存但规格或标签后续保存失败时，页面会保留后端已接受的 `image_asset_id` 和预览图，不再回退到旧图片状态
- [x] 已增强 `merchant/dishes` 列表的搜索与错误态可信度：首屏失败不再伪装成“暂无菜品”，而是提供页内 error/retry；静默刷新失败时保留上次列表；搜索改为按当前分类顺序拉取完整分页后再过滤，避免只筛当前页导致结果和空态失真
- [x] 已增强 `merchant/reservations` 的当日列表完整性：预订列表改为按日期顺序拉取全部分页，不再只展示前 50 条；代客创建后的提示文案也改为以后端返回状态为准，避免前端先行承诺固定 confirmed 状态
- [x] 已按后端真实套餐图片契约收口 `merchant/combos` 与 `merchant/combos/edit`：商户套餐详情补齐 `dish_image_urls` enrich，列表页按真实图片字段展示封面并改为用 `total` 推导尾态，编辑页补齐候选菜与已选菜摘要图片预览
- [x] 已为 `merchant/combos/edit` 收口套餐保存闭环：后端 `updateComboSet` 改为事务更新，避免基础字段已落库但成员菜品/标签失败时出现半成功；编辑页在创建/更新成功后会立即同步 `comboId`、价格、状态与已选菜本地真值，避免停留页继续显示旧快照
- [x] 已为 `merchant/combos/edit` 补齐首屏弱网恢复壳：数据加载失败时页面不再只剩 toast，而是提供页内 error/retry；菜品选择区也区分“暂无菜品”和“仅上架筛选后暂无结果”两类空态
- [x] 已补齐 `merchant/merchant-categories` 的真实后端落点：页面改为以 `GET/PUT /v1/merchants/me/tags` 和 `GET /v1/tags?type=merchant` 为唯一真值来源，保存成功后以后端返回标签回写，并补齐页内 error/retry、待保存提示和保存中禁用
- [x] 已为 `merchant/merchant-categories` 补齐下拉刷新恢复能力，类目列表弱网或长时间停留后可通过页内重试或下拉刷新重新拉取最新服务端真值；当前代码与契约审查评分提升到 95/100
- [x] 已完成 `merchant/tables` 与 `merchant/profile-images` 的二轮代码与契约审查评分：前者补齐列表首屏 error/retry 后提升到 96/100，后者补齐首屏 error/retry 与空态恢复后提升到 95/100；两页当前主要剩余项为真机弱网回归
- [x] 已收口 `merchant/settings/application` 的真实提交前置条件：后端基础信息保存接口现接通统一信用代码、经营范围、法人姓名、法人身份证号人工修正，页面补齐食品经营许可证上传、经营位置选择与区域匹配提示，并在提交前按真实后端要求拦截 OCR 未完成、未定位、未匹配区域等失败路径
- [x] 已补齐 `merchant/settings/application` 的 approved 重改语义：后端 `merchant_application` 现允许 approved 状态通过自动重置回 draft 后继续编辑，页面同步放开已通过申请的编辑、证照替换、定位和重新提交入口，并明确“修改后会回到草稿”的提示文案
- [x] 已补齐 `merchant/settings/application` 的弱网 OCR 超时反馈：证照上传后若 OCR 长时间未完成，页面会保留已上传证照和处理中状态，并提供“刷新识别结果”入口，不再只剩 toast 导致用户误以为上传未生效
- [ ] KDS 人工回归完成
- [ ] 经营统计人工回归完成
- [ ] 店铺资料人工回归完成
- [ ] 会员设置人工回归完成
- [ ] 业务时段、员工、投诉、预订、显示配置人工回归完成
- [x] finance、claims、appeals 动作闭环完成
- [ ] 反查完成
- [ ] 回归完成