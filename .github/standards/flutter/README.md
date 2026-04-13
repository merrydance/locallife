# Flutter Standards Index

本目录是 LocalLife Flutter 移动端在 `.github` 下的正式权威入口。

适用范围：

- `merchant_app/`
- 与 Flutter 实现直接相关的 `.github/` 资产

## 推荐阅读顺序

1. `.github/standards/engineering/README.md`
2. `.github/standards/flutter/PRODUCTION_ROBUSTNESS_BASELINE.md`
3. `.github/standards/flutter/FLUTTER_UI_DESIGN_STANDARDS.md`
4. `.github/standards/flutter/FLUTTER_APP_ARCHITECTURE.md`
5. `.github/standards/flutter/APP_AUTH_BINDING.md`
6. `.github/standards/flutter/PUSH_NOTIFICATION_STANDARDS.md`
7. `.github/standards/flutter/ANDROID_KEEP_ALIVE_GUIDE.md`
8. `.github/standards/flutter/REVIEW_CHECKLIST.md`
9. `.github/standards/flutter/TASK_ANNOTATION_TEMPLATE.md`

## 文档角色

- `PRODUCTION_ROBUSTNESS_BASELINE.md`: 生产级鲁棒性默认基线、失败模型、禁止项、风险提示、验证深度、任务标注字段。
- `FLUTTER_UI_DESIGN_STANDARDS.md`: merchant_app 的长期视觉与交互设计基线，解决 DESIGN.md 不足以直接落地的问题。
- `FLUTTER_APP_ARCHITECTURE.md`: 分层架构、状态管理、依赖注入、目录约定。
- `APP_AUTH_BINDING.md`: 绑定码认证方案、Token 管理、安全设计。
- `PUSH_NOTIFICATION_STANDARDS.md`: 三通道消息投递、去重、接单确认闭环、语音播报规范。
- `ANDROID_KEEP_ALIVE_GUIDE.md`: 国产厂商保活适配指南、前台服务配置、权限引导。
- `REVIEW_CHECKLIST.md`: `G2` / `G3` 路径默认审查清单，强调高风险缺陷和残余风险。
- `TASK_ANNOTATION_TEMPLATE.md`: 任务卡、实现请求、缺陷修复请求可直接复用的标注模板。

## 使用规则

- 先看本目录和 engineering index，再下钻到某个特定 feature 的实现。
- 非平凡实现或 review 先用 `PRODUCTION_ROBUSTNESS_BASELINE.md` 统一风险、恢复、失败模型和验证语言，再进入专题文档。
- 任何涉及页面布局、视觉层级、状态呈现、按钮优先级、中文可读性的任务，都要同时看 `FLUTTER_UI_DESIGN_STANDARDS.md`。
- `G2` / `G3` review 默认使用 `REVIEW_CHECKLIST.md`，避免 review 只看代码风格不看恢复与误操作风险。
- Flutter 的 UI 反馈标准复用 `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md` 的核心原则（一次操作一个反馈、不泄漏原始错误）。
- 与后端推送网关相关的改动，同时参考 `.github/standards/backend/README.md`。

## CI 与门禁

- `merchant_app/` 目录变更会触发 `.github/workflows/flutter-quality.yml`。
- CI 默认执行 `flutter analyze` 和 `flutter test`。
- changed-file architecture guard 会拦截 widget 或 page 层直接导入 `Dio`、`FlutterSecureStorage`、`sqflite`、`ApiClient` 这类基础设施，以及 `lib/config/env.dart` 之外的硬编码接口地址。
