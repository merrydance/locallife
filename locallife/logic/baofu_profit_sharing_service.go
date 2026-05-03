package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	BaofuPaymentFeeRateBps = 30
)

var (
	ErrBaofuProfitSharingInvalidAmount          = errors.New("baofu profit sharing amount input is invalid")
	ErrBaofuProfitSharingMerchantAmountNegative = errors.New("baofu profit sharing merchant amount is negative")
	ErrBaofuProfitSharingServiceNotConfigured   = errors.New("baofu profit sharing service is not configured")
	ErrBaofuProfitSharingFactInvalidInput       = errors.New("baofu profit sharing fact input is invalid")
)

type baofuProfitSharingOrderStore interface {
	baofuProfitSharingReceiverStore
	CreateBaofuProfitSharingOrderTx(ctx context.Context, arg db.CreateBaofuProfitSharingOrderTxParams) (db.CreateBaofuProfitSharingOrderTxResult, error)
}

type BaofuProfitSharingService struct {
	store baofuProfitSharingOrderStore
}

type BaofuProfitSharingOrderInput struct {
	PaymentOrderID  int64
	MerchantID      int64
	RiderID         int64
	OperatorID      int64
	PlatformOwnerID int64
	OrderSource     string
	TotalAmountFen  int64
	DeliveryFeeFen  int64
	PlatformRateBps int32
	OperatorRateBps int32
	OutOrderNo      string
}

type RecordBaofuShareFactInput struct {
	ProfitSharingOrder db.ProfitSharingOrder
	Fact               aggregatecontracts.ShareFact
	FactSource         string
	SourceEventID      string
	SourceEventType    string
	OccurredAt         time.Time
	ObservedAt         time.Time
}

type baofuSharingDetailSnapshot struct {
	Provider          string                          `json:"provider"`
	Channel           string                          `json:"channel"`
	PaymentFee        int64                           `json:"payment_fee"`
	PaymentFeeRateBps int32                           `json:"payment_fee_rate_bps"`
	Receivers         []baofuSharingDetailSnapshotRow `json:"receivers"`
}

type baofuSharingDetailSnapshotRow struct {
	Role         string `json:"role"`
	SharingMerID string `json:"sharing_mer_id"`
	Amount       int64  `json:"amount"`
}

func NewBaofuProfitSharingService(store baofuProfitSharingOrderStore) *BaofuProfitSharingService {
	return &BaofuProfitSharingService{store: store}
}

func (s *BaofuProfitSharingService) CreatePendingOrder(ctx context.Context, input BaofuProfitSharingOrderInput) (db.CreateBaofuProfitSharingOrderTxResult, error) {
	if s == nil || s.store == nil || input.PaymentOrderID <= 0 || input.MerchantID <= 0 || strings.TrimSpace(input.OutOrderNo) == "" {
		return db.CreateBaofuProfitSharingOrderTxResult{}, ErrBaofuProfitSharingInvalidAmount
	}

	receivers, err := ResolveBaofuProfitSharingReceivers(ctx, s.store, BaofuProfitSharingReceiverInput{
		MerchantID:      input.MerchantID,
		RiderID:         input.RiderID,
		OperatorID:      input.OperatorID,
		PlatformOwnerID: input.PlatformOwnerID,
	})
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, err
	}
	amounts, err := CalculateBaofuProfitSharingAmounts(BaofuProfitSharingAmountInput{
		TotalAmountFen:      input.TotalAmountFen,
		DeliveryFeeFen:      input.DeliveryFeeFen,
		PlatformRateBps:     input.PlatformRateBps,
		OperatorRateBps:     input.OperatorRateBps,
		HasRiderReceiver:    input.RiderID > 0,
		HasOperatorReceiver: input.OperatorID > 0,
		RedirectMissingOperatorCommissionToPlatform: true,
	})
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, err
	}
	snapshot, err := buildBaofuSharingDetailSnapshot(amounts, receivers)
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, err
	}

	return s.store.CreateBaofuProfitSharingOrderTx(ctx, db.CreateBaofuProfitSharingOrderTxParams{
		ProfitSharingOrder: db.CreateProfitSharingOrderParams{
			PaymentOrderID:        input.PaymentOrderID,
			MerchantID:            input.MerchantID,
			OperatorID:            pgtype.Int8{Int64: input.OperatorID, Valid: input.OperatorID > 0},
			OrderSource:           strings.TrimSpace(input.OrderSource),
			TotalAmount:           amounts.TotalAmountFen,
			DeliveryFee:           amounts.DeliveryFeeFen,
			RiderID:               pgtype.Int8{Int64: input.RiderID, Valid: input.RiderID > 0},
			RiderAmount:           amounts.RiderAmountFen,
			DistributableAmount:   amounts.DistributableAmountFen,
			PlatformRate:          amounts.PlatformRateBps,
			OperatorRate:          amounts.OperatorRateBps,
			PlatformCommission:    amounts.PlatformCommissionFen,
			OperatorCommission:    amounts.OperatorCommissionFen,
			MerchantAmount:        amounts.MerchantAmountFen,
			OutOrderNo:            strings.TrimSpace(input.OutOrderNo),
			Status:                db.ProfitSharingOrderStatusPending,
			PaymentFee:            amounts.PaymentFeeFen,
			PaymentFeeRateBps:     amounts.PaymentFeeRateBps,
			Provider:              db.ExternalPaymentProviderBaofu,
			Channel:               db.PaymentChannelBaofuAggregate,
			MerchantSharingMerID:  pgtype.Text{String: receivers.MerchantSharingMerID, Valid: receivers.MerchantSharingMerID != ""},
			RiderSharingMerID:     pgtype.Text{String: receivers.RiderSharingMerID, Valid: receivers.RiderSharingMerID != ""},
			OperatorSharingMerID:  pgtype.Text{String: receivers.OperatorSharingMerID, Valid: receivers.OperatorSharingMerID != ""},
			PlatformSharingMerID:  pgtype.Text{String: receivers.PlatformSharingMerID, Valid: receivers.PlatformSharingMerID != ""},
			SharingDetailSnapshot: snapshot,
		},
		PaymentFeeLedger: db.CreateBaofuFeeLedgerParams{
			FeeType:            db.BaofuFeeTypePaymentFee,
			PayerType:          db.BaofuFeePayerTypeMerchant,
			PayerID:            pgtype.Int8{Int64: input.MerchantID, Valid: true},
			BusinessObjectType: "payment_order",
			BusinessObjectID:   input.PaymentOrderID,
			Amount:             amounts.PaymentFeeFen,
			FeeRateBps:         pgtype.Int4{Int32: amounts.PaymentFeeRateBps, Valid: true},
			Status:             "recorded",
		},
	})
}

func (s *BaofuProfitSharingService) RecordShareFact(ctx context.Context, input RecordBaofuShareFactInput) (RecordExternalPaymentFactResult, error) {
	var result RecordExternalPaymentFactResult
	if s == nil || s.store == nil {
		return result, ErrBaofuProfitSharingServiceNotConfigured
	}
	factStore, ok := s.store.(baofuPaymentFactStore)
	if !ok {
		return result, ErrBaofuProfitSharingServiceNotConfigured
	}
	if err := validateRecordBaofuShareFactInput(input); err != nil {
		return result, err
	}

	order := input.ProfitSharingOrder
	shareFact := input.Fact
	outTradeNo := strings.TrimSpace(shareFact.OutTradeNo)
	if outTradeNo == "" {
		outTradeNo = strings.TrimSpace(order.OutOrderNo)
	}
	upstreamState := strings.TrimSpace(shareFact.TransactionState)
	terminalStatus := aggregatecontracts.NormalizeShareTerminalStatus(upstreamState)
	observedAt := input.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	occurredAtParam := pgtype.Timestamptz{}
	if !input.OccurredAt.IsZero() {
		occurredAtParam = pgtype.Timestamptz{Time: input.OccurredAt.UTC(), Valid: true}
	}
	rawResource := shareFact.Raw
	if len(rawResource) == 0 {
		rawResource = []byte(`{}`)
	}
	amount := shareFact.SuccessAmountFen
	if amount <= 0 {
		amount = order.MerchantAmount + order.RiderAmount + order.OperatorCommission + order.PlatformCommission
	}

	fact, err := factStore.CreateExternalPaymentFact(ctx, db.CreateExternalPaymentFactParams{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuProfitSharing,
		FactSource:           strings.TrimSpace(input.FactSource),
		SourceEventID:        pgtype.Text{String: strings.TrimSpace(input.SourceEventID), Valid: strings.TrimSpace(input.SourceEventID) != ""},
		SourceEventType:      pgtype.Text{String: strings.TrimSpace(input.SourceEventType), Valid: strings.TrimSpace(input.SourceEventType) != ""},
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:    outTradeNo,
		ExternalSecondaryKey: pgtype.Text{String: strings.TrimSpace(shareFact.TradeNo), Valid: strings.TrimSpace(shareFact.TradeNo) != ""},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerProfitSharing, Valid: true},
		BusinessObjectType:   pgtype.Text{String: paymentFactBusinessObjectProfitSharingOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: order.ID, Valid: true},
		UpstreamState:        upstreamState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           isExternalPaymentTerminalStatus(terminalStatus),
		Amount:               pgtype.Int8{Int64: amount, Valid: amount > 0},
		Currency:             "CNY",
		OccurredAt:           occurredAtParam,
		ObservedAt:           observedAt.UTC(),
		RawResource:          rawResource,
		DedupeKey:            baofuShareFactDedupeKey(input, outTradeNo, upstreamState),
		ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
	})
	if err != nil {
		return result, err
	}
	result.Fact = fact
	if !fact.IsTerminal {
		return result, nil
	}

	application, err := factStore.CreateExternalPaymentFactApplication(ctx, db.CreateExternalPaymentFactApplicationParams{
		FactID:             fact.ID,
		Consumer:           paymentFactConsumerProfitSharingDomain,
		BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder,
		BusinessObjectID:   order.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	})
	if err != nil {
		return result, err
	}
	result.Application = &application
	return result, nil
}

func validateRecordBaofuShareFactInput(input RecordBaofuShareFactInput) error {
	if input.ProfitSharingOrder.ID == 0 || strings.TrimSpace(input.ProfitSharingOrder.OutOrderNo) == "" {
		return ErrBaofuProfitSharingFactInvalidInput
	}
	if !isExternalPaymentFactSource(input.FactSource) {
		return fmt.Errorf("unsupported fact source %q", input.FactSource)
	}
	outTradeNo := strings.TrimSpace(input.Fact.OutTradeNo)
	if outTradeNo != "" && outTradeNo != strings.TrimSpace(input.ProfitSharingOrder.OutOrderNo) {
		return ErrBaofuProfitSharingFactInvalidInput
	}
	if strings.TrimSpace(input.Fact.TransactionState) == "" {
		return ErrBaofuProfitSharingFactInvalidInput
	}
	return nil
}

func baofuShareFactDedupeKey(input RecordBaofuShareFactInput, outTradeNo string, upstreamState string) string {
	source := strings.TrimSpace(input.FactSource)
	if source == db.ExternalPaymentFactSourceCallback && strings.TrimSpace(input.SourceEventID) != "" {
		return fmt.Sprintf("baofu:callback:profit_sharing:%s:%s", outTradeNo, strings.TrimSpace(input.SourceEventID))
	}
	secondary := strings.TrimSpace(input.Fact.TradeNo)
	if secondary == "" {
		secondary = strings.TrimSpace(upstreamState)
	}
	return fmt.Sprintf("baofu:%s:profit_sharing:%s:%s", source, outTradeNo, secondary)
}

func buildBaofuSharingDetailSnapshot(amounts BaofuProfitSharingAmountResult, receivers BaofuProfitSharingReceiverResult) ([]byte, error) {
	snapshot := baofuSharingDetailSnapshot{
		Provider:          db.ExternalPaymentProviderBaofu,
		Channel:           db.PaymentChannelBaofuAggregate,
		PaymentFee:        amounts.PaymentFeeFen,
		PaymentFeeRateBps: amounts.PaymentFeeRateBps,
		Receivers: []baofuSharingDetailSnapshotRow{
			{Role: "merchant", SharingMerID: receivers.MerchantSharingMerID, Amount: amounts.MerchantAmountFen},
		},
	}
	if amounts.RiderAmountFen > 0 {
		snapshot.Receivers = append(snapshot.Receivers, baofuSharingDetailSnapshotRow{Role: "rider", SharingMerID: receivers.RiderSharingMerID, Amount: amounts.RiderAmountFen})
	}
	if amounts.OperatorCommissionFen > 0 {
		snapshot.Receivers = append(snapshot.Receivers, baofuSharingDetailSnapshotRow{Role: "operator", SharingMerID: receivers.OperatorSharingMerID, Amount: amounts.OperatorCommissionFen})
	}
	if amounts.PlatformCommissionFen > 0 {
		snapshot.Receivers = append(snapshot.Receivers, baofuSharingDetailSnapshotRow{Role: "platform", SharingMerID: receivers.PlatformSharingMerID, Amount: amounts.PlatformCommissionFen})
	}
	return json.Marshal(snapshot)
}

type BaofuProfitSharingAmountInput struct {
	TotalAmountFen                              int64
	DeliveryFeeFen                              int64
	PlatformRateBps                             int32
	OperatorRateBps                             int32
	HasRiderReceiver                            bool
	HasOperatorReceiver                         bool
	RedirectMissingOperatorCommissionToPlatform bool
}

type BaofuProfitSharingAmountResult struct {
	TotalAmountFen                         int64
	DeliveryFeeFen                         int64
	RiderAmountFen                         int64
	DistributableAmountFen                 int64
	PlatformRateBps                        int32
	OperatorRateBps                        int32
	PlatformCommissionFen                  int64
	OperatorCommissionFen                  int64
	PaymentFeeFen                          int64
	PaymentFeeRateBps                      int32
	MerchantAmountFen                      int64
	OperatorCommissionRedirectedToPlatform bool
}

type baofuProfitSharingReceiverStore interface {
	GetBaofuAccountBindingByOwner(ctx context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error)
}

type BaofuProfitSharingReceiverInput struct {
	MerchantID      int64
	RiderID         int64
	OperatorID      int64
	PlatformOwnerID int64
}

type BaofuProfitSharingReceiverResult struct {
	MerchantSharingMerID string
	RiderSharingMerID    string
	OperatorSharingMerID string
	PlatformSharingMerID string
}

func CalculateBaofuPaymentFeeFen(totalAmountFen int64) int64 {
	if totalAmountFen <= 0 {
		return 0
	}
	return (totalAmountFen*BaofuPaymentFeeRateBps + 9999) / 10000
}

func CalculateBaofuProfitSharingAmounts(input BaofuProfitSharingAmountInput) (BaofuProfitSharingAmountResult, error) {
	if input.TotalAmountFen < 0 || input.DeliveryFeeFen < 0 || input.PlatformRateBps < 0 || input.OperatorRateBps < 0 {
		return BaofuProfitSharingAmountResult{}, ErrBaofuProfitSharingInvalidAmount
	}

	result := BaofuProfitSharingAmountResult{
		TotalAmountFen:    input.TotalAmountFen,
		DeliveryFeeFen:    input.DeliveryFeeFen,
		PlatformRateBps:   input.PlatformRateBps,
		OperatorRateBps:   input.OperatorRateBps,
		PaymentFeeFen:     CalculateBaofuPaymentFeeFen(input.TotalAmountFen),
		PaymentFeeRateBps: BaofuPaymentFeeRateBps,
	}
	if input.HasRiderReceiver {
		result.RiderAmountFen = minInt64(input.DeliveryFeeFen, input.TotalAmountFen)
	}
	result.DistributableAmountFen = input.TotalAmountFen - result.RiderAmountFen
	if result.DistributableAmountFen < 0 {
		result.DistributableAmountFen = 0
	}

	result.PlatformCommissionFen = input.TotalAmountFen * int64(input.PlatformRateBps) / 10000
	result.OperatorCommissionFen = input.TotalAmountFen * int64(input.OperatorRateBps) / 10000
	if !input.HasOperatorReceiver && result.OperatorCommissionFen > 0 && input.RedirectMissingOperatorCommissionToPlatform {
		result.PlatformCommissionFen += result.OperatorCommissionFen
		result.OperatorCommissionFen = 0
		result.OperatorCommissionRedirectedToPlatform = true
	}

	result.MerchantAmountFen = result.DistributableAmountFen - result.PlatformCommissionFen - result.OperatorCommissionFen - result.PaymentFeeFen
	if result.MerchantAmountFen < 0 {
		return BaofuProfitSharingAmountResult{}, ErrBaofuProfitSharingMerchantAmountNegative
	}
	return result, nil
}

func ResolveBaofuProfitSharingReceivers(ctx context.Context, store baofuProfitSharingReceiverStore, input BaofuProfitSharingReceiverInput) (BaofuProfitSharingReceiverResult, error) {
	if store == nil || input.MerchantID <= 0 {
		return BaofuProfitSharingReceiverResult{}, ErrBaofuAccountReceiverRequired
	}

	platformOwnerID := input.PlatformOwnerID
	result := BaofuProfitSharingReceiverResult{}
	merchantReceiver, err := resolveBaofuProfitSharingReceiver(ctx, store, db.BaofuAccountOwnerTypeMerchant, input.MerchantID)
	if err != nil {
		return result, fmt.Errorf("merchant baofu receiver: %w", err)
	}
	result.MerchantSharingMerID = merchantReceiver

	if input.RiderID > 0 {
		riderReceiver, err := resolveBaofuProfitSharingReceiver(ctx, store, db.BaofuAccountOwnerTypeRider, input.RiderID)
		if err != nil {
			return result, fmt.Errorf("rider baofu receiver: %w", err)
		}
		result.RiderSharingMerID = riderReceiver
	}

	if input.OperatorID > 0 {
		operatorReceiver, err := resolveBaofuProfitSharingReceiver(ctx, store, db.BaofuAccountOwnerTypeOperator, input.OperatorID)
		if err != nil {
			return result, fmt.Errorf("operator baofu receiver: %w", err)
		}
		result.OperatorSharingMerID = operatorReceiver
	}

	platformReceiver, err := resolveBaofuProfitSharingReceiver(ctx, store, db.BaofuAccountOwnerTypePlatform, platformOwnerID)
	if err != nil {
		return result, fmt.Errorf("platform baofu receiver: %w", err)
	}
	result.PlatformSharingMerID = platformReceiver
	return result, nil
}

func resolveBaofuProfitSharingReceiver(ctx context.Context, store baofuProfitSharingReceiverStore, ownerType string, ownerID int64) (string, error) {
	binding, err := store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: ownerType,
		OwnerID:   ownerID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return "", ErrBaofuAccountReceiverRequired
		}
		return "", fmt.Errorf("get baofu account binding: %w", err)
	}
	if binding.OpenState != db.BaofuAccountOpenStateActive {
		return "", ErrBaofuAccountReceiverRequired
	}
	receiverID := baofuProfitSharingReceiverID(binding)
	if receiverID == "" {
		return "", ErrBaofuAccountReceiverRequired
	}
	return receiverID, nil
}

func baofuProfitSharingReceiverID(binding db.BaofuAccountBinding) string {
	if binding.SharingMerID.Valid {
		return strings.TrimSpace(binding.SharingMerID.String)
	}
	return ""
}
