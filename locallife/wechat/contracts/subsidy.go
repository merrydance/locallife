package contracts

// SubsidyRequest creates an ecommerce subsidy.
type SubsidyRequest struct {
	SubMchID      string
	TransactionID string
	Amount        int64
	Description   string
	OutSubsidyNo  string
}

// SubsidyResponse is returned by the subsidy create interface.
type SubsidyResponse struct {
	SubMchID      string `json:"sub_mchid"`
	TransactionID string `json:"transaction_id"`
	SubsidyID     string `json:"subsidy_id"`
	Description   string `json:"description"`
	Amount        int64  `json:"amount"`
	Result        string `json:"result"`
	SuccessTime   string `json:"success_time"`
	OutSubsidyNo  string `json:"out_subsidy_no"`
}

// SubsidyReturnRequest requests subsidy return before refund.
type SubsidyReturnRequest struct {
	SubMchID      string
	OutOrderNo    string
	TransactionID string
	RefundID      string
	Amount        int64
	Description   string
	SubsidyID     string
	From          []SubsidyReturnFrom
}

// SubsidyReturnResponse is returned by the subsidy return interface.
type SubsidyReturnResponse struct {
	SubMchID        string              `json:"sub_mchid"`
	TransactionID   string              `json:"transaction_id"`
	SubsidyRefundID string              `json:"subsidy_refund_id"`
	RefundID        string              `json:"refund_id"`
	OutOrderNo      string              `json:"out_order_no"`
	Amount          int64               `json:"amount"`
	Description     string              `json:"description"`
	Result          string              `json:"result"`
	SuccessTime     string              `json:"success_time"`
	SubsidyID       string              `json:"subsidy_id"`
	From            []SubsidyReturnFrom `json:"from"`
}

// SubsidyCancelRequest cancels a subsidy before settlement finalization.
type SubsidyCancelRequest struct {
	SubMchID      string
	TransactionID string
	Description   string
}

// SubsidyCancelResponse is returned by the subsidy cancel interface.
type SubsidyCancelResponse struct {
	SubMchID      string `json:"sub_mchid"`
	TransactionID string `json:"transaction_id"`
	Result        string `json:"result"`
	Description   string `json:"description"`
}

// SubsidyReturnFrom represents the funding account and amount used for a return.
type SubsidyReturnFrom struct {
	Account string `json:"account"`
	Amount  int64  `json:"amount"`
}

const (
	SubsidyResultSuccess = "SUCCESS"
	SubsidyResultFail    = "FAIL"
	SubsidyResultRefund  = "REFUND"
)

const (
	SubsidyReturnAccountAvailable   = "AVAILABLE"
	SubsidyReturnAccountUnavailable = "UNAVAILABLE"
)
