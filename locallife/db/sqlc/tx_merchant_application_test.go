package db

import (
	"context"
	"encoding/json"
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

func TestApproveMerchantApplicationTxPersistsMerchantSubjectProfileAtomically(t *testing.T) {
	user := createRandomUser(t)
	app := prepareSubmittedMerchantApplication(t, user.ID)
	profileArg := UpsertMerchantSubjectProfileParams{
		MerchantApplicationID:  app.ID,
		UserID:                 user.ID,
		BusinessLicenseNumber:  "91330100MANUAL0001",
		BusinessLicenseName:    "人工修正后的餐饮店",
		BusinessLicenseAddress: "杭州市西湖区修正路1号",
		LegalPersonName:        "李四",
		LegalPersonIDNumber:    "110101199001010099",
		FoodPermitNumber:       "JY11105000000001",
		FoodPermitCompanyName:  "人工修正后的餐饮店",
		BusinessLicensePayload: []byte(`{"valid_period":"2020年01月01日至2040年01月01日"}`),
		FoodPermitPayload:      []byte(`{"valid_to":"2030年12月31日"}`),
		LegalPersonPayload:     []byte(`{"id_card_valid_date":"2020.01.01-2035.01.01"}`),
		SourceSnapshot:         []byte(`{"source":"merchant_application","status":"approved"}`),
	}

	result, err := testStore.ApproveMerchantApplicationTx(context.Background(), ApproveMerchantApplicationTxParams{
		ApplicationID:                 app.ID,
		UserID:                        user.ID,
		MerchantName:                  app.MerchantName,
		Phone:                         app.ContactPhone,
		Address:                       app.BusinessAddress,
		Latitude:                      app.Latitude,
		Longitude:                     app.Longitude,
		RegionID:                      app.RegionID.Int64,
		AppData:                       []byte(`{"source":"test"}`),
		SubjectProfile:                &profileArg,
		SubjectProfileVersionSnapshot: []byte(`{"business_license":{"number":"91330100MANUAL0001"}}`),
	})
	require.NoError(t, err)
	require.NotZero(t, result.SubjectProfile.ID)
	require.True(t, result.SubjectProfile.MerchantID.Valid)
	require.Equal(t, result.Merchant.ID, result.SubjectProfile.MerchantID.Int64)

	profile, err := testStore.GetMerchantSubjectProfileByMerchant(context.Background(), pgtype.Int8{Int64: result.Merchant.ID, Valid: true})
	require.NoError(t, err)
	require.Equal(t, app.ID, profile.MerchantApplicationID)
	require.Equal(t, result.Merchant.ID, profile.MerchantID.Int64)
	require.Equal(t, "91330100MANUAL0001", profile.BusinessLicenseNumber)

	version, err := testStore.CreateMerchantSubjectProfileVersion(context.Background(), CreateMerchantSubjectProfileVersionParams{
		ProfileID:             profile.ID,
		MerchantApplicationID: profile.MerchantApplicationID,
		MerchantID:            profile.MerchantID,
		UserID:                profile.UserID,
		Version:               profile.Version,
		Snapshot:              []byte(`{"duplicate":"ignored"}`),
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
	require.Zero(t, version.ID)
}

func TestApproveMerchantApplicationTxUpdatesExistingMerchantSubjectProfileOnReapproval(t *testing.T) {
	user := createRandomUser(t)
	app := prepareSubmittedMerchantApplication(t, user.ID)
	firstProfileArg := UpsertMerchantSubjectProfileParams{
		MerchantApplicationID:  app.ID,
		UserID:                 user.ID,
		BusinessLicenseNumber:  "91330100OLD0001",
		BusinessLicenseName:    "旧主体餐饮店",
		BusinessLicensePayload: []byte(`{"credit_code":"91330100OLD0001"}`),
		FoodPermitPayload:      []byte(`{}`),
		LegalPersonPayload:     []byte(`{}`),
		SourceSnapshot:         []byte(`{"status":"approved"}`),
	}

	first, err := testStore.ApproveMerchantApplicationTx(context.Background(), ApproveMerchantApplicationTxParams{
		ApplicationID:                 app.ID,
		UserID:                        user.ID,
		MerchantName:                  app.MerchantName,
		Phone:                         app.ContactPhone,
		Address:                       app.BusinessAddress,
		Latitude:                      app.Latitude,
		Longitude:                     app.Longitude,
		RegionID:                      app.RegionID.Int64,
		AppData:                       []byte(`{"business_license_number":"91330100OLD0001"}`),
		SubjectProfile:                &firstProfileArg,
		SubjectProfileVersionSnapshot: []byte(`{"business_license":{"number":"91330100OLD0001"}}`),
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), first.SubjectProfile.Version)

	resetResult, err := testStore.(*SQLStore).ResetMerchantApplicationTx(context.Background(), ResetMerchantApplicationTxParams{
		ApplicationID: app.ID,
		UserID:        user.ID,
	})
	require.NoError(t, err)
	require.Equal(t, MerchantApplicationStatusDraft, resetResult.Application.Status)
	require.Equal(t, first.Merchant.ID, resetResult.Merchant.ID)

	updated, err := testStore.UpdateMerchantApplicationBasicInfo(context.Background(), UpdateMerchantApplicationBasicInfoParams{
		ID:                    app.ID,
		MerchantName:          pgtype.Text{String: "人工修正后的餐饮店", Valid: true},
		BusinessLicenseNumber: pgtype.Text{String: "91330100MANUAL0001", Valid: true},
	})
	require.NoError(t, err)
	resubmitted, err := testStore.SubmitMerchantApplication(context.Background(), updated.ID)
	require.NoError(t, err)
	require.Equal(t, MerchantApplicationStatusSubmitted, resubmitted.Status)

	secondProfileArg := UpsertMerchantSubjectProfileParams{
		MerchantApplicationID:  resubmitted.ID,
		UserID:                 user.ID,
		BusinessLicenseNumber:  "91330100MANUAL0001",
		BusinessLicenseName:    "人工修正后的餐饮店",
		BusinessLicensePayload: []byte(`{"credit_code":"91330100MANUAL0001"}`),
		FoodPermitPayload:      []byte(`{}`),
		LegalPersonPayload:     []byte(`{}`),
		SourceSnapshot:         []byte(`{"status":"approved"}`),
	}
	second, err := testStore.ApproveMerchantApplicationTx(context.Background(), ApproveMerchantApplicationTxParams{
		ApplicationID:                 resubmitted.ID,
		UserID:                        user.ID,
		MerchantName:                  resubmitted.MerchantName,
		Phone:                         resubmitted.ContactPhone,
		Address:                       resubmitted.BusinessAddress,
		Latitude:                      resubmitted.Latitude,
		Longitude:                     resubmitted.Longitude,
		RegionID:                      resubmitted.RegionID.Int64,
		AppData:                       []byte(`{"business_license_number":"91330100MANUAL0001"}`),
		SubjectProfile:                &secondProfileArg,
		SubjectProfileVersionSnapshot: []byte(`{"business_license":{"number":"91330100MANUAL0001"}}`),
	})
	require.NoError(t, err)
	require.Equal(t, first.Merchant.ID, second.Merchant.ID)
	require.Equal(t, first.SubjectProfile.ID, second.SubjectProfile.ID)
	require.Equal(t, int32(2), second.SubjectProfile.Version)
	require.Equal(t, "91330100MANUAL0001", second.SubjectProfile.BusinessLicenseNumber)
	require.Equal(t, "人工修正后的餐饮店", second.SubjectProfile.BusinessLicenseName)
	require.JSONEq(t, `{"business_license_number":"91330100MANUAL0001"}`, string(second.Merchant.ApplicationData))

	profile, err := testStore.GetMerchantSubjectProfileByMerchant(context.Background(), pgtype.Int8{Int64: first.Merchant.ID, Valid: true})
	require.NoError(t, err)
	require.Equal(t, second.SubjectProfile.ID, profile.ID)
	require.Equal(t, int32(2), profile.Version)
	require.Equal(t, "91330100MANUAL0001", profile.BusinessLicenseNumber)
	merchant, err := testStore.GetMerchant(context.Background(), first.Merchant.ID)
	require.NoError(t, err)
	require.JSONEq(t, `{"business_license_number":"91330100MANUAL0001"}`, string(merchant.ApplicationData))
}

func TestApproveMerchantApplicationTxRollsBackWhenSubjectProfileInvalid(t *testing.T) {
	user := createRandomUser(t)
	app := prepareSubmittedMerchantApplication(t, user.ID)
	wrongApp := createRandomMerchantApplicationWithUser(t, user.ID)
	profileArg := UpsertMerchantSubjectProfileParams{
		MerchantApplicationID:  wrongApp.ID,
		UserID:                 user.ID,
		BusinessLicenseNumber:  "91330100MANUAL0001",
		BusinessLicenseName:    "人工修正后的餐饮店",
		LegalPersonName:        "李四",
		LegalPersonIDNumber:    "110101199001010099",
		BusinessLicensePayload: []byte(`{}`),
		FoodPermitPayload:      []byte(`{}`),
		LegalPersonPayload:     []byte(`{}`),
		SourceSnapshot:         []byte(`{"source":"test"}`),
	}

	_, err := testStore.ApproveMerchantApplicationTx(context.Background(), ApproveMerchantApplicationTxParams{
		ApplicationID:  app.ID,
		UserID:         user.ID,
		MerchantName:   app.MerchantName,
		Phone:          app.ContactPhone,
		Address:        app.BusinessAddress,
		Latitude:       app.Latitude,
		Longitude:      app.Longitude,
		RegionID:       app.RegionID.Int64,
		AppData:        []byte(`{"source":"test"}`),
		SubjectProfile: &profileArg,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "merchant subject profile application mismatch")

	unchanged, getErr := testStore.GetMerchantApplication(context.Background(), app.ID)
	require.NoError(t, getErr)
	require.Equal(t, "submitted", unchanged.Status)
	_, getErr = testStore.GetMerchantOwnedByUser(context.Background(), user.ID)
	require.ErrorIs(t, getErr, ErrRecordNotFound)
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

func TestResetMerchantApplicationTxCancelsActiveReviewRuns(t *testing.T) {
	user := createRandomUser(t)
	app := prepareSubmittedMerchantApplication(t, user.ID)

	run, err := testStore.CreateMerchantOnboardingReviewRun(context.Background(), CreateMerchantOnboardingReviewRunParams{
		MerchantApplicationID: pgtype.Int8{Int64: app.ID, Valid: true},
		RunStatus:             "queued",
		Stage:                 "review",
		Evidence:              []byte(`{}`),
		RuleHits:              []string{},
		OcrJobRefs:            []int64{101},
		Snapshot:              []byte(`{"status":"submitted"}`),
		RequestedBy:           pgtype.Int8{Int64: user.ID, Valid: true},
	})
	require.NoError(t, err)

	queuedSummary, err := json.Marshal(map[string]any{
		"run_id":       run.ID,
		"stage":        "review",
		"outcome":      "",
		"reason_code":  "",
		"ocr_job_refs": []int64{101},
		"created_at":   "2026-06-11T00:00:00Z",
	})
	require.NoError(t, err)
	_, err = testStore.UpdateMerchantApplicationReviewSummary(context.Background(), UpdateMerchantApplicationReviewSummaryParams{
		ID:            app.ID,
		ReviewSummary: queuedSummary,
	})
	require.NoError(t, err)

	result, err := testStore.(*SQLStore).ResetMerchantApplicationTx(context.Background(), ResetMerchantApplicationTxParams{
		ApplicationID: app.ID,
		UserID:        user.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "draft", result.Application.Status)

	cancelledRun, err := testStore.GetOnboardingReviewRun(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", cancelledRun.RunStatus)
	require.Equal(t, "needs_resubmit", cancelledRun.Outcome.String)
	require.Equal(t, "superseded_by_edit", cancelledRun.ReasonCode.String)
	require.Contains(t, cancelledRun.ReasonMessage.String, "重新编辑")
	require.True(t, cancelledRun.FinishedAt.Valid)

	var summary map[string]any
	require.NoError(t, json.Unmarshal(result.Application.ReviewSummary, &summary))
	require.Equal(t, float64(run.ID), summary["run_id"])
	require.Equal(t, "needs_resubmit", summary["outcome"])
	require.Equal(t, "superseded_by_edit", summary["reason_code"])
	require.Contains(t, summary["reason_message"], "重新提交")
}

func TestResetMerchantApplicationTxPreservesApprovedMerchantTruth(t *testing.T) {
	testCases := []struct {
		name           string
		merchantStatus string
	}{
		{
			name:           "active merchant",
			merchantStatus: MerchantStatusActive,
		},
		{
			name:           "approved merchant",
			merchantStatus: MerchantStatusApproved,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			user := createRandomUser(t)
			app := prepareSubmittedMerchantApplication(t, user.ID)
			merchant := createRandomMerchantWithOwner(t, user.ID)
			merchant, err := testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
				ID:     merchant.ID,
				Status: tc.merchantStatus,
			})
			require.NoError(t, err)

			storefrontImages := []byte(`["uploads/merchant/live-storefront.jpg"]`)
			environmentImages := []byte(`["uploads/merchant/live-environment.jpg"]`)
			merchant, err = testStore.UpdateMerchantShopImages(context.Background(), UpdateMerchantShopImagesParams{
				ID:                merchant.ID,
				StorefrontImages:  storefrontImages,
				EnvironmentImages: environmentImages,
			})
			require.NoError(t, err)

			approvedApp, err := testStore.UpdateMerchantApplicationStatus(context.Background(), UpdateMerchantApplicationStatusParams{
				ID:         app.ID,
				Status:     MerchantApplicationStatusApproved,
				ReviewedBy: pgtype.Int8{Int64: user.ID, Valid: true},
			})
			require.NoError(t, err)

			result, err := testStore.(*SQLStore).ResetMerchantApplicationTx(context.Background(), ResetMerchantApplicationTxParams{
				ApplicationID: approvedApp.ID,
				UserID:        user.ID,
			})
			require.NoError(t, err)
			require.Equal(t, MerchantApplicationStatusDraft, result.Application.Status)
			require.Equal(t, merchant.ID, result.Merchant.ID)
			require.Equal(t, tc.merchantStatus, result.Merchant.Status)
			require.JSONEq(t, string(storefrontImages), string(result.Merchant.StorefrontImages))
			require.JSONEq(t, string(environmentImages), string(result.Merchant.EnvironmentImages))

			unchangedMerchant, err := testStore.GetMerchant(context.Background(), merchant.ID)
			require.NoError(t, err)
			require.Equal(t, tc.merchantStatus, unchangedMerchant.Status)
			require.Equal(t, merchant.Name, unchangedMerchant.Name)
			require.Equal(t, merchant.Phone, unchangedMerchant.Phone)
			require.Equal(t, merchant.Address, unchangedMerchant.Address)
			require.JSONEq(t, string(storefrontImages), string(unchangedMerchant.StorefrontImages))
			require.JSONEq(t, string(environmentImages), string(unchangedMerchant.EnvironmentImages))
		})
	}
}

func TestResetMerchantApplicationTxIgnoresStaffAssociatedMerchant(t *testing.T) {
	merchantOwner := createRandomUser(t)
	applicant := createRandomUser(t)
	staffMerchant := createRandomMerchantWithOwner(t, merchantOwner.ID)
	staffMerchant, err := testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     staffMerchant.ID,
		Status: MerchantStatusSuspended,
	})
	require.NoError(t, err)
	_, err = testStore.CreateMerchantStaff(context.Background(), CreateMerchantStaffParams{
		MerchantID: staffMerchant.ID,
		UserID:     applicant.ID,
		Role:       MerchantStaffRoleManager,
		Status:     MerchantStaffStatusActive,
		InvitedBy:  pgtype.Int8{Int64: merchantOwner.ID, Valid: true},
	})
	require.NoError(t, err)

	app := prepareSubmittedMerchantApplication(t, applicant.ID)
	approvedApp, err := testStore.UpdateMerchantApplicationStatus(context.Background(), UpdateMerchantApplicationStatusParams{
		ID:         app.ID,
		Status:     MerchantApplicationStatusApproved,
		ReviewedBy: pgtype.Int8{Int64: applicant.ID, Valid: true},
	})
	require.NoError(t, err)

	result, err := testStore.(*SQLStore).ResetMerchantApplicationTx(context.Background(), ResetMerchantApplicationTxParams{
		ApplicationID: approvedApp.ID,
		UserID:        applicant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, MerchantApplicationStatusDraft, result.Application.Status)
	require.Zero(t, result.Merchant.ID)

	unchangedStaffMerchant, err := testStore.GetMerchant(context.Background(), staffMerchant.ID)
	require.NoError(t, err)
	require.Equal(t, MerchantStatusSuspended, unchangedStaffMerchant.Status)
	require.Equal(t, merchantOwner.ID, unchangedStaffMerchant.OwnerUserID)
}

func TestResetMerchantApplicationTxRejectsMismatchedApplicationOwner(t *testing.T) {
	applicationOwner := createRandomUser(t)
	otherUser := createRandomUser(t)
	app := prepareSubmittedMerchantApplication(t, applicationOwner.ID)
	approvedApp, err := testStore.UpdateMerchantApplicationStatus(context.Background(), UpdateMerchantApplicationStatusParams{
		ID:         app.ID,
		Status:     MerchantApplicationStatusApproved,
		ReviewedBy: pgtype.Int8{Int64: applicationOwner.ID, Valid: true},
	})
	require.NoError(t, err)

	otherMerchant := createRandomMerchantWithOwner(t, otherUser.ID)
	otherMerchant, err = testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     otherMerchant.ID,
		Status: MerchantStatusSuspended,
	})
	require.NoError(t, err)

	_, err = testStore.(*SQLStore).ResetMerchantApplicationTx(context.Background(), ResetMerchantApplicationTxParams{
		ApplicationID: approvedApp.ID,
		UserID:        otherUser.ID,
	})
	require.Error(t, err)

	unchangedApp, err := testStore.GetMerchantApplication(context.Background(), approvedApp.ID)
	require.NoError(t, err)
	require.Equal(t, MerchantApplicationStatusApproved, unchangedApp.Status)
	require.Equal(t, applicationOwner.ID, unchangedApp.UserID)

	unchangedMerchant, err := testStore.GetMerchant(context.Background(), otherMerchant.ID)
	require.NoError(t, err)
	require.Equal(t, MerchantStatusSuspended, unchangedMerchant.Status)
	require.Equal(t, otherUser.ID, unchangedMerchant.OwnerUserID)
}
