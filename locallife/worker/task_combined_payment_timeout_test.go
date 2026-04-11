package worker_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskCombinedPaymentOrderTimeout_RemotePaidReconcilesInsteadOfClosing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)

	subOrders, err := json.Marshal([]map[string]any{{
		"sub_mchid":    "1900001111",
		"out_trade_no": "PO_REMOTE_PAID_1",
	}})
	require.NoError(t, err)

	combinedOrder := db.CombinedPaymentOrder{
		ID:                2001,
		CombineOutTradeNo: "CP_REMOTE_PAID_1",
		Status:            "pending",
		ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	}
	pendingPaymentOrder := db.PaymentOrder{
		ID:           3001,
		OutTradeNo:   "PO_REMOTE_PAID_1",
		Amount:       5000,
		Status:       "pending",
		BusinessType: "order",
	}
	paidPaymentOrder := pendingPaymentOrder
	paidPaymentOrder.Status = "paid"
	paidPaymentOrder.TransactionID = pgtype.Text{String: "wx_tx_remote_paid_1", Valid: true}
	paidPaymentOrder.PaidAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}

	store.EXPECT().GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), "CP_REMOTE_PAID_1").Return(combinedOrder, nil)
	store.EXPECT().GetCombinedPaymentOrderWithSubOrders(gomock.Any(), combinedOrder.ID).Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
		ID:                combinedOrder.ID,
		CombineOutTradeNo: combinedOrder.CombineOutTradeNo,
		Status:            combinedOrder.Status,
		SubOrders:         subOrders,
	}, nil)
	ecommerceClient.EXPECT().QueryCombineOrder(gomock.Any(), "CP_REMOTE_PAID_1").Return(&wechat.CombineQueryResponse{
		CombineOutTradeNo: "CP_REMOTE_PAID_1",
		SubOrders: []wechat.CombineSubOrderResult{{
			SubMchID:      "1900001111",
			OutTradeNo:    "PO_REMOTE_PAID_1",
			TransactionID: "wx_tx_remote_paid_1",
			TradeType:     "JSAPI",
			TradeState:    "SUCCESS",
			Amount: struct {
				TotalAmount    int64  `json:"total_amount"`
				PayerAmount    int64  `json:"payer_amount"`
				Currency       string `json:"currency"`
				PayerCurrency  string `json:"payer_currency"`
				SettlementRate int64  `json:"settlement_rate"`
			}{
				TotalAmount:   5000,
				PayerAmount:   5000,
				Currency:      "CNY",
				PayerCurrency: "CNY",
			},
		}},
	}, nil)
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), "PO_REMOTE_PAID_1").Return(pendingPaymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            pendingPaymentOrder.ID,
		TransactionID: pgtype.Text{String: "wx_tx_remote_paid_1", Valid: true},
	}).Return(paidPaymentOrder, nil)
	distributor.EXPECT().
		DistributeTaskProcessPaymentSuccess(gomock.Any(), gomock.AssignableToTypeOf(&worker.PaymentSuccessPayload{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.PaymentSuccessPayload, _ ...asynq.Option) error {
			require.Equal(t, paidPaymentOrder.ID, payload.PaymentOrderID)
			require.Equal(t, "wx_tx_remote_paid_1", payload.TransactionID)
			require.Equal(t, "order", payload.BusinessType)
			return nil
		})
	store.EXPECT().UpdateCombinedPaymentOrderToPaid(gomock.Any(), db.UpdateCombinedPaymentOrderToPaidParams{
		ID:            combinedOrder.ID,
		TransactionID: pgtype.Text{Valid: false},
	}).Return(db.CombinedPaymentOrder{ID: combinedOrder.ID, Status: "paid"}, nil)

	task := asynq.NewTask(worker.TaskCombinedPaymentOrderTimeout, mustMarshalJSON(t, worker.PayloadCombinedPaymentOrderTimeout{
		CombineOutTradeNo: combinedOrder.CombineOutTradeNo,
	}))

	err = processor.ProcessTaskCombinedPaymentOrderTimeout(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskCombinedPaymentOrderTimeout_ClosedMainOrderRemotePaidReconcilesToPaid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)

	subOrders, err := json.Marshal([]map[string]any{{
		"sub_mchid":    "1900002222",
		"out_trade_no": "PO_REMOTE_PAID_2",
	}})
	require.NoError(t, err)

	combinedOrder := db.CombinedPaymentOrder{
		ID:                2002,
		CombineOutTradeNo: "CP_REMOTE_PAID_2",
		Status:            "closed",
		ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	}
	pendingPaymentOrder := db.PaymentOrder{
		ID:           3002,
		OutTradeNo:   "PO_REMOTE_PAID_2",
		Amount:       6800,
		Status:       "pending",
		BusinessType: "order",
	}
	paidPaymentOrder := pendingPaymentOrder
	paidPaymentOrder.Status = "paid"
	paidPaymentOrder.TransactionID = pgtype.Text{String: "wx_tx_remote_paid_2", Valid: true}
	paidPaymentOrder.PaidAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}

	store.EXPECT().GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), "CP_REMOTE_PAID_2").Return(combinedOrder, nil)
	store.EXPECT().GetCombinedPaymentOrderWithSubOrders(gomock.Any(), combinedOrder.ID).Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
		ID:                combinedOrder.ID,
		CombineOutTradeNo: combinedOrder.CombineOutTradeNo,
		Status:            combinedOrder.Status,
		SubOrders:         subOrders,
	}, nil)
	ecommerceClient.EXPECT().QueryCombineOrder(gomock.Any(), "CP_REMOTE_PAID_2").Return(&wechat.CombineQueryResponse{
		CombineOutTradeNo: "CP_REMOTE_PAID_2",
		SubOrders: []wechat.CombineSubOrderResult{{
			SubMchID:      "1900002222",
			OutTradeNo:    "PO_REMOTE_PAID_2",
			TransactionID: "wx_tx_remote_paid_2",
			TradeType:     "JSAPI",
			TradeState:    "SUCCESS",
			Amount: struct {
				TotalAmount    int64  `json:"total_amount"`
				PayerAmount    int64  `json:"payer_amount"`
				Currency       string `json:"currency"`
				PayerCurrency  string `json:"payer_currency"`
				SettlementRate int64  `json:"settlement_rate"`
			}{
				TotalAmount:   6800,
				PayerAmount:   6800,
				Currency:      "CNY",
				PayerCurrency: "CNY",
			},
		}},
	}, nil)
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), "PO_REMOTE_PAID_2").Return(pendingPaymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            pendingPaymentOrder.ID,
		TransactionID: pgtype.Text{String: "wx_tx_remote_paid_2", Valid: true},
	}).Return(paidPaymentOrder, nil)
	distributor.EXPECT().
		DistributeTaskProcessPaymentSuccess(gomock.Any(), gomock.AssignableToTypeOf(&worker.PaymentSuccessPayload{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.PaymentSuccessPayload, _ ...asynq.Option) error {
			require.Equal(t, paidPaymentOrder.ID, payload.PaymentOrderID)
			require.Equal(t, "wx_tx_remote_paid_2", payload.TransactionID)
			require.Equal(t, "order", payload.BusinessType)
			return nil
		})
	store.EXPECT().UpdateCombinedPaymentOrderToPaid(gomock.Any(), db.UpdateCombinedPaymentOrderToPaidParams{
		ID:            combinedOrder.ID,
		TransactionID: pgtype.Text{Valid: false},
	}).Return(db.CombinedPaymentOrder{ID: combinedOrder.ID, Status: "paid"}, nil)

	task := asynq.NewTask(worker.TaskCombinedPaymentOrderTimeout, mustMarshalJSON(t, worker.PayloadCombinedPaymentOrderTimeout{
		CombineOutTradeNo: combinedOrder.CombineOutTradeNo,
	}))

	err = processor.ProcessTaskCombinedPaymentOrderTimeout(context.Background(), task)
	require.NoError(t, err)
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()

	payload, err := json.Marshal(value)
	require.NoError(t, err)
	return payload
}
