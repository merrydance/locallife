# Android 应用市场上架清单

适用范围：Google Play 以外的国内 Android 应用市场，包括小米、华为、荣耀、OPPO、vivo。

## 1. 当前项目已确认的信息

- 应用名称：乐客来福商户
- 包名：`com.merrydance.locallife.merchant`
- 当前版本：`1.0.0+1`
- 签名 keystore：`android/release-keystore.jks`
- 证书导出文件：`android/release-cert.cer`、`android/release-cert.pem`
- 已配置正式 release signing，构建入口见 `android/app/build.gradle.kts`

### 签名指纹

- MD5：`AF:A4:DF:BE:77:AE:ED:C6:7E:4B:12:9E:4B:25:97:6B`
- SHA1：`D8:55:49:C9:D4:81:09:1F:34:A4:F4:82:19:DF:0F:71:23:70:C7:21`
- SHA256：`BF:78:A8:17:A9:13:BD:76:30:5B:12:2F:AD:D4:09:24:0E:96:A9:7B:BD:CF:51:EF:57:5F:7E:44:CA:65:C0:67`

### 证书主体

- `CN=LocalLife Merchant, O=LocalLife, OU=Merchant App, L=Hangzhou, ST=Zhejiang, C=CN`

## 2. 所有市场都会要的基础材料

- 开发者主体：个人或企业开发者账号
- 应用名称、应用简介、详细介绍
- 应用图标
- 安装包：通常为 APK；部分渠道也支持 AAB 或要求额外补充
- 包名
- 版本号、更新说明
- 隐私政策链接
- 用户协议链接
- 应用截图
- 联系方式：邮箱、手机号或客服信息
- 软著、ICP备案、行业资质：是否需要取决于应用类型和市场规则

## 3. 当前项目里已经能对上的材料

- 应用名在 `AndroidManifest.xml` 中已配置为“乐客来福商户”
- App 内已有“隐私政策”和“用户协议”入口，见 `lib/features/settings/about_page.dart`
- 后端已定义协议查询能力，支持 `PRIVACY_POLICY` 和 `USER_AGREEMENT`，见 `docs/backend-interface-requirements.md`
- 正式签名已配置完成，证书文件已导出到 `android/`

## 4. 仍需要你补齐或确认的材料

- 对外可访问的隐私政策 URL
- 对外可访问的用户协议 URL
- 应用商店展示截图
- 应用介绍文案、更新说明文案
- 开发者账号主体资料
- 如平台要求，提供测试账号或审核演示账号
- 若涉及经营性业务，补齐软著、ICP备案、经营资质等证明材料

## 5. 这个项目在审核中最容易被问到的权限和能力

以下权限都出现在 `android/app/src/main/AndroidManifest.xml`，提交市场前要准备好用途说明：

- `INTERNET`
- `ACCESS_NETWORK_STATE`
- `WAKE_LOCK`
- `RECEIVE_BOOT_COMPLETED`
- `VIBRATE`
- `FOREGROUND_SERVICE`
- `USE_FULL_SCREEN_INTENT`
- `ACCESS_WIFI_STATE`
- `CHANGE_WIFI_STATE`

建议提前准备这类说明：

- 为什么需要开机自启或开机后恢复服务
- 为什么需要前台服务常驻通知
- 为什么需要全屏意图通知
- 为什么需要保持 Wi-Fi 连接能力

当前版本已移除 `READ_PHONE_STATE`、存储权限、应用内安装相关权限，降低了国内市场审核阻力。

## 6. 第三方 SDK 与审核问答准备

当前技术路线已废弃 JPush，Android 构建应只保留各手机厂商原生推送所需配置，见 `android/app/build.gradle.kts`。

建议准备一份第三方 SDK 清单，至少说明：

- SDK 名称
- 使用目的
- 采集的数据类型
- 数据处理规则
- 对应隐私政策链接

审核时如果问到“为什么需要消息通知、保活、推送、自启动”，需要能明确回答这是商户接单、打印、订单提醒场景，而不是泛化的后台常驻。

## 7. 各市场额外关注点

### 小米应用商店

- 对权限说明、推送、自启动、后台保活比较敏感
- 经常会看安装包权限与隐私政策是否一致
- 如果有应用内更新，可能会追问 `REQUEST_INSTALL_PACKAGES` 的用途

### 华为 AppGallery Connect

- 更重视隐私政策、权限说明、账号体系、审核测试路径
- 若应用必须登录后才能使用，通常要准备审核账号
- 推送、通知、后台能力说明需要更完整

### 荣耀应用市场

- 整体要求与华为接近
- 注意区分是否需要单独开发者平台资料与单独审核材料
- 提前准备测试账号、测试路径、商户端核心操作流程说明

### OPPO 开放平台

- 对通知、后台、推送、自启动的用途说明较常见
- 如果涉及更新安装、蓝牙、设备信息读取，容易被问细节
- 审核时建议附上“核心业务流程截图 + 权限用途说明”

### vivo 开放平台

- 对权限和隐私合规审查通常较细
- 如果存在消息提醒、后台驻留、开机恢复，要准备业务必需性说明
- 登录后才能体验的应用，通常要准备测试账号或演示视频

## 8. 上架前建议做的最后检查

- 用正式签名重新打包，确认提交包来自 `release-keystore.jks`
- 固化版本号和版本说明
- 确认隐私政策、用户协议内容与实际权限、SDK 使用一致
- 检查权限是否可以最小化
- 准备审核账号、验证码接收方式或演示视频
- 准备截图：登录页、首页、订单提醒、打印或商户核心功能页
- 备份 `android/release-keystore.jks` 和 `android/key.properties`

## 9. 建议的提交顺序

1. 先准备隐私政策和用户协议的外链版本
2. 再整理权限用途说明和第三方 SDK 清单
3. 然后生成最终上架包
4. 再上传到小米、华为、荣耀、OPPO、vivo 平台分别补齐表单
5. 审核被问到问题时，以同一份权限/SDK 说明为基础统一回复

## 10. 配套文档

- 协议外链发布方案：`docs/agreement-external-link-publish-plan.md`
- 应用市场权限说明：`docs/android-market-permission-explanations.md`