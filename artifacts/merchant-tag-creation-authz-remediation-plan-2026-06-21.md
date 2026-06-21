# 商家端标签新增与关联越权修复计划

日期：2026-06-21

## 背景

前端上报显示 `pages/merchant/dishes/edit/index` 对 `POST /v1/tags` 有 12 次 403，错误是 `this endpoint requires admin role`，同时出现 `Submit dish failed`。商家菜品编辑页当前允许点击“新增标签”，但实现调用的是平台管理员接口 `POST /v1/tags`。

正确业务目标：商家应当可以添加自己的菜品、桌台等可选标签。实现借鉴菜品分类：底层 `tags(name,type)` 全局唯一，商家把某个全局字典项加入自己的可用集合。不得把商家角色加入管理员 `/v1/tags` 写权限，也不得删除商家新增标签能力。

## 目标模型

- `tags` 继续作为全局唯一字典，唯一键保持 `(name, type)`。
- 新增 `merchant_selectable_tags(merchant_id, tag_id, sort_order, created_by_user_id, created_at)`。
- 商家新增标签时，后端先创建或复用全局 active tag，再幂等关联到当前 merchant。
- 商家读取标签默认只返回当前 merchant 已关联且 active/type 匹配的标签。
- 平台管理员仍维护全局标签；商家不能修改全局 status、type 或归属。

## 修复要求

- 新增 `GET /v1/merchant/tags?type=...` 和 `POST /v1/merchant/tags`。
- 服务端从认证上下文获取 `merchant_id`，不接受客户端传入。
- 同名同类型 active tag 已存在：复用并 upsert 商户关联。
- 同名同类型 inactive tag 已存在：返回 409，不静默复活。
- 菜品、桌台、套餐最终保存时，事务内复核 tag active、type 和当前 merchant 可选集合。
- 小程序商家页改用 `MerchantTagService`，不再调用平台 `TagService.createTag`。

## 测试门禁

- 商家 owner/manager 创建新 dish/table tag 成功，并产生全局 tag + merchant 关联。
- 同一商家重复创建同名 tag 返回同一 tag，不重复关联。
- 不同商家创建同名 tag 复用同一全局 tag，但各自有关联。
- inactive tag 同名创建返回 409。
- 商家保存菜品时提交未关联、inactive、错误 type 的 tag_id 均失败。
- 平台 admin `/v1/tags` 写接口保持可用；商家直接调用 `/v1/tags` 仍 403。
- 菜品/桌台编辑页仍保留新增标签入口，但不再调用平台 `TagService.createTag`。
