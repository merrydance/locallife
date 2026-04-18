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

func TestCreateMerchantAppealClaimNotFound(t *testing.T) {
	claimID := int64(10)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(db.GetClaimForAppealRow{}, db.ErrRecordNotFound)

	_, err := CreateMerchantAppeal(context.Background(), store, CreateMerchantAppealInput{
		MerchantID:       1,
		ClaimID:          claimID,
		Reason:           "test reason",
		AppealWindowDays: 7,
		Now:              time.Now(),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "claim not found or not eligible for appeal", reqErr.Err.Error())
}

func TestCreateMerchantAppealForbidden(t *testing.T) {
	claimID := int64(10)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(db.GetClaimForAppealRow{MerchantID: 2}, nil)

	_, err := CreateMerchantAppeal(context.Background(), store, CreateMerchantAppealInput{
		MerchantID:       1,
		ClaimID:          claimID,
		Reason:           "test reason",
		AppealWindowDays: 7,
		Now:              time.Now(),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "this claim does not belong to your merchant", reqErr.Err.Error())
}

func TestCreateMerchantAppealWindowExpired(t *testing.T) {
	claimID := int64(10)
	now := time.Date(2026, 2, 12, 9, 0, 0, 0, time.UTC)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(db.GetClaimForAppealRow{MerchantID: 1, CreatedAt: now.AddDate(0, 0, -8)}, nil)

	_, err := CreateMerchantAppeal(context.Background(), store, CreateMerchantAppealInput{
		MerchantID:       1,
		ClaimID:          claimID,
		Reason:           "test reason",
		AppealWindowDays: 7,
		Now:              now,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "申诉窗口期已过（索赔后7天内可申诉）", reqErr.Err.Error())
}

func TestCreateMerchantAppealAlreadyExists(t *testing.T) {
	claimID := int64(10)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(db.GetClaimForAppealRow{MerchantID: 1, CreatedAt: time.Now()}, nil)
	store.EXPECT().
		CheckAppealExists(gomock.Any(), db.CheckAppealExistsParams{ClaimID: claimID, AppellantType: "merchant"}).
		Times(1).
		Return(true, nil)

	_, err := CreateMerchantAppeal(context.Background(), store, CreateMerchantAppealInput{
		MerchantID:       1,
		ClaimID:          claimID,
		Reason:           "test reason",
		AppealWindowDays: 7,
		Now:              time.Now(),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 409, reqErr.Status)
	require.Equal(t, "appeal already exists for this claim", reqErr.Err.Error())
}

func TestCreateMerchantAppealSuccess(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	regionID := int64(30)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(db.GetClaimForAppealRow{MerchantID: merchantID, RegionID: regionID, CreatedAt: time.Now()}, nil)
	store.EXPECT().
		CheckAppealExists(gomock.Any(), db.CheckAppealExistsParams{ClaimID: claimID, AppellantType: "merchant"}).
		Times(1).
		Return(false, nil)
	store.EXPECT().
		CreateAppealWithRecoveryTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateAppealWithRecoveryTxResult{Appeal: db.Appeal{ID: 55, ClaimID: claimID, AppellantType: "merchant", AppellantID: merchantID}}, nil)

	appeal, err := CreateMerchantAppeal(context.Background(), store, CreateMerchantAppealInput{
		MerchantID:       merchantID,
		ClaimID:          claimID,
		Reason:           "test reason",
		AppealWindowDays: 7,
		Now:              time.Now(),
	})

	require.NoError(t, err)
	require.Equal(t, int64(55), appeal.ID)
}

func TestCreateMerchantAppealRecoveryTransitionFailure(t *testing.T) {
	claimID := int64(10)
	merchantID := int64(20)
	regionID := int64(30)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(db.GetClaimForAppealRow{MerchantID: merchantID, RegionID: regionID, CreatedAt: time.Now()}, nil)
	store.EXPECT().
		CheckAppealExists(gomock.Any(), db.CheckAppealExistsParams{ClaimID: claimID, AppellantType: "merchant"}).
		Times(1).
		Return(false, nil)
	store.EXPECT().
		CreateAppealWithRecoveryTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateAppealWithRecoveryTxResult{}, fmt.Errorf("mark claim recovery appealed: update failed"))

	_, err := CreateMerchantAppeal(context.Background(), store, CreateMerchantAppealInput{
		MerchantID:       merchantID,
		ClaimID:          claimID,
		Reason:           "test reason",
		AppealWindowDays: 7,
		Now:              time.Now(),
	})

	require.EqualError(t, err, "mark claim recovery appealed: update failed")
}

func TestCreateRiderAppealClaimNotRelated(t *testing.T) {
	claimID := int64(10)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(db.GetClaimForAppealRow{RiderID: pgtype.Int8{Int64: 99, Valid: true}}, nil)

	_, err := CreateRiderAppeal(context.Background(), store, CreateRiderAppealInput{
		RiderID:          1,
		ClaimID:          claimID,
		Reason:           "test reason",
		AppealWindowDays: 7,
		Now:              time.Now(),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "this claim is not related to your deliveries", reqErr.Err.Error())
}

func TestCreateRiderAppealAlreadyExistsSameRider(t *testing.T) {
	claimID := int64(10)
	riderID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(db.GetClaimForAppealRow{RiderID: pgtype.Int8{Int64: riderID, Valid: true}, CreatedAt: time.Now()}, nil)
	store.EXPECT().
		CheckAppealExists(gomock.Any(), db.CheckAppealExistsParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(true, nil)
	store.EXPECT().
		GetAppealByClaim(gomock.Any(), db.GetAppealByClaimParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(db.Appeal{ID: 66, ClaimID: claimID, AppellantType: "rider", AppellantID: riderID}, nil)

	result, err := CreateRiderAppeal(context.Background(), store, CreateRiderAppealInput{
		RiderID:          riderID,
		ClaimID:          claimID,
		Reason:           "test reason",
		AppealWindowDays: 7,
		Now:              time.Now(),
	})

	require.NoError(t, err)
	require.True(t, result.AlreadyExists)
	require.Equal(t, int64(66), result.Appeal.ID)
}

func TestCreateRiderAppealAlreadyExistsOtherRider(t *testing.T) {
	claimID := int64(10)
	riderID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(db.GetClaimForAppealRow{RiderID: pgtype.Int8{Int64: riderID, Valid: true}, CreatedAt: time.Now()}, nil)
	store.EXPECT().
		CheckAppealExists(gomock.Any(), db.CheckAppealExistsParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(true, nil)
	store.EXPECT().
		GetAppealByClaim(gomock.Any(), db.GetAppealByClaimParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(db.Appeal{ID: 66, ClaimID: claimID, AppellantType: "rider", AppellantID: riderID + 1}, nil)

	_, err := CreateRiderAppeal(context.Background(), store, CreateRiderAppealInput{
		RiderID:          riderID,
		ClaimID:          claimID,
		Reason:           "test reason",
		AppealWindowDays: 7,
		Now:              time.Now(),
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 409, reqErr.Status)
	require.Equal(t, "appeal already exists for this claim", reqErr.Err.Error())
}

func TestCreateRiderAppealSuccess(t *testing.T) {
	claimID := int64(10)
	riderID := int64(20)
	regionID := int64(30)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimForAppeal(gomock.Any(), claimID).
		Times(1).
		Return(db.GetClaimForAppealRow{RiderID: pgtype.Int8{Int64: riderID, Valid: true}, RegionID: regionID, CreatedAt: time.Now()}, nil)
	store.EXPECT().
		CheckAppealExists(gomock.Any(), db.CheckAppealExistsParams{ClaimID: claimID, AppellantType: "rider"}).
		Times(1).
		Return(false, nil)
	store.EXPECT().
		CreateAppealWithRecoveryTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateAppealWithRecoveryTxResult{Appeal: db.Appeal{ID: 77, ClaimID: claimID, AppellantType: "rider", AppellantID: riderID}}, nil)

	result, err := CreateRiderAppeal(context.Background(), store, CreateRiderAppealInput{
		RiderID:          riderID,
		ClaimID:          claimID,
		Reason:           "test reason",
		AppealWindowDays: 7,
		Now:              time.Now(),
	})

	require.NoError(t, err)
	require.False(t, result.AlreadyExists)
	require.Equal(t, int64(77), result.Appeal.ID)
}
