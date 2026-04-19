package contracts

// ProfitSharingRequest requests an ecommerce profit-sharing order.
type ProfitSharingRequest struct {
	AppID         string
	SubMchID      string
	TransactionID string
	OutOrderNo    string
	Receivers     []ProfitSharingReceiver
	Finish        bool
}

// ProfitSharingReceiver describes a single profit-sharing receiver.
type ProfitSharingReceiver struct {
	Type                  string
	ReceiverAccount       string
	ReceiverName          string
	EncryptedReceiverName string
	Amount                int64
	Description           string
}

// ProfitSharingResponse is returned by the create profit-sharing API and reused for
// finish responses, which return the same identity fields but omit receivers/status.
type ProfitSharingResponse struct {
	SubMchID      string                        `json:"sub_mchid"`
	TransactionID string                        `json:"transaction_id"`
	OutOrderNo    string                        `json:"out_order_no"`
	OrderID       string                        `json:"order_id"`
	Receivers     []ProfitSharingReceiverResult `json:"receivers,omitempty"`
	Status        string                        `json:"status,omitempty"`
}

// ProfitSharingAmountsResponse returns the remaining unsplit amount.
type ProfitSharingAmountsResponse struct {
	TransactionID string `json:"transaction_id"`
	UnsplitAmount int64  `json:"unsplit_amount"`
}

// ProfitSharingQueryResponse represents the current profit-sharing result.
type ProfitSharingQueryResponse struct {
	SubMchID          string                        `json:"sub_mchid"`
	TransactionID     string                        `json:"transaction_id"`
	OutOrderNo        string                        `json:"out_order_no"`
	OrderID           string                        `json:"order_id"`
	Status            string                        `json:"status"`
	Receivers         []ProfitSharingReceiverResult `json:"receivers"`
	FinishAmount      int64                         `json:"finish_amount"`
	FinishDescription string                        `json:"finish_description"`
}

// ProfitSharingReceiverResult represents a receiver result in query responses.
type ProfitSharingReceiverResult struct {
	Type                      string                          `json:"type"`
	ReceiverAccount           string                          `json:"receiver_account"`
	Amount                    int64                           `json:"amount"`
	Description               string                          `json:"description"`
	Result                    string                          `json:"result"`
	FinishTime                string                          `json:"finish_time"`
	FailReason                string                          `json:"fail_reason,omitempty"`
	DetailID                  string                          `json:"detail_id"`
	AbnormalStatus            string                          `json:"abnormal_status,omitempty"`
	FundsAbnormalClosedReason string                          `json:"funds_abnormal_closed_reason,omitempty"`
	FundsAbnormalRedirectID   string                          `json:"funds_abnormal_redirect_id,omitempty"`
	FundsAbnormalReceivers    []ProfitSharingAbnormalReceiver `json:"funds_abnormal_receivers,omitempty"`
}

// ProfitSharingAbnormalReceiver describes a documented abnormal funds receiver.
type ProfitSharingAbnormalReceiver struct {
	MchID string `json:"mchid"`
}

const (
	ReceiverTypeMerchant = "MERCHANT_ID"
	ReceiverTypePersonal = "PERSONAL_OPENID"
)

const (
	ProfitSharingStatusProcessing = "PROCESSING"
	ProfitSharingStatusFinished   = "FINISHED"
)

const (
	ProfitSharingResultPending = "PENDING"
	ProfitSharingResultSuccess = "SUCCESS"
	ProfitSharingResultClosed  = "CLOSED"
)

const (
	ProfitSharingFailReasonAccountAbnormal             = "ACCOUNT_ABNORMAL"
	ProfitSharingFailReasonNoRelation                  = "NO_RELATION"
	ProfitSharingFailReasonReceiverHighRisk            = "RECEIVER_HIGH_RISK"
	ProfitSharingFailReasonReceiverRealNameNotVerified = "RECEIVER_REAL_NAME_NOT_VERIFIED"
	ProfitSharingFailReasonNoAuth                      = "NO_AUTH"
	ProfitSharingFailReasonReceiverReceiptLimit        = "RECEIVER_RECEIPT_LIMIT"
	ProfitSharingFailReasonPayerAccountAbnormal        = "PAYER_ACCOUNT_ABNORMAL"
	ProfitSharingFailReasonInvalidRequest              = "INVALID_REQUEST"
)

const (
	ProfitSharingAbnormalStatusPending  = "ABNORMAL_PENDING"
	ProfitSharingAbnormalStatusFinished = "ABNORMAL_FINISHED"
	ProfitSharingAbnormalStatusClosed   = "ABNORMAL_CLOSED"
)

const (
	ProfitSharingAbnormalClosedReasonTimeout          = "CLOSED_REASON_TIMEOUT"
	ProfitSharingAbnormalClosedReasonRestrictTransfer = "CLOSED_REASON_RESTRICT_TRANSFER"
)

const (
	RelationServiceProvider = "SERVICE_PROVIDER"
	RelationDistributor     = "DISTRIBUTOR"
	RelationSupplier        = "SUPPLIER"
	RelationPlatform        = "PLATFORM"
	RelationOthers          = "OTHERS"
)

// AddReceiverRequest adds a profit-sharing receiver relationship.
type AddReceiverRequest struct {
	AppID         string `json:"appid,omitempty"`
	Type          string `json:"type"`
	Account       string `json:"account"`
	Name          string `json:"name,omitempty"`
	EncryptedName string `json:"-"`
	RelationType  string `json:"relation_type"`
}

// AddReceiverResponse is returned after a receiver add call.
type AddReceiverResponse struct {
	Type    string `json:"type"`
	Account string `json:"account"`
}

// DeleteReceiverRequest deletes a profit-sharing receiver relationship.
type DeleteReceiverRequest struct {
	AppID   string `json:"appid,omitempty"`
	Type    string `json:"type"`
	Account string `json:"account"`
}

// DeleteReceiverResponse is returned after a receiver delete call.
type DeleteReceiverResponse struct {
	Type    string `json:"type"`
	Account string `json:"account"`
}

// ProfitSharingReturnRequest requests a profit-sharing return.
type ProfitSharingReturnRequest struct {
	SubMchID      string
	OrderID       string
	OutOrderNo    string
	OutReturnNo   string
	TransactionID string
	ReturnMchID   string
	Amount        int64
	Description   string
}

// ProfitSharingReturnResponse represents a profit-sharing return result.
type ProfitSharingReturnResponse struct {
	SubMchID      string `json:"sub_mchid"`
	OrderID       string `json:"order_id"`
	OutOrderNo    string `json:"out_order_no"`
	OutReturnNo   string `json:"out_return_no"`
	ReturnID      string `json:"return_no"`
	ReturnMchID   string `json:"return_mchid"`
	Amount        int64  `json:"amount"`
	Result        string `json:"result"`
	FinishTime    string `json:"finish_time"`
	FailReason    string `json:"fail_reason,omitempty"`
	TransactionID string `json:"transaction_id"`
}

const (
	ProfitSharingReturnResultProcessing = "PROCESSING"
	ProfitSharingReturnResultSuccess    = "SUCCESS"
	ProfitSharingReturnResultFailed     = "FAILED"
)

const (
	ProfitSharingReturnFailReasonAccountAbnormal      = "ACCOUNT_ABNORMAL"
	ProfitSharingReturnFailReasonTimeoutClosed        = "TIME_OUT_CLOSED"
	ProfitSharingReturnFailReasonPayerAccountAbnormal = "PAYER_ACCOUNT_ABNORMAL"
	ProfitSharingReturnFailReasonInvalidRequest       = "INVALID_REQUEST"
)

// ProfitSharingNotification represents decrypted profit-sharing callback data.
type ProfitSharingNotification struct {
	SPMchID       string                            `json:"sp_mchid"`
	SubMchID      string                            `json:"sub_mchid"`
	TransactionID string                            `json:"transaction_id"`
	OrderID       string                            `json:"order_id"`
	OutOrderNo    string                            `json:"out_order_no"`
	Receiver      ProfitSharingNotificationReceiver `json:"receiver"`
	SuccessTime   string                            `json:"success_time"`
}

// ProfitSharingNotificationReceiver is the receiver section of a callback.
type ProfitSharingNotificationReceiver struct {
	Type        string `json:"type"`
	Account     string `json:"account"`
	Amount      int64  `json:"amount"`
	Description string `json:"description"`
}
