package contracts

// 官方文档：账户资金管理组
// 查询二级商户账户实时余额：https://pay.weixin.qq.com/doc/v3/partner/4012476690.md
// 查询二级商户账户日终余额：https://pay.weixin.qq.com/doc/v3/partner/4012476693.md
// 查询平台账户实时余额：https://pay.weixin.qq.com/doc/v3/partner/4012476700.md
// 查询平台账户日终余额：https://pay.weixin.qq.com/doc/v3/partner/4012476702.md
// 二级商户预约提现：https://pay.weixin.qq.com/doc/v3/partner/4012476652.md
// 二级商户查询预约提现状态（根据商户预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476656.md
// 二级商户查询预约提现状态（根据微信支付预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476665.md
// 平台预约提现：https://pay.weixin.qq.com/doc/v3/partner/4012476670.md
// 平台查询预约提现状态（根据商户预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476672.md
// 平台查询预约提现状态（根据微信支付预约提现单号查询）：https://pay.weixin.qq.com/doc/v3/partner/4012476674.md
// 二级商户按日终余额预约提现：https://pay.weixin.qq.com/doc/v3/partner/4013328143.md
// 查询二级商户按日终余额预约提现状态：https://pay.weixin.qq.com/doc/v3/partner/4013328163.md
// 按日下载提现异常文件：https://pay.weixin.qq.com/doc/v3/partner/4012476678.md
// 商户提现状态变更通知：https://pay.weixin.qq.com/doc/v3/partner/4013049135.md

const (
	FundManagementAccountTypeBasic     = "BASIC"
	FundManagementAccountTypeFees      = "FEES"
	FundManagementAccountTypeOperation = "OPERATION"
	FundManagementAccountTypeDeposit   = "DEPOSIT"
)

const (
	FundManagementWithdrawStatusCreateSuccess = "CREATE_SUCCESS"
	FundManagementWithdrawStatusSuccess       = "SUCCESS"
	FundManagementWithdrawStatusFail          = "FAIL"
	FundManagementWithdrawStatusRefund        = "REFUND"
	FundManagementWithdrawStatusClose         = "CLOSE"
	FundManagementWithdrawStatusInit          = "INIT"
)

const (
	FundManagementDayEndWithdrawStatusCreated    = "CREATED"
	FundManagementDayEndWithdrawStatusProcessing = "PROCESSING"
	FundManagementDayEndWithdrawStatusFinished   = "FINISHED"
	FundManagementDayEndWithdrawStatusAbnormal   = "ABNORMAL"
)

const (
	FundManagementCalculateAmountTypeOnlyDayEndBalance   = "ONLY_DAY_END_BALANCE"
	FundManagementCalculateAmountTypeAllowCurrentBalance = "ALLOW_CURRENT_BALANCE"
)

const (
	FundManagementBillTypeNoSucc = "NO_SUCC"
)

const (
	FundManagementTarTypeGzip = "GZIP"
)

const (
	FundManagementHashTypeSHA1 = "SHA1"
)

const (
	FundManagementNotificationEventType    = "MCHWITHDRAW.CHANGE"
	FundManagementNotificationResourceType = "encrypt-resource"
	FundManagementNotificationOriginalType = "mch_withdraw"
)

type EcommerceFundBalanceQueryRequest struct {
	SubMchID    string `json:"sub_mchid"`
	AccountType string `json:"account_type,omitempty"`
}

type EcommerceFundDayEndBalanceQueryRequest struct {
	SubMchID    string `json:"sub_mchid"`
	Date        string `json:"date,omitempty"`
	AccountType string `json:"account_type,omitempty"`
}

type EcommerceFundBalanceResponse struct {
	SubMchID        string `json:"sub_mchid"`
	AvailableAmount int64  `json:"available_amount"`
	PendingAmount   int64  `json:"pending_amount,omitempty"`
	AccountType     string `json:"account_type,omitempty"`
}

type PlatformFundBalanceQueryRequest struct {
	AccountType string `json:"account_type"`
}

type PlatformFundDayEndBalanceQueryRequest struct {
	AccountType string `json:"account_type"`
	Date        string `json:"date,omitempty"`
}

type PlatformFundBalanceResponse struct {
	AvailableAmount int64 `json:"available_amount"`
	PendingAmount   int64 `json:"pending_amount,omitempty"`
}

// 官方文档：POST /v3/ecommerce/fund/withdraw
type EcommerceWithdrawRequest struct {
	SubMchID     string `json:"sub_mchid"`
	OutRequestNo string `json:"out_request_no"`
	Amount       int64  `json:"amount"`
	Remark       string `json:"remark,omitempty"`
	BankMemo     string `json:"bank_memo,omitempty"`
	AccountType  string `json:"account_type,omitempty"`
	NotifyURL    string `json:"notify_url,omitempty"`
}

// 官方文档：POST /v3/ecommerce/fund/withdraw
type EcommerceWithdrawCreateResponse struct {
	SubMchID     string `json:"sub_mchid"`
	WithdrawID   string `json:"withdraw_id"`
	OutRequestNo string `json:"out_request_no"`
}

// 官方文档：GET /v3/ecommerce/fund/withdraw/out-request-no/{out_request_no}
type EcommerceWithdrawQueryByOutRequestNoRequest struct {
	OutRequestNo string `json:"out_request_no"`
	SubMchID     string `json:"sub_mchid"`
}

// 官方文档：GET /v3/ecommerce/fund/withdraw/{withdraw_id}
type EcommerceWithdrawQueryByWithdrawIDRequest struct {
	WithdrawID string `json:"withdraw_id"`
	SubMchID   string `json:"sub_mchid"`
}

// 官方文档：GET /v3/ecommerce/fund/withdraw/out-request-no/{out_request_no}
// 官方文档：GET /v3/ecommerce/fund/withdraw/{withdraw_id}
type EcommerceWithdrawQueryResponse struct {
	SPMchID       string `json:"sp_mchid"`
	SubMchID      string `json:"sub_mchid"`
	Status        string `json:"status"`
	WithdrawID    string `json:"withdraw_id"`
	OutRequestNo  string `json:"out_request_no"`
	Amount        int64  `json:"amount"`
	CreateTime    string `json:"create_time"`
	UpdateTime    string `json:"update_time"`
	Reason        string `json:"reason,omitempty"`
	Remark        string `json:"remark,omitempty"`
	BankMemo      string `json:"bank_memo,omitempty"`
	AccountType   string `json:"account_type"`
	AccountNumber string `json:"account_number"`
	AccountBank   string `json:"account_bank"`
	BankName      string `json:"bank_name,omitempty"`
}

// 官方文档：POST /v3/merchant/fund/withdraw
type PlatformWithdrawRequest struct {
	OutRequestNo string `json:"out_request_no"`
	Amount       int64  `json:"amount"`
	Remark       string `json:"remark,omitempty"`
	BankMemo     string `json:"bank_memo,omitempty"`
	AccountType  string `json:"account_type"`
	NotifyURL    string `json:"notify_url,omitempty"`
}

// 官方文档：POST /v3/merchant/fund/withdraw
type PlatformWithdrawCreateResponse struct {
	WithdrawID   string `json:"withdraw_id"`
	OutRequestNo string `json:"out_request_no"`
}

// 官方文档：GET /v3/merchant/fund/withdraw/out-request-no/{out_request_no}
type PlatformWithdrawQueryByOutRequestNoRequest struct {
	OutRequestNo string `json:"out_request_no"`
}

// 官方文档：GET /v3/merchant/fund/withdraw/withdraw-id/{withdraw_id}
type PlatformWithdrawQueryByWithdrawIDRequest struct {
	WithdrawID string `json:"withdraw_id"`
}

// 官方文档：GET /v3/merchant/fund/withdraw/out-request-no/{out_request_no}
// 官方文档：GET /v3/merchant/fund/withdraw/withdraw-id/{withdraw_id}
type PlatformWithdrawQueryResponse struct {
	Status        string `json:"status"`
	WithdrawID    string `json:"withdraw_id"`
	OutRequestNo  string `json:"out_request_no"`
	Amount        int64  `json:"amount"`
	CreateTime    string `json:"create_time"`
	UpdateTime    string `json:"update_time"`
	Reason        string `json:"reason,omitempty"`
	Remark        string `json:"remark,omitempty"`
	BankMemo      string `json:"bank_memo,omitempty"`
	AccountType   string `json:"account_type"`
	Solution      string `json:"solution,omitempty"`
	AccountNumber string `json:"account_number"`
	AccountBank   string `json:"account_bank"`
	BankName      string `json:"bank_name,omitempty"`
}

// 官方文档：POST /v3/platsolution/ecommerce/withdraw/day-end-balance-withdraw
type DayEndBalanceWithdrawRequest struct {
	SubMchID            string `json:"sub_mchid"`
	OutRequestNo        string `json:"out_request_no"`
	CalculateAmountType string `json:"calculate_amount_type"`
	Remark              string `json:"remark,omitempty"`
	BankMemo            string `json:"bank_memo,omitempty"`
	NotifyURL           string `json:"notify_url,omitempty"`
	ReserveAmount       int64  `json:"reserve_amount,omitempty"`
}

// 官方文档：GET /v3/platsolution/ecommerce/withdraw/day-end-balance-withdraw/out-request-no/{out_request_no}
type DayEndBalanceWithdrawQueryRequest struct {
	OutRequestNo string `json:"out_request_no"`
	SubMchID     string `json:"sub_mchid"`
}

// 官方文档：POST /v3/platsolution/ecommerce/withdraw/day-end-balance-withdraw
// 官方文档：GET /v3/platsolution/ecommerce/withdraw/day-end-balance-withdraw/out-request-no/{out_request_no}
type DayEndBalanceWithdrawResponse struct {
	SPMchID       string `json:"sp_mchid"`
	SubMchID      string `json:"sub_mchid"`
	Status        string `json:"status"`
	WithdrawID    string `json:"withdraw_id"`
	OutRequestNo  string `json:"out_request_no"`
	TotalAmount   int64  `json:"total_amount"`
	SuccessAmount int64  `json:"success_amount,omitempty"`
	FailAmount    int64  `json:"fail_amount,omitempty"`
	RefundAmount  int64  `json:"refund_amount,omitempty"`
	CreateTime    string `json:"create_time"`
	UpdateTime    string `json:"update_time"`
	Reason        string `json:"reason,omitempty"`
	Remark        string `json:"remark,omitempty"`
	BankMemo      string `json:"bank_memo,omitempty"`
	AccountType   string `json:"account_type"`
	AccountNumber string `json:"account_number"`
	AccountBank   string `json:"account_bank"`
	BankName      string `json:"bank_name,omitempty"`
}

// 官方文档：GET /v3/merchant/fund/withdraw/bill-type/{bill_type}
type WithdrawBillRequest struct {
	BillType string `json:"bill_type"`
	BillDate string `json:"bill_date"`
	TarType  string `json:"tar_type,omitempty"`
}

// 官方文档：GET /v3/merchant/fund/withdraw/bill-type/{bill_type}
type WithdrawBillResponse struct {
	HashType    string `json:"hash_type"`
	HashValue   string `json:"hash_value"`
	DownloadURL string `json:"download_url"`
}

type WithdrawNotificationEnvelopeResource struct {
	OriginalType   string `json:"original_type"`
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	AssociatedData string `json:"associated_data,omitempty"`
	Nonce          string `json:"nonce"`
}

// 官方文档：商户提现状态变更通知外层 envelope
type WithdrawNotificationEnvelope struct {
	ID           string                               `json:"id"`
	CreateTime   string                               `json:"create_time"`
	EventType    string                               `json:"event_type"`
	ResourceType string                               `json:"resource_type"`
	Resource     WithdrawNotificationEnvelopeResource `json:"resource"`
	Summary      string                               `json:"summary"`
}

// 官方文档：商户提现状态变更通知 resource 解密后字段。
// 解密后的业务字段同对应提现查询接口返回体；这里使用 capability group 的并集结构承接三类提现通知。
type WithdrawNotificationResource struct {
	SPMchID       string `json:"sp_mchid,omitempty"`
	SubMchID      string `json:"sub_mchid,omitempty"`
	Status        string `json:"status"`
	WithdrawID    string `json:"withdraw_id"`
	OutRequestNo  string `json:"out_request_no"`
	Amount        int64  `json:"amount,omitempty"`
	TotalAmount   int64  `json:"total_amount,omitempty"`
	SuccessAmount int64  `json:"success_amount,omitempty"`
	FailAmount    int64  `json:"fail_amount,omitempty"`
	RefundAmount  int64  `json:"refund_amount,omitempty"`
	CreateTime    string `json:"create_time"`
	UpdateTime    string `json:"update_time"`
	Reason        string `json:"reason,omitempty"`
	Remark        string `json:"remark,omitempty"`
	BankMemo      string `json:"bank_memo,omitempty"`
	AccountType   string `json:"account_type"`
	Solution      string `json:"solution,omitempty"`
	AccountNumber string `json:"account_number"`
	AccountBank   string `json:"account_bank"`
	BankName      string `json:"bank_name,omitempty"`
}
