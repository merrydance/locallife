package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
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
	now := time.Date(2026, 2, 12, 9, 0, 0, 0, time.UTC)

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
		GetMerchantSettlementAdjustmentByRelatedAndType(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.MerchantSettlementAdjustment{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateMerchantSettlementAdjustment(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateMerchantSettlementAdjustmentParams) (db.MerchantSettlementAdjustment, error) {
			require.Equal(t, merchantID, arg.MerchantID)
			require.Equal(t, int64(-500), arg.Amount)
			require.True(t, arg.PostedAt.Valid)
			require.Equal(t, now, arg.PostedAt.Time)
			return db.MerchantSettlementAdjustment{}, nil
		})
	store.EXPECT().
		UnsuspendMerchantTakeout(gomock.Any(), merchantID).
		Times(1).
		Return(nil)

	got, err := PayMerchantClaimRecovery(context.Background(), store, PayMerchantClaimRecoveryInput{
		ClaimID:    claimID,
		MerchantID: merchantID,
		Now:        now,
	})

	require.NoError(t, err)
	require.Equal(t, updated.Status, got.Status)
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

func TestWaiveClaimRecoveryMerchantPaid(t *testing.T) {
	claimID := int64(10)
	regionID := int64(40)
	merchantID := int64(60)
	orderID := int64(70)
	recoveryID := int64(80)
	now := time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC)

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
		GetMerchantSettlementAdjustmentByRelatedAndType(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.MerchantSettlementAdjustment{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateMerchantSettlementAdjustment(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateMerchantSettlementAdjustmentParams) (db.MerchantSettlementAdjustment, error) {
			require.Equal(t, merchantID, arg.MerchantID)
			require.Equal(t, int64(300), arg.Amount)
			require.True(t, arg.PostedAt.Valid)
			require.Equal(t, now, arg.PostedAt.Time)
			return db.MerchantSettlementAdjustment{}, nil
		})
	store.EXPECT().
		UnsuspendMerchantTakeout(gomock.Any(), merchantID).
		Times(1).
		Return(nil)

	got, err := WaiveClaimRecovery(context.Background(), store, WaiveClaimRecoveryInput{
		ClaimID:  claimID,
		RegionID: regionID,
		Now:      now,
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
