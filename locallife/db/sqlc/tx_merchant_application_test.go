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

	capability, err := testStore.GetMerchantCapabilities(context.Background(), result.Merchant.ID)
	require.NoError(t, err)
	require.Equal(t, MerchantCapabilityStatusUnknown, capability.OpenKitchenStatus)
	require.Equal(t, MerchantCapabilityStatusUnknown, capability.DineInStatus)
	require.Equal(t, MerchantCapabilitySourceSystemDefault, capability.Source)

	labels, err := testStore.ListMerchantSystemLabels(context.Background(), result.Merchant.ID)
	require.NoError(t, err)
	require.Len(t, labels, 1)
	require.Equal(t, SystemTagNoOpenKitchen, labels[0].Name)
}

func TestApproveMerchantApplicationTxCopiesApplicationImagesToMerchantTruth(t *testing.T) {
	user := createRandomUser(t)
	app := prepareSubmittedMerchantApplication(t, user.ID)
	storefrontImages := []byte(`["uploads/merchant/application-storefront.jpg"]`)
	environmentImages := []byte(`["uploads/merchant/application-environment.jpg"]`)

	sqlStore, ok := testStore.(*SQLStore)
	require.True(t, ok)
	_, err := sqlStore.connPool.Exec(context.Background(), `
		UPDATE merchant_applications
		SET storefront_images = $1,
		    environment_images = $2
		WHERE id = $3
	`, storefrontImages, environmentImages, app.ID)
	require.NoError(t, err)

	app, err = testStore.GetMerchantApplication(context.Background(), app.ID)
	require.NoError(t, err)

	result, err := testStore.ApproveMerchantApplicationTx(context.Background(), ApproveMerchantApplicationTxParams{
		ApplicationID:     app.ID,
		UserID:            user.ID,
		MerchantName:      app.MerchantName,
		Phone:             app.ContactPhone,
		Address:           app.BusinessAddress,
		Latitude:          app.Latitude,
		Longitude:         app.Longitude,
		RegionID:          app.RegionID.Int64,
		AppData:           []byte(`{"source":"test"}`),
		StorefrontImages:  app.StorefrontImages,
		EnvironmentImages: app.EnvironmentImages,
	})
	require.NoError(t, err)
	require.JSONEq(t, string(storefrontImages), string(result.Merchant.StorefrontImages))
	require.JSONEq(t, string(environmentImages), string(result.Merchant.EnvironmentImages))

	merchant, err := testStore.GetMerchant(context.Background(), result.Merchant.ID)
	require.NoError(t, err)
	require.JSONEq(t, string(storefrontImages), string(merchant.StorefrontImages))
	require.JSONEq(t, string(environmentImages), string(merchant.EnvironmentImages))
}

func TestApproveMerchantApplicationTxIgnoresStaffAssociatedMerchant(t *testing.T) {
	merchantOwner := createRandomUser(t)
	applicant := createRandomUser(t)
	staffMerchant := createRandomMerchantWithOwner(t, merchantOwner.ID)
	app := prepareSubmittedMerchantApplication(t, applicant.ID)
	storefrontImages := []byte(`["uploads/merchant/applicant-storefront.jpg"]`)

	_, err := testStore.CreateMerchantStaff(context.Background(), CreateMerchantStaffParams{
		MerchantID: staffMerchant.ID,
		UserID:     applicant.ID,
		Role:       "manager",
		Status:     MerchantStaffStatusActive,
		InvitedBy:  pgtype.Int8{Int64: merchantOwner.ID, Valid: true},
	})
	require.NoError(t, err)

	result, err := testStore.ApproveMerchantApplicationTx(context.Background(), ApproveMerchantApplicationTxParams{
		ApplicationID:    app.ID,
		UserID:           applicant.ID,
		MerchantName:     app.MerchantName,
		Phone:            app.ContactPhone,
		Address:          app.BusinessAddress,
		Latitude:         app.Latitude,
		Longitude:        app.Longitude,
		RegionID:         app.RegionID.Int64,
		AppData:          []byte(`{"source":"test"}`),
		StorefrontImages: storefrontImages,
	})
	require.NoError(t, err)
	require.NotEqual(t, staffMerchant.ID, result.Merchant.ID)
	require.Equal(t, applicant.ID, result.Merchant.OwnerUserID)
	require.JSONEq(t, string(storefrontImages), string(result.Merchant.StorefrontImages))

	unchangedStaffMerchant, err := testStore.GetMerchant(context.Background(), staffMerchant.ID)
	require.NoError(t, err)
	require.Equal(t, merchantOwner.ID, unchangedStaffMerchant.OwnerUserID)
	require.Equal(t, staffMerchant.Name, unchangedStaffMerchant.Name)
	require.Nil(t, unchangedStaffMerchant.StorefrontImages)
}

func TestApproveMerchantApplicationTxRejectsMismatchedApplicationOwner(t *testing.T) {
	applicationOwner := createRandomUser(t)
	wrongOwner := createRandomUser(t)
	app := prepareSubmittedMerchantApplication(t, applicationOwner.ID)

	_, err := testStore.ApproveMerchantApplicationTx(context.Background(), ApproveMerchantApplicationTxParams{
		ApplicationID: app.ID,
		UserID:        wrongOwner.ID,
		MerchantName:  app.MerchantName,
		Phone:         app.ContactPhone,
		Address:       app.BusinessAddress,
		Latitude:      app.Latitude,
		Longitude:     app.Longitude,
		RegionID:      app.RegionID.Int64,
		AppData:       []byte(`{"source":"test"}`),
	})
	require.Error(t, err)

	unchangedApp, appErr := testStore.GetMerchantApplication(context.Background(), app.ID)
	require.NoError(t, appErr)
	require.Equal(t, "submitted", unchangedApp.Status)

	_, merchantErr := testStore.GetMerchantOwnedByUser(context.Background(), wrongOwner.ID)
	require.ErrorIs(t, merchantErr, ErrRecordNotFound)
}
