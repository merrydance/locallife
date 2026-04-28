# Flutter Task Annotation Template

> 作用：把 `PRODUCTION_ROBUSTNESS_BASELINE.md` 里的标注字段收成一个可直接填写、可复用、可放进任务卡或实现请求的模板。

## 使用时机

以下情况建议先写这份标注，再进入实现或 review：

- 任务不是纯样式或纯文案
- 任务涉及认证、推送、去重、接单、前台服务、弱网恢复、权限恢复、打印协同
- 任务存在冷启动、后台恢复、重复点击、重复消息、乱序消息、Token 失效等失败模式
- 你需要把需求交给 AI、交给其他开发、或沉淀为可复盘的任务卡

## 模板

```text
Title: <一句话说明任务>
Target: <feature/path>
User task: <商户要可靠完成什么>
Risk: <G0/G1/G2/G3 + 为什么>
Source of truth: <swagger/backend handler/typed contract>
State owner: <provider/service>
Recovery boundary: <什么落盘、什么重建、什么是临时态>
Failure modes: <duplicate/retry/cold start/re-entry/weak network/permission loss/...>
Invariant: <最终必须成立的业务事实>
Non-goals / prohibited guesses: <明确不做什么，哪些语义不能猜>
Validation: <commands + scenarios>
Residual risk: <未验证项>
```

## 字段解释

### Title

- 用一句话说明任务，不要把实现细节写进标题。
- 好标题应该描述业务目标或错误行为，而不是“加一个 provider”。

### Target

- 写清 feature、页面、provider、service 或持久化入口。
- 如果涉及多层，至少写出主入口和状态 owner。

### User task

- 从商户视角写，不从工程结构写。
- 例如“商户在弱网恢复后仍能看见待接单并继续接单”，而不是“恢复 WebSocket 状态”。

### Risk

- `G0`: 纯展示小改动
- `G1`: 一般页面状态和非关键功能
- `G2`: 认证、连接恢复、设备注册、订单正确性、打印协同
- `G3`: 漏单、重复接单、去重、接单提醒、前台服务、推送主链路

### Source of truth

- 写清后端接口、消息协议、字段定义来源。
- 如果后端语义不清晰，这里要直接标注“缺失/待确认”，不要默认猜。

### State owner

- 只能有清晰 owner。
- 一般应是 provider 或 service，而不是 Widget。

### Recovery boundary

- 回答三个问题：
- 什么状态要持久化。
- 什么状态要从服务端重建。
- 什么状态允许丢失。

### Failure modes

- 至少勾出这个任务真正相关的失败模式。
- 不要把所有模式都写上，也不要一个都不写。

### Invariant

- 写成一句可验证的事实。
- 例如“同一条 `message_id` 无论通过几个通道到达，都只能触发一次接单提醒”。

### Non-goals / prohibited guesses

- 明确哪些范围不改。
- 明确哪些服务端语义、厂商行为或未来能力不能靠客户端自行假设。

### Validation

- 至少写出要跑的命令和要走的场景。
- `G2` / `G3` 不要只写 `flutter analyze`。

### Residual risk

- 任何未做的真机验证、厂商验证、弱网验证、冷启动验证，都写在这里。

## 快速示例

```text
Title: 修复新订单重复提醒导致的重复播报
Target: merchant_app/lib/core/service/message_dedup.dart
User task: 商户收到同一订单的多通道消息时，只被提醒一次
Risk: G3，因为会导致重复提醒并影响接单链路稳定性
Source of truth: 后端推送消息中的 message_id 定义
State owner: message dedup service
Recovery boundary: 24 小时内的 message_id 持久化；重启后从本地去重记录恢复；音频播放队列不持久化
Failure modes: duplicate, re-entry, cold start
Invariant: 同一 message_id 只能被接受一次并触发一次提醒
Non-goals / prohibited guesses: 不修改后端推送协议；不猜测 message_id 缺失时的替代规则
Validation: flutter analyze; flutter test; 手动验证 WebSocket + push + 轮询重复到达；冷启动后重复消息
Residual risk: 未做华为和 OPPO 真机验证
```

## 使用规则

- 任务卡、实现请求、bugfix 请求、review 请求可以直接嵌入这份模板。
- 如果任务已经能用一句话说清，但涉及 `G2` 或 `G3`，仍建议至少补全 `Risk`、`State owner`、`Recovery boundary`、`Failure modes`、`Validation` 五项。
- 如果标注写不出来，通常不是文档问题，而是任务边界还没想清楚。