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

func TestCalculateBaofuPaymentFeeFenRoundsUpMerchantBorneFee(t *testing.T) {
	require.Equal(t, int64(30), CalculateBaofuPaymentFeeFen(10000))
	require.Equal(t, int64(1), CalculateBaofuPaymentFeeFen(1))
	require.Equal(t, int64(1), CalculateBaofuPaymentFeeFen(333))
	require.Equal(t, int64(0), CalculateBaofuPaymentFeeFen(0))
}

func TestCalculateBaofuProfitSharingAmountsMerchantBearsPaymentFee(t *testing.T) {
	result, err := CalculateBaofuProfitSharingAmounts(BaofuProfitSharingAmountInput{
		TotalAmountFen:      10000,
		DeliveryFeeFen:      500,
		PlatformRateBps:     200,
		OperatorRateBps:     300,
		HasRiderReceiver:    true,
		HasOperatorReceiver: true,
		RedirectMissingOperatorCommissionToPlatform: true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(500), result.RiderAmountFen)
	require.Equal(t, int64(9500), result.DistributableAmountFen)
	require.Equal(t, int64(200), result.PlatformCommissionFen)
	require.Equal(t, int64(300), result.OperatorCommissionFen)
	require.Equal(t, int64(30), result.PaymentFeeFen)
	require.Equal(t, int64(8970), result.MerchantAmountFen)
	require.False(t, result.OperatorCommissionRedirectedToPlatform)
}

func TestCalculateBaofuProfitSharingAmountsRedirectsMissingOperatorToPlatform(t *testing.T) {
	result, err := CalculateBaofuProfitSharingAmounts(BaofuProfitSharingAmountInput{
		TotalAmountFen:      10000,
		DeliveryFeeFen:      500,
		PlatformRateBps:     200,
		OperatorRateBps:     300,
		HasRiderReceiver:    true,
		HasOperatorReceiver: false,
		RedirectMissingOperatorCommissionToPlatform: true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(500), result.RiderAmountFen)
	require.Equal(t, int64(500), result.PlatformCommissionFen)
	require.Equal(t, int64(0), result.OperatorCommissionFen)
	require.Equal(t, int64(30), result.PaymentFeeFen)
	require.Equal(t, int64(8970), result.MerchantAmountFen)
	require.True(t, result.OperatorCommissionRedirectedToPlatform)
}

func TestCalculateBaofuProfitSharingAmountsRejectsNegativeMerchantAmount(t *testing.T) {
	_, err := CalculateBaofuProfitSharingAmounts(BaofuProfitSharingAmountInput{
		TotalAmountFen:      100,
		DeliveryFeeFen:      0,
		PlatformRateBps:     9900,
		OperatorRateBps:     100,
		HasOperatorReceiver: true,
		RedirectMissingOperatorCommissionToPlatform: true,
	})

	require.ErrorIs(t, err, ErrBaofuProfitSharingMerchantAmountNegative)
}

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
	require.Equal(t, db.ExternalPaymentProviderBaofu, store.lastTx.ProfitSharingOrder.Provider)
	require.Equal(t, db.PaymentChannelBaofuAggregate, store.lastTx.ProfitSharingOrder.Channel)
	require.Equal(t, "MER_SHARE", store.lastTx.ProfitSharingOrder.MerchantSharingMerID.String)
	require.Equal(t, "RIDER_SHARE", store.lastTx.ProfitSharingOrder.RiderSharingMerID.String)
	require.Equal(t, "OP_SHARE", store.lastTx.ProfitSharingOrder.OperatorSharingMerID.String)
	require.Equal(t, "PLATFORM_SHARE", store.lastTx.ProfitSharingOrder.PlatformSharingMerID.String)
	require.JSONEq(t, `{"provider":"baofu","channel":"baofu_aggregate","payment_fee":30,"payment_fee_rate_bps":30,"receivers":[{"role":"merchant","sharing_mer_id":"MER_SHARE","amount":8970},{"role":"rider","sharing_mer_id":"RIDER_SHARE","amount":500},{"role":"operator","sharing_mer_id":"OP_SHARE","amount":300},{"role":"platform","sharing_mer_id":"PLATFORM_SHARE","amount":200}]}`, string(store.lastTx.ProfitSharingOrder.SharingDetailSnapshot.([]byte)))
	require.Equal(t, db.BaofuFeeTypePaymentFee, store.lastTx.PaymentFeeLedger.FeeType)
	require.Equal(t, db.BaofuFeePayerTypeMerchant, store.lastTx.PaymentFeeLedger.PayerType)
	require.Equal(t, int64(101), store.lastTx.PaymentFeeLedger.PayerID.Int64)
	require.Equal(t, "payment_order", store.lastTx.PaymentFeeLedger.BusinessObjectType)
	require.Equal(t, int64(9001), store.lastTx.PaymentFeeLedger.BusinessObjectID)
	require.Equal(t, int64(30), store.lastTx.PaymentFeeLedger.Amount)
}

type fakeBaofuProfitSharingOrderStore struct {
	fakeBaofuProfitSharingReceiverStore
	lastTx db.CreateBaofuProfitSharingOrderTxParams
}

func (s *fakeBaofuProfitSharingOrderStore) CreateBaofuProfitSharingOrderTx(_ context.Context, arg db.CreateBaofuProfitSharingOrderTxParams) (db.CreateBaofuProfitSharingOrderTxResult, error) {
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
		SourceEventType: "SHARE.SUCCESS",
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

type fakeBaofuProfitSharingFactStore struct {
	fakeBaofuProfitSharingReceiverStore
	lastFact        db.CreateExternalPaymentFactParams
	lastApplication db.CreateExternalPaymentFactApplicationParams
}

func (s *fakeBaofuProfitSharingFactStore) CreateBaofuProfitSharingOrderTx(context.Context, db.CreateBaofuProfitSharingOrderTxParams) (db.CreateBaofuProfitSharingOrderTxResult, error) {
	return db.CreateBaofuProfitSharingOrderTxResult{}, nil
}

func (s *fakeBaofuProfitSharingFactStore) CreateExternalPaymentFact(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
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
