# 阿里云 OSS / CDN 控制台操作手册

> 对应 MEDIA_TASK_PLAN Phase 1 基础设施配置。
> 所有操作均在 **阿里云控制台** 完成，无需改动代码。
> 操作完成后对应勾选 MEDIA_TASK_PLAN Phase 1 中的条目。

---

## 前置信息

| 参数 | 值 |
|---|---|
| OSS 地域 | `cn-beijing`（华北2）|
| 公共桶名 | `locallife-media-public-2025` |
| 私有桶名 | `locallife-media-private-2025` |
| CDN 域名 | `cdn.test.merrydance.cn` |
| Web 前端域名 | `https://ls.merrydance.cn` |
| API 服务域名 | `https://llapi.merrydance.cn` |
| 图片 thumb 宽度 | 200px |
| 图片 card 宽度 | 400px |
| 图片 detail 宽度 | 960px |

---

## 一、公共桶 `locallife-media-public-2025`

### 1.1 创建桶

1. 进入 OSS 控制台 → **Bucket 列表** → **创建 Bucket**
2. 填写：
   - Bucket 名称：`locallife-media-public-2025`
   - 地域：**华北2（北京）**
   - 存储类型：标准存储
   - 读写权限：**私有**（不选"公共读"，CDN 回源访问，不直接对外开放）
   - 版本控制：关闭（sha256 文件名已自带去重，无需版本控制）
3. 点击确定

> 如桶已存在，跳到 1.2 确认 ACL 为私有。

---

### 1.2 访问控制（ACL）

1. 进入桶 → **权限管理** → **读写权限**
2. 确认为 **私有**（禁止匿名读，只有 CDN 回源可访问）

---

### 1.3 CORS 配置（允许客户端直传）

客户端（Web 和小程序）通过 OSS POST Policy 上传图片，需要 OSS 允许跨域请求。

1. 进入桶 → **权限管理** → **跨域设置（CORS）** → **创建规则**
2. 填写以下规则：

| 字段 | 值 |
|---|---|
| 来源 | `https://ls.merrydance.cn` |
| 允许 Methods | `GET, POST, PUT, HEAD, OPTIONS` |
| 允许 Headers | `*` |
| 暴露 Headers | `ETag, x-oss-request-id` |
| 缓存时间 | `600`（秒） |

3. 再添加一条规则，用于小程序（微信小程序直传 OSS 时 Origin 为 `https://servicewechat.com`）：

| 字段 | 值 |
|---|---|
| 来源 | `https://servicewechat.com` |
| 允许 Methods | `POST, OPTIONS` |
| 允许 Headers | `*` |
| 暴露 Headers | `ETag, x-oss-request-id` |
| 缓存时间 | `600` |

---

### 1.4 防盗链（Referer 白名单）

防止外部直接通过 OSS 域名访问，只允许 CDN 回源。

1. 进入桶 → **权限管理** → **防盗链**
2. 开启防盗链
3. Referer 白名单填写 CDN 回源域名（格式为桶的访问域名，如 `*.cdn.aliyun.com`）
   - 也可以在 CDN 侧配置回源鉴权（推荐，见第三部分），两者选一即可
4. **勾选"允许空 Referer"：否**（禁止无 Referer 的直接访问）

> 注意：配置 CDN 回源鉴权后，此步骤可省略，回源鉴权安全性更高。

---

### 1.5 图片处理说明

代码使用 **OSS 内联图片处理参数**，无需在控制台创建命名样式。

URL 格式示例：
```
https://cdn.test.merrydance.cn/public/dish_image/abc123.jpg?x-oss-process=image/resize,w_200,m_lfit/format,webp
```

OSS 图片处理默认开启，只需确认：

1. 进入桶 → **数据处理** → **图片处理**
2. 确认图片处理服务已**开启**（默认开启，若未开启点击开启）
3. **不需要**创建任何命名样式，代码已通过 URL 参数直接传递处理规则

---

## 二、私有桶 `locallife-media-private-2025`

### 2.1 创建桶

1. OSS 控制台 → **Bucket 列表** → **创建 Bucket**
2. 填写：
   - Bucket 名称：`locallife-media-private-2025`
   - 地域：**华北2（北京）**
   - 存储类型：标准存储
   - 读写权限：**私有**
3. 点击确定

> 如桶已存在，确认 ACL 为私有即可。

---

### 2.2 访问控制（ACL）

1. 进入桶 → **权限管理** → **读写权限**
2. 确认为 **私有**（所有访问均通过后端签名 URL）

---

### 2.3 CORS 配置（允许客户端直传私有材料）

骑手/运营商入驻证照会直传到私有桶，同样需要 CORS。

1. 进入桶 → **权限管理** → **跨域设置（CORS）** → **创建规则**

| 字段 | 值 |
|---|---|
| 来源 | `https://ls.merrydance.cn` |
| 允许 Methods | `GET, POST, PUT, HEAD, OPTIONS` |
| 允许 Headers | `*` |
| 暴露 Headers | `ETag, x-oss-request-id` |
| 缓存时间 | `600` |

再添加小程序规则：

| 字段 | 值 |
|---|---|
| 来源 | `https://servicewechat.com` |
| 允许 Methods | `POST, OPTIONS` |
| 允许 Headers | `*` |
| 暴露 Headers | `ETag, x-oss-request-id` |
| 缓存时间 | `600` |

---

### 2.4 Presigned URL 过期时间确认

私有图片通过后端签发的 Presigned URL 访问，默认 5 分钟。无需控制台配置，代码已控制。

---

## 三、CDN 配置

CDN 仅对接 **公共桶**，私有桶不走 CDN。

### 3.1 添加域名

1. 进入 **CDN 控制台** → **域名管理** → **添加域名**
2. 填写：
   - 加速域名：`cdn.test.merrydance.cn`
   - 业务类型：**图片小文件**
   - 源站类型：**OSS 域名**
   - 源站地址：`locallife-media-public-2025.oss-cn-beijing.aliyuncs.com`
   - 端口：443（HTTPS 回源）

---

### 3.2 HTTPS 证书

1. 进入加速域名 → **HTTPS 配置**
2. 开启 HTTPS
3. 上传或选择已有证书（`cdn.test.merrydance.cn` 对应的 SSL 证书）
4. 开启 **HTTP/2**
5. 开启 **强制跳转 HTTP→HTTPS**

---

### 3.3 压缩配置

1. 进入域名 → **性能优化**
2. 开启 **Gzip 压缩**
3. 开启 **Brotli 压缩**（图片本身不受影响，对 JSON 响应有效）

---

### 3.4 缓存规则

图片文件名包含 sha256 哈希，内容不变，可以设置超长缓存。

1. 进入域名 → **缓存配置** → **缓存过期时间**
2. 添加规则：

| 规则类型 | 路径/后缀 | 缓存时间 |
|---|---|---|
| 文件后缀 | `jpg,jpeg,png,webp,gif,avif` | 365 天 |
| 文件后缀 | `mp4,mov` | 365 天 |

---

### 3.5 图片处理查询参数透传（关键）

CDN 需要把 `?x-oss-process=...` 参数透传给 OSS 源站，并且**不同参数值缓存不同结果**，否则 CDN 会把 thumb/card/detail 当同一个文件缓存。

1. 进入域名 → **缓存配置** → **自定义缓存键** 或 **参数过滤**
2. 选择 **保留所有参数**（即不过滤任何查询参数）
   - 阿里云 CDN 对应设置：**URL 参数** → 选择 **保留所有参数**

> 这一步非常重要：若设置为"忽略参数"，`thumb`/`card`/`detail` 三个规格会被缓存为同一 URL，导致尺寸混乱。

---

### 3.6 回源鉴权（防止直接访问 OSS 源站绕过 CDN）

配置后，只有 CDN 能访问 OSS 源站，直接访问 OSS 域名会被拒绝。

1. 进入桶 `locallife-media-public-2025` → **权限管理** → **授权策略**
2. 或在 CDN 侧开启 **回源鉴权**（URL 鉴权）
   - CDN 控制台 → 域名 → **访问控制** → **URL 鉴权**（可选，根据安全需求决定）

简单方案：在 OSS 侧设置 Referer 白名单，只允许 CDN 回源 IP 段访问（阿里云 CDN 提供固定回源 IP 段列表）。

---

### 3.7 验证（配置完成后）

依次访问以下 URL 确认配置正确：

```bash
# 1. 上传一张测试图到公共桶（可用 ossutil 或控制台手动上传）
#    假设上传到：public/test/sample.jpg

# 2. 原图访问（应返回 200）
curl -I "https://cdn.test.merrydance.cn/public/test/sample.jpg"

# 3. thumb 规格（应返回 200，Content-Type: image/webp）
curl -I "https://cdn.test.merrydance.cn/public/test/sample.jpg?x-oss-process=image/resize,w_200,m_lfit/format,webp"

# 4. card 规格
curl -I "https://cdn.test.merrydance.cn/public/test/sample.jpg?x-oss-process=image/resize,w_400,m_lfit/format,webp"

# 5. detail 规格
curl -I "https://cdn.test.merrydance.cn/public/test/sample.jpg?x-oss-process=image/resize,w_960,m_lfit/format,webp"

# 6. 确认 CDN 命中（响应头 X-Cache 或 Via 中有 CDN 标识）
# 第一次请求 MISS，第二次请求同 URL 应 HIT
```

---

### 3.8 CDN 预热（上线时执行，非现在）

上线后对高频资源执行预热，避免首屏冷启动延迟。

1. 进入 CDN 控制台 → **刷新预热** → **URL 预热**
2. 提交菜品图、商户 logo 等高频资源的 URL 列表
3. 等待预热完成（通常 5~30 分钟）

---

## 四、操作完成核对清单

完成每项操作后在此打勾：

### 公共桶
- [ ] 桶已创建，ACL=私有
- [ ] CORS 规则已配置（Web + 小程序两条）
- [ ] 图片处理服务已确认开启
- [ ] 防盗链或回源鉴权已配置

### 私有桶
- [ ] 桶已创建，ACL=私有
- [ ] CORS 规则已配置（Web + 小程序两条）

### CDN
- [ ] 域名 `cdn.test.merrydance.cn` 已添加，回源指向公共桶
- [ ] HTTPS 证书已配置，HTTP/2 已开启
- [ ] Gzip / Brotli 压缩已开启
- [ ] 缓存规则已配置：图片文件 365 天
- [ ] **URL 参数缓存策略：保留所有参数（最关键）**
- [ ] 验证步骤 3.7 全部通过

---

## 五、常见问题

**Q: 上传时报 CORS 错误怎么办？**
检查 OSS CORS 规则中的"来源"是否与实际请求的 Origin 完全匹配（包括 https:// 前缀）。小程序的 Origin 是 `https://servicewechat.com`。

**Q: CDN 返回的图片没有被压缩为 webp？**
确认 CDN 的"URL 参数"设置为保留所有参数，同时确认 OSS 图片处理功能已开启。用 curl 加 `-v` 查看返回的 Content-Type 是否为 `image/webp`。

**Q: 不同规格图片（thumb/card/detail）返回的是同一张大图？**
原因是 CDN 忽略了查询参数，把不同 `x-oss-process` 值当同一 URL 缓存了。按 3.5 节把 URL 参数策略改为"保留所有参数"，然后刷新 CDN 缓存。

**Q: Presigned URL 过期后仍能访问？**
私有桶 Presigned URL 过期由 OSS 服务端强制执行，无需额外配置。如遇问题，检查服务器时钟是否与 NTP 同步。
