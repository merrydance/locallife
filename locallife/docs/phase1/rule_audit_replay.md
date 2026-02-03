# Phase1 规则命中审计与回放（草案）

## 目标
- 记录规则命中细节，支持回放与追溯。

## 数据结构（建议）
- rule_hits
  - rule_id / rule_version_id
  - domain / decision / reason
  - inputs / outputs（JSON）
  - actor_id / actor_role / region_id / merchant_id
  - created_at

## 回放流程
- 选取 rule_hit 记录
- 使用当时输入重放规则评估
- 对比输出与当前规则版本

## 参考接口（草案）
- GET /v1/platform/rules/hits?rule_id=xxx
  - 响应结构： [locallife/docs/phase1/rule_hits_response.md](locallife/docs/phase1/rule_hits_response.md)

## 备注
- 先落库与采样记录，后续接入完整回放工具。
