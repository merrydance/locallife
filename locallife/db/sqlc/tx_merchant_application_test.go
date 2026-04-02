package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func prepareSubmittedMerchantApplication(t *testing.T, userID int64) MerchantApplication {
	t.Helper()

	region := createRandomRegion(t)
	app := createRandomMerchantApplicationWithUser(t, userID)
	address := "测试商户地址-" + util.RandomString(8)

	updated, err := testStore.UpdateMerchantApplicationBasicInfo(context.Background(), UpdateMerchantApplicationBasicInfoParams{
		ID:              app.ID,
		MerchantName:    pgtype.Text{String: "测试餐厅", Valid: true},
		ContactPhone:    pgtype.Text{String: "13812345678", Valid: true},
		BusinessAddress: pgtype.Text{String: address, Valid: true},
		BusinessScope:   pgtype.Text{String: "餐饮服务", Valid: true},
		Longitude:       numericFromFloat(116.4507),
		Latitude:        numericFromFloat(39.9282),
		RegionID:        pgtype.Int8{Int64: region.ID, Valid: true},
	})
	require.NoError(t, err)

	updated, err = testStore.SubmitMerchantApplication(context.Background(), updated.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", updated.Status)

	return updated
}

func TestApproveMerchantApplicationTx_AssignsMerchantOwnerRoleAndOwnerStaff(t *testing.T) {
	user := createRandomUser(t)
	app := prepareSubmittedMerchantApplication(t, user.ID)

	result, err := testStore.ApproveMerchantApplicationTx(context.Background(), ApproveMerchantApplicationTxParams{
		ApplicationID: app.ID,
		UserID:        user.ID,
		MerchantName:  app.MerchantName,
		Phone:         app.ContactPhone,
		Address:       app.BusinessAddress,
		Latitude:      app.Latitude,
		Longitude:     app.Longitude,
		RegionID:      app.RegionID.Int64,
		AppData:       []byte(`{"source":"test"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "approved", result.Application.Status)
	require.Equal(t, "approved", result.Merchant.Status)

	role, err := testStore.GetUserRoleByType(context.Background(), GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleMerchantOwner,
	})
	require.NoError(t, err)
	require.Equal(t, MerchantStaffStatusActive, role.Status)
	require.True(t, role.RelatedEntityID.Valid)
	require.Equal(t, result.Merchant.ID, role.RelatedEntityID.Int64)

	staff, err := testStore.GetMerchantStaff(context.Background(), GetMerchantStaffParams{
		MerchantID: result.Merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)
	require.Equal(t, MerchantStaffRoleOwner, staff.Role)
	require.Equal(t, MerchantStaffStatusActive, staff.Status)
	require.False(t, staff.InvitedBy.Valid)

	merchantByOwner, err := testStore.GetMerchantByOwner(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, result.Merchant.ID, merchantByOwner.ID)
}
