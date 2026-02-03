# Phase1 规则命中查询响应结构（草案）

## GET /v1/platform/rules/hits

```json
{
  "hits": [
    {
      "id": 1,
      "rule_id": 10,
      "rule_version_id": 12,
      "domain": "order",
      "decision": "deny",
      "reason": "外卖服务已被限制",
      "inputs": {},
      "outputs": {},
      "actor_id": 10001,
      "actor_role": "customer",
      "region_id": 1101,
      "merchant_id": 20001,
      "created_at": "2026-02-03T12:00:00Z"
    }
  ],
  "count": 1
}
```
