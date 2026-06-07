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
