# Phase1 规则管理 API 示例（草案）

## 创建规则
POST /v1/platform/rules

```json
{
  "name": "takeout_blocklist_deny",
  "category": "order",
  "status": "draft"
}
```

## 创建规则版本
POST /v1/platform/rules/{id}/versions

```json
{
  "version": 1,
  "status": "published",
  "priority": 10,
  "scope": {"order_type": "takeout"},
  "condition": {"behavior_blocklist": true},
  "action": {"type": "deny", "reason": "外卖服务已被限制：该账号存在异常索赔记录"},
  "gray_config": {"region_id": [1101]}
}
```

## 发布规则版本
POST /v1/platform/rules/{id}/publish

```json
{
  "version_id": 123
}
```

## 禁用规则
POST /v1/platform/rules/{id}/disable

```json
{
  "reason": "活动结束，先整体下线"
}
```

## 回滚到指定版本
POST /v1/platform/rules/{id}/rollback

```json
{
  "version_id": 122
}
```

## 自动回滚到上一发布版本
POST /v1/platform/rules/{id}/rollback

```json
{}
```
