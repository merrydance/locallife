# Rider Phase 0 基线台账

日期：2026-03-28

## 1. 用途

这份台账用于把骑手侧从“页面看起来不少，但接口真值、页面职责、入口结构已经漂移”的状态，收束成一份可以持续执行的基线清单。

使用原则：

1. 只以真实代码为准，不以后端文档、Swagger 注释或前端类型声明自证为准。
2. 每个注册页面至少核对一次页面代码、前端 service、后端路由、handler 和关键 logic。
3. 每个页面都要标记入口状态、合同状态、关键阻断和能力缺口。
4. 注册但无入口的孤岛页也必须入账，避免继续承载历史假能力。

## 2. 审查方法

本轮审查仅依据以下真实实现：

1. 小程序注册页与页面代码：pages/rider 下 8 个注册页面。
2. 小程序 API 层：api/rider.ts、api/delivery.ts、api/rider-basic-management.ts、api/rider-exception-handling.ts、api/appeals-customer-service.ts、api/rider-delivery.ts。
3. 后端路由与 handler：locallife/api/server.go、delivery.go、rider.go、appeal.go、claim_recovery.go。
4. 关键访问控制逻辑：locallife/logic/delivery_access.go。
5. 编辑器静态诊断：已对 dashboard、task-detail、credit、delivery/manage 做诊断检查，未发现 TypeScript 级报错，但这不代表运行时合同正确。

## 3. 基线快照

1. rider 子包注册页面：8 个。
2. 当前明确主入口：1 个，为 dashboard/index。
3. 当前明确次入口：3 个，为 task-detail/index、deposit/index、exception/index。
4. 当前工具页：1 个，为 tasks/index。
5. 当前注册但无明显入口页面：3 个，为 claims/index、credit/index、delivery/manage/manage。
6. 当前整体评分：46 / 100。
7. 当前已确认的共性重点：主链详情权限错误、前端 service 自造合同、异常/申诉/索赔域混用、孤岛页并存、运行时状态不完备。

## 4. 全局已确认结论

1. task-detail/index 调用的 /v1/delivery/order/:order_id 在后端真实逻辑中只允许订单 owner 访问，不允许骑手查看，导致骑手任务详情主链天然不闭环。
2. dashboard/index 和 tasks/index 这两条主链使用的是较新的真实接口集合，但 details、history 分页和错误态没有完整接上，形成“列表能看、深链即断”的状态。
3. credit/index 不是简单字段名不同，而是整页建立在错误的积分合同之上。前端期待 current_score、score_level、history，后端真实返回 premium_score、can_accept_premium_order、logs。
4. exception/index、claims/index 和 rider-exception-handling.ts 将“异常上报”“索赔”“申诉”“追偿支付”混成了一页或一组接口，页面职责和后端语义已经错位。
5. delivery/manage/manage 代表一套更旧的骑手实现，内部使用过期状态枚举、缺失 /v1 前缀的接口路径、未绑定输入事件和不存在的字段，当前应视为高风险孤岛页。
6. deposit/index 的充值和账单基础链路基本能跑通，但提现能力、失败回查、页内错误壳仍缺失，离“骑手资金中心”还有差距。
7. dashboard/index 存在死代码导航到 /pages/chat/index，但仓库里没有该页面，说明骑手主入口仍残留未清理的历史分支。
8. 后端 ValidateDeliveryViewer 使用 delivery.rider_id 与 user_id 直接比对，而不是先映射 rider profile，这会让骑手查看配送定位类接口的能力进一步不可信；虽然当前骑手页未主用这条接口，但它暴露出配送访问控制仍有后端真值问题。

## 5. 页面台账

状态说明：

1. 入口状态：主入口 / 次入口 / 工具页 / 无明显入口。
2. 合同核查：未开始 / 部分完成 / 已确认失配 / 已完成。
3. 优先级：P0 / P1 / P2。

| 页面 | 入口状态 | 领域 | 优先级 | 合同核查 | 评分 | 关键问题 | 功能缺失 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| dashboard/index | 主入口 | 骑手工作台 | P0 | 部分完成 | 63 | refresh 失败直接清空列表，没有页内 error/retry；抢单大厅把 real_distance 原始值直接拼成 km；任务卡跳详情后落到无权限接口；保留了死代码 onChat | 缺历史入口、信用分入口、异常与申诉入口收口 |
| task-detail/index | 次入口 | 任务详情 | P0 | 已确认失配 | 28 | 调用 /v1/delivery/order/:order_id，但后端真实逻辑只允许订单 owner 查看；依赖不存在的 deadline_desc；页面 JSON 未注册 t-empty；没有真正 retry 壳 | 缺骑手专属详情数据源、失败回退、列表回流 |
| tasks/index | 工具页 | 配送历史 | P0 | 已确认失配 | 49 | 使用 page_id/page_size 请求 /v1/delivery/history，而后端真实参数是 page/limit，翻页会重复拿第一页；没有错误态，只有日志输出；当前无明确入口 | 缺真实分页、筛选、错误壳、回到任务详情的可靠深链 |
| deposit/index | 次入口 | 押金与账单 | P1 | 部分完成 | 72 | 充值与账单基本对齐真实接口，但只有 toast 失败反馈；未接 /v1/rider/withdraw；缺页内错误和重试面 | 缺提现入口、处理中状态、账单筛选和刷新策略 |
| exception/index | 次入口 | 异常上报 | P0 | 已确认失配 | 45 | 上报接口本身可达，但历史 Tab 实际读的是 /v1/rider/appeals，不是异常记录；图片上传只停留在 UI；缺 error 态 | 缺真实异常历史、延迟申报、图片上报链路、页内 retry |
| credit/index | 无明显入口 | 高值单资格积分 | P0 | 已确认失配 | 22 | 前端期待 current_score、score_level、can_take_high_value_orders、history；后端真实返回 premium_score、can_accept_premium_order、logs；整页展示模型错误 | 缺真实积分展示、日志映射、入口决策 |
| claims/index | 无明显入口 | 索赔/申诉 | P0 | 已确认失配 | 16 | 页面文案是“异常上报”，实际加载 rider claims；提交时把 taskId 当 claim_id 创建 rider appeal；状态比较使用大写 PENDING，与后端小写状态不一致；当前无明确入口 | 缺真正的申诉页定位、claim 选择、支付追偿闭环、页面命名收口 |
| delivery/manage/manage | 无明显入口 | 旧配送管理 | P0 | 已确认失配 | 12 | getRecommendedTasks 未传后端必需的经纬度；异常/延迟接口缺 /v1 前缀；组件状态枚举还在 accepted/picking_up/picked_up； modal 输入未绑定；引用 today_deliveries/today_earnings 等不存在字段 | 应判定为迁移或删除，不应继续作为真实页面 |

## 6. 页面到真实接口映射速览

| 页面 | 当前主要接口 | 审计结论 |
| --- | --- | --- |
| dashboard/index | /v1/rider/me, /v1/rider/status, /v1/delivery/recommend, /v1/delivery/active, /v1/delivery/grab/:order_id, /v1/delivery/:id/*, /v1/rider/location | 主入口基本接到真实接口，但详情深链和错误壳未闭环 |
| task-detail/index | /v1/delivery/order/:order_id | 接错权限语义，必须重做数据源 |
| tasks/index | /v1/delivery/history | URL 正确，分页 query contract 错误 |
| deposit/index | /v1/rider/deposit, /v1/rider/deposits | 基础合同可用，但资金域只做了充值和账单 |
| exception/index | /v1/rider/orders/:id/exception, /v1/delivery/active, /v1/rider/appeals | “异常历史”与“申诉历史”混用，职责错位 |
| credit/index | /v1/rider/score, /v1/rider/score/history | service 类型声明和页面消费字段均与后端真实 JSON 不一致 |
| claims/index | /v1/rider/claims, /v1/rider/claims/:id/recovery, /v1/rider/claims/:id/recovery/pay, /v1/rider/appeals | 页面职责混乱，claim 与 appeal 未分层 |
| delivery/manage/manage | /v1/delivery/*, /rider/orders/* | 旧实现混入错误路径、旧状态机和未完成交互 |

## 7. 已确认的主链路阻断

1. 任务详情权限阻断：task-detail/index 无法用真实骑手身份成功加载详情。
2. 历史翻页阻断：tasks/index 翻页合同错误，无法稳定浏览历史任务。
3. 积分页合同阻断：credit/index 不能按真实 JSON 渲染。
4. 异常域语义阻断：exception/index 和 claims/index 将 exception、claim、appeal 混在一起，用户无法理解也无法形成可验证链路。
5. 旧配送管理阻断：delivery/manage/manage 不是轻微过时，而是整体不可作为真实入口继续维护。

## 8. 后端已提供但骑手页未完整落地的能力

1. /v1/rider/withdraw：押金提现能力，当前前端未暴露。
2. /v1/rider/orders/:id/delay：骑手延迟申报能力，当前注册页面没有真实入口闭环。
3. /v1/rider/claims/:id/recovery/pay：追偿支付返回支付单信息，但 claims/index 只 await 接口，没有发起支付流程。
4. /v1/rider/claims/:id/decision：骑手索赔判定依据接口，当前前端未消费。
5. /v1/delivery/:delivery_id/track 与 /v1/delivery/:delivery_id/rider-location：定位类接口存在真实后端能力，但骑手侧没有形成可靠展示链。

## 9. 下一批执行顺序

1. dashboard/index、task-detail/index、tasks/index：先修主链路真值和详情闭环。
2. credit/index、exception/index、claims/index：拆分积分、异常、索赔、申诉四类职责。
3. deposit/index：补提现、失败回查和页内状态。
4. delivery/manage/manage：决定迁移还是删除，停止旧实现继续污染骑手域。
5. 完成统一回归，按真实接口重新评分并封板。