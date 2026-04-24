package logic

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRiderDepositRefundService_SubmitWithdrawal_SynchronousSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	service := NewRiderDepositRefundService(store, paymentClient)

	rider := db.Rider{
		ID:            21,
		UserID:        88,
		Status:        "active",
		DepositAmount: 30000,
		FrozenDeposit: 0,
	}
	paymentOrder := db.PaymentOrder{
		ID:         501,
		UserID:     rider.UserID,
		OutTradeNo: "OTN_SUCCESS_001",
		Amount:     30000,
	}
	refundOrder := db.RefundOrder{
		ID:           701,
		OutRefundNo:  "ORN_SUCCESS_001",
		RefundAmount: 30000,
	}
	plan := db.RiderDepositRefundPlan{
		RefundOrder:        refundOrder,
		SourcePaymentOrder: paymentOrder,
	}

	gomock.InOrder(
		store.EXPECT().GetRiderByUserID(gomock.Any(), rider.UserID).Return(rider, nil),
		store.EXPECT().ListRiderActiveDeliveries(gomock.Any(), pgtype.Int8{Int64: rider.ID, Valid: true}).Return([]db.Delivery{}, nil),
		store.EXPECT().PrepareRiderDepositRefundTx(gomock.Any(), db.PrepareRiderDepositRefundTxParams{
			RiderID: rider.ID,
			Amount:  30000,
			Remark:  "押金提现",
		}).Return(db.PrepareRiderDepositRefundTxResult{
			Rider:        db.Rider{ID: rider.ID, UserID: rider.UserID, DepositAmount: 30000, FrozenDeposit: 30000},
			RefundPlans:  []db.RiderDepositRefundPlan{plan},
			FrozenAmount: 30000,
		}, nil),
		paymentClient.EXPECT().CreateRefund(gomock.Any(), &wechat.RefundRequest{
			OutTradeNo:   paymentOrder.OutTradeNo,
			OutRefundNo:  refundOrder.OutRefundNo,
			Reason:       "押金提现",
			RefundAmount: refundOrder.RefundAmount,
			TotalAmount:  paymentOrder.Amount,
		}).Return(&wechat.RefundResponse{
			RefundID:    "WX_REFUND_SUCCESS_001",
			OutRefundNo: refundOrder.OutRefundNo,
			OutTradeNo:  paymentOrder.OutTradeNo,
			Status:      wechat.RefundStatusSuccess,
		}, nil),
		store.EXPECT().ResolveRiderDepositRefundTx(gomock.Any(), db.ResolveRiderDepositRefundTxParams{
			RefundOrderID: refundOrder.ID,
			RefundStatus:  riderDepositRefundStatusSuccess,
			RefundID:      "WX_REFUND_SUCCESS_001",
		}).Return(db.ResolveRiderDepositRefundTxResult{}, nil),
		store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder.Amount, nil),
		store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil),
	)

	result, err := service.SubmitWithdrawal(context.Background(), SubmitRiderDepositWithdrawalInput{
		UserID: rider.UserID,
		Amount: 30000,
		Remark: "押金提现",
	})
	require.NoError(t, err)
	require.Equal(t, int64(30000), result.RequestedAmount)
	require.Equal(t, int64(30000), result.AcceptedAmount)
	require.Equal(t, riderDepositWithdrawStatusSuccess, result.Status)
	require.Len(t, result.Refunds, 1)
	require.Equal(t, riderDepositWithdrawStatusSuccess, result.Refunds[0].Status)
	require.Equal(t, refundOrder.ID, result.Refunds[0].RefundOrder.ID)
	require.Equal(t, paymentOrder.ID, result.Refunds[0].PaymentOrder.ID)
}

func TestRiderDepositRefundService_SubmitWithdrawal_RefundRequestFailureCompensates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	service := NewRiderDepositRefundService(store, paymentClient)

	rider := db.Rider{
		ID:            22,
		UserID:        89,
		Status:        "active",
		DepositAmount: 30000,
		FrozenDeposit: 0,
	}
	paymentOrder := db.PaymentOrder{
		ID:         502,
		UserID:     rider.UserID,
		OutTradeNo: "OTN_FAIL_001",
		Amount:     30000,
	}
	refundOrder := db.RefundOrder{
		ID:           702,
		OutRefundNo:  "ORN_FAIL_001",
		RefundAmount: 30000,
	}
	plan := db.RiderDepositRefundPlan{
		RefundOrder:        refundOrder,
		SourcePaymentOrder: paymentOrder,
	}

	gomock.InOrder(
		store.EXPECT().GetRiderByUserID(gomock.Any(), rider.UserID).Return(rider, nil),
		store.EXPECT().ListRiderActiveDeliveries(gomock.Any(), pgtype.Int8{Int64: rider.ID, Valid: true}).Return([]db.Delivery{}, nil),
		store.EXPECT().PrepareRiderDepositRefundTx(gomock.Any(), db.PrepareRiderDepositRefundTxParams{
			RiderID: rider.ID,
			Amount:  30000,
			Remark:  "押金提现失败补偿",
		}).Return(db.PrepareRiderDepositRefundTxResult{
			Rider:        db.Rider{ID: rider.ID, UserID: rider.UserID, DepositAmount: 30000, FrozenDeposit: 30000},
			RefundPlans:  []db.RiderDepositRefundPlan{plan},
			FrozenAmount: 30000,
		}, nil),
		paymentClient.EXPECT().CreateRefund(gomock.Any(), &wechat.RefundRequest{
			OutTradeNo:   paymentOrder.OutTradeNo,
			OutRefundNo:  refundOrder.OutRefundNo,
			Reason:       "押金提现失败补偿",
			RefundAmount: refundOrder.RefundAmount,
			TotalAmount:  paymentOrder.Amount,
		}).Return(nil, errors.New("wechat unavailable")),
		store.EXPECT().ResolveRiderDepositRefundTx(gomock.Any(), db.ResolveRiderDepositRefundTxParams{
			RefundOrderID: refundOrder.ID,
			RefundStatus:  riderDepositRefundStatusFailed,
			RefundID:      "",
		}).Return(db.ResolveRiderDepositRefundTxResult{}, nil),
	)

	result, err := service.SubmitWithdrawal(context.Background(), SubmitRiderDepositWithdrawalInput{
		UserID: rider.UserID,
		Amount: 30000,
		Remark: "押金提现失败补偿",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "wechat unavailable")
	require.Equal(t, int64(30000), result.RequestedAmount)
	require.Equal(t, int64(0), result.AcceptedAmount)
	require.Equal(t, riderDepositWithdrawStatusProcessing, result.Status)
	require.Empty(t, result.Refunds)
}

func TestRiderDepositRefundService_SubmitWithdrawal_ReturnsBusinessErrorWhenRefundBalanceNotEnough(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	service := NewRiderDepositRefundService(store, paymentClient)

	rider := db.Rider{
		ID:            24,
		UserID:        91,
		Status:        "active",
		DepositAmount: 30000,
		FrozenDeposit: 0,
	}
	paymentOrder := db.PaymentOrder{
		ID:         504,
		UserID:     rider.UserID,
		OutTradeNo: "OTN_NOT_ENOUGH_001",
		Amount:     30000,
	}
	refundOrder := db.RefundOrder{
		ID:           704,
		OutRefundNo:  "ORN_NOT_ENOUGH_001",
		RefundAmount: 30000,
	}
	plan := db.RiderDepositRefundPlan{
		RefundOrder:        refundOrder,
		SourcePaymentOrder: paymentOrder,
	}
	wxErr := &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: wechaterrorcodes.DirectPaymentCodeNotEnough, Message: "基本账户余额不足，请充值后重新发起"}

	gomock.InOrder(
		store.EXPECT().GetRiderByUserID(gomock.Any(), rider.UserID).Return(rider, nil),
		store.EXPECT().ListRiderActiveDeliveries(gomock.Any(), pgtype.Int8{Int64: rider.ID, Valid: true}).Return([]db.Delivery{}, nil),
		store.EXPECT().PrepareRiderDepositRefundTx(gomock.Any(), db.PrepareRiderDepositRefundTxParams{
			RiderID: rider.ID,
			Amount:  30000,
			Remark:  "押金提现",
		}).Return(db.PrepareRiderDepositRefundTxResult{
			Rider:        db.Rider{ID: rider.ID, UserID: rider.UserID, DepositAmount: 30000, FrozenDeposit: 30000},
			RefundPlans:  []db.RiderDepositRefundPlan{plan},
			FrozenAmount: 30000,
		}, nil),
		paymentClient.EXPECT().CreateRefund(gomock.Any(), &wechat.RefundRequest{
			OutTradeNo:   paymentOrder.OutTradeNo,
			OutRefundNo:  refundOrder.OutRefundNo,
			Reason:       "押金提现",
			RefundAmount: refundOrder.RefundAmount,
			TotalAmount:  paymentOrder.Amount,
		}).Return(nil, wxErr),
		store.EXPECT().ResolveRiderDepositRefundTx(gomock.Any(), db.ResolveRiderDepositRefundTxParams{
			RefundOrderID: refundOrder.ID,
			RefundStatus:  riderDepositRefundStatusFailed,
			RefundID:      "",
		}).Return(db.ResolveRiderDepositRefundTxResult{}, nil),
	)

	result, err := service.SubmitWithdrawal(context.Background(), SubmitRiderDepositWithdrawalInput{
		UserID: rider.UserID,
		Amount: 30000,
		Remark: "押金提现",
	})
	require.Error(t, err)
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.EqualError(t, reqErr.Err, "商户退款余额不足，暂时无法原路退款，请联系平台处理")
	require.Same(t, wxErr, LoggableError(err))
	require.Equal(t, int64(0), result.AcceptedAmount)
	require.Equal(t, riderDepositWithdrawStatusProcessing, result.Status)
	require.Empty(t, result.Refunds)

	var unwrapped *wechat.WechatPayError
	require.ErrorAs(t, err, &unwrapped)
	require.Same(t, wxErr, unwrapped)
}

func TestRiderDepositRefundService_SubmitWithdrawal_ApprovedRiderAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	service := NewRiderDepositRefundService(store, paymentClient)

	rider := db.Rider{
		ID:            23,
		UserID:        90,
		Status:        db.RiderStatusApproved,
		DepositAmount: 30000,
		FrozenDeposit: 0,
	}
	paymentOrder := db.PaymentOrder{
		ID:         503,
		UserID:     rider.UserID,
		OutTradeNo: "OTN_APPROVED_001",
		Amount:     30000,
	}
	refundOrder := db.RefundOrder{
		ID:           703,
		OutRefundNo:  "ORN_APPROVED_001",
		RefundAmount: 30000,
	}
	plan := db.RiderDepositRefundPlan{
		RefundOrder:        refundOrder,
		SourcePaymentOrder: paymentOrder,
	}

	gomock.InOrder(
		store.EXPECT().GetRiderByUserID(gomock.Any(), rider.UserID).Return(rider, nil),
		store.EXPECT().ListRiderActiveDeliveries(gomock.Any(), pgtype.Int8{Int64: rider.ID, Valid: true}).Return([]db.Delivery{}, nil),
		store.EXPECT().PrepareRiderDepositRefundTx(gomock.Any(), db.PrepareRiderDepositRefundTxParams{
			RiderID: rider.ID,
			Amount:  30000,
			Remark:  "approved rider withdrawal",
		}).Return(db.PrepareRiderDepositRefundTxResult{
			Rider:        db.Rider{ID: rider.ID, UserID: rider.UserID, DepositAmount: 30000, FrozenDeposit: 30000},
			RefundPlans:  []db.RiderDepositRefundPlan{plan},
			FrozenAmount: 30000,
		}, nil),
		paymentClient.EXPECT().CreateRefund(gomock.Any(), &wechat.RefundRequest{
			OutTradeNo:   paymentOrder.OutTradeNo,
			OutRefundNo:  refundOrder.OutRefundNo,
			Reason:       "approved rider withdrawal",
			RefundAmount: refundOrder.RefundAmount,
			TotalAmount:  paymentOrder.Amount,
		}).Return(&wechat.RefundResponse{
			RefundID:    "WX_REFUND_APPROVED_001",
			OutRefundNo: refundOrder.OutRefundNo,
			OutTradeNo:  paymentOrder.OutTradeNo,
			Status:      wechat.RefundStatusSuccess,
		}, nil),
		store.EXPECT().ResolveRiderDepositRefundTx(gomock.Any(), db.ResolveRiderDepositRefundTxParams{
			RefundOrderID: refundOrder.ID,
			RefundStatus:  riderDepositRefundStatusSuccess,
			RefundID:      "WX_REFUND_APPROVED_001",
		}).Return(db.ResolveRiderDepositRefundTxResult{}, nil),
		store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder.Amount, nil),
		store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil),
	)

	result, err := service.SubmitWithdrawal(context.Background(), SubmitRiderDepositWithdrawalInput{
		UserID: rider.UserID,
		Amount: 30000,
		Remark: "approved rider withdrawal",
	})
	require.NoError(t, err)
	require.Equal(t, riderDepositWithdrawStatusSuccess, result.Status)
	require.Equal(t, int64(30000), result.AcceptedAmount)
}

func TestRiderDepositRefundService_ResolveRefund_DeleteReceiverWhenBalanceZero(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewRiderDepositRefundService(store, nil, ecommerceClient)

	paymentOrder := db.PaymentOrder{
		ID:           801,
		UserID:       901,
		Amount:       30000,
		Status:       "paid",
		BusinessType: "rider_deposit",
	}
	rider := db.Rider{
		ID:            71,
		UserID:        paymentOrder.UserID,
		RealName:      "骑手甲",
		DepositAmount: 0,
		FrozenDeposit: 0,
	}
	user := db.User{ID: paymentOrder.UserID, WechatOpenid: "rider-openid-901"}

	gomock.InOrder(
		store.EXPECT().ResolveRiderDepositRefundTx(gomock.Any(), db.ResolveRiderDepositRefundTxParams{
			RefundOrderID: 701,
			RefundStatus:  riderDepositRefundStatusSuccess,
			RefundID:      "WX_REFUND_701",
		}).Return(db.ResolveRiderDepositRefundTxResult{}, nil),
		store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder.Amount, nil),
		store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil),
		store.EXPECT().GetRiderByUserID(gomock.Any(), paymentOrder.UserID).Return(rider, nil),
		store.EXPECT().GetUser(gomock.Any(), paymentOrder.UserID).Return(user, nil),
		ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123"),
		ecommerceClient.EXPECT().DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
			AppID:   "wx_sp_app_123",
			Type:    wechatcontracts.ReceiverTypePersonal,
			Account: user.WechatOpenid,
		}).Return(&wechatcontracts.DeleteReceiverResponse{Type: wechatcontracts.ReceiverTypePersonal, Account: user.WechatOpenid}, nil),
	)

	err := service.ResolveRefund(context.Background(), 701, paymentOrder, riderDepositRefundStatusSuccess, "WX_REFUND_701")
	require.NoError(t, err)
}
