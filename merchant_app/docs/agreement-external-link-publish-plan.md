# 商户端协议外链发布方案

适用目标：为小米、华为、荣耀、OPPO、vivo、Google Play 等应用市场准备可公开访问的隐私政策链接和协议链接。

## 1. 当前项目现状

### 已有内容

- 已有静态 HTML 导出：`legal_exports/agreements_v1_2_0/USER_AGREEMENT.html`
- 已有静态 HTML 导出：`legal_exports/agreements_v1_2_0/MERCHANT_AGREEMENT.html`
- 已有静态 HTML 导出：`legal_exports/agreements_v1_2_0/PRIVACY_POLICY.html`
- 商户端应用内已有“隐私政策”和“用户协议”入口：`lib/features/settings/about_page.dart`
- 后端接口文档已定义可查询的协议类型：`PRIVACY_POLICY`、`USER_AGREEMENT`

### 当前缺口

- 商户端是面向商户使用的应用，但当前应用内入口指向的是 `USER_AGREEMENT`，而不是现成的 `MERCHANT_AGREEMENT`
- 应用市场通常要求填写“公开可访问链接”，不能只依赖 App 内接口页

## 2. 建议的发布原则

1. 对外提交给应用市场的链接，使用稳定 URL，不直接用临时文件地址
2. 协议正文使用带版本号的静态 HTML，保证可追溯
3. 对外再提供一个“latest”稳定链接，供应用市场长期填写
4. 隐私政策和协议都要保留发布日期、版本号、运营主体信息
5. 商户端应用优先提交“隐私政策 + 商户协议”，不要把消费者协议误当成商户端协议

## 3. 推荐的 URL 结构

建议在公司官网、后端静态资源域名或对象存储 CDN 下发布。

示例结构：

- `https://static.example.com/legal/merchant/privacy-policy/v1.0.0.html`
- `https://static.example.com/legal/merchant/agreement/v1.2.0.html`
- `https://static.example.com/legal/merchant/privacy-policy/latest.html`
- `https://static.example.com/legal/merchant/agreement/latest.html`

应用市场表单里建议填写：

- 隐私政策：`.../privacy-policy/latest.html`
- 用户协议或软件许可协议：`.../agreement/latest.html`

## 4. 推荐的正文来源映射

### 隐私政策

目标链接：商户端隐私政策

当前可用源：`legal_exports/agreements_v1_2_0/PRIVACY_POLICY.html`

建议动作：

1. 将现有 `PRIVACY_POLICY.html` 发布到公开静态地址
2. 再额外提供一个 stable URL，例如 `latest.html`
3. 在应用市场和官网统一使用 stable URL

### 协议页

目标链接：商户端服务协议或商户入驻及数字化服务协议

当前可用源：`legal_exports/agreements_v1_2_0/MERCHANT_AGREEMENT.html`

建议动作：

1. 对外上架时优先使用 `MERCHANT_AGREEMENT.html` 作为商户端协议来源
2. 如果应用内后端目前只支持 `USER_AGREEMENT`，需要后续补齐商户协议接口类型，或在设置页对协议文案做区分
3. 在应用市场填写时，不建议提交消费者用户协议作为商户端主协议

## 5. 推荐的实际发布路径

### 方案 A：后端静态文件托管

适合已有官网域名或后端网关。

实施方式：

1. 在后端静态资源目录挂载 `legal/merchant/`
2. 把版本化 HTML 文件放进去
3. 用 Nginx 或对象网关把 `latest.html` 指向当前版本

优点：

- 统一域名
- 便于长期维护
- 不依赖第三方平台

### 方案 B：对象存储 + CDN

适合快速上线。

实施方式：

1. 将 HTML 上传到 OSS、COS、OBS 或 S3 兼容存储
2. 绑定自定义域名
3. 对外提供稳定 URL

优点：

- 发布快
- 容易做版本管理

注意：

- 不建议直接使用带签名的临时下载链接
- 不建议使用需要登录才能访问的地址

## 6. 建议的最小交付物

上架前至少准备这四项：

1. 一份公开可访问的商户端隐私政策 HTML
2. 一份公开可访问的商户端协议 HTML
3. 两个稳定链接：隐私政策 latest、商户协议 latest
4. 一份版本记录表，记录协议版本号和生效日期

## 7. 建议的落地顺序

1. 先确认商户端最终应该展示的是 `MERCHANT_AGREEMENT` 还是新的商户端用户协议
2. 将 `PRIVACY_POLICY.html` 发布到对外可访问的静态地址
3. 选定静态托管方式：后端静态目录或对象存储 CDN
4. 发布版本化 HTML 和 latest URL
5. 把应用市场链接统一换成 stable URL
6. 之后再考虑是否同步调整 App 内协议入口与后端协议类型

## 8. 当前建议结论

- 应用市场对外链接：优先准备“商户端隐私政策 + 商户协议”
- 当前可直接利用的协议源：`MERCHANT_AGREEMENT.html`
- 当前可直接利用的隐私政策源：`PRIVACY_POLICY.html`
- 当前残余风险：商户端 App 内入口仍是 `USER_AGREEMENT`，与现有商户协议资产存在语义不一致，后续建议统一