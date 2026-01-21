# 商户侧 Web 替代小程序 + 集团/品牌架构对齐调研（2026-01-21）

## 1. 背景与目标
- 现状：商户侧主要在小程序内实现“桌面级餐厅管理 SaaS”。
- 变化：后端不只是新增“集团/品牌-门店”架构，还包含转台等一批功能重写与新增。
- 目标：开发 Web 前端**全量替代**小程序内的商户侧页面，并在迁移过程中与最新后端**逐项对齐**。
- 登录设想：Web 通过**小程序扫码**触发登录，后端根据扫码用户 `openid` 是否已注册、是否有关联商户及角色来决定授权。

## 2. 后端能力范围与对齐方式
> 重要说明：后端变化不限于集团/品牌。需要以**实时接口文档**为准逐项核对字段与类型。
> 推荐对齐资料：
> - [locallife/docs/swagger.yaml](locallife/docs/swagger.yaml)
> - [locallife/docs/swagger.json](locallife/docs/swagger.json)
> - 相关实现代码（如 [locallife/api/group.go](locallife/api/group.go)、[locallife/api/dining_session.go](locallife/api/dining_session.go)、[locallife/api/server.go](locallife/api/server.go)）

### 2.1 集团入驻申请
- 创建/获取草稿、更新基础信息、提交、审核等流程完整。 
- 对应路由集中在 `/v1/groups/applications/*`。

### 2.2 集团/品牌管理
- 集团搜索、详情、更新、查看旗下门店。
- 品牌创建、品牌详情。
- 集团策略与菜单模板管理。

### 2.3 门店申请加入集团（含品牌归属）
- 门店可发起加入集团申请，集团侧可审核通过/驳回/撤回。
- 审核通过时可指定 `brand_id`。
- 路由集中在 `/v1/groups/:id/join-requests/*`。

## 3. 认证与扫码能力（澄清版）
> 主要代码位于：
> - [locallife/api/wechat.go](locallife/api/wechat.go)
> - [locallife/api/token.go](locallife/api/token.go)
> - [locallife/api/server.go](locallife/api/server.go)

### 3.1 小程序登录
- 小程序启动时调用 `/v1/auth/wechat-login`：
  - 未注册：静默注册并落库。
  - 已注册：直接登录并返回 token。
- Web 登录**不需要**使用 `code` 换取 `openid`。
- Web 只需要在扫码登录流程中，由后端判断扫码用户的 `openid` 是否已注册、是否有关联商户与角色；符合条件后直接签发 token。

### 3.2 桌台二维码（堂食扫码点餐）
- 桌台二维码是**小程序码**，用于用户打开微信直接扫码进入菜单页面。
- `/v1/tables/:id/qrcode` 由后端调用微信小程序码接口生成二维码并保存。
- 对应逻辑在 [locallife/api/scan.go](locallife/api/scan.go) 与路由定义在 [locallife/api/server.go](locallife/api/server.go)。

## 4. 现有商户侧（小程序）相关流程
> 主要代码位于：
> - [weapp/miniprogram/pages/merchant/staff/index.ts](weapp/miniprogram/pages/merchant/staff/index.ts)
> - [weapp/miniprogram/pages/user/bind-merchant/index.ts](weapp/miniprogram/pages/user/bind-merchant/index.ts)
> - [weapp/miniprogram/api/personal.ts](weapp/miniprogram/api/personal.ts)
> - [locallife/api/staff.go](locallife/api/staff.go)
> - [locallife/api/server.go](locallife/api/server.go)

### 4.1 员工邀请码绑定
- 后端：`POST /v1/merchant/staff/invite-code` 生成邀请码（保存到 `bind_code`，24h 过期）。
- 后端：`POST /v1/bind-merchant` 用邀请码完成绑定。
- 小程序端：邀请码二维码在商户侧页面生成，二维码内容为小程序页面路径 `/pages/user/bind-merchant/index?code=...`。
- **新要求**：Web 端也需要生成员工邀请码二维码，且员工需用小程序扫码完成绑定入职。

## 5. 二维码“类型与用途”的现状结论（更新）
- **桌台二维码**：后端生成微信小程序码，固定用于堂食扫码点餐。
- **员工邀请码二维码**：用于小程序内扫码绑定入职，后端仅生成邀请码；二维码由商户侧页面生成（小程序/未来 Web）。
- **Web 登录二维码**：用于小程序内扫码登录 Web。扫码后后端判断 `openid` 是否已注册、是否有关联商户/角色，满足条件即给 Web 签发 token。
- **Boss/认领码**：`boss_bind_code` 已废弃，建议删除字段并迁移清理。

## 6. Web 替代小程序的关键对齐项（更新）
### 6.1 认证与权限
- 设计 Web 登录扫码流（Web 生成登录码 + 小程序扫码确认 + 后端校验 `openid` 与商户/角色 + Web 获取 token）。
- Web 登录不需要 `code -> openid`。
- 登录后仍需获取用户角色与商户列表（如 `/v1/users/me`、`/v1/merchants/my`）。

### 6.2 集团/品牌与门店加入
- Web 需支持：搜索集团、提交门店加入申请、集团侧审核。
- 同时需支持品牌归属的选择/显示。

### 6.3 员工管理
- Web 需复用：邀请码生成、员工绑定、员工列表/角色管理。

### 6.4 桌台二维码管理
- Web 需支持：桌台二维码生成、预览与下载。

## 7. 二维码类型与使用原则（更新）
- 桌台二维码：小程序码，长期可用，面向 C 端扫码点餐。
- 员工邀请码二维码：小程序内扫码绑定入职，二维码由商户侧页面生成。
- Web 登录二维码：小程序内扫码完成 Web 授权登录。
- 三类二维码用途不同，不应混用；但可统一 payload 结构规范与后端校验模式。

## 8. 已确认结论（更新）
1. Web 登录走小程序扫码；Web 不需要 `code -> openid`。
2. `boss_bind_code` 已废弃，应删除字段并执行迁移清理。
3. 员工邀请码二维码由前端（小程序/未来 Web）生成，扫码在小程序内完成绑定。
4. Web 替换范围为小程序商户侧**全量替代**。

---

后续可补充（待你确认后执行）：
- Web 扫码登录接口设计草案（含状态机与安全策略）
- Web 端页面结构与权限矩阵（全量对齐）
- 依据 swagger 的字段/类型逐项对齐清单（含差异点）
