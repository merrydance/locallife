package logic

import (
	"context"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUpdateMembershipSettingsForOwnerPartialUpdatePreservesExistingFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const ownerUserID int64 = 10
	merchant := db.Merchant{ID: 20, OwnerUserID: ownerUserID}
	existing := db.MerchantMembershipSetting{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: []string{"takeaway"},
		BonusUsableScenes:   []string{},
		AllowWithVoucher:    false,
		AllowWithDiscount:   false,
		MaxDeductionPercent: 60,
	}
	nextPercent := int32(80)

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), ownerUserID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
		Times(1).
		Return(existing, nil)
	store.EXPECT().
		UpsertMerchantMembershipSettings(gomock.Any(), db.UpsertMerchantMembershipSettingsParams{
			MerchantID:          merchant.ID,
			BalanceUsableScenes: []string{"takeaway"},
			BonusUsableScenes:   []string{},
			AllowWithVoucher:    false,
			AllowWithDiscount:   false,
			MaxDeductionPercent: nextPercent,
		}).
		Times(1).
		Return(db.MerchantMembershipSetting{
			MerchantID:          merchant.ID,
			BalanceUsableScenes: []string{"takeaway"},
			BonusUsableScenes:   []string{},
			AllowWithVoucher:    false,
			AllowWithDiscount:   false,
			MaxDeductionPercent: nextPercent,
		}, nil)

	result, err := UpdateMembershipSettingsForOwner(context.Background(), store, UpdateMembershipSettingsInput{
		OwnerUserID:         ownerUserID,
		MaxDeductionPercent: &nextPercent,
	})

	require.NoError(t, err)
	require.Equal(t, []string{"takeaway"}, result.BalanceUsableScenes)
	require.Empty(t, result.BonusUsableScenes)
	require.False(t, result.AllowWithVoucher)
	require.False(t, result.AllowWithDiscount)
	require.Equal(t, nextPercent, result.MaxDeductionPercent)
}

func TestUpdateMembershipSettingsForOwnerExplicitEmptyScenesPreserved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const ownerUserID int64 = 10
	merchant := db.Merchant{ID: 20, OwnerUserID: ownerUserID}
	existing := db.MerchantMembershipSetting{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: []string{"dine_in", "takeaway"},
		BonusUsableScenes:   []string{"dine_in"},
		AllowWithVoucher:    true,
		AllowWithDiscount:   true,
		MaxDeductionPercent: 100,
	}

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), ownerUserID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
		Times(1).
		Return(existing, nil)
	store.EXPECT().
		UpsertMerchantMembershipSettings(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpsertMerchantMembershipSettingsParams) (db.MerchantMembershipSetting, error) {
			require.NotNil(t, arg.BalanceUsableScenes)
			require.Empty(t, arg.BalanceUsableScenes)
			require.NotNil(t, arg.BonusUsableScenes)
			require.Empty(t, arg.BonusUsableScenes)
			require.True(t, arg.AllowWithVoucher)
			require.True(t, arg.AllowWithDiscount)
			require.Equal(t, int32(100), arg.MaxDeductionPercent)
			return db.MerchantMembershipSetting{
				MerchantID:          arg.MerchantID,
				BalanceUsableScenes: arg.BalanceUsableScenes,
				BonusUsableScenes:   arg.BonusUsableScenes,
				AllowWithVoucher:    arg.AllowWithVoucher,
				AllowWithDiscount:   arg.AllowWithDiscount,
				MaxDeductionPercent: arg.MaxDeductionPercent,
			}, nil
		})

	result, err := UpdateMembershipSettingsForOwner(context.Background(), store, UpdateMembershipSettingsInput{
		OwnerUserID:         ownerUserID,
		BalanceUsableScenes: []string{},
		BonusUsableScenes:   []string{},
	})

	require.NoError(t, err)
	require.NotNil(t, result.BalanceUsableScenes)
	require.Empty(t, result.BalanceUsableScenes)
	require.NotNil(t, result.BonusUsableScenes)
	require.Empty(t, result.BonusUsableScenes)
	require.True(t, result.AllowWithVoucher)
	require.True(t, result.AllowWithDiscount)
	require.Equal(t, int32(100), result.MaxDeductionPercent)
}

func TestUpdateMembershipSettingsForOwnerMissingRowUsesDefaultsAsMergeBase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const ownerUserID int64 = 10
	merchant := db.Merchant{ID: 20, OwnerUserID: ownerUserID}
	nextPercent := int32(80)

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), ownerUserID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.MerchantMembershipSetting{}, db.ErrRecordNotFound)
	store.EXPECT().
		UpsertMerchantMembershipSettings(gomock.Any(), db.UpsertMerchantMembershipSettingsParams{
			MerchantID:          merchant.ID,
			BalanceUsableScenes: []string{"dine_in", "takeaway"},
			BonusUsableScenes:   []string{"dine_in"},
			AllowWithVoucher:    true,
			AllowWithDiscount:   true,
			MaxDeductionPercent: nextPercent,
		}).
		Times(1).
		Return(db.MerchantMembershipSetting{
			MerchantID:          merchant.ID,
			BalanceUsableScenes: []string{"dine_in", "takeaway"},
			BonusUsableScenes:   []string{"dine_in"},
			AllowWithVoucher:    true,
			AllowWithDiscount:   true,
			MaxDeductionPercent: nextPercent,
		}, nil)

	result, err := UpdateMembershipSettingsForOwner(context.Background(), store, UpdateMembershipSettingsInput{
		OwnerUserID:         ownerUserID,
		MaxDeductionPercent: &nextPercent,
	})

	require.NoError(t, err)
	require.Equal(t, []string{"dine_in", "takeaway"}, result.BalanceUsableScenes)
	require.Equal(t, []string{"dine_in"}, result.BonusUsableScenes)
	require.True(t, result.AllowWithVoucher)
	require.True(t, result.AllowWithDiscount)
	require.Equal(t, nextPercent, result.MaxDeductionPercent)
}
