package logic

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func recoveryDisputeContextQuery(claimID int64, recoveryTarget string) db.GetClaimRecoveryContextByClaimIDAndTargetParams {
	return db.GetClaimRecoveryContextByClaimIDAndTargetParams{
		ClaimID:        claimID,
		RecoveryTarget: pgtype.Text{String: recoveryTarget, Valid: true},
	}
}

func recoveryDisputeContextForTest(claimID, orderID, merchantID, regionID int64, riderID *int64, createdAt time.Time) db.GetClaimRecoveryContextByClaimIDAndTargetRow {
	row := db.GetClaimRecoveryContextByClaimIDAndTargetRow{
		ClaimID:        claimID,
		OrderID:        orderID,
		MerchantID:     merchantID,
		RegionID:       regionID,
		ClaimCreatedAt: createdAt,
	}
	if riderID != nil {
		row.RiderID = pgtype.Int8{Int64: *riderID, Valid: true}
	}
	return row
}

func TestCreateMerchantRecoveryDisputeClaimNotFound(t *testing.T) {
	claimID := int64(10)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "merchant")).
		Times(1).
		Return(db.GetClaimRecoveryContextByClaimIDAndTargetRow{}, db.ErrRecordNotFound)

	_, err := CreateMerchantRecoveryDispute(context.Background(), store, CreateMerchantRecoveryDisputeInput{
		MerchantID:        1,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               time.Now(),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "claim not found or not eligible for recovery dispute", reqErr.Err.Error())
}

func TestCreateMerchantRecoveryDisputeForbidden(t *testing.T) {
	claimID := int64(10)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "merchant")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, 2, 0, nil, time.Now()), nil)

	_, err := CreateMerchantRecoveryDispute(context.Background(), store, CreateMerchantRecoveryDisputeInput{
		MerchantID:        1,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               time.Now(),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "this claim does not belong to your merchant", reqErr.Err.Error())
}

func TestCreateMerchantRecoveryDisputeWindowExpired(t *testing.T) {
	claimID := int64(10)
	now := time.Date(2026, 2, 12, 9, 0, 0, 0, time.UTC)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "merchant")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, 1, 0, nil, now.AddDate(0, 0, -8)), nil)

	_, err := CreateMerchantRecoveryDispute(context.Background(), store, CreateMerchantRecoveryDisputeInput{
		MerchantID:        1,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               now,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "申诉窗口期已过（索赔后7天内可申诉）", reqErr.Err.Error())
}

func TestCreateMerchantRecoveryDisputeAlreadyExists(t *testing.T) {
	claimID := int64(10)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "merchant")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, 1, 0, nil, time.Now()), nil)
	store.EXPECT().
		CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{ClaimID: claimID, AppellantType: "merchant"}).
		Times(1).
		Return(true, nil)

	_, err := CreateMerchantRecoveryDispute(context.Background(), store, CreateMerchantRecoveryDisputeInput{
		MerchantID:        1,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               time.Now(),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 409, reqErr.Status)
	require.Equal(t, "recovery dispute already exists for this claim", reqErr.Err.Error())
}

func TestCreateMerchantRecoveryDisputeConcurrentDuplicateMapsToConflict(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	regionID := int64(30)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "merchant")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, merchantID, regionID, nil, time.Now()), nil)
	store.EXPECT().
		CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{ClaimID: claimID, AppellantType: "merchant"}).
		Times(1).
		Return(false, nil)
	store.EXPECT().
		CreateRecoveryDisputeWithRecoveryTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRecoveryDisputeWithRecoveryTxResult{}, db.ErrUniqueViolation)

	_, err := CreateMerchantRecoveryDispute(context.Background(), store, CreateMerchantRecoveryDisputeInput{
		MerchantID:        merchantID,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               time.Now(),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 409, reqErr.Status)
	require.Equal(t, "recovery dispute already exists for this claim", reqErr.Err.Error())
}

func TestCreateMerchantRecoveryDisputeSuccess(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	regionID := int64(30)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "merchant")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, merchantID, regionID, nil, time.Now()), nil)
	store.EXPECT().
		CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{ClaimID: claimID, AppellantType: "merchant"}).
		Times(1).
		Return(false, nil)
	store.EXPECT().
		CreateRecoveryDisputeWithRecoveryTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRecoveryDisputeWithRecoveryTxResult{RecoveryDispute: db.RecoveryDispute{ID: 55, ClaimID: claimID, AppellantType: "merchant", AppellantID: merchantID}}, nil)

	recoveryDispute, err := CreateMerchantRecoveryDispute(context.Background(), store, CreateMerchantRecoveryDisputeInput{
		MerchantID:        merchantID,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               time.Now(),
	})

	require.NoError(t, err)
	require.Equal(t, int64(55), recoveryDispute.ID)
}

func TestCreateMerchantRecoveryDisputeUsesRecoveryContext(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	regionID := int64(30)
	createdAt := time.Now()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "merchant")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, merchantID, regionID, nil, createdAt), nil)
	store.EXPECT().
		CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{ClaimID: claimID, AppellantType: "merchant"}).
		Times(1).
		Return(false, nil)
	store.EXPECT().
		CreateRecoveryDisputeWithRecoveryTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRecoveryDisputeWithRecoveryTxResult{RecoveryDispute: db.RecoveryDispute{ID: 56, ClaimID: claimID, AppellantType: "merchant", AppellantID: merchantID}}, nil)

	recoveryDispute, err := CreateMerchantRecoveryDispute(context.Background(), store, CreateMerchantRecoveryDisputeInput{
		MerchantID:        merchantID,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               createdAt,
	})

	require.NoError(t, err)
	require.Equal(t, int64(56), recoveryDispute.ID)
}

func TestCreateMerchantRecoveryDisputeTransitionFailure(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	regionID := int64(30)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "merchant")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, merchantID, regionID, nil, time.Now()), nil)
	store.EXPECT().
		CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{ClaimID: claimID, AppellantType: "merchant"}).
		Times(1).
		Return(false, nil)
	store.EXPECT().
		CreateRecoveryDisputeWithRecoveryTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRecoveryDisputeWithRecoveryTxResult{}, fmt.Errorf("mark claim recovery disputed: update failed"))

	_, err := CreateMerchantRecoveryDispute(context.Background(), store, CreateMerchantRecoveryDisputeInput{
		MerchantID:        merchantID,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               time.Now(),
	})

	require.EqualError(t, err, "mark claim recovery disputed: update failed")
}

func TestCreateRiderRecoveryDisputeClaimNotRelated(t *testing.T) {
	claimID := int64(10)
	relatedRiderID := int64(99)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "rider")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, 0, 0, &relatedRiderID, time.Now()), nil)

	_, err := CreateRiderRecoveryDispute(context.Background(), store, CreateRiderRecoveryDisputeInput{
		RiderID:           1,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               time.Now(),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "this claim is not related to your deliveries", reqErr.Err.Error())
}

func TestCreateRiderRecoveryDisputeAlreadyExistsSameRider(t *testing.T) {
	claimID := int64(10)
	riderID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "rider")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, 0, 0, &riderID, time.Now()), nil)
	store.EXPECT().
		CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(true, nil)
	store.EXPECT().
		GetRecoveryDisputeByClaim(gomock.Any(), db.GetRecoveryDisputeByClaimParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(db.RecoveryDispute{ID: 66, ClaimID: claimID, AppellantType: "rider", AppellantID: riderID}, nil)

	result, err := CreateRiderRecoveryDispute(context.Background(), store, CreateRiderRecoveryDisputeInput{
		RiderID:           riderID,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               time.Now(),
	})

	require.NoError(t, err)
	require.True(t, result.AlreadyExists)
	require.Equal(t, int64(66), result.RecoveryDispute.ID)
}

func TestCreateRiderRecoveryDisputeConcurrentDuplicateReturnsExistingSameRider(t *testing.T) {
	claimID := int64(10)
	riderID := int64(20)
	regionID := int64(30)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "rider")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, 0, regionID, &riderID, time.Now()), nil)
	store.EXPECT().
		CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(false, nil)
	store.EXPECT().
		CreateRecoveryDisputeWithRecoveryTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRecoveryDisputeWithRecoveryTxResult{}, db.ErrUniqueViolation)
	store.EXPECT().
		GetRecoveryDisputeByClaim(gomock.Any(), db.GetRecoveryDisputeByClaimParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(db.RecoveryDispute{ID: 66, ClaimID: claimID, AppellantType: "rider", AppellantID: riderID}, nil)

	result, err := CreateRiderRecoveryDispute(context.Background(), store, CreateRiderRecoveryDisputeInput{
		RiderID:           riderID,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               time.Now(),
	})

	require.NoError(t, err)
	require.True(t, result.AlreadyExists)
	require.Equal(t, int64(66), result.RecoveryDispute.ID)
}

func TestCreateRiderRecoveryDisputeAlreadyExistsOtherRider(t *testing.T) {
	claimID := int64(10)
	riderID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "rider")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, 0, 0, &riderID, time.Now()), nil)
	store.EXPECT().
		CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(true, nil)
	store.EXPECT().
		GetRecoveryDisputeByClaim(gomock.Any(), db.GetRecoveryDisputeByClaimParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(db.RecoveryDispute{ID: 66, ClaimID: claimID, AppellantType: "rider", AppellantID: riderID + 1}, nil)

	_, err := CreateRiderRecoveryDispute(context.Background(), store, CreateRiderRecoveryDisputeInput{
		RiderID:           riderID,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               time.Now(),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 409, reqErr.Status)
	require.Equal(t, "recovery dispute already exists for this claim", reqErr.Err.Error())
}

func TestCreateRiderRecoveryDisputeSuccess(t *testing.T) {
	claimID := int64(10)
	riderID := int64(20)
	regionID := int64(30)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), recoveryDisputeContextQuery(claimID, "rider")).
		Times(1).
		Return(recoveryDisputeContextForTest(claimID, 99, 0, regionID, &riderID, time.Now()), nil)
	store.EXPECT().
		CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(false, nil)
	store.EXPECT().
		CreateRecoveryDisputeWithRecoveryTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRecoveryDisputeWithRecoveryTxResult{RecoveryDispute: db.RecoveryDispute{ID: 77, ClaimID: claimID, AppellantType: "rider", AppellantID: riderID}}, nil)

	result, err := CreateRiderRecoveryDispute(context.Background(), store, CreateRiderRecoveryDisputeInput{
		RiderID:           riderID,
		ClaimID:           claimID,
		Reason:            "test reason",
		DisputeWindowDays: 7,
		Now:               time.Now(),
	})

	require.NoError(t, err)
	require.False(t, result.AlreadyExists)
	require.Equal(t, int64(77), result.RecoveryDispute.ID)
}
