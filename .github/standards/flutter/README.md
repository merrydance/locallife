# Flutter Standards Index

本目录是 LocalLife Flutter 移动端在 `.github` 下的正式权威入口。

适用范围：

- `merchant_app/`
- 与 Flutter 实现直接相关的 `.github/` 资产

## 推荐阅读顺序

1. `.github/standards/engineering/README.md`
2. `.github/standards/flutter/FLUTTER_APP_ARCHITECTURE.md`
3. `.github/standards/flutter/APP_AUTH_BINDING.md`
4. `.github/standards/flutter/PUSH_NOTIFICATION_STANDARDS.md`
5. `.github/standards/flutter/ANDROID_KEEP_ALIVE_GUIDE.md`

## 文档角色

- `FLUTTER_APP_ARCHITECTURE.md`: 分层架构、状态管理、依赖注入、目录约定。
- `APP_AUTH_BINDING.md`: 绑定码认证方案、Token 管理、安全设计。
- `PUSH_NOTIFICATION_STANDARDS.md`: 三通道消息投递、去重、接单确认闭环、语音播报规范。
- `ANDROID_KEEP_ALIVE_GUIDE.md`: 国产厂商保活适配指南、前台服务配置、权限引导。

## 使用规则

- 先看本目录和 engineering index，再下钻到某个特定 feature 的实现。
- Flutter 的 UI 反馈标准复用 `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md` 的核心原则（一次操作一个反馈、不泄漏原始错误）。
- 与后端推送网关相关的改动，同时参考 `.github/standards/backend/README.md`。
