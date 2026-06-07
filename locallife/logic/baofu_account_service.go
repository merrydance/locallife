package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

var (
	ErrBaofuAccountInvalidOwnerAccount  = errors.New("baofu account owner and account type do not match")
	ErrBaofuAccountInactive             = errors.New("baofu account is not active")
	ErrBaofuAccountReceiverRequired     = errors.New("baofu account receiver id is required")
	ErrBaofuAccountOutRequestNoRequired = errors.New("baofu account out request no is required")
)

const (
	BaofuAccountOpenVerifyFeeFen = 200

	BaofuOnboardingStateProfilePending       = "profile_pending"
	BaofuOnboardingStateOpeningProcessing    = "baofu_opening_processing"
	BaofuOnboardingStateWechatChannelPending = "wechat_channel_pending"
	BaofuOnboardingStateReady                = "ready"
	BaofuOnboardingStateOpenFailed           = "open_failed"
)

type BaofuAccountReadiness struct {
	State        string
	Label        string
	PaymentReady bool
	SubMchID     string
}

type baofuAccountStore interface {
	UpsertBaofuAccountBinding(ctx context.Context, arg db.UpsertBaofuAccountBindingParams) (db.BaofuAccountBinding, error)
	CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error)
	MarkBaofuAccountBindingActiveWithFeeLedgerTx(ctx context.Context, arg db.MarkBaofuAccountBindingActiveWithFeeLedgerTxParams) (db.MarkBaofuAccountBindingActiveWithFeeLedgerTxResult, error)
	MarkBaofuAccountBindingFailed(ctx context.Context, arg db.MarkBaofuAccountBindingFailedParams) (db.BaofuAccountBinding, error)
	MarkBaofuAccountBindingAbnormal(ctx context.Context, arg db.MarkBaofuAccountBindingAbnormalParams) (db.BaofuAccountBinding, error)
}

type BaofuAccountClient interface {
	OpenAccount(ctx context.Context, req baofucontracts.OpenAccountRequest) (*baofucontracts.AccountResult, error)
	QueryAccount(ctx context.Context, req baofucontracts.QueryAccountRequest) (*baofucontracts.AccountResult, error)
}

type BaofuAccountService struct {
	store  baofuAccountStore
	client BaofuAccountClient
	now    func() time.Time
}

func NewBaofuAccountService(store baofuAccountStore, client BaofuAccountClient) *BaofuAccountService {
	return &BaofuAccountService{store: store, client: client, now: time.Now}
}

func (s *BaofuAccountService) ValidateOwnerAccount(ownerType, accountType string) error {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		if accountType == db.BaofuAccountTypeBusiness || accountType == db.BaofuAccountTypePersonal {
			return nil
		}
	case db.BaofuAccountOwnerTypeRider:
		if accountType == db.BaofuAccountTypePersonal {
			return nil
		}
	case db.BaofuAccountOwnerTypeOperator:
		if accountType == db.BaofuAccountTypePersonal {
			return nil
		}
	case db.BaofuAccountOwnerTypePlatform:
		if accountType == db.BaofuAccountTypeBusiness {
			return nil
		}
	}
	return ErrBaofuAccountInvalidOwnerAccount
}

func (s *BaofuAccountService) ValidatePaymentReady(binding db.BaofuAccountBinding) error {
	if strings.TrimSpace(binding.OpenState) != db.BaofuAccountOpenStateActive {
		return ErrBaofuAccountInactive
	}
	if strings.TrimSpace(binding.SharingMerID.String) == "" {
		return ErrBaofuAccountReceiverRequired
	}
	return nil
}

func (s *BaofuAccountService) ReadinessFromBinding(binding db.BaofuAccountBinding, found bool) BaofuAccountReadiness {
	if !found {
		return baofuAccountReadiness(BaofuOnboardingStateProfilePending, false)
	}
	if strings.TrimSpace(binding.OwnerType) != "" || strings.TrimSpace(binding.AccountType) != "" {
		if err := s.ValidateOwnerAccount(binding.OwnerType, binding.AccountType); err != nil {
			return baofuAccountReadiness(BaofuOnboardingStateOpenFailed, false)
		}
	}
	switch strings.TrimSpace(binding.OpenState) {
	case db.BaofuAccountOpenStateProcessing:
		return baofuAccountReadiness(BaofuOnboardingStateOpeningProcessing, false)
	case db.BaofuAccountOpenStateActive:
		if strings.TrimSpace(binding.SharingMerID.String) == "" {
			return baofuAccountReadiness(BaofuOnboardingStateOpenFailed, false)
		}
		return baofuAccountReadiness(BaofuOnboardingStateReady, true)
	default:
		return baofuAccountReadiness(BaofuOnboardingStateOpenFailed, false)
	}
}

func (s *BaofuAccountService) OpenAccount(ctx context.Context, req baofucontracts.OpenAccountRequest) (*baofucontracts.AccountResult, error) {
	if s == nil || s.store == nil || s.client == nil {
		return nil, errors.New("baofu account service is not configured")
	}
	if err := s.ValidateOwnerAccount(req.OwnerType, req.AccountType); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.OutRequestNo) == "" {
		return nil, ErrBaofuAccountOutRequestNoRequired
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	rawSnapshot := []byte(`{"state":"submitted"}`)
	binding, err := s.store.UpsertBaofuAccountBinding(ctx, db.UpsertBaofuAccountBindingParams{
		OwnerType:             strings.TrimSpace(req.OwnerType),
		OwnerID:               req.OwnerID,
		AccountType:           strings.TrimSpace(req.AccountType),
		OpeningMode:           baofuAccountOpeningModeForOwnerAccount(req.OwnerType, req.AccountType),
		LoginNo:               pgtype.Text{},
		OpenState:             db.BaofuAccountOpenStateProcessing,
		WechatSubMchID:        pgtype.Text{},
		LastOpenTransSerialNo: pgtype.Text{String: strings.TrimSpace(req.OutRequestNo), Valid: strings.TrimSpace(req.OutRequestNo) != ""},
		RawSnapshot:           rawSnapshot,
	})
	if err != nil {
		return nil, err
	}
	_, err = s.store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuAccount,
		CommandType:        db.ExternalPaymentCommandTypeOpenBaofuAccount,
		BusinessOwner:      businessOwnerForBaofuAccount(req.OwnerType),
		BusinessObjectType: pgtype.Text{String: "baofu_account_binding", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: binding.ID, Valid: true},
		ExternalObjectType: "baofu_account",
		ExternalObjectKey:  strings.TrimSpace(req.OutRequestNo),
		CommandStatus:      db.ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:        s.now().UTC(),
		ResponseSnapshot:   rawSnapshot,
	})
	if err != nil {
		return nil, err
	}
	result, err := s.client.OpenAccount(ctx, req)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("baofu account open returned empty result")
	}
	normalized := result.Normalized()
	switch normalized.OpenState {
	case db.BaofuAccountOpenStateActive:
		_, err = s.store.MarkBaofuAccountBindingActiveWithFeeLedgerTx(ctx, db.MarkBaofuAccountBindingActiveWithFeeLedgerTxParams{
			ActiveBinding: db.MarkBaofuAccountBindingActiveParams{
				ID:           binding.ID,
				ContractNo:   pgtype.Text{String: normalized.ContractNo, Valid: normalized.ContractNo != ""},
				SharingMerID: pgtype.Text{String: normalized.SharingMerID, Valid: normalized.SharingMerID != ""},
				RawSnapshot:  baofuAccountRawSnapshot(normalized.Raw),
			},
			AccountOpenFeeLedger: db.CreateBaofuFeeLedgerParams{
				FeeType:            db.BaofuFeeTypeAccountOpenVerifyFee,
				PayerType:          db.BaofuFeePayerTypePlatform,
				PayerID:            pgtype.Int8{Valid: false},
				BusinessObjectType: "baofu_account_binding",
				BusinessObjectID:   binding.ID,
				Amount:             BaofuAccountOpenVerifyFeeFen,
				Status:             "recorded",
			},
		})
	case db.BaofuAccountOpenStateFailed:
		_, err = s.store.MarkBaofuAccountBindingFailed(ctx, db.MarkBaofuAccountBindingFailedParams{ID: binding.ID, RawSnapshot: baofuAccountOpenResultFailureSnapshot(normalized.FailCode, normalized.Raw)})
	case db.BaofuAccountOpenStateAbnormal:
		_, err = s.store.MarkBaofuAccountBindingAbnormal(ctx, db.MarkBaofuAccountBindingAbnormalParams{ID: binding.ID, RawSnapshot: baofuAccountRawSnapshot(normalized.Raw)})
	}
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func baofuAccountReadiness(state string, paymentReady bool) BaofuAccountReadiness {
	return BaofuAccountReadiness{
		State:        state,
		Label:        baofuOnboardingStateLabel(state),
		PaymentReady: paymentReady,
	}
}

func baofuOnboardingStateLabel(state string) string {
	switch state {
	case BaofuOnboardingStateOpeningProcessing:
		return "宝付开户处理中"
	case BaofuOnboardingStateWechatChannelPending:
		return "微信支付通道待开通"
	case BaofuOnboardingStateReady:
		return "结算账户可用"
	case BaofuOnboardingStateOpenFailed:
		return "开通失败"
	case db.BaofuAccountOpeningStateVerifyFeePending, db.BaofuAccountOpeningStateVerifyFeeProcessing:
		return "核验费待确认"
	case db.BaofuAccountOpeningStateOpeningProcessing:
		return "宝付开户处理中"
	case db.BaofuAccountOpeningStateMerchantReportProcessing:
		return "商户报备处理中"
	case db.BaofuAccountOpeningStateAppletAuthPending:
		return "授权目录绑定中"
	case db.BaofuAccountOpeningStateFailed:
		return "开通失败"
	default:
		return "资料待提交"
	}
}

func baofuAccountRawSnapshot(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	if safe := baofuSafeAccountSnapshot(raw); len(safe) > 0 {
		return safe
	}
	return []byte(`{}`)
}

func baofuSafeAccountSnapshot(raw []byte) []byte {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	if provider, _ := payload["provider"].(string); strings.TrimSpace(provider) == "baofu" {
		return baofuOpeningSnapshot(baofuAccountSnapshotWhitelist(payload))
	}
	safe := map[string]any{
		"provider":   "baofu",
		"capability": "account",
	}
	if value, ok := safeString(payload["retCode"]); ok {
		safe["ret_code"] = value
	}
	if result, ok := firstAccountSnapshotResult(payload["result"]); ok {
		if value, ok := safeString(result["state"]); ok {
			safe["result_state"] = value
		}
		if value, ok := safeString(result["errorCode"]); ok {
			safe["source_path"] = "body.result[0].errorCode"
			safe["result_error_code"] = value
		} else {
			safe["source_path"] = "body.result[0]"
		}
		if message, ok := safeString(result["errorMsg"]); ok && strings.TrimSpace(message) != "" {
			if sanitized := baofu.SanitizeUpstreamMessageForRecord(message); sanitized != "" {
				safe["result_error_message_sanitized"] = sanitized
			}
			safe["result_error_message_present"] = true
		}
	} else {
		if value, ok := safeString(payload["state"]); ok {
			safe["result_state"] = value
			safe["source_path"] = "body.state"
		}
		if value, ok := safeString(payload["errorCode"]); ok {
			safe["source_path"] = "body.errorCode"
			safe["result_error_code"] = value
		}
		if message, ok := safeString(payload["errorMsg"]); ok && strings.TrimSpace(message) != "" {
			if sanitized := baofu.SanitizeUpstreamMessageForRecord(message); sanitized != "" {
				safe["result_error_message_sanitized"] = sanitized
			}
			safe["result_error_message_present"] = true
		}
	}
	return baofuOpeningSnapshot(baofuAccountSnapshotWhitelist(safe))
}

func baofuAccountSnapshotWhitelist(payload map[string]any) map[string]any {
	safe := make(map[string]any, len(payload))
	for _, key := range []string{
		"provider",
		"capability",
		"operation",
		"source_path",
		"ret_code",
		"result_state",
		"result_error_code",
		"result_error_message_sanitized",
		"result_error_message_present",
	} {
		if value, ok := payload[key]; ok {
			if safeValue, ok := baofuSafeDiagnosticValue(key, value); ok {
				safe[key] = safeValue
			}
		}
	}
	if len(safe) == 0 {
		return map[string]any{}
	}
	return safe
}

func firstAccountSnapshotResult(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case []any:
		if len(typed) == 0 {
			return nil, false
		}
		item, ok := typed[0].(map[string]any)
		return item, ok
	case map[string]any:
		return typed, true
	default:
		return nil, false
	}
}

func safeString(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		return trimmed, trimmed != ""
	case float64:
		text := strconv.FormatFloat(typed, 'f', -1, 64)
		return text, text != ""
	case bool:
		if typed {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
}

func baofuAccountSafeFailureMessage(code string, upstreamMessages ...string) pgtype.Text {
	for _, upstreamMessage := range upstreamMessages {
		if message := baofu.UserVisibleUpstreamReason(code, upstreamMessage); message != "" {
			return pgtype.Text{String: message, Valid: true}
		}
	}
	classified := baofu.ClassifyBaofuError(code, "")
	message := strings.TrimSpace(classified.PublicMessage)
	return pgtype.Text{String: message, Valid: message != ""}
}

func baofuAccountOpenResultFailureSnapshot(failureCode string, raw []byte) []byte {
	return baofuOpeningProviderFailureSnapshot(failureCode, baofuAccountRawSnapshot(raw))
}

func businessOwnerForBaofuAccount(ownerType string) string {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeRider:
		return "rider"
	case db.BaofuAccountOwnerTypeOperator:
		return "operator"
	case db.BaofuAccountOwnerTypePlatform:
		return "platform"
	default:
		return db.ExternalPaymentBusinessOwnerApplyment
	}
}
