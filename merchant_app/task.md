# 乐客来福商户端 Android App — 开发进度

> 注：本清单已按当前代码库实际落地情况同步。部分功能虽然代码已落地，但仍可能依赖后端接口联调或真机验证。

## Phase 0：提示词工程 & 规范文档
- [x] 根目录 `CLAUDE.md` 创建
- [x] `merchant_app/CLAUDE.md` 创建
- [x] `.github/standards/flutter/` 规范文档
  - [x] `README.md`
  - [x] `FLUTTER_APP_ARCHITECTURE.md`
  - [x] `APP_AUTH_BINDING.md`
  - [x] `PUSH_NOTIFICATION_STANDARDS.md`
  - [x] `ANDROID_KEEP_ALIVE_GUIDE.md`
- [x] `.github/instructions/flutter-merchant-app.instructions.md`
- [x] `.agents/workflows/flutter-dev.md`
- [x] Copilot instructions / .github README 更新

## Phase 1：基础设施

### Go 后端 — 绑定码认证
- [x] Redis 集成（已有 go-redis/v8）
- [x] `api/app_bind.go`: 生成绑定码 `POST /v1/auth/app-bind/code`
- [x] `api/app_bind.go`: 验证绑定码 `POST /v1/auth/app-bind/verify`
- [x] App 专用 refresh_token 有效期 365 天配置
- [ ] 频率限制：生成 3次/分钟/用户，验证 10次/分钟/IP

### Go 后端 — 厂商原生推送网关
- [ ] 推送网关接口定义 `push/provider.go`
- [ ] 华为/荣耀/小米/OPPO/vivo 厂商 Provider 实现
- [ ] 推送网关统一入口 `push/gateway.go`

### Go 后端 — 数据库 & API
- [ ] 数据库表：merchant_devices
- [ ] 数据库表：app_versions
- [ ] SQLC 查询生成
- [ ] API：设备注册 `/v1/merchant/device/register`
- [ ] API：版本检测 `/v1/app/version/latest`
- [ ] API：待接单订单查询 `/v1/merchant/orders/pending`
- [ ] 支付回调中集成推送通道
- [ ] 超时未接单定时任务

### Flutter App
- [x] 项目骨架搭建 + 依赖配置
- [x] 主题 / 配置 / 环境变量
- [x] 绑定码登录模块 (bind_code_page + auth_service)
  - [x] 输入 6 位码 UI
  - [x] 调用 `/v1/auth/app-bind/verify`
  - [x] `flutter_secure_storage` Token 存储
  - [x] Dio 拦截器自动续期
  - [x] 启动时自动登录 (tryAutoLogin)
- [x] WebSocket 客户端对接
- [x] 前台服务 (Foreground Service)
- [x] 语音播报 (预录音频 + TTS)
- [x] 全屏接单弹窗 (Full-Screen Intent)
- [x] 消息去重机制
- [x] 轮询兜底 (30s)
- [x] 厂商原生推送原生层接入
- [x] 接单确认交互
- [x] 订单列表页
- [ ] 云打印对接 (飞鹅)
- [x] OTA 自更新（前端检查流程与弹窗）
- [x] 权限引导页
- [x] 连接状态指示器

### 小程序端
- [x] 新增"绑定商户端 App"入口与弹窗
- [x] 调用 `POST /v1/auth/app-bind/code`
- [x] 展示 6 位绑定码 + 倒计时

## Phase 2：测试 & 打磨
- [ ] 推送网关单元测试
- [ ] 绑定码认证单元测试
- [ ] 四品牌真机测试
- [ ] 地推检查单文档
