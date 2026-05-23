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
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRiderDepositRefundService_SubmitWithdrawal_CreateRefundSuccessReturnsAccepted(t *testing.T) {
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
		store.EXPECT().GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), rider.UserID).Return(int64(0), nil),
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
		store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: "WX_REFUND_SUCCESS_001", Valid: true},
		}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil),
		store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: 8101}, nil),
	)

	result, err := service.SubmitWithdrawal(context.Background(), SubmitRiderDepositWithdrawalInput{
		UserID: rider.UserID,
		Amount: 30000,
		Remark: "押金提现",
	})
	require.NoError(t, err)
	require.Equal(t, int64(30000), result.RequestedAmount)
	require.Equal(t, int64(30000), result.AcceptedAmount)
	require.Equal(t, riderDepositWithdrawStatusProcessing, result.Status)
	require.Len(t, result.Refunds, 1)
	require.Equal(t, riderDepositWithdrawStatusProcessing, result.Refunds[0].Status)
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
		store.EXPECT().GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), rider.UserID).Return(int64(0), nil),
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
		store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: 8102}, nil),
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

func TestRiderDepositRefundService_SubmitWithdrawal_AlreadyFullyRefundedErrorReconcilesStaleCredit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	service := NewRiderDepositRefundService(store, paymentClient)

	rider := db.Rider{
		ID:            122,
		UserID:        189,
		Status:        "active",
		DepositAmount: 30000,
		FrozenDeposit: 0,
	}
	paymentOrder := db.PaymentOrder{
		ID:         1502,
		UserID:     rider.UserID,
		OutTradeNo: "OTN_STALE_001",
		Amount:     30000,
	}
	refundOrder := db.RefundOrder{
		ID:           1702,
		OutRefundNo:  "ORN_STALE_001",
		RefundAmount: 10000,
	}
	plan := db.RiderDepositRefundPlan{
		RefundOrder:        refundOrder,
		SourcePaymentOrder: paymentOrder,
	}
	wxErr := &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: wechaterrorcodes.DirectPaymentCodeInvalidRequest, Message: "订单已全额退款"}

	gomock.InOrder(
		store.EXPECT().GetRiderByUserID(gomock.Any(), rider.UserID).Return(rider, nil),
		store.EXPECT().GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), rider.UserID).Return(int64(0), nil),
		store.EXPECT().ListRiderActiveDeliveries(gomock.Any(), pgtype.Int8{Int64: rider.ID, Valid: true}).Return([]db.Delivery{}, nil),
		store.EXPECT().PrepareRiderDepositRefundTx(gomock.Any(), db.PrepareRiderDepositRefundTxParams{
			RiderID: rider.ID,
			Amount:  10000,
			Remark:  "押金提现",
		}).Return(db.PrepareRiderDepositRefundTxResult{
			Rider:        db.Rider{ID: rider.ID, UserID: rider.UserID, DepositAmount: 30000, FrozenDeposit: 10000},
			RefundPlans:  []db.RiderDepositRefundPlan{plan},
			FrozenAmount: 10000,
		}, nil),
		paymentClient.EXPECT().CreateRefund(gomock.Any(), &wechat.RefundRequest{
			OutTradeNo:   paymentOrder.OutTradeNo,
			OutRefundNo:  refundOrder.OutRefundNo,
			Reason:       "押金提现",
			RefundAmount: refundOrder.RefundAmount,
			TotalAmount:  paymentOrder.Amount,
		}).Return(nil, wxErr),
		store.EXPECT().ResolveRiderDepositRefundTx(gomock.Any(), db.ResolveRiderDepositRefundTxParams{
			RefundOrderID:        refundOrder.ID,
			RefundStatus:         riderDepositRefundStatusSuccess,
			RefundID:             "",
			DrainRemainingCredit: true,
		}).Return(db.ResolveRiderDepositRefundTxResult{ReconciledAmount: 20000}, nil),
		store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil),
		store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: 8103}, nil),
	)

	result, err := service.SubmitWithdrawal(context.Background(), SubmitRiderDepositWithdrawalInput{
		UserID: rider.UserID,
		Amount: 10000,
		Remark: "押金提现",
	})
	require.NoError(t, err)
	require.Equal(t, int64(10000), result.RequestedAmount)
	require.Equal(t, int64(10000), result.AcceptedAmount)
	require.Equal(t, riderDepositWithdrawStatusSuccess, result.Status)
	require.Len(t, result.Refunds, 1)
	require.Equal(t, riderDepositWithdrawStatusSuccess, result.Refunds[0].Status)
	require.Equal(t, refundOrder.ID, result.Refunds[0].RefundOrder.ID)
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
		store.EXPECT().GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), rider.UserID).Return(int64(0), nil),
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
		store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: 8104}, nil),
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

func TestRiderDepositRefundService_SubmitWithdrawal_PendingRefundReducesAvailableBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	service := NewRiderDepositRefundService(store, paymentClient)

	rider := db.Rider{
		ID:            26,
		UserID:        93,
		Status:        db.RiderStatusActive,
		DepositAmount: 30000,
		FrozenDeposit: 0,
	}

	gomock.InOrder(
		store.EXPECT().GetRiderByUserID(gomock.Any(), rider.UserID).Return(rider, nil),
		store.EXPECT().GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), rider.UserID).Return(int64(25000), nil),
	)

	_, err := service.SubmitWithdrawal(context.Background(), SubmitRiderDepositWithdrawalInput{
		UserID: rider.UserID,
		Amount: 10000,
		Remark: "押金提现",
	})
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadRequest, reqErr.Status)
	require.ErrorIs(t, reqErr.Err, ErrRiderAvailableDepositInsufficient)
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
		store.EXPECT().GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), rider.UserID).Return(int64(0), nil),
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
		store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: "WX_REFUND_APPROVED_001", Valid: true},
		}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil),
		store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: 8105}, nil),
	)

	result, err := service.SubmitWithdrawal(context.Background(), SubmitRiderDepositWithdrawalInput{
		UserID: rider.UserID,
		Amount: 30000,
		Remark: "approved rider withdrawal",
	})
	require.NoError(t, err)
	require.Equal(t, riderDepositWithdrawStatusProcessing, result.Status)
	require.Equal(t, int64(30000), result.AcceptedAmount)
}

func TestRiderDepositRefundService_SubmitWithdrawal_CreateRefundTerminalResponseReturnsAccepted(t *testing.T) {
	for _, status := range []string{wechat.RefundStatusClosed, wechat.RefundStatusAbnormal} {
		t.Run(status, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
			service := NewRiderDepositRefundService(store, paymentClient)

			rider := db.Rider{
				ID:            25,
				UserID:        92,
				Status:        db.RiderStatusActive,
				DepositAmount: 30000,
				FrozenDeposit: 0,
			}
			paymentOrder := db.PaymentOrder{
				ID:         505,
				UserID:     rider.UserID,
				OutTradeNo: "OTN_TERMINAL_CREATE_001",
				Amount:     30000,
			}
			refundOrder := db.RefundOrder{
				ID:           705,
				OutRefundNo:  "ORN_TERMINAL_CREATE_001",
				RefundAmount: 30000,
			}
			plan := db.RiderDepositRefundPlan{
				RefundOrder:        refundOrder,
				SourcePaymentOrder: paymentOrder,
			}

			gomock.InOrder(
				store.EXPECT().GetRiderByUserID(gomock.Any(), rider.UserID).Return(rider, nil),
				store.EXPECT().GetPendingRiderDepositRefundAmountByUserID(gomock.Any(), rider.UserID).Return(int64(0), nil),
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
					RefundID:    "WX_REFUND_TERMINAL_CREATE_001",
					OutRefundNo: refundOrder.OutRefundNo,
					OutTradeNo:  paymentOrder.OutTradeNo,
					Status:      status,
				}, nil),
				store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
					ID:       refundOrder.ID,
					RefundID: pgtype.Text{String: "WX_REFUND_TERMINAL_CREATE_001", Valid: true},
				}).Return(db.RefundOrder{ID: refundOrder.ID, Status: "processing"}, nil),
				store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: 8106}, nil),
			)

			result, err := service.SubmitWithdrawal(context.Background(), SubmitRiderDepositWithdrawalInput{
				UserID: rider.UserID,
				Amount: 30000,
				Remark: "押金提现",
			})
			require.NoError(t, err)
			require.Equal(t, int64(30000), result.AcceptedAmount)
			require.Equal(t, riderDepositWithdrawStatusProcessing, result.Status)
			require.Len(t, result.Refunds, 1)
			require.Equal(t, riderDepositWithdrawStatusProcessing, result.Refunds[0].Status)
		})
	}
}

func TestRiderDepositRefundService_ResolveRefund_DoesNotWriteReceiverAbsentTargetWhenBalanceZero(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewRiderDepositRefundService(store, nil)

	paymentOrder := db.PaymentOrder{
		ID:           801,
		UserID:       901,
		Amount:       30000,
		Status:       "paid",
		BusinessType: "rider_deposit",
	}
	gomock.InOrder(
		store.EXPECT().ResolveRiderDepositRefundTx(gomock.Any(), db.ResolveRiderDepositRefundTxParams{
			RefundOrderID: 701,
			RefundStatus:  riderDepositRefundStatusSuccess,
			RefundID:      "WX_REFUND_701",
		}).Return(db.ResolveRiderDepositRefundTxResult{}, nil),
		store.EXPECT().GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder.Amount, nil),
		store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil),
	)

	err := service.ResolveRefund(context.Background(), 701, paymentOrder, riderDepositRefundStatusSuccess, "WX_REFUND_701")
	require.NoError(t, err)
}
