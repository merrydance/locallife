package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
)

const (
	platformBaofuOpeningOwnerID = int64(0)
	baofuOpeningPaymentTTL      = 30 * time.Minute
)

var ErrBaofuAccountOnboardingNotConfigured = errors.New("baofu account onboarding service is not configured")

type baofuAccountOnboardingStore interface {
	GetBaofuAccountBindingByOwner(ctx context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error)
	GetBaofuAccountOpeningProfileByOwner(ctx context.Context, arg db.GetBaofuAccountOpeningProfileByOwnerParams) (db.BaofuAccountOpeningProfile, error)
	GetBaofuAccountOpeningProfile(ctx context.Context, id int64) (db.BaofuAccountOpeningProfile, error)
	UpsertBaofuAccountOpeningProfile(ctx context.Context, arg db.UpsertBaofuAccountOpeningProfileParams) (db.BaofuAccountOpeningProfile, error)
	GetActiveBaofuAccountOpeningFlowByOwner(ctx context.Context, arg db.GetActiveBaofuAccountOpeningFlowByOwnerParams) (db.BaofuAccountOpeningFlow, error)
	GetBaofuAccountOpeningFlowByPaymentOrder(ctx context.Context, verifyFeePaymentOrderID pgtype.Int8) (db.BaofuAccountOpeningFlow, error)
	CreateBaofuAccountOpeningFlow(ctx context.Context, arg db.CreateBaofuAccountOpeningFlowParams) (db.BaofuAccountOpeningFlow, error)
	SetBaofuAccountOpeningFlowProfilePending(ctx context.Context, arg db.SetBaofuAccountOpeningFlowProfilePendingParams) (db.BaofuAccountOpeningFlow, error)
	MarkBaofuAccountOpeningFlowVerifyFeePending(ctx context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams) (db.BaofuAccountOpeningFlow, error)
	MarkBaofuAccountOpeningFlowVerifyFeeProcessing(ctx context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeeProcessingParams) (db.BaofuAccountOpeningFlow, error)
	MarkBaofuAccountOpeningFlowOpeningProcessing(ctx context.Context, arg db.MarkBaofuAccountOpeningFlowOpeningProcessingParams) (db.BaofuAccountOpeningFlow, error)
	MarkBaofuAccountOpeningFlowMerchantReportProcessing(ctx context.Context, arg db.MarkBaofuAccountOpeningFlowMerchantReportProcessingParams) (db.BaofuAccountOpeningFlow, error)
	MarkBaofuAccountOpeningFlowReady(ctx context.Context, arg db.MarkBaofuAccountOpeningFlowReadyParams) (db.BaofuAccountOpeningFlow, error)
	MarkBaofuAccountOpeningFlowFailed(ctx context.Context, arg db.MarkBaofuAccountOpeningFlowFailedParams) (db.BaofuAccountOpeningFlow, error)
	GetBaofuAccountBindingByContractNo(ctx context.Context, contractNo pgtype.Text) (db.BaofuAccountBinding, error)
	UpsertBaofuAccountBinding(ctx context.Context, arg db.UpsertBaofuAccountBindingParams) (db.BaofuAccountBinding, error)
	MarkBaofuAccountBindingActive(ctx context.Context, arg db.MarkBaofuAccountBindingActiveParams) (db.BaofuAccountBinding, error)
	MarkBaofuAccountBindingActiveWithFeeLedgerTx(ctx context.Context, arg db.MarkBaofuAccountBindingActiveWithFeeLedgerTxParams) (db.MarkBaofuAccountBindingActiveWithFeeLedgerTxResult, error)
	MarkBaofuAccountBindingFailed(ctx context.Context, arg db.MarkBaofuAccountBindingFailedParams) (db.BaofuAccountBinding, error)
	MarkBaofuAccountBindingAbnormal(ctx context.Context, arg db.MarkBaofuAccountBindingAbnormalParams) (db.BaofuAccountBinding, error)
	GetReusableBaofuVerifyFeePayment(ctx context.Context, arg db.GetReusableBaofuVerifyFeePaymentParams) (db.PaymentOrder, error)
	CreatePaymentOrder(ctx context.Context, arg db.CreatePaymentOrderParams) (db.PaymentOrder, error)
	UpdatePaymentOrderPrepayId(ctx context.Context, arg db.UpdatePaymentOrderPrepayIdParams) (db.PaymentOrder, error)
	UpdatePaymentOrderToClosed(ctx context.Context, id int64) (db.PaymentOrder, error)
	GetUser(ctx context.Context, id int64) (db.User, error)
	CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error)
	CreatePlatformAlertEvent(ctx context.Context, arg db.CreatePlatformAlertEventParams) (db.PlatformAlertEvent, error)
}

type BaofuAccountOnboardingConfig struct {
	VerifyFeeFen int64
	IndustryID   string
}

type BaofuAccountOpeningInput struct {
	OwnerType string
	OwnerID   int64
	UserID    int64
	ClientIP  string
	Profile   *BaofuAccountOpeningProfileInput
}

type BaofuAccountOpeningProfileInput struct {
	LegalName           string `json:"legal_name,omitempty"`
	CertificateNo       string `json:"certificate_no,omitempty"`
	BusinessLicenseNo   string `json:"business_license_number,omitempty"`
	LegalPersonName     string `json:"legal_person_name,omitempty"`
	LegalPersonIDNumber string `json:"legal_person_id_number,omitempty"`
	CorporateMobile     string `json:"corporate_mobile,omitempty"`
	Email               string `json:"email,omitempty"`
	BankAccountNo       string `json:"bank_account_no,omitempty"`
	BankMobile          string `json:"bank_mobile,omitempty"`
	BankName            string `json:"bank_name,omitempty"`
	DepositBankProvince string `json:"deposit_bank_province,omitempty"`
	DepositBankCity     string `json:"deposit_bank_city,omitempty"`
	DepositBankName     string `json:"deposit_bank_name,omitempty"`
	ContactName         string `json:"contact_name,omitempty"`
	ContactMobile       string `json:"contact_mobile,omitempty"`
	CardUserName        string `json:"card_user_name,omitempty"`
	SelfEmployed        bool   `json:"self_employed,omitempty"`
	SelfEmployedSet     bool   `json:"-"`
}

type baofuAccountOpeningProfileField struct {
	code  string
	value string
}

type BaofuAccountOpeningResult struct {
	State         string
	Label         string
	StatusDesc    string
	MissingFields []string
	Flow          db.BaofuAccountOpeningFlow
	Profile       db.BaofuAccountOpeningProfile
	Binding       *db.BaofuAccountBinding
	PaymentOrder  db.PaymentOrder
	PayParams     *wechat.JSAPIPayParams
}

type BaofuAccountOpenApplyResult struct {
	Flow    db.BaofuAccountOpeningFlow
	Binding *db.BaofuAccountBinding
}

type BaofuAccountOnboardingService struct {
	store                baofuAccountOnboardingStore
	accountClient        BaofuAccountClient
	directPaymentClient  wechat.DirectPaymentClientInterface
	merchantReportClient baofuMerchantReportClient
	encryptor            util.DataEncryptor
	config               BaofuAccountOnboardingConfig
	merchantReportConfig BaofuAccountMerchantReportConfig
	now                  func() time.Time
}

func NewBaofuAccountOnboardingService(store baofuAccountOnboardingStore, accountClient BaofuAccountClient, directPaymentClient wechat.DirectPaymentClientInterface, encryptor util.DataEncryptor, config BaofuAccountOnboardingConfig) *BaofuAccountOnboardingService {
	return &BaofuAccountOnboardingService{
		store:               store,
		accountClient:       accountClient,
		directPaymentClient: directPaymentClient,
		encryptor:           encryptor,
		config:              config.normalized(),
		now:                 time.Now,
	}
}

func (s *BaofuAccountOnboardingService) WithMerchantReportContinuation(client baofuMerchantReportClient, config BaofuAccountMerchantReportConfig) *BaofuAccountOnboardingService {
	if s == nil {
		return s
	}
	s.merchantReportClient = client
	s.merchantReportConfig = config.normalized()
	return s
}

func (c BaofuAccountOnboardingConfig) normalized() BaofuAccountOnboardingConfig {
	if c.VerifyFeeFen <= 0 {
		c.VerifyFeeFen = BaofuAccountOpenVerifyFeeFen
	}
	c.IndustryID = strings.TrimSpace(c.IndustryID)
	if c.IndustryID == "" {
		c.IndustryID = "9931"
	}
	return c
}

func (s *BaofuAccountOnboardingService) StartOrRecoverOpening(ctx context.Context, input BaofuAccountOpeningInput) (BaofuAccountOpeningResult, error) {
	if s == nil || s.store == nil {
		return BaofuAccountOpeningResult{}, ErrBaofuAccountOnboardingNotConfigured
	}
	cfg := s.config.normalized()
	ownerType := strings.TrimSpace(input.OwnerType)
	accountType, err := baofuOpeningAccountType(ownerType)
	if err != nil {
		return BaofuAccountOpeningResult{}, err
	}
	ownerID := input.OwnerID
	if ownerType == db.BaofuAccountOwnerTypePlatform {
		ownerID = platformBaofuOpeningOwnerID
	}

	if binding, err := s.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{OwnerType: ownerType, OwnerID: ownerID}); err == nil && strings.TrimSpace(binding.OpenState) == db.BaofuAccountOpenStateActive {
		return s.activeBindingOpeningResult(ctx, ownerType, ownerID, binding)
	} else if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return BaofuAccountOpeningResult{}, err
	}

	profile, err := s.resolveProfile(ctx, ownerType, ownerID, accountType, input.Profile)
	if err != nil {
		return BaofuAccountOpeningResult{}, err
	}
	flow, err := s.getOrCreateFlow(ctx, ownerType, ownerID, accountType, profile)
	if err != nil {
		return BaofuAccountOpeningResult{}, err
	}
	if strings.TrimSpace(profile.ProfileStatus) != db.BaofuAccountOpeningProfileStatusComplete {
		flow, err = s.markProfilePending(ctx, flow, profile)
		if err != nil {
			return BaofuAccountOpeningResult{}, err
		}
		return baofuOpeningResult(flow, profile), nil
	}
	if baofuOpeningFlowInProviderProgress(flow.State) {
		return baofuOpeningResult(flow, profile), nil
	}

	if baofuOpeningRequiresUserFee(ownerType) {
		if strings.TrimSpace(flow.State) == db.BaofuAccountOpeningStateProfilePending {
			flow, err = s.store.MarkBaofuAccountOpeningFlowVerifyFeePending(ctx, db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams{
				ID:                      flow.ID,
				ProfileID:               pgtype.Int8{Int64: profile.ID, Valid: profile.ID > 0},
				VerifyFeeAmount:         cfg.VerifyFeeFen,
				VerifyFeePaymentOrderID: flow.VerifyFeePaymentOrderID,
				RawSnapshot:             baofuOpeningSnapshot(map[string]any{"state": db.BaofuAccountOpeningStateVerifyFeePending, "profile_id": profile.ID}),
			})
			if err != nil {
				return BaofuAccountOpeningResult{}, err
			}
		}
		payment, payParams, err := s.ensureVerifyFeePayment(ctx, flow, input.UserID, input.ClientIP, cfg)
		if err != nil {
			return BaofuAccountOpeningResult{}, err
		}
		if strings.TrimSpace(payment.Status) != "paid" {
			flow, err = s.store.MarkBaofuAccountOpeningFlowVerifyFeeProcessing(ctx, db.MarkBaofuAccountOpeningFlowVerifyFeeProcessingParams{
				ID:                      flow.ID,
				VerifyFeePaymentOrderID: pgtype.Int8{Int64: payment.ID, Valid: true},
				RawSnapshot:             baofuOpeningSnapshot(map[string]any{"state": db.BaofuAccountOpeningStateVerifyFeeProcessing, "payment_order_id": payment.ID}),
			})
			if err != nil {
				return BaofuAccountOpeningResult{}, err
			}
			result := baofuOpeningResult(flow, profile)
			result.PaymentOrder = payment
			result.PayParams = payParams
			return result, nil
		}
		flow.VerifyFeePaymentOrderID = pgtype.Int8{Int64: payment.ID, Valid: true}
	}

	flow, binding, err := s.openFromProfile(ctx, flow, profile, cfg)
	if err != nil {
		return BaofuAccountOpeningResult{}, err
	}
	result := baofuOpeningResult(flow, profile)
	if binding != nil {
		result.Binding = binding
	}
	return result, nil
}

type baofuMerchantReportLookupStore interface {
	GetBaofuMerchantReportByOwner(ctx context.Context, arg db.GetBaofuMerchantReportByOwnerParams) (db.BaofuMerchantReport, error)
}

func (s *BaofuAccountOnboardingService) activeBindingOpeningResult(ctx context.Context, ownerType string, ownerID int64, binding db.BaofuAccountBinding) (BaofuAccountOpeningResult, error) {
	bindingResult := &binding
	if strings.TrimSpace(ownerType) != db.BaofuAccountOwnerTypeMerchant {
		return BaofuAccountOpeningResult{State: db.BaofuAccountOpeningStateReady, Label: baofuOnboardingStateLabel(db.BaofuAccountOpeningStateReady), Binding: bindingResult}, nil
	}
	reportStore, ok := s.store.(baofuMerchantReportLookupStore)
	if !ok {
		return BaofuAccountOpeningResult{State: db.BaofuAccountOpeningStateMerchantReportProcessing, Label: baofuOnboardingStateLabel(db.BaofuAccountOpeningStateMerchantReportProcessing), Binding: bindingResult}, nil
	}
	report, err := reportStore.GetBaofuMerchantReportByOwner(ctx, db.GetBaofuMerchantReportByOwnerParams{
		OwnerType:  db.BaofuAccountOwnerTypeMerchant,
		OwnerID:    ownerID,
		ReportType: db.BaofuMerchantReportTypeWechat,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return BaofuAccountOpeningResult{State: db.BaofuAccountOpeningStateMerchantReportProcessing, Label: baofuOnboardingStateLabel(db.BaofuAccountOpeningStateMerchantReportProcessing), Binding: bindingResult}, nil
		}
		return BaofuAccountOpeningResult{}, err
	}
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateFailed ||
		strings.TrimSpace(report.AppletAuthState) == db.BaofuMerchantReportAppletAuthStateFailed {
		return BaofuAccountOpeningResult{State: db.BaofuAccountOpeningStateFailed, Label: baofuOnboardingStateLabel(db.BaofuAccountOpeningStateFailed), StatusDesc: baofuAccountMerchantReportFailureStatusDesc(report), Binding: bindingResult}, nil
	}
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateSucceeded &&
		strings.TrimSpace(report.AppletAuthState) == db.BaofuMerchantReportAppletAuthStateSucceeded {
		return BaofuAccountOpeningResult{State: db.BaofuAccountOpeningStateReady, Label: baofuOnboardingStateLabel(db.BaofuAccountOpeningStateReady), Binding: bindingResult}, nil
	}
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateSucceeded {
		return BaofuAccountOpeningResult{State: db.BaofuAccountOpeningStateAppletAuthPending, Label: baofuOnboardingStateLabel(db.BaofuAccountOpeningStateAppletAuthPending), Binding: bindingResult}, nil
	}
	return BaofuAccountOpeningResult{State: db.BaofuAccountOpeningStateMerchantReportProcessing, Label: baofuOnboardingStateLabel(db.BaofuAccountOpeningStateMerchantReportProcessing), Binding: bindingResult}, nil
}

func baofuAccountMerchantReportFailureStatusDesc(report db.BaofuMerchantReport) string {
	if strings.TrimSpace(report.AppletAuthState) == db.BaofuMerchantReportAppletAuthStateFailed {
		return "微信支付授权目录绑定失败，请联系平台处理后重试"
	}
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateFailed {
		return "微信支付商户报备失败，请核对商户资料后重试；如持续失败请联系平台处理"
	}
	return "开户未通过，请核对资料后重试"
}

func (s *BaofuAccountOnboardingService) ContinueAfterVerifyFeePaid(ctx context.Context, paymentOrder db.PaymentOrder) error {
	if s == nil || s.store == nil {
		return ErrBaofuAccountOnboardingNotConfigured
	}
	flow, err := s.store.GetBaofuAccountOpeningFlowByPaymentOrder(ctx, pgtype.Int8{Int64: paymentOrder.ID, Valid: paymentOrder.ID > 0})
	if err != nil {
		return err
	}
	if !baofuOpeningRequiresUserFee(flow.OwnerType) {
		return nil
	}
	profileID := flow.ProfileID
	if !profileID.Valid {
		return errors.New("baofu account opening profile is required")
	}
	profile, err := s.store.GetBaofuAccountOpeningProfile(ctx, profileID.Int64)
	if err != nil {
		return err
	}
	if strings.TrimSpace(profile.ProfileStatus) != db.BaofuAccountOpeningProfileStatusComplete {
		_, err = s.markProfilePending(ctx, flow, profile)
		return err
	}
	_, _, err = s.openFromProfile(ctx, flow, profile, s.config.normalized())
	return err
}
