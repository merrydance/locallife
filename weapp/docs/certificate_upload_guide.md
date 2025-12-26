# 证照上传接口使用指南

本文档说明所有入驻申请中涉及的证照上传接口的调用规范。

> [!IMPORTANT]
> **核心规则**：所有证照上传接口都是"上传 + OCR"一体化的。前端必须在请求中携带图片文件，后端会自动保存图片并异步执行 OCR。

---

## 一、商户入驻申请

### 1.1 接口列表

| 接口 | 方法 | 说明 |
|------|------|------|
| `/v1/merchant/application` | GET | 获取或创建申请草稿 |
| `/v1/merchant/application/basic` | PUT | 更新基础信息（名称、电话、地址、经纬度） |
| `/v1/merchant/application/images` | PUT | 更新门头照和环境照（JSON 数组） |
| `/v1/merchant/application/license/ocr` | POST | **上传营业执照** + 异步 OCR |
| `/v1/merchant/application/foodpermit/ocr` | POST | **上传食品经营许可证** + 异步 OCR |
| `/v1/merchant/application/idcard/ocr` | POST | **上传身份证** + 异步 OCR |
| `/v1/merchant/application/submit` | POST | 提交申请 |
| `/v1/merchant/application/reset` | POST | 重置被拒绝的申请 |

### 1.2 证照上传接口详情

#### POST `/v1/merchant/application/license/ocr`
**上传营业执照并触发 OCR**

```
Content-Type: multipart/form-data

字段:
- image (file, 必填): 营业执照图片文件
```

**响应**: 返回完整的申请草稿对象，其中 `business_license_ocr.status` 为 `"pending"`，表示 OCR 正在后台执行。

---

#### POST `/v1/merchant/application/foodpermit/ocr`
**上传食品经营许可证并触发 OCR**

```
Content-Type: multipart/form-data

字段:
- image (file, 必填): 食品经营许可证图片文件
```

**响应**: 返回完整的申请草稿对象，其中 `food_permit_ocr.status` 为 `"pending"`。

---

#### POST `/v1/merchant/application/idcard/ocr`
**上传法人身份证并触发 OCR**

```
Content-Type: multipart/form-data

字段:
- image (file, 必填): 身份证图片文件
- side (string, 必填): "Front" (正面) 或 "Back" (背面)
```

**响应**: 返回完整的申请草稿对象，其中 `id_card_front_ocr.status` 或 `id_card_back_ocr.status` 为 `"pending"`。

---

## 二、骑手入驻申请

### 2.1 接口列表

| 接口 | 方法 | 说明 |
|------|------|------|
| `/v1/rider/application` | GET | 获取或创建申请草稿 |
| `/v1/rider/application/basic` | PUT | 更新基础信息 |
| `/v1/rider/application/idcard/ocr` | POST | **上传身份证** + 异步 OCR |
| `/v1/rider/application/healthcert` | POST | **上传健康证** + 异步 OCR |
| `/v1/rider/application/submit` | POST | 提交申请 |
| `/v1/rider/application/reset` | POST | 重置被拒绝的申请 |

### 2.2 证照上传接口详情

#### POST `/v1/rider/application/idcard/ocr`
**上传身份证并触发 OCR**

```
Content-Type: multipart/form-data

字段:
- image (file, 必填): 身份证图片文件
- side (string, 必填): "front" (正面) 或 "back" (背面)
```

---

#### POST `/v1/rider/application/healthcert`
**上传健康证并触发 OCR**

```
Content-Type: multipart/form-data

字段:
- image (file, 必填): 健康证图片文件
```

---

## 三、运营商入驻申请

### 3.1 接口列表

| 接口 | 方法 | 说明 |
|------|------|------|
| `/v1/operator/application` | POST | 创建申请草稿 |
| `/v1/operator/application` | GET | 获取申请状态 |
| `/v1/operator/application/region` | PUT | 更新申请区域 |
| `/v1/operator/application/basic` | PUT | 更新基础信息 |
| `/v1/operator/application/license/ocr` | POST | **上传营业执照** + 异步 OCR |
| `/v1/operator/application/idcard/ocr` | POST | **上传身份证** + 异步 OCR |
| `/v1/operator/application/submit` | POST | 提交申请 |
| `/v1/operator/application/reset` | POST | 重置申请 |

### 3.2 证照上传接口详情

#### POST `/v1/operator/application/license/ocr`
**上传营业执照并触发 OCR**

```
Content-Type: multipart/form-data

字段:
- image (file, 必填): 营业执照图片文件
```

---

#### POST `/v1/operator/application/idcard/ocr`
**上传身份证并触发 OCR**

```
Content-Type: multipart/form-data

字段:
- image (file, 必填): 身份证图片文件
- side (string, 必填): "Front" (正面) 或 "Back" (背面)
```

---

## 四、OCR 状态轮询

所有证照上传后，OCR 是异步执行的。前端需要轮询申请草稿接口来获取 OCR 结果。

### OCR 状态字段

```json
{
  "business_license_ocr": {
    "status": "pending" | "done" | "failed",
    "enterprise_name": "xxx公司",
    "credit_code": "91xxx",
    ...
  }
}
```

### 轮询策略

1. 上传成功后，立即获取返回的草稿对象
2. 如果 `xxx_ocr.status === "pending"`，等待 2-3 秒后调用 `GET /v1/{role}/application` 刷新
3. 重复直到 `status` 变为 `"done"` 或 `"failed"`
4. 建议最多轮询 10 次（约 30 秒），超时后提示用户手动填写

---

## 五、常见错误

| HTTP 状态码 | 错误信息 | 原因 |
|-------------|----------|------|
| 400 | 请先上传营业执照图片 | 未携带 `image` 字段或字段为空 |
| 400 | side参数必须是Front或Back | 身份证上传缺少 `side` 参数或值不正确 |
| 400 | 非草稿状态无法编辑 | 申请状态为 `submitted`，需等待审核或调用 reset |
| 404 | 请先创建申请 | 未调用 GET 接口创建草稿 |
| 502 | 微信图片安全检测服务异常 | 微信 API 调用失败（通常是网络问题） |

---

## 六、前端调用示例（小程序）

```javascript
// 选择图片并上传营业执照
wx.chooseImage({
  count: 1,
  success: (res) => {
    const filePath = res.tempFilePaths[0];
    
    wx.uploadFile({
      url: 'https://llapi.merrydance.cn/v1/merchant/application/license/ocr',
      filePath: filePath,
      name: 'image',  // 关键：字段名必须是 image
      header: {
        'Authorization': 'Bearer ' + token
      },
      success: (uploadRes) => {
        const data = JSON.parse(uploadRes.data);
        if (data.code === 0) {
          // 开始轮询 OCR 结果
          pollOCRResult();
        }
      }
    });
  }
});

// 上传身份证（需要额外的 side 参数）
wx.uploadFile({
  url: 'https://llapi.merrydance.cn/v1/merchant/application/idcard/ocr',
  filePath: filePath,
  name: 'image',
  formData: {
    side: 'Front'  // 或 'Back'
  },
  header: {
    'Authorization': 'Bearer ' + token
  },
  success: (uploadRes) => {
    // 处理响应
  }
});
```

---

## 七、门头照和环境照（非证照类）

门头照和环境照**不需要 OCR**，使用不同的接口：

1. **先上传图片**：调用 `POST /v1/merchants/images/upload`（需要 `category` 参数）
2. **保存 URL 数组**：调用 `PUT /v1/merchant/application/images`

```javascript
// 上传门头照/环境照
wx.uploadFile({
  url: 'https://llapi.merrydance.cn/v1/merchants/images/upload',
  filePath: filePath,
  name: 'image',
  formData: {
    category: 'storefront'  // 或 'environment'
  },
  header: { 'Authorization': 'Bearer ' + token },
  success: (res) => {
    const { image_url } = JSON.parse(res.data).data;
    // 将 image_url 加入数组，稍后统一保存
  }
});

// 保存图片 URL 数组
wx.request({
  url: 'https://llapi.merrydance.cn/v1/merchant/application/images',
  method: 'PUT',
  header: { 
    'Authorization': 'Bearer ' + token,
    'Content-Type': 'application/json'
  },
  data: {
    storefront_images: ['url1', 'url2'],
    environment_images: ['url3', 'url4', 'url5']
  }
});
```
