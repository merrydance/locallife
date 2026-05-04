package worker_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
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

	store.EXPECT().
		ListBaofuOrdersReadyForProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(db.ListBaofuOrdersReadyForProfitSharingParams{})).
		Return([]db.ListBaofuOrdersReadyForProfitSharingRow{{
			PaymentOrderID: paymentOrder.ID,
			OrderID:        paymentOrder.OrderID,
		}}, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(operator, nil)
	expectBaofuReceiverLookup(store, db.BaofuAccountOwnerTypeMerchant, merchant.ID, "MER_SHARE")
	expectBaofuReceiverLookup(store, db.BaofuAccountOwnerTypeOperator, operator.ID, "OP_SHARE")
	expectBaofuReceiverLookup(store, db.BaofuAccountOwnerTypePlatform, int64(0), "PLATFORM_SHARE")
	store.EXPECT().
		CreateBaofuProfitSharingOrderTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateBaofuProfitSharingOrderTxParams) (db.CreateBaofuProfitSharingOrderTxResult, error) {
			require.Equal(t, paymentOrder.ID, arg.ProfitSharingOrder.PaymentOrderID)
			require.Equal(t, merchant.ID, arg.ProfitSharingOrder.MerchantID)
			require.Equal(t, operator.ID, arg.ProfitSharingOrder.OperatorID.Int64)
			require.Equal(t, int32(200), arg.ProfitSharingOrder.PlatformRate)
			require.Equal(t, int32(300), arg.ProfitSharingOrder.OperatorRate)
			require.Equal(t, int64(200), arg.ProfitSharingOrder.PlatformCommission)
			require.Equal(t, int64(300), arg.ProfitSharingOrder.OperatorCommission)
			require.EqualValues(t, 30, arg.ProfitSharingOrder.PaymentFee)
			require.Equal(t, int64(9470), arg.ProfitSharingOrder.MerchantAmount)
			return db.CreateBaofuProfitSharingOrderTxResult{
				ProfitSharingOrder: db.ProfitSharingOrder{ID: 801, PaymentOrderID: paymentOrder.ID, OutOrderNo: arg.ProfitSharingOrder.OutOrderNo},
				PaymentFeeLedger:   db.BaofuFeeLedger{ID: 901},
			}, nil
		})

	scheduler := worker.NewBaofuPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()

	require.Equal(t, []int64{801}, distributor.profitSharingOrderIDs)
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

func TestBaofuPaymentRecoverySchedulerRunOnceQueriesPendingPaymentAndEnqueuesFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &baofuProfitSharingEnqueueRecorder{}
	client := &baofuRecoveryAggregateClient{
		paymentResult: &aggregatecontracts.UnifiedOrderResult{
			TradeNo:    "BFPAY_UP_301",
			OutTradeNo: "BFPAY_301",
			TxnState:   aggregatecontracts.PaymentStateSuccess,
			Raw:        json.RawMessage(`{"txnState":"SUCCESS"}`),
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
	lastPaymentQuery aggregatecontracts.PaymentQueryRequest
	lastShareQuery   aggregatecontracts.ShareQueryRequest
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

func (c *baofuRecoveryAggregateClient) QueryRefund(context.Context, aggregatecontracts.RefundQueryRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, nil
}

func (c *baofuRecoveryAggregateClient) CloseOrder(context.Context, aggregatecontracts.OrderCloseRequest) (*aggregatecontracts.OrderCloseResult, error) {
	return nil, nil
}
