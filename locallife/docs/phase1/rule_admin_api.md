# Phase1 规则配置 API（草案）

## 目标
- 提供规则的读写/发布/回滚入口。

## 接口草案
- POST /v1/platform/rules
- GET /v1/platform/rules
- GET /v1/platform/rules/{id}
- POST /v1/platform/rules/{id}/versions
- POST /v1/platform/rules/{id}/publish
- POST /v1/platform/rules/{id}/disable
- POST /v1/platform/rules/{id}/rollback

## 说明
- 全部接口需 admin 权限
- 发布动作需写 rule_audits
- 发布/回滚会将规则状态置为 active；禁用会将规则状态置为 disabled
- 禁用会清空 current_version_id
- version 必须为 published 才会被引擎读取
- 回滚未指定 version_id 时，自动选择最近的已发布版本（排除当前版本）
- 示例： [locallife/docs/phase1/rule_admin_api_examples.md](locallife/docs/phase1/rule_admin_api_examples.md)

## 响应格式
统一响应封装：

```json
{
	"code": 0,
	"message": "ok",
	"data": {}
}
```

主要 data 结构：
- 创建规则 / 发布 / 禁用 / 回滚：返回规则对象
- 创建规则版本：返回规则版本对象
- 列表查询：{ "rules": [规则对象], "count": 数量 }
- 详情查询：{ "rule": 规则对象, "versions": [规则版本对象] }

## 错误码与含义
- 400：参数非法、版本状态非法、无可回滚版本
- 401/403：无权限
- 404：规则或规则版本不存在
- 500：内部错误
