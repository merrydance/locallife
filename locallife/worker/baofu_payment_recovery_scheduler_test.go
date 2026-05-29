package worker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBaofuPaymentRecoverySchedulerRunOnceCreatesPendingShareAndEnqueuesCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}

	paymentOrder := db.PaymentOrder{
		ID:                    301,
		OrderID:               pgtype.Int8{Int64: 401, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Amount:                10000,
		OutTradeNo:            "BFPAY_301",
		TransactionID:         pgtype.Text{String: "BFUP_301", Valid: true},
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	order := db.Order{
		ID:          401,
		MerchantID:  501,
		OrderType:   "dine_in",
		TotalAmount: 10000,
		Status:      db.OrderStatusCompleted,
		CompletedAt: pgtype.Timestamptz{
			Time:  time.Now().Add(-time.Minute),
			Valid: true,
		},
	}
	merchant := db.Merchant{ID: 501, RegionID: 601}
	operator := db.Operator{ID: 701, RegionID: 601}
	profitSharingConfig := db.ProfitSharingConfig{PlatformRate: 4, OperatorRate: 1}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuOrdersReadyForProfitSharingParams{})).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{{
			PaymentOrderID: paymentOrder.ID,
			OrderID:        paymentOrder.OrderID,
			BusinessType:   paymentOrder.BusinessType,
		}}, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: db.OrderTypeDineIn,
		MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
	}).Return(profitSharingConfig, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(operator, nil)
	expectBaofuReceiverLookup(store, db.BaofuAccountOwnerTypeMerchant, merchant.ID, "MER_SHARE")
	expectBaofuReceiverLookup(store, db.BaofuAccountOwnerTypeOperator, operator.ID, "OP_SHARE")
	expectBaofuReceiverLookup(store, db.BaofuAccountOwnerTypePlatform, int64(0), "PLATFORM_SHARE")
	store.EXPECT().
		EnsureBaofuProfitSharingBillTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateBaofuProfitSharingOrderTxParams) (db.CreateBaofuProfitSharingOrderTxResult, error) {
			require.Equal(t, paymentOrder.ID, arg.ProfitSharingOrder.PaymentOrderID)
			require.Equal(t, merchant.ID, arg.ProfitSharingOrder.MerchantID)
			require.Equal(t, operator.ID, arg.ProfitSharingOrder.OperatorID.Int64)
			require.Equal(t, int32(400), arg.ProfitSharingOrder.PlatformRate)
			require.Equal(t, int32(100), arg.ProfitSharingOrder.OperatorRate)
			require.Equal(t, int64(0), arg.ProfitSharingOrder.PlatformCommission)
			require.Equal(t, int64(0), arg.ProfitSharingOrder.OperatorCommission)
			require.EqualValues(t, 30, arg.ProfitSharingOrder.PaymentFee)
			require.Equal(t, int64(9940), arg.ProfitSharingOrder.MerchantAmount)
			require.Equal(t, db.ProfitSharingSettlementModeFeeOnlyShare, arg.FeeBreakdown.SettlementMode)
			require.Equal(t, int64(60), arg.FeeBreakdown.MerchantPaymentFee)
			require.Equal(t, int64(30), arg.FeeBreakdown.PlatformReceiverAmount)
			return db.CreateBaofuProfitSharingOrderTxResult{
				ProfitSharingOrder: db.ProfitSharingOrder{ID: 801, PaymentOrderID: paymentOrder.ID, OutOrderNo: arg.ProfitSharingOrder.OutOrderNo},
				PaymentFeeLedger:   db.BaofuFeeLedger{ID: 901},
			}, nil
		})
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{}, nil)
	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()

	require.Equal(t, []int64{801}, distributor.profitSharingOrderIDs)
}

func TestBaofuPaymentRecoverySchedulerRunOnceCreatesReservationShareAndEnqueuesCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}

	paymentOrder := db.PaymentOrder{
		ID:                    302,
		ReservationID:         pgtype.Int8{Int64: 402, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerReservation,
		Amount:                10000,
		OutTradeNo:            "BFR_302",
		TransactionID:         pgtype.Text{String: "BFUP_302", Valid: true},
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	reservation := db.TableReservation{
		ID:         402,
		MerchantID: 502,
		Status:     "completed",
		CompletedAt: pgtype.Timestamptz{
			Time:  time.Now().Add(-time.Minute),
			Valid: true,
		},
	}
	merchant := db.Merchant{ID: 502, RegionID: 602}
	operator := db.Operator{ID: 702, RegionID: 602}
	profitSharingConfig := db.ProfitSharingConfig{PlatformRate: 4, OperatorRate: 1}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuOrdersReadyForProfitSharingParams{})).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{{
			PaymentOrderID: paymentOrder.ID,
			ReservationID:  paymentOrder.ReservationID,
			BusinessType:   paymentOrder.BusinessType,
		}}, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservation.ID).Return(reservation, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: db.OrderTypeReservation,
		MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
	}).Return(profitSharingConfig, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(operator, nil)
	expectBaofuReceiverLookup(store, db.BaofuAccountOwnerTypeMerchant, merchant.ID, "MER_SHARE")
	expectBaofuReceiverLookup(store, db.BaofuAccountOwnerTypeOperator, operator.ID, "OP_SHARE")
	expectBaofuReceiverLookup(store, db.BaofuAccountOwnerTypePlatform, int64(0), "PLATFORM_SHARE")
	store.EXPECT().
		EnsureBaofuProfitSharingBillTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateBaofuProfitSharingOrderTxParams) (db.CreateBaofuProfitSharingOrderTxResult, error) {
			require.Equal(t, paymentOrder.ID, arg.ProfitSharingOrder.PaymentOrderID)
			require.Equal(t, merchant.ID, arg.ProfitSharingOrder.MerchantID)
			require.Equal(t, operator.ID, arg.ProfitSharingOrder.OperatorID.Int64)
			require.Equal(t, db.OrderTypeReservation, arg.ProfitSharingOrder.OrderSource)
			require.Equal(t, int64(0), arg.ProfitSharingOrder.DeliveryFee)
			require.False(t, arg.ProfitSharingOrder.RiderID.Valid)
			require.Equal(t, int64(0), arg.ProfitSharingOrder.RiderAmount)
			require.Equal(t, int32(400), arg.ProfitSharingOrder.PlatformRate)
			require.Equal(t, int32(100), arg.ProfitSharingOrder.OperatorRate)
			require.Equal(t, int64(400), arg.ProfitSharingOrder.PlatformCommission)
			require.Equal(t, int64(100), arg.ProfitSharingOrder.OperatorCommission)
			require.EqualValues(t, 30, arg.ProfitSharingOrder.PaymentFee)
			require.Equal(t, int64(9440), arg.ProfitSharingOrder.MerchantAmount)
			require.Equal(t, "BFPS302R402", arg.ProfitSharingOrder.OutOrderNo)
			require.Equal(t, db.ProfitSharingSettlementModeCommissionShare, arg.FeeBreakdown.SettlementMode)
			require.Equal(t, int64(60), arg.FeeBreakdown.MerchantPaymentFee)
			require.Equal(t, int64(10000), arg.FeeBreakdown.CommissionBaseAmount)
			require.Equal(t, int64(430), arg.FeeBreakdown.PlatformReceiverAmount)
			return db.CreateBaofuProfitSharingOrderTxResult{
				ProfitSharingOrder: db.ProfitSharingOrder{ID: 802, PaymentOrderID: paymentOrder.ID, OutOrderNo: arg.ProfitSharingOrder.OutOrderNo},
				PaymentFeeLedger:   db.BaofuFeeLedger{ID: 902},
			}, nil
		})
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{}, nil)
	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()

	require.Equal(t, []int64{802}, distributor.profitSharingOrderIDs)
}

func TestBaofuPaymentRecoverySchedulerRunOnceSkipsReservationShareWithoutCompletedAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}

	paymentOrder := db.PaymentOrder{
		ID:                    312,
		ReservationID:         pgtype.Int8{Int64: 412, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerReservation,
		Amount:                10000,
		OutTradeNo:            "BFR_312",
		TransactionID:         pgtype.Text{String: "BFUP_312", Valid: true},
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	reservation := db.TableReservation{
		ID:         412,
		MerchantID: 512,
		Status:     "completed",
	}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuOrdersReadyForProfitSharingParams{})).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{{
			PaymentOrderID: paymentOrder.ID,
			ReservationID:  paymentOrder.ReservationID,
			BusinessType:   paymentOrder.BusinessType,
		}}, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservation.ID).Return(reservation, nil)
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{}, nil)

	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()

	require.Empty(t, distributor.profitSharingOrderIDs)
}

func TestBaofuPaymentRecoverySchedulerRunOnceEnqueuesExistingPendingShare(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}

	shareOrder := db.ProfitSharingOrder{
		ID:             803,
		PaymentOrderID: 303,
		Provider:       db.ExternalPaymentProviderBaofu,
		Channel:        db.PaymentChannelBaofuAggregate,
		Status:         db.ProfitSharingOrderStatusPending,
		CreatedAt:      time.Now().Add(-3 * time.Minute),
	}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuOrdersReadyForProfitSharingParams{})).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{}, nil)
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{shareOrder}, nil)
	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()

	require.Equal(t, []int64{803}, distributor.profitSharingOrderIDs)
}

func TestBaofuPaymentRecoverySchedulerRunOnceQueriesProcessingShareAndEnqueuesFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}
	client := &baofuRecoveryAggregateClient{
		shareResult: &aggregatecontracts.ShareResult{
			TradeNo:          "BFSHARE_UP_301",
			OutTradeNo:       "BFPS301O401",
			TxnState:         aggregatecontracts.ShareStateSuccess,
			SuccessAmountFen: 10000,
			Raw:              json.RawMessage(`{"txnState":"SUCCESS"}`),
		},
	}
	shareOrder := db.ProfitSharingOrder{
		ID:                 801,
		PaymentOrderID:     301,
		OutOrderNo:         "BFPS301O401",
		SharingOrderID:     pgtype.Text{String: "BFSHARE_UP_301", Valid: true},
		Status:             db.ProfitSharingOrderStatusProcessing,
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		MerchantAmount:     9470,
		PlatformCommission: 200,
		OperatorCommission: 300,
		PaymentFee:         30,
	}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{}, nil)
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListBaofuProcessingProfitSharingOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{shareOrder}, nil)
	store.EXPECT().
		ListBaofuPendingPaymentOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuProfitSharing, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceManualReconciliation, arg.FactSource)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, shareOrder.ID, arg.BusinessObjectID.Int64)
			return db.ExternalPaymentFact{ID: 1101, IsTerminal: true}, nil
		})
	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentFactApplication{ID: 1201}, nil)

	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.SetBaofuAggregateClient(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	require.Equal(t, []int64{1201}, distributor.factApplicationIDs)
	require.Equal(t, "COLLECT_MER", client.lastShareQuery.MerchantID)
	require.Equal(t, "COLLECT_TER", client.lastShareQuery.TerminalID)
	require.Equal(t, "BFSHARE_UP_301", client.lastShareQuery.TradeNo)
	require.Empty(t, client.lastShareQuery.OutTradeNo)
}

func TestBaofuPaymentRecoverySchedulerRunOnceQueriesProcessingShareByOutTradeNoWhenUpstreamShareIDMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}
	client := &baofuRecoveryAggregateClient{
		shareResult: &aggregatecontracts.ShareResult{
			TradeNo:          "BFSHARE_UP_302",
			OutTradeNo:       "BFPS302O402",
			TxnState:         aggregatecontracts.ShareStateSuccess,
			SuccessAmountFen: 10000,
			Raw:              json.RawMessage(`{"txnState":"SUCCESS"}`),
		},
	}
	shareOrder := db.ProfitSharingOrder{
		ID:                 802,
		PaymentOrderID:     302,
		OutOrderNo:         "BFPS302O402",
		SharingOrderID:     pgtype.Text{String: "BFPS302O402", Valid: true},
		Status:             db.ProfitSharingOrderStatusProcessing,
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		MerchantAmount:     9470,
		PlatformCommission: 200,
		OperatorCommission: 300,
		PaymentFee:         30,
	}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{}, nil)
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListBaofuProcessingProfitSharingOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{shareOrder}, nil)
	store.EXPECT().
		ListBaofuPendingPaymentOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentFact{ID: 1102, IsTerminal: true}, nil)
	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentFactApplication{ID: 1202}, nil)

	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.SetBaofuAggregateClient(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	require.Equal(t, []int64{1202}, distributor.factApplicationIDs)
	require.Empty(t, client.lastShareQuery.TradeNo)
	require.Equal(t, "BFPS302O402", client.lastShareQuery.OutTradeNo)
}

func TestBaofuPaymentRecoverySchedulerRunOnceMarksProcessingShareFailedWhenProviderHasNoRelation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}
	client := &baofuRecoveryAggregateClient{
		shareErrors: []error{baofu.NewProviderBusinessError("share_query", "ORDER_NOT_EXIST", "raw upstream order not found")},
	}
	shareOrder := db.ProfitSharingOrder{
		ID:             805,
		PaymentOrderID: 305,
		OutOrderNo:     "BFPS305O405",
		Status:         db.ProfitSharingOrderStatusProcessing,
		Provider:       db.ExternalPaymentProviderBaofu,
		Channel:        db.PaymentChannelBaofuAggregate,
	}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{}, nil)
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListBaofuProcessingProfitSharingOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{shareOrder}, nil)
	store.EXPECT().
		UpdateProfitSharingOrderToFailed(gomock.Any(), shareOrder.ID).
		Return(profitSharingOrderWithStatus(shareOrder, db.ProfitSharingOrderStatusFailed), nil)
	store.EXPECT().
		ListBaofuPendingPaymentOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{}, nil)

	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.SetBaofuAggregateClient(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	require.Empty(t, distributor.factApplicationIDs)
	require.Len(t, client.shareQueries, 1)
	require.Empty(t, client.lastShareQuery.TradeNo)
	require.Equal(t, shareOrder.OutOrderNo, client.lastShareQuery.OutTradeNo)
}

func TestBaofuPaymentRecoverySchedulerRunOnceRetriesProcessingShareByOutTradeNoWhenTradeNoInvalidData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}
	client := &baofuRecoveryAggregateClient{
		shareErrors: []error{&baofu.ProviderError{
			Operation:    "share_query",
			UpstreamCode: baofu.PublicEnvelopeUpstreamCodeInvalidDataContent,
		}},
		shareResults: []*aggregatecontracts.ShareResult{nil, {
			TradeNo:          "260524111107334577001122",
			OutTradeNo:       "BFPS303O403",
			TxnState:         aggregatecontracts.ShareStateSuccess,
			SuccessAmountFen: 10000,
			Raw:              json.RawMessage(`{"txnState":"SUCCESS"}`),
		}},
	}
	shareOrder := db.ProfitSharingOrder{
		ID:                 803,
		PaymentOrderID:     303,
		OutOrderNo:         "BFPS303O403",
		SharingOrderID:     pgtype.Text{String: "260524111107334577001122", Valid: true},
		Status:             db.ProfitSharingOrderStatusProcessing,
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		MerchantAmount:     9470,
		PlatformCommission: 200,
		OperatorCommission: 300,
		PaymentFee:         30,
	}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{}, nil)
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListBaofuProcessingProfitSharingOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{shareOrder}, nil)
	store.EXPECT().
		ListBaofuPendingPaymentOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentFact{ID: 1103, IsTerminal: true}, nil)
	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentFactApplication{ID: 1203}, nil)

	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.SetBaofuAggregateClient(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	require.Equal(t, []int64{1203}, distributor.factApplicationIDs)
	require.Len(t, client.shareQueries, 2)
	require.Equal(t, "260524111107334577001122", client.shareQueries[0].TradeNo)
	require.Empty(t, client.shareQueries[0].OutTradeNo)
	require.Empty(t, client.shareQueries[1].TradeNo)
	require.Equal(t, "BFPS303O403", client.shareQueries[1].OutTradeNo)
}

func TestBaofuPaymentRecoverySchedulerRunOnceRetriesProcessingShareByOutTradeNoWhenTradeNoBusinessFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}
	client := &baofuRecoveryAggregateClient{
		shareErrors: []error{baofu.NewProviderBusinessError("share_query", "ORDER_NOT_EXIST", "raw upstream order not found")},
		shareResults: []*aggregatecontracts.ShareResult{nil, {
			TradeNo:          "260524111107334577001123",
			OutTradeNo:       "BFPS304O404",
			TxnState:         aggregatecontracts.ShareStateSuccess,
			SuccessAmountFen: 10000,
			Raw:              json.RawMessage(`{"txnState":"SUCCESS"}`),
		}},
	}
	shareOrder := db.ProfitSharingOrder{
		ID:                 804,
		PaymentOrderID:     304,
		OutOrderNo:         "BFPS304O404",
		SharingOrderID:     pgtype.Text{String: "260524111107334577001123", Valid: true},
		Status:             db.ProfitSharingOrderStatusProcessing,
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		MerchantAmount:     9470,
		PlatformCommission: 200,
		OperatorCommission: 300,
		PaymentFee:         30,
	}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{}, nil)
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListBaofuProcessingProfitSharingOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{shareOrder}, nil)
	store.EXPECT().
		ListBaofuPendingPaymentOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentFact{ID: 1104, IsTerminal: true}, nil)
	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentFactApplication{ID: 1204}, nil)

	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.SetBaofuAggregateClient(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	require.Equal(t, []int64{1204}, distributor.factApplicationIDs)
	require.Len(t, client.shareQueries, 2)
	require.Equal(t, "260524111107334577001123", client.shareQueries[0].TradeNo)
	require.Empty(t, client.shareQueries[0].OutTradeNo)
	require.Empty(t, client.shareQueries[1].TradeNo)
	require.Equal(t, "BFPS304O404", client.shareQueries[1].OutTradeNo)
}

func TestBaofuPaymentRecoverySchedulerLogsSafeShareQueryProviderErrorDetails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}
	client := &baofuRecoveryAggregateClient{
		shareErrors: []error{
			baofu.NewProviderBusinessError("share_query", "ORDER_NOT_EXIST", "raw upstream order not found"),
			baofu.NewProviderBusinessError("share_query", "ORDER_NOT_EXIST", "raw upstream order not found"),
		},
	}
	shareOrder := db.ProfitSharingOrder{
		ID:             805,
		PaymentOrderID: 305,
		OutOrderNo:     "BFPS305O405",
		SharingOrderID: pgtype.Text{String: "260524111107334577001124", Valid: true},
		Status:         db.ProfitSharingOrderStatusProcessing,
		Provider:       db.ExternalPaymentProviderBaofu,
		Channel:        db.PaymentChannelBaofuAggregate,
	}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{}, nil)
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListBaofuProcessingProfitSharingOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{shareOrder}, nil)
	store.EXPECT().
		ListBaofuPendingPaymentOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{}, nil)

	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.SetBaofuAggregateClient(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	body := logs.String()
	require.Contains(t, body, `"provider_operation":"share_query"`)
	require.Contains(t, body, `"provider_method":"share_query"`)
	require.Contains(t, body, `"upstream_code":"ORDER_NOT_EXIST"`)
	require.Contains(t, body, `"query_key_mode":"tradeNo"`)
	require.Contains(t, body, `"query_key_mode":"outTradeNo"`)
	require.NotContains(t, body, "raw upstream order not found")
}

func TestBaofuPaymentRecoverySchedulerRunOnceQueriesPendingPaymentAndEnqueuesFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}
	client := &baofuRecoveryAggregateClient{
		paymentResult: &aggregatecontracts.UnifiedOrderResult{
			TradeNo:          "BFPAY_UP_301",
			OutTradeNo:       "BFPAY_301",
			TxnState:         aggregatecontracts.PaymentStateSuccess,
			SuccessAmountFen: 9980,
			Raw:              json.RawMessage(`{"txnState":"SUCCESS","succAmt":9980}`),
		},
	}
	paymentOrder := db.PaymentOrder{
		ID:             301,
		OrderID:        pgtype.Int8{Int64: 401, Valid: true},
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		Amount:         10000,
		OutTradeNo:     "BFPAY_301",
		Status:         "pending",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
	}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{}, nil)
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListBaofuProcessingProfitSharingOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListBaofuPendingPaymentOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{paymentOrder}, nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceManualReconciliation, arg.FactSource)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, paymentOrder.ID, arg.BusinessObjectID.Int64)
			require.True(t, arg.Amount.Valid)
			require.Equal(t, int64(9980), arg.Amount.Int64)
			return db.ExternalPaymentFact{ID: 2101, IsTerminal: true}, nil
		})
	store.EXPECT().
		CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentFactApplication{ID: 2201}, nil)

	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.SetBaofuAggregateClient(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	require.Equal(t, []int64{2201}, distributor.factApplicationIDs)
	require.Equal(t, "COLLECT_MER", client.lastPaymentQuery.MerchantID)
	require.Equal(t, "COLLECT_TER", client.lastPaymentQuery.TerminalID)
	require.Equal(t, "BFPAY_301", client.lastPaymentQuery.OutTradeNo)
}

func TestBaofuPaymentRecoverySchedulerRunOnceRecordsEmptyPaymentStateAsUnknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}
	client := &baofuRecoveryAggregateClient{
		paymentResult: &aggregatecontracts.UnifiedOrderResult{
			TradeNo:          "BFPAY_UP_302",
			OutTradeNo:       "BFPAY_302",
			ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
			SuccessAmountFen: 10000,
			Raw:              json.RawMessage(`{"resultCode":"SUCCESS","succAmt":10000}`),
		},
	}
	paymentOrder := db.PaymentOrder{
		ID:             302,
		OrderID:        pgtype.Int8{Int64: 402, Valid: true},
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		Amount:         10000,
		OutTradeNo:     "BFPAY_302",
		Status:         "pending",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
	}

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{}, nil)
	store.EXPECT().
		ListBaofuProfitSharingOrdersReadyForCommand(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuProfitSharingOrdersReadyForCommandParams{})).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListBaofuProcessingProfitSharingOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListBaofuPendingPaymentOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{paymentOrder}, nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, arg.TerminalStatus)
			require.False(t, arg.IsTerminal)
			require.Empty(t, arg.UpstreamState)
			require.Equal(t, "baofu:manual_reconciliation:payment:BFPAY_302:BFPAY_UP_302:unknown", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 2102, IsTerminal: false}, nil
		})

	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.SetBaofuAggregateClient(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	require.Empty(t, distributor.factApplicationIDs)
}

type baofuProfitSharingEnqueueRecorder struct {
	worker.NoopTaskDistributor
	profitSharingOrderIDs []int64
	factApplicationIDs    []int64
}

func (d *baofuProfitSharingEnqueueRecorder) DistributeTaskProcessBaofuProfitSharing(_ context.Context, payload *worker.BaofuProfitSharingPayload, _ ...asynq.Option) error {
	d.profitSharingOrderIDs = append(d.profitSharingOrderIDs, payload.ProfitSharingOrderID)
	return nil
}

func (d *baofuProfitSharingEnqueueRecorder) DistributeTaskProcessPaymentFactApplication(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
	d.factApplicationIDs = append(d.factApplicationIDs, payload.ApplicationID)
	return nil
}

type baofuRecoveryAggregateClient struct {
	paymentResult    *aggregatecontracts.UnifiedOrderResult
	shareResult      *aggregatecontracts.ShareResult
	shareResults     []*aggregatecontracts.ShareResult
	shareErrors      []error
	refundResult     *aggregatecontracts.RefundResult
	lastPaymentQuery aggregatecontracts.PaymentQueryRequest
	lastShareQuery   aggregatecontracts.ShareQueryRequest
	shareQueries     []aggregatecontracts.ShareQueryRequest
	lastRefundQuery  aggregatecontracts.RefundQueryRequest
}

func (c *baofuRecoveryAggregateClient) CreateUnifiedOrder(context.Context, aggregatecontracts.UnifiedOrderRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, nil
}

func (c *baofuRecoveryAggregateClient) CreateProfitSharing(context.Context, aggregatecontracts.ShareAfterPayRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, nil
}

func (c *baofuRecoveryAggregateClient) QueryPayment(_ context.Context, req aggregatecontracts.PaymentQueryRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	c.lastPaymentQuery = req
	return c.paymentResult, nil
}

func (c *baofuRecoveryAggregateClient) QueryProfitSharing(_ context.Context, req aggregatecontracts.ShareQueryRequest) (*aggregatecontracts.ShareResult, error) {
	c.lastShareQuery = req
	c.shareQueries = append(c.shareQueries, req)
	index := len(c.shareQueries) - 1
	if index < len(c.shareErrors) && c.shareErrors[index] != nil {
		return nil, c.shareErrors[index]
	}
	if index < len(c.shareResults) && c.shareResults[index] != nil {
		return c.shareResults[index], nil
	}
	return c.shareResult, nil
}

func expectBaofuReceiverLookup(store *mockdb.MockStore, ownerType string, ownerID int64, sharingMerID string) {
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: ownerType,
			OwnerID:   ownerID,
		}).
		Return(db.BaofuAccountBinding{
			OwnerType:    ownerType,
			OwnerID:      ownerID,
			OpenState:    db.BaofuAccountOpenStateActive,
			SharingMerID: pgtype.Text{String: sharingMerID, Valid: true},
		}, nil)
}

func (c *baofuRecoveryAggregateClient) CreateRefund(context.Context, aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, nil
}

func (c *baofuRecoveryAggregateClient) QueryRefund(_ context.Context, req aggregatecontracts.RefundQueryRequest) (*aggregatecontracts.RefundResult, error) {
	c.lastRefundQuery = req
	return c.refundResult, nil
}

func (c *baofuRecoveryAggregateClient) CloseOrder(context.Context, aggregatecontracts.OrderCloseRequest) (*aggregatecontracts.OrderCloseResult, error) {
	return nil, nil
}
