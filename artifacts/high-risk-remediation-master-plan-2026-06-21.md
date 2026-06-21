# 五项高风险隐患专项审计与修复执行计划

日期：2026-06-21

## 背景

本分支从 `main` 创建专用 worktree 执行近 48 小时暴露的 5 个高风险隐患修复。执行口径是：每个小任务先审计真实链路和根因，再用测试锁定行为，修复后自查 review，通过后提交；每个隐患完成后再做隐患级 review，确认不影响正常功能且达到设计目标。

## 范围

| 编号 | 隐患 | 修复目标 |
| --- | --- | --- |
| HR-001 | 微信登录错误日志泄露 WeChat app secret | 畸形 `js_code` 本地 400，不外呼；provider URL/query/error 脱敏；轮换 secret 作为运维动作记录 |
| HR-002 | 商家 39 公开套餐接口真实用户可见 500 | 套餐 tags 解码兼容历史/驱动类型漂移，非核心 tags 异常不拖垮公开接口 |
| HR-003 | 微信/Tencent Security Team 扫描噪音过大 | 扫描拦截日志降噪，敏感 query 脱敏扩展，保留扫描 5xx 高信号 |
| HR-004 | 商家编辑菜品页调用 admin-only `/v1/tags` | 保留商家新增标签能力，新增商家域 create-or-link 标签接口，平台 `/v1/tags` 继续 admin-only |
| HR-005 | Baofoo account/open webhook 接近 5 秒 | webhook ACK 路径可观测、重复回调幂等收敛，重业务应用不阻塞 ACK |

## 全局约束

- 不修改主工作区里其他人的未提交内容。
- 不删除商家新增标签能力。
- 不把商家角色加入平台 admin-only `/v1/tags` 写权限。
- 不信任客户端传入的 `merchant_id`、owner、状态或 provider 字段。
- G3 路径必须说明时序、幂等、越权、重复触发、回滚或补偿。
- 敏感字段默认不进日志、响应和错误链。

## 执行顺序

1. HR-001：微信登录输入校验、URL 构造和脱敏。
2. HR-002：公开套餐 tags 解码兼容和降级观测。
3. HR-003：扫描日志分类降噪和请求 query 脱敏补齐。
4. HR-004：商家标签新增权限域、后端接口、SQL/迁移、前端改线。
5. HR-005：宝付开户回调耗时观测、重复回调幂等和快速 ACK。

## 验收口径

- HR-001：畸形 `js_code` 返回 400；mock provider 不被调用；日志和错误不含 secret/code/token。
- HR-002：`[]interface{}` tags 不再导致 500；恶性 tags 走明确降级或错误记录；正常 tags 不变。
- HR-003：scanner 401/403/404/429 不再刷 warn；scanner 5xx 仍为 error；安全指标/字段可定位。
- HR-004：商家可以新增自己的标签；重复新增幂等；未关联、inactive、错误 type 的 tag 不能保存到业务对象；商家直接调用 `/v1/tags` 仍 403。
- HR-005：重复开户回调只产生一次事实/应用；重复回调快速 ACK；签名或 identity 失败不写成功事实。

## 验证基线

- 后端：`PATH="/usr/local/go/bin:$PATH" go test ./api` 已作为 worktree 基线通过。
- SQL 变更：运行 `make sqlc` 和 `make check-generated`。
- 小程序变更：使用 `PATH="$HOME/.local/bin:$PATH"` 运行最小相关 lint/quality 命令。
