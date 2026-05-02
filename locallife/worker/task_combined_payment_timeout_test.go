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
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	mockordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskCombinedPaymentOrderTimeout_RemotePaidReconcilesInsteadOfClosing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	distributor := &combinedOrderPaymentFactApplicationRecorder{}
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
		ID:             3001,
		OutTradeNo:     "PO_REMOTE_PAID_1",
		Amount:         5000,
		Status:         "pending",
		PaymentChannel: db.PaymentChannelEcommerce,
		BusinessType:   "order",
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
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), "PO_REMOTE_PAID_1").Return(pendingPaymentOrder, nil)
	ecommerceClient.EXPECT().QueryCombineOrder(gomock.Any(), "CP_REMOTE_PAID_1").Return(&wechatcontracts.CombineQueryResponse{
		CombineOutTradeNo: "CP_REMOTE_PAID_1",
		SubOrders: []wechatcontracts.CombineSubOrderResult{{
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
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentCapabilityCombinePayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, combinedOrder.CombineOutTradeNo, arg.ExternalObjectKey)
		return db.ExternalPaymentFact{ID: 7001, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(7001), arg.FactID)
		require.Equal(t, paidPaymentOrder.ID, arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 7101, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	distributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(7101), payload.ApplicationID)
		return nil
	}
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

type combinedOrderPaymentFactApplicationRecorder struct {
	worker.NoopTaskDistributor
	processPaymentFactApplication func(context.Context, *worker.PaymentFactApplicationPayload, ...asynq.Option) error
}

func (d *combinedOrderPaymentFactApplicationRecorder) DistributeTaskProcessPaymentFactApplication(ctx context.Context, payload *worker.PaymentFactApplicationPayload, opts ...asynq.Option) error {
	if d.processPaymentFactApplication == nil {
		return nil
	}
	return d.processPaymentFactApplication(ctx, payload, opts...)
}

func TestProcessTaskCombinedPaymentOrderTimeout_ClosedMainOrderRemotePaidReconcilesToPaid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	distributor := &combinedOrderPaymentFactApplicationRecorder{}
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
		ID:             3002,
		OutTradeNo:     "PO_REMOTE_PAID_2",
		Amount:         6800,
		Status:         "pending",
		PaymentChannel: db.PaymentChannelEcommerce,
		BusinessType:   "order",
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
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), "PO_REMOTE_PAID_2").Return(pendingPaymentOrder, nil)
	ecommerceClient.EXPECT().QueryCombineOrder(gomock.Any(), "CP_REMOTE_PAID_2").Return(&wechatcontracts.CombineQueryResponse{
		CombineOutTradeNo: "CP_REMOTE_PAID_2",
		SubOrders: []wechatcontracts.CombineSubOrderResult{{
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
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentCapabilityCombinePayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, combinedOrder.CombineOutTradeNo, arg.ExternalObjectKey)
		return db.ExternalPaymentFact{ID: 7002, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(7002), arg.FactID)
		require.Equal(t, paidPaymentOrder.ID, arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 7102, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	distributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(7102), payload.ApplicationID)
		return nil
	}
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

func TestProcessTaskCombinedPaymentOrderTimeout_RemotePaidReservationReconcilesToFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	distributor := &combinedOrderPaymentFactApplicationRecorder{}
	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)

	subOrders, err := json.Marshal([]map[string]any{{
		"sub_mchid":    "1900003333",
		"out_trade_no": "PO_REMOTE_PAID_RES_1",
	}})
	require.NoError(t, err)

	combinedOrder := db.CombinedPaymentOrder{ID: 2003, CombineOutTradeNo: "CP_REMOTE_PAID_RES_1", Status: "pending", ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true}}
	pendingPaymentOrder := db.PaymentOrder{ID: 3003, OutTradeNo: "PO_REMOTE_PAID_RES_1", Amount: 7200, Status: "pending", PaymentChannel: db.PaymentChannelEcommerce, BusinessType: db.ExternalPaymentBusinessOwnerReservation, ReservationID: pgtype.Int8{Int64: 4003, Valid: true}}
	paidPaymentOrder := pendingPaymentOrder
	paidPaymentOrder.Status = "paid"
	paidPaymentOrder.TransactionID = pgtype.Text{String: "wx_tx_remote_paid_res_1", Valid: true}
	paidPaymentOrder.PaidAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}

	store.EXPECT().GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), combinedOrder.CombineOutTradeNo).Return(combinedOrder, nil)
	store.EXPECT().GetCombinedPaymentOrderWithSubOrders(gomock.Any(), combinedOrder.ID).Return(db.GetCombinedPaymentOrderWithSubOrdersRow{ID: combinedOrder.ID, CombineOutTradeNo: combinedOrder.CombineOutTradeNo, Status: combinedOrder.Status, SubOrders: subOrders}, nil)
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), pendingPaymentOrder.OutTradeNo).Return(pendingPaymentOrder, nil)
	ecommerceClient.EXPECT().QueryCombineOrder(gomock.Any(), combinedOrder.CombineOutTradeNo).Return(&wechatcontracts.CombineQueryResponse{
		CombineOutTradeNo: combinedOrder.CombineOutTradeNo,
		SubOrders: []wechatcontracts.CombineSubOrderResult{{
			SubMchID:      "1900003333",
			OutTradeNo:    pendingPaymentOrder.OutTradeNo,
			TransactionID: "wx_tx_remote_paid_res_1",
			TradeType:     "JSAPI",
			TradeState:    "SUCCESS",
			Amount: struct {
				TotalAmount    int64  `json:"total_amount"`
				PayerAmount    int64  `json:"payer_amount"`
				Currency       string `json:"currency"`
				PayerCurrency  string `json:"payer_currency"`
				SettlementRate int64  `json:"settlement_rate"`
			}{TotalAmount: 7200, PayerAmount: 7200, Currency: "CNY", PayerCurrency: "CNY"},
		}},
	}, nil)
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), pendingPaymentOrder.OutTradeNo).Return(pendingPaymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{ID: pendingPaymentOrder.ID, TransactionID: pgtype.Text{String: "wx_tx_remote_paid_res_1", Valid: true}}).Return(paidPaymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner.String)
		return db.ExternalPaymentFact{ID: 7003, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(7003), arg.FactID)
		require.Equal(t, paidPaymentOrder.ID, arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 7103, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	distributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(7103), payload.ApplicationID)
		return nil
	}
	store.EXPECT().UpdateCombinedPaymentOrderToPaid(gomock.Any(), db.UpdateCombinedPaymentOrderToPaidParams{ID: combinedOrder.ID, TransactionID: pgtype.Text{Valid: false}}).Return(db.CombinedPaymentOrder{ID: combinedOrder.ID, Status: "paid"}, nil)

	task := asynq.NewTask(worker.TaskCombinedPaymentOrderTimeout, mustMarshalJSON(t, worker.PayloadCombinedPaymentOrderTimeout{CombineOutTradeNo: combinedOrder.CombineOutTradeNo}))
	err = processor.ProcessTaskCombinedPaymentOrderTimeout(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskCombinedPaymentOrderTimeout_RoutesHistoricalEcommerceBySubOrderChannel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	processor.SetOrdinaryServiceProviderClient(ordinaryClient)

	combinedOrder := db.CombinedPaymentOrder{
		ID:                2004,
		CombineOutTradeNo: "CP_ECOMMERCE_COLD_1",
		Status:            "pending",
		ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	}
	subOrders, err := json.Marshal([]map[string]any{{
		"sub_mchid":    "1900004444",
		"out_trade_no": "PO_ECOMMERCE_COLD_1",
	}})
	require.NoError(t, err)
	paymentOrder := db.PaymentOrder{
		ID:             3004,
		OutTradeNo:     "PO_ECOMMERCE_COLD_1",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         3200,
		Status:         "pending",
		BusinessType:   "order",
	}

	store.EXPECT().GetCombinedPaymentOrderByOutTradeNo(gomock.Any(), combinedOrder.CombineOutTradeNo).Return(combinedOrder, nil)
	store.EXPECT().GetCombinedPaymentOrderWithSubOrders(gomock.Any(), combinedOrder.ID).Return(db.GetCombinedPaymentOrderWithSubOrdersRow{
		ID:                combinedOrder.ID,
		CombineOutTradeNo: combinedOrder.CombineOutTradeNo,
		Status:            combinedOrder.Status,
		SubOrders:         subOrders,
	}, nil)
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(paymentOrder, nil)
	ecommerceClient.EXPECT().QueryCombineOrder(gomock.Any(), combinedOrder.CombineOutTradeNo).Return(&wechatcontracts.CombineQueryResponse{
		CombineOutTradeNo: combinedOrder.CombineOutTradeNo,
		SubOrders: []wechatcontracts.CombineSubOrderResult{{
			SubMchID:   "1900004444",
			OutTradeNo: paymentOrder.OutTradeNo,
			TradeState: "NOTPAY",
		}},
	}, nil)
	ecommerceClient.EXPECT().CloseCombineOrder(gomock.Any(), combinedOrder.CombineOutTradeNo, gomock.Any()).Return(nil)
	store.EXPECT().UpdateCombinedPaymentOrderToClosed(gomock.Any(), combinedOrder.ID).Return(db.CombinedPaymentOrder{ID: combinedOrder.ID, Status: "closed"}, nil)
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(paymentOrder, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "closed"}, nil)

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
