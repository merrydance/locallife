package ordinaryserviceprovider

import (
	"context"
	"net/http"

	"github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
)

type NotificationTarget string

const (
	NotificationTargetPayment           NotificationTarget = "payment"
	NotificationTargetCombinePayment    NotificationTarget = "combine_payment"
	NotificationTargetRefund            NotificationTarget = "refund"
	NotificationTargetProfitSharing     NotificationTarget = "profit_sharing"
	NotificationTargetMerchantViolation NotificationTarget = "merchant_violation"
)

type OrdinaryServiceProviderClientInterface interface {
	ServiceProviderAppID() string
	ServiceProviderMchID() string
	ServiceProviderMchName() string
	PaymentNotifyURL() string
	CombineNotifyURL() string
	RefundNotifyURL() string
	ProfitSharingNotifyURL() string
	GenerateJSAPIPayParams(prepayID string) (*contracts.JSAPIPayParams, error)

	SubmitApplyment(ctx context.Context, req contracts.ApplymentSubmitRequest) (*contracts.ApplymentSubmitResponse, error)
	QueryApplymentByID(ctx context.Context, req contracts.ApplymentQueryByIDRequest) (*contracts.ApplymentQueryResponse, error)
	QueryApplymentByBusinessCode(ctx context.Context, req contracts.ApplymentQueryByBusinessCodeRequest) (*contracts.ApplymentQueryResponse, error)
	QuerySettlement(ctx context.Context, req contracts.SettlementQueryRequest) (*contracts.SettlementQueryResponse, error)
	ModifySettlement(ctx context.Context, req contracts.SettlementModifyRequest) (*contracts.SettlementModifyResponse, error)
	QuerySettlementModification(ctx context.Context, req contracts.SettlementModificationQueryRequest) (*contracts.SettlementModificationQueryResponse, error)

	QueryMerchantLimitation(ctx context.Context, req contracts.MerchantLimitationQueryRequest) (*contracts.MerchantLimitationQueryResponse, error)
	CreateViolationNotificationConfig(ctx context.Context, req contracts.ViolationNotificationConfigRequest) (*contracts.ViolationNotificationConfigResponse, error)
	QueryViolationNotificationConfig(ctx context.Context) (*contracts.ViolationNotificationConfigResponse, error)
	UpdateViolationNotificationConfig(ctx context.Context, req contracts.ViolationNotificationConfigRequest) (*contracts.ViolationNotificationConfigResponse, error)
	DeleteViolationNotificationConfig(ctx context.Context) error
	CreateInactiveMerchantIdentityVerification(ctx context.Context, req contracts.InactiveMerchantIdentityVerificationCreateRequest) (*contracts.InactiveMerchantIdentityVerificationCreateResponse, error)
	QueryInactiveMerchantIdentityVerification(ctx context.Context, req contracts.InactiveMerchantIdentityVerificationQueryRequest) (*contracts.InactiveMerchantIdentityVerificationQueryResponse, error)

	CreatePayment(ctx context.Context, req contracts.PaymentPrepayRequest) (*contracts.PaymentPrepayResponse, error)
	QueryPayment(ctx context.Context, req contracts.PaymentQueryRequest) (*contracts.PaymentQueryResponse, error)
	ClosePayment(ctx context.Context, req contracts.PaymentCloseRequest) error
	CreateCombinePayment(ctx context.Context, req contracts.CombinePrepayRequest) (*contracts.CombinePrepayResponse, error)
	QueryCombinePayment(ctx context.Context, req contracts.CombineQueryRequest) (*contracts.CombineQueryResponse, error)
	CloseCombinePayment(ctx context.Context, req contracts.CombineCloseRequest) error

	CreateRefund(ctx context.Context, req contracts.RefundCreateRequest) (*contracts.RefundResponse, error)
	QueryRefund(ctx context.Context, req contracts.RefundQueryRequest) (*contracts.RefundResponse, error)

	EncryptSensitiveData(plaintext string) (string, error)
	DecryptSensitiveResponseData(ciphertext string) (string, error)
	UploadImage(ctx context.Context, filename string, fileData []byte) (*contracts.MediaUploadResponse, error)
	ListPersonalBankingBanks(ctx context.Context, offset, limit int) (*contracts.CapitalBankListResponse, error)
	ListCorporateBankingBanks(ctx context.Context, offset, limit int) (*contracts.CapitalBankListResponse, error)
	SearchBanksByBankAccount(ctx context.Context, accountNumber string) (*contracts.CapitalBankAccountSearchResponse, error)
	ListProvinceAreas(ctx context.Context) (*contracts.CapitalProvinceListResponse, error)
	ListCityAreas(ctx context.Context, provinceCode int) (*contracts.CapitalCityListResponse, error)
	ListBankBranches(ctx context.Context, bankAliasCode string, cityCode, offset, limit int) (*contracts.CapitalBranchListResponse, error)

	AddProfitSharingReceiver(ctx context.Context, req contracts.ProfitSharingReceiverAddRequest) (*contracts.ProfitSharingReceiverResponse, error)
	DeleteProfitSharingReceiver(ctx context.Context, req contracts.ProfitSharingReceiverDeleteRequest) (*contracts.ProfitSharingReceiverResponse, error)
	CreateProfitSharingOrder(ctx context.Context, req contracts.ProfitSharingOrderRequest) (*contracts.ProfitSharingOrderResponse, error)
	QueryProfitSharingOrder(ctx context.Context, req contracts.ProfitSharingQueryRequest) (*contracts.ProfitSharingOrderResponse, error)
	CreateProfitSharingReturn(ctx context.Context, req contracts.ProfitSharingReturnRequest) (*contracts.ProfitSharingReturnResponse, error)
	QueryProfitSharingReturn(ctx context.Context, req contracts.ProfitSharingReturnQueryRequest) (*contracts.ProfitSharingReturnResponse, error)
	UnfreezeProfitSharing(ctx context.Context, req contracts.ProfitSharingUnfreezeRequest) (*contracts.ProfitSharingUnfreezeResponse, error)
	QueryProfitSharingRemainingAmount(ctx context.Context, req contracts.ProfitSharingRemainingAmountRequest) (*contracts.ProfitSharingRemainingAmountResponse, error)

	ParseNotification(ctx context.Context, request *http.Request, target NotificationTarget) (*NotificationEnvelope, error)
}
