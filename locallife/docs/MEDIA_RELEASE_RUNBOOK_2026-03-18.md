# 媒体系统上线切换 Runbook

## 1. 文档目标

本文档用于指导媒体系统最终态的上线切换过程，确保在正式上线前后按固定步骤完成：

1. 基础设施准备
2. 配置项就绪
3. 数据库迁移
4. 后端部署
5. Web 发布
6. 小程序发布
7. 旧链路下线
8. 回滚处理

本 Runbook 默认场景为：

1. 服务尚未正式上线
2. 可以接受一次性切到最终态
3. 不保留长期双轨上传链路

## 2. 适用前提

执行本 Runbook 前，必须满足：

1. 所有设计文档已评审通过。
2. 代码实现已完成并通过测试。
3. OSS、公网域名、CDN、鉴权策略已准备完成。
4. Web 和小程序均已完成新上传链路接入。
5. 媒体 migration 已准备就绪。

## 3. 角色分工建议

建议上线时明确以下角色：

1. 发布负责人
2. 后端负责人
3. Web 负责人
4. 小程序负责人
5. 基础设施负责人
6. 验收负责人

每个步骤必须指定责任人，不允许“多人都在看但没人拍板”。

## 4. 上线前检查清单

### 4.1 基础设施检查

必须确认：

1. public bucket 已创建
2. private bucket 已创建
3. CDN 域名已配置
4. CDN 回源到 public bucket 正常
5. OSS 私有签名能力可用
6. STS 或等价临时授权能力可用
7. 水印规则或样式已配置完成

### 4.2 配置项检查

必须确认生产配置已具备：

1. FILE_STORAGE_PROVIDER=oss
2. OSS_PUBLIC_ENDPOINT
3. OSS_PRIVATE_ENDPOINT
4. OSS_PUBLIC_BUCKET
5. OSS_PRIVATE_BUCKET
6. OSS_REGION
7. OSS_STS_ROLE_ARN
8. OSS_STS_EXTERNAL_ID
9. CDN_PUBLIC_BASE_URL
10. PRIVATE_DOWNLOAD_URL_TTL
11. MEDIA_MAX_UPLOAD_BYTES
12. MEDIA_ALLOWED_IMAGE_TYPES
13. MEDIA_DIRECT_UPLOAD_EXPIRE
14. IMAGE_VARIANT_THUMB_WIDTH
15. IMAGE_VARIANT_CARD_WIDTH
16. IMAGE_VARIANT_DETAIL_WIDTH

### 4.3 代码与制品检查

必须确认：

1. 后端镜像或二进制已包含新媒体模块代码
2. Web 构建产物已切换到新上传链路
3. 小程序构建版本已切换到新上传链路
4. Swagger 文档已更新
5. migration 文件已进入主分支

### 4.4 测试检查

必须确认测试与验收清单中的关键项已经通过，至少包括：

1. upload-sessions
2. complete
3. private-access
4. Web 直传
5. 小程序直传
6. 公共图规格化访问
7. 私有图访问控制

## 5. 环境变量准备

### 5.1 app.env 变更原则

本次切换会新增媒体系统配置，并弱化旧本地上传配置。

上线前需确认：

1. 生产环境不再依赖 UPLOADS_BASE_DIR 作为主存储路径
2. 生产环境不再依赖本地 /uploads 路由作为主访问路径
3. 本地文件上传仅保留开发环境 fallback，如确有需要

### 5.2 生产配置模板要求

应新增媒体配置模板段，避免上线当天临时拼接环境变量。

建议上线前由基础设施负责人提供最终生产配置清单，并完成双人复核。

## 6. 发布顺序

### 6.1 推荐顺序总览

上线顺序固定为：

1. 基础设施与配置
2. 数据库 migration
3. 后端发布
4. Web 发布
5. 小程序发布
6. 旧上传链路禁用
7. 上线后验收

不得颠倒为：

1. 先发前端，再发后端
2. 先下旧接口，再发新前端

## 7. 数据库迁移步骤

### 7.1 执行前检查

执行 migration 前必须确认：

1. 当前数据库版本位于预期 migration 基线
2. migration 文件顺序正确
3. sqlc 代码已与 migration 对齐

### 7.2 执行方式

按现有 Makefile 习惯执行：

```bash
make migrateup
```

若需逐步执行：

```bash
make migrateup1
```

### 7.3 migration 后检查

必须确认：

1. media_assets 表存在
2. media_upload_sessions 表存在
3. 单图 media_asset_id 字段存在
4. 多图关联表存在
5. 旧 URL 字段的删除步骤已按发布计划执行

## 8. 后端发布步骤

### 8.1 发布前检查

必须确认：

1. 新后端版本已包含媒体模块
2. util.Config 已支持媒体配置项
3. Server 已注入 mediaService
4. 新 /v1/media 路由已注册
5. 旧上传接口是否仍保留兼容窗口已明确

### 8.2 发布动作

建议步骤：

1. 更新生产配置
2. 部署新后端版本
3. 观察 /health 与 /ready
4. 观察日志中媒体模块初始化是否成功

### 8.3 发布后即时验证

必须立刻验证：

1. /v1/media/upload-sessions 可用
2. /v1/media/complete 可用
3. /v1/media/private-access 可用
4. /metrics 正常输出，无异常错误飙升

## 9. Web 发布步骤

### 9.1 发布前检查

必须确认：

1. Web 已不再依赖旧图片上传接口
2. uploadMedia 统一封装已接入
3. 表单提交流程改为提交 media_asset_id
4. 关键列表页默认读取 thumb 或 card

### 9.2 发布后验证

至少验证：

1. 菜品图片上传
2. 桌台图片上传
3. 商户 logo 上传
4. 菜品列表图片展示
5. 商户列表或订单列表图片展示

## 10. 小程序发布步骤

### 10.1 发布前检查

必须确认：

1. 小程序已切换到 upload-sessions -> OSS -> complete
2. uploadFile 不再承担图片上传主链路
3. 业务提交改为 media_asset_id
4. getMediaDisplayUrl 已覆盖核心读图场景

### 10.2 发布后验证

至少验证：

1. 菜品图片上传
2. 桌台图片上传
3. 评价图片上传
4. 入驻材料上传
5. 菜单页、购物车、订单确认页图片显示

## 11. 旧链路下线步骤

### 11.1 可下线的旧能力

在 Web 和小程序均验证通过后，可执行下线：

1. /v1/dishes/images/upload
2. /v1/tables/images/upload
3. /v1/reviews/images/upload
4. /v1/merchants/images/upload
5. /uploads/*filepath 本地主链路
6. util/upload.go 生产主用途
7. util/image.go 生产主用途

### 11.2 下线顺序

顺序必须为：

1. 先确认客户端已切换
2. 再在后端禁用旧接口
3. 最后移除本地访问主链路

禁止先下线旧接口，再观察客户端是否报错。

## 12. 上线后核心验收步骤

上线完成后，按顺序执行人工验收：

### 12.1 公共图链路

1. Web 上传一张菜品图
2. 小程序上传一张评价图
3. 检查 media_assets 中记录是否生成
4. 检查业务表是否写入 media_asset_id
5. 检查前端读取的是否为 CDN 域名
6. 检查缩略图与详情图是否可访问

### 12.2 私有图链路

1. 上传一张营业执照
2. 检查业务表写入 media_asset_id
3. 检查私有访问接口可正确签发短期地址
4. 检查未授权用户无法访问

### 12.3 性能验收

1. 列表页图片加载明显快于旧方案
2. 应用服务日志中不再出现旧本地上传主链路日志
3. 应用 CPU、带宽不再随图片上传显著增长

## 13. 监控观察窗口

上线后建议观察至少 1 个完整业务高峰窗口，重点看：

1. upload-sessions 错误率
2. complete 错误率
3. private-access 错误率
4. OSS 失败率
5. CDN 命中率
6. 应用服务 5xx 错误率

## 14. 回滚策略

### 14.1 回滚原则

由于本次是最终态切换，不建议使用“回滚到旧前端但保留新数据库语义”的混乱方式。

回滚必须分层判断：

1. 配置回滚
2. 后端版本回滚
3. 前端版本回滚
4. 数据库回滚仅在必要时使用

### 14.2 可优先回滚的层

推荐优先顺序：

1. 回滚 Web/小程序发布版本
2. 回滚后端服务版本
3. 保留数据库 migration，不轻易 down

原因：

1. 数据库回滚风险最大
2. 一旦存在已写入的 media_asset_id 数据，盲目回滚 schema 容易造成更大问题

### 14.3 数据库回滚策略

除非刚上线且未产生实际新数据，否则不建议执行 migratedown。

数据库问题优先策略：

1. 热修后端代码适配当前 schema
2. 必要时补增量 migration

### 14.4 回滚触发条件

以下任一条件满足，可触发回滚评估：

1. upload-sessions 大面积失败
2. complete 大面积失败
3. 私有图片存在越权访问
4. Web 与小程序均无法正常上传
5. 核心页面图片大面积不可见

## 15. 故障处理手册

### 15.1 upload-sessions 失败

排查顺序：

1. 配置是否完整
2. STS 或存储服务凭证是否正常
3. media policy 是否错误拦截
4. 新后端版本是否正确加载媒体模块

### 15.2 complete 失败

排查顺序：

1. 对象是否真实上传到 OSS
2. object_key 是否一致
3. upload session 是否过期
4. OSS StatObject 是否异常

### 15.3 公共图打不开

排查顺序：

1. CDN 域名是否正确
2. 回源是否正确
3. 规格样式是否配置正确
4. 对象是否存在于 public bucket

### 15.4 私有图打不开

排查顺序：

1. private-access 是否返回成功
2. 签名 TTL 是否过短
3. 对象是否存于 private bucket
4. 用户权限判断是否误拒绝

## 16. 上线完成判定

当且仅当以下条件全部满足，才视为本次媒体系统上线完成：

1. 新媒体接口可用
2. Web 上传与读图正常
3. 小程序上传与读图正常
4. 业务表写入 media_asset_id
5. 公共图全部通过 CDN 访问
6. 私有图权限链路正常
7. 旧主链路已下线或明确禁用
8. 关键监控指标稳定

## 17. 建议后续补充文档

如需进一步提高执行性，建议继续补充：

1. 施工任务拆分表
2. 发布会议 checklist
3. 上线后 24 小时观察模板
