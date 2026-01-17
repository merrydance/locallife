# Deprecated & Removed Endpoints

> 生效日期：2026-01-17

本文件用于记录已弃用或已移除的旧接口，以及推荐替代路径。

## 已移除（不再对外提供）

### 商户入驻旧接口（已移除）
- `GET /v1/merchants/applications`：旧版商户申请列表
- `GET /v1/merchants/applications/{id}`：旧版商户申请详情

**替代接口**：
- `GET /v1/merchant/application`：获取申请草稿/状态
- `PUT /v1/merchant/application/*`：更新申请信息
- `POST /v1/merchant/application/submit`：提交申请

### 骑手入驻旧接口（已移除）
- `POST /v1/rider/apply`：旧版骑手申请入口

**替代接口**：
- `GET /v1/rider/application`：获取申请草稿/状态
- `PUT /v1/rider/application/*`：更新申请信息
- `POST /v1/rider/application/submit`：提交申请

## 说明
- `*/applyment/*` 系列接口用于微信二级商户开户流程（绑卡/进件状态查询），并非旧版入驻入口，暂不弃用。
