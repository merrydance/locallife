package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskInitiateRefund_MembershipRechargeMismatchRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	attachBytes, err := json.Marshal(map[string]int64{"membership_id": 77})
	require.NoError(t, err)

	paymentOrder := db.PaymentOrder{
		ID:           2,
		OutTradeNo:   "MBR_PAY_2",
		Amount:       10000,
		Status:       "paid",
		BusinessType: "membership_recharge",
		PaymentType:  "profit_sharing",
		Attach:       pgtype.Text{String: string(attachBytes), Valid: true},
	}
	refundOrder := db.RefundOrder{ID: 22, PaymentOrderID: 2, Status: "pending", OutRefundNo: "RFM2_M"}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(2)).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetMembershipForUpdate(gomock.Any(), int64(77)).
		Return(db.MerchantMembership{ID: 77, MerchantID: 55}, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(55)).
		Return(db.MerchantPaymentConfig{MerchantID: 55, SubMchID: "sub_mch_55"}, nil)
	store.EXPECT().
		GetRefundOrderByOutRefundNo(gomock.Any(), "RFM2_M").
		Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderParams) (db.RefundOrder, error) {
			require.Equal(t, int64(2), arg.PaymentOrderID)
			require.Equal(t, int64(12000), arg.RefundAmount)
			require.Equal(t, "RFM2_M", arg.OutRefundNo)
			return refundOrder, nil
		})
	ecommerceClient.EXPECT().
		CreateEcommerceRefund(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.EcommerceRefundRequest) (*wechat.EcommerceRefundResponse, error) {
			require.Equal(t, "sub_mch_55", req.SubMchID)
			require.Equal(t, "MBR_PAY_2", req.OutTradeNo)
			require.Equal(t, "RFM2_M", req.OutRefundNo)
			require.Equal(t, int64(12000), req.RefundAmount)
			require.Equal(t, int64(12000), req.TotalAmount)
			return &wechat.EcommerceRefundResponse{RefundID: "refund_membership_2", Status: wechat.RefundStatusSuccess}, nil
		})
	store.EXPECT().
		UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).
		Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, Status: "success", OutRefundNo: refundOrder.OutRefundNo}, nil)
	store.EXPECT().
		GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(int64(12000), nil)
	store.EXPECT().
		UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).
		Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   12000,
		Reason:         "金额异常，系统自动退款",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskInitiateRefund_RiderDepositMismatchRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:           3,
		OutTradeNo:   "RIDER_PAY_3",
		Amount:       10000,
		Status:       "paid",
		BusinessType: "rider_deposit",
		PaymentType:  "miniprogram",
	}
	refundOrder := db.RefundOrder{ID: 33, PaymentOrderID: 3, Status: "pending", OutRefundNo: "RFM3_D"}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(3)).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetRefundOrderByOutRefundNo(gomock.Any(), "RFM3_D").
		Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderParams) (db.RefundOrder, error) {
			require.Equal(t, int64(3), arg.PaymentOrderID)
			require.Equal(t, int64(12000), arg.RefundAmount)
			require.Equal(t, "RFM3_D", arg.OutRefundNo)
			return refundOrder, nil
		})
	ecommerceClient.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.RefundRequest) (*wechat.RefundResponse, error) {
			require.Equal(t, "RIDER_PAY_3", req.OutTradeNo)
			require.Equal(t, "RFM3_D", req.OutRefundNo)
			require.Equal(t, int64(12000), req.RefundAmount)
			require.Equal(t, int64(12000), req.TotalAmount)
			return &wechat.RefundResponse{RefundID: "refund_rider_3", Status: wechat.RefundStatusSuccess}, nil
		})
	store.EXPECT().
		UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).
		Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, Status: "success", OutRefundNo: refundOrder.OutRefundNo}, nil)
	store.EXPECT().
		GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(int64(12000), nil)
	store.EXPECT().
		UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).
		Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   12000,
		Reason:         "金额异常，系统自动退款",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskInitiateRefund_MembershipRechargeMismatchRefund_StatusPersistFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	attachBytes, err := json.Marshal(map[string]int64{"membership_id": 77})
	require.NoError(t, err)

	paymentOrder := db.PaymentOrder{
		ID:           2,
		OutTradeNo:   "MBR_PAY_2",
		Amount:       10000,
		Status:       "paid",
		BusinessType: "membership_recharge",
		PaymentType:  "profit_sharing",
		Attach:       pgtype.Text{String: string(attachBytes), Valid: true},
	}
	refundOrder := db.RefundOrder{ID: 22, PaymentOrderID: 2, Status: "pending", OutRefundNo: "RFM2_M"}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(2)).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetMembershipForUpdate(gomock.Any(), int64(77)).
		Return(db.MerchantMembership{ID: 77, MerchantID: 55}, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(55)).
		Return(db.MerchantPaymentConfig{MerchantID: 55, SubMchID: "sub_mch_55"}, nil)
	store.EXPECT().
		GetRefundOrderByOutRefundNo(gomock.Any(), "RFM2_M").
		Return(db.RefundOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.Any()).
		Return(refundOrder, nil)
	ecommerceClient.EXPECT().
		CreateEcommerceRefund(gomock.Any(), gomock.Any()).
		Return(&wechat.EcommerceRefundResponse{RefundID: "refund_membership_2", Status: wechat.RefundStatusSuccess}, nil)
	store.EXPECT().
		UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).
		Return(db.RefundOrder{}, errors.New("persist refund success failed"))

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   12000,
		Reason:         "金额异常，系统自动退款",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mark refund order as success")
}
