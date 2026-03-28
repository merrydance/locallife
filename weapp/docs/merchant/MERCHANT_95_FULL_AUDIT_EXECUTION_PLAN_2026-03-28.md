# Merchant 95+ 全域审查与收口执行计划

日期：2026-03-28

## 1. 目标

本计划用于把商户侧小程序从“主链路已补齐一部分，但页面合同、媒体语义、状态闭环仍有漂移”的状态，提升到稳定 95 分以上。

本轮目标有且只有两条：

1. 完全对齐后端真实实现。
2. 把商户侧整体交互、弱网韧性、信息架构和经营工具完整性提升到 95 分以上。

本计划不是零散修 bug，而是按页面域、共性基建、后端真实合同做一轮全量审查和分阶段收口。

## 2. 当前关键判断

基于 2026-03-28 的复核，商户侧已经不是“有没有页面”的问题，而是进入“页面是否真的按后端真实合同运行”的阶段。

当前已经确认的高风险事实：

1. 公共媒体真实合同已切到 OSS + CDN，公共图片应优先返回可直接访问的绝对 URL；local 开发环境才允许走 dev route。
2. 小程序侧当前仍存在多套媒体 URL 语义：有的页面走共享工具，有的页面仍在手动拼接路径。
3. 个别页面存在 DTO 漂移问题，不只是体验问题。例如桌台图片列表接口后端返回 `media_asset_id`，但前端仍有页面按 `image_url` 形态假设渲染。
4. merchant 子包已经形成完整经营工作台，但各页面质量层次不一，必须统一用同一评分和验收标准治理。

结论：下一阶段不能继续以单页修补为主，必须先锁定共性合同，再按域推进。

## 3. 审查边界

### 3.1 页面范围

当前 merchant 子包注册页面共 32 个，以 app.json 为准：

1. dashboard
2. kitchen
3. stats
4. staff
5. settings/profile
6. settings/business-hours
7. settings/membership
8. settings/application
9. settings/applyment
10. settings/display-config
11. config
12. complaints
13. complaints/detail
14. claims
15. claims/detail
16. appeals
17. appeals/detail
18. finance
19. dishes
20. dishes/categories
21. dishes/edit
22. combos
23. combos/edit
24. inventory
25. reservations
26. orders/list
27. orders/detail
28. tables
29. printers
30. profile-images
31. delivery-promotions
32. merchant-categories

### 3.2 共享组件范围

当前需要纳入 merchant 复核的共享组件共 26 个，优先审查被 merchant 高风险页面直接依赖的组件：

1. auth-image
2. custom-navbar
3. list-skeleton
4. info-row
5. merchant-feed-card
6. ll-stats-card
7. document-uploader
8. virtual-list

其余组件在各阶段按引用关系补充检查。

### 3.3 后端核对范围

每个页面必须至少反查以下一项或多项：

1. handler
2. request / response DTO
3. 实际注册路由
4. 最新测试
5. 媒体 URL resolver 或上传链路

## 4. 95 分评分标准

每个页面统一按 100 分评分，低于 95 分不得视为完成。

### 4.1 评分维度

1. 后端合同一致性：25 分
2. 动作闭环完整性：20 分
3. 媒体与资源正确性：15 分
4. loading / success / empty / error / retry 完整性：10 分
5. 弱网与失败反馈：10 分
6. 性能与 setData 控制：10 分
7. 信息架构、文案与危险操作反馈：10 分

### 4.2 95 分达标标准

同时满足以下条件才算达标：

1. 页面字段、动作、状态机与后端真实合同一致。
2. 所有关键按钮都调用真实 API，而不是只改本地状态。
3. 媒体 URL、二维码 URL、私有签名图和公共 CDN 图使用统一规则。
4. 页面具备 loading、success、empty、error、retry 五态或四态中的完整分支。
5. 弱网或失败后页面仍可恢复，不留下半完成状态。
6. 没有明显的死链、重复入口、重复状态源、职责漂移。

## 5. 共性审查清单

每个页面都必须走以下检查表：

1. 路由是否已注册。
2. 入口是否唯一，是否与配置中心或工作台信息架构一致。
3. 页面调用的 API wrapper 是否与真实 handler 和 DTO 对齐。
4. 动作按钮是否真的调用后端。
5. 成功、失败、空态、重试是否显式可见。
6. 分页、筛选、has_more 是否以后端返回为准。
7. 媒体字段是否走共享媒体工具，而不是页面自定义拼接。
8. 私有图是否通过 media id 或签名接口获取。
9. 是否有大块重复 setData、长列表首屏扇出、冷启动丢订阅等性能问题。
10. 用户可感知文案是否准确，不使用开发术语或伪状态文案。

## 6. 分阶段执行计划

### Phase 0：基线锁定

目标：先把商户侧所有共性真相定成唯一标准，避免后续反复返工。

二级任务：

1. 锁定公共媒体、私有媒体、二维码、图片变体、dev route 的唯一合同。
2. 锁定 merchant 子包页面清单与共享组件清单。
3. 为每个页面建立统一审查模板与评分卡。
4. 建立风险台账，区分 P0、P1、P2 页面和组件。

产出：

1. 商户全域页面清单
2. 共享组件清单
3. 共性合同矩阵
4. 页面评分模板

退出标准：

1. 后续页面改造不再依赖前端旧实现猜测后端语义。

### Phase 1：共性基建收口

目标：先解决所有页面都会踩到的公共问题。

二级任务：

1. 统一公共媒体 URL 解析规则。
2. 统一私有媒体 URL 获取规则。
3. 统一二维码预览与图片预览规则。
4. 清理页面内自定义 API_BASE 拼接。
5. 清理死路由、重复入口、客户端伪状态。
6. 统一 loading、empty、error、retry 页壳表达。

重点对象：

1. tables
2. dishes
3. combos
4. profile-images
5. dashboard
6. stats
7. auth-image

退出标准：

1. merchant 侧不再存在页面级自定义公共媒体语义。
2. 高风险页面的媒体字段与二维码字段合同统一。

### Phase 2：交易与履约主链

目标：把商户真正日常高频操作链打透。

二级任务：

1. 复核 orders/list 与 orders/detail 的真实状态机、动作和时间线。
2. 复核 reservations 的筛选、代客建预订、编辑、取消、确认、未到店、完成。
3. 复核 kitchen 的冷启动、实时态、履约动作和弱网恢复。
4. 复核 tables 的桌台状态、二维码、图片、预订关联、标签与图片上传。
5. 复核 printers 与 display-config 的设备配置和分发开关。

页面范围：

1. dashboard
2. orders/list
3. orders/detail
4. reservations
5. kitchen
6. tables
7. printers
8. settings/display-config

退出标准：

1. 从接单到履约完成的商户主链动作全部可信。

### Phase 3：售后与资金高风险链

目标：把最容易产生合同漂移的资金与售后链统一到真实后端语义。

二级任务：

1. 复核 claims 与 claims/detail 的责任、追偿、异议、支付动作。
2. 复核 appeals 与 appeals/detail 的状态、分页、结果文案和详情字段。
3. 复核 complaints 与 complaints/detail 的投诉状态、回复、完结和同步文案。
4. 复核 finance 的余额、提现、明细、进件跳转和错误态。
5. 复核 settings/applyment 的进件、签约、银行结算资料重提。

页面范围：

1. claims
2. claims/detail
3. appeals
4. appeals/detail
5. complaints
6. complaints/detail
7. finance
8. settings/applyment

退出标准：

1. 售后和资金页不存在伪动作和双重状态语义。

### Phase 4：商品与经营工具链

目标：把商户经营能力从“页面存在”升级到“经营闭环完整”。

二级任务：

1. 复核 dishes、dishes/edit、dishes/categories 的字段、分页、分类、规格、上下架和图片。
2. 复核 combos、combos/edit 的套餐成员、图片、状态和价格规则。
3. 复核 inventory 的当日库存、分页、保存与回流策略。
4. 复核 delivery-promotions 的规则字段、时间条件和保存反馈。
5. 复核 merchant-categories 与 profile-images 的真实后端落点。

页面范围：

1. dishes
2. dishes/edit
3. dishes/categories
4. combos
5. combos/edit
6. inventory
7. delivery-promotions
8. merchant-categories
9. profile-images

退出标准：

1. 商户经营工具链从内容维护到展示到状态切换全部闭环。

### Phase 5：设置与组织治理

目标：把设置域和组织协作域收成真正统一的经营后台。

二级任务：

1. 复核 profile、business-hours、membership 的合同与状态闭环。
2. 复核 application 的证照上传、OCR、草稿、提交、驳回重置。
3. 复核 staff 的邀请码、角色、移除、权限边界。
4. 复核 config 页面信息架构，消除重复入口和职责重叠。

页面范围：

1. config
2. settings/profile
3. settings/business-hours
4. settings/membership
5. settings/application
6. staff

退出标准：

1. merchant 设置域职责清晰，无重复流程和重复状态源。

### Phase 6：95 分体验抛光与真机验收

目标：在合同完全对齐后做最后一轮体验拔高。

二级任务：

1. 统一底部弹层滚动策略和键盘顶起行为。
2. 统一 skeleton、empty、error、retry 视觉表达。
3. 复核弱网下的提交、刷新、分页和重试表现。
4. 复核首屏请求扇出与长列表滚动性能。
5. 做真机人工回归清单。

验收维度：

1. 后端对齐
2. 动作闭环
3. 媒体正确性
4. 弱网韧性
5. 首屏性能
6. 信息架构
7. 视觉一致性

退出标准：

1. 商户侧整体稳定达到 95 分以上。

## 7. 建议执行顺序

推荐顺序：

1. Phase 0
2. Phase 1
3. Phase 2
4. Phase 3
5. Phase 4
6. Phase 5
7. Phase 6

原因：

1. 若不先锁定媒体、二维码和 DTO 共性合同，后续每个页面都会返工。
2. 交易履约和售后资金链风险最高，必须优先于纯视觉或低频配置页。
3. 体验抛光必须晚于合同收口，否则只是把错误实现做得更顺滑。

## 8. 当前已确认的第一批 P0 风险

1. merchant tables 页曾使用页面内自定义公共媒体拼接规则，与后端 CDN/dev route 真实合同不一致。
2. tables 图片列表的前后端 DTO 已出现潜在漂移，需优先复核真实返回结构。
3. merchant 页面内仍有部分页面使用自定义 image_url 标准化逻辑，需统一回收至共享工具。

## 9. 下一步执行建议

最合理的下一步不是继续散点修页面，而是立即开始 Phase 0 与 Phase 1：

1. 形成 merchant 审查台账。
2. 扫清所有媒体 URL 与二维码语义漂移。
3. 再进入交易履约与售后资金主链复核。
