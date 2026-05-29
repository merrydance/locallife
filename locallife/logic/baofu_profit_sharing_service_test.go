package logic

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestResolveBaofuProfitSharingReceiversUsesCanonicalSharingMerID(t *testing.T) {
	store := &fakeBaofuProfitSharingReceiverStore{
		bindings: map[string]db.BaofuAccountBinding{
			"merchant:101": activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeMerchant, 101, "MER_CONTRACT", "MER_SHARE"),
			"rider:202":    activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeRider, 202, "RIDER_CONTRACT", "RIDER_SHARE"),
			"operator:303": activeBaofuReceiverBinding(
				db.BaofuAccountOwnerTypeOperator,
				303,
				"OP_CONTRACT",
				"OP_SHARE",
			),
			"platform:0": activeBaofuReceiverBinding(db.BaofuAccountOwnerTypePlatform, 0, "PLATFORM_CONTRACT", "PLATFORM_SHARE"),
		},
	}

	receivers, err := ResolveBaofuProfitSharingReceivers(context.Background(), store, BaofuProfitSharingReceiverInput{
		MerchantID: 101,
		RiderID:    202,
		OperatorID: 303,
	})

	require.NoError(t, err)
	require.Equal(t, "MER_SHARE", receivers.MerchantSharingMerID)
	require.Equal(t, "RIDER_SHARE", receivers.RiderSharingMerID)
	require.Equal(t, "OP_SHARE", receivers.OperatorSharingMerID)
	require.Equal(t, "PLATFORM_SHARE", receivers.PlatformSharingMerID)
}

func TestResolveBaofuProfitSharingReceiversRejectsInactiveReceiver(t *testing.T) {
	store := &fakeBaofuProfitSharingReceiverStore{
		bindings: map[string]db.BaofuAccountBinding{
			"merchant:101": {
				OwnerType:    db.BaofuAccountOwnerTypeMerchant,
				OwnerID:      101,
				AccountType:  db.BaofuAccountTypeBusiness,
				OpenState:    db.BaofuAccountOpenStateProcessing,
				SharingMerID: pgtype.Text{String: "MER_SHARE", Valid: true},
			},
			"platform:0": activeBaofuReceiverBinding(db.BaofuAccountOwnerTypePlatform, 0, "PLATFORM_CONTRACT", "PLATFORM_SHARE"),
		},
	}

	_, err := ResolveBaofuProfitSharingReceivers(context.Background(), store, BaofuProfitSharingReceiverInput{MerchantID: 101})

	require.ErrorIs(t, err, ErrBaofuAccountReceiverRequired)
}

func TestResolveBaofuProfitSharingReceiversRejectsContractOnlyReceiver(t *testing.T) {
	store := &fakeBaofuProfitSharingReceiverStore{
		bindings: map[string]db.BaofuAccountBinding{
			"merchant:101": {
				OwnerType:   db.BaofuAccountOwnerTypeMerchant,
				OwnerID:     101,
				AccountType: db.BaofuAccountTypeBusiness,
				OpenState:   db.BaofuAccountOpenStateActive,
				ContractNo:  pgtype.Text{String: "MER_CONTRACT", Valid: true},
			},
			"platform:0": activeBaofuReceiverBinding(db.BaofuAccountOwnerTypePlatform, 0, "PLATFORM_CONTRACT", "PLATFORM_SHARE"),
		},
	}

	_, err := ResolveBaofuProfitSharingReceivers(context.Background(), store, BaofuProfitSharingReceiverInput{MerchantID: 101})

	require.ErrorIs(t, err, ErrBaofuAccountReceiverRequired)
}

type fakeBaofuProfitSharingReceiverStore struct {
	bindings map[string]db.BaofuAccountBinding
}

func (s *fakeBaofuProfitSharingReceiverStore) GetBaofuAccountBindingByOwner(_ context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error) {
	if binding, ok := s.bindings[arg.OwnerType+":"+strconv.FormatInt(arg.OwnerID, 10)]; ok {
		return binding, nil
	}
	return db.BaofuAccountBinding{}, db.ErrRecordNotFound
}

func activeBaofuReceiverBinding(ownerType string, ownerID int64, contractNo string, sharingMerID string) db.BaofuAccountBinding {
	return db.BaofuAccountBinding{
		OwnerType:    ownerType,
		OwnerID:      ownerID,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: contractNo, Valid: contractNo != ""},
		SharingMerID: pgtype.Text{String: sharingMerID, Valid: sharingMerID != ""},
	}
}

func TestBaofuProfitSharingServiceCreatePendingOrderPersistsSnapshotAndFeeLedger(t *testing.T) {
	store := &fakeBaofuProfitSharingOrderStore{
		fakeBaofuProfitSharingReceiverStore: fakeBaofuProfitSharingReceiverStore{bindings: map[string]db.BaofuAccountBinding{
			"merchant:101": activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeMerchant, 101, "MER_CONTRACT", "MER_SHARE"),
			"rider:202":    activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeRider, 202, "RIDER_CONTRACT", "RIDER_SHARE"),
			"operator:303": activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeOperator, 303, "OP_CONTRACT", "OP_SHARE"),
			"platform:0":   activeBaofuReceiverBinding(db.BaofuAccountOwnerTypePlatform, 0, "PLATFORM_CONTRACT", "PLATFORM_SHARE"),
		}},
	}
	service := NewBaofuProfitSharingService(store)

	_, err := service.CreatePendingOrder(context.Background(), BaofuProfitSharingOrderInput{
		PaymentOrderID:  9001,
		MerchantID:      101,
		RiderID:         202,
		OperatorID:      303,
		OrderSource:     "takeout",
		TotalAmountFen:  10000,
		DeliveryFeeFen:  500,
		PlatformRateBps: 200,
		OperatorRateBps: 300,
		OutOrderNo:      "BAOFU_SHARE_9001",
	})
	require.NoError(t, err)
	require.Equal(t, int64(9001), store.lastTx.ProfitSharingOrder.PaymentOrderID)
	require.Equal(t, int64(30), store.lastTx.ProfitSharingOrder.PaymentFee)
	require.Equal(t, int32(30), store.lastTx.ProfitSharingOrder.PaymentFeeRateBps)
	require.Equal(t, BaofuSettlementCalculationVersionV2, store.lastTx.FeeBreakdown.CalculationVersion)
	require.Equal(t, BaofuSettlementModeCommissionShare, store.lastTx.FeeBreakdown.SettlementMode)
	require.Equal(t, int64(30), store.lastTx.FeeBreakdown.ProviderPaymentFee)
	require.Equal(t, int64(57), store.lastTx.FeeBreakdown.MerchantPaymentFee)
	require.Equal(t, int64(3), store.lastTx.FeeBreakdown.RiderPaymentFee)
	require.Equal(t, int64(9500), store.lastTx.FeeBreakdown.CommissionBaseAmount)
	require.Equal(t, int64(220), store.lastTx.FeeBreakdown.PlatformReceiverAmount)
	require.Equal(t, db.ExternalPaymentProviderBaofu, store.lastTx.ProfitSharingOrder.Provider)
	require.Equal(t, db.PaymentChannelBaofuAggregate, store.lastTx.ProfitSharingOrder.Channel)
	require.Equal(t, "MER_SHARE", store.lastTx.ProfitSharingOrder.MerchantSharingMerID.String)
	require.Equal(t, "RIDER_SHARE", store.lastTx.ProfitSharingOrder.RiderSharingMerID.String)
	require.Equal(t, "OP_SHARE", store.lastTx.ProfitSharingOrder.OperatorSharingMerID.String)
	require.Equal(t, "PLATFORM_SHARE", store.lastTx.ProfitSharingOrder.PlatformSharingMerID.String)
	require.JSONEq(t, `{"provider":"baofu","channel":"baofu_aggregate","calculation_version":"baofu_fee_v2","settlement_mode":"commission_share","shareable_amount":9970,"platform_receiver_amount":220,"fees":{"provider_payment_fee":30,"merchant_payment_fee":57,"rider_payment_fee":3,"provider_payment_fee_source":"estimated","provider_payment_fee_timing":"realtime_deducted_before_reserve"},"bases":{"total_amount":10000,"merchant_payment_fee_base":9500,"rider_payment_fee_base":500,"commission_base":9500},"receivers":[{"role":"merchant","sharing_mer_id":"MER_SHARE","amount":8968},{"role":"rider","sharing_mer_id":"RIDER_SHARE","amount":497},{"role":"operator","sharing_mer_id":"OP_SHARE","amount":285},{"role":"platform","sharing_mer_id":"PLATFORM_SHARE","amount":220}]}`, string(store.lastTx.ProfitSharingOrder.SharingDetailSnapshot.([]byte)))
	require.Equal(t, db.BaofuFeeTypePaymentFee, store.lastTx.PaymentFeeLedger.FeeType)
	require.Equal(t, db.BaofuFeePayerTypePlatform, store.lastTx.PaymentFeeLedger.PayerType)
	require.False(t, store.lastTx.PaymentFeeLedger.PayerID.Valid)
	require.Equal(t, "payment_order", store.lastTx.PaymentFeeLedger.BusinessObjectType)
	require.Equal(t, int64(9001), store.lastTx.PaymentFeeLedger.BusinessObjectID)
	require.Equal(t, int64(30), store.lastTx.PaymentFeeLedger.Amount)
	require.Len(t, store.lastTx.OrderPaymentFeeLedgers, 3)
	require.Equal(t, db.OrderPaymentFeeTypeProviderPaymentFee, store.lastTx.OrderPaymentFeeLedgers[0].FeeType)
	require.Equal(t, db.OrderPaymentFeeTypeMerchantPaymentServiceFee, store.lastTx.OrderPaymentFeeLedgers[1].FeeType)
	require.Equal(t, int64(57), store.lastTx.OrderPaymentFeeLedgers[1].Amount)
	require.Equal(t, db.OrderPaymentFeeTypeRiderPaymentServiceFee, store.lastTx.OrderPaymentFeeLedgers[2].FeeType)
	require.Equal(t, int64(3), store.lastTx.OrderPaymentFeeLedgers[2].Amount)
}

func TestBaofuProfitSharingServiceCreatePendingOrderUsesNetAmountAfterSuccessfulRefund(t *testing.T) {
	store := &fakeBaofuProfitSharingOrderStore{
		fakeBaofuProfitSharingReceiverStore: fakeBaofuProfitSharingReceiverStore{bindings: map[string]db.BaofuAccountBinding{
			"merchant:101": activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeMerchant, 101, "MER_CONTRACT", "MER_SHARE"),
			"platform:0":   activeBaofuReceiverBinding(db.BaofuAccountOwnerTypePlatform, 0, "PLATFORM_CONTRACT", "PLATFORM_SHARE"),
		}},
	}
	service := NewBaofuProfitSharingService(store)

	_, err := service.CreatePendingOrder(context.Background(), BaofuProfitSharingOrderInput{
		PaymentOrderID:  9002,
		MerchantID:      101,
		OrderSource:     "reservation",
		TotalAmountFen:  10000,
		RefundedFen:     2500,
		PlatformRateBps: 200,
		OperatorRateBps: 300,
		OutOrderNo:      "BAOFU_SHARE_9002",
	})
	require.NoError(t, err)
	require.Equal(t, int64(7500), store.lastTx.ProfitSharingOrder.TotalAmount)
	require.Equal(t, int64(7500), store.lastTx.ProfitSharingOrder.DistributableAmount)
	require.Equal(t, int64(375), store.lastTx.ProfitSharingOrder.PlatformCommission)
	require.Equal(t, int64(0), store.lastTx.ProfitSharingOrder.OperatorCommission)
	require.Equal(t, int64(7080), store.lastTx.ProfitSharingOrder.MerchantAmount)
	require.Equal(t, int64(23), store.lastTx.ProfitSharingOrder.PaymentFee)
	require.Equal(t, int64(45), store.lastTx.FeeBreakdown.MerchantPaymentFee)
	require.Equal(t, int64(397), store.lastTx.FeeBreakdown.PlatformReceiverAmount)
	require.JSONEq(t, `{"provider":"baofu","channel":"baofu_aggregate","calculation_version":"baofu_fee_v2","settlement_mode":"commission_share","shareable_amount":7477,"platform_receiver_amount":397,"fees":{"provider_payment_fee":23,"merchant_payment_fee":45,"rider_payment_fee":0,"provider_payment_fee_source":"estimated","provider_payment_fee_timing":"realtime_deducted_before_reserve"},"bases":{"total_amount":7500,"merchant_payment_fee_base":7500,"rider_payment_fee_base":0,"commission_base":7500},"receivers":[{"role":"merchant","sharing_mer_id":"MER_SHARE","amount":7080},{"role":"platform","sharing_mer_id":"PLATFORM_SHARE","amount":397}]}`, string(store.lastTx.ProfitSharingOrder.SharingDetailSnapshot.([]byte)))
}

type fakeBaofuProfitSharingOrderStore struct {
	fakeBaofuProfitSharingReceiverStore
	lastTx db.CreateBaofuProfitSharingOrderTxParams
}

func (s *fakeBaofuProfitSharingOrderStore) EnsureBaofuProfitSharingBillTx(_ context.Context, arg db.CreateBaofuProfitSharingOrderTxParams) (db.CreateBaofuProfitSharingOrderTxResult, error) {
	s.lastTx = arg
	return db.CreateBaofuProfitSharingOrderTxResult{ProfitSharingOrder: db.ProfitSharingOrder{PaymentOrderID: arg.ProfitSharingOrder.PaymentOrderID}, PaymentFeeLedger: db.BaofuFeeLedger{Amount: arg.PaymentFeeLedger.Amount}}, nil
}

func TestBaofuProfitSharingServiceRecordShareFactCreatesApplicationForTerminalShare(t *testing.T) {
	store := &fakeBaofuProfitSharingFactStore{}
	service := NewBaofuProfitSharingService(store)
	now := time.Date(2026, 5, 3, 12, 30, 0, 0, time.UTC)

	result, err := service.RecordShareFact(context.Background(), RecordBaofuShareFactInput{
		ProfitSharingOrder: db.ProfitSharingOrder{
			ID:             3001,
			OutOrderNo:     "BFSHARE202605030001",
			PaymentOrderID: 9001,
		},
		Fact: aggregatecontracts.ShareFact{
			OutTradeNo:       "BFSHARE202605030001",
			TradeNo:          "BFSHAREUP202605030001",
			TransactionState: aggregatecontracts.ShareStateSuccess,
			SuccessAmountFen: 10000,
			Raw:              []byte(`{"txnState":"SUCCESS","succAmt":10000}`),
		},
		FactSource:      db.ExternalPaymentFactSourceCallback,
		SourceEventID:   "BFSN202605030001",
		SourceEventType: "SHARING",
		OccurredAt:      now,
		ObservedAt:      now,
	})

	require.NoError(t, err)
	require.Equal(t, int64(801), result.Fact.ID)
	require.NotNil(t, result.Application)
	require.Equal(t, db.ExternalPaymentProviderBaofu, store.lastFact.Provider)
	require.Equal(t, db.PaymentChannelBaofuAggregate, store.lastFact.Channel)
	require.Equal(t, db.ExternalPaymentCapabilityBaofuProfitSharing, store.lastFact.Capability)
	require.Equal(t, db.ExternalPaymentFactSourceCallback, store.lastFact.FactSource)
	require.Equal(t, db.ExternalPaymentObjectProfitSharing, store.lastFact.ExternalObjectType)
	require.Equal(t, "BFSHARE202605030001", store.lastFact.ExternalObjectKey)
	require.Equal(t, "BFSHAREUP202605030001", store.lastFact.ExternalSecondaryKey.String)
	require.Equal(t, db.ExternalPaymentBusinessOwnerProfitSharing, store.lastFact.BusinessOwner.String)
	require.Equal(t, paymentFactBusinessObjectProfitSharingOrder, store.lastFact.BusinessObjectType.String)
	require.Equal(t, int64(3001), store.lastFact.BusinessObjectID.Int64)
	require.Equal(t, aggregatecontracts.ShareStateSuccess, store.lastFact.UpstreamState)
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, store.lastFact.TerminalStatus)
	require.True(t, store.lastFact.IsTerminal)
	require.Equal(t, int64(10000), store.lastFact.Amount.Int64)
	require.Equal(t, "baofu:callback:profit_sharing:BFSHARE202605030001:BFSN202605030001", store.lastFact.DedupeKey)
	require.Equal(t, int64(801), store.lastApplication.FactID)
	require.Equal(t, paymentFactConsumerProfitSharingDomain, store.lastApplication.Consumer)
	require.Equal(t, paymentFactBusinessObjectProfitSharingOrder, store.lastApplication.BusinessObjectType)
	require.Equal(t, int64(3001), store.lastApplication.BusinessObjectID)
}

func TestBaofuProfitSharingServiceRecordShareFactRejectsSuccessWithoutAmount(t *testing.T) {
	store := &fakeBaofuProfitSharingFactStore{}
	service := NewBaofuProfitSharingService(store)
	now := time.Date(2026, 5, 5, 12, 30, 0, 0, time.UTC)

	_, err := service.RecordShareFact(context.Background(), RecordBaofuShareFactInput{
		ProfitSharingOrder: db.ProfitSharingOrder{
			ID:                     3002,
			OutOrderNo:             "BFSHARE202605050001",
			PaymentOrderID:         9002,
			MerchantAmount:         8970,
			RiderAmount:            500,
			OperatorCommission:     280,
			PlatformCommission:     190,
			PlatformReceiverAmount: 220,
			CalculationVersion:     BaofuSettlementCalculationVersionV2,
		},
		Fact: aggregatecontracts.ShareFact{
			OutTradeNo:       "BFSHARE202605050001",
			TradeNo:          "BFSHAREUP202605050001",
			TransactionState: aggregatecontracts.ShareStateSuccess,
			Raw:              []byte(`{"txnState":"SUCCESS"}`),
		},
		FactSource:      db.ExternalPaymentFactSourceCallback,
		SourceEventID:   "BFSN202605050001",
		SourceEventType: "SHARING",
		OccurredAt:      now,
		ObservedAt:      now,
	})

	require.ErrorIs(t, err, ErrBaofuProfitSharingFactInvalidInput)
	require.Equal(t, 0, store.createFactCalls)
	require.Equal(t, 0, store.createApplicationCalls)
}

func TestBaofuProfitSharingServiceRecordShareFactKeepsProcessingAmountEmpty(t *testing.T) {
	store := &fakeBaofuProfitSharingFactStore{}
	service := NewBaofuProfitSharingService(store)
	now := time.Date(2026, 5, 5, 12, 35, 0, 0, time.UTC)

	result, err := service.RecordShareFact(context.Background(), RecordBaofuShareFactInput{
		ProfitSharingOrder: db.ProfitSharingOrder{
			ID:                     3003,
			OutOrderNo:             "BFSHARE202605050003",
			PaymentOrderID:         9003,
			MerchantAmount:         8970,
			RiderAmount:            500,
			OperatorCommission:     280,
			PlatformReceiverAmount: 220,
			CalculationVersion:     BaofuSettlementCalculationVersionV2,
		},
		Fact: aggregatecontracts.ShareFact{
			OutTradeNo:       "BFSHARE202605050003",
			TradeNo:          "BFSHAREUP202605050003",
			TransactionState: aggregatecontracts.ShareStateProcessing,
			Raw:              []byte(`{"txnState":"PROCESSING"}`),
		},
		FactSource:      db.ExternalPaymentFactSourceCallback,
		SourceEventID:   "BFSN202605050003",
		SourceEventType: "SHARING",
		OccurredAt:      now,
		ObservedAt:      now,
	})

	require.NoError(t, err)
	require.Equal(t, int64(801), result.Fact.ID)
	require.Nil(t, result.Application)
	require.False(t, store.lastFact.Amount.Valid)
	require.Equal(t, 1, store.createFactCalls)
	require.Equal(t, 0, store.createApplicationCalls)
}

type fakeBaofuProfitSharingFactStore struct {
	fakeBaofuProfitSharingReceiverStore
	lastFact               db.CreateExternalPaymentFactParams
	lastApplication        db.CreateExternalPaymentFactApplicationParams
	createFactCalls        int
	createApplicationCalls int
}

func (s *fakeBaofuProfitSharingFactStore) EnsureBaofuProfitSharingBillTx(context.Context, db.CreateBaofuProfitSharingOrderTxParams) (db.CreateBaofuProfitSharingOrderTxResult, error) {
	return db.CreateBaofuProfitSharingOrderTxResult{}, nil
}

func (s *fakeBaofuProfitSharingFactStore) CreateExternalPaymentFact(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
	s.createFactCalls++
	s.lastFact = arg
	return db.ExternalPaymentFact{
		ID:                 801,
		Provider:           arg.Provider,
		Channel:            arg.Channel,
		Capability:         arg.Capability,
		ExternalObjectType: arg.ExternalObjectType,
		ExternalObjectKey:  arg.ExternalObjectKey,
		TerminalStatus:     arg.TerminalStatus,
		IsTerminal:         arg.IsTerminal,
	}, nil
}

func (s *fakeBaofuProfitSharingFactStore) CreateExternalPaymentFactApplication(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
	s.createApplicationCalls++
	s.lastApplication = arg
	return db.ExternalPaymentFactApplication{
		ID:                 901,
		FactID:             arg.FactID,
		Consumer:           arg.Consumer,
		BusinessObjectType: arg.BusinessObjectType,
		BusinessObjectID:   arg.BusinessObjectID,
		Status:             arg.Status,
	}, nil
}
