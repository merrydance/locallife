package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
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

type baofuProfitSharingEnqueueRecorder struct {
	worker.NoopTaskDistributor
	profitSharingOrderIDs []int64
}

func (d *baofuProfitSharingEnqueueRecorder) DistributeTaskProcessBaofuProfitSharing(_ context.Context, payload *worker.BaofuProfitSharingPayload, _ ...asynq.Option) error {
	d.profitSharingOrderIDs = append(d.profitSharingOrderIDs, payload.ProfitSharingOrderID)
	return nil
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
