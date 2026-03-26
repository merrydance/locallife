# 阿里云 OCR RAM / STS 最小权限接入方案

## 1. 目的

本文固定统一 OCR 主链路在阿里云侧的最小权限接入原则，目标是完成 T33D：

- 不使用阿里云主账号长期 AccessKey
- 生产环境优先采用最小权限 RAM 方案
- 当运行环境支持 STS 时，保留切换到临时凭证模式的配置入口
- 明确当前代码实现边界，避免误以为 STS 已经可直接上线

## 2. 当前代码边界

截至 2026-03-25，项目内与阿里云 OCR 凭证相关的现状如下：

- 已支持配置项：
  - `ALIYUN_OCR_ENABLED`
  - `ALIYUN_OCR_ENDPOINT`
  - `ALIYUN_OCR_REGION`
  - `ALIYUN_OCR_ACCESS_KEY_ID`
  - `ALIYUN_OCR_ACCESS_KEY_SECRET`
  - `ALIYUN_OCR_STS_ENABLED`
  - `ALIYUN_OCR_ROLE_ARN`
  - `ALIYUN_OCR_ROLE_SESSION_NAME`
  - `ALIYUN_OCR_ROLE_EXTERNAL_ID`
- 已支持启动校验：
  - STS 模式下必须提供 `ROLE_ARN` 与 `ROLE_SESSION_NAME`
  - 非 STS 模式下必须提供 AK/SK
- 当前运行时代码仍未实现 STS 凭证获取 client
  - `ocr/provider_aliyun.go` 在 `ALIYUN_OCR_STS_ENABLED=true` 时会直接返回 `aliyun ocr sts mode is not implemented yet`

因此当前可执行落地方案固定为：

1. 生产环境立即禁用主账号长钥
2. 使用最小权限 RAM 用户 AK/SK 接入 OCR
3. 保留 STS 配置位，但在运行时实现补齐前，不在生产环境开启 `ALIYUN_OCR_STS_ENABLED=true`

## 3. 推荐接入顺序

### 3.1 当前生产可执行方案

当前推荐方案：

- 使用专用 RAM 用户，仅服务于 Locallife OCR
- 该 RAM 用户只绑定 OCR 所需最小权限策略
- AK/SK 只注入服务端 API / worker 部署环境
- 不与 OSS、短信、其他 AI 产品共享同一凭证

生产要求：

- 禁止使用主账号 AK/SK
- 禁止复用已有高权限通用 AK/SK
- 禁止把 OCR 凭证下发到任何客户端或前端配置

### 3.2 后续升级方案

当代码补齐 STS client 后，再升级为：

- 计算节点实例角色
- OIDC / K8s ServiceAccount 绑定角色
- 或受控的 STS AssumeRole

升级后目标：

- OCR 不再依赖长期 AK/SK
- 凭证自动轮转
- 角色信任链可审计

## 4. 最小权限原则

RAM 策略必须满足以下限制：

- 仅允许 OCR OpenAPI 调用所需动作
- 仅允许指定地域的 OCR 访问
- 不授予 OSS Bucket 管理、RAM 管理、ECS 管理等无关权限
- 不授予通配符管理员权限

建议拆分为两个层次：

1. 基础 OCR 调用策略
2. 凭证管理与轮换流程

其中基础 OCR 调用策略只负责“调用 OCR API”，不负责其他云资源访问。

## 5. 推荐 RAM 策略模板

以下策略模板用于表达“最小权限方向”，上线前应由云平台管理员按实际阿里云 OCR API Action 名称复核：

```json
{
  "Version": "1",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ocr:RecognizeBusinessLicense",
        "ocr:RecognizeFoodManageLicense",
        "ocr:RecognizeIdentityCard",
        "ocr:RecognizeGeneral"
      ],
      "Resource": "*",
      "Condition": {
        "StringEquals": {
          "acs:Region": [
            "cn-hangzhou"
          ]
        }
      }
    }
  ]
}
```

使用说明：

- 若阿里云 OCR OpenAPI 的实际 Action 名称与上例不同，必须按控制台或 OpenAPI Explorer 的真实名称替换
- 若无法按 Action 粒度收窄，也必须至少限制为 OCR 产品范围，不得放宽到 `*`
- 若地域条件对实际 OCR API 不生效，则保留产品最小动作范围，并在部署文档中明确 endpoint 与 region 固定值

## 6. 环境配置要求

当前生产建议配置如下：

```env
ALIYUN_OCR_ENABLED=true
ALIYUN_OCR_ENDPOINT=https://ocr-api.cn-hangzhou.aliyuncs.com
ALIYUN_OCR_REGION=cn-hangzhou
ALIYUN_OCR_ACCESS_KEY_ID=<ram_user_ak>
ALIYUN_OCR_ACCESS_KEY_SECRET=<ram_user_sk>
ALIYUN_OCR_STS_ENABLED=false
ALIYUN_OCR_ROLE_ARN=
ALIYUN_OCR_ROLE_SESSION_NAME=locallife-ocr
ALIYUN_OCR_ROLE_EXTERNAL_ID=
```

约束：

- `ALIYUN_OCR_ACCESS_KEY_ID` / `ALIYUN_OCR_ACCESS_KEY_SECRET` 必须来自专用 RAM 用户
- `ALIYUN_OCR_STS_ENABLED` 在当前版本必须保持 `false`
- 若后续补齐 STS 运行时实现，再切换该开关

## 7. 凭证轮换与审计

建议最少执行以下流程：

1. 为 OCR 专用 RAM 用户设置独立名称与标签
2. 记录当前 AK 创建时间、轮换责任人、用途说明
3. 至少按季度轮换一次 AK/SK
4. 轮换前先在测试环境验证新凭证
5. 轮换后观察：
   - `ocr_provider_unauthorized`
   - `ocr_provider_forbidden`
   - `OCR_JOB_FAILED`
6. 保留旧凭证短暂并行窗口后立即删除

## 8. 上线检查清单

发布前必须确认：

1. 当前 OCR 凭证不是主账号 AK/SK
2. 当前 OCR 凭证不是共享高权限通用 AK/SK
3. RAM 策略只覆盖 OCR 所需动作
4. `ALIYUN_OCR_STS_ENABLED=false` 与当前运行时实现一致
5. 运维已知晓 STS 仍是后续增强项，而不是当前可切换项

## 9. 结论

T33D 的当前完成定义如下：

- 已明确并文档化“主账号长钥禁用”原则
- 已明确当前版本的可执行最小权限落地方式为“专用 RAM 用户 AK/SK”
- 已保留 STS 配置入口，但同时明确其尚未具备运行时实现，不允许误开

后续若进入真正的 STS 实现阶段，应新增独立任务，而不是继续把它混在 T33D 中。