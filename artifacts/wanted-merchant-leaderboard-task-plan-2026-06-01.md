# 想吃商户榜单活动任务文档 - 2026-06-01

## 目标

在小程序外卖首页新增一个 hero 活动入口：

> 你想吃谁家外卖？告诉我们，我们优先邀他入驻！

活动页按当前定位区县展示独立榜单。用户可以通过地图选择或手动录入想吃的商户，支持给榜单商户 `+1`。后台用微信文本内容安全审查手动录入内容，防重复投票，并在商户入驻后将其移出榜单。

## 风险等级

风险等级：`G2`。

原因：

- 涉及后端持久化、用户提交、去重与并发计数。
- 涉及微信外部接口 `msg_sec_check`，需要区分内容安全失败与上游调用失败。
- 涉及商户入驻成功链路的联动移除。
- 不涉及资金、支付、退款或权限高危写路径，因此不按 `G3` 处理。

## 范围

### 包含

- 小程序外卖首页新增活动 hero 入口。
- 小程序新增活动榜单页。
- 区县维度榜单：一个 `region_id` 一个榜单。
- 地图选择商户、手动录入商户。
- 榜单行 `+1` 按钮和成功动效。
- 后端榜单、提交、投票、重复提交处理。
- 手动录入走微信文本内容安全审查。
- 商户已入驻时跳转店铺主页。
- 商户入驻后从榜单移除。

### 不包含

- WebSocket 实时推送。
- 运营后台配置活动内容。
- 活动奖励、优惠券、积分。
- 跨区县总榜。
- 复杂商户名称相似度人工合并后台。

## 产品规则

### 榜单维度

- 榜单按当前定位区县隔离，使用后端已存在的 `region_id` 作为真值。
- 用户定位变化到另一区县后，应加载新区县榜单。
- 榜单查询只返回未入驻、未移除、状态为 active 的候选商户。

### 排序

- 主排序：`want_count` 降序。
- 次排序：最近一次 `+1` 时间倒序。
- 再兜底：`id` 升序，保证稳定分页或稳定刷新。

### 榜单展示

- 前五名使用特殊序号或徽章。
- 第 1 名视觉上突出冠军感。
- 第 6 名以后使用普通序号。
- 商户名后展示 `+1` 按钮。
- 用户已支持过该商户时，按钮显示为 `已+1` 或 disabled 状态，再点提示“你已经支持过这家了”。
- 点击 `+1` 成功后：
  - 后端计数成功才更新前端。
  - 榜单刷新一次。
  - 当前行短暂高亮。
  - 按钮附近浮出 `+1` 动效。
  - 若排名变化，刷新后新位置高亮。

### 手动录入

手动录入店名后，后端按以下顺序处理：

1. 归一化店名。
2. 查询当前区县已入驻外卖商户。
3. 查询当前区县榜单候选商户。
4. 若未命中已入驻和榜单，再调用微信 `msg_sec_check`。
5. 内容安全通过后创建候选商户，并给当前用户记录一次支持。

处理结果：

- 已入驻：不进榜单，返回 `merchant_available` 和 `merchant_id`，前端提示“这家已经可以点外卖了”，提供“去点单”按钮跳转店铺主页。
- 已在榜单：不创建重复项，返回 `found_in_rank`，前端滚动定位到对应榜单行并高亮，提示“榜单里已经有这家，点 +1 支持一下”。
- 新候选：返回 `created` 或 `voted`，刷新榜单。
- 内容安全不通过：返回稳定中文业务错误，前端提示“店名包含暂不支持发布的内容，请修改后再试”。

### 地图选择

地图选择拿到店名、地址、经纬度后，后端按以下顺序处理：

1. 归一化店名。
2. 查询当前区县已入驻外卖商户。
3. 查询当前区县榜单候选商户。
4. 未命中则直接创建候选商户，并给当前用户记录一次支持。

地图来源不需要走微信文本内容安全审查，但仍需要长度、空白、坐标和区县校验。

### 重复提交

- 同一用户在同一区县对同一个榜单商户只能贡献一次 `+1`。
- 后端通过唯一约束和事务保证并发重复点击不会多加。
- 重复点击返回 `already_voted`，前端不刷新计数，只提示已支持过。

### 商户入驻联动

- 商户入驻成功或激活为可外卖商户后，后端按 `region_id + normalized_name` 匹配榜单候选。
- 命中后将候选状态改为 `matched` 或 `removed`，记录 `matched_merchant_id`。
- 榜单查询排除已匹配项。

## 后端设计

### 建议数据表

#### `wanted_merchants`

- `id`
- `region_id`
- `normalized_name`
- `display_name`
- `address`
- `latitude`
- `longitude`
- `source`: `map` / `manual`
- `want_count`
- `status`: `active` / `matched` / `removed`
- `matched_merchant_id`
- `last_voted_at`
- `created_at`
- `updated_at`

建议约束：

- unique partial index: `(region_id, normalized_name)` where `status = 'active'`
- `want_count >= 0`

#### `wanted_merchant_votes`

- `id`
- `wanted_merchant_id`
- `region_id`
- `user_id`
- `created_at`

建议约束：

- unique: `(region_id, user_id, wanted_merchant_id)`

### 接口草案

#### 查询榜单

`GET /v1/wanted-merchants?region_id=...&page_id=1&page_size=50`

返回：

```json
{
  "items": [
    {
      "id": 123,
      "region_id": 9,
      "rank": 1,
      "display_name": "某某炸鸡",
      "address": "宁晋县...",
      "want_count": 24,
      "has_voted": false
    }
  ],
  "total": 1,
  "page_id": 1,
  "page_size": 50
}
```

#### 提交或投票

`POST /v1/wanted-merchants/votes`

请求：

```json
{
  "region_id": 9,
  "source": "manual",
  "name": "某某炸鸡",
  "address": "",
  "latitude": null,
  "longitude": null
}
```

返回：

```json
{
  "result": "created",
  "wanted_merchant_id": 123,
  "merchant_id": null,
  "rank": 1,
  "want_count": 24
}
```

`result` 枚举：

- `created`: 新建候选并计一次支持。
- `voted`: 已存在候选并成功 `+1`。
- `already_voted`: 当前用户已支持过该候选。
- `found_in_rank`: 用户重复输入，候选已在榜单中，但本次不自动加数，前端定位并引导点击 `+1`。
- `merchant_available`: 已入驻外卖商户，前端跳店铺主页。

### 后端模块建议

- `locallife/db/migration/*_create_wanted_merchants.*.sql`
- `locallife/db/query/wanted_merchant.sql`
- `locallife/db/sqlc/tx_wanted_merchant_vote.go`
- `locallife/logic/wanted_merchant_service.go`
- `locallife/api/wanted_merchant.go`
- `locallife/api/server.go`

### 外部接口

- 手动录入调用 `server.wechatClient.MsgSecCheck(ctx, user.WechatOpenid, scene, name)`。
- 建议沿用评价文本审核的错误处理方式，`wechat.ErrRiskyTextContent` 映射为 400 业务错误，上游调用失败映射为 502 或内部稳定错误。

## 小程序设计

### 首页 hero

修改：

- `weapp/miniprogram/pages/takeout/index.wxml`
- `weapp/miniprogram/pages/takeout/index.ts`
- `weapp/miniprogram/pages/takeout/index.wxss`

位置：

- 外卖首页搜索框下方、菜系品类网格上方。

行为：

- 点击跳转活动页。
- hero 不额外拉重数据，避免首页首屏请求膨胀。

### 活动页

建议新增：

- `weapp/miniprogram/pages/takeout/wanted-merchants/index.json`
- `weapp/miniprogram/pages/takeout/wanted-merchants/index.ts`
- `weapp/miniprogram/pages/takeout/wanted-merchants/index.wxml`
- `weapp/miniprogram/pages/takeout/wanted-merchants/index.wxss`
- `weapp/miniprogram/api/wanted-merchant.ts`

页面任务：

- 展示当前区县榜单。
- 支持手动输入店名。
- 支持地图选择店铺。
- 支持榜单行 `+1`。
- 处理重复输入后的滚动定位和高亮。
- 处理已入驻商户跳转店铺主页。

状态：

- 首屏 loading。
- 首屏 error + 重试。
- 当前区县空榜。
- 定位缺失，引导定位或手动选择位置。
- 提交中，禁用提交按钮和当前行 `+1`。
- 内容安全失败。
- 已支持过。
- 提交成功后刷新并高亮。

视觉：

- 顾客侧页面，使用 `.github/standards/weapp/DESIGN_SYSTEM.md`。
- 优先使用 TDesign Miniprogram：
  - `t-search` 或 `t-input` 承接输入。
  - `t-button` 承接提交和 `+1`。
  - `t-empty` 承接空态。
  - `t-loading` 承接加载。
  - `t-tag` 或本地轻量 rank 样式承接前五名徽章。

## 实施任务

### Task 1: 后端 schema 和 sqlc

- 新增 migration 创建 `wanted_merchants` 和 `wanted_merchant_votes`。
- 新增 `locallife/db/query/wanted_merchant.sql`。
- 生成 sqlc。
- 验证命令：
  - `cd locallife && make sqlc`
  - `cd locallife && make check-generated`

### Task 2: 后端投票事务

- 新增事务 helper，保证“创建候选 + 创建 vote + want_count +1”原子完成。
- 并发重复投票必须只成功一次。
- 重复投票返回明确结果，不吞掉 unique conflict。

### Task 3: 后端业务服务

- 新增店名归一化。
- 当前区县已入驻商户匹配。
- 当前区县榜单候选匹配。
- 手动录入内容安全审查。
- 地图来源坐标校验。
- 商户入驻成功后候选移除。

### Task 4: 后端 API

- 新增查询榜单接口。
- 新增提交或投票接口。
- 注册路由。
- 补 Swagger 注释。
- 验证命令：
  - `cd locallife && make swagger`
  - `cd locallife && make check-generated`

### Task 5: 后端测试

覆盖：

- 手动录入内容安全通过后创建候选并计数。
- 手动录入内容安全失败不落库。
- 地图选择不走内容安全但创建候选。
- 已入驻商户返回 `merchant_available`。
- 已在榜单商户返回 `found_in_rank` 或投票成功。
- 重复投票返回 `already_voted` 且不加数。
- 榜单按区县隔离。
- 商户入驻后候选不再出现在榜单。

建议命令：

- `cd locallife && go test ./api ./logic`
- 若 DB-backed sqlc 测试可用，补跑相关 `db/sqlc` 测试。

### Task 6: 小程序 API service

- 新增 `weapp/miniprogram/api/wanted-merchant.ts`。
- 定义请求、响应和 result 枚举类型。
- 不要把接口塞入已有大 service。

### Task 7: 小程序首页 hero

- 在外卖首页增加活动 hero。
- 点击进入活动页。
- 不额外增加首页首屏后端请求。

### Task 8: 小程序活动页

- 新增页面并注册到 `app.json` 的外卖相关分包。
- 加载当前定位区县榜单。
- 实现手动录入、地图选择、榜单行 `+1`。
- 实现滚动定位、行高亮、`+1` 动效。
- 实现已入驻商户跳店铺主页。
- 实现重复提交和重复投票反馈。

### Task 9: 小程序验证

建议命令：

- `cd weapp && npm run compile`
- 若变更跨多个页面和 service，跑：
  - `cd weapp && npm run quality:check`

## 验收标准

- 外卖首页能看到活动 hero，并能进入活动页。
- 活动页按当前定位区县加载榜单。
- 手动输入违规内容不会入榜，用户看到中文业务提示。
- 手动输入已入驻商户，前端提示并能跳转店铺主页。
- 手动输入已在榜单商户，页面滚动定位到该行并提示可点击 `+1`。
- 点击榜单行 `+1` 后，后端确认成功才刷新榜单，当前行出现 `+1` 动效。
- 同一用户重复 `+1` 不增加计数。
- 不同用户 `+1` 后榜单按总想吃人数刷新排序。
- 商户入驻后不再出现在榜单。
- 首页没有因为 hero 增加额外重型首屏请求。

## 待确认项

- 地图选择若没有稳定 `place_id`，第一版按 `region_id + normalized_name` 合并；后续如能拿到腾讯地图稳定地点 ID，可追加字段增强去重。
- 手动录入是否允许补充地址：第一版建议只录店名，降低提交成本；地图选择负责地址和坐标。
- 榜单每页数量：第一版建议 50 条，后续按活动流量调整分页。
