package contracts

// 官方文档：商户注销组
// 产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4018153750.md
// 注销预校验-商户注销资格校验：https://pay.weixin.qq.com/doc/v3/partner/4016420099.md
// 注销提现-提交注销提现申请：https://pay.weixin.qq.com/doc/v3/partner/4013892756.md
// 注销提现-商户申请单号查询申请单状态：https://pay.weixin.qq.com/doc/v3/partner/4013892759.md
// 注销提现-微信支付申请单号查询申请单状态：https://pay.weixin.qq.com/doc/v3/partner/4013892765.md

// ----- Validate (GET /v3/ecommerce/account/apply-cancel-withdraw/validate-cancel/{sub_mchid}) -----

// CancelWithdrawMerchantState 商户号状态
const (
	CancelWithdrawMerchantStateNormal    = "NORMAL"
	CancelWithdrawMerchantStateCancelled = "HAS_BEEN_CANCELLED"
)

// CancelWithdrawValidateResult 注销资格检查结果
const (
	CancelWithdrawValidateResultAllow    = "ALLOW_CANCEL_WITHDRAW"
	CancelWithdrawValidateResultNotAllow = "NOT_ALLOW_CANCEL_WITHDRAW"
)

// CancelWithdrawBlockReasonType 不可注销原因类型
const (
	CancelWithdrawBlockReasonTypeConsumerComplaint = "CONSUMER_COMPLAINT_UNPROCESSED"
	CancelWithdrawBlockReasonTypeBlockingControl   = "HAS_BLOCKING_CONTROL"
	CancelWithdrawBlockReasonTypeFundsPending      = "FUNDS_PENDING_PROCESSING"
	CancelWithdrawBlockReasonTypeOtherReason       = "OTHER_REASON"
)

// CancelWithdrawOutAccountType 二级商户号的出款子账户类型
const (
	CancelWithdrawOutAccountTypeBasic    = "BASIC_ACCOUNT"
	CancelWithdrawOutAccountTypeOperate  = "OPERATE_ACCOUNT"
	CancelWithdrawOutAccountTypeMargin   = "MARGIN_ACCOUNT"
	CancelWithdrawOutAccountTypeTradeFee = "TRADE_FEE_ACCOUNT"
)

type CancelWithdrawAccountInfo struct {
	OutAccountType string `json:"out_account_type"`
	Amount         int64  `json:"amount"`
}

type CancelWithdrawBlockReason struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// CancelWithdrawEligibilityResponse GET validate-cancel 响应体
type CancelWithdrawEligibilityResponse struct {
	SubMchID       string                      `json:"sub_mchid"`
	MerchantState  string                      `json:"merchant_state"`
	ValidateResult string                      `json:"validate_result"`
	AccountInfo    []CancelWithdrawAccountInfo `json:"account_info,omitempty"`
	BlockReasons   []CancelWithdrawBlockReason `json:"block_reasons,omitempty"`
}

// ----- Create (POST /v3/ecommerce/account/apply-cancel-withdraw) -----

// CancelWithdrawMode 是否提取资金
const (
	CancelWithdrawModeNotApply = "NOT_APPLY_WITHDRAW"
	CancelWithdrawModeApply    = "APPLY_WITHDRAW"
)

// CancelWithdrawAccountType 收款账户类型
const (
	CancelWithdrawAccountTypeCorporate = "ACCOUNT_TYPE_CORPORATE"
	CancelWithdrawAccountTypePersonal  = "ACCOUNT_TYPE_PERSONAL"
)

// CancelWithdrawIDDocType 收款人身份证件类型
const (
	CancelWithdrawIDDocTypeIDCard                = "IDENTIFICATION_TYPE_ID_CARD"
	CancelWithdrawIDDocTypeOverseaPassport       = "IDENTIFICATION_TYPE_OVERSEA_PASSPORT"
	CancelWithdrawIDDocTypeHongkongPassport      = "IDENTIFICATION_TYPE_HONGKONG_PASSPORT"
	CancelWithdrawIDDocTypeMacaoPassport         = "IDENTIFICATION_TYPE_MACAO_PASSPORT"
	CancelWithdrawIDDocTypeTaiwanPassport        = "IDENTIFICATION_TYPE_TAIWAN_PASSPORT"
	CancelWithdrawIDDocTypeForeignResident       = "IDENTIFICATION_TYPE_FOREIGN_RESIDENT"
	CancelWithdrawIDDocTypeHongkongMacaoResident = "IDENTIFICATION_TYPE_HONGKONG_MACAO_RESIDENT"
	CancelWithdrawIDDocTypeTaiwanResident        = "IDENTIFICATION_TYPE_TAIWAN_RESIDENT"
)

type CancelWithdrawIdentityInfo struct {
	IDDocType          string `json:"id_doc_type,omitempty"`
	IdentificationName string `json:"identification_name,omitempty"`
	IdentificationNo   string `json:"identification_no,omitempty"`
}

type CancelWithdrawBankAccountInfo struct {
	AccountName    string `json:"account_name,omitempty"`
	AccountBank    string `json:"account_bank,omitempty"`
	BankBranchID   string `json:"bank_branch_id,omitempty"`
	BankBranchName string `json:"bank_branch_name,omitempty"`
	AccountNumber  string `json:"account_number,omitempty"`
}

type CancelWithdrawPayeeInfo struct {
	AccountType     string                         `json:"account_type,omitempty"`
	BankAccountInfo *CancelWithdrawBankAccountInfo `json:"bank_account_info,omitempty"`
	IdentityInfo    *CancelWithdrawIdentityInfo    `json:"identity_info,omitempty"`
}

type CancelWithdrawProofMedia struct {
	ProofMediaType string `json:"proof_media_type"`
	ProofMedia     string `json:"proof_media"`
}

// CancelWithdrawRequest POST apply-cancel-withdraw 请求体
type CancelWithdrawRequest struct {
	SubMchID            string                     `json:"sub_mchid"`
	OutRequestNo        string                     `json:"out_request_no"`
	Withdraw            string                     `json:"withdraw,omitempty"`
	PayeeInfo           *CancelWithdrawPayeeInfo   `json:"payee_info,omitempty"`
	ProofMedias         []CancelWithdrawProofMedia `json:"proof_medias,omitempty"`
	AdditionalMaterials []string                   `json:"additional_materials,omitempty"`
	Remark              string                     `json:"remark,omitempty"`
}

// CancelWithdrawCreateResponse POST apply-cancel-withdraw 响应体
type CancelWithdrawCreateResponse struct {
	ApplymentID  string `json:"applyment_id"`
	OutRequestNo string `json:"out_request_no"`
}

// ----- Query (GET apply-cancel-withdraw/out-request-no/{} or applyment-id/{}) -----

// CancelWithdrawCancelState 注销提现申请单状态
const (
	CancelWithdrawCancelStateAccepted               = "ACCEPTED"
	CancelWithdrawCancelStateReviewing              = "REVIEWING"
	CancelWithdrawCancelStateRejected               = "REJECTED"
	CancelWithdrawCancelStateWaitingMerchantConfirm = "WAITING_MERCHANT_CONFIRM"
	CancelWithdrawCancelStateRevoked                = "REVOKED"
	CancelWithdrawCancelStateSystemProcessing       = "SYSTEM_PROCESSING"
	CancelWithdrawCancelStateCanceled               = "CANCELED"
	CancelWithdrawCancelStateFundProcessing         = "FUND_PROCESSING"
	CancelWithdrawCancelStateFinish                 = "FINISH"
)

// CancelWithdrawWithdrawState 提现状态
const (
	CancelWithdrawWithdrawStateProcessing = "WITHDRAW_PROCESSING"
	CancelWithdrawWithdrawStateException  = "WITHDRAW_EXCEPTION"
	CancelWithdrawWithdrawStateSucceed    = "WITHDRAW_SUCCEED"
)

// CancelWithdrawPayState 账户付款状态
const (
	CancelWithdrawPayStateProcessing   = "PAY_PROCESSING"
	CancelWithdrawPayStateSucceed      = "PAY_SUCCEED"
	CancelWithdrawPayStateFail         = "PAY_FAIL"
	CancelWithdrawPayStateBankRefunded = "BANK_REFUNDED"
)

type CancelWithdrawAccountWithdrawResult struct {
	OutAccountType   string `json:"out_account_type"`
	PayState         string `json:"pay_state"`
	StateDescription string `json:"state_description"`
}

type CancelWithdrawConfirmCancel struct {
	ConfirmCancelURL string `json:"confirm_cancel_url,omitempty"`
}

// CancelWithdrawQueryResponse 查询注销提现申请单响应体（by out_request_no 和 by applyment_id 共用）
type CancelWithdrawQueryResponse struct {
	ApplymentID              string                                `json:"applyment_id"`
	OutRequestNo             string                                `json:"out_request_no"`
	CancelState              string                                `json:"cancel_state"`
	CancelStateDescription   string                                `json:"cancel_state_description"`
	Withdraw                 string                                `json:"withdraw,omitempty"`
	WithdrawState            string                                `json:"withdraw_state,omitempty"`
	WithdrawStateDescription string                                `json:"withdraw_state_description,omitempty"`
	AccountWithdrawResult    []CancelWithdrawAccountWithdrawResult `json:"account_withdraw_result,omitempty"`
	ModifyTime               string                                `json:"modify_time,omitempty"`
	SubMchID                 string                                `json:"sub_mchid"`
	AccountInfo              []CancelWithdrawAccountInfo           `json:"account_info,omitempty"`
	ConfirmCancel            *CancelWithdrawConfirmCancel          `json:"confirm_cancel,omitempty"`
}
