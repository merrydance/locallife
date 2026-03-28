# Merchant Phase 0 基线台账

日期：2026-03-28

## 1. 用途

这份台账用于把商户侧从“聊天里的排查结论”变成“可持续执行的基线清单”。

使用原则：

1. 每个页面至少核对一次真实 handler、DTO、路由或测试。
2. 每个页面都要标记媒体合同状态，避免 OSS/CDN 迁移后继续沿用旧的 API_BASE 拼接逻辑。
3. 所有风险必须进台账后再排执行顺序。

## 2. 基线快照

1. merchant 子包页面入口：32 个。
2. 当前组件入口扫描结果：25 个 `components/*/index.ts`，其中 merchant 高风险直接依赖组件优先处理。
3. 当前已确认的共性重点：媒体 URL、二维码 URL、DTO 漂移、四态壳一致性。

## 3. 第一批已确认结论

1. `tables` 页面已收回共享公共图片语义，不再使用页面内自定义 API_BASE 拼接二维码和桌台图片 URL。
2. `tables` 的 table images 合同已补齐为 `media_asset_id + image_url` 双字段返回，桌台图片列表、主图设置和新增图片响应现在都能直接回显 CDN/公共 URL。
3. `dishes` 列表曾使用页面内 `API_BASE` 拼接图片 URL，本轮已切回共享 `getPublicImageUrl` 语义。
4. `profile-images` 已切到 `getPublicImageUrl`，避免 local 开发态 `/dev/uploads/...` 被 `getMediaDisplayUrl` 吞掉；页面合同风险从 P0 显示错误降为待继续复核上传与保存闭环。
5. `dishes/edit` 已补齐创建/更新成功后的本地 persisted 图片真值同步；即便后续规格或标签保存失败，页面也不会继续拿旧 `image_asset_id` 回滚成错误图片状态。
6. `merchant-categories` 已按真实 `GET/PUT /v1/merchants/me/tags` 与 `GET /v1/tags?type=merchant` 收口当前类目选择、保存回写与页内 error/retry，保存后统一以后端返回标签结果为准。

## 4. 页面台账

状态说明：

1. 合同核查：未开始 / 部分完成 / 已完成
2. 媒体状态：对齐 / 风险 / 待确认 / 不适用
3. 优先级：P0 / P1 / P2

| 页面 | 领域 | 优先级 | 合同核查 | 媒体状态 | 备注 |
| --- | --- | --- | --- | --- | --- |
| dashboard | 交易入口 | P1 | 部分完成 | 不适用 | 已补齐首屏 error/retry、订单流分区错误态，以及概览/待办提醒局部刷新失败时的页内提示与旧结果保留，待纳入统一评分 |
| kitchen | 履约 | P1 | 部分完成 | 不适用 | 已补齐 websocket 通知监听、前后台重绑刷新，以及开始制作/标记出餐后的本地真值同步与静默刷新失败降级，待统一评分 |
| stats | 经营分析 | P2 | 部分完成 | 不适用 | 已补齐经营概览、热销菜、预订池的分区错误态与静默同步降级，待统一评分 |
| staff | 组织治理 | P2 | 部分完成 | 不适用 | 已补齐改角色/移除后的本地真值同步与静默回读失败降级，待统一评分 |
| settings/profile | 设置域 | P1 | 部分完成 | 部分对齐 | 已补齐未保存编辑保护与静默刷新失败降级，待统一评分 |
| settings/business-hours | 设置域 | P2 | 部分完成 | 不适用 | 已补齐未保存编辑保护与静默刷新失败降级，待统一评分 |
| settings/membership | 设置域 | P2 | 部分完成 | 不适用 | 已补齐未保存编辑保护与静默刷新失败降级，待统一评分 |
| settings/application | 设置域 | P1 | 部分完成 | 风险已收敛 | 已补齐食品经营许可证、经营位置与提交前真实后端校验，修正 approved 申请可编辑/可重提的真实合同，并补齐 OCR 超时后的页内提示、处理中状态保留与刷新入口；待继续做人工回归评分 |
| settings/applyment | 资金设置 | P1 | 部分完成 | 不适用 | 已按后端状态收口重提门禁与刷新回流，待统一评分 |
| settings/display-config | 履约设备 | P1 | 部分完成 | 不适用 | 已收口全局开关与子开关一致性，并补齐未保存编辑保护与静默刷新失败降级，待统一评分 |
| config | 设置导航 | P1 | 部分完成 | 不适用 | 已重构为按能力域分组的设置导航页，移除运营型入口职责重叠，待统一评分 |
| complaints | 售后 | P1 | 部分完成 | 不适用 | 已补齐回访/下拉刷新的静默同步失败降级，保留旧列表并提供重新同步，待统一评分 |
| complaints/detail | 售后 | P1 | 部分完成 | 不适用 | 已补齐动作后静默回读失败降级与微信成功待同步提示，待统一评分 |
| claims | 售后资金 | P1 | 部分完成 | 不适用 | 已补齐责任判定与追偿信息独立错误/重试，待统一评分 |
| claims/detail | 售后资金 | P1 | 部分完成 | 不适用 | 已补齐异议提交与追偿支付后的本地真值同步、页内“状态稍后同步”提示，以及静默刷新失败保留当前详情并支持重新同步，待统一评分 |
| appeals | 售后资金 | P1 | 部分完成 | 不适用 | 已复核列表与详情闭环，未发现新增代码缺口，待统一评分 |
| appeals/detail | 售后资金 | P1 | 部分完成 | 不适用 | 已补齐批准结果语义展示，并补齐回访与下拉刷新的静默刷新降级；失败时保留当前详情并提供重新同步，待统一评分 |
| finance | 资金 | P1 | 部分完成 | 不适用 | 已补齐提现申请与单笔状态同步失败反馈透传，待统一评分 |
| dishes | 商品 | P0 | 部分完成 | 风险已收敛 | 已补齐首屏 error/retry 与静默刷新失败保留旧列表；搜索改为按当前分类拉取完整分页后再过滤，避免只筛当前页与伪空态；待统一评分 |
| dishes/categories | 商品 | P2 | 未开始 | 不适用 | 待统一复核分类增删改排序 |
| dishes/edit | 商品 | P1 | 部分完成 | 风险已收敛 | 已补回商户菜品详情 `image_asset_id`，修正编辑态图片回填，并补齐创建/更新成功后的 persisted 图片真值同步；待继续做人工回归评分 |
| combos | 商品 | P1 | 部分完成 | 风险已收敛 | 已按真实 `dish_image_urls` 收口套餐封面图与分页尾态，待继续复核筛选与动作评分 |
| combos/edit | 商品 | P1 | 部分完成 | 风险已收敛 | 已补齐候选菜图与摘要预览，将套餐更新改为后端事务保存，并补齐首屏 error/retry 与筛选空态区分；编辑页在创建/更新成功后会同步本地 persisted 套餐真值，待继续做人工回归评分 |
| inventory | 经营工具 | P2 | 未开始 | 不适用 | 重点看库存保存和回流 |
| reservations | 交易 | P1 | 部分完成 | 不适用 | 已补齐预订列表与备菜明细的分区错误态和静默刷新降级，并改为按日期拉取全部分页预订，移除仅展示前 50 条的截断；代客创建后续状态改为以后端返回为准，待统一评分 |
| orders/list | 交易 | P1 | 部分完成 | 不适用 | 已补齐首屏错误壳、加载更多修复与动作后本地真值同步，待统一评分 |
| orders/detail | 交易 | P1 | 部分完成 | 待确认 | 已补齐首屏错误壳、静默刷新失败降级与动作后本地真值同步，待统一评分 |
| tables | 桌台设备 | P0 | 部分完成 | 风险已收敛 | 已补齐 table images 返回 `image_url`，修正桌台标签取消关联时误删本地可选标签池的问题，并补齐创建后图片补传失败的半成功提示、图片动作防重入以及列表首屏 error/retry 页壳；当前代码与契约审查评分提升到 96/100，待补真机弱网回归 |
| printers | 桌台设备 | P1 | 部分完成 | 不适用 | 已补齐首屏错误壳、静默刷新失败降级，以及新增/编辑/删除后的本地真值同步与防重入反馈，待统一评分 |
| profile-images | 商户资料 | P0 | 部分完成 | 风险已收敛 | 已修复 local `/dev/uploads/...` 显示问题，并补齐上传/删除以后端响应回写真值、删除失败不再留下本地脏态，同时补齐首屏 error/retry 与空态恢复页壳；当前代码与契约审查评分提升到 95/100，待补真机弱网回归 |
| delivery-promotions | 经营工具 | P2 | 未开始 | 不适用 | 重点看规则字段和保存回流 |
| merchant-categories | 商户资料 | P1 | 部分完成 | 不适用 | 已按真实 merchants/me/tags 合同收口类目加载与保存，保存成功后以后端返回标签回写真值，并补齐页内 error/retry、待保存状态提示和下拉刷新；当前代码与契约审查评分提升到 95/100，待补人工回归评分 |

## 5. 组件台账

| 组件 | 优先级 | 备注 |
| --- | --- | --- |
| auth-image | P0 | 私有图与签名链路核心组件，需先复核 |
| custom-navbar | P2 | 商户页通用导航壳 |
| list-skeleton | P1 | 四态一致性关键组件 |
| info-row | P2 | 明细展示组件 |
| merchant-feed-card | P2 | dashboard 与列表卡片组件 |
| ll-stats-card | P2 | stats 卡片组件 |
| document-uploader | P1 | 设置域上传链路关键组件 |
| virtual-list | P1 | 若页面已接入需检查弱机表现；当前待确认入口 |
| category-tabs | P2 | 商品域筛选组件 |
| cart-bar | P3 | merchant 非核心 |
| review-card | P3 | merchant 非核心 |
| remark-input | P3 | merchant 非核心 |
| delivery-card | P3 | merchant 非核心 |
| room-card | P3 | merchant 非核心 |
| map-view | P3 | merchant 非核心 |
| restaurant-card | P3 | merchant 非核心 |
| delivery-map | P3 | merchant 非核心 |
| recharge-promo | P3 | merchant 非核心 |
| search-filter | P2 | 列表页筛选体验 |
| dish-skeleton | P2 | 商品页 loading 态 |
| delivery-task-card | P3 | merchant 非核心 |
| package-card | P2 | 套餐域列表卡片 |
| search-bar | P2 | 商品域与列表页搜索 |
| card-skeleton | P2 | 卡片型 loading |
| dish-card | P2 | 商品图与状态展示 |
| merchant-promos | P3 | merchant 非核心 |

## 6. 下一批执行顺序

1. `merchant-categories`：代码与契约审查评分已提升到 95/100，待补人工回归评分。
2. `combos/edit`：已收口事务保存、本地真值同步和首屏 error/retry，待补人工回归评分。
3. `settings/application`：已补齐 approved 重改/重提语义与 OCR 超时页内提示，待补人工回归评分。
4. `tables`：代码与契约审查评分已提升到 96/100，待补真机弱网回归。
5. `profile-images`：代码与契约审查评分已提升到 95/100，待补真机弱网回归。
