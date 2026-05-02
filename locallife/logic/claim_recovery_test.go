package logic

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func claimRecoveryContextFor(recovery db.ClaimRecovery, merchantID, regionID int64, riderID *int64) db.GetClaimRecoveryContextByIDRow {
	row := db.GetClaimRecoveryContextByIDRow{
		ID:               recovery.ID,
		ClaimID:          recovery.ClaimID,
		OrderID:          recovery.OrderID,
		ResponsibleParty: recovery.ResponsibleParty,
		RecoveryTarget:   recovery.RecoveryTarget,
		RecoveryAmount:   recovery.RecoveryAmount,
		Status:           recovery.Status,
		DueAt:            recovery.DueAt,
		DecisionSnapshot: recovery.DecisionSnapshot,
		CreatedAt:        recovery.CreatedAt,
		UpdatedAt:        recovery.UpdatedAt,
		DecisionID:       recovery.DecisionID,
		RecoveryBasis:    recovery.RecoveryBasis,
		MerchantID:       merchantID,
		RegionID:         regionID,
		PaidAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	if riderID != nil {
		row.RiderID = pgtype.Int8{Int64: *riderID, Valid: true}
	}
	return row
}

func TestGetClaimRecoveryForMerchant(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	recovery := db.ClaimRecovery{ID: 30, ClaimID: claimID}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, got db.ClaimRecovery, err error)
	}{
		{
			name: "ClaimNotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaimRecoveryContextByID(gomock.Any(), claimID).
					Times(1).
					Return(db.GetClaimRecoveryContextByIDRow{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ db.ClaimRecovery, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "claim recovery not found", reqErr.Err.Error())
			},
		},
		{
			name: "Forbidden",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaimRecoveryContextByID(gomock.Any(), claimID).
					Times(1).
					Return(claimRecoveryContextFor(recovery, merchantID+1, 99, nil), nil)
			},
			check: func(t *testing.T, _ db.ClaimRecovery, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "this claim does not belong to your merchant", reqErr.Err.Error())
			},
		},
		{
			name: "RecoveryNotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaimRecoveryContextByID(gomock.Any(), claimID).
					Times(1).
					Return(db.GetClaimRecoveryContextByIDRow{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ db.ClaimRecovery, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "claim recovery not found", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaimRecoveryContextByID(gomock.Any(), claimID).
					Times(1).
					Return(claimRecoveryContextFor(recovery, merchantID, 99, nil), nil)
			},
			check: func(t *testing.T, got db.ClaimRecovery, err error) {
				require.NoError(t, err)
				require.Equal(t, recovery.ID, got.ID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			if tc.buildStubs != nil {
				tc.buildStubs(store)
			}

			got, err := GetClaimRecoveryForMerchant(context.Background(), store, MerchantClaimRecoveryInput{
				RecoveryID: claimID,
				MerchantID: merchantID,
			})
			tc.check(t, got, err)
		})
	}
}

func TestCreateMerchantClaimRecoveryPaymentSuccess(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	payerUserID := int64(21)
	recoveryID := int64(30)
	orderID := int64(40)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	recovery := db.ClaimRecovery{
		ID:             recoveryID,
		ClaimID:        claimID,
		OrderID:        orderID,
		RecoveryAmount: 500,
		Status:         "pending",
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	createdPayment := db.PaymentOrder{
		ID:           100,
		UserID:       payerUserID,
		Amount:       recovery.RecoveryAmount,
		BusinessType: businessTypeClaimRecovery,
		Status:       "pending",
		OutTradeNo:   "CR_test_001",
	}
	updatedPayment := createdPayment
	updatedPayment.PrepayID = pgtype.Text{String: "prepay_claim_recovery_001", Valid: true}
	updatedPayment.ExpiresAt = pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true}

	store.EXPECT().
		GetClaimRecoveryContextByID(gomock.Any(), claimID).
		Times(1).
		Return(claimRecoveryContextFor(recovery, merchantID, 99, nil), nil)
	store.EXPECT().
		GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetUser(gomock.Any(), payerUserID).
		Times(1).
		Return(db.User{ID: payerUserID, WechatOpenid: "openid_merchant_payer"}, nil)
	store.EXPECT().
		CreatePaymentOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(ctx context.Context, arg db.CreatePaymentOrderParams) (db.PaymentOrder, error) {
			require.Equal(t, orderID, arg.OrderID.Int64)
			require.True(t, arg.OrderID.Valid)
			require.Equal(t, payerUserID, arg.UserID)
			require.Equal(t, businessTypeClaimRecovery, arg.BusinessType)
			require.Equal(t, recovery.RecoveryAmount, arg.Amount)
			require.True(t, arg.Attach.Valid)
			require.Contains(t, arg.Attach.String, "\"recovery_id\":30")
			createdPayment.OutTradeNo = arg.OutTradeNo
			return createdPayment, nil
		})
	paymentClient.EXPECT().
		CreateJSAPIOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(ctx context.Context, req *wechatcontracts.DirectJSAPIOrderRequest) (*wechatcontracts.DirectJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
			require.Equal(t, createdPayment.OutTradeNo, req.OutTradeNo)
			require.Equal(t, "商户索赔追偿支付", req.Description)
			require.Equal(t, recovery.RecoveryAmount, req.TotalAmount)
			require.Equal(t, "openid_merchant_payer", req.PayerOpenID)
			return &wechatcontracts.DirectJSAPIOrderResponse{PrepayID: "prepay_claim_recovery_001"}, &wechat.JSAPIPayParams{Package: "prepay_id=prepay_claim_recovery_001"}, nil
		})
	store.EXPECT().
		UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(ctx context.Context, arg db.UpdatePaymentOrderPrepayIdParams) (db.PaymentOrder, error) {
			require.Equal(t, createdPayment.ID, arg.ID)
			require.Equal(t, "prepay_claim_recovery_001", arg.PrepayID.String)
			return updatedPayment, nil
		})
	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentCapabilityDirectJSAPIPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentCommandTypeCreatePayment, arg.CommandType)
			require.Equal(t, db.ExternalPaymentBusinessOwnerClaimRecovery, arg.BusinessOwner)
			require.Equal(t, db.ExternalPaymentObjectPayment, arg.ExternalObjectType)
			require.Equal(t, createdPayment.OutTradeNo, arg.ExternalObjectKey)
			require.Equal(t, "prepay_claim_recovery_001", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
			require.Contains(t, string(arg.ResponseSnapshot), "prepay_claim_recovery_001")
			require.NotContains(t, string(arg.ResponseSnapshot), "paySign")
			return db.ExternalPaymentCommand{ID: 9101}, nil
		})
	store.EXPECT().
		CreateClaimRecoveryEvent(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateClaimRecoveryEventParams) (db.ClaimRecoveryEvent, error) {
			require.Equal(t, recovery.ID, arg.RecoveryID)
			require.Equal(t, db.ClaimRecoveryEventTypePaymentStarted, arg.EventType)
			return db.ClaimRecoveryEvent{ID: 1, RecoveryID: recovery.ID, EventType: arg.EventType}, nil
		})

	result, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		RecoveryID:  claimID,
		MerchantID:  merchantID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	require.NoError(t, err)
	require.Equal(t, recovery.ID, result.Recovery.ID)
	require.Equal(t, updatedPayment.ID, result.PaymentOrder.ID)
	require.NotNil(t, result.PayParams)
	require.Equal(t, "prepay_id=prepay_claim_recovery_001", result.PayParams.Package)
}

func TestCreateMerchantClaimRecoveryPaymentLogsCleanupFailureAfterPrepayUpdateError(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	payerUserID := int64(21)
	recovery := db.ClaimRecovery{
		ID:             30,
		ClaimID:        claimID,
		OrderID:        40,
		RecoveryAmount: 500,
		Status:         "pending",
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	createdPayment := db.PaymentOrder{
		ID:           100,
		UserID:       payerUserID,
		Amount:       recovery.RecoveryAmount,
		BusinessType: businessTypeClaimRecovery,
		Status:       "pending",
		OutTradeNo:   "CR_test_cleanup",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	store.EXPECT().GetClaimRecoveryContextByID(gomock.Any(), claimID).Return(claimRecoveryContextFor(recovery, merchantID, 99, nil), nil)
	store.EXPECT().GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetUser(gomock.Any(), payerUserID).Return(db.User{ID: payerUserID, WechatOpenid: "openid_merchant_payer"}, nil)
	store.EXPECT().CreatePaymentOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePaymentOrderParams) (db.PaymentOrder, error) {
		createdPayment.OutTradeNo = arg.OutTradeNo
		return createdPayment, nil
	})
	paymentClient.EXPECT().CreateJSAPIOrder(gomock.Any(), gomock.Any()).Return(&wechatcontracts.DirectJSAPIOrderResponse{PrepayID: "prepay_claim_recovery_cleanup"}, &wechat.JSAPIPayParams{Package: "prepay_id=prepay_claim_recovery_cleanup"}, nil)
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       createdPayment.ID,
		PrepayID: pgtype.Text{String: "prepay_claim_recovery_cleanup", Valid: true},
	}).Return(db.PaymentOrder{}, errors.New("update prepay failed"))
	store.EXPECT().UpdatePaymentOrderToFailed(gomock.Any(), createdPayment.ID).Return(db.PaymentOrder{}, errors.New("mark claim recovery failed"))
	paymentClient.EXPECT().CloseOrder(gomock.Any(), gomock.Any()).Return(nil)

	_, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		RecoveryID:  claimID,
		MerchantID:  merchantID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "update claim recovery prepay id")
	require.Contains(t, logs.String(), "failed to mark claim recovery payment order failed after prepay update failure")
	require.Contains(t, logs.String(), "mark claim recovery failed")
}

func TestCreateMerchantClaimRecoveryPaymentWechatRejectedRecordsCommand(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	payerUserID := int64(21)
	recovery := db.ClaimRecovery{
		ID:             30,
		ClaimID:        claimID,
		OrderID:        40,
		RecoveryAmount: 500,
		Status:         "pending",
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	createdPayment := db.PaymentOrder{
		ID:           101,
		UserID:       payerUserID,
		Amount:       recovery.RecoveryAmount,
		BusinessType: businessTypeClaimRecovery,
		Status:       "pending",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	store.EXPECT().
		GetClaimRecoveryContextByID(gomock.Any(), claimID).
		Times(1).
		Return(claimRecoveryContextFor(recovery, merchantID, 99, nil), nil)
	store.EXPECT().
		GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetUser(gomock.Any(), payerUserID).
		Times(1).
		Return(db.User{ID: payerUserID, WechatOpenid: "openid_merchant_payer"}, nil)
	store.EXPECT().
		CreatePaymentOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreatePaymentOrderParams) (db.PaymentOrder, error) {
			createdPayment.OutTradeNo = arg.OutTradeNo
			return createdPayment, nil
		})
	paymentClient.EXPECT().
		CreateJSAPIOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil, nil, &wechat.WechatPayError{StatusCode: 503, Code: "SYSTEM_ERROR", Message: "system busy"})
	store.EXPECT().
		UpdatePaymentOrderToClosed(gomock.Any(), createdPayment.ID).
		Times(1).
		Return(db.PaymentOrder{ID: createdPayment.ID, Status: "closed"}, nil)
	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentCapabilityDirectJSAPIPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentCommandTypeCreatePayment, arg.CommandType)
			require.Equal(t, db.ExternalPaymentBusinessOwnerClaimRecovery, arg.BusinessOwner)
			require.Equal(t, db.ExternalPaymentCommandStatusRejected, arg.CommandStatus)
			require.Equal(t, createdPayment.OutTradeNo, arg.ExternalObjectKey)
			require.Equal(t, "SYSTEM_ERROR", arg.LastErrorCode.String)
			require.Contains(t, string(arg.ResponseSnapshot), "SYSTEM_ERROR")
			return db.ExternalPaymentCommand{ID: 9103}, nil
		})

	_, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		RecoveryID:  claimID,
		MerchantID:  merchantID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 503, reqErr.Status)
}

func TestCreateMerchantClaimRecoveryPaymentWechatRejectedSkipsCommandWhenCloseFails(t *testing.T) {
	claimID := int64(11)
	merchantID := int64(21)
	payerUserID := int64(22)
	recovery := db.ClaimRecovery{
		ID:             31,
		ClaimID:        claimID,
		OrderID:        41,
		RecoveryAmount: 600,
		Status:         "pending",
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	createdPayment := db.PaymentOrder{
		ID:           102,
		UserID:       payerUserID,
		Amount:       recovery.RecoveryAmount,
		BusinessType: businessTypeClaimRecovery,
		Status:       "pending",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	store.EXPECT().
		GetClaimRecoveryContextByID(gomock.Any(), claimID).
		Times(1).
		Return(claimRecoveryContextFor(recovery, merchantID, 99, nil), nil)
	store.EXPECT().
		GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetUser(gomock.Any(), payerUserID).
		Times(1).
		Return(db.User{ID: payerUserID, WechatOpenid: "openid_merchant_payer"}, nil)
	store.EXPECT().
		CreatePaymentOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreatePaymentOrderParams) (db.PaymentOrder, error) {
			createdPayment.OutTradeNo = arg.OutTradeNo
			return createdPayment, nil
		})
	paymentClient.EXPECT().
		CreateJSAPIOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil, nil, &wechat.WechatPayError{StatusCode: 503, Code: "SYSTEM_ERROR", Message: "system busy"})
	store.EXPECT().
		UpdatePaymentOrderToClosed(gomock.Any(), createdPayment.ID).
		Times(1).
		Return(db.PaymentOrder{}, errors.New("close unavailable"))

	_, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		RecoveryID:  claimID,
		MerchantID:  merchantID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 503, reqErr.Status)
}

func TestCreateMerchantClaimRecoveryPaymentLookupFailureReturnsError(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	payerUserID := int64(21)
	recovery := db.ClaimRecovery{
		ID:             30,
		ClaimID:        claimID,
		OrderID:        40,
		RecoveryAmount: 500,
		Status:         "pending",
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	lookupErr := errors.New("db unavailable")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	store.EXPECT().
		GetClaimRecoveryContextByID(gomock.Any(), claimID).
		Times(1).
		Return(claimRecoveryContextFor(recovery, merchantID, 99, nil), nil)
	store.EXPECT().
		GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.PaymentOrder{}, lookupErr)

	_, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		RecoveryID:  claimID,
		MerchantID:  merchantID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "lookup claim recovery payment order")
	require.ErrorContains(t, err, "db unavailable")
}

func TestCreateMerchantClaimRecoveryPaymentRequiresPlatformPayoutComplete(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	payerUserID := int64(21)
	recovery := db.ClaimRecovery{
		ID:             30,
		ClaimID:        claimID,
		OrderID:        40,
		RecoveryAmount: 500,
		Status:         "pending",
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	recoveryCtx := claimRecoveryContextFor(recovery, merchantID, 99, nil)
	recoveryCtx.PaidAt = pgtype.Timestamptz{}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	store.EXPECT().
		GetClaimRecoveryContextByID(gomock.Any(), claimID).
		Times(1).
		Return(recoveryCtx, nil)

	_, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		RecoveryID:  claimID,
		MerchantID:  merchantID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "claim recovery cannot be paid before platform payout completes", reqErr.Err.Error())
}

func TestCreateMerchantClaimRecoveryPaymentCreateUniqueViolationReusesExisting(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	payerUserID := int64(21)
	recoveryID := int64(30)
	recovery := db.ClaimRecovery{
		ID:             recoveryID,
		ClaimID:        claimID,
		OrderID:        40,
		RecoveryAmount: 500,
		Status:         "pending",
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	existingPayment := db.PaymentOrder{
		ID:           100,
		UserID:       payerUserID,
		Amount:       recovery.RecoveryAmount,
		BusinessType: businessTypeClaimRecovery,
		Status:       "pending",
		OutTradeNo:   "CR_test_existing",
		PrepayID:     pgtype.Text{String: "prepay_claim_recovery_existing", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	store.EXPECT().
		GetClaimRecoveryContextByID(gomock.Any(), claimID).
		Times(1).
		Return(claimRecoveryContextFor(recovery, merchantID, 99, nil), nil)
	gomock.InOrder(
		store.EXPECT().
			GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).
			Times(1).
			Return(db.PaymentOrder{}, db.ErrRecordNotFound),
		store.EXPECT().
			GetUser(gomock.Any(), payerUserID).
			Times(1).
			Return(db.User{ID: payerUserID, WechatOpenid: "openid_merchant_payer"}, nil),
		store.EXPECT().
			CreatePaymentOrder(gomock.Any(), gomock.Any()).
			Times(1).
			Return(db.PaymentOrder{}, db.ErrUniqueViolation),
		store.EXPECT().
			GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).
			Times(1).
			Return(existingPayment, nil),
	)
	paymentClient.EXPECT().
		GenerateJSAPIPayParams("prepay_claim_recovery_existing").
		Times(1).
		Return(&wechat.JSAPIPayParams{Package: "prepay_id=prepay_claim_recovery_existing"}, nil)

	result, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		RecoveryID:  claimID,
		MerchantID:  merchantID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	require.NoError(t, err)
	require.Equal(t, existingPayment.ID, result.PaymentOrder.ID)
	require.NotNil(t, result.PayParams)
	require.Equal(t, "prepay_id=prepay_claim_recovery_existing", result.PayParams.Package)
}

func TestCreateMerchantClaimRecoveryPaymentExpiredPendingCreatesFreshPayment(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	payerUserID := int64(21)
	recoveryID := int64(30)
	recovery := db.ClaimRecovery{
		ID:             recoveryID,
		ClaimID:        claimID,
		OrderID:        40,
		RecoveryAmount: 500,
		Status:         "pending",
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	existingPayment := db.PaymentOrder{
		ID:           100,
		UserID:       payerUserID,
		Amount:       recovery.RecoveryAmount,
		BusinessType: businessTypeClaimRecovery,
		Status:       "pending",
		OutTradeNo:   "CR_test_expired",
		PrepayID:     pgtype.Text{String: "prepay_expired", Valid: true},
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(-5 * time.Minute), Valid: true},
	}
	createdPayment := db.PaymentOrder{
		ID:           101,
		UserID:       payerUserID,
		Amount:       recovery.RecoveryAmount,
		BusinessType: businessTypeClaimRecovery,
		Status:       "pending",
		OutTradeNo:   "CR_test_fresh",
	}
	updatedPayment := createdPayment
	updatedPayment.PrepayID = pgtype.Text{String: "prepay_claim_recovery_fresh", Valid: true}
	updatedPayment.ExpiresAt = pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	store.EXPECT().
		GetClaimRecoveryContextByID(gomock.Any(), claimID).
		Times(1).
		Return(claimRecoveryContextFor(recovery, merchantID, 99, nil), nil)
	store.EXPECT().
		GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).
		Times(1).
		Return(existingPayment, nil)
	paymentClient.EXPECT().
		CloseOrder(gomock.Any(), existingPayment.OutTradeNo).
		Times(1).
		Return(nil)
	store.EXPECT().
		UpdatePaymentOrderToClosed(gomock.Any(), existingPayment.ID).
		Times(1).
		Return(db.PaymentOrder{ID: existingPayment.ID, Status: "closed"}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), payerUserID).
		Times(1).
		Return(db.User{ID: payerUserID, WechatOpenid: "openid_merchant_payer"}, nil)
	store.EXPECT().
		CreatePaymentOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(createdPayment, nil)
	paymentClient.EXPECT().
		CreateJSAPIOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&wechatcontracts.DirectJSAPIOrderResponse{PrepayID: "prepay_claim_recovery_fresh"}, &wechat.JSAPIPayParams{Package: "prepay_id=prepay_claim_recovery_fresh"}, nil)
	store.EXPECT().
		UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Any()).
		Times(1).
		Return(updatedPayment, nil)
	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentBusinessOwnerClaimRecovery, arg.BusinessOwner)
			require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
			require.Equal(t, createdPayment.OutTradeNo, arg.ExternalObjectKey)
			require.Equal(t, "prepay_claim_recovery_fresh", arg.ExternalSecondaryKey.String)
			return db.ExternalPaymentCommand{ID: 9102}, nil
		})
	store.EXPECT().
		CreateClaimRecoveryEvent(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateClaimRecoveryEventParams) (db.ClaimRecoveryEvent, error) {
			require.Equal(t, recovery.ID, arg.RecoveryID)
			require.Equal(t, db.ClaimRecoveryEventTypePaymentStarted, arg.EventType)
			return db.ClaimRecoveryEvent{ID: 2, RecoveryID: recovery.ID, EventType: arg.EventType}, nil
		})

	result, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		RecoveryID:  claimID,
		MerchantID:  merchantID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	require.NoError(t, err)
	require.Equal(t, createdPayment.ID, result.PaymentOrder.ID)
	require.Equal(t, updatedPayment.PrepayID.String, result.PaymentOrder.PrepayID.String)
	require.NotNil(t, result.PayParams)
	require.Equal(t, "prepay_id=prepay_claim_recovery_fresh", result.PayParams.Package)
}

func TestCreateMerchantClaimRecoveryPaymentExpiredPendingCloseWechatOrderFailureStopsRotation(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	payerUserID := int64(21)
	recoveryID := int64(30)
	recovery := db.ClaimRecovery{
		ID:             recoveryID,
		ClaimID:        claimID,
		OrderID:        40,
		RecoveryAmount: 500,
		Status:         "pending",
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	existingPayment := db.PaymentOrder{
		ID:           100,
		UserID:       payerUserID,
		Amount:       recovery.RecoveryAmount,
		BusinessType: businessTypeClaimRecovery,
		Status:       "pending",
		OutTradeNo:   "CR_test_expired",
		PrepayID:     pgtype.Text{String: "prepay_expired", Valid: true},
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(-5 * time.Minute), Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	store.EXPECT().
		GetClaimRecoveryContextByID(gomock.Any(), claimID).
		Times(1).
		Return(claimRecoveryContextFor(recovery, merchantID, 99, nil), nil)
	store.EXPECT().
		GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).
		Times(1).
		Return(existingPayment, nil)
	paymentClient.EXPECT().
		CloseOrder(gomock.Any(), existingPayment.OutTradeNo).
		Times(1).
		Return(errors.New("wechat close failed"))

	_, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		RecoveryID:  claimID,
		MerchantID:  merchantID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "close expired claim recovery wechat order")
}

func TestCreateRiderClaimRecoveryPaymentReusePending(t *testing.T) {
	claimID := int64(10)
	riderID := int64(20)
	payerUserID := int64(21)
	recoveryID := int64(30)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	recovery := db.ClaimRecovery{
		ID:             recoveryID,
		ClaimID:        claimID,
		OrderID:        40,
		RecoveryAmount: 800,
		Status:         "pending",
		RecoveryTarget: pgtype.Text{String: "rider", Valid: true},
	}
	existingPayment := db.PaymentOrder{
		ID:           101,
		UserID:       payerUserID,
		Amount:       recovery.RecoveryAmount,
		BusinessType: businessTypeClaimRecovery,
		Status:       "pending",
		OutTradeNo:   "CR_test_002",
		PrepayID:     pgtype.Text{String: "prepay_claim_recovery_existing", Valid: true},
	}

	store.EXPECT().
		GetClaimRecoveryContextByID(gomock.Any(), claimID).
		Times(1).
		Return(claimRecoveryContextFor(recovery, 99, 88, &riderID), nil)
	store.EXPECT().
		GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(ctx context.Context, arg db.GetLatestPaymentOrderByBusinessTypeAndAttachParams) (db.PaymentOrder, error) {
			require.Equal(t, businessTypeClaimRecovery, arg.BusinessType)
			require.True(t, arg.Attach.Valid)
			require.Contains(t, arg.Attach.String, "\"recovery_id\":30")
			return existingPayment, nil
		})
	paymentClient.EXPECT().
		GenerateJSAPIPayParams("prepay_claim_recovery_existing").
		Times(1).
		Return(&wechat.JSAPIPayParams{Package: "prepay_id=prepay_claim_recovery_existing"}, nil)

	result, err := CreateRiderClaimRecoveryPayment(context.Background(), store, paymentClient, CreateRiderClaimRecoveryPaymentInput{
		RecoveryID:  claimID,
		RiderID:     riderID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	require.NoError(t, err)
	require.Equal(t, existingPayment.ID, result.PaymentOrder.ID)
	require.NotNil(t, result.PayParams)
	require.Equal(t, "prepay_id=prepay_claim_recovery_existing", result.PayParams.Package)
}
