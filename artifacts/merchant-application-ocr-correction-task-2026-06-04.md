# 商户入驻 OCR 识别字段更正能力任务文档

日期：2026-06-04
风险等级：G3

## 背景

骑手健康证 OCR 已支持用户在入驻流程中更正识别错误，并将更正后的字段写回 OCR JSON，供后续提交审核使用。商户入驻当前同样依赖 OCR 结果完成证照审核，但营业执照、食品经营许可证、法人身份证的识别字段更多，误识别后对商户提交成功率和客服介入成本影响更大。

本任务目标是给商户入驻增加同类能力，但第一期只覆盖营业执照和食品经营许可证，不开放法人身份证字段更正。

## 现状审核结论

现有代码没有完整实现“商户可修改 OCR 错误并让审核使用修正值”的能力。

证据：

- 后端 OCR DTO 中有营业执照、食品经营许可证、法人身份证 OCR 字段，但没有 `correction` 审计元数据字段，也没有商户 OCR 更正接口。见 `locallife/api/merchant_application.go:55`、`locallife/api/merchant_application.go:77`、`locallife/api/merchant_application.go:96`。
- 后端基础信息接口可以保存 `business_license_number`、`business_scope`、`legal_person_name`、`legal_person_id_number` 等普通列。见 `locallife/api/merchant_application.go:398` 和 `locallife/db/query/merchant_application.sql:28`。但这不是 OCR 更正能力，因为审核输入仍从 OCR JSON 构建。
- 商户提交审核调用 `BuildMerchantDocumentReviewInputFromPayloads`，输入来自 `app.BusinessLicenseOcr`、`app.FoodPermitOcr`、`app.IDCardFrontOcr`、`app.IDCardBackOcr`。见 `locallife/api/merchant_application.go:1050`。
- 审核逻辑读取 OCR JSON 内的 `enterprise_name`、`legal_representative`、`address`、`business_scope`、`valid_period`、`company_name`、`valid_to`、身份证姓名/号/有效期等字段，并据此判定是否通过。见 `locallife/logic/merchant_document_review.go:54`。
- 小程序商户入驻 Step 2 当前用只读 `t-cell` 展示 OCR 字段，没有可编辑输入控件。见 `weapp/miniprogram/pages/register/merchant/store/index.wxml:58`。
- 小程序 `syncToBackend` 只提交商户名、联系电话、地址、经纬度、区域；不会提交统一信用代码、营业期限、经营范围、食品经营许可证有效期等 OCR 修正字段。见 `weapp/miniprogram/pages/register/merchant/store/_utils/merchant-store-registration-runtime.ts:384`。
- 小程序 Step 2/提交前只校验本地 `formData.creditCode`、`formData.legalPerson`、`formData.idCard` 是否存在；即使用户能在本地改值，也没有保存到后端 OCR JSON 的路径。见 `weapp/miniprogram/pages/register/merchant/store/_utils/merchant-store-registration-runtime.ts:688` 和 `weapp/miniprogram/pages/register/merchant/store/_utils/merchant-store-registration-runtime.ts:1268`。

## 设计目标

1. 商户在入驻草稿状态下，可以更正营业执照和食品经营许可证的 OCR 识别字段。
2. 更正必须写回后端对应 OCR JSON，提交审核时必须使用更正后的字段。
3. 更正必须保留审计元数据，能追踪更正人、更正时间、更正来源、更正字段和更正前值。
4. 不影响正常功能：原有上传、OCR 自动回填、删除证照、保存基础信息、提交审核、自动修复食品许可证字段、官方核验逻辑必须继续工作。
5. 不开放法人身份证 OCR 字段更正，避免身份证姓名/号码被用户直接篡改后绕过身份风控。

## 一期范围

### 后端接口

新增商户 OCR 字段更正接口：

`PATCH /v1/merchant/application/documents/:document_type/ocr-fields`

`document_type` 只允许：

- `business_license`
- `food_permit`

不允许：

- `id_card_front`
- `id_card_back`
- 其他任意值

请求必须绑定当前登录用户自己的商户申请，不接受客户端传入 owner/user/application id。

仅允许以下申请状态：

- `draft`

对 `submitted` 状态必须拒绝。

对 `approved` / `rejected` 状态，本任务第一期不自动 reset，避免“修改 OCR 字段”隐式改变已审核申请。若后续需要支持驳回后修正，应另行设计“重置为草稿后修改”的显式流程。

### 营业执照可更正字段

请求字段：

- `enterprise_name`
- `credit_code`
- `reg_num`
- `legal_representative`
- `address`
- `business_scope`
- `valid_period`

写回规则：

- 写入 `business_license_ocr` JSON 对应字段。
- `credit_code` 和 `reg_num` 至少一个非空；若两者都传，以 `credit_code` 作为统一社会信用代码主展示值，`reg_num` 用于兼容旧 OCR 结果。
- 同步更新普通列：
  - `business_license_number` = `credit_code` 优先，否则 `reg_num`
  - `business_scope` = `business_scope`
  - `merchant_name` 仅在当前为空时可由 `enterprise_name` 回填；不得覆盖商户已经手填的店名。
  - `business_address` 不由更正接口强制覆盖，避免覆盖用户地图选点/经营地址；前端可继续让商户在位置步骤确认地址。
- 更正后重算 `readiness`，必需字段为 `enterprise_name`、`legal_representative`、`address`、`business_scope`、`valid_period`，以及统一信用代码字段 `credit_code|reg_num`。
- 清空旧的 `error`、`error_code`、`alert_emitted_at`。
- 保留 `status`、`ocr_job_id`、`queued_at`、`started_at`、`ocr_at`、原始非更正字段。

### 食品经营许可证可更正字段

请求字段：

- `permit_no`
- `company_name`
- `operator_name`
- `valid_from`
- `valid_to`

写回规则：

- 写入 `food_permit_ocr` JSON 对应字段。
- 不修改营业执照普通列。
- 更正后重算 `readiness`，必需字段为 `company_name`、`valid_to`；`permit_no` 和 `operator_name` 作为辅助核验字段保留但不作为第一期提交阻断必需字段。
- `valid_to` 必须可被现有商户审核日期解析逻辑识别，并且至少超过当前日期 30 天；否则接口返回 400。
- 清空旧的 `error`、`error_code`、`alert_emitted_at`。
- 保留 `raw_text`、`status`、`ocr_job_id`、`queued_at`、`started_at`、`ocr_at`、原始非更正字段。

### 审计元数据

在 `business_license_ocr` 和 `food_permit_ocr` JSON 内新增：

```json
{
  "correction": {
    "corrected_by": 123,
    "corrected_at": "2026-06-04T22:30:00+08:00",
    "source": "merchant",
    "fields": ["valid_period", "business_scope"],
    "previous": {
      "valid_period": "",
      "business_scope": "..."
    }
  }
}
```

同一次请求若字段值与当前 OCR JSON 完全一致，应返回当前申请，不写库，也不覆盖已有 `correction` 元数据。

## 前端任务

目标页面：

`weapp/miniprogram/pages/register/merchant/store/index.wxml`

目标运行时：

`weapp/miniprogram/pages/register/merchant/store/_utils/merchant-store-registration-runtime.ts`

要求：

1. Step 2 的营业执照、食品经营许可证识别结果从只读展示改为可编辑输入。
2. 法人身份证信息保持只读，不提供身份证姓名、身份证号、身份证有效期编辑入口。
3. 在 Step 2 点击下一步前，先保存营业执照/食品经营许可证 OCR 更正；保存失败时停留在 Step 2 并展示后端业务错误。
4. 在最终提交前，再保存一次更正，防止用户返回 Step 2 修改后未持久化。
5. 保存成功后用后端返回的最新申请重新回填页面状态。
6. 不改变原有上传流程和 OCR job 轮询/回填流程；OCR 完成后仍然自动回填，用户只是在回填结果基础上更正。

## 非目标

本任务不做以下事项：

- 不开放法人身份证 OCR 字段更正。
- 不改变商户申请状态机。
- 不改变商户审核规则本身。
- 不取消食品经营许可证官方核验。
- 不弱化营业执照重复校验、身份证重复校验、地址重复校验。
- 不引入人工审核后台页面。
- 不做数据库 schema migration；审计元数据放入现有 OCR JSON。
- 不把更正能力扩展到团体商户、运营商申请、平台后台或 Flutter merchant app。

## 正常功能不受影响的保护要求

实现必须满足：

1. 未修改 OCR 字段时，现有上传、OCR 回填、提交审核行为不变。
2. 只上传证照但不进入 Step 2 修改字段时，原有 OCR JSON 仍按当前逻辑进入审核。
3. 删除营业执照或食品经营许可证时，对应 OCR JSON 和 correction 元数据一起清空。
4. OCR 处于 `pending` / `processing` 时，接口拒绝更正并提示“OCR处理中”；不得把未完成 OCR 强行标记为可审核。
5. OCR 处于 `failed` 时，接口拒绝更正并提示重新上传；不得允许用户仅靠手填绕过失败证照。
6. 更正后提交仍必须经过现有 `EvaluateMerchantDocumentReview`、食品许可证官方核验、重复证照/身份证/地址校验。

## 后端验证要求

新增或更新测试至少覆盖：

1. 营业执照更正成功：写回 OCR JSON、更新 `business_license_number`/`business_scope`、写入 correction、重算 readiness。
2. 食品经营许可证更正成功：写回 OCR JSON、写入 correction、重算 readiness。
3. 无变化请求不写库、不覆盖 correction。
4. 非 `business_license` / `food_permit` 的 document_type 返回 400。
5. `submitted` 状态返回 400。
6. 缺少证照媒体或 OCR JSON 返回 400。
7. OCR `pending` / `processing` / `failed` 返回 400。
8. 食品许可证 `valid_to` 不可解析或不足 30 天返回 400。
9. 提交审核使用更正后的营业执照/食品许可证字段。

后端命令：

- `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestPatchMerchantApplication.*OCRFields|TestCheckMerchantApplicationApproval|TestSubmitMerchantApplication' -count=1`
- `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestMerchantDocumentReview|TestBuildMerchantDocumentReviewInputFromPayloads' -count=1`
- 若新增路由或 Swagger 注释：`PATH="/usr/local/go/bin:$PATH" make swagger`
- 若修改 SQL/sqlc：`PATH="/usr/local/go/bin:$PATH" make sqlc` 和 `PATH="/usr/local/go/bin:$PATH" make check-generated`

## 小程序验证要求

新增一个轻量守护脚本，类似骑手健康证更正守护脚本，验证：

1. 商户 API 层暴露 OCR 更正方法。
2. 页面 Step 2 的营业执照/食品许可证字段是可编辑输入，不再是只读 `t-cell`。
3. Step 2 下一步和最终提交前都会 `await` 保存 OCR 更正。
4. 身份证字段仍保持只读。

小程序命令：

- `npm run lint`
- `npm run compile`
- `node scripts/check-merchant-ocr-correction.test.js`

## 验收标准

1. 商户上传证照后，Step 2 能看到 OCR 回填值，并可更正营业执照和食品经营许可证字段。
2. 更正后的字段刷新页面后仍存在，说明已经落库。
3. 更正后的字段能通过提交审核链路被 `BuildMerchantDocumentReviewInputFromPayloads` 和 `EvaluateMerchantDocumentReview` 使用。
4. 身份证 OCR 字段没有任何可编辑入口和后端更正接口。
5. 正常未更正路径不回归：现有 OCR 上传、自动回填、删除证照、提交审核仍可运行。
6. Swagger、前端类型、后端 DTO 与实际接口一致。

## 实施备注

- 优先复用骑手健康证更正接口的模式，但不要共用会模糊证照字段含义的泛型 DTO。
- 商户申请相关后端文件较大，实现时应拆分到新的 `merchant_application_ocr_correction.go`，避免继续扩大 `merchant_application.go`。
- 日期解析和 readiness 重算应放在可测试的 helper 中，不应散落在 handler 中。
- 不要用前端本地表单值直接决定审核通过；后端持久化 OCR JSON 是唯一提交审核事实来源。
