package contracts

const (
	DirectMerchantTransferSceneEnterpriseCompensation = "1011"
)

const (
	DirectMerchantTransferUserRecvPerceptionRefund               = "退款"
	DirectMerchantTransferUserRecvPerceptionMerchantCompensation = "商家赔付"
)

const (
	DirectMerchantTransferReportInfoTypeCompensationReason = "赔付原因"
)

const (
	DirectMerchantTransferStateAccepted        = "ACCEPTED"
	DirectMerchantTransferStateProcessing      = "PROCESSING"
	DirectMerchantTransferStateWaitUserConfirm = "WAIT_USER_CONFIRM"
	DirectMerchantTransferStateTransfering     = "TRANSFERING"
	DirectMerchantTransferStateSuccess         = "SUCCESS"
	DirectMerchantTransferStateFail            = "FAIL"
	DirectMerchantTransferStateCanceling       = "CANCELING"
	DirectMerchantTransferStateCancelled       = "CANCELLED"
)

const (
	DirectMerchantTransferNotifyEventTypeBillFinished = "MCHTRANSFER.BILL.FINISHED"
	DirectMerchantTransferNotifyResourceTypeEncrypt   = "encrypt-resource"
	DirectMerchantTransferNotifyOriginalTypePayment   = "mch_payment"
	DirectMerchantTransferNotifyAlgorithmAES256GCM    = "AEAD_AES_256_GCM"
)

const (
	DirectMerchantTransferPaymentMethodTypeWallet   = "CFT"
	DirectMerchantTransferPaymentMethodTypeHKWallet = "WPHK"
)

const (
	RequestMerchantTransferResultOK     = "requestMerchantTransfer:ok"
	RequestMerchantTransferResultFail   = "requestMerchantTransfer:fail"
	RequestMerchantTransferResultCancel = "requestMerchantTransfer:cancel"
)

const (
	DirectMerchantTransferFailReasonAccountFrozen                    = "ACCOUNT_FROZEN"
	DirectMerchantTransferFailReasonAccountNotExist                  = "ACCOUNT_NOT_EXIST"
	DirectMerchantTransferFailReasonBankCardAccountAbnormal          = "BANK_CARD_ACCOUNT_ABNORMAL"
	DirectMerchantTransferFailReasonBankCardBankInfoWrong            = "BANK_CARD_BANK_INFO_WRONG"
	DirectMerchantTransferFailReasonBankCardCardInfoWrong            = "BANK_CARD_CARD_INFO_WRONG"
	DirectMerchantTransferFailReasonBankCardCollectionsAboveQuota    = "BANK_CARD_COLLECTIONS_ABOVE_QUOTA"
	DirectMerchantTransferFailReasonBankCardParamError               = "BANK_CARD_PARAM_ERROR"
	DirectMerchantTransferFailReasonBankCardStatusAbnormal           = "BANK_CARD_STATUS_ABNORMAL"
	DirectMerchantTransferFailReasonBlockBSRuleMonthLimit            = "BLOCK_B2C_USERLIMITAMOUNT_BSRULE_MONTH"
	DirectMerchantTransferFailReasonBlockMonthLimit                  = "BLOCK_B2C_USERLIMITAMOUNT_MONTH"
	DirectMerchantTransferFailReasonDayReceivedCountExceed           = "DAY_RECEIVED_COUNT_EXCEED"
	DirectMerchantTransferFailReasonDayReceivedQuotaExceed           = "DAY_RECEIVED_QUOTA_EXCEED"
	DirectMerchantTransferFailReasonExceededEstimatedAmount          = "EXCEEDED_ESTIMATED_AMOUNT"
	DirectMerchantTransferFailReasonIDCardNotCorrect                 = "ID_CARD_NOT_CORRECT"
	DirectMerchantTransferFailReasonMerchantCancel                   = "MCH_CANCEL"
	DirectMerchantTransferFailReasonMerchantReject                   = "MERCHANT_REJECT"
	DirectMerchantTransferFailReasonMerchantNotConfirm               = "MERCHANT_NOT_CONFIRM"
	DirectMerchantTransferFailReasonNameNotCorrect                   = "NAME_NOT_CORRECT"
	DirectMerchantTransferFailReasonOpenIDInvalid                    = "OPENID_INVALID"
	DirectMerchantTransferFailReasonOther                            = "OTHER_FAIL_REASON_TYPE"
	DirectMerchantTransferFailReasonOverdueClose                     = "OVERDUE_CLOSE"
	DirectMerchantTransferFailReasonPayeeAccountAbnormal             = "PAYEE_ACCOUNT_ABNORMAL"
	DirectMerchantTransferFailReasonPayerAccountAbnormal             = "PAYER_ACCOUNT_ABNORMAL"
	DirectMerchantTransferFailReasonProductAuthCheckFail             = "PRODUCT_AUTH_CHECK_FAIL"
	DirectMerchantTransferFailReasonRealnameAccountQuotaExceed       = "REALNAME_ACCOUNT_RECEIVED_QUOTA_EXCEED"
	DirectMerchantTransferFailReasonRealNameCheckFail                = "REAL_NAME_CHECK_FAIL"
	DirectMerchantTransferFailReasonReceiveAccountNotConfigure       = "RECEIVE_ACCOUNT_NOT_CONFIGURE"
	DirectMerchantTransferFailReasonReservationInfoNotMatch          = "RESERVATION_INFO_NOT_MATCH"
	DirectMerchantTransferFailReasonReservationSceneNotMatch         = "RESERVATION_SCENE_NOT_MATCH"
	DirectMerchantTransferFailReasonReservationStateInvalid          = "RESERVATION_STATE_INVALID"
	DirectMerchantTransferFailReasonTransferQuotaExceed              = "TRANSFER_QUOTA_EXCEED"
	DirectMerchantTransferFailReasonTransferRemarkSetFail            = "TRANSFER_REMARK_SET_FAIL"
	DirectMerchantTransferFailReasonTransferRisk                     = "TRANSFER_RISK"
	DirectMerchantTransferFailReasonTransferSceneInvalid             = "TRANSFER_SCENE_INVALID"
	DirectMerchantTransferFailReasonTransferSceneUnavailable         = "TRANSFER_SCENE_UNAVAILABLE"
	DirectMerchantTransferFailReasonRelatedOrderTransferAmountExceed = "RELATED_ORDER_TRANSFER_AMOUNT_EXCEED"
	DirectMerchantTransferFailReasonRelatedOrderTransferCountExceed  = "RELATED_ORDER_TRANSFER_COUNT_EXCEED"
	DirectMerchantTransferFailReasonBudgetNotEnough                  = "BUDGET_NOT_ENOUGH"
)

type DirectMerchantTransferSceneReportInfo struct {
	InfoType    string `json:"info_type"`
	InfoContent string `json:"info_content"`
}

// DirectMerchantTransferCreateRequest is the caller-shaped contract for
// merchant transfer creation in the direct merchant capability group.
type DirectMerchantTransferCreateRequest struct {
	AppID                    string
	OutBillNo                string
	TransferSceneID          string
	OpenID                   string
	UserName                 string
	TransferAmount           int64
	TransferRemark           string
	NotifyURL                string
	UserRecvPerception       string
	TransferSceneReportInfos []DirectMerchantTransferSceneReportInfo
}

// DirectMerchantTransferCreateRequestBody is the official wire request body
// for POST /v3/fund-app/mch-transfer/transfer-bills.
type DirectMerchantTransferCreateRequestBody struct {
	AppID                    string                                  `json:"appid"`
	OutBillNo                string                                  `json:"out_bill_no"`
	TransferSceneID          string                                  `json:"transfer_scene_id"`
	OpenID                   string                                  `json:"openid"`
	UserName                 string                                  `json:"user_name,omitempty"`
	TransferAmount           int64                                   `json:"transfer_amount"`
	TransferRemark           string                                  `json:"transfer_remark"`
	NotifyURL                string                                  `json:"notify_url,omitempty"`
	UserRecvPerception       string                                  `json:"user_recv_perception,omitempty"`
	TransferSceneReportInfos []DirectMerchantTransferSceneReportInfo `json:"transfer_scene_report_infos"`
}

type DirectMerchantTransferCreateResponse struct {
	OutBillNo      string `json:"out_bill_no"`
	TransferBillNo string `json:"transfer_bill_no"`
	CreateTime     string `json:"create_time"`
	State          string `json:"state"`
	PackageInfo    string `json:"package_info,omitempty"`
}

type DirectMerchantTransferQueryByOutBillNoRequest struct {
	OutBillNo string `json:"out_bill_no"`
}

type DirectMerchantTransferQueryByTransferBillNoRequest struct {
	TransferBillNo string `json:"transfer_bill_no"`
}

type DirectMerchantTransferQueryResponse struct {
	MchID          string `json:"mch_id"`
	OutBillNo      string `json:"out_bill_no"`
	TransferBillNo string `json:"transfer_bill_no"`
	AppID          string `json:"appid"`
	State          string `json:"state"`
	TransferAmount int64  `json:"transfer_amount"`
	TransferRemark string `json:"transfer_remark"`
	FailReason     string `json:"fail_reason,omitempty"`
	OpenID         string `json:"openid,omitempty"`
	UserName       string `json:"user_name,omitempty"`
	CreateTime     string `json:"create_time"`
	UpdateTime     string `json:"update_time"`
}

type DirectMerchantTransferCancelRequest struct {
	OutBillNo string `json:"out_bill_no"`
}

type DirectMerchantTransferCancelResponse struct {
	OutBillNo      string `json:"out_bill_no"`
	TransferBillNo string `json:"transfer_bill_no"`
	State          string `json:"state"`
	UpdateTime     string `json:"update_time"`
}

type DirectMerchantTransferNotification struct {
	ID           string                                              `json:"id"`
	CreateTime   string                                              `json:"create_time"`
	ResourceType string                                              `json:"resource_type"`
	EventType    string                                              `json:"event_type"`
	Summary      string                                              `json:"summary"`
	Resource     DirectMerchantTransferEncryptedNotificationResource `json:"resource"`
}

type DirectMerchantTransferEncryptedNotificationResource struct {
	OriginalType   string `json:"original_type"`
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	AssociatedData string `json:"associated_data,omitempty"`
	Nonce          string `json:"nonce"`
}

type DirectMerchantTransferNotificationResource struct {
	OutBillNo         string `json:"out_bill_no"`
	TransferBillNo    string `json:"transfer_bill_no"`
	State             string `json:"state"`
	MchID             string `json:"mch_id"`
	TransferAmount    int64  `json:"transfer_amount"`
	OpenID            string `json:"openid"`
	FailReason        string `json:"fail_reason,omitempty"`
	CreateTime        string `json:"create_time"`
	UpdateTime        string `json:"update_time"`
	PaymentMethodType string `json:"payment_method_type,omitempty"`
}

// RequestMerchantTransferParams is the canonical wx.requestMerchantTransfer
// parameter contract consumed by Mini Program or JSAPI callers.
type RequestMerchantTransferParams struct {
	MchID   string `json:"mchId"`
	AppID   string `json:"appId"`
	Package string `json:"package"`
}
