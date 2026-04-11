---
description: Flutter 商户端开发流程
---

# Flutter 商户端开发 Workflow

// turbo-all

## 前置检查

1. 确认当前在 `feature/merchant-android-app` 分支
```bash
git branch --show-current
```

2. 确认 Flutter SDK 可用
```bash
flutter --version
```

## 开发流程

### 修改 Flutter 代码后

3. 运行静态分析
```bash
cd merchant_app && flutter analyze
```

4. 运行单元测试
```bash
cd merchant_app && flutter test
```

5. 构建调试版 APK
```bash
cd merchant_app && flutter build apk --debug
```

### 修改后端代码后

6. 如果改了 SQL，重新生成
```bash
cd locallife && make sqlc
```

7. 如果改了 API 注解，重新生成 Swagger
```bash
cd locallife && make swagger
```

8. 运行后端单元测试
```bash
cd locallife && make test-unit
```

### 构建发布版

9. 构建 release APK
```bash
cd merchant_app && flutter build apk --release
```

## 参考文档

- Flutter 架构: `.github/standards/flutter/FLUTTER_APP_ARCHITECTURE.md`
- 推送标准: `.github/standards/flutter/PUSH_NOTIFICATION_STANDARDS.md`
- 保活指南: `.github/standards/flutter/ANDROID_KEEP_ALIVE_GUIDE.md`
- 后端规范: `.github/standards/backend/README.md`
