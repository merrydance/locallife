package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestUpsertMerchantSubjectProfileDoesNotBumpVersionForIdenticalProjection(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	app := createRandomMerchantApplicationWithUser(t, user.ID)

	arg := UpsertMerchantSubjectProfileParams{
		MerchantApplicationID:  app.ID,
		UserID:                 user.ID,
		BusinessLicenseNumber:  "91330100MAIDEMPOTENT",
		BusinessLicenseName:    "Idempotent Merchant",
		BusinessLicenseAddress: "No. 1 Test Road",
		LegalPersonName:        "Alice",
		LegalPersonIDNumber:    "110101199001011234",
		FoodPermitNumber:       "JY12345678901234",
		FoodPermitCompanyName:  "Idempotent Merchant",
		BusinessLicensePayload: []byte(`{"credit_code":"91330100MAIDEMPOTENT"}`),
		FoodPermitPayload:      []byte(`{"permit_no":"JY12345678901234"}`),
		LegalPersonPayload:     []byte(`{"name":"Alice"}`),
		SourceSnapshot:         []byte(`{"source":"test"}`),
	}

	first, err := testStore.UpsertMerchantSubjectProfile(ctx, arg)
	require.NoError(t, err)
	require.Equal(t, int32(1), first.Version)

	second, err := testStore.UpsertMerchantSubjectProfile(ctx, arg)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.Version, second.Version)
	require.Equal(t, first.UpdatedAt, second.UpdatedAt)

	arg.BusinessLicenseName = "Changed Merchant"
	third, err := testStore.UpsertMerchantSubjectProfile(ctx, arg)
	require.NoError(t, err)
	require.Equal(t, first.ID, third.ID)
	require.Equal(t, first.Version+1, third.Version)
	require.Equal(t, "Changed Merchant", third.BusinessLicenseName)
}

func TestGetMerchantSubjectProfileByMerchantOnlyReturnsApprovedApplication(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, user.ID)
	app := createRandomMerchantApplicationWithUser(t, user.ID)

	arg := UpsertMerchantSubjectProfileParams{
		MerchantApplicationID:  app.ID,
		MerchantID:             pgtype.Int8{Int64: merchant.ID, Valid: true},
		UserID:                 user.ID,
		BusinessLicenseNumber:  "91330100" + util.RandomString(10),
		BusinessLicenseName:    "Draft Merchant",
		BusinessLicensePayload: []byte(`{}`),
		FoodPermitPayload:      []byte(`{}`),
		LegalPersonPayload:     []byte(`{}`),
		SourceSnapshot:         []byte(`{"source":"test"}`),
	}
	profile, err := testStore.UpsertMerchantSubjectProfile(ctx, arg)
	require.NoError(t, err)

	_, err = testStore.GetMerchantSubjectProfileByMerchant(ctx, pgtype.Int8{Int64: merchant.ID, Valid: true})
	require.ErrorIs(t, err, ErrRecordNotFound)

	_, err = testStore.UpdateMerchantApplicationStatus(ctx, UpdateMerchantApplicationStatusParams{
		ID:     app.ID,
		Status: MerchantApplicationStatusApproved,
	})
	require.NoError(t, err)

	approvedProfile, err := testStore.GetMerchantSubjectProfileByMerchant(ctx, pgtype.Int8{Int64: merchant.ID, Valid: true})
	require.NoError(t, err)
	require.Equal(t, profile.ID, approvedProfile.ID)
	require.Equal(t, app.ID, approvedProfile.MerchantApplicationID)
}

func TestDetachMerchantSubjectProfileMerchantFromOtherApplicationsAllowsLatestApplicationAuthority(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, user.ID)
	firstApp := createRandomMerchantApplicationWithUser(t, user.ID)
	secondApp := createRandomMerchantApplicationWithUser(t, user.ID)

	first, err := testStore.UpsertMerchantSubjectProfile(ctx, UpsertMerchantSubjectProfileParams{
		MerchantApplicationID:  firstApp.ID,
		MerchantID:             pgtype.Int8{Int64: merchant.ID, Valid: true},
		UserID:                 user.ID,
		BusinessLicenseNumber:  "91330100OLD0001",
		BusinessLicenseName:    "旧主体餐饮店",
		BusinessLicensePayload: []byte(`{"credit_code":"91330100OLD0001"}`),
		FoodPermitPayload:      []byte(`{}`),
		LegalPersonPayload:     []byte(`{}`),
		SourceSnapshot:         []byte(`{"source":"first"}`),
	})
	require.NoError(t, err)
	require.Equal(t, firstApp.ID, first.MerchantApplicationID)

	detached, err := testStore.DetachMerchantSubjectProfileMerchantFromOtherApplications(ctx, DetachMerchantSubjectProfileMerchantFromOtherApplicationsParams{
		MerchantID:            pgtype.Int8{Int64: merchant.ID, Valid: true},
		MerchantApplicationID: secondApp.ID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), detached)

	second, err := testStore.UpsertMerchantSubjectProfile(ctx, UpsertMerchantSubjectProfileParams{
		MerchantApplicationID:  secondApp.ID,
		MerchantID:             pgtype.Int8{Int64: merchant.ID, Valid: true},
		UserID:                 user.ID,
		BusinessLicenseNumber:  "91330100MANUAL0001",
		BusinessLicenseName:    "人工修正后的餐饮店",
		BusinessLicensePayload: []byte(`{"credit_code":"91330100MANUAL0001"}`),
		FoodPermitPayload:      []byte(`{}`),
		LegalPersonPayload:     []byte(`{}`),
		SourceSnapshot:         []byte(`{"source":"second"}`),
	})
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
	require.Equal(t, secondApp.ID, second.MerchantApplicationID)
	require.Equal(t, pgtype.Int8{Int64: merchant.ID, Valid: true}, second.MerchantID)
	require.Equal(t, "91330100MANUAL0001", second.BusinessLicenseNumber)
	require.Equal(t, int32(1), second.Version)

	firstDetached, err := testStore.GetMerchantSubjectProfileByApplication(ctx, firstApp.ID)
	require.NoError(t, err)
	require.Equal(t, first.ID, firstDetached.ID)
	require.False(t, firstDetached.MerchantID.Valid)
	require.Equal(t, int32(2), firstDetached.Version)

	approvedSecondApp, err := testStore.UpdateMerchantApplicationStatus(ctx, UpdateMerchantApplicationStatusParams{
		ID:     secondApp.ID,
		Status: MerchantApplicationStatusApproved,
	})
	require.NoError(t, err)
	byMerchant, err := testStore.GetMerchantSubjectProfileByMerchant(ctx, pgtype.Int8{Int64: merchant.ID, Valid: true})
	require.NoError(t, err)
	require.Equal(t, approvedSecondApp.ID, byMerchant.MerchantApplicationID)
	require.Equal(t, "91330100MANUAL0001", byMerchant.BusinessLicenseNumber)
}

func TestMerchantApplicationOCRResultBackfillDoesNotOverwriteManualSubjectFields(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	app := createRandomMerchantApplicationWithUser(t, user.ID)

	manual, err := testStore.UpdateMerchantApplicationBasicInfo(ctx, UpdateMerchantApplicationBasicInfoParams{
		ID:                    app.ID,
		MerchantName:          pgtype.Text{String: "人工修正后的餐饮店", Valid: true},
		BusinessLicenseNumber: pgtype.Text{String: "91330100MANUAL0001", Valid: true},
		BusinessScope:         pgtype.Text{String: "餐饮服务", Valid: true},
		LegalPersonName:       pgtype.Text{String: "李四", Valid: true},
		LegalPersonIDNumber:   pgtype.Text{String: "110101199001010099", Valid: true},
	})
	require.NoError(t, err)

	ocrUpdated, err := testStore.UpdateMerchantApplicationBusinessLicenseOCRResult(ctx, UpdateMerchantApplicationBusinessLicenseOCRResultParams{
		ID:                    manual.ID,
		BusinessLicenseNumber: pgtype.Text{String: "91330100OCRWRONG01", Valid: true},
		BusinessScope:         pgtype.Text{String: "旧经营范围", Valid: true},
		LegalPersonName:       pgtype.Text{String: "张三", Valid: true},
		MerchantName:          pgtype.Text{String: "OCR错误餐饮店", Valid: true},
		BusinessLicenseOcr:    []byte(`{"enterprise_name":"OCR错误餐饮店","credit_code":"91330100OCRWRONG01","legal_representative":"张三"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "人工修正后的餐饮店", ocrUpdated.MerchantName)
	require.Equal(t, "91330100MANUAL0001", ocrUpdated.BusinessLicenseNumber)
	require.Equal(t, "餐饮服务", ocrUpdated.BusinessScope.String)
	require.Equal(t, "李四", ocrUpdated.LegalPersonName)
	require.JSONEq(t, `{"enterprise_name":"OCR错误餐饮店","credit_code":"91330100OCRWRONG01","legal_representative":"张三"}`, string(ocrUpdated.BusinessLicenseOcr))

	idUpdated, err := testStore.UpdateMerchantApplicationIDCardFrontOCRResult(ctx, UpdateMerchantApplicationIDCardFrontOCRResultParams{
		ID:                  manual.ID,
		LegalPersonName:     pgtype.Text{String: "王五", Valid: true},
		LegalPersonIDNumber: pgtype.Text{String: "110101199001010011", Valid: true},
		IDCardFrontOcr:      []byte(`{"name":"王五","id_number":"110101199001010011"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "李四", idUpdated.LegalPersonName)
	require.Equal(t, "110101199001010099", idUpdated.LegalPersonIDNumber)
	require.JSONEq(t, `{"name":"王五","id_number":"110101199001010011"}`, string(idUpdated.IDCardFrontOcr))
}
