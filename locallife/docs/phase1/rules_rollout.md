# Phase1 规则灰度发布与生效策略

## 模式说明
- 旁路模式：规则命中仅记录，不影响业务结果。
- 强制模式：规则命中可拦截或执行动作。

## 灰度维度与优先级
灰度以 `rule_versions.gray_config` 控制，支持按多维度收敛流量：

```json
{
	"region_id": [1101, 1102],
	"merchant_id": [10001, 10002],
	"user_id": [20001],
	"percent": 10
}
```

匹配顺序建议：
1. `user_id`（精确最小粒度）
2. `merchant_id`
3. `region_id`
4. `percent`（兜底桶，便于快速扩大覆盖率）

> 当前实现已支持 `region_id`/`merchant_id`/`user_id`，`percent` 作为预留字段可在后续扩展。

## 推荐切换流程
1. 默认旁路（仅记录命中）
2. 小流量强制（按 region/merchant 灰度）
3. 扩大灰度范围（region 级）
4. 全量强制

## 操作步骤（草案）
1. 设置 `RULES_ENGINE_ENABLED=true`
2. 创建规则版本并将状态设为 `published`
3. 填写 `gray_config`（region/merchant/user）进行灰度
4. 观察 `rule_hits` 命中与业务指标
5. 逐步扩大灰度范围

## 观测指标（建议）
- 规则命中：`rule_hits` 数量、命中率、deny/alert/allow 分布
- 业务核心指标：下单成功率、退款率、申诉率、支付失败率
- 回滚信号：命中激增、业务错误码异常、关键链路失败

## 开关建议
- `RULES_ENGINE_ENABLED`：启用 DB 规则引擎
- 规则灰度配置：通过 `rule_versions.gray_config` 控制

## 回滚策略
优先级：灰度缩减 → 规则禁用 → 引擎开关关闭

1. 缩减 `gray_config` 覆盖范围
2. `POST /v1/platform/rules/{id}/disable`
3. 关闭 `RULES_ENGINE_ENABLED`

## 风险控制清单
- 同一规则的多个版本不得同时发布为强制生效
- 发布前确保规则版本 `status=published`
- 强制模式前至少观察 1-2 天旁路命中
- 灰度扩大需保留回滚版本（上一发布版本）

## 快速校验（上线前）
- 规则命中是否落库（rule_hits）
- 规则发布后是否生效（命中动作与预期一致）
- 灰度范围是否符合预期（region/merchant/user）

