package algorithm

import (
	"context"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMerchantForeignObjectTracker_CheckStatus_Normal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	tracker := NewMerchantForeignObjectTracker(store)

	merchantID := int64(100)

	// 只有1次异物索赔，不需要通知
	store.EXPECT().
		CountMerchantClaimsByType(gomock.Any(), gomock.Any()).
		Return(int64(1), nil)

	result, err := tracker.CheckMerchantForeignObjectStatus(context.Background(), merchantID)
	require.NoError(t, err)
	require.Equal(t, 1, result.ForeignObjectNum)
	require.False(t, result.ShouldNotify)
}

func TestMerchantForeignObjectTracker_CheckStatus_NeedNotify(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	tracker := NewMerchantForeignObjectTracker(store)

	merchantID := int64(100)

	// 3次异物索赔，达到通知阈值
	store.EXPECT().
		CountMerchantClaimsByType(gomock.Any(), gomock.Any()).
		Return(int64(MerchantForeignObjectWarningNum), nil)

	result, err := tracker.CheckMerchantForeignObjectStatus(context.Background(), merchantID)
	require.NoError(t, err)
	require.Equal(t, 3, result.ForeignObjectNum)
	require.True(t, result.ShouldNotify)
	require.Contains(t, result.Message, "异物索赔")
}

func TestMerchantForeignObjectTracker_CheckStatus_MoreThanThreshold(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	tracker := NewMerchantForeignObjectTracker(store)

	merchantID := int64(100)

	// 5次异物索赔，超过阈值也只是通知
	store.EXPECT().
		CountMerchantClaimsByType(gomock.Any(), gomock.Any()).
		Return(int64(5), nil)

	result, err := tracker.CheckMerchantForeignObjectStatus(context.Background(), merchantID)
	require.NoError(t, err)
	require.Equal(t, 5, result.ForeignObjectNum)
	require.True(t, result.ShouldNotify)
}

func TestMerchantForeignObjectTracker_GetHistory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	tracker := NewMerchantForeignObjectTracker(store)

	merchantID := int64(100)

	mockClaims := []db.Claim{
		{ID: 1, ClaimType: ClaimTypeForeignObject},
		{ID: 2, ClaimType: ClaimTypeForeignObject},
	}

	store.EXPECT().
		ListMerchantClaimsByTypeInPeriod(gomock.Any(), gomock.Any()).
		Return(mockClaims, nil)

	claims, err := tracker.GetMerchantForeignObjectHistory(context.Background(), merchantID, 7)
	require.NoError(t, err)
	require.Len(t, claims, 2)
}

func TestMerchantForeignObjectTracker_HandleClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	tracker := NewMerchantForeignObjectTracker(store)

	merchantID := int64(100)

	store.EXPECT().
		CountMerchantClaimsByType(gomock.Any(), gomock.Any()).
		Return(int64(MerchantForeignObjectWarningNum), nil)

	result, err := tracker.HandleForeignObjectClaim(context.Background(), merchantID, 1)
	require.NoError(t, err)
	require.True(t, result.ShouldNotify)
}
