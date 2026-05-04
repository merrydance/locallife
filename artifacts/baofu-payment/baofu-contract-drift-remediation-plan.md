# 宝付宝财通契约漂移修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `baofu-api-contract-coverage-audit.md` 中 C-001 到 C-012 的契约漂移风险修复到可进入宝付测试地址联调的 C3 基线，并为 C4 沙箱证据留出固定记录位置。

**Architecture:** 以 `locallife/baofu` 作为唯一宝付协议边界，先把 endpoint profile、公共 envelope、官方字段级 DTO、附录枚举、错误码、金额单位和通知 ACK 固化成可测试契约，再接真实 HTTP transport 和业务 readiness。主业务商户固定采用宝付开户后逐户 `merchant_report`，再 `bind_sub_config(authType=APPLET, authContent=<LocalLife 小程序 appid>)` 的异主体授权目录绑定流程；`share_after_pay` 只读取宝付二级商户号 `sharing_mer_id`。

**Tech Stack:** Go, PostgreSQL migrations, sqlc, Gin callbacks, asynq workers, existing `external_payment_commands` / `external_payment_facts` / outbox recovery model, Baofu union-gw, Baofu aggregate pay, Baofu merchant report.

---

## 0. Current Confirmed Business Rules

- 宝付支持异主体报备；不再保留项目内微信支付特约商户进件作为宝付主链路前置条件。
- 主业务商户宝付开户成功后必须逐户做 `merchant_report` 取得商户微信渠道 `subMchId`。
- 每个商户 `subMchId` 必须调用 `bind_sub_config`，`authType=APPLET`，`authContent=<LocalLife 小程序 appid>`。
- `unified_order.subMchId` 读取商户聚合商户报备成功后的 `sub_mch_id`；不读取普通服务商 `txResult.SubMchID`。
- `share_after_pay` 不传 `subMchId`，分账接收方只读 `sharingDetails[].sharingMerId = baofu_account_bindings.sharing_mer_id`。
- 不做降级：宝付配置、报备、授权目录、HTTP client 任一缺失时，主业务支付 fail-closed，不回退微信普通服务商/平台收付通。
- 支付手续费 0.3% 由商户承担；开户验证费 1 元由平台承担。
- 宝付替换边界只覆盖主业务普通服务商/平台收付通能力；微信直连支付能力保持 `direct`。

## 1. Findings To Task Mapping

| Finding | 修复任务 | 完成标准 |
| --- | --- | --- |
| C-001 endpoint profile mismatch | Task 1 | 三组官方 endpoint 拆分配置，默认不再指向 `https://api.baofoo.com`。 |
| C-002 account DTO drift | Task 3 | 开户/查询/余额/提现使用官方字段级 DTO 和条件必填测试。 |
| C-003 account notification drift | Task 4 | 开户/提现通知按官方字段解析，ACK 固定纯文本 `OK`。 |
| C-004 aggregate public envelope missing | Task 2 | 聚合支付/报备公共 envelope 和 `bizContent` 分层，签名字段完整。 |
| C-005 unified_order validation incomplete | Task 7 | `unified_order` M/C 字段、长度、枚举、金额关系、`riskInfo` 全部校验。 |
| C-006 merchantreport missing | Task 5 | `merchant_report`、`merchant_report_query`、`bind_sub_config` 契约/表/服务完成。 |
| C-007 subMchId source wrong | Task 6 | 支付 readiness 和下单只读取商户报备 `sub_mch_id` + APPLET 授权状态。 |
| C-008 refund/close missing | Task 9 | 分账前退款、退款查询、退款通知、关单契约和互斥完成。 |
| C-009 amount unit drift | Task 3, Task 8 | 账户金额元/分转换集中在契约边界并有舍入测试。 |
| C-010 error/status classification missing | Task 10 | 账户/报备/支付错误码分类，前端语义安全。 |
| C-011 real transport missing | Task 8 | union-gw、aggregate pay、merchant report 真实 HTTP client 可打测试地址。 |
| C-012 readiness copy drift | Task 6, Task 11 | 旧“商户微信渠道待报备”文案替换为产品语义。 |

## 2. Target File Map

### 2.1 Baofu Common Boundary

- Modify: `locallife/baofu/config.go` - endpoint profile、收单/代付商户号边界、notify URL 校验。
- Modify: `locallife/baofu/config_test.go` - endpoint profile fail-closed 测试。
- Modify: `locallife/baofu/client.go` - concrete clients composition。
- Modify: `locallife/baofu/transport.go` - HTTP request metadata、timeout、redacted logging。
- Modify: `locallife/baofu/signing.go` - aggregate public envelope signing/verification helpers。
- Create: `locallife/baofu/envelope.go` - aggregate/merchant-report public envelope DTO。
- Create: `locallife/baofu/envelope_test.go` - public envelope required fields and canonical serialization tests。

### 2.2 Account API Boundary

- Modify: `locallife/baofu/account/contracts/types.go` - keep normalized project result types only when still useful。
- Create: `locallife/baofu/account/contracts/official_open.go` - `T-1001-013-01` official open account request/response DTO。
- Create: `locallife/baofu/account/contracts/official_query.go` - `T-1001-013-03` official query DTO。
- Create: `locallife/baofu/account/contracts/official_balance.go` - `T-1001-013-06` official balance DTO and yuan/fen conversion。
- Create: `locallife/baofu/account/contracts/official_withdraw.go` - `T-1001-013-14/15` official withdraw DTO and yuan/fen conversion。
- Modify: `locallife/baofu/account/contracts/types_test.go` - official field/conditional-required tests。
- Modify: `locallife/baofu/account/notification/notification.go` - official open/withdraw notification parser and ACK。
- Modify: `locallife/baofu/account/notification/notification_test.go` - official notification fixture tests。
- Modify: `locallife/baofu/account/client.go` - concrete union-gw client methods。

### 2.3 Merchant Report Boundary

- Create: `locallife/baofu/merchantreport/contracts/types.go` - `merchant_report` / query / `bind_sub_config` DTO。
- Create: `locallife/baofu/merchantreport/contracts/enums.go` - appendix enums。
- Create: `locallife/baofu/merchantreport/contracts/categories_generated.go` - generated WeChat category allowlist。
- Create: `locallife/baofu/merchantreport/contracts/types_test.go` - DTO validation tests。
- Create: `locallife/baofu/merchantreport/contracts/categories_test.go` - xlsx hash/row count/illegal value tests。
- Create: `locallife/baofu/merchantreport/client.go` - concrete merchant report HTTP client。
- Create: `locallife/db/migration/000229_add_baofu_merchant_reports.up.sql` - report/auth table。
- Create: `locallife/db/migration/000229_add_baofu_merchant_reports.down.sql` - down migration。
- Create: `locallife/db/query/baofu_merchant_report.sql` - sqlc report/auth lifecycle queries。
- Create: `locallife/logic/baofu_merchant_report_service.go` - report/auth orchestration。
- Create: `locallife/logic/baofu_merchant_report_service_test.go` - command-before-client, state transition, readiness tests。

### 2.4 Aggregate Pay Boundary

- Modify: `locallife/baofu/aggregatepay/contracts/types.go` - official `unified_order`, query, share, refund, close DTOs and validation。
- Modify: `locallife/baofu/aggregatepay/contracts/types_test.go` - field matrix tests。
- Modify: `locallife/baofu/aggregatepay/notification/notification.go` - official payment/share/refund notification parser and ACK。
- Modify: `locallife/baofu/aggregatepay/notification/notification_test.go` - notification fixture tests。
- Modify: `locallife/baofu/aggregatepay/client.go` - concrete HTTP client for payment/share/refund/close/query。
- Modify: `locallife/logic/baofu_payment_service.go` - merchant report `sub_mch_id` source and no ordinary-service-provider fallback。
- Modify: `locallife/logic/baofu_payment_order_route.go` - remove `txResult.SubMchID` from Baofu request source。
- Modify: `locallife/logic/baofu_payment_readiness.go` - semantic readiness copy and merchant report/auth checks。
- Modify: `locallife/api/logic_adapters.go` - runtime wiring to concrete Baofu clients when configured。

### 2.5 Documentation And Sandbox Evidence

- Modify: `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md` - update C0/C1/C2/C3/C4 as tasks land。
- Modify: `artifacts/baofu-payment/baofu-profit-sharing-implementation-plan.md` - mark task progress and link this remediation plan。
- Create: `artifacts/baofu-payment/baofu-sandbox-evidence.md` - records request/response/callback/query evidence for C4。

## 2.6 Test Helper Contract

Plan snippets use helper names such as `validBaofuConfigForTest`, `validPublicEnvelopeForTest`, `newMockStoreWithoutSharingMerID`, `fakeMerchantReportClient`, `activeBaofuBindingWithSharingMerID`, `succeededMerchantReportWithoutAppletAuth`, `newTestServerWithBaofuMainBusiness`, and `postCreateMainBusinessPayment`. When implementing a task, define the helper in the same `_test.go` file or reuse an existing helper with the same behavior. Each helper must build only safe fake data; do not include real merchant号、身份证、银行卡、手机号、证书、密钥或真实宝付响应原文。

Required helper behavior:

| Helper | Required behavior |
| --- | --- |
| `validBaofuConfigForTest` | Returns a complete sandbox config with official endpoint URLs, separated collect/payout merchant IDs, fake test keys, and https notify URL. |
| `validPublicEnvelopeForTest` | Returns a syntactically complete public envelope with `method`, fixed `UTF-8/1.0/json`, timestamp, sign serials, sign string, and JSON `bizContent`. |
| `newMockStoreWithoutSharingMerID` | Returns a store mock whose merchant Baofu binding is active but has empty `sharing_mer_id`, so report creation must fail before client call. |
| `fakeMerchantReportClient` | Captures whether `merchant_report`, query, and `bind_sub_config` were called; returns deterministic sanitized results. |
| `activeBaofuBindingWithSharingMerID` | Returns an active merchant binding with non-empty `sharing_mer_id` and no raw PII. |
| `succeededMerchantReportWithoutAppletAuth` | Returns a succeeded report with `sub_mch_id` but `applet_auth_state=pending`, so payment readiness remains false. |
| `validWechatUnifiedOrderRequestForTest` | Returns a complete `WECHAT_JSAPI` + `SHARING` + `orderType=7` request with merchant `subMchId`, `sub_appid`, `sub_openid`, body, amount, time, notify URL, and `riskInfo.clientIp`. |
| `validBaofuRefundBeforeShareRequestForTest` | Returns a refund request that references the original payment and intentionally has no post-share refund fields. |
| `newTestServerWithBaofuMainBusiness` | Builds an API test server where main-business payment is configured to use Baofu and direct-payment clients remain separately injectable. |
| `postCreateMainBusinessPayment` | Sends the existing API request shape for a main-business payment and returns the recorder/response wrapper used by adjacent payment API tests. |

## 3. Task Cards

### Task 1: Endpoint Profiles And Config Fail-Closed

**Files:**
- Modify: `locallife/baofu/config.go`
- Modify: `locallife/baofu/config_test.go`
- Modify: `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`

- [x] **Step 1: Write endpoint profile tests**

Add tests in `locallife/baofu/config_test.go`:

```go
func TestConfigValidateRequiresOfficialEndpointProfiles(t *testing.T) {
    cfg := validBaofuConfigForTest()
    cfg.AccountGatewayBaseURL = ""
    require.EqualError(t, cfg.Validate(), "baofu account gateway base url is required")

    cfg = validBaofuConfigForTest()
    cfg.AggregatePayBaseURL = "https://api.baofoo.com"
    require.EqualError(t, cfg.Validate(), "baofu aggregate pay base url must be an official endpoint")

    cfg = validBaofuConfigForTest()
    cfg.MerchantReportBaseURL = "https://api.baofoo.com"
    require.EqualError(t, cfg.Validate(), "baofu merchant report base url must be an official endpoint")
}

func TestConfigNormalizedUsesSandboxEndpointProfile(t *testing.T) {
    cfg := validBaofuConfigForTest()
    cfg.Environment = BaofuEnvironmentSandbox
    normalized := cfg.Normalized()
    require.Equal(t, "https://vgw.baofoo.com/union-gw/api", normalized.AccountGatewayBaseURL)
    require.Equal(t, "https://mch-juhe.baofoo.com/api", normalized.AggregatePayBaseURL)
    require.Equal(t, "https://mch-juhe.baofoo.com/mch-service/api", normalized.MerchantReportBaseURL)
}
```

- [x] **Step 2: Run tests and confirm failure**

Run from `locallife/`:

```bash
go test ./baofu -run 'TestConfigValidateRequiresOfficialEndpointProfiles|TestConfigNormalizedUsesSandboxEndpointProfile' -count=1
```

Expected before implementation: compile failure or assertion failure because endpoint profile fields/constants do not exist.

- [x] **Step 3: Implement config fields and official allowlist**

In `locallife/baofu/config.go`, add:

```go
const (
    BaofuEnvironmentSandbox = "sandbox"
    BaofuEnvironmentProduction = "production"

    SandboxAccountGatewayBaseURL = "https://vgw.baofoo.com/union-gw/api"
    ProductionAccountGatewayBaseURL = "https://public.baofu.com/union-gw/api"
    SandboxAggregatePayBaseURL = "https://mch-juhe.baofoo.com/api"
    ProductionAggregatePayBaseURL = "https://juhe.baofoo.com/api"
    ProductionAggregatePayBackupBaseURL = "https://juhe-backup.baofoo.com/api"
    SandboxMerchantReportBaseURL = "https://mch-juhe.baofoo.com/mch-service/api"
    ProductionMerchantReportBaseURL = "https://juhe.baofoo.com/mch-service/api"
)

type Config struct {
    Environment string
    AccountGatewayBaseURL string
    AggregatePayBaseURL string
    AggregatePayBackupBaseURL string
    MerchantReportBaseURL string
    CollectMerchantID string
    CollectTerminalID string
    PayoutMerchantID string
    PayoutTerminalID string
    AppID string
    PrivateKeyPEM string
    BaofuPublicKeyPEM string
    NotifyBaseURL string
    Timeout time.Duration
}
```

Keep existing fields that callers still need, but stop defaulting to `https://api.baofoo.com`.

- [x] **Step 4: Run config tests**

```bash
go test ./baofu -run 'TestConfig' -count=1
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add locallife/baofu/config.go locallife/baofu/config_test.go artifacts/baofu-payment/baofu-api-contract-coverage-audit.md
git commit -m "feat(baofu): split official endpoint profiles"
```

### Task 2: Public Envelope For Aggregate Pay And Merchant Report

**Files:**
- Create: `locallife/baofu/envelope.go`
- Create: `locallife/baofu/envelope_test.go`
- Modify: `locallife/baofu/signing.go`
- Modify: `locallife/baofu/signing_test.go`

- [x] **Step 1: Add failing envelope tests**

Create `locallife/baofu/envelope_test.go`:

```go
func TestPublicEnvelopeValidateRequiresOfficialFields(t *testing.T) {
    env := PublicRequestEnvelope{
        MerchantID: "100000",
        TerminalID: "200000",
        Method: "unified_order",
        Charset: "UTF-8",
        Version: "1.0",
        Format: "json",
        Timestamp: "20260504120000",
        SignType: SignTypeSM2,
        SignSerialNo: "1",
        EncryptionSerialNo: "1",
        SignString: "abcd",
        BizContent: json.RawMessage(`{"outTradeNo":"BF1"}`),
    }
    require.NoError(t, env.Validate())

    env.Method = ""
    require.EqualError(t, env.Validate(), "baofu public envelope method is required")
}

func TestPublicEnvelopeRejectsInvalidFixedValues(t *testing.T) {
    env := validPublicEnvelopeForTest()
    env.Charset = "GBK"
    require.EqualError(t, env.Validate(), "baofu public envelope charset must be UTF-8")

    env = validPublicEnvelopeForTest()
    env.Format = "xml"
    require.EqualError(t, env.Validate(), "baofu public envelope format must be json")
}
```

- [x] **Step 2: Run tests and confirm failure**

```bash
go test ./baofu -run 'TestPublicEnvelope' -count=1
```

Expected: compile failure because `PublicRequestEnvelope` does not exist.

- [x] **Step 3: Implement envelope DTO**

Create `locallife/baofu/envelope.go` with:

```go
type PublicRequestEnvelope struct {
    MerchantID string `json:"merId"`
    TerminalID string `json:"terId"`
    Method string `json:"method"`
    Charset string `json:"charset"`
    Version string `json:"version"`
    Format string `json:"format"`
    Timestamp string `json:"timestamp"`
    SignType string `json:"signType"`
    SignSerialNo string `json:"signSn"`
    EncryptionSerialNo string `json:"ncrptnSn"`
    DigitalEnvelope string `json:"dgtlEnvlp,omitempty"`
    SignString string `json:"signStr"`
    BizContent json.RawMessage `json:"bizContent"`
}
```

`Validate()` must reject missing `merId/terId/method/timestamp/signType/signSn/ncrptnSn/signStr/bizContent`, non-`UTF-8`, non-`1.0`, non-`json`, and invalid sign type.

- [x] **Step 4: Run envelope/signing tests**

```bash
go test ./baofu -run 'TestPublicEnvelope|TestSigning' -count=1
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add locallife/baofu/envelope.go locallife/baofu/envelope_test.go locallife/baofu/signing.go locallife/baofu/signing_test.go
git commit -m "feat(baofu): add public request envelope"
```

### Task 3: Official Account Contracts And Amount Unit Conversion

**Files:**
- Create: `locallife/baofu/account/contracts/official_open.go`
- Create: `locallife/baofu/account/contracts/official_query.go`
- Create: `locallife/baofu/account/contracts/official_balance.go`
- Create: `locallife/baofu/account/contracts/official_withdraw.go`
- Modify: `locallife/baofu/account/contracts/types_test.go`

- [x] **Step 1: Add official DTO tests**

Add tests in `locallife/baofu/account/contracts/types_test.go`:

```go
func TestOfficialOpenAccountRequestRequiresBCT20Fields(t *testing.T) {
    req := OfficialOpenAccountRequest{
        Version: "4.1.0",
        AccountType: OfficialAccountTypePersonal,
        NoticeURL: "https://api.example.com/v1/webhooks/baofu/account/open",
        BusinessType: OfficialBusinessTypeBCT20,
        AccountInfo: OfficialPersonalAccountInfo{
            TransSerialNo: "OPEN202605040001",
            LoginNo: "rider13800138000",
            CustomerName: "张三",
            CertificateType: OfficialCertificateTypeID,
            CertificateNo: "110101199001011234",
            CardNo: "6222020000000000000",
            MobileNo: "13800138000",
            CardUserName: "张三",
            NeedUploadFile: false,
        },
    }
    require.NoError(t, req.Validate())

    req.BusinessType = ""
    require.EqualError(t, req.Validate(), "baofu open account businessType must be BCT2.0")
}

func TestOfficialBalanceAmountConvertsYuanToFen(t *testing.T) {
    got, err := YuanStringToFen("123.45")
    require.NoError(t, err)
    require.Equal(t, int64(12345), got)

    _, err = YuanStringToFen("123.456")
    require.EqualError(t, err, "baofu amount supports at most 2 decimal places")
}
```

- [x] **Step 2: Run tests and confirm failure**

```bash
go test ./baofu/account/contracts -run 'TestOfficial|Test.*Yuan' -count=1
```

Expected: compile failure because official DTOs and conversion helpers do not exist.

- [x] **Step 3: Implement official DTOs**

Implement official request/response structs with JSON tags matching Baofu docs:

```go
const (
    OfficialBusinessTypeBCT20 = "BCT2.0"
    OfficialOpenAccountVersion = "4.1.0"
    OfficialWithdrawVersion = "4.2.0"
    OfficialAccountTypePersonal = 1
    OfficialAccountTypeBusiness = 2
    OfficialCertificateTypeID = "ID"
)
```

`OfficialOpenAccountRequest.Validate()` must distinguish personal two-factor, personal four-factor, enterprise, and self-employed required fields. Do not write `wechat_sub_mch_id` into account open DTO.

- [x] **Step 4: Run account contract tests**

```bash
go test ./baofu/account/contracts -count=1
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add locallife/baofu/account/contracts
git commit -m "feat(baofu): add official account contracts"
```

### Task 4: Official Account And Withdrawal Notifications

**Files:**
- Modify: `locallife/baofu/account/notification/notification.go`
- Modify: `locallife/baofu/account/notification/notification_test.go`
- Modify: `locallife/api/baofu_callback.go`
- Test: `locallife/api/baofu_callback_test.go`

- [x] **Step 1: Add official notification fixture tests**

Add tests that decrypt/parse a plaintext fixture equivalent to official fields:

```go
func TestParseOpenAccountNotificationUsesOfficialFields(t *testing.T) {
    raw := []byte(`{"member_id":"100000","terminal_id":"200000","memberType":"2","state":"1","errorCode":"","errorMsg":"","transSerialNo":"OPEN202605040001","loginNo":"merchant-login-001","customerName":"商户A","contractNo":"CM202605040001","noticeType":"OPEN_ACC"}`)
    got, err := ParseOpenAccountPlaintext(raw)
    require.NoError(t, err)
    require.Equal(t, "OPEN202605040001", got.OutRequestNo)
    require.Equal(t, "CM202605040001", got.ContractNo)
    require.Equal(t, contracts.OpenStateActive, got.OpenState)
}

func TestAccountNotificationACKIsPlainOK(t *testing.T) {
    require.Equal(t, "OK", AccountNotificationACK())
}
```

- [x] **Step 2: Run tests and confirm failure**

```bash
go test ./baofu/account/notification ./api -run 'TestParseOpenAccountNotificationUsesOfficialFields|TestAccountNotificationACK|TestBaofuAccountCallback' -count=1
```

Expected: fail because parser still expects invented fields or API returns JSON ACK.

- [x] **Step 3: Implement parser and ACK**

Parser must read `member_id`, `terminal_id`, `memberType`, `state`, `errorCode`, `errorMsg`, `transSerialNo`, `loginNo`, `customerName`, `contractNo`, `noticeType`. Callback handler must return `text/plain` body `OK` only after verified payload is persisted.

- [x] **Step 4: Run notification/API tests**

```bash
go test ./baofu/account/notification ./api -run 'TestBaofu.*Callback|TestParse.*Notification|Test.*ACK' -count=1
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add locallife/baofu/account/notification locallife/api/baofu_callback.go locallife/api/baofu_callback_test.go
git commit -m "fix(baofu): align account notification contract"
```

### Task 5: Merchant Report, Query, And APPLET Authorization Contracts

**Files:**
- Create: `locallife/baofu/merchantreport/contracts/types.go`
- Create: `locallife/baofu/merchantreport/contracts/enums.go`
- Create: `locallife/baofu/merchantreport/contracts/categories_generated.go`
- Create: `locallife/baofu/merchantreport/contracts/types_test.go`
- Create: `locallife/baofu/merchantreport/contracts/categories_test.go`

- [x] **Step 1: Add merchant report contract tests**

Create tests:

```go
func TestWechatMerchantReportRequiresMerchantBCTMerID(t *testing.T) {
    req := WechatMerchantReportRequest{
        MerchantID: "100000",
        TerminalID: "200000",
        ReportType: ReportTypeWechat,
        ReportNo: "MR202605040001",
        BCTMerchantID: "CM202605040001",
        ReportInfo: WechatReportInfo{
            MerchantName: "上海某某餐饮有限公司",
            MerchantShortName: "某某餐饮",
            ServicePhone: "02112345678",
            Business: "餐饮",
            ServiceCodes: []string{WechatServiceTypeApplet},
            BusinessLicenseType: WechatCertificateTypeNationalLegalMerge,
            BusinessLicense: "91310000123456789X",
        },
    }
    require.NoError(t, req.Validate())

    req.BCTMerchantID = ""
    require.EqualError(t, req.Validate(), "baofu merchant report bctMerId is required")
}

func TestBindSubConfigRequiresAppletAppID(t *testing.T) {
    req := BindSubConfigRequest{SubMchID: "1900000109", AuthType: AuthTypeApplet, AuthContent: "wx1234567890abcdef", Remark: "LocalLife mini program"}
    require.NoError(t, req.Validate())

    req.AuthContent = ""
    require.EqualError(t, req.Validate(), "baofu bind_sub_config authContent is required for APPLET")
}
```

- [x] **Step 2: Add category allowlist tests**

```go
func TestWechatCategoryAllowlistSource(t *testing.T) {
    require.Equal(t, "c521b7b15397a5aa63be9a3d8297c8a8c207e68e7d7fea7a26f8450945b4793f", WechatCategorySourceSHA256)
    require.Len(t, WechatCategories, 110)
    require.True(t, IsValidWechatCategory(WechatCategories[0].Value))
    require.False(t, IsValidWechatCategory("INVALID_CATEGORY"))
}
```

- [x] **Step 3: Run tests and confirm failure**

```bash
go test ./baofu/merchantreport/contracts -count=1
```

Expected: package missing or compile failure.

- [x] **Step 4: Implement contracts and generated allowlist**

Implement `merchant_report`, `merchant_report_query`, and `bind_sub_config` request/response DTOs with appendix enums. Generate or manually add the WeChat category allowlist from `/home/sam/文档/分账/宝付/经营类目&MCC.xlsx`; keep SHA256 and row count constants in the generated file.

- [x] **Step 5: Run contract tests**

```bash
go test ./baofu/merchantreport/contracts -count=1
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add locallife/baofu/merchantreport/contracts
git commit -m "feat(baofu): add merchant report contracts"
```

### Task 6: Merchant Report Persistence, Service, And Readiness Source

**Files:**
- Create: `locallife/db/migration/000229_add_baofu_merchant_reports.up.sql`
- Create: `locallife/db/migration/000229_add_baofu_merchant_reports.down.sql`
- Create: `locallife/db/query/baofu_merchant_report.sql`
- Create: `locallife/logic/baofu_merchant_report_service.go`
- Create: `locallife/logic/baofu_merchant_report_service_test.go`
- Modify: `locallife/logic/baofu_payment_readiness.go`
- Modify: `locallife/logic/baofu_payment_order_route.go`
- Modify: `locallife/logic/baofu_payment_service.go`
- Modify: `locallife/db/mock/store.go` via `make mock` or `make sqlc`

- [x] **Step 1: Add failing service tests**

Create `locallife/logic/baofu_merchant_report_service_test.go`:

```go
func TestBaofuMerchantReportServiceRequiresMerchantSharingMerID(t *testing.T) {
    store := newMockStoreWithoutSharingMerID(t)
    service := NewBaofuMerchantReportService(store, fakeMerchantReportClient{}, BaofuMerchantReportConfig{MiniProgramAppID: "wx123"})
    _, err := service.SubmitWechatMerchantReport(context.Background(), SubmitBaofuMerchantReportInput{MerchantID: 123})
    require.ErrorIs(t, err, ErrBaofuMerchantReportReceiverRequired)
}

func TestBaofuPaymentReadinessRequiresMerchantSubMchIDAndAppletAuth(t *testing.T) {
    binding := activeBaofuBindingWithSharingMerID(123, "CM202605040001")
    report := succeededMerchantReportWithoutAppletAuth(123, "1900000109")
    readiness := ReadinessFromBaofuBindingAndMerchantReport(binding, report)
    require.False(t, readiness.PaymentReady)
    require.Equal(t, "微信支付通道待开通", readiness.Label)
}
```

- [x] **Step 2: Run tests and confirm failure**

```bash
go test ./logic -run 'TestBaofuMerchantReport|TestBaofuPaymentReadinessRequiresMerchantSubMchID' -count=1
```

Expected: compile failure because service/table/readiness helpers do not exist.

- [x] **Step 3: Add migration and sqlc query**

Create `baofu_merchant_reports` with:

```sql
owner_type TEXT NOT NULL CHECK (owner_type = 'merchant'),
owner_id BIGINT NOT NULL,
report_type TEXT NOT NULL CHECK (report_type IN ('WECHAT','ALIPAY')),
report_no TEXT NOT NULL UNIQUE,
bct_mer_id TEXT NOT NULL,
sub_mch_id TEXT,
report_state TEXT NOT NULL CHECK (report_state IN ('processing','succeeded','failed','unknown')),
applet_auth_state TEXT NOT NULL DEFAULT 'pending' CHECK (applet_auth_state IN ('pending','succeeded','failed','not_required')),
platform_biz_no TEXT,
failure_code TEXT,
failure_message TEXT,
raw_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
UNIQUE (owner_type, owner_id, report_type)
```

- [x] **Step 4: Implement service and readiness**

Service must:

1. Read merchant Baofu binding.
2. Require `open_state=active` and non-empty `sharing_mer_id`.
3. Submit `merchant_report` with `bctMerId = sharing_mer_id`.
4. Persist command before client call.
5. Query report and sync `sub_mch_id` only on success.
6. Submit `bind_sub_config` after `sub_mch_id` exists.
7. Mark readiness only when report succeeded and APPLET auth succeeded.

- [x] **Step 5: Remove old `txResult.SubMchID` source**

In `locallife/logic/baofu_payment_order_route.go`, replace Baofu payment input source:

```go
MerchantWechatSubMchID: txResult.SubMchID,
```

with a merchant report readiness lookup result, for example:

```go
MerchantWechatSubMchID: readiness.SubMchID,
```

Then rename field to `MerchantSubMchID` in `CreateBaofuWechatJSAPIOrderInput` to avoid implying ordinary-service-provider origin.

- [x] **Step 6: Regenerate and test**

```bash
make sqlc
make mock
go test ./db/sqlc ./logic -run 'TestBaofuMerchantReport|TestBaofuPaymentReadiness|TestPaymentOrderServiceCreatePaymentOrder_RequiresMerchantBaofuReadiness' -count=1
make check-generated
```

Expected: pass; no code path reads ordinary-service-provider `txResult.SubMchID` for Baofu unified order.

- [x] **Step 7: Commit**

```bash
git add locallife/db/migration/000229_add_baofu_merchant_reports.* locallife/db/query/baofu_merchant_report.sql locallife/db/sqlc locallife/db/mock locallife/logic
git commit -m "feat(baofu): add merchant report readiness"
```

### Task 7: Complete Unified Order Validation Matrix

**Files:**
- Modify: `locallife/baofu/aggregatepay/contracts/types.go`
- Modify: `locallife/baofu/aggregatepay/contracts/types_test.go`

- [x] **Step 1: Add table-driven validation tests**

Add tests covering missing/invalid `merId`, `terId`, `outTradeNo`, `txnAmt`, `totalAmt`, `txnTime`, `prodType`, `orderType`, `payCode`, `payExtend.sub_appid`, `payExtend.sub_openid`, `payExtend.body`, `subMchId`, `riskInfo.clientIp`, `pageUrl` https, `forbidCredit`, and amount relation.

Example case:

```go
func TestUnifiedOrderRequestValidateOfficialRequiredFields(t *testing.T) {
    cases := []struct{
        name string
        mutate func(*UnifiedOrderRequest)
        want error
    }{
        {"missing merchant", func(r *UnifiedOrderRequest){ r.MerchantID = "" }, ErrUnifiedOrderMerchantIDRequired},
        {"missing sub mch for wechat", func(r *UnifiedOrderRequest){ r.SubMchID = "" }, ErrUnifiedOrderSubMchIDRequired},
        {"invalid product", func(r *UnifiedOrderRequest){ r.ProductType = "PAYMENT" }, ErrUnifiedOrderProductTypeUnsupported},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            req := validWechatUnifiedOrderRequestForTest()
            tc.mutate(&req)
            require.ErrorIs(t, req.Validate(), tc.want)
        })
    }
}
```

- [x] **Step 2: Run tests and confirm failure**

```bash
go test ./baofu/aggregatepay/contracts -run 'TestUnifiedOrderRequestValidateOfficialRequiredFields' -count=1
```

Expected: fail because current `Validate()` only checks `riskInfo.clientIp`.

- [x] **Step 3: Implement validation errors and checks**

Add typed errors and enforce:

```text
merId S(16) required
terId S(16) required
outTradeNo S(32) required
txnAmt > 0
totalAmt >= txnAmt
txnTime yyyyMMddHHmmss required
prodType == SHARING
orderType == 7
payCode == WECHAT_JSAPI for first version
subMchId required for WECHAT_/ALIPAY_
payExtend.sub_appid/sub_openid/body required for WECHAT_JSAPI
riskInfo.clientIp required for WECHAT_/ALIPAY_
pageUrl if present must be https
forbidCredit if present must be 0 or 1
```

- [x] **Step 4: Run tests**

```bash
go test ./baofu/aggregatepay/contracts -run 'TestUnifiedOrder|TestNormalizePaymentTerminalStatus|TestShareAfterPay' -count=1
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add locallife/baofu/aggregatepay/contracts/types.go locallife/baofu/aggregatepay/contracts/types_test.go
git commit -m "feat(baofu): validate unified order contract"
```

### Task 8: Concrete HTTP Transport For Account, Merchant Report, And Aggregate Pay

**Files:**
- Modify: `locallife/baofu/transport.go`
- Modify: `locallife/baofu/client.go`
- Modify: `locallife/baofu/account/client.go`
- Modify: `locallife/baofu/merchantreport/client.go`
- Modify: `locallife/baofu/aggregatepay/client.go`
- Tests: `locallife/baofu/**/*_test.go`

- [x] **Step 1: Add httptest transport tests**

For each client, add tests that assert URL, method, public envelope, and sanitized error mapping. Example:

```go
func TestAggregateClientCreateUnifiedOrderPostsPublicEnvelope(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        require.Equal(t, "/api", r.URL.Path)
        var env baofu.PublicRequestEnvelope
        require.NoError(t, json.NewDecoder(r.Body).Decode(&env))
        require.Equal(t, "unified_order", env.Method)
        require.NotEmpty(t, env.BizContent)
        _, _ = w.Write([]byte(successUnifiedOrderEnvelopeFixture()))
    }))
    defer server.Close()

    client := NewHTTPClient(testConfigWithAggregateBaseURL(server.URL + "/api"))
    _, err := client.AggregatePay.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForTest())
    require.NoError(t, err)
}
```

- [x] **Step 2: Run tests and confirm failure**

```bash
go test ./baofu ./baofu/account ./baofu/merchantreport ./baofu/aggregatepay -run 'Test.*Client' -count=1
```

Expected: fail because concrete clients are not implemented.

- [x] **Step 3: Implement clients**

Implement:

```go
AccountClient.OpenAccount(ctx, req)
AccountClient.QueryAccount(ctx, req)
AccountClient.QueryBalance(ctx, req)
AccountClient.CreateWithdraw(ctx, req)
AccountClient.QueryWithdraw(ctx, req)
MerchantReportClient.SubmitReport(ctx, req)
MerchantReportClient.QueryReport(ctx, req)
MerchantReportClient.BindSubConfig(ctx, req)
AggregatePayClient.CreateUnifiedOrder(ctx, req)
AggregatePayClient.QueryPayment(ctx, req)
AggregatePayClient.ShareAfterPay(ctx, req)
AggregatePayClient.QueryShare(ctx, req)
AggregatePayClient.Refund(ctx, req)
AggregatePayClient.QueryRefund(ctx, req)
AggregatePayClient.CloseOrder(ctx, req)
```

All methods must use context, configured timeout, one structured logging boundary, redacted request metadata, and no raw payload in ordinary errors.

Progress: current DTO-backed HTTP clients are implemented for account open/query/balance/withdraw/query withdraw, merchant report/report query/APPLET bind, unified order/query, share/share query, refund/query refund, and order close. Account client now uses official union-gw `verifyType=1` URL params + encrypted `content` + `header/body` response validation instead of aggregate public envelope. `verifyType=2` and sandbox evidence remain C4 open.

- [x] **Step 4: Run transport tests**

```bash
go test ./baofu ./baofu/account ./baofu/merchantreport ./baofu/aggregatepay -count=1
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add locallife/baofu artifacts/baofu-payment
git commit -m "feat(baofu): use official union gateway envelope"
```

### Task 9: Refund Before Share And Order Close Contracts

**Files:**
- Modify: `locallife/baofu/aggregatepay/contracts/types.go`
- Modify: `locallife/baofu/aggregatepay/contracts/types_test.go`
- Modify: `locallife/baofu/aggregatepay/notification/notification.go`
- Modify: `locallife/baofu/aggregatepay/notification/notification_test.go`
- Modify: `locallife/logic/refund_service.go`
- Modify: `locallife/logic/refund_service_test.go`

- [x] **Step 1: Add refund/close contract tests**

```go
func TestRefundBeforeShareRequestRejectsPostShareFields(t *testing.T) {
    req := validBaofuRefundBeforeShareRequestForTest()
    req.SharingRefundInfo = []SharingRefundDetail{{SharingMerID: "CM1", SharingAmountFen: 100}}
    require.ErrorIs(t, req.Validate(), ErrBaofuRefundSharingRefundInfoUnsupported)

    req = validBaofuRefundBeforeShareRequestForTest()
    req.AdvanceAmountFen = 100
    require.ErrorIs(t, req.Validate(), ErrBaofuRefundAdvanceUnsupported)
}

func TestOrderCloseRequiresOriginalPaymentReference(t *testing.T) {
    req := BaofuOrderCloseRequest{MerchantID: "100000", TerminalID: "200000"}
    require.ErrorIs(t, req.Validate(), ErrBaofuOrderCloseReferenceRequired)
}
```

- [x] **Step 2: Run tests and confirm failure**

```bash
go test ./baofu/aggregatepay/contracts ./logic -run 'TestRefundBeforeShare|TestOrderClose|TestBaofuRefund' -count=1
```

Expected: compile failure or missing DTOs.

- [x] **Step 3: Implement DTOs and business guards**

Add DTOs for `order_refund`, `refund_query`, `order_close`, refund notification. Logic must allow only pre-share refund and must call Baofu close when upstream order exists but local payment creation fails.

Progress: `order_refund`、`refund_query`、`order_close` DTO/client 已补齐；退款通知 parser、API callback route、退款 fact application 和退款查询恢复 scheduler 已补齐；`RefundService` 已把宝付分账前退款接入 `order_refund`，继续拒绝已进入分账流程的退款；宝付支付创建后本地 pay data 解析失败会调用 `order_close` 再关闭本地支付单。

- [x] **Step 4: Run refund tests**

```bash
go test ./baofu/aggregatepay/contracts ./baofu/aggregatepay/notification ./logic ./api ./worker -run 'TestRefund|TestOrderClose|TestBaofu|TestRefundRecoverySchedulerRunOnceQueriesBaofuRefundStatusByOrder' -count=1
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add locallife/baofu/aggregatepay locallife/logic/refund_service.go locallife/logic/refund_service_test.go
git commit -m "feat(baofu): add refund and close contracts"
```

### Task 10: Error Codes, Status Classification, And Frontend-Safe Semantics

**Files:**
- Create: `locallife/baofu/errors.go`
- Create: `locallife/baofu/errors_test.go`
- Modify: `locallife/logic/baofu_payment_service.go`
- Modify: `locallife/logic/baofu_account_service.go`
- Modify: `locallife/logic/baofu_merchant_report_service.go`
- Modify: `locallife/api/*baofu*.go`

- [x] **Step 1: Add error classification tests**

```go
func TestClassifyBaofuErrorCodeForFrontendSemantics(t *testing.T) {
    cases := []struct{
        code string
        wantCategory BaofuErrorCategory
        wantPublic string
    }{
        {"PARAM_ERROR", BaofuErrorCategoryUserActionRequired, "资料信息不完整，请核对后重新提交"},
        {"MERCHANT_NOT_REPORTED", BaofuErrorCategoryPlatformConfiguration, "微信支付通道待开通，请联系平台处理"},
        {"SYSTEM_BUSY", BaofuErrorCategoryRetryable, "支付通道处理中，请稍后重试"},
        {"UNKNOWN", BaofuErrorCategoryManualReview, "支付通道异常，请联系平台处理"},
    }
    for _, tc := range cases {
        got := ClassifyBaofuError(tc.code, "raw upstream message")
        require.Equal(t, tc.wantCategory, got.Category)
        require.Equal(t, tc.wantPublic, got.PublicMessage)
        require.NotContains(t, got.PublicMessage, "raw upstream")
    }
}
```

- [x] **Step 2: Run tests and confirm failure**

```bash
go test ./baofu ./logic ./api -run 'TestClassifyBaofuError|TestBaofu.*Error' -count=1
```

Expected: compile failure or missing classifier.

- [x] **Step 3: Implement classifier**

Implement categories:

```go
type BaofuErrorCategory string
const (
    BaofuErrorCategoryUserActionRequired BaofuErrorCategory = "user_action_required"
    BaofuErrorCategoryPlatformConfiguration BaofuErrorCategory = "platform_configuration"
    BaofuErrorCategoryRetryable BaofuErrorCategory = "retryable"
    BaofuErrorCategoryManualReview BaofuErrorCategory = "manual_review"
)
```

Only log upstream code/raw message at the provider boundary. API responses use public Chinese copy and no raw identifiers.

Progress: `baofu.ClassifyBaofuError` now maps the official account error-code page and aggregate payment error-code page into safe Chinese frontend guidance:资料需修改、身份/银行卡核验失败、平台/商户配置待开通、商户微信渠道待报备、可查询/可重试处理中、渠道/宝付异常需人工处理。Union-gw account `retCode` failures and aggregate/merchant-report `resultCode != SUCCESS` business failures are converted to `ProviderError` before DTO unmarshalling, preserving upstream code/message only in the provider error/log boundary. Baofu payment/refund creation errors map provider errors into `RequestError` without exposing upstream raw messages.

- [x] **Step 4: Run error tests**

```bash
go test ./baofu ./logic ./api -run 'TestClassifyBaofuError|TestBaofu.*Error|Test.*Sanitized' -count=1
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add locallife/baofu/errors.go locallife/baofu/errors_test.go locallife/logic locallife/api
git commit -m "feat(baofu): classify provider errors"
```

### Task 11: Runtime Wiring And Direct-Payment Boundary

**Files:**
- Modify: `locallife/api/logic_adapters.go`
- Modify: `locallife/api/server.go`
- Modify: `locallife/logic/payment_order_service.go`
- Modify: `locallife/logic/combined_payment_service.go`
- Tests: `locallife/api/payment_order_test.go`, `locallife/logic/payment_order_service_test.go`, `locallife/logic/direct_payment_order_errors_test.go`

- [x] **Step 1: Add fail-closed runtime tests**

```go
func TestCreatePaymentOrderAPIUsesBaofuWhenMainBusinessConfigured(t *testing.T) {
    server := newTestServerWithBaofuMainBusiness(t)
    resp := postCreateMainBusinessPayment(t, server)
    require.Equal(t, http.StatusOK, resp.Code)
    require.True(t, fakeBaofuAggregateClient.CalledCreateUnifiedOrder)
    require.False(t, fakeOrdinaryServiceProviderClient.CalledCreatePayment)
}

func TestDirectPaymentPathsDoNotUseBaofu(t *testing.T) {
    server := newTestServerWithBaofuMainBusiness(t)
    resp := postRiderDepositPayment(t, server)
    require.Equal(t, http.StatusOK, resp.Code)
    require.False(t, fakeBaofuAggregateClient.CalledCreateUnifiedOrder)
    require.True(t, fakeDirectPaymentClient.CalledCreatePayment)
}
```

- [x] **Step 2: Run tests and confirm failure**

```bash
go test ./api ./logic -run 'TestCreatePaymentOrderAPIUsesBaofu|TestDirectPaymentPathsDoNotUseBaofu|TestPaymentOrderServiceCreatePaymentOrder_UsesBaofu' -count=1
```

Expected: fail if API runtime still hard-codes ordinary service provider facade.

- [x] **Step 3: Implement runtime wiring**

`server.buildPaymentFacade()` must choose Baofu for main-business ordinary-service-provider replacement only when Baofu config/client is complete. Missing config returns safe 500/400 product error before local payment rows are created. Existing `direct` flows remain unchanged.

- [x] **Step 4: Run boundary tests**

```bash
go test ./api ./logic -run 'TestCreatePaymentOrderAPI|TestPaymentOrderServiceCreatePaymentOrder|TestDirectPayment|TestRiderDeposit|TestClaimRecovery' -count=1
make check-generated
```

Expected: pass; Baofu path has no ordinary-service-provider fallback.

- [x] **Step 5: Commit**

```bash
git add locallife/api locallife/logic locallife/docs
git commit -m "feat(payment): wire baofu main business payments"
```

### Task 12: Sandbox Evidence And Audit Status Updates

**Files:**
- Create: `artifacts/baofu-payment/baofu-sandbox-evidence.md`
- Modify: `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`
- Modify: `artifacts/baofu-payment/baofu-profit-sharing-implementation-plan.md`

- [x] **Step 1: Create evidence template**

Create `baofu-sandbox-evidence.md` with sections:

```markdown
# 宝付宝财通 Sandbox Evidence

## Evidence Rules

- Do not store raw ID card numbers, full bank cards, phone numbers, private keys, AES keys, full signatures, or raw payloads.
- Store masked request/response summaries, interface name, endpoint, out request number, Baofu trade/report number, observed status, callback/query recovery result, test time, and commit SHA.

## Account Open `T-1001-013-01`

| Date | Env | OutRequestNo | Owner | Result | Callback | Query | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Merchant Report `merchant_report`

| Date | Env | ReportNo | Merchant | subMchId Masked | APPLET Auth | Query | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Unified Order `unified_order`

| Date | Env | OutTradeNo | subMchId Masked | wc_pay_data | Callback | Query | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
```

- [x] **Step 2: Update audit status after each C3 task**

After Tasks 1-11, update `baofu-api-contract-coverage-audit.md` from C0/C1/C2 to C3 only when code has real transport and local tests. Do not mark C4 until sandbox evidence table has a row.

- [x] **Step 3: Run doc validation**

```bash
git diff --check -- artifacts/baofu-payment/baofu-api-contract-coverage-audit.md artifacts/baofu-payment/baofu-profit-sharing-implementation-plan.md artifacts/baofu-payment/baofu-sandbox-evidence.md
```

Expected: no whitespace errors.

- [x] **Step 4: Commit**

```bash
git add artifacts/baofu-payment/baofu-api-contract-coverage-audit.md artifacts/baofu-payment/baofu-profit-sharing-implementation-plan.md artifacts/baofu-payment/baofu-sandbox-evidence.md
git commit -m "docs(baofu): add sandbox evidence tracker"
```

## 4. Recommended Commit Order

1. `feat(baofu): split official endpoint profiles`
2. `feat(baofu): add public request envelope`
3. `feat(baofu): add official account contracts`
4. `fix(baofu): align account notification contract`
5. `feat(baofu): add merchant report contracts`
6. `feat(baofu): add merchant report readiness`
7. `feat(baofu): validate unified order contract`
8. `feat(baofu): implement http clients`
9. `feat(baofu): add refund and close contracts`
10. `feat(baofu): classify provider errors`
11. `feat(payment): wire baofu main business payments`
12. `docs(baofu): add sandbox evidence tracker`

## 5. Validation Matrix

| Change area | Required command |
| --- | --- |
| Config/envelope/signing | `go test ./baofu -count=1` |
| Account contracts/notifications | `go test ./baofu/account ./baofu/account/contracts ./baofu/account/notification -count=1` |
| Merchant report contracts/service | `make sqlc && make mock && go test ./baofu/merchantreport ./logic ./db/sqlc -run 'TestBaofuMerchantReport|TestBaofuPaymentReadiness' -count=1 && make check-generated` |
| Aggregate payment contracts/client | `go test ./baofu/aggregatepay ./baofu/aggregatepay/contracts ./baofu/aggregatepay/notification -count=1` |
| Payment runtime boundary | `go test ./api ./logic -run 'TestCreatePaymentOrderAPI|TestPaymentOrderServiceCreatePaymentOrder|TestDirectPayment|TestRiderDeposit|TestClaimRecovery' -count=1` |
| High-risk safety pass | `make test-safety && make check-generated` |
| Full pre-release backend pass | `make test-unit && make test-integration` |

## 6. Completion Gate

This remediation is not complete until all of the following are true:

- `baofu-api-contract-coverage-audit.md` no longer marks required production interfaces as C0/C1/C2, except explicitly deferred non-MVP interfaces.
- `merchant_report` + `bind_sub_config` can open a merchant's WeChat channel with LocalLife mini program appid.
- `unified_order` uses merchant `sub_mch_id` from successful report/auth readiness.
- `share_after_pay` has no dependency on `subMchId` and only uses `sharing_mer_id` receivers.
- Refund-before-share and close-order contracts exist and are tested.
- All provider errors map to safe Chinese product semantics.
- No direct-payment path calls Baofu.
- At least one sandbox evidence row exists for account open, merchant report, APPLET auth, unified order, payment callback/query, share callback/query, refund-before-share, and withdraw.

### 2026-05-04 Follow-up: Baofu Withdrawal Query Recovery

- Added `BaofuWithdrawalRecoveryScheduler` to scan processing BaoCaiTong withdrawal orders, call `T-1001-013-15` through the account client with the payout merchant/terminal, and enqueue `BaofuWithdrawalFactApplicationPayload` for terminal states.
- Added `DistributeTaskProcessBaofuWithdrawalFactApplication` to the task distributor boundary and regenerated worker mocks.
- Wired `baofu-withdrawal-recovery` in `main.go` when Baofu account runtime config is available; missing config logs a scheduler-boundary warning instead of silently pretending recovery is active.
- Residual risk: sandbox query evidence and withdraw notification callback route were still C4/C3-open before the next follow-up slice.

### 2026-05-04 Follow-up: Baofu Withdrawal Notification Callback

- Added encrypted BaoCaiTong withdraw notification parsing through the account notification parser and covered official withdraw plaintext fields plus envelope parsing locally.
- Added `/v1/webhooks/baofu/withdraw`; it resolves local withdrawal orders by `transSerialNo`, enqueues `BaofuWithdrawalFactApplicationPayload`, and returns the official plain-text `OK` only after durable task enqueue succeeds.
- Wired the default account notification parser in `api.NewServer` when Baofu runtime config is available.
- Residual risk: real BaoCaiTong withdraw notification URL shape, replay behavior, and sandbox callback samples remain C4-open.

### 2026-05-04 Follow-up: Aggregate Merchant Report Query Recovery

- Added `RecoverWechatMerchantReport` to the merchant-report service so delayed `merchant_report_query` success writes `sub_mch_id` and then binds `authType=APPLET` to the LocalLife mini-program appid through `bind_sub_config`.
- Added `ListRecoverableBaofuMerchantReports` and `baofu-merchant-report-recovery` scheduler for processing WeChat reports and pending APPLET auth bindings.
- Wired concrete merchant-report client construction in `main.go`; the scheduler is registered only when Baofu runtime config is available and otherwise fails closed with a scheduler-boundary warning.
- Residual risk: real Baofu merchant report/query/bind_sub_config sandbox evidence and complete production資料来源映射 remain C4/open.

### 2026-05-04 Follow-up: Merchant Report Appendix Enums

- Added typed constants and allowlist helpers for the merchant-report appendix enums identified in the audit: terminal device types, operation flags, device statuses, WeChat/Alipay service and certificate values, contact business/type values, site types, indirect levels, merchant statuses, transaction controls, auth order states, and merchant auth states.
- Added table-driven coverage so unsupported appendix values fail closed before future DTO fields can silently drift.
- Residual risk: these enums are local C3 guardrails only; production资料映射 and sandbox response samples still need evidence.


## 7. Pre-`dataContent` Contract Drift Audit Plan

> Purpose: before depending on any future real Baofoo `dataContent` business sample, re-audit the full local contract boundary against Baofoo docs/demo material and eliminate every drift that can be found from static docs, Java demo, request builders, validators, tests, and safe smoke diagnostics. This plan must not mark any interface C4; C4 still requires real sandbox evidence in `baofu-sandbox-evidence.md`.

### 7.1 Scope And Rules

- Scope is only Baofoo/BaoCaiTong main-business replacement: `locallife/baofu/**`, Baofoo-facing payment/report/account logic, and `artifacts/baofu-payment/**`.
- Do not touch `merchant_app/`, WeChat direct-payment flows, or ordinary-service-provider cold-reserve code except read-only boundary checks.
- Do not use real PII, private keys, full signatures, raw encrypted payloads, full bank card numbers, full ID card numbers, or full Baofoo payload dumps in tests/docs.
- Every drift finding must end in one of three states: fixed with a local regression, explicitly deferred with a Baofoo question, or proven not applicable to LocalLife first version.
- The implementation loop for each task is `done -> review -> fix -> review -> lint/script -> commit -> next`.

### 7.2 Drift Classes To Eliminate Before Real `dataContent`

| Class | Meaning | Local proof required |
| --- | --- | --- |
| Wire-format drift | HTTP method, content type, top-level envelope field name, signing input, timestamp, serial index, encrypted field shape | Request recorder tests + safe smoke diagnostics show exact form/query/header keys and masked values. |
| Request DTO drift | Field name, type, length, required/conditional-required, enum, amount unit, nested JSON string/object shape | Table-driven `Validate()` tests for every required and conditional-required branch used by LocalLife. |
| Response DTO drift | Top-level success/failure envelope, business payload field, business status/error fields, unknown/empty payload behavior | Fixture tests using doc/demo-shaped response bodies; no direct `BizContent` reads in public-response callers. |
| Error/status drift | Official status enums and error fields are misread or collapsed into success | Provider-error tests prove fail-closed behavior and safe Chinese frontend guidance. |
| Cross-flow source drift | Wrong identifier source, such as using WeChat applyment `subMchId` or Baofoo一级商户号 where Baofoo二级户 is required | Static checks and service tests prove `subMchId` comes from merchant report, and `sharingMerId` comes from `sharing_mer_id`. |
| Documentation drift | Docs/audit say one thing while code tests enforce another | Every code fix updates `baofu-api-contract-coverage-audit.md`, this plan, or `baofu-sandbox-evidence.md` as applicable. |

### Task P1: Freeze Official Source Ledger

**Files:**
- Modify: `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`
- Modify: `artifacts/baofu-payment/baofu-contract-drift-remediation-plan.md`

- [x] **Step 1: Inventory local official sources**

Run from repo root:

```bash
rg -n "接口请求入口|bizContent|dataContent|riskInfo|share_after_pay|merchant_report|merchant_report_query|bind_sub_config|T-1001-013-01|T-1001-013-03|T-1001-013-06|T-1001-013-14|T-1001-013-15" \
  /home/sam/文档/分账/宝付 /tmp/baofu_demo/java \
  > /tmp/baofu_contract_source_hits.txt
```

Expected: source hits include BaoCaiTong union-gw docs/material, aggregate-pay public envelope docs/demo, merchant-report docs/demo, and Java `ResultMasterEntity.dataContent`.

- [x] **Step 2: Add source ledger table**

In `baofu-api-contract-coverage-audit.md`, add a "Source Ledger" table with one row per interface group:

| Group | Local source file/demo | Official URL | Current trust level | Notes |
| --- | --- | --- | --- | --- |
| union-gw account | `/home/sam/文档/分账/宝付/...` | doc.mandao `unionGw/openAcc/query/balance/withdraw` | doc + local tests | verifyType=1 only until sandbox callbacks prove more. |
| aggregate pay | `/tmp/baofu_demo/java/...PostMasterEntity/ResultMasterEntity...` | doc.mandao 聚合支付 | doc + Java demo + smoke | request `bizContent`, response `dataContent`. |
| merchant report | `/tmp/baofu_demo/java/...` and merchant-report docs | doc.mandao 聚合商户报备 | doc + local tests | APPLET bind and异主体报备 are required. |

- [x] **Step 3: Review**

Check the ledger for missing first-version interfaces: account open/query/balance/withdraw/query withdraw, merchant report/query/APPLET bind, unified order/query/payment callback, share/query/share callback, refund/query/refund callback, close.

- [x] **Step 4: Commit**

```bash
git add artifacts/baofu-payment/baofu-api-contract-coverage-audit.md artifacts/baofu-payment/baofu-contract-drift-remediation-plan.md
git commit -m "docs(baofu): freeze contract source ledger"
```

### Task P2: Lock Public Envelope Request/Response Shape

**Files:**
- Modify: `locallife/baofu/envelope.go`
- Modify: `locallife/baofu/client.go`
- Modify: `locallife/baofu/envelope_test.go`
- Modify: `locallife/baofu/aggregatepay/client_test.go`
- Modify: `locallife/baofu/merchantreport/client_test.go`
- Modify: `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`

- [x] **Step 1: Add/verify request fixture tests**

`go test ./baofu -run 'TestPublicEnvelope' -count=1` must prove:

- request is form-compatible through `FormValues()`;
- request business payload field is exactly `bizContent`;
- `bizContent` is JSON text, not an embedded object in form transport;
- `timestamp` is `yyyyMMddHHmmss` in `Asia/Shanghai`;
- `signSn/ncrptnSn` reject values longer than S(10).

- [x] **Step 2: Add/verify response fixture tests**

`go test ./baofu -run 'TestPublicResponseEnvelope' -count=1` must prove:

- official success response reads business JSON from `dataContent`;
- local legacy `bizContent` fallback is allowed only through `BusinessContent()`;
- missing business content on `returnCode=SUCCESS` fails closed;
- `returnCode=FAIL` does not require business content and preserves `returnMsg`.

- [x] **Step 3: Remove direct response `BizContent` reads**

Run:

```bash
rg -n "responseEnvelope\\.BizContent|PublicResponseEnvelope\\{[^\\n]*BizContent|BizContent:.*responseBizContent" locallife/baofu
```

Expected: no production caller reads response `BizContent` directly. Test fixtures should prefer `DataContent`.

- [x] **Step 4: Validate**

```bash
cd locallife
PATH="/usr/local/go/bin:$PATH" go test ./baofu ./baofu/aggregatepay ./baofu/merchantreport -count=1
```

- [x] **Step 5: Commit**

```bash
git add locallife/baofu artifacts/baofu-payment/baofu-api-contract-coverage-audit.md
git commit -m "fix(baofu): lock public envelope contract"
```

### Task P3: Re-Audit BaoCaiTong union-gw Account Contracts

**Files:**
- Modify: `locallife/baofu/uniongw.go`
- Modify: `locallife/baofu/client.go`
- Modify: `locallife/baofu/account/contracts/official_open.go`
- Modify: `locallife/baofu/account/contracts/official_query.go`
- Modify: `locallife/baofu/account/contracts/official_balance.go`
- Modify: `locallife/baofu/account/contracts/official_withdraw.go`
- Modify: `locallife/baofu/account/contracts/types_test.go`
- Modify: `locallife/baofu/account/notification/notification.go`
- Modify: `locallife/baofu/account/notification/notification_test.go`
- Modify: `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`

- [x] **Step 1: Build account field matrix**

In the audit doc, for each union-gw interface, fill a table with: official field name, local struct field, JSON tag, type/length, required rule, conditional rule, enum/constant, and test name.

- [x] **Step 2: Add negative validation table tests**

For account contracts, tests must cover:

- open account `version=4.1.0`, `businessType=BCT2.0`, `accType`, `noticeUrl`, `transSerialNo`, `loginNo`;
- personal two-factor vs four-factor required fields;
- enterprise/individual-business required fields and the known `selfEmployed`/private-card mobile condition;
- query key rules: one official query key only, no fake literal placeholders;
- balance amount unit conversion yuan <-> fen and invalid decimal precision;
- withdraw amount unit conversion, `contractNo`, order number, bank/card fields, and query order number.

- [x] **Step 3: Verify union-gw envelope behavior**

Tests must assert:

- URL path includes `/union-gw/api/{serviceTp}/transReq.do`;
- query has `memberId`, `terminalId`, `verifyType=1`, `content`;
- plaintext request has `header.serviceTp` equal to the path service number;
- response validates `sysRespCode`, `memberId`, `terminalId`, and `serviceTp`;
- account business error payload with `errorCode/errorMsg` and missing `retCode` fails closed.

- [x] **Step 4: Verify account notifications**

Tests must assert:

- account/withdraw notification parser accepts official encrypted `data_content` parameter name;
- parser rejects missing `data_content`;
- callback ACK remains plain text `OK` only after durable enqueue succeeds;
- no static AES key is required for `verifyType=1`.

- [x] **Step 5: Validate**

```bash
cd locallife
PATH="/usr/local/go/bin:$PATH" go test ./baofu ./baofu/account ./baofu/account/contracts ./baofu/account/notification -count=1
```

- [x] **Step 6: Commit**

```bash
git add locallife/baofu locallife/baofu/account artifacts/baofu-payment/baofu-api-contract-coverage-audit.md
git commit -m "fix(baofu): re-audit account contract drift"
```

### Task P4: Re-Audit Aggregate Pay Contracts

**Files:**
- Modify: `locallife/baofu/aggregatepay/contracts/types.go`
- Modify: `locallife/baofu/aggregatepay/contracts/types_test.go`
- Modify: `locallife/baofu/aggregatepay/client.go`
- Modify: `locallife/baofu/aggregatepay/client_test.go`
- Modify: `locallife/baofu/aggregatepay/notification/notification.go`
- Modify: `locallife/baofu/aggregatepay/notification/notification_test.go`
- Modify: `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`

- [x] **Step 1: Build aggregate method field matrix**

Add or complete audit tables for `unified_order`, `order_query`, `share_after_pay`, `share_query`, `order_refund`, `refund_query`, and `order_close`. Each row must name the method, field, type/length, M/C/O rule, enum source, local JSON tag, and local test name.

- [x] **Step 2: Verify unified order mandatory and conditional fields**

Table tests must prove:

- `payType=WECHAT_JSAPI`;
- `prodType=SHARING`;
- `orderType=7`;
- `subMchId` is required for WeChat JSAPI;
- `payExtend.sub_openid` is required for WeChat JSAPI;
- `riskInfo.clientIp` is required for WeChat/Alipay channel use;
- `txnAmt`/amount fields use fen if docs say amount is integer fen, and tests reject zero/negative;
- notify URL is HTTPS and not placeholder.

- [x] **Step 3: Verify payment query contract**

Tests must prove `merId`, `terId`, and one of `tradeNo/outTradeNo` are required before POST, and missing identifiers never reach Baofoo.

- [x] **Step 4: Verify share contract**

Tests must prove:

- `share_after_pay` has no `subMchId` field;
- every receiver has `sharingMerId`, amount, and receiver role/type if required by docs;
- local receiver source is `baofu_account_bindings.sharing_mer_id`, not Baofoo一级商户号, not `contractNo`, and not WeChat `subMchId`.

- [x] **Step 5: Verify refund-before-share and close contracts**

Tests must prove:

- first-version refund DTO excludes post-share refund/advance/垫付 fields unless docs make them mandatory for pre-share refund;
- refund is blocked after share terminal success;
- `order_close` has the documented identifiers and cannot close direct-payment orders through Baofoo.

- [x] **Step 6: Verify notification payloads**

Notification parser tests must cover payment, share, and refund terminal statuses, duplicate-safe ACK behavior, and unknown status fail-closed classification.

- [x] **Step 7: Validate**

```bash
cd locallife
PATH="/usr/local/go/bin:$PATH" go test ./baofu/aggregatepay ./baofu/aggregatepay/contracts ./baofu/aggregatepay/notification -count=1
```

- [x] **Step 8: Commit**

```bash
git add locallife/baofu/aggregatepay artifacts/baofu-payment/baofu-api-contract-coverage-audit.md
git commit -m "fix(baofu): re-audit aggregate pay contracts"
```

### Task P5: Re-Audit Merchant Report And APPLET Bind Contracts

**Files:**
- Modify: `locallife/baofu/merchantreport/contracts/types.go`
- Modify: `locallife/baofu/merchantreport/contracts/enums.go`
- Modify: `locallife/baofu/merchantreport/contracts/categories_generated.go`
- Modify: `locallife/baofu/merchantreport/contracts/types_test.go`
- Modify: `locallife/baofu/merchantreport/contracts/categories_test.go`
- Modify: `locallife/baofu/merchantreport/client.go`
- Modify: `locallife/logic/baofu_merchant_report_service.go`
- Modify: `locallife/logic/baofu_payment_readiness.go`
- Modify: `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`

- [x] **Step 1: Build merchant-report field matrix**

Audit tables must cover `merchant_report`, `merchant_report_query`, and `bind_sub_config`, including report identity fields, `bctMerId`, channel type, business category/MCC, contact/settlement fields, service codes, certificate types, site info, and APPLET auth fields.

- [x] **Step 2: Verify APPLET bind requirement**

Tests must prove every merchant `subMchId` from report success gets `bind_sub_config(authType=APPLET, authContent=<WECHAT_MINI_APP_ID>)`, and payment readiness remains false until APPLET bind success.

- [x] **Step 3: Verify异主体报备 model**

Service/readiness tests must prove LocalLife uses each merchant's Baofoo report `subMchId` for `unified_order`, not one platform-wide `subMchId`, because Baofoo confirmed异主体报备 support.

- [x] **Step 4: Verify appendix enums and category file**

Tests must prove:

- all enum values used by LocalLife are constants/allowlisted;
- unsupported appendix values fail validation;
- `/home/sam/文档/分账/宝付/经营类目&MCC.xlsx` derived category constants have a recorded source hash/row count;
- no hardcoded unknown category string bypasses the allowlist.

- [x] **Step 5: Validate**

```bash
cd locallife
PATH="/usr/local/go/bin:$PATH" go test ./baofu/merchantreport ./baofu/merchantreport/contracts ./logic -run 'TestBaofuMerchantReport|TestBaofuPaymentReadiness' -count=1
```

- [x] **Step 6: Commit**

```bash
git add locallife/baofu/merchantreport locallife/logic/baofu_merchant_report_service.go locallife/logic/baofu_payment_readiness.go artifacts/baofu-payment/baofu-api-contract-coverage-audit.md
git commit -m "fix(baofu): re-audit merchant report contracts"
```

### Task P6: Re-Audit Cross-Flow Identifier Sources

**Files:**
- Modify: `locallife/logic/baofu_account_service.go`
- Modify: `locallife/logic/baofu_merchant_report_service.go`
- Modify: `locallife/logic/baofu_payment_service.go`
- Modify: `locallife/logic/baofu_payment_readiness.go`
- Modify: `locallife/db/query/baofu_account_binding.sql`
- Modify: `locallife/db/query/baofu_merchant_report.sql`
- Modify: `artifacts/baofu-payment/baofu-profit-sharing-integration-design.md`

- [ ] **Step 1: Add source-of-truth table to design doc**

Document these invariants:

| Runtime need | Correct source | Incorrect sources that must never be used |
| --- | --- | --- |
| `sharingMerId` | Baofoo open-account returned Baofoo二级商户号 stored in `baofu_account_bindings.sharing_mer_id` | Baofoo收单一级商户号, Baofoo代付一级商户号, WeChat `subMchId`, `contractNo` unless Baofoo returns the same value and code explicitly syncs to `sharing_mer_id`. |
| `unified_order.subMchId` | Successful merchant report `sub_mch_id` | WeChat ordinary-service-provider applyment result, platform unified `subMchId`, Baofoo二级商户号. |
| APPLET auth content | `WECHAT_MINI_APP_ID` | merchant-provided appid, random report field, empty placeholder. |

- [ ] **Step 2: Static checks**

Run:

```bash
rg -n "ordinaryserviceprovider|wechat_sub_mch|TxResult\\.SubMCH|TxResult\\.SubMch|contractNo.*sharing|CollectMerchantID.*sharing|subMchId.*share" locallife/logic locallife/baofu locallife/db/query
```

Expected: any hit is either read-only cold-reserve code outside Baofoo main path or has an explicit test proving the correct boundary.

- [ ] **Step 3: Service tests**

Tests must prove:

- missing `sharing_mer_id` blocks share receiver creation before client call;
- missing merchant report `sub_mch_id` blocks unified order before client call;
- pending APPLET bind blocks unified order before client call;
- direct-payment order creation never calls Baofoo client.

- [ ] **Step 4: Validate**

```bash
cd locallife
PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuPayment|TestBaofuMerchantReport|TestDirectPayment' -count=1
```

- [ ] **Step 5: Commit**

```bash
git add locallife/logic locallife/db/query artifacts/baofu-payment/baofu-profit-sharing-integration-design.md
git commit -m "fix(baofu): lock identifier source boundaries"
```

### Task P7: Re-Audit Provider Error And Status Mapping

**Files:**
- Modify: `locallife/baofu/errors.go`
- Modify: `locallife/baofu/errors_test.go`
- Modify: `locallife/baofu/client.go`
- Modify: `locallife/baofu/account/contracts/*.go`
- Modify: `locallife/baofu/aggregatepay/contracts/*.go`
- Modify: `locallife/baofu/merchantreport/contracts/*.go`
- Modify: `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`

- [ ] **Step 1: Build status/error matrix**

Audit doc must list every locally interpreted status/error field:

- union-gw `sysRespCode/sysRespDesc`, business `retCode/errorCode/errorMsg`, account `state`;
- public envelope `returnCode/returnMsg`;
- public business `resultCode/errCode/errMsg`;
- payment/share/refund/report/auth status enums.

- [ ] **Step 2: Add fail-closed tests**

Tests must prove:

- unknown non-empty failure codes produce `ProviderError`;
- missing success indicators with error fields fail closed;
- upstream raw message is kept in `ProviderError.UpstreamMessage` for logs/ops but not leaked through `Frontend.Message`;
- frontend guidance is stable Chinese product guidance.

- [ ] **Step 3: Validate**

```bash
cd locallife
PATH="/usr/local/go/bin:$PATH" go test ./baofu ./baofu/account/contracts ./baofu/aggregatepay/contracts ./baofu/merchantreport/contracts -run 'Test.*Error|Test.*Status|Test.*Validate' -count=1
```

- [ ] **Step 4: Commit**

```bash
git add locallife/baofu artifacts/baofu-payment/baofu-api-contract-coverage-audit.md
git commit -m "fix(baofu): lock error and status contracts"
```

### Task P8: Add Static Drift Guard Script

**Files:**
- Create: `locallife/scripts/check_baofu_contract_drift.sh`
- Modify: `locallife/Makefile`
- Modify: `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`

- [ ] **Step 1: Create static guard script**

The script must fail on known drift patterns:

```bash
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

! rg -n 'responseEnvelope\.BizContent' baofu
! rg -n 'https://api\.baofoo\.com' baofu util
! rg -n 'subMchId.*share|share.*subMchId' baofu/aggregatepay logic
! rg -n 'CollectMerchantID.*sharingMerId|PayoutMerchantID.*sharingMerId' baofu logic
! rg -n 'BAOFU_AES_KEY' baofu util app.env.example
```

- [ ] **Step 2: Wire make target**

Add:

```make
check-baofu-contract:
	./scripts/check_baofu_contract_drift.sh
```

- [ ] **Step 3: Validate**

```bash
cd locallife
chmod +x scripts/check_baofu_contract_drift.sh
make check-baofu-contract
```

- [ ] **Step 4: Commit**

```bash
git add locallife/scripts/check_baofu_contract_drift.sh locallife/Makefile artifacts/baofu-payment/baofu-api-contract-coverage-audit.md
git commit -m "test(baofu): add contract drift guard"
```

### Task P9: Final Pre-`dataContent` Review Gate

**Files:**
- Modify: `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`
- Modify: `artifacts/baofu-payment/baofu-contract-drift-remediation-plan.md`
- Modify: `artifacts/baofu-payment/baofu-sandbox-evidence.md`

- [ ] **Step 1: Run full pre-dataContent validation**

```bash
cd locallife
PATH="/usr/local/go/bin:$PATH" go test ./baofu/... ./logic -run 'TestBaofu|TestDirectPayment' -count=1
make check-baofu-contract
git diff --check
```

- [ ] **Step 2: Update audit grades**

Only mark an interface C3 when all of these are true:

- official source row exists;
- field matrix has required/conditional/type/enum rows;
- contract tests cover success and failure/invalid branches;
- runtime client/wiring uses the DTO;
- static guard has no known drift pattern.

Keep every interface C4-open until a real sandbox row is added.

- [ ] **Step 3: Produce next-test checklist**

In `baofu-sandbox-evidence.md`, add a "Ready for next sandbox test" checklist for:

- synthetic `order_query` parsing of `dataContent`;
- successful account open using safe test identity material;
- merchant report + query + APPLET bind;
- unified order with real `subMchId/sub_openid`;
- share/query/callback;
- refund-before-share/query/callback;
- withdraw/query/callback.

- [ ] **Step 4: Commit**

```bash
git add artifacts/baofu-payment/baofu-api-contract-coverage-audit.md artifacts/baofu-payment/baofu-contract-drift-remediation-plan.md artifacts/baofu-payment/baofu-sandbox-evidence.md
git commit -m "docs(baofu): close pre-dataContent drift audit"
```

### 7.3 Completion Gate For This Pre-`dataContent` Audit

This pre-sandbox-positive audit is complete only when:

- public-envelope request/response asymmetry is locked by tests and no production caller reads response `BizContent` directly;
- every required first-version Baofoo interface has a field matrix row for request, response, status/error fields, required/conditional-required rules, type/length, and enum source;
- every required/conditional-required rule used by LocalLife has a negative table test;
- static guard blocks the drift patterns already discovered during smoke;
- docs state clearly that C3 means local contract/transport coverage only, and C4 still requires real Baofoo sandbox request/callback/query evidence.
