# 食安群体事件闭环设计

## 1. 目标

这份文档只解决一个问题：

当前仓库里，顾客侧食安上报、群体事件熔断、运营商处置、监管报送、整改报告、恢复经营，没有形成同一条业务闭环。

本文输出：

1. 当前实现断点
2. 推荐的权威数据关系
3. 目标状态机
4. API 改造方案
5. 分阶段落地顺序

风险等级判断：`G3`

原因：顾客输入会直接驱动商户暂停营业，且后续涉及运营处置、监管报送、恢复经营，属于高影响状态流转链路。

## 2. 当前实现现状

### 2.1 已有前链路

顾客食安上报入口已经存在：

- [locallife/api/server.go](locallife/api/server.go)
- [locallife/api/risk_management.go](locallife/api/risk_management.go)
- [locallife/algorithm/food_safety_handler.go](locallife/algorithm/food_safety_handler.go)

当前行为是：

1. 顾客调用 `POST /v1/food-safety/report`
2. 系统按“同商户 1 小时内 3 起”评估是否熔断
3. 满足阈值后直接调用 `SuspendMerchant`
4. 创建一条 `food_safety_incidents` 记录

对应的数据模型在：

- [locallife/db/query/trust_score.sql](locallife/db/query/trust_score.sql)
- [locallife/db/migration/000018_add_trust_score_system.up.sql](locallife/db/migration/000018_add_trust_score_system.up.sql)

### 2.2 已有后链路

运营商侧也有一套“安全报告”系统：

- [locallife/api/operator_features.go](locallife/api/operator_features.go)
- [locallife/db/query/operator_safety.sql](locallife/db/query/operator_safety.sql)
- [locallife/db/migration/000106_add_operator_finance_safety.up.sql](locallife/db/migration/000106_add_operator_finance_safety.up.sql)

当前行为是：

1. 运营商可以创建 `safety_reports`
2. 运营商可以填写 `resolution_notes`
3. 运营商可以直接恢复商户上线

### 2.3 当前不是闭环，而是两段分裂实现

当前系统里的断点是：

1. 顾客上报落在 `food_safety_incidents`
2. 运营商报告落在 `safety_reports`
3. 两张表之间没有外键、没有来源引用、没有状态回写、没有自动建案
4. 商户恢复经营基于 `safety_reports`，不是基于顾客侧食安事件的调查结论

所以当前真实状态不是“已实现闭环”，而是“前半段能触发停业，后半段另有一套手工处置系统”。

## 3. 当前缺失闭环清单

## 3.1 顾客身份核验不够可信

当前食安接口直接接收 `reporter_id` 请求字段，没有以登录态用户作为权威身份来源，相关代码在：

- [locallife/api/risk_management.go](locallife/api/risk_management.go)

这会带来两个问题：

1. 顾客身份边界不可信
2. “无感身份核验”在进入食安主链前就已经弱化

最低要求应改为：

1. 上报用户 ID 只能来自 token
2. 必须校验 `order_id` 属于当前用户
3. 必须校验订单的 `merchant_id` 与请求一致

## 3.2 “同一手机”没有直接接入食安链

当前食安恶意举报判断只会检查：

1. 是否全是首单新用户
2. 是否命中过去已确认的 `fraud_patterns`
3. 最近订单地址是否重复

问题在于：

1. 它没有直接查询当前参与举报用户的设备复用
2. 它依赖的是旧 `fraud_patterns` 历史结果，而不是当前实时判定

这不符合“系统自动核验是不是来自同一个手机”的业务目标。

## 3.3 群体事件口径缺少产品维度

当前阈值口径只有：

1. 同商户
2. 1 小时内
3. 3 起举报

没有：

1. 同菜品 / 同订单项集合
2. 去重用户
3. 举报簇唯一键

如果业务要求是“午饭都点了同个商户的外卖，都拉肚子了”，最低可接受实现也应该支持：

1. 同商户
2. 同时段
3. 同主投诉菜品或高度重叠订单项

## 3.4 顾客食安事件没有调查状态机

`food_safety_incidents` 模型本身预留了：

1. `status`
2. `investigation_report`
3. `resolution`
4. `resolved_at`

但是当前业务代码没有把这些字段用起来。sqlc 层虽然有：

- `GetFoodSafetyIncident`
- `GetActiveFoodSafetyIncidents`
- `UpdateFoodSafetyIncidentStatus`

但未看到进入 API / logic / worker 主路径。

这意味着顾客食安事件停留在“建一条记录 + 可能熔断”，没有进入正式调查流转。

## 3.5 运营商处置对象不是顾客食安事件

当前 `safety_reports` 更像“运营商自由创建的区域安全报表”，字段只有：

1. `region_id`
2. `title`
3. `description`
4. `level`
5. `merchant_ids`
6. `resolution_notes`

缺少食安闭环所需的关键字段：

1. 来源顾客事件 / 举报簇 ID
2. 监管报送状态
3. 监管报送编号
4. 整改报告正文/附件
5. 恢复经营审核结论
6. 恢复经营时间

## 3.6 没有监管报送能力

当前仓库里只在协议文案里提到“向监管部门报告”，没有看到：

1. 监管报送实体
2. 监管报送接口
3. 监管报送状态机
4. 监管回执号 / 报送编号

所以“本地运营商需要上报到食药监部门”目前不是系统能力。

## 3.7 测试缺失

未发现这条链路的专门测试：

1. 顾客上报 handler 测试
2. 食安判定算法测试
3. 运营商处置测试
4. 恢复经营测试
5. 顾客事件和运营商处置联动测试

## 4. 目标闭环设计

## 4.1 设计原则

目标不是再加第三套系统，而是明确一个权威主对象。

推荐原则：

1. 顾客侧原子事实保留在 `food_safety_incidents`
2. 运营商/监管处置使用单独的“案件对象”承接
3. 商户暂停和恢复只由“案件对象”的裁决结果驱动
4. `safety_reports` 不再作为顾客食安闭环的权威对象

## 4.2 推荐主对象

推荐新增 `food_safety_cases`，不要继续把 `safety_reports` 硬改成食安案件表。

原因：

1. `safety_reports` 当前语义是区域运营报表，不是顾客举报聚合案件
2. 把 generic report 改造成案件对象，字段会持续漂移
3. 食安闭环本身是高风险状态机，需要单独的权威聚合实体

### 推荐数据关系

```text
food_safety_incidents  (顾客原子上报)
        |
        | many-to-one
        v
food_safety_cases      (运营/监管调查案件，权威对象)
        |
        | one-to-many
        v
food_safety_case_events (审计事件流)

food_safety_case_regulator_reports (监管报送记录，可选一对多)
```

### `food_safety_incidents` 保留职责

只做原子事实记录：

1. 谁报的
2. 哪个订单
3. 哪个商户
4. 举报时间
5. 主投诉类型
6. 订单快照
7. 关联用户风险信号
8. 是否被计入群体事件
9. 所属案件 ID

### `food_safety_cases` 承担职责

作为权威案件对象，负责：

1. 是否触发停业
2. 当前调查状态
3. 运营商接案与处置
4. 监管报送
5. 商户整改材料
6. 恢复经营审核

## 4.3 推荐字段

### `food_safety_incidents` 新增字段

建议新增：

1. `reporter_device_fingerprint text`
2. `reporter_address_id bigint`
3. `risk_flags jsonb not null default '{}'`
4. `risk_decision text`
5. `counted_in_cluster boolean not null default false`
6. `cluster_key text`
7. `primary_dish_id bigint null`
8. `order_item_fingerprint text`
9. `case_id bigint null`

其中：

1. `primary_dish_id` 用于“同产品”聚类
2. `order_item_fingerprint` 用于套餐/多菜品场景归并
3. `risk_flags` 用于记录新用户、共享设备、共享地址、异常图谱等命中结果

### `food_safety_cases` 建议字段

建议包含：

1. `id`
2. `merchant_id`
3. `region_id`
4. `trigger_cluster_key`
5. `trigger_reason_code`
6. `status`
7. `suspend_started_at`
8. `suspend_until`
9. `operator_id`
10. `investigation_summary`
11. `regulator_reporting_required`
12. `regulator_reporting_status`
13. `regulator_report_ref`
14. `merchant_rectification_report`
15. `merchant_rectification_media_ids`
16. `recovery_review_notes`
17. `recovered_at`
18. `closed_at`
19. `created_at`
20. `updated_at`

## 5. 目标状态机

## 5.1 原子事件状态机

`food_safety_incidents.status` 推荐调整为：

1. `reported`
2. `screened`
3. `rejected_as_malicious`
4. `counted`
5. `escalated`
6. `closed`

含义：

1. `reported`: 顾客完成上报
2. `screened`: 已完成无感身份核验
3. `rejected_as_malicious`: 命中恶意攻击信号，不计入群体事件
4. `counted`: 被计入群体事件簇
5. `escalated`: 已归并到案件
6. `closed`: 案件关闭后，原子事件归档完成

## 5.2 案件状态机

`food_safety_cases.status` 推荐使用：

1. `open`
2. `merchant_suspended`
3. `investigating`
4. `reported_to_regulator`
5. `awaiting_merchant_rectification`
6. `reviewing_rectification`
7. `ready_for_recovery`
8. `recovered`
9. `closed`

其中关键约束是：

1. 只有 `merchant_suspended` 或之后状态，商户才保持停业
2. 只有 `ready_for_recovery` 之后，才能执行恢复经营
3. 恢复经营必须绑定调查结论和整改报告，不能只凭运营商手工输入商户 ID 直接恢复

## 5.3 监管报送状态机

`regulator_reporting_status` 推荐使用：

1. `not_required`
2. `pending`
3. `submitted`
4. `acknowledged`
5. `failed`

如果初期没有真实监管接口，也应该先把这条状态链记录下来，至少让系统知道：

1. 这起案件需不需要报送
2. 有没有报送
3. 报送编号是什么

## 6. API 改造方案

## 6.1 顾客上报接口

保留：

- `POST /v1/food-safety/report`

但请求语义必须改：

1. 删除 `reporter_id`
2. `user_id` 从 token 中解析
3. 强制校验订单归属
4. 强制校验订单商户归属
5. 读取订单项，生成 `primary_dish_id` 和 `order_item_fingerprint`
6. 同步执行无感身份核验

返回值建议增加：

1. `incident_id`
2. `counted_in_group_event`
3. `merchant_suspended`
4. `case_id`（如果已升级为案件）

## 6.2 运营商案件接口

不要继续以 `safety_reports` 作为食安闭环 API。

建议新增：

1. `GET /v1/operator/food-safety/cases`
2. `GET /v1/operator/food-safety/cases/:id`
3. `POST /v1/operator/food-safety/cases/:id/assign`
4. `POST /v1/operator/food-safety/cases/:id/investigation`
5. `POST /v1/operator/food-safety/cases/:id/regulator-report`
6. `POST /v1/operator/food-safety/cases/:id/merchant-rectification`
7. `POST /v1/operator/food-safety/cases/:id/recover`
8. `POST /v1/operator/food-safety/cases/:id/close`

## 6.3 商户恢复经营接口

恢复经营不应再接受“随手传一个 `recover_merchant_ids`”的模式。

应改为：

1. 对某个案件执行恢复
2. 服务端从案件里确定 `merchant_id`
3. 必须存在调查结论
4. 必须存在整改报告
5. 如果要求监管报送，必须已经达到 `submitted` 或 `acknowledged`

## 7. 推荐判定逻辑

## 7.1 无感身份核验规则

顾客不会感知，但系统需要在食安主链实时计算以下信号：

1. 是否新用户
2. 是否首单/低订单用户
3. 当前事件簇内是否共享设备指纹
4. 当前事件簇内是否共享地址
5. 是否命中既有高风险用户限制状态
6. 是否存在异常聚合图谱

建议输出统一结构：

```json
{
  "is_new_user": true,
  "shared_device": true,
  "shared_address": false,
  "high_risk_user": false,
  "decision": "exclude_from_group_count",
  "reason_codes": ["shared_device", "new_user_cluster"]
}
```

## 7.2 群体事件聚类规则

推荐使用以下聚类键：

`merchant_id + time_bucket_1h + primary_dish_id_or_order_item_fingerprint`

同一个聚类键下：

1. 只统计通过无感身份核验的上报
2. 同一用户只能计 1 次
3. 命中共享设备/共享地址的上报默认不计入触发数

达到阈值后：

1. 创建或打开一个 `food_safety_case`
2. 自动暂停商户
3. 发送预警
4. 把关联 incident 全部标记为 `escalated`

## 8. 迁移建议

## 8.1 第一阶段：修正信任边界

先做最小闭环修正：

1. `ReportFoodSafety` 删除 `reporter_id`
2. 从 token 读取当前用户
3. 校验 `order_id` 所属用户和商户一致性
4. 把设备、地址、订单项指纹写入 `food_safety_incidents`

这一阶段先不改运营商处置，只先把前链路变成可信输入。

## 8.2 第二阶段：补齐案件对象

新增 `food_safety_cases` 和 `food_safety_case_events`：

1. 让顾客原子事件能升级为案件
2. 让暂停营业与恢复经营绑定同一案件

## 8.3 第三阶段：替换运营商处理入口

新增 `operator/food-safety/cases/*` 接口，停止把 `safety_reports` 当成食安案件。

`safety_reports` 可以保留给区域运营的一般安全报表，不再承担顾客食安闭环。

## 8.4 第四阶段：监管报送接入

即使初期没有真实政府接口，也应先落库：

1. 是否需要报送
2. 谁报送的
3. 什么时候报送的
4. 报送编号
5. 结果状态

等真实接口准备好，再接异步 worker。

## 8.5 第五阶段：测试闭环

至少补以下测试：

1. 顾客上报身份边界测试
2. 共享设备/共享地址恶意聚类测试
3. 同商户同产品群体事件触发测试
4. 案件建案与商户停业联动测试
5. 监管报送前禁止恢复测试
6. 整改报告后恢复经营测试

## 9. 建议取舍

## 9.1 保留

保留：

1. `food_safety_incidents` 作为顾客原子上报事实表
2. `ReportFoodSafety` 作为统一用户上报入口
3. `FoodSafetyHandler` 的“达到阈值则升级”职责，但要改成案件化输出

## 9.2 弃用或降级

对顾客食安闭环来说，应停止把以下对象当成权威主对象：

1. `safety_reports` 作为顾客食安案件表的角色
2. `recover_merchant_ids` 这种与案件脱钩的恢复模式
3. 基于旧 `fraud_patterns` 的单一恶意判定来源

## 10. 最终判断

当前仓库里存在的是：

1. 顾客食安上报
2. 基于简单规则的群体事件熔断
3. 一套独立的运营商安全报告工具

当前仓库里不存在的是：

1. 顾客事件到运营处置的同对象闭环
2. 监管报送闭环
3. 整改报告驱动的恢复经营闭环
4. 强可信的“无感身份核验”实时接入

所以当前实现不应被判断为“完整食安群体事件闭环”，而应被判断为“顾客熔断前链已上线，案件处置后链未打通”。