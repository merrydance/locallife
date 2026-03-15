## LocalLife API 审查报告（Phase 0）

审查时间：2026-03-15  
审查范围：`locallife/api/**/*.go` 中 `@Router` 注释与 handler 响应模式  
基线样本：`@Router` 注释 423 条

### 1. 结论摘要

1. 已存在明显的契约不一致，审查是必要且紧急的。
2. 主要问题不是“有没有接口”，而是“同类接口语义不统一”。
3. 建议先收敛高频后台场景（merchant/operator/platform 控制台）再全量治理。

### 2. 关键发现

#### A. 路由注释前缀不一致（高优先级）

现象：部分 `@Router` 注释未使用 `/v1` 前缀，但实际服务在 `/v1` 组内注册。

样例：
- `api/platform_stats.go` 注释使用 `/platform/...`（无 `/v1`）
- `api/platform_profit_sharing_config.go` 注释使用 `/platform/...`（无 `/v1`）
- `api/delivery_fee.go` 多个注释使用 `/delivery-fee/...`（无 `/v1`）

影响：
- Swagger 文档和真实路由易漂移
- 联调时出现“文档可调/线上 404”错觉

建议：
- 全量修正 `@Router` 注释到实际路径
- 将“注释路径一致性”纳入 CI 检查

#### B. 动作型路径使用较多（中优先级）

现象：存在较多 `.../approve`、`.../reject`、`.../submit`、`.../reset` 等动作型端点。

样例：
- `/v1/groups/{id}/join-requests/{request_id}/approve`
- `/v1/operator/appeals/{id}/review`
- `/v1/orders/{id}/cancel`

评估：
- 这类路径在状态机驱动领域并非错误
- 但应建立白名单与解释，避免无节制扩散

建议：
- 保留状态机动作类端点
- 新增动作型端点需在注释说明“为何非资源化”

#### C. 成功响应结构混用（高优先级）

现象：`200` 返回大量使用 `gin.H` 动态 map，结构体响应与 map 响应并存。

样本统计：
- `ctx.JSON(http.StatusOK, gin.H...)`：107 处
- `ctx.JSON(http.StatusOK, ...)` 总计：529 处

影响：
- 前端类型契约不稳定
- 字段名漂移风险高，回归成本高

建议：
- 优先将后台高频接口改为强类型响应结构体
- 列表响应统一分页字段

#### D. 状态码语义重载（高优先级）

现象：`404` 既用于资源不存在，也被用于“业务未开通/未配置”等空态。

样本统计（全局）：
- `http.StatusNotFound`：225 次
- `http.StatusBadRequest`：832 次
- `http.StatusInternalServerError`：1033 次

影响：
- 前端需要通过字符串猜状态
- 业务空态与异常态混淆

建议：
- 对“未开通/未配置/未申请”统一改为 `200 + status`
- 仅对象真实缺失时返回 `404`

### 3. 本轮已落地修复

1. 商户进件状态空态语义已标准化：
- 无申请记录 -> `200 + status=not_applied`

2. 商户资金账户读取接口已标准化：
- 未配置/未激活 -> `200 + account_status`（不再用 `404/400` 表示空态）

3. 标签删除接口缺口已补齐：
- 新增 `DELETE /v1/tags/:id`

### 4. 下一阶段建议（可直接排期）

P0（本周）
1. 修复全部 `@Router` 注释与真实路由不一致
2. 定义并冻结动作型路径白名单
3. 对 merchant/operator 控制台接口统一空态语义

P1（下周）
1. 将高频 `gin.H` 返回改造为强类型结构体
2. 统一分页返回字段（`items/total/page/limit/total_pages`）
3. 补契约回归测试（状态码 + 响应字段）

P2（后续）
1. 增加 CI 审计脚本：检查 `@Router` 前缀、响应结构、状态码规则
2. 输出机器可读契约报告（供前端自动对齐）
