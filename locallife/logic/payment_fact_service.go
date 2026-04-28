package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

type paymentFactRefundCreator interface {
	CreateEcommerceRefund(ctx context.Context, req *wechatcontracts.EcommerceRefundRequest) (*wechatcontracts.EcommerceRefundCreateResponse, error)
}

type paymentFactEcommerceRefundCreatorAdapter struct {
	client wechat.EcommerceClientInterface
}

func (a paymentFactEcommerceRefundCreatorAdapter) CreateEcommerceRefund(ctx context.Context, req *wechatcontracts.EcommerceRefundRequest) (*wechatcontracts.EcommerceRefundCreateResponse, error) {
	return createEcommerceRefundContract(ctx, a.client, req)
}

type PaymentFactService struct {
	store              db.Store
	now                func() time.Time
	ecommerceClient    wechat.EcommerceClientInterface
	refundCreator      paymentFactRefundCreator
	riderAverageSpeed  int
	defaultPrepareTime int
}

func NewPaymentFactService(store db.Store) *PaymentFactService {
	return &PaymentFactService{
		store: store,
		now:   time.Now,
	}
}

func (svc *PaymentFactService) WithEcommerceClient(client wechat.EcommerceClientInterface) *PaymentFactService {
	svc.ecommerceClient = client
	svc.refundCreator = nil
	if client != nil {
		svc.refundCreator = paymentFactEcommerceRefundCreatorAdapter{client: client}
	}
	return svc
}

func (svc *PaymentFactService) WithRefundCreator(creator paymentFactRefundCreator) *PaymentFactService {
	svc.refundCreator = creator
	return svc
}

func (svc *PaymentFactService) WithPaymentSuccessConfig(riderAverageSpeed, defaultPrepareTime int) *PaymentFactService {
	svc.riderAverageSpeed = riderAverageSpeed
	svc.defaultPrepareTime = defaultPrepareTime
	return svc
}

type RecordExternalPaymentFactInput struct {
	Provider                    string
	Channel                     string
	Capability                  string
	FactSource                  string
	SourceEventID               *string
	SourceEventType             *string
	ExternalObjectType          string
	ExternalObjectKey           string
	ExternalSecondaryKey        *string
	BusinessOwner               *string
	BusinessObjectType          *string
	BusinessObjectID            *int64
	UpstreamState               string
	TerminalStatus              string
	Amount                      *int64
	Currency                    string
	OccurredAt                  *time.Time
	UpstreamUpdatedAt           *time.Time
	ObservedAt                  *time.Time
	RawResource                 []byte
	DedupeKey                   string
	Application                 *ExternalPaymentFactApplicationTarget
	AllowNonTerminalApplication bool
}

type ExternalPaymentFactApplicationTarget struct {
	Consumer           string
	BusinessObjectType string
	BusinessObjectID   int64
}

type RecordExternalPaymentFactResult struct {
	Fact        db.ExternalPaymentFact
	Application *db.ExternalPaymentFactApplication
}

func (svc *PaymentFactService) RecordExternalPaymentFact(ctx context.Context, input RecordExternalPaymentFactInput) (RecordExternalPaymentFactResult, error) {
	var result RecordExternalPaymentFactResult

	if err := validateRecordExternalPaymentFactInput(input); err != nil {
		return result, err
	}

	observedAt := svc.resolveObservedAt(input.ObservedAt)
	rawResource := input.RawResource
	if len(rawResource) == 0 {
		rawResource = []byte(`{}`)
	}
	currency := input.Currency
	if currency == "" {
		currency = "CNY"
	}

	isTerminal := isExternalPaymentTerminalStatus(input.TerminalStatus)
	fact, err := svc.store.CreateExternalPaymentFact(ctx, db.CreateExternalPaymentFactParams{
		Provider:             input.Provider,
		Channel:              input.Channel,
		Capability:           input.Capability,
		FactSource:           input.FactSource,
		SourceEventID:        textFromStringPtr(input.SourceEventID),
		SourceEventType:      textFromStringPtr(input.SourceEventType),
		ExternalObjectType:   input.ExternalObjectType,
		ExternalObjectKey:    input.ExternalObjectKey,
		ExternalSecondaryKey: textFromStringPtr(input.ExternalSecondaryKey),
		BusinessOwner:        textFromStringPtr(input.BusinessOwner),
		BusinessObjectType:   textFromStringPtr(input.BusinessObjectType),
		BusinessObjectID:     int8FromInt64Ptr(input.BusinessObjectID),
		UpstreamState:        input.UpstreamState,
		TerminalStatus:       input.TerminalStatus,
		IsTerminal:           isTerminal,
		Amount:               int8FromInt64Ptr(input.Amount),
		Currency:             currency,
		OccurredAt:           timestamptzFromTimePtr(input.OccurredAt),
		UpstreamUpdatedAt:    timestamptzFromTimePtr(input.UpstreamUpdatedAt),
		ObservedAt:           observedAt,
		RawResource:          rawResource,
		DedupeKey:            input.DedupeKey,
		ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
	})
	if err != nil {
		return result, err
	}
	result.Fact = fact

	if input.Application == nil {
		return result, nil
	}
	if !fact.IsTerminal && !input.AllowNonTerminalApplication {
		return result, nil
	}

	application, err := svc.store.CreateExternalPaymentFactApplication(ctx, db.CreateExternalPaymentFactApplicationParams{
		FactID:             fact.ID,
		Consumer:           input.Application.Consumer,
		BusinessObjectType: input.Application.BusinessObjectType,
		BusinessObjectID:   input.Application.BusinessObjectID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	})
	if err != nil {
		return result, err
	}
	result.Application = &application

	return result, nil
}

func (svc *PaymentFactService) resolveObservedAt(observedAt *time.Time) time.Time {
	if observedAt != nil {
		return observedAt.UTC()
	}
	return svc.now().UTC()
}

func validateRecordExternalPaymentFactInput(input RecordExternalPaymentFactInput) error {
	if strings.TrimSpace(input.Provider) == "" {
		return fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(input.Channel) == "" {
		return fmt.Errorf("channel is required")
	}
	if strings.TrimSpace(input.Capability) == "" {
		return fmt.Errorf("capability is required")
	}
	if !isExternalPaymentFactSource(input.FactSource) {
		return fmt.Errorf("unsupported fact source %q", input.FactSource)
	}
	if strings.TrimSpace(input.ExternalObjectType) == "" {
		return fmt.Errorf("external object type is required")
	}
	if strings.TrimSpace(input.ExternalObjectKey) == "" {
		return fmt.Errorf("external object key is required")
	}
	if strings.TrimSpace(input.UpstreamState) == "" {
		return fmt.Errorf("upstream state is required")
	}
	if !isExternalPaymentKnownTerminalStatus(input.TerminalStatus) {
		return fmt.Errorf("unsupported terminal status %q", input.TerminalStatus)
	}
	if input.FactSource == db.ExternalPaymentFactSourceCommandResponse {
		if input.TerminalStatus != db.ExternalPaymentTerminalStatusUnknown {
			return fmt.Errorf("command response facts must use terminal status %q", db.ExternalPaymentTerminalStatusUnknown)
		}
		if input.Application != nil {
			return fmt.Errorf("command response facts cannot create applications")
		}
	}
	if strings.TrimSpace(input.DedupeKey) == "" {
		return fmt.Errorf("dedupe key is required")
	}
	if input.Application != nil {
		if strings.TrimSpace(input.Application.Consumer) == "" {
			return fmt.Errorf("application consumer is required")
		}
		if strings.TrimSpace(input.Application.BusinessObjectType) == "" {
			return fmt.Errorf("application business object type is required")
		}
		if input.Application.BusinessObjectID == 0 {
			return fmt.Errorf("application business object id is required")
		}
	}
	return nil
}

func isExternalPaymentFactSource(source string) bool {
	switch source {
	case db.ExternalPaymentFactSourceCallback,
		db.ExternalPaymentFactSourceCommandResponse,
		db.ExternalPaymentFactSourceQuery,
		db.ExternalPaymentFactSourceManualReconciliation:
		return true
	default:
		return false
	}
}

func isExternalPaymentKnownTerminalStatus(status string) bool {
	switch status {
	case db.ExternalPaymentTerminalStatusSuccess,
		db.ExternalPaymentTerminalStatusFailed,
		db.ExternalPaymentTerminalStatusClosed,
		db.ExternalPaymentTerminalStatusExpired,
		db.ExternalPaymentTerminalStatusProcessing,
		db.ExternalPaymentTerminalStatusUnknown:
		return true
	default:
		return false
	}
}

func isExternalPaymentTerminalStatus(status string) bool {
	switch status {
	case db.ExternalPaymentTerminalStatusSuccess,
		db.ExternalPaymentTerminalStatusFailed,
		db.ExternalPaymentTerminalStatusClosed,
		db.ExternalPaymentTerminalStatusExpired:
		return true
	default:
		return false
	}
}

func NormalizeDirectPaymentTerminalStatus(upstreamState string) string {
	switch upstreamState {
	case "SUCCESS":
		return db.ExternalPaymentTerminalStatusSuccess
	case "CLOSED":
		return db.ExternalPaymentTerminalStatusClosed
	case "PAYERROR":
		return db.ExternalPaymentTerminalStatusFailed
	case "NOTPAY", "USERPAYING", "ACCEPT":
		return db.ExternalPaymentTerminalStatusProcessing
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

func NormalizeDirectRefundTerminalStatus(upstreamState string) string {
	switch upstreamState {
	case "SUCCESS":
		return db.ExternalPaymentTerminalStatusSuccess
	case "CLOSED":
		return db.ExternalPaymentTerminalStatusClosed
	case "ABNORMAL":
		return db.ExternalPaymentTerminalStatusFailed
	case "PROCESSING":
		return db.ExternalPaymentTerminalStatusProcessing
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

func NormalizeEcommerceRefundTerminalStatus(upstreamState string) string {
	switch upstreamState {
	case "SUCCESS":
		return db.ExternalPaymentTerminalStatusSuccess
	case "CLOSED":
		return db.ExternalPaymentTerminalStatusClosed
	case "ABNORMAL":
		return db.ExternalPaymentTerminalStatusFailed
	case "PROCESSING":
		return db.ExternalPaymentTerminalStatusProcessing
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

func NormalizeProfitSharingTerminalStatus(upstreamState string) string {
	switch strings.ToUpper(strings.TrimSpace(upstreamState)) {
	case "SUCCESS", "FINISHED":
		return db.ExternalPaymentTerminalStatusSuccess
	case "FAILED":
		return db.ExternalPaymentTerminalStatusFailed
	case "CLOSED":
		return db.ExternalPaymentTerminalStatusClosed
	case "PROCESSING", "PENDING":
		return db.ExternalPaymentTerminalStatusProcessing
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

func ResolveProfitSharingQueryFinalResult(queryResp *wechatcontracts.ProfitSharingQueryResponse) (string, string) {
	if queryResp == nil {
		return "PROCESSING", ""
	}

	allSuccess := strings.ToUpper(queryResp.Status) == wechatcontracts.ProfitSharingStatusFinished
	hasFailed := false
	failedReasons := make([]string, 0)

	for _, receiver := range queryResp.Receivers {
		result := strings.ToUpper(strings.TrimSpace(receiver.Result))
		switch result {
		case wechatcontracts.ProfitSharingResultSuccess:
			// pass
		case "FAILED", wechatcontracts.ProfitSharingResultClosed:
			hasFailed = true
			allSuccess = false
			if receiver.FailReason != "" {
				failedReasons = append(failedReasons, receiver.FailReason)
			}
		default:
			allSuccess = false
		}
	}

	switch {
	case hasFailed:
		return "FAILED", strings.Join(failedReasons, ";")
	case allSuccess:
		return "SUCCESS", ""
	default:
		return "PROCESSING", ""
	}
}

func textFromStringPtr(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func int8FromInt64Ptr(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

func timestamptzFromTimePtr(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}
