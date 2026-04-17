package logic

import (
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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func claimInfoFor(merchantID, regionID int64, riderID *int64) db.GetClaimForAppealRow {
	row := db.GetClaimForAppealRow{
		MerchantID: merchantID,
		RegionID:   regionID,
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
					GetClaimForAppeal(gomock.Any(), claimID).
					Times(1).
					Return(db.GetClaimForAppealRow{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ db.ClaimRecovery, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "claim not found or not eligible for recovery", reqErr.Err.Error())
			},
		},
		{
			name: "Forbidden",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), claimID).
					Times(1).
					Return(claimInfoFor(merchantID+1, 99, nil), nil)
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
					GetClaimForAppeal(gomock.Any(), claimID).
					Times(1).
					Return(claimInfoFor(merchantID, 99, nil), nil)
				store.EXPECT().
					GetClaimRecoveryByClaimID(gomock.Any(), claimID).
					Times(1).
					Return(db.ClaimRecovery{}, db.ErrRecordNotFound)
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
					GetClaimForAppeal(gomock.Any(), claimID).
					Times(1).
					Return(claimInfoFor(merchantID, 99, nil), nil)
				store.EXPECT().
					GetClaimRecoveryByClaimID(gomock.Any(), claimID).
					Times(1).
					Return(recovery, nil)
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
				ClaimID:    claimID,
				MerchantID: merchantID,
			})
			tc.check(t, got, err)
		})
	}
}

func TestPayMerchantClaimRecoveryTargetMismatch(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(claimInfoFor(merchantID, 99, nil), nil)
	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claimID).
		Times(1).
		Return(db.ClaimRecovery{ID: 30, RecoveryTarget: pgtype.Text{String: "rider", Valid: true}}, nil)

	_, err := PayMerchantClaimRecovery(context.Background(), store, PayMerchantClaimRecoveryInput{
		ClaimID:    claimID,
		MerchantID: merchantID,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "recovery target mismatch", reqErr.Err.Error())
}

func TestPayMerchantClaimRecoverySuccess(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	recoveryID := int64(30)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	recovery := db.ClaimRecovery{ID: recoveryID, ClaimID: claimID, RecoveryTarget: pgtype.Text{String: "merchant", Valid: true}, RecoveryAmount: 500}
	updated := recovery
	updated.Status = "paid"

	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(claimInfoFor(merchantID, 99, nil), nil)
	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claimID).
		Times(1).
		Return(recovery, nil)
	store.EXPECT().
		MarkClaimRecoveryPaid(gomock.Any(), recoveryID).
		Times(1).
		Return(updated, nil)
	store.EXPECT().
		UnsuspendMerchantTakeout(gomock.Any(), merchantID).
		Times(1).
		Return(nil)

	got, err := PayMerchantClaimRecovery(context.Background(), store, PayMerchantClaimRecoveryInput{
		ClaimID:    claimID,
		MerchantID: merchantID,
	})

	require.NoError(t, err)
	require.Equal(t, updated.Status, got.Status)
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
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(claimInfoFor(merchantID, 99, nil), nil)
	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claimID).
		Times(1).
		Return(recovery, nil)
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
			require.Contains(t, arg.Attach.String, "\"claim_id\":10")
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

	result, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		ClaimID:     claimID,
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
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(claimInfoFor(merchantID, 99, nil), nil)
	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claimID).
		Times(1).
		Return(recovery, nil)
	store.EXPECT().
		GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.PaymentOrder{}, lookupErr)

	_, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		ClaimID:     claimID,
		MerchantID:  merchantID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "lookup claim recovery payment order")
	require.ErrorContains(t, err, "db unavailable")
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
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(claimInfoFor(merchantID, 99, nil), nil)
	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claimID).
		Times(1).
		Return(recovery, nil)
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
		ClaimID:     claimID,
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
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(claimInfoFor(merchantID, 99, nil), nil)
	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claimID).
		Times(1).
		Return(recovery, nil)
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

	result, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		ClaimID:     claimID,
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
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(claimInfoFor(merchantID, 99, nil), nil)
	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claimID).
		Times(1).
		Return(recovery, nil)
	store.EXPECT().
		GetLatestPaymentOrderByBusinessTypeAndAttach(gomock.Any(), gomock.Any()).
		Times(1).
		Return(existingPayment, nil)
	paymentClient.EXPECT().
		CloseOrder(gomock.Any(), existingPayment.OutTradeNo).
		Times(1).
		Return(errors.New("wechat close failed"))

	_, err := CreateMerchantClaimRecoveryPayment(context.Background(), store, paymentClient, CreateMerchantClaimRecoveryPaymentInput{
		ClaimID:     claimID,
		MerchantID:  merchantID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "close expired claim recovery wechat order")
}

func TestPayRiderClaimRecoverySuccess(t *testing.T) {
	claimID := int64(10)
	riderID := int64(20)
	recoveryID := int64(30)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	recovery := db.ClaimRecovery{ID: recoveryID, ClaimID: claimID, RecoveryTarget: pgtype.Text{String: "rider", Valid: true}, RecoveryAmount: 200}
	updated := recovery
	updated.Status = "paid"

	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(claimInfoFor(99, 88, &riderID), nil)
	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claimID).
		Times(1).
		Return(recovery, nil)
	store.EXPECT().
		MarkClaimRecoveryPaid(gomock.Any(), recoveryID).
		Times(1).
		Return(updated, nil)
	store.EXPECT().
		UnsuspendRider(gomock.Any(), riderID).
		Times(1).
		Return(nil)

	got, err := PayRiderClaimRecovery(context.Background(), store, PayRiderClaimRecoveryInput{
		ClaimID: claimID,
		RiderID: riderID,
	})

	require.NoError(t, err)
	require.Equal(t, updated.Status, got.Status)
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
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(claimInfoFor(99, 88, &riderID), nil)
	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claimID).
		Times(1).
		Return(recovery, nil)
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
		ClaimID:     claimID,
		RiderID:     riderID,
		PayerUserID: payerUserID,
		ClientIP:    "127.0.0.1",
	})

	require.NoError(t, err)
	require.Equal(t, existingPayment.ID, result.PaymentOrder.ID)
	require.NotNil(t, result.PayParams)
	require.Equal(t, "prepay_id=prepay_claim_recovery_existing", result.PayParams.Package)
}

func TestWaiveClaimRecoveryMerchantPaid(t *testing.T) {
	claimID := int64(10)
	regionID := int64(40)
	merchantID := int64(60)
	orderID := int64(70)
	recoveryID := int64(80)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	recovery := db.ClaimRecovery{ID: recoveryID, ClaimID: claimID, OrderID: orderID, RecoveryTarget: pgtype.Text{String: "merchant", Valid: true}, RecoveryAmount: 300, Status: "paid"}
	updated := recovery
	updated.Status = "waived"

	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(claimInfoFor(merchantID, regionID, nil), nil)
	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claimID).
		Times(1).
		Return(recovery, nil)
	store.EXPECT().
		MarkClaimRecoveryWaived(gomock.Any(), recoveryID).
		Times(1).
		Return(updated, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), orderID).
		Times(1).
		Return(db.Order{ID: orderID, MerchantID: merchantID}, nil)
	store.EXPECT().
		UnsuspendMerchantTakeout(gomock.Any(), merchantID).
		Times(1).
		Return(nil)

	got, err := WaiveClaimRecovery(context.Background(), store, WaiveClaimRecoveryInput{
		ClaimID:  claimID,
		RegionID: regionID,
	})

	require.NoError(t, err)
	require.Equal(t, updated.Status, got.Status)
}

func TestWaiveClaimRecoveryRider(t *testing.T) {
	claimID := int64(10)
	regionID := int64(40)
	orderID := int64(70)
	recoveryID := int64(80)
	riderID := int64(90)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	recovery := db.ClaimRecovery{ID: recoveryID, ClaimID: claimID, OrderID: orderID, RecoveryTarget: pgtype.Text{String: "rider", Valid: true}, RecoveryAmount: 300, Status: "pending"}
	updated := recovery
	updated.Status = "waived"

	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(claimInfoFor(99, regionID, &riderID), nil)
	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claimID).
		Times(1).
		Return(recovery, nil)
	store.EXPECT().
		MarkClaimRecoveryWaived(gomock.Any(), recoveryID).
		Times(1).
		Return(updated, nil)
	store.EXPECT().
		GetDeliveryByOrderID(gomock.Any(), orderID).
		Times(1).
		Return(db.Delivery{OrderID: orderID, RiderID: pgtype.Int8{Int64: riderID, Valid: true}}, nil)
	store.EXPECT().
		UnsuspendRider(gomock.Any(), riderID).
		Times(1).
		Return(nil)

	got, err := WaiveClaimRecovery(context.Background(), store, WaiveClaimRecoveryInput{
		ClaimID:  claimID,
		RegionID: regionID,
	})

	require.NoError(t, err)
	require.Equal(t, updated.Status, got.Status)
}
