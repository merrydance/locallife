# 宝付宝财通分账接入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 LocalLife 主业务订单支付、分账、二级户开户、余额、提现全量接入宝付宝财通，首版固定分账后不退款、支付手续费由商户承担、骑手和商户都通过宝付二级户收款。

**Architecture:** 新增 `locallife/baofu` 作为宝付协议边界，业务层只依赖项目内 request/result/notification 类型。复用现有 `payment_orders`、`profit_sharing_orders`、`external_payment_commands`、`external_payment_facts`、`payment_domain_outbox` 的资金域解耦模型，新增宝付账户绑定、费用台账和提现流水表。所有外部请求先记录 command，所有回调和查询结果先落 fact，再由 worker 在事务内推进本地状态。

**Tech Stack:** Go, PostgreSQL migrations, sqlc, Gin HTTP handlers, asynq workers, zerolog structured logging, existing LocalLife payment-domain outbox and recovery schedulers.

---

## 0. Scope And Risk

- 风险等级：G3，高风险资金链路。原因：支付、分账、提现、手续费、退款互斥、外部回调和查询补偿都会改变真实资金状态。
- 首版切换方式：未正式开展业务，宝付链路上线即全量承接主业务交易；不做微信普通服务商分账灰度并行。
- 固定业务规则：开户验证费由平台承担；支付手续费 0.3% 由商户承担；分账后不退款；骑手必须开宝付个人二级户才可接收配送费分账。
- 账户边界：宝付收款商户号用于开户、转账、支付、分账；宝付支付商户号用于提现、提现查询，并承接平台预存的开户验证费。
- 分账接收方：已确认分账接口直接上送开户接口返回的二级商户号。本地用 `sharing_mer_id` 作为分账接收方规范字段；开户/查询解析层必须把宝付返回的二级商户号写入 `sharing_mer_id`，后续分账只读 `sharing_mer_id`。`contract_no` 只保留上游开户/查询字段和对账留痕，不作为分账创建兜底字段。不得使用微信 `openid`、微信报备 `subMchId` 或平台宝付收款商户号作为分账接收方。
- 平台佣金接收方：平台也必须为平台自己开一个平台名下宝付二级户并保存到 `owner_type=platform, owner_id=0`，不能直接使用平台宝付收款商户号收平台 2% 分账。

## 1. Target File Map

### 1.1 Backend Integration Boundary

- Create: `locallife/baofu/config.go` - environment/config validation, collect merchant and payout merchant separation.
- Create: `locallife/baofu/client.go` - top-level interfaces and concrete client composition.
- Create: `locallife/baofu/transport.go` - HTTP transport, request IDs, timeout, structured request metadata.
- Create: `locallife/baofu/signing.go` - aggregate payment request signing and callback verification.
- Create: `locallife/baofu/crypto/uniongw.go` - BaoCaiTong account API encryption/decryption/signing helpers.
- Create: `locallife/baofu/account/contracts/types.go` - open account, query account, balance, withdraw request/result DTOs.
- Create: `locallife/baofu/account/client.go` - account opening, query, balance, withdraw, withdraw query.
- Create: `locallife/baofu/account/notification/notification.go` - account and withdrawal callback parse/verify/normalize.
- Create: `locallife/baofu/aggregatepay/contracts/types.go` - unified order, payment query, share, share query, refund-before-share DTOs.
- Create: `locallife/baofu/aggregatepay/client.go` - payment and profit-sharing client implementation.
- Create: `locallife/baofu/aggregatepay/notification/notification.go` - payment and share callback parse/verify/normalize.
- Create: `locallife/baofu/mock/client.go` - test mock implementing account/payment interfaces.

### 1.2 Persistence And Generated Code

- Create: `locallife/db/migration/000227_add_baofu_payment_foundation.up.sql` - Baofu channel constraints, account bindings, fee ledger, withdrawal orders, profit sharing columns.
- Create: `locallife/db/migration/000227_add_baofu_payment_foundation.down.sql` - down migration for the same objects and constraints.
- Create: `locallife/db/query/baofu_account_binding.sql` - sqlc queries for account binding lifecycle.
- Create: `locallife/db/query/baofu_fee_ledger.sql` - sqlc queries for fee ledger writes and reads.
- Create: `locallife/db/query/baofu_withdrawal_order.sql` - sqlc queries for withdrawal lifecycle.
- Modify: `locallife/db/query/payment_order.sql` - Baofu paid-unprocessed selection and payment locking helpers.
- Modify: `locallife/db/query/profit_sharing_order.sql` - Baofu fee columns, receiver snapshot columns, retry selection, terminal update queries.
- Modify: `locallife/db/query/refund_order.sql` - refund-before-sharing guard queries.
- Modify: `locallife/db/sqlc/constants.go` - provider, channel, capability, command, fee, account and withdrawal constants.
- Regenerate: `locallife/db/sqlc/*.sql.go` via `make sqlc`.

### 1.3 Business Logic, API, Workers

- Create: `locallife/logic/baofu_account_service.go` - merchant/rider/operator/platform Baofu account orchestration.
- Create: `locallife/logic/baofu_payment_service.go` - unified order and payment query orchestration.
- Create: `locallife/logic/baofu_profit_sharing_service.go` - fee calculation, receiver resolution, share command creation.
- Create: `locallife/logic/baofu_withdraw_service.go` - balance and withdrawal orchestration.
- Create: `locallife/api/baofu_callback.go` - Baofu callback HTTP entrypoints that verify and persist facts.
- Modify: `locallife/api/payment_order.go` - create order payment through Baofu aggregate payment for main business orders.
- Modify: `locallife/api/merchant_finance.go` - include merchant-borne payment fee in finance detail.
- Modify: `locallife/api/rider_income.go` - include Baofu account readiness and withdraw state.
- Modify: `locallife/api/server.go` - register Baofu callback and finance routes.
- Create: `locallife/worker/task_baofu_payment_fact_application.go` - apply Baofu payment facts.
- Create: `locallife/worker/task_baofu_profit_sharing.go` - create share orders after refund window closes.
- Create: `locallife/worker/task_baofu_profit_sharing_fact_application.go` - apply share facts.
- Create: `locallife/worker/task_baofu_withdrawal_fact_application.go` - apply withdrawal facts.
- Create: `locallife/worker/baofu_payment_recovery_scheduler.go` - query payment/share/withdraw processing records.
- Modify: `locallife/worker/processor.go` - register Baofu tasks and schedulers.

### 1.4 Frontend Surfaces

- Modify: `weapp` payment caller paths that invoke backend payment creation - consume Baofu returned `wc_pay_data` without exposing Baofu terminology to users.
- Modify: `merchant_app/lib/features` finance/onboarding screens - display 宝付支付开通, 微信渠道报备, 结算账户可用, 手续费.
- Modify: `web/src` operator/merchant finance pages - add Baofu account status, fee ledger, withdrawal states and reconciliation views.

## 2. Task Cards

### Task 0: Constants, Channel Naming, And Migration Shell

**Files:**
- Create: `locallife/db/migration/000227_add_baofu_payment_foundation.up.sql`
- Create: `locallife/db/migration/000227_add_baofu_payment_foundation.down.sql`
- Modify: `locallife/db/sqlc/constants.go`
- Test: `locallife/db/sqlc/payment_order_channel_boundary_test.go`

- [x] **Step 1: Add constants first**

Add these constants to `locallife/db/sqlc/constants.go` near the existing payment constants:

```go
const (
    PaymentChannelBaofuAggregate = "baofu_aggregate"

    ExternalPaymentProviderBaofu = "baofu"

    ExternalPaymentCapabilityBaofuAccount       = "baofu_account"
    ExternalPaymentCapabilityBaofuPayment       = "baofu_payment"
    ExternalPaymentCapabilityBaofuProfitSharing = "baofu_profit_sharing"
    ExternalPaymentCapabilityBaofuWithdraw      = "baofu_withdraw"

    ExternalPaymentCommandTypeOpenBaofuAccount    = "open_baofu_account"
    ExternalPaymentCommandTypeQueryBaofuAccount   = "query_baofu_account"
    ExternalPaymentCommandTypeQueryBaofuBalance   = "query_baofu_balance"
    ExternalPaymentCommandTypeCreateBaofuWithdraw = "create_baofu_withdraw"
    ExternalPaymentCommandTypeQueryBaofuWithdraw  = "query_baofu_withdraw"

    BaofuAccountOwnerTypeMerchant = "merchant"
    BaofuAccountOwnerTypeRider    = "rider"
    BaofuAccountOwnerTypeOperator = "operator"
    BaofuAccountOwnerTypePlatform = "platform"

    BaofuAccountTypePersonal = "personal"
    BaofuAccountTypeBusiness = "business"
    BaofuAccountTypePlatform = "platform"

    BaofuAccountOpenStateProcessing = "processing"
    BaofuAccountOpenStateActive     = "active"
    BaofuAccountOpenStateFailed     = "failed"
    BaofuAccountOpenStateAbnormal   = "abnormal"

    BaofuFeeTypePaymentFee           = "payment_fee"
    BaofuFeeTypeAccountOpenVerifyFee = "account_open_verify_fee"
    BaofuFeePayerTypeMerchant        = "merchant"
    BaofuFeePayerTypePlatform        = "platform"

    BaofuWithdrawalStatusProcessing = "processing"
    BaofuWithdrawalStatusSucceeded  = "succeeded"
    BaofuWithdrawalStatusFailed     = "failed"
    BaofuWithdrawalStatusReturned   = "returned"
)
```

- [x] **Step 2: Add migration with hard constraints**

Create `locallife/db/migration/000227_add_baofu_payment_foundation.up.sql` with these objects:

```sql
ALTER TABLE payment_orders
    DROP CONSTRAINT IF EXISTS payment_orders_payment_channel_check;

ALTER TABLE payment_orders
    ADD CONSTRAINT payment_orders_payment_channel_check
    CHECK (payment_channel IN ('direct', 'ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

ALTER TABLE external_payment_commands
    DROP CONSTRAINT IF EXISTS external_payment_commands_channel_check;

ALTER TABLE external_payment_commands
    ADD CONSTRAINT external_payment_commands_channel_check
    CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

ALTER TABLE external_payment_facts
    DROP CONSTRAINT IF EXISTS external_payment_facts_channel_check;

ALTER TABLE external_payment_facts
    ADD CONSTRAINT external_payment_facts_channel_check
    CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

ALTER TABLE profit_sharing_orders
    ADD COLUMN IF NOT EXISTS payment_fee BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS payment_fee_rate_bps INTEGER NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'wechat',
    ADD COLUMN IF NOT EXISTS channel TEXT NOT NULL DEFAULT 'ecommerce',
    ADD COLUMN IF NOT EXISTS merchant_sharing_mer_id TEXT,
    ADD COLUMN IF NOT EXISTS rider_sharing_mer_id TEXT,
    ADD COLUMN IF NOT EXISTS operator_sharing_mer_id TEXT,
    ADD COLUMN IF NOT EXISTS platform_sharing_mer_id TEXT,
    ADD COLUMN IF NOT EXISTS sharing_detail_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_provider_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_provider_check CHECK (provider IN ('wechat', 'baofu'));

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_channel_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_channel_check CHECK (channel IN ('ecommerce', 'ordinary_service_provider', 'baofu_aggregate'));

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_payment_fee_check;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_payment_fee_check CHECK (payment_fee >= 0 AND payment_fee_rate_bps >= 0);

CREATE TABLE IF NOT EXISTS baofu_account_bindings (
    id BIGSERIAL PRIMARY KEY,
    owner_type TEXT NOT NULL,
    owner_id BIGINT NOT NULL,
    account_type TEXT NOT NULL,
    contract_no TEXT,
    sharing_mer_id TEXT,
    login_no TEXT,
    open_state TEXT NOT NULL DEFAULT 'processing',
    wechat_sub_mch_id TEXT,
    bank_card_last4 TEXT,
    last_open_trans_serial_no TEXT,
    raw_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT baofu_account_bindings_owner_type_check CHECK (owner_type IN ('merchant', 'rider', 'operator', 'platform')),
    CONSTRAINT baofu_account_bindings_account_type_check CHECK (account_type IN ('personal', 'business', 'platform')),
    CONSTRAINT baofu_account_bindings_open_state_check CHECK (open_state IN ('processing', 'active', 'failed', 'abnormal')),
    CONSTRAINT baofu_account_bindings_owner_uidx UNIQUE (owner_type, owner_id),
    CONSTRAINT baofu_account_bindings_contract_uidx UNIQUE (contract_no),
    CONSTRAINT baofu_account_bindings_sharing_uidx UNIQUE (sharing_mer_id),
    CONSTRAINT baofu_account_bindings_active_receiver_check CHECK (
        open_state <> 'active' OR length(trim(COALESCE(sharing_mer_id, ''))) > 0
    )
);

CREATE INDEX IF NOT EXISTS idx_baofu_account_bindings_open_state
    ON baofu_account_bindings(open_state, updated_at ASC, id ASC);

CREATE TABLE IF NOT EXISTS baofu_fee_ledger (
    id BIGSERIAL PRIMARY KEY,
    fee_type TEXT NOT NULL,
    payer_type TEXT NOT NULL,
    payer_id BIGINT,
    business_object_type TEXT NOT NULL,
    business_object_id BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    fee_rate_bps INTEGER,
    provider_bill_no TEXT,
    status TEXT NOT NULL DEFAULT 'recorded',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT baofu_fee_ledger_fee_type_check CHECK (fee_type IN ('payment_fee', 'account_open_verify_fee')),
    CONSTRAINT baofu_fee_ledger_payer_type_check CHECK (payer_type IN ('merchant', 'platform')),
    CONSTRAINT baofu_fee_ledger_status_check CHECK (status IN ('recorded', 'reconciled', 'adjusted')),
    CONSTRAINT baofu_fee_ledger_amount_check CHECK (amount >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_baofu_fee_ledger_business_uidx
    ON baofu_fee_ledger(fee_type, business_object_type, business_object_id);

CREATE TABLE IF NOT EXISTS baofu_withdrawal_orders (
    id BIGSERIAL PRIMARY KEY,
    owner_type TEXT NOT NULL,
    owner_id BIGINT NOT NULL,
    account_binding_id BIGINT NOT NULL REFERENCES baofu_account_bindings(id),
    out_request_no TEXT NOT NULL,
    baofu_withdraw_no TEXT,
    amount BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'processing',
    raw_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT baofu_withdrawal_orders_owner_type_check CHECK (owner_type IN ('merchant', 'rider', 'operator', 'platform')),
    CONSTRAINT baofu_withdrawal_orders_status_check CHECK (status IN ('processing', 'succeeded', 'failed', 'returned')),
    CONSTRAINT baofu_withdrawal_orders_amount_check CHECK (amount > 0),
    CONSTRAINT baofu_withdrawal_orders_out_request_no_uidx UNIQUE (out_request_no)
);

CREATE INDEX IF NOT EXISTS idx_baofu_withdrawal_orders_owner
    ON baofu_withdrawal_orders(owner_type, owner_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_baofu_withdrawal_orders_status
    ON baofu_withdrawal_orders(status, created_at ASC, id ASC);
```

- [x] **Step 3: Add down migration**

Create `locallife/db/migration/000227_add_baofu_payment_foundation.down.sql`:

```sql
DROP TABLE IF EXISTS baofu_withdrawal_orders;
DROP TABLE IF EXISTS baofu_fee_ledger;
DROP TABLE IF EXISTS baofu_account_bindings;

ALTER TABLE profit_sharing_orders
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_payment_fee_check,
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_channel_check,
    DROP CONSTRAINT IF EXISTS profit_sharing_orders_provider_check;

ALTER TABLE profit_sharing_orders
    DROP COLUMN IF EXISTS sharing_detail_snapshot,
    DROP COLUMN IF EXISTS platform_sharing_mer_id,
    DROP COLUMN IF EXISTS operator_sharing_mer_id,
    DROP COLUMN IF EXISTS rider_sharing_mer_id,
    DROP COLUMN IF EXISTS merchant_sharing_mer_id,
    DROP COLUMN IF EXISTS channel,
    DROP COLUMN IF EXISTS provider,
    DROP COLUMN IF EXISTS payment_fee_rate_bps,
    DROP COLUMN IF EXISTS payment_fee;

ALTER TABLE external_payment_facts
    DROP CONSTRAINT IF EXISTS external_payment_facts_channel_check;

ALTER TABLE external_payment_facts
    ADD CONSTRAINT external_payment_facts_channel_check
    CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider'));

ALTER TABLE external_payment_commands
    DROP CONSTRAINT IF EXISTS external_payment_commands_channel_check;

ALTER TABLE external_payment_commands
    ADD CONSTRAINT external_payment_commands_channel_check
    CHECK (channel IN ('direct', 'ecommerce', 'ordinary_service_provider'));

ALTER TABLE payment_orders
    DROP CONSTRAINT IF EXISTS payment_orders_payment_channel_check;

ALTER TABLE payment_orders
    ADD CONSTRAINT payment_orders_payment_channel_check
    CHECK (payment_channel IN ('direct', 'ecommerce', 'ordinary_service_provider'));
```

- [x] **Step 4: Regenerate and run focused DB checks**

Run from `locallife/`:

```bash
make sqlc
make check-generated
go test ./db/sqlc -run 'TestPaymentOrderChannelBoundary|TestExternalPaymentFact' -count=1
```

Expected: `make sqlc` regenerates sqlc files, `make check-generated` exits 0, focused tests exit 0.

### Task 1: Baofu Base Module, Config, Signing, And Crypto

**Files:**
- Create: `locallife/baofu/config.go`
- Create: `locallife/baofu/client.go`
- Create: `locallife/baofu/transport.go`
- Create: `locallife/baofu/signing.go`
- Create: `locallife/baofu/crypto/uniongw.go`
- Test: `locallife/baofu/config_test.go`
- Test: `locallife/baofu/signing_test.go`
- Test: `locallife/baofu/crypto/uniongw_test.go`

- [x] **Step 1: Define config with two merchant IDs**

Implement `Config` with these explicit fields:

```go
type Config struct {
    BaseURL           string
    CollectMerchantID string
    CollectTerminalID string
    PayoutMerchantID  string
    PayoutTerminalID  string
    AppID             string
    PrivateKeyPEM     string
    BaofuPublicKeyPEM string
    AESKey            string
    NotifyBaseURL     string
    Timeout           time.Duration
}

func (c Config) Validate() error {
    if strings.TrimSpace(c.CollectMerchantID) == "" {
        return errors.New("baofu collect merchant id is required")
    }
    if strings.TrimSpace(c.PayoutMerchantID) == "" {
        return errors.New("baofu payout merchant id is required")
    }
    if c.CollectMerchantID == c.PayoutMerchantID {
        return errors.New("baofu collect merchant id and payout merchant id must be different")
    }
    return nil
}
```

- [x] **Step 2: Write tests for merchant boundary**

Add tests asserting:

```go
func TestConfigValidateRequiresSeparatedMerchants(t *testing.T) {
    cfg := validBaofuConfigForTest()
    cfg.PayoutMerchantID = cfg.CollectMerchantID
    require.EqualError(t, cfg.Validate(), "baofu collect merchant id and payout merchant id must be different")
}
```

- [x] **Step 3: Implement HTTP transport and signing helpers**

Implement deterministic request serialization, signature generation, signature verification, and sanitized logging. Log metadata must include provider `baofu`, capability, out request number, HTTP status, and upstream code; it must not log ID card numbers, bank cards, phone numbers, plaintext AES keys, or private keys.

- [x] **Step 4: Implement union-gw crypto helpers**

Implement helpers for BaoCaiTong account API encryption/decryption and signature wrapping:

```go
type UnionGWEnvelope struct {
    MerchantID string `json:"merchantNo"`
    TerminalID string `json:"terminalNo"`
    DataType   string `json:"dataType"`
    Data       string `json:"data"`
    Sign       string `json:"sign"`
}
```

Tests must cover round-trip encrypt/decrypt with fixture keys and reject tampered signatures.

- [x] **Step 5: Run focused package tests**

Run from `locallife/`:

```bash
go test ./baofu ./baofu/crypto ./baofu/account ./baofu/account/contracts ./baofu/account/notification ./baofu/aggregatepay ./baofu/aggregatepay/contracts ./baofu/aggregatepay/notification -count=1
```

Expected: all Baofu package tests pass.

### Task 2: Account Contracts, Persistence, Opening, Query, And Notifications

**Files:**
- Create: `locallife/baofu/account/contracts/types.go`
- Create: `locallife/baofu/account/client.go`
- Create: `locallife/baofu/account/notification/notification.go`
- Create: `locallife/db/query/baofu_account_binding.sql`
- Create: `locallife/logic/baofu_account_service.go`
- Create: `locallife/api/baofu_callback.go`
- Modify: `locallife/api/server.go`
- Test: `locallife/baofu/account/contracts/types_test.go`
- Test: `locallife/logic/baofu_account_service_test.go`
- Test: `locallife/api/baofu_callback_test.go`

- [x] **Step 1: Add sqlc account binding queries**

Create `locallife/db/query/baofu_account_binding.sql` with queries for upsert by owner, get by owner, get by contract, mark processing, mark active, mark failed, list processing for recovery. `MarkBaofuAccountBindingActive` must write `sharing_mer_id` with the开户接口返回的二级商户号；`contract_no` 仅保留上游开户/查询字段和对账留痕，不作为分账创建兜底字段。

- [x] **Step 2: Define account contract DTOs**

Define normalized project-level types:

```go
type OwnerType string

type OpenAccountRequest struct {
    OwnerType       string
    OwnerID         int64
    AccountType     string
    OutRequestNo    string
    LegalName       string
    CertificateNo   string
    BankAccountNo   string
    BankMobile      string
    WechatSubMchID  string
}

type AccountResult struct {
    OutRequestNo  string
    ContractNo    string
    SharingMerID  string
    OpenState     string
    UpstreamState string
    FailCode      string
    FailMessage   string
    Raw           json.RawMessage
}
```

- [x] **Step 3: Enforce owner/account rules in service**

`locallife/logic/baofu_account_service.go` must enforce:

```text
merchant -> business account required
operator -> business or platform account allowed
platform -> platform account required
rider -> personal account required
active account -> sharing_mer_id required
rider active account -> no wechat_sub_mch_id requirement
merchant active account -> wechat_sub_mch_id required before payment creation, not before account opening
```

- [x] **Step 4: Record command for account opening**

Before calling Baofu open account, insert `external_payment_commands` with provider `baofu`, channel `baofu_aggregate`, capability `baofu_account`, command type `open_baofu_account`, business owner matching the local owner, and external key `out_request_no`.

- [x] **Step 5: Normalize account notifications into facts**

`locallife/api/baofu_callback.go` registers the account callback on the existing webhook namespace:

```text
POST /v1/webhooks/baofu/account/open
```

The account callback handler verifies/decrypts via `locallife/baofu/account/notification`, inserts an `external_payment_facts` row, and returns Baofu ACK only after persistence succeeds. Payment, profit-sharing, and withdrawal callback routes remain in their own payment/withdraw task cards so Task 2 does not expose unimplemented money-movement callbacks as successful paths.

- [x] **Step 6: Validate account path**

Run from `locallife/`:

```bash
make sqlc
go test ./baofu/account ./baofu/account/contracts ./baofu/account/notification ./logic ./api -run 'TestBaofuAccount|TestBaofuCallback' -count=1
make check-generated
```

Expected: account service rejects invalid owner/account combinations, callback duplicates dedupe by fact key, generated code is clean.

### Task 3: Merchant, Rider, Operator, And Platform Onboarding Propagation

**Files:**
- Modify: `locallife/api/merchant_application.go`
- Modify: `locallife/api/rider_application.go`
- Modify: `locallife/api/operator_application.go`
- Modify: `locallife/api/profit_sharing_capability.go`
- Modify: `locallife/worker/task_onboarding_review.go`
- Modify: `weapp` rider onboarding API usage paths
- Modify: `merchant_app/lib/features` merchant onboarding paths
- Modify: `web/src` operator onboarding and finance admin paths
- Test: `locallife/api/profit_sharing_capability_test.go`
- Test: `locallife/worker/task_onboarding_review_test.go`

- [x] **Step 1: Add backend readiness checks**

Extend account readiness so a business owner is considered Baofu-ready only when:

```text
baofu_account_bindings.open_state = active
sharing_mer_id is present
merchant payment creation additionally has wechat_sub_mch_id present
```

- [x] **Step 2: Block rider assignment without active Baofu account**

In delivery/rider eligibility code, reject riders whose `baofu_account_bindings` row is missing or not active. User-facing Chinese copy: `骑手结算账户未开通，暂不能接收配送费分账订单`.

- [ ] **Step 3: Surface onboarding states**

Expose states to clients as product terms:

```text
资料待提交
宝付开户处理中
微信渠道待报备
结算账户可用
开通失败
```

Do not expose `contractNo`, `sharingMerId`, or raw upstream error payloads to ordinary users. Operator/admin views may display masked IDs.

Current backend status: rider status now returns sanitized `settlement_account` readiness (`state`, `label`, `payment_ready`) and aligns `can_go_online` / `online_block_reason` with the same Baofu settlement guard. Merchant open status now returns the same sanitized readiness shape for the merchant Baofu settlement account. Operator application status now returns sanitized readiness after the approved application has a formal operator account. Platform finance status now returns sanitized readiness for the platform singleton account (`owner_type=platform`, `owner_id=0`). Frontend surfaces remain in later Task 3 slices.
Merchant open status now also checks Baofu merchant readiness before allowing open-for-business: ordinary service provider channel identity must be active, Baofu merchant account binding must be active, and the binding must carry a WeChat channel identity. Operator readiness display does not require a WeChat channel identity because operator commission is received into the Baofu secondary account.
Single-order and combined-payment main-business payment creation now check merchant Baofu readiness before creating new local payment rows or calling the upstream payment API. Combined payment uses a pre-transaction order-to-merchant lookup so an unready merchant cannot leave local pending child payment rows behind.

- [ ] **Step 4: Validate API and onboarding workers**

Run from `locallife/`:

```bash
go test ./api ./worker -run 'TestProfitSharingCapability|TestOnboardingReview|TestRider' -count=1
```

Expected: merchant readiness requires Baofu active account plus channel report identity, rider readiness requires Baofu active personal account.

### Task 4: Baofu Aggregate Payment Unified Order And Payment Facts

**Files:**
- Create: `locallife/baofu/aggregatepay/contracts/types.go`
- Create: `locallife/baofu/aggregatepay/client.go`
- Create: `locallife/baofu/aggregatepay/notification/notification.go`
- Create: `locallife/logic/baofu_payment_service.go`
- Modify: `locallife/api/payment_order.go`
- Create: `locallife/worker/task_baofu_payment_fact_application.go`
- Test: `locallife/baofu/aggregatepay/contracts/types_test.go`
- Test: `locallife/logic/baofu_payment_service_test.go`
- Test: `locallife/worker/task_baofu_payment_fact_application_test.go`

- [x] **Step 1: Define unified order request**

Baofu payment service must create aggregate payment with:

```text
prodType = SHARING
orderType = 7
payCode = WECHAT_JSAPI
payExtend.sub_appid = platform mini program appid
payExtend.sub_openid = paying user's openid
subMchId = merchant channel report ID
```

`sub_openid` is payer identity only. It must never be written as a profit-sharing receiver.

- [ ] **Step 2: Route main business order payment to Baofu**

For main business orders, create `payment_orders` with:

```text
payment_channel = baofu_aggregate
requires_profit_sharing = true
payment_type = profit_sharing
business_type = order
```

Return `chlRetParam.wc_pay_data` to the mini program in the existing payment response envelope.

- [ ] **Step 3: Persist payment commands and facts**

For create-payment command use provider `baofu`, channel `baofu_aggregate`, capability `baofu_payment`, command type `create_payment`, external object type `baofu_payment_order`, external key `out_trade_no`. Payment callbacks and payment queries insert facts with terminal status mapping:

```text
WAIT_PAYING -> processing
SUCCESS -> success
CLOSED -> closed
PAY_ERROR -> failed
REFUND -> success only for pre-share refund facts handled by refund path
ABNORMAL -> unknown
```

- [ ] **Step 4: Apply payment success idempotently**

`task_baofu_payment_fact_application.go` must lock `payment_orders` by out trade number, update pending to paid once, store Baofu transaction ID, and enqueue outbox event for profit sharing. Duplicate success facts become ignored applications.

- [ ] **Step 5: Validate payment path**

Run from `locallife/`:

```bash
go test ./baofu/aggregatepay ./baofu/aggregatepay/contracts ./baofu/aggregatepay/notification ./logic ./api ./worker -run 'TestBaofuPayment|TestPaymentOrder|TestBaofuPaymentFact' -count=1
```

Expected: payment creation refuses missing merchant Baofu readiness, stores command, returns WeChat JSAPI payload, and duplicate callbacks do not double-apply.

### Task 5: Profit Sharing Calculation, Fee Ledger, And Receiver Snapshot

**Files:**
- Create: `locallife/logic/baofu_profit_sharing_service.go`
- Create: `locallife/db/query/baofu_fee_ledger.sql`
- Modify: `locallife/db/query/profit_sharing_order.sql`
- Modify: `locallife/api/merchant_finance.go`
- Test: `locallife/logic/baofu_profit_sharing_service_test.go`
- Test: `locallife/db/sqlc/profit_sharing_order_test.go`
- Test: `locallife/api/merchant_finance_test.go`

- [x] **Step 1: Implement deterministic fee calculation**

Use basis points and fen integer math:

```go
func CalculateBaofuPaymentFeeFen(totalAmountFen int64) int64 {
    return (totalAmountFen*30 + 9999) / 10000
}
```

This rounds 0.3% up to fen so the merchant deduction is never lower than configured fee. Store `payment_fee_rate_bps = 30` and `payment_fee` on `profit_sharing_orders`.

- [x] **Step 2: Implement split formula**

For an order paid 10000 fen with delivery fee 500 fen, assert:

```text
rider_amount = 500
platform_commission = 200
operator_commission = 300
payment_fee = 30
merchant_amount = 8970
```

Service must reject negative merchant amount before writing a share order.

- [x] **Step 3: Resolve receivers from Baofu bindings**

Resolve merchant, rider, operator and platform receiver IDs from `baofu_account_bindings.sharing_mer_id`. Do not fall back to `contract_no` at分账创建 time;开户/查询同步层 must already normalize the returned二级商户号 into `sharing_mer_id`. Store the resolved IDs in `profit_sharing_orders.*_sharing_mer_id` and in `sharing_detail_snapshot`.

- [x] **Step 4: Record fee ledger rows**

For every Baofu profit-sharing order, insert one payment-fee ledger row:

```text
fee_type = payment_fee
payer_type = merchant
payer_id = merchant_id
business_object_type = payment_order
business_object_id = payment_order_id
amount = payment_fee
fee_rate_bps = 30
```

For every successful account-open result, insert one account-open ledger row:

```text
fee_type = account_open_verify_fee
payer_type = platform
payer_id = null
business_object_type = baofu_account_binding
business_object_id = baofu_account_bindings.id
amount = 100
```

- [x] **Step 5: Validate calculation and finance display**

Run from `locallife/`:

```bash
make sqlc
go test ./logic ./api ./db/sqlc -run 'TestBaofuProfitSharing|TestMerchantFinance|TestProfitSharingOrder' -count=1
make check-generated
```

Expected: merchant finance totals show platform/operator fees and Baofu payment fee as separate deductions; platform 2% is not reduced by the payment fee.

### Task 6: Confirm Profit Sharing, Notification, Query, And Recovery

**Files:**
- Modify: `locallife/baofu/aggregatepay/client.go`
- Modify: `locallife/baofu/aggregatepay/notification/notification.go`
- Create: `locallife/worker/task_baofu_profit_sharing.go`
- Create: `locallife/worker/task_baofu_profit_sharing_fact_application.go`
- Create: `locallife/worker/baofu_payment_recovery_scheduler.go`
- Modify: `locallife/worker/processor.go`
- Test: `locallife/worker/task_baofu_profit_sharing_test.go`
- Test: `locallife/worker/task_baofu_profit_sharing_fact_application_test.go`

- [ ] **Step 1: Gate share creation on refund-closed state**

Share worker only selects paid Baofu orders when the related order is terminal for refund purposes:

```text
order status = completed
refund window closed according to business policy
no refund_orders row in pending, processing, success
no existing profit_sharing_orders row for payment_order_id
```

- [ ] **Step 2: Create `share_after_pay` command**

Call Baofu confirm profit sharing with original Baofu payment reference, local unique share out order number, and `sharingDetails` generated from `sharing_detail_snapshot`. Command row uses capability `baofu_profit_sharing` and command type `create_profit_sharing`.

- [ ] **Step 3: Map share states**

Fact mapping:

```text
PROCESSING -> processing
SUCCESS -> success
CANCELED -> failed
ABNORMAL -> unknown
```

Only `SUCCESS` marks `profit_sharing_orders.status = finished`. `PROCESSING` stays processing and is picked by recovery query.

- [ ] **Step 4: Add recovery scheduler**

Scheduler scans:

```text
payment_orders baofu_aggregate pending past expected callback time -> query payment
profit_sharing_orders provider baofu status processing -> query share
baofu_withdrawal_orders status processing -> query withdraw
```

Each query result inserts an `external_payment_facts` row, then fact application advances business state.

- [ ] **Step 5: Validate share workflow**

Run from `locallife/`:

```bash
go test ./worker ./logic -run 'TestBaofuProfitSharing|TestBaofuPaymentRecovery' -count=1
```

Expected: share command is emitted once, processing query is retryable, success fact terminalizes the share order once.

### Task 7: Refund-Before-Sharing Only And Concurrency Guard

**Files:**
- Modify: `locallife/db/query/refund_order.sql`
- Modify: `locallife/db/query/payment_order.sql`
- Modify: `locallife/logic` refund service paths
- Modify: `locallife/worker/refund_recovery_scheduler.go`
- Test: `locallife/db/sqlc/profit_sharing_order_test.go`
- Test: `locallife/worker/task_process_payment_ecommerce_refund_command_test.go`
- Test: `locallife/api/payment_order_test.go`

- [ ] **Step 1: Add refund guard query**

Add a sqlc query that locks payment order and checks no share has started:

```sql
-- name: GetBaofuPaymentOrderRefundGuardForUpdate :one
SELECT po.id,
       po.status,
       po.payment_channel,
       EXISTS (
           SELECT 1 FROM profit_sharing_orders pso
           WHERE pso.payment_order_id = po.id
             AND pso.status IN ('pending', 'processing', 'finished')
       ) AS has_started_profit_sharing
FROM payment_orders po
WHERE po.id = $1
FOR UPDATE;
```

- [ ] **Step 2: Reject refund after share starts**

Refund API and worker must return a business error when `has_started_profit_sharing = true`. Chinese user copy: `订单已进入结算分账流程，不支持退款`.

- [ ] **Step 3: Keep Baofu split-refund out of first version**

Do not call Baofu `sharingRefundInfo`, `part_share_refund_info`, or `advanceAmt` fields in first version. If a payment was refunded before sharing, mark payment/refund facts and prevent share order creation.

- [ ] **Step 4: Validate race safety**

Run from `locallife/` with a DB-backed test database:

```bash
go test ./db/sqlc ./worker ./api -run 'TestBaofuRefund|TestProfitSharingOrder|TestRefund' -count=1
```

Expected: concurrent refund and share creation leaves exactly one path accepted; share accepted blocks refund, refund accepted blocks share.

### Task 8: Balance And Withdrawal With Payout Merchant ID

**Files:**
- Modify: `locallife/baofu/account/client.go`
- Create: `locallife/db/query/baofu_withdrawal_order.sql`
- Create: `locallife/logic/baofu_withdraw_service.go`
- Create: `locallife/worker/task_baofu_withdrawal_fact_application.go`
- Modify: `locallife/api/merchant_finance.go`
- Modify: `locallife/api/rider_income.go`
- Test: `locallife/logic/baofu_withdraw_service_test.go`
- Test: `locallife/worker/task_baofu_withdrawal_fact_application_test.go`

- [ ] **Step 1: Enforce payout merchant for withdrawal**

`CreateWithdraw` and `QueryWithdraw` must use `Config.PayoutMerchantID` and `Config.PayoutTerminalID`. Account open, balance query, payment and sharing must not use payout merchant credentials.

- [ ] **Step 2: Implement balance query**

Return both in-transit and available balances when Baofu provides them. API response labels:

```text
待结算金额
可提现金额
提现中金额
已提现金额
```

- [ ] **Step 3: Implement withdrawal lifecycle**

Create `baofu_withdrawal_orders` row before calling Baofu withdrawal. Synchronous accepted result leaves status `processing`. Callback/query facts map:

```text
1 -> succeeded
0 -> failed
2 -> processing
3 -> returned
```

- [ ] **Step 4: Validate payout boundary**

Run from `locallife/`:

```bash
make sqlc
go test ./baofu/account ./logic ./worker ./api -run 'TestBaofuWithdraw|TestMerchantFinance|TestRiderIncome' -count=1
make check-generated
```

Expected: tests prove withdrawal requests use payout merchant ID and account/payment/share requests use collect merchant ID.

### Task 9: Reconciliation, Observability, And Production First-Order Checklist

**Files:**
- Modify: `locallife/api/platform_finance.go`
- Modify: `locallife/worker/alert_payloads.go`
- Modify: `locallife/worker/platform_alert_history.go`
- Create: `artifacts/baofu-payment/baofu-production-first-order-checklist.md`
- Test: `locallife/api/platform_stats_test.go`
- Test: `locallife/worker/alert_payloads_test.go`

- [ ] **Step 1: Add reconciliation views**

Platform finance should expose daily sums by provider/channel:

```text
paid_amount
payment_fee
merchant_amount
rider_amount
platform_commission
operator_commission
withdraw_succeeded_amount
withdraw_processing_amount
unapplied_fact_count
unknown_command_count
```

- [ ] **Step 2: Add alerts**

Alert when:

```text
Baofu payment callback missing after configured SLA
Baofu share processing exceeds configured SLA
Baofu withdrawal processing exceeds configured SLA
external_payment_facts processing_status = failed
baofu_fee_ledger amount mismatch against profit_sharing_orders.payment_fee
```

- [ ] **Step 3: Write production first-order checklist**

Create `artifacts/baofu-payment/baofu-production-first-order-checklist.md` with check items for:

```text
collect merchant balance and config verified
payout merchant prefunded for account opening fee
merchant Baofu account active
rider Baofu account active
merchant WeChat subMchId present
mini program payment succeeds
payment callback persisted
share order created after refund window closes
share callback persisted
merchant/rider/operator/platform balances match expected formula
withdraw test amount succeeds
```

- [ ] **Step 4: Validate finance and alert views**

Run from `locallife/`:

```bash
go test ./api ./worker -run 'TestPlatformStats|TestAlertPayload|TestBaofu' -count=1
```

Expected: metrics and alerts include provider/channel labels and never include plaintext PII.

### Task 10: Frontend Updates For Payment, Onboarding, Funds, And Withdrawals

**Files:**
- Modify: `weapp` payment submit pages and API adapters
- Modify: `merchant_app/lib/features` onboarding, finance, withdrawal views
- Modify: `web/src` operator/merchant/platform finance and account status views
- Test: app-specific existing lint/build/test targets

- [ ] **Step 1: Mini program payment response handling**

Mini program must pass backend Baofu-provided WeChat payment payload into the existing `wx.requestPayment` flow. It must not construct nonce, package, sign type, or pay sign locally.

- [ ] **Step 2: Merchant app onboarding and finance copy**

Use these Chinese labels consistently:

```text
宝付支付开通
微信渠道报备
结算账户可用
支付手续费
待结算金额
可提现金额
提现中
提现成功
提现退回
```

- [ ] **Step 3: Rider onboarding and income gating**

When rider Baofu account is not active, display: `结算账户未开通，暂不能接收配送费分账订单` and hide withdrawal action.

- [ ] **Step 4: Web operator/platform pages**

Operator/admin views display masked `contract_no`, masked `sharing_mer_id`, fee ledger rows, withdrawal order states, and reconciliation anomaly counts.

- [ ] **Step 5: Validate frontend surfaces**

Run the smallest relevant commands from each changed app:

```bash
cd weapp && npm run lint
cd merchant_app && flutter analyze
cd web && npm run lint
```

Expected: changed app commands exit 0. If a toolchain is unavailable on the implementation machine, record the missing executable and run the corresponding command in CI before production release.

## 3. Cross-Task Invariants

- No code path writes `openid`, `sub_openid`, `subMchId`, or the platform Baofu collect merchant ID into `sharing_mer_id`.
- No withdrawal request uses the collect merchant ID.
- No account opening, payment, or profit sharing request uses the payout merchant ID.
- No Baofu callback mutates business state before its verified payload is persisted to `external_payment_facts`.
- No share order is created while a refund is pending or while the order can still be refunded by product policy.
- No refund starts after a share order reaches pending, processing, or finished.
- No merchant finance view subtracts the 0.3% Baofu fee from platform 2% or operator 3%; it subtracts the fee from merchant net revenue.
- No ordinary user-facing API response exposes raw upstream payload, full bank card, full ID card, private key, AES key, or signature material.

## 4. Regeneration And Validation Matrix

| Change | Required regeneration | Focused validation |
| --- | --- | --- |
| `locallife/db/migration/*.sql`, `locallife/db/query/*.sql` | `make sqlc`, `make check-generated` | `go test ./db/sqlc -run 'TestBaofu|TestProfitSharing|TestPaymentOrder' -count=1` |
| Swagger annotations or new public routes | `make swagger`, `make check-generated` | `go test ./api -run 'TestBaofu|TestPaymentOrder|TestMerchantFinance|TestRiderIncome' -count=1` |
| Baofu client/contracts | none | `go test ./baofu ./baofu/crypto ./baofu/account ./baofu/account/contracts ./baofu/account/notification ./baofu/aggregatepay ./baofu/aggregatepay/contracts ./baofu/aggregatepay/notification -count=1` |
| Payment/profit-sharing workers | none | `go test ./worker -run 'TestBaofu|TestPaymentFact|TestProfitSharing|TestWithdraw' -count=1` |
| Main business payment path | possibly `make swagger` when API shape changes | `make test-safety` plus focused payment tests |
| Frontend payment and finance views | app build/lint | `npm run lint`, `flutter analyze`, or app-specific equivalent |

Before production release, run from `locallife/`:

```bash
make test-safety
make check-generated
```

For a full backend confidence pass when DB and services are available:

```bash
make test-unit
make test-integration
```

## 5. Implementation Order And Commit Boundaries

1. Commit Task 0 alone: `feat(payment): add baofu payment persistence foundation`.
2. Commit Task 1 alone: `feat(baofu): add client config and signing foundation`.
3. Commit Task 2 alone: `feat(baofu): add account opening lifecycle`.
4. Commit Task 3 alone: `feat(onboarding): require baofu settlement accounts`.
5. Commit Task 4 alone: `feat(payment): create baofu aggregate payments`.
6. Commit Task 5 alone: `feat(payment): calculate baofu merchant-borne fees`.
7. Commit Task 6 alone: `feat(payment): process baofu profit sharing`.
8. Commit Task 7 alone: `fix(payment): enforce refund before baofu sharing`.
9. Commit Task 8 alone: `feat(payment): add baofu balance and withdrawal`.
10. Commit Task 9 alone: `feat(finance): add baofu reconciliation and alerts`.
11. Commit Task 10 alone: `feat(ui): surface baofu payment and settlement states`.

## 6. Release Gate

- Baofu sandbox first: one merchant, one rider, one operator, one platform revenue receiver, one complete payment-share-withdraw loop.
- Production first order: execute `artifacts/baofu-payment/baofu-production-first-order-checklist.md` with finance/operator present.
- Rollback approach: disable Baofu payment creation at configuration/routing level before any new payment is created. Existing paid orders must continue through Baofu share/withdraw recovery; do not route an already-paid Baofu order into WeChat ordinary service provider sharing.
- Operational ownership: payment callbacks, share callbacks, withdraw callbacks, and recovery schedulers must have alert owners and log search queries before traffic is enabled.


## 7. Execution Log

### 2026-05-03 Task 0

- Implemented Baofu payment foundation constants in `locallife/db/sqlc/constants.go`.
- Added migration `locallife/db/migration/000227_add_baofu_payment_foundation.up.sql` and matching down migration.
- Extended `profit_sharing_orders` query projections so regenerated sqlc methods keep returning `db.ProfitSharingOrder` where existing worker code expects it.
- Added `TestBaofuPaymentConstantsAreExplicit`, `TestCreateExternalPaymentCommand_AcceptsBaofuAggregateChannel`, and `TestCreateExternalPaymentFact_AcceptsBaofuAggregateChannel` to lock provider, channel, capability strings, and DB channel constraints.
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" make sqlc`; `PATH="/usr/local/go/bin:$PATH" make check-generated`; `PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -run 'TestPaymentOrderChannelBoundary|TestExternalPaymentFact|TestBaofuPaymentConstantsAreExplicit|TestCreateExternalPayment(Command|Fact)_AcceptsBaofuAggregateChannel' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -run 'TestPaymentOrderChannelBoundary|TestExternalPaymentFact' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./worker -run '^$' -count=1`; `git diff --check`.
- Additional lint attempt: `PATH="/usr/local/go/bin:$PATH" make lint-filesize` still fails on 71 pre-existing oversized Go files; Task 0 does not add or enlarge those files.


### 2026-05-03 Task 1

- Added `locallife/baofu` foundation with config validation, explicit collect/payout merchant separation, client construction, sanitized request metadata, provider error logging boundary, canonical JSON, and RSA SHA256 signing helpers.
- Added `locallife/baofu/crypto` union-gw envelope foundation with AES-GCM payload encryption and envelope signature verification; tampered envelopes return `ErrInvalidEnvelopeSignature`.
- Added placeholder package roots for account and aggregate payment submodules so subsequent task cards can add contracts without changing package layout.
- Added tests for config merchant separation, default normalization, deterministic JSON, RSA sign/verify, sensitive log field redaction, union-gw round trip, and tamper rejection.
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" gofmt -w locallife/baofu`; `PATH="/usr/local/go/bin:$PATH" go test ./baofu ./baofu/crypto ./baofu/account ./baofu/account/contracts ./baofu/account/notification ./baofu/aggregatepay ./baofu/aggregatepay/contracts ./baofu/aggregatepay/notification -count=1`; `git diff --check`.

### 2026-05-03 Task 2

- Added BaoCaiTong account binding sqlc queries for owner upsert, owner/contract lookup, active/failed/processing transitions, and stale processing scans; `make sqlc` regenerated `db/sqlc`, `db/mock`, `worker/mock`, and WeChat mocks.
- Added Baofu account contracts and notification parsing with normalized `contractNo` / `sharingMerId` handling, upstream state mapping, union-gw decrypt/verify boundary, and missing-codec rejection.
- Added `logic.BaofuAccountService` to enforce owner/account-type rules, merchant WeChat channel identity readiness, receiver readiness, and command-before-client-call audit persistence for account opening.
- Added `POST /v1/webhooks/baofu/account/open` callback handling; it persists an `external_payment_facts` callback fact before ACK and keeps unresolved `business_object_type/id` null to satisfy the external fact pair constraint until a later application worker resolves the local binding.
- Review/fix notes: fixed the callback fact business object pair so DB constraints cannot reject account callbacks without a known binding ID; defaulted empty account raw snapshots to `{}` for JSONB safety; added a parser configuration guard.
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" make sqlc`; `PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc ./baofu/account ./baofu/account/contracts ./baofu/account/notification ./logic ./api -run 'TestBaofuAccount|TestBaofuCallback' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -run 'TestBaofuAccountBinding|TestMarkBaofuAccountBinding' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./baofu/account ./baofu/account/contracts ./baofu/account/notification ./logic ./api -count=1`; `PATH="/usr/local/go/bin:$PATH" make check-generated`; `git diff --check`.
- Additional lint attempt: `PATH="/usr/local/go/bin:$PATH" make lint-filesize` still fails on 71 pre-existing oversized Go files; Task 2 new files are under the 500-line guardrail.
- Residual risk: Task 2 stops at account opening contracts, persistence, command creation, and account callback fact capture. It does not yet wire runtime Baofu config into `NewServer`, does not call real BaoCaiTong account transport, and does not apply callback facts into `baofu_account_bindings`; those remain assigned to later task cards before production traffic.

### 2026-05-03 Task 3 Partial - Rider Online Readiness

- Added the first onboarding propagation guard: `POST /v1/rider/online` now requires an active rider BaoCaiTong personal account with `sharing_mer_id` before the rider can go online to receive delivery-fee profit-sharing orders.
- The public error is semantic and product-facing: `骑手结算账户未开通，暂不能接收配送费分账订单`; internal storage errors still flow to the existing logged internal-error boundary.
- Added rider API regression coverage for a missing Baofu account and updated successful online cases to require an active Baofu rider binding.
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestGoOnlineAPI/BaofuAccountMissing|TestGoOnlineAPI/OK|TestGoOnlineAPI/ApprovedRiderPromotedByCurrentRegionDeposit' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestGoOnlineAPI' -count=1`; `git diff --check`.
- Additional lint attempt: `PATH="/usr/local/go/bin:$PATH" make lint-filesize` still fails on 71 pre-existing oversized Go files; this partial moved the Baofu readiness helper to a new small file and only added the necessary call site to the existing `api/rider.go`.
- Residual risk: this is only Task 3's rider-online guard. Merchant/operator/platform onboarding surfaces, worker propagation, rider status display, and delivery assignment/query paths still need the remaining Task 3 work before the Baofu readiness story is complete.

### 2026-05-03 Task 3 Partial - Rider Readiness Status And Assignment Guard

- Added `logic.BaofuAccountService.ReadinessFromBinding` with product-facing states: `资料待提交`, `宝付开户处理中`, `微信渠道待报备`, `结算账户可用`, `开通失败`.
- `GET /v1/rider/status` now returns sanitized `settlement_account` readiness and blocks `can_go_online` with `骑手结算账户未开通，暂不能接收配送费分账订单` when the rider lacks a ready Baofu personal account.
- `logic.GrabDeliveryOrder` now blocks rider assignment before pool/order mutation if the rider Baofu settlement account is missing or not payment-ready; the response uses the same safe product copy and does not expose contract numbers, sharing IDs, raw upstream payloads, cards, ID numbers, or phone numbers.
- Delivery API tests now model the new Baofu readiness store read before grab-order flow proceeds to pool, merchant, deposit, and transaction checks.
- Assignment-path review: current rider order assignment entrypoint is `api.grabOrder` -> `logic.GrabDeliveryOrder` -> `db.GrabOrderTx`; no separate worker/scheduler auto-assignment path was found in this slice.
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuAccountReadinessStates' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestGetRiderStatusAPI|TestGoOnlineAPI' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuAccount(Readiness|Service)' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestGrabDeliveryOrder_BlocksMissingBaofuSettlementAccount|TestGrabDeliveryOrder_Success' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestGrabDeliveryOrder' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestGrabOrderAPI' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api ./logic -run 'TestGetRiderStatusAPI|TestGoOnlineAPI|TestGrabDeliveryOrder|TestGrabOrderAPI|TestBaofuAccountReadiness' -count=1`; `git diff --check`.
- Additional lint attempt: `PATH="/usr/local/go/bin:$PATH" make lint-filesize` still fails on 71 pre-existing oversized Go files, including existing `api/rider.go` and `api/delivery.go`; this slice keeps new Baofu helper files small but still touches existing oversized handlers/tests.
- Residual risk: Task 3 still needs merchant/operator/platform onboarding propagation, payment-creation merchant readiness enforcement, onboarding worker propagation, and frontend display/wizard updates before Baofu readiness is end-to-end complete.

### 2026-05-03 Task 3 Partial - Merchant Open Readiness Guard

- Added the merchant-side Baofu readiness gate to `PATCH /v1/merchants/me/status` when opening the merchant.
- Opening now requires the existing ordinary service provider payment config to be active and the merchant Baofu account binding to be payment-ready with a WeChat channel identity on the binding.
- Public errors stay product-facing: `商户结算账户未开通，暂不能开业接收分账订单` and `商户微信渠道待报备，暂不能开业接收微信生态支付订单`; raw `contractNo`, `sharingMerId`, upstream payloads, bank/card/ID/phone details remain hidden.
- `GET /v1/merchants/me/status` now returns sanitized `settlement_account` readiness (`state`, `label`, `payment_ready`) so merchant clients can show `资料待提交`, `宝付开户处理中`, `微信渠道待报备`, `结算账户可用`, or `开通失败` without exposing `contractNo`, `sharingMerId`, or upstream payloads.
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestUpdateMerchantOpenStatus_RequireBaofuAccountWhenOpen' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestUpdateMerchantOpenStatus' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api ./worker -run 'TestProfitSharingCapability|TestOnboardingReview|TestRider|TestUpdateMerchantOpenStatus' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestGetMerchantOpenStatus_IncludesBaofuSettlementReadiness|TestUpdateMerchantOpenStatus' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api ./logic -run 'TestGetMerchantOpenStatus|TestUpdateMerchantOpenStatus|TestBaofuAccountReadiness|TestCreatePaymentOrderAPI|TestCreateCombinedPaymentOrderAPI' -count=1`; `PATH="/usr/local/go/bin:$PATH" make swagger`; `PATH="/usr/local/go/bin:$PATH" make check-generated`; `git diff --check`.
- Additional lint attempt: `PATH="/usr/local/go/bin:$PATH" make lint-filesize` still fails on the same 71 pre-existing oversized Go files, including existing `api/merchant.go`; the new merchant Baofu helper is in a small separate file.
- Residual risk: Task 3 still needs operator/platform readiness response surfaces, onboarding worker propagation, and frontend display/wizard updates.

### 2026-05-03 Task 3 Partial - Single Order Payment Readiness Guard

- Added a payment-creation Baofu readiness guard in `logic.PaymentOrderService` before new ordinary-service-provider main-business order payments create local payment rows or call upstream payment APIs.
- Missing/unready merchant Baofu account now returns `商户结算账户未开通，暂不能创建支付订单`; missing WeChat channel identity on an otherwise active Baofu account returns `商户微信渠道待报备，暂不能创建微信生态支付订单`.
- API coverage verifies the frontend-facing response is semantic and does not expose `contract`, `sharing`, provider internals, or upstream identifiers.
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestPaymentOrderServiceCreatePaymentOrder_RequiresMerchantBaofuReadiness' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestPaymentOrderServiceCreatePaymentOrder' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestCreatePaymentOrderAPI' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api ./logic -run 'TestCreatePaymentOrderAPI|TestPaymentOrderServiceCreatePaymentOrder|TestBaofuAccountReadiness|TestUpdateMerchantOpenStatus' -count=1`; `git diff --check`.
- Additional lint attempt: `PATH="/usr/local/go/bin:$PATH" make lint-filesize` still fails on the same 71 pre-existing oversized Go files, including existing `logic/payment_order_service.go`; the new Baofu payment readiness helper is in a small separate file.
- Residual risk: this guard covers single-order payment creation. Combined payment creation still needs a pre-transaction merchant Baofu readiness strategy or a transaction-level guard so it does not create local pending child payments before detecting an unready merchant.

### 2026-05-03 Task 3 Partial - Combined Payment Readiness Guard

- Added a combined-payment Baofu readiness guard in `logic.CombinedPaymentService` for the ordinary service provider main-business channel.
- The guard deduplicates order IDs, reads each order to resolve unique merchant IDs, verifies ownership, then requires each merchant Baofu binding to be active with `sharing_mer_id` and `wechat_sub_mch_id` before `CreateCombinedPaymentTx`.
- Missing/unready merchant Baofu account returns `商户结算账户未开通，暂不能创建支付订单`; missing WeChat channel identity returns `商户微信渠道待报备，暂不能创建微信生态支付订单`. The new path does not expose contract numbers, sharing IDs, raw upstream payloads, card/ID/phone data, or provider internals.
- The pre-transaction strategy intentionally stops before local combined payment and child `payment_orders` rows are created, so a blocked merchant does not leave pending local payment anchors or trigger upstream combine payment.
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestCreateCombinedPaymentOrder_OrdinaryServiceProviderRequiresMerchantBaofuReadiness' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestCreateCombinedPaymentOrder' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestCreateCombinedPaymentOrder|TestPaymentOrderServiceCreatePaymentOrder|TestBaofuAccountReadiness' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestCreateCombinedPaymentOrderAPI_BaofuReadinessErrorIsSanitized' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api ./logic -run 'TestCreateCombinedPaymentOrderAPI|TestCreateCombinedPaymentOrder|TestCreatePaymentOrderAPI|TestPaymentOrderServiceCreatePaymentOrder|TestBaofuAccountReadiness|TestUpdateMerchantOpenStatus' -count=1`; `git diff --check`.
- Additional lint attempt: `PATH="/usr/local/go/bin:$PATH" make lint-filesize` still fails on the same 71 pre-existing oversized Go files, including existing `api/payment_order.go` and `logic/combined_payment_service.go`; this slice keeps the shared readiness helper small and only adds the necessary call site/test coverage to existing oversized payment files.
- API regression now verifies the combined payment readiness error is exposed as safe frontend-facing Chinese copy and does not include `contract`, `sharing`, `baofu`, or provider internals.
- Residual risk: this slice covers backend logic and API creation-time blocking. It does not yet add merchant/operator/platform onboarding status surfaces or frontend display/wizard updates.

### 2026-05-03 Task 3 Partial - Operator Application Readiness Display

- `GET /v1/operator/application` now returns sanitized `settlement_account` readiness after an approved application has a formal operator account, using the same `state`, `label`, `payment_ready` response shape as merchant and rider status surfaces.
- Operator readiness checks the Baofu operator binding with `owner_type=operator` and does not require `wechat_sub_mch_id`; operator commission is received into the Baofu secondary account rather than a WeChat secondary merchant account.
- Store errors from `GetOperatorByUser` or Baofu binding lookup now go through the existing logged internal-error boundary instead of being silently ignored on approved applications.
- API regression coverage verifies the response exposes the product state `宝付开户处理中` while hiding `contract_no`, `sharing_mer_id`, `contractNo`, `sharingMerId`, and concrete upstream receiver identifiers.
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestGetOperatorApplicationAPI_IncludesBaofuSettlementReadiness' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestGetOperatorApplicationAPI_IncludesBaofuSettlementReadiness|TestGetOperatorApplicationAPI' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api ./logic -run 'TestGetOperatorApplicationAPI|TestBaofuAccountReadiness|TestGetMerchantOpenStatus|TestGetRiderStatusAPI' -count=1`; `PATH="/usr/local/go/bin:$PATH" make swagger`; `PATH="/usr/local/go/bin:$PATH" make check-generated`; `git diff --check`.
- Additional lint attempt: `PATH="/usr/local/go/bin:$PATH" make lint-filesize` still fails on the same 71 pre-existing oversized Go files, including existing `api/operator_application.go`; the new operator Baofu helper is in a small separate file.
- Residual risk: Task 3 still needs platform readiness response surfaces, onboarding worker propagation, and frontend display/wizard updates.

### 2026-05-03 Task 3 Partial - Platform Settlement Readiness Display

- Added `GET /v1/platform/finance/settlement-account/status` for admins to view the platform commission receiver's Baofu settlement readiness with the same sanitized `settlement_account` response shape.
- Platform readiness uses a singleton binding (`owner_type=platform`, `owner_id=0`) and does not require `wechat_sub_mch_id`; the account receives platform 2% commission into a Baofu secondary/platform account rather than a WeChat secondary merchant account.
- API regression coverage verifies the response can show `结算账户可用` while hiding `contract_no`, `sharing_mer_id`, `contractNo`, `sharingMerId`, and concrete upstream receiver identifiers.
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestGetPlatformBaofuSettlementStatusAPI_IncludesSanitizedReadiness' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestGetPlatformBaofuSettlementStatusAPI_IncludesSanitizedReadiness|TestGetPlatformAccountBalanceAPI|TestGetOperatorApplicationAPI_IncludesBaofuSettlementReadiness|TestGetMerchantOpenStatus_IncludesBaofuSettlementReadiness|TestGetRiderStatusAPI' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api ./logic -run 'TestGetPlatformBaofuSettlementStatusAPI|TestGetOperatorApplicationAPI|TestGetMerchantOpenStatus|TestGetRiderStatusAPI|TestBaofuAccountReadiness' -count=1`; `PATH="/usr/local/go/bin:$PATH" make swagger`; `PATH="/usr/local/go/bin:$PATH" make check-generated`; `git diff --check`.
- Additional lint attempt: `PATH="/usr/local/go/bin:$PATH" make lint-filesize` still fails on the same 71 pre-existing oversized Go files, including existing `api/server.go` and `api/platform_finance.go`; the new platform Baofu helper is in a small separate file.
- Residual risk: Task 3 still needs onboarding worker propagation and frontend display/wizard updates.

### 2026-05-03 Task 4 Partial - Aggregate Payment Contracts

- Verified BaoCaiTong aggregate payment docs for unified order, confirm sharing, payment notification, sharing notification, payment query, sharing query, order state appendix, and channel return appendix.
- Added project-level aggregate payment contract DTOs in `locallife/baofu/aggregatepay/contracts/types.go` for WeChat JSAPI `SHARING` unified order, channel return `wc_pay_data`, payment facts, `share_after_pay`, share facts, and payment/share terminal-state normalization.
- Locked the no receiver-identity mixup invariant in tests: payer `sub_openid` only appears in `payExtend`, while share receivers use `sharingMerId`; the unified-order request never writes `sharingMerId`.
- Implemented Baofu state mapping for payment (`WAIT_PAYING` -> processing, `SUCCESS` -> success, `CLOSED` -> closed, `PAY_ERROR` -> failed, `REFUND` -> success for pre-share refund handling, `ABNORMAL` -> unknown) and sharing (`PROCESSING` -> processing, `SUCCESS` -> success, `CANCELED` -> failed, `ABNORMAL` -> unknown).
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" gofmt -w baofu/aggregatepay/contracts/types.go baofu/aggregatepay/contracts/types_test.go`; `PATH="/usr/local/go/bin:$PATH" go test ./baofu/aggregatepay/contracts -run 'TestUnifiedOrder|TestNormalizePaymentTerminalStatus|TestShareAfterPay|TestNormalizeShareTerminalStatus' -count=1`.
- Residual risk: this slice is contract-only. It does not yet route main-business payment creation to Baofu, persist Baofu payment commands/facts, apply payment success, or wire runtime transport.

### 2026-05-03 Task 4 Partial - Unified Order Service Command

- Added `logic.BaofuPaymentService` as the service-level Baofu unified-order boundary. It builds BaoCaiTong WeChat JSAPI `SHARING` unified-order requests with the collect merchant/terminal, mini-program appid, payer `sub_openid`, merchant `subMchId`, and payment notify URL.
- Added command-before-client-call persistence for Baofu create-payment commands using provider `baofu`, channel `baofu_aggregate`, capability `baofu_payment`, command type `create_payment`, business object `payment_order`, and external object type `baofu_payment_order`.
- Added sanitized command snapshots for create-payment attempts; payer openid is not stored in the snapshot, and upstream `wc_pay_data` is returned only from the service result for the mini program payment call.
- Extended `PaymentCommandService` validation to accept Baofu provider/channel/capabilities and the `baofu_payment_order` external object type so later slices can reuse the common command recorder instead of bypassing validation.
- Added the initial `baofu/aggregatepay.Client` interface for the logic layer to depend on project-owned aggregate payment contracts rather than upstream/raw DTOs.
- TDD verification: first run failed with missing `NewBaofuPaymentService`, `BaofuPaymentServiceConfig`, `CreateBaofuWechatJSAPIOrderInput`, `ErrBaofuPaymentWechatPayDataRequired`, and `ExternalPaymentObjectBaofuPaymentOrder`; after implementation, `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuPaymentService|TestPaymentCommandServiceRecordExternalPaymentCommand_AcceptsBaofuPaymentCommand' -count=1` passed.
- Additional verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" go test ./baofu/aggregatepay ./baofu/aggregatepay/contracts ./logic -run 'TestBaofuPayment|TestUnifiedOrder|TestPaymentCommandServiceRecordExternalPaymentCommand_AcceptsBaofuPaymentCommand' -count=1`.
- Residual risk: this slice still does not update `payment_orders` routing, does not call the runtime HTTP transport, does not persist payment facts from callbacks/queries, and does not apply payment success into local order state. Task 4 Step 2/3/4 remain open.

### 2026-05-03 Task 4 Partial - Payment Notification Facts

- Added `locallife/baofu/aggregatepay/notification` parser for Baofu aggregate payment notifications. It normalizes `outTradeNo`, `tradeNo`, `txnState`, success amount, fee amount, notification IDs/types, and occurrence time into project-owned `aggregatepay/contracts.PaymentFact`.
- Added `logic.BaofuPaymentService.RecordPaymentFact` to persist Baofu payment callback/query facts with provider `baofu`, channel `baofu_aggregate`, capability `baofu_payment`, external object type `baofu_payment_order`, business object `payment_order`, and normalized terminal status from the BaoCaiTong state map.
- Terminal Baofu payment facts now create an `external_payment_fact_applications` row for `order_domain/payment_order`; processing facts remain received-only and do not enqueue application work.
- Dedupe keys are deterministic: callback facts use `baofu:callback:payment:<outTradeNo>:<notifyId>` and query facts use `baofu:query:payment:<outTradeNo>:<tradeNo-or-state>`.
- TDD verification: first run failed with missing `NewParser`, `ErrPaymentNotificationOutTradeNoRequired`, `RecordPaymentFact`, and `RecordBaofuPaymentFactInput`; after implementation, `PATH="/usr/local/go/bin:$PATH" go test ./baofu/aggregatepay/notification ./logic -run 'TestParserParsePaymentNotification|TestBaofuPaymentServiceRecordPayment' -count=1` passed.
- Residual risk: this slice does not yet expose the Baofu payment callback HTTP route, does not verify real Baofu aggregate-payment callback signatures, does not enqueue the worker task after application creation, and does not implement the fact application worker that updates `payment_orders`.

### 2026-05-03 Task 4 Partial - Baofu Payment Fact Application

- Extended the existing payment fact application service to accept Baofu main-business payment facts in addition to WeChat ordinary/ecommerce payment facts.
- Baofu successful payment facts now mark the local `payment_orders` row paid with the Baofu `tradeNo` as `transaction_id` before running the existing idempotent `ProcessPaymentSuccessTx` order-domain transition.
- The existing order payment outbox path is reused, so a processed Baofu main-business payment emits `order_payment_succeeded` for later profit-sharing work instead of adding a parallel worker path.
- The Baofu paid update is idempotency-aware: if the conditional pending->paid update has already happened, the service reloads the payment order and only proceeds when the current state is already `paid`.
- TDD verification: first run failed because Baofu facts were rejected as unsupported WeChat-only main-business payment facts; after implementation, `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuOrderPaymentSuccessMarksPaidAndProcessesOrder|TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderPaymentSuccessProcessesOrder|TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderOrdinaryPaymentSuccessProcessesOrder' -count=1` passed.
- Residual risk: this slice still depends on a future Baofu callback route or recovery scheduler to enqueue the application task, and it does not yet cover closed/failed Baofu payment facts or combined-payment routing.

### 2026-05-03 Task 4 Partial - Single Order Baofu Payment Routing

- Added `NewPaymentOrderServiceWithBaofu` and the first main-business create-payment routing path for Baofu aggregate payment.
- `CreatePaymentOrder` now routes single order/reservation main-business payments to `baofu_aggregate` when the service is constructed with the Baofu payment service. The Baofu path keeps merchant Baofu readiness checks, reads the payer WeChat openid, creates the local `payment_orders` row, calls Baofu unified order, and returns BaoCaiTong `wc_pay_data` as the existing mini-program `PayParams` envelope.
- Extended `CreatePartnerPaymentTxParams` with optional `payment_channel` and `requires_profit_sharing`; existing callers default to `ordinary_service_provider`, while the Baofu route explicitly writes `payment_channel=baofu_aggregate` and `requires_profit_sharing=true`.
- The Baofu path forces profit sharing for main-business payments, matching the full-switch design where merchant, rider, operator, and platform receivers are all Baofu secondary-account receivers.
- TDD verification: first run failed with missing `CreatePartnerPaymentTxParams.PaymentChannel`, `CreatePartnerPaymentTxParams.RequiresProfitSharing`, and `NewPaymentOrderServiceWithBaofu`; after implementation, `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestPaymentOrderServiceCreatePaymentOrder_UsesBaofuForMainBusiness' -count=1` passed.
- Residual risk: runtime server wiring still constructs the ordinary-service-provider payment service; existing-pending Baofu payment re-signing is not supported because `wc_pay_data` is not persisted; combined payment routing remains on the existing channel until a dedicated Baofu combined/single-order policy is implemented.

### 2026-05-03 Task 4 Partial - Baofu Payment Callback Route

- Added `POST /v1/webhooks/baofu/payment` to receive Baofu aggregate payment notifications.
- The callback reads the request body, uses the Baofu aggregate payment notification parser, loads the local `payment_orders` row by `outTradeNo`, records the Baofu payment callback fact through `logic.BaofuPaymentService.RecordPaymentFact`, and enqueues the existing `payment:process_fact_application` worker when a terminal application is created.
- The callback ACKs only after fact persistence succeeds. If worker enqueue fails after persistence, it still returns success and relies on the existing fact-application scheduler to retry.
- Public callback responses stay generic (`SUCCESS/FAIL`) while logs use sanitized order/payment identifiers; no `openid`, `contractNo`, `sharingMerId`, bank/card/ID/phone, signatures, or raw upstream payloads are exposed in normal responses.
- TDD verification: first run failed with missing `SetBaofuAggregatePaymentNotificationParserForTest`; after implementation, `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestBaofuPaymentCallbackPersistsFactAndEnqueuesApplication' -count=1` passed.
- Residual risk: the parser is still a project-owned notification parser without real Baofu aggregate-payment signature verification wired into runtime config; server production wiring still needs to install the real parser and Baofu payment route dependencies.

### 2026-05-03 Task 5 Partial - Fee And Split Calculation

- Added `logic.CalculateBaofuPaymentFeeFen` with the fixed 30 bps merchant-borne payment fee, rounded up to fen so the merchant deduction is never below the configured 0.3%.
- Added `logic.CalculateBaofuProfitSharingAmounts` for deterministic Baofu split math. Platform 2% and operator 3% are calculated from the paid order total, rider delivery fee is deducted separately, and the payment fee is deducted from the merchant share.
- Locked the sample formula from the implementation plan: 10000 fen total + 500 fen delivery fee produces rider 500, platform 200, operator 300, payment fee 30, and merchant 8970.
- Missing operator commission can be redirected to platform when requested, preserving the existing operational fallback without reducing merchant-borne payment fee accounting.
- The calculator rejects negative merchant amount before any future share-order write path can persist an invalid split.
- TDD verification: first run failed with missing `CalculateBaofuPaymentFeeFen`, `CalculateBaofuProfitSharingAmounts`, `BaofuProfitSharingAmountInput`, and `ErrBaofuProfitSharingMerchantAmountNegative`; after implementation, `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestCalculateBaofu' -count=1` passed.
- Residual risk: this slice is pure calculation only. It does not yet resolve Baofu receiver IDs, write `profit_sharing_orders` Baofu receiver/fee columns, write `baofu_fee_ledger`, or update finance API display.

### 2026-05-03 Task 5 Partial - Receiver Resolution

- Added `logic.ResolveBaofuProfitSharingReceivers` to resolve merchant, rider, operator, and platform Baofu receiver IDs from `baofu_account_bindings`.
- Receiver resolution uses canonical `sharing_mer_id` only. The account opening/query layer is responsible for syncing the开户接口返回的二级商户号 into `sharing_mer_id`; resolver deliberately rejects active rows that only have `contract_no`, so分账创建 cannot accidentally use an unsynchronized/account-query-only field.
- Resolver requires active Baofu bindings and rejects inactive, missing, or receiver-less bindings before a future share order can be written.
- Platform receiver resolution defaults to the platform singleton owner (`owner_type=platform`, `owner_id=0`); rider/operator receiver resolution is conditional on the caller providing a rider/operator ID.
- TDD verification: first run failed with missing `ResolveBaofuProfitSharingReceivers` and `BaofuProfitSharingReceiverInput`; after implementation, `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestResolveBaofuProfitSharingReceivers|TestCalculateBaofu' -count=1` passed.
- Residual risk: this slice resolves receivers only. It does not yet persist the resolved IDs into `profit_sharing_orders.*_sharing_mer_id`, build `sharing_detail_snapshot`, write fee ledger rows, or create the Baofu share command.

### 2026-05-03 Receiver Field Correction - Canonical Secondary Merchant ID

- Incorporated the confirmed Baofu guidance: the profit-sharing receiver is the secondary merchant ID returned by the account-opening interface. The local canonical field for that value is `baofu_account_bindings.sharing_mer_id`.
- Updated `BaofuAccountService` readiness so an active account with only `contract_no` is not payment/share-ready; account-opening/query normalization must sync the returned secondary merchant ID into `sharing_mer_id` first.
- Updated receiver resolution so share creation reads only `sharing_mer_id` and never falls back to `contract_no`, `openid`, `sub_openid`, merchant `subMchId`, or the platform Baofu collect merchant ID.
- Added migration `000228_require_baofu_sharing_mer_id` to backfill active rows from `contract_no` once, then tighten `baofu_account_bindings_active_receiver_check` so future active rows require `sharing_mer_id`.
- Updated the integration design to state that the platform commission receiver must also be a platform-owned Baofu secondary account (`owner_type=platform`, `owner_id=0`), not the platform collect merchant account.
- TDD verification: first run failed because contract-only active rows were still treated as ready/resolvable; after implementation, `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuAccountService|TestResolveBaofuProfitSharingReceivers|TestCalculateBaofu' -count=1` passed.

### 2026-05-03 Task 5 - Receiver Snapshot And Fee Ledger Persistence

- Tightened the confirmed receiver contract: account active state now requires canonical `sharing_mer_id`; `MarkBaofuAccountBindingActive` no longer silently falls back from `contract_no` to `sharing_mer_id`. The account-open/query parser must pass the Baofu-returned secondary merchant ID as `sharing_mer_id`; `contract_no` remains query/audit trace only.
- Extended `CreateProfitSharingOrder` so Baofu orders persist merchant-borne `payment_fee`, rate bps, provider/channel, four resolved receiver IDs, and `sharing_detail_snapshot` while existing WeChat callers keep defaults.
- Added `baofu_fee_ledger` sqlc queries plus an atomic `CreateBaofuProfitSharingOrderTx` that creates the Baofu profit-sharing order and merchant payment-fee ledger row in one DB transaction.
- Added `BaofuProfitSharingService.CreatePendingOrder` to calculate the split, resolve receivers from `sharing_mer_id`, build a deterministic snapshot, and persist the share order plus fee ledger row.
- Account opening now records the platform-borne 1 RMB account-open verification fee ledger row in the same DB transaction that marks the binding active, so an active Baofu account cannot be committed without its opening-fee ledger row.
- Merchant finance overview, order details, service-fee details, and daily finance responses now expose Baofu payment fee separately from platform/operator service fees; platform 2% and operator 3% are not reduced by the 0.3% payment fee.
- Verification run from `locallife/`: `PATH="/usr/local/go/bin:$PATH" make sqlc`; `PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -run 'TestCreateProfitSharingOrderPersistsBaofuFields|TestCreateBaofuFeeLedger|TestGetBaofuFeeLedger|TestMarkBaofuAccountBindingActiveRejectsContractOnlyReceiver|TestMarkBaofuAccountBindingActiveWithFeeLedgerTx' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofuProfitSharingServiceCreatePendingOrderPersistsSnapshotAndFeeLedger|TestBaofuAccountServiceOpenAccountRecordsCommandBeforeClientCall' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./logic ./api ./db/sqlc -run 'TestBaofuProfitSharing|TestMerchantFinance|TestProfitSharingOrder|TestCreateBaofuFeeLedger|TestGetBaofuFeeLedger|TestCreateBaofuProfitSharingOrderTx|TestBaofuAccountServiceOpenAccount|TestMarkBaofuAccountBindingActive' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestGetMerchantFinanceOverviewAPI|TestListMerchantFinanceOrdersAPI|TestListMerchantServiceFeesAPI|TestListMerchantDailyFinanceAPI' -count=1`; `PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc ./logic ./api -count=1`; `PATH="/usr/local/go/bin:$PATH" make check-generated`; `git diff --check`.
- Additional lint attempt: `PATH="/usr/local/go/bin:$PATH" make lint-filesize` still fails on the same pre-existing 71 oversized Go files, including existing `api/merchant_finance.go`; this slice adds only the fields needed for Baofu fee display.
- Residual risk: this slice persists the pending Baofu share order and fee ledger. It does not yet run the share worker, call Baofu `share_after_pay`, apply share callbacks, or enforce refund-before-sharing concurrency; those remain Task 6 and Task 7.
