package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

type merchantSubjectProfileStoreStub struct {
	upsertFn           func(context.Context, db.UpsertMerchantSubjectProfileParams) (db.MerchantSubjectProfile, error)
	detachFn           func(context.Context, db.DetachMerchantSubjectProfileMerchantFromOtherApplicationsParams) (int64, error)
	createVersionFn    func(context.Context, db.CreateMerchantSubjectProfileVersionParams) (db.MerchantSubjectProfileVersion, error)
	getByApplicationFn func(context.Context, int64) (db.MerchantSubjectProfile, error)
	getByMerchantFn    func(context.Context, pgtype.Int8) (db.MerchantSubjectProfile, error)
}

func (stub merchantSubjectProfileStoreStub) UpsertMerchantSubjectProfile(ctx context.Context, arg db.UpsertMerchantSubjectProfileParams) (db.MerchantSubjectProfile, error) {
	if stub.upsertFn == nil {
		return db.MerchantSubjectProfile{}, fmt.Errorf("unexpected UpsertMerchantSubjectProfile call")
	}
	return stub.upsertFn(ctx, arg)
}

func (stub merchantSubjectProfileStoreStub) DetachMerchantSubjectProfileMerchantFromOtherApplications(ctx context.Context, arg db.DetachMerchantSubjectProfileMerchantFromOtherApplicationsParams) (int64, error) {
	if stub.detachFn == nil {
		return 0, fmt.Errorf("unexpected DetachMerchantSubjectProfileMerchantFromOtherApplications call")
	}
	return stub.detachFn(ctx, arg)
}

func (stub merchantSubjectProfileStoreStub) CreateMerchantSubjectProfileVersion(ctx context.Context, arg db.CreateMerchantSubjectProfileVersionParams) (db.MerchantSubjectProfileVersion, error) {
	if stub.createVersionFn == nil {
		return db.MerchantSubjectProfileVersion{}, fmt.Errorf("unexpected CreateMerchantSubjectProfileVersion call")
	}
	return stub.createVersionFn(ctx, arg)
}

func (stub merchantSubjectProfileStoreStub) GetMerchantSubjectProfileByApplication(ctx context.Context, applicationID int64) (db.MerchantSubjectProfile, error) {
	if stub.getByApplicationFn == nil {
		return db.MerchantSubjectProfile{}, fmt.Errorf("unexpected GetMerchantSubjectProfileByApplication call")
	}
	return stub.getByApplicationFn(ctx, applicationID)
}

func (stub merchantSubjectProfileStoreStub) GetMerchantSubjectProfileByMerchant(ctx context.Context, merchantID pgtype.Int8) (db.MerchantSubjectProfile, error) {
	if stub.getByMerchantFn == nil {
		return db.MerchantSubjectProfile{}, fmt.Errorf("unexpected GetMerchantSubjectProfileByMerchant call")
	}
	return stub.getByMerchantFn(ctx, merchantID)
}

func TestBuildMerchantSubjectProfileFromApplication_ManualStructuredFieldsOverrideStaleOCR(t *testing.T) {
	app := db.MerchantApplication{
		ID:                          101,
		UserID:                      202,
		MerchantName:                "人工修正后的餐饮店",
		BusinessLicenseNumber:       "91330100MANUAL0001",
		BusinessScope:               pgtype.Text{String: "餐饮服务", Valid: true},
		LegalPersonName:             "李四",
		LegalPersonIDNumber:         "110101199001010099",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 11, Valid: true},
		FoodPermitMediaAssetID:      pgtype.Int8{Int64: 12, Valid: true},
		IDCardFrontMediaAssetID:     pgtype.Int8{Int64: 13, Valid: true},
		IDCardBackMediaAssetID:      pgtype.Int8{Int64: 14, Valid: true},
		BusinessLicenseOcr: []byte(`{
			"status":"done",
			"ocr_job_id":301,
			"enterprise_name":"OCR错误餐饮店",
			"credit_code":"91330100OCRWRONG01",
			"reg_num":"REGWRONG",
			"legal_representative":"张三",
			"address":"OCR错误地址",
			"business_scope":"旧经营范围",
			"valid_period":"2020年01月01日至2040年01月01日",
			"confirmation":{"confirmed_by":202,"source":"merchant","snapshot":{"enterprise_name":"OCR错误餐饮店","credit_code":"91330100OCRWRONG01"}}
		}`),
		FoodPermitOcr: []byte(`{
			"status":"done",
			"ocr_job_id":302,
			"permit_no":"JY11111111111111",
			"company_name":"OCR错误餐饮店",
			"operator_name":"张三",
			"valid_to":"2030年12月31日"
		}`),
		IDCardFrontOcr: []byte(`{"status":"done","ocr_job_id":303,"name":"张三","id_number":"110101199001010011","address":"旧身份证地址"}`),
		IDCardBackOcr:  []byte(`{"status":"done","ocr_job_id":304,"valid_date":"2020.01.01-2035.01.01"}`),
	}

	profile, err := BuildMerchantSubjectProfileFromApplication(app)
	require.NoError(t, err)

	require.Equal(t, int64(101), profile.ApplicationID)
	require.Equal(t, int64(202), profile.UserID)
	require.Equal(t, "91330100MANUAL0001", profile.BusinessLicense.Number)
	require.Equal(t, "人工修正后的餐饮店", profile.BusinessLicense.Name)
	require.Equal(t, "OCR错误地址", profile.BusinessLicense.Address)
	require.Equal(t, "餐饮服务", profile.BusinessLicense.BusinessScope)
	require.Equal(t, "李四", profile.LegalPerson.Name)
	require.Equal(t, "110101199001010099", profile.LegalPerson.IDNumber)
	require.Equal(t, "JY11111111111111", profile.FoodPermit.PermitNo)
	require.Equal(t, "OCR错误餐饮店", profile.FoodPermit.CompanyName)
	require.Equal(t, "张三", profile.FoodPermit.OperatorName)

	var licensePayload map[string]any
	require.NoError(t, json.Unmarshal(profile.BusinessLicensePayload, &licensePayload))
	require.Equal(t, "91330100MANUAL0001", licensePayload["credit_code"])
	require.Equal(t, "人工修正后的餐饮店", licensePayload["enterprise_name"])
	require.Equal(t, "李四", licensePayload["legal_representative"])
	require.Contains(t, licensePayload, "confirmation")
}

func TestBuildMerchantSubjectProfileFromApplication_CorrectedOCRFieldsOverrideStaleStructuredFields(t *testing.T) {
	app := db.MerchantApplication{
		ID:                    102,
		UserID:                202,
		MerchantName:          "旧申请表店名",
		BusinessLicenseNumber: "91330100STALE0001",
		BusinessScope:         pgtype.Text{String: "旧经营范围", Valid: true},
		LegalPersonName:       "旧法人",
		LegalPersonIDNumber:   "110101199001010099",
		BusinessLicenseOcr: []byte(`{
			"status":"done",
			"ocr_job_id":301,
			"enterprise_name":"人工修正后的营业执照名称",
			"credit_code":"91330100CORRECT01",
			"legal_representative":"修正法人",
			"address":"杭州市西湖区修正路1号",
			"business_scope":"餐饮服务",
			"valid_period":"2020年01月01日至2040年01月01日",
			"correction":{"corrected_by":202,"source":"merchant","fields":["enterprise_name","credit_code","legal_representative","address","business_scope"],"previous":{"enterprise_name":"旧申请表店名","credit_code":"91330100STALE0001","legal_representative":"旧法人","business_scope":"旧经营范围"}}
		}`),
		FoodPermitOcr: []byte(`{
			"status":"done",
			"ocr_job_id":302,
			"permit_no":"JY11111111111111",
			"company_name":"人工确认后的食品证主体名",
			"operator_name":"食品证经营者",
			"valid_to":"2030年12月31日",
			"confirmation":{"confirmed_by":202,"source":"merchant","snapshot":{"company_name":"人工确认后的食品证主体名","permit_no":"JY11111111111111"}}
		}`),
		IDCardFrontOcr: []byte(`{"status":"done","ocr_job_id":303,"name":"旧法人","id_number":"110101199001010099"}`),
		IDCardBackOcr:  []byte(`{"status":"done","ocr_job_id":304,"valid_date":"2020.01.01-2035.01.01"}`),
	}

	profile, err := BuildMerchantSubjectProfileFromApplication(app)
	require.NoError(t, err)

	require.Equal(t, "91330100CORRECT01", profile.BusinessLicense.Number)
	require.Equal(t, "人工修正后的营业执照名称", profile.BusinessLicense.Name)
	require.Equal(t, "修正法人", profile.BusinessLicense.LegalRepresentative)
	require.Equal(t, "杭州市西湖区修正路1号", profile.BusinessLicense.Address)
	require.Equal(t, "餐饮服务", profile.BusinessLicense.BusinessScope)
	require.Equal(t, "修正法人", profile.LegalPerson.Name)
	require.Equal(t, "人工确认后的食品证主体名", profile.FoodPermit.CompanyName)
	require.Equal(t, "食品证经营者", profile.FoodPermit.OperatorName)

	var licensePayload map[string]any
	require.NoError(t, json.Unmarshal(profile.BusinessLicensePayload, &licensePayload))
	require.Equal(t, "人工修正后的营业执照名称", licensePayload["enterprise_name"])
	require.Equal(t, "91330100CORRECT01", licensePayload["credit_code"])
	require.Equal(t, "修正法人", licensePayload["legal_representative"])
	require.Contains(t, licensePayload, "correction")
}

func TestBuildMerchantSubjectProfileFromApplication_LatestStructuredFieldsOverrideStaleCorrection(t *testing.T) {
	app := db.MerchantApplication{
		ID:                    103,
		UserID:                202,
		MerchantName:          "最终人工修正主体名称",
		BusinessLicenseNumber: "91330100LATEST01",
		BusinessScope:         pgtype.Text{String: "餐饮服务;热食类食品制售", Valid: true},
		LegalPersonName:       "最终法人",
		LegalPersonIDNumber:   "110101199001010099",
		BusinessLicenseOcr: []byte(`{
			"status":"done",
			"ocr_job_id":301,
			"enterprise_name":"旧OCR更正主体名称",
			"credit_code":"91330100OLD0001",
			"legal_representative":"旧法人",
			"address":"杭州市西湖区修正路1号",
			"business_scope":"旧经营范围",
			"valid_period":"2020年01月01日至2040年01月01日",
			"correction":{"corrected_by":202,"source":"merchant","fields":["enterprise_name","credit_code","legal_representative","business_scope"],"previous":{"enterprise_name":"OCR错误名称","credit_code":"91330100WRONG01","legal_representative":"错误法人","business_scope":"旧经营范围"}}
		}`),
		FoodPermitOcr: []byte(`{
			"status":"done",
			"ocr_job_id":302,
			"permit_no":"JY11111111111111",
			"company_name":"旧OCR更正主体名称",
			"operator_name":"旧法人",
			"valid_to":"2030年12月31日"
		}`),
		IDCardFrontOcr: []byte(`{"status":"done","ocr_job_id":303,"name":"旧法人","id_number":"110101199001010011"}`),
		IDCardBackOcr:  []byte(`{"status":"done","ocr_job_id":304,"valid_date":"2020.01.01-2035.01.01"}`),
	}

	profile, err := BuildMerchantSubjectProfileFromApplication(app)
	require.NoError(t, err)

	require.Equal(t, "91330100LATEST01", profile.BusinessLicense.Number)
	require.Equal(t, "最终人工修正主体名称", profile.BusinessLicense.Name)
	require.Equal(t, "最终法人", profile.BusinessLicense.LegalRepresentative)
	require.Equal(t, "餐饮服务;热食类食品制售", profile.BusinessLicense.BusinessScope)
	require.Equal(t, "最终法人", profile.LegalPerson.Name)
	require.Equal(t, "110101199001010099", profile.LegalPerson.IDNumber)
	require.Equal(t, "旧OCR更正主体名称", profile.FoodPermit.CompanyName)
	require.Equal(t, "旧法人", profile.FoodPermit.OperatorName)

	var licensePayload map[string]any
	require.NoError(t, json.Unmarshal(profile.BusinessLicensePayload, &licensePayload))
	require.Equal(t, "91330100LATEST01", licensePayload["credit_code"])
	require.Equal(t, "最终人工修正主体名称", licensePayload["enterprise_name"])
	require.Equal(t, "最终法人", licensePayload["legal_representative"])
	require.Equal(t, "餐饮服务;热食类食品制售", licensePayload["business_scope"])
}

func TestMerchantSubjectProfile_BuildsReviewAndBaofuInputsFromAuthority(t *testing.T) {
	profile := MerchantSubjectProfile{
		ApplicationID: 101,
		UserID:        202,
		BusinessLicense: MerchantSubjectBusinessLicense{
			Number:              "91330100MANUAL0001",
			Name:                "人工修正后的餐饮店",
			LegalRepresentative: "李四",
			Address:             "杭州市西湖区修正路1号",
			BusinessScope:       "餐饮服务",
			ValidPeriod:         "2020年01月01日至2040年01月01日",
			MediaAssetID:        11,
			OCRJobID:            int64PtrForSubjectProfileTest(301),
		},
		FoodPermit: MerchantSubjectFoodPermit{
			PermitNo:     "JY11111111111111",
			CompanyName:  "人工修正后的餐饮店",
			OperatorName: "李四",
			ValidTo:      "2030年12月31日",
			MediaAssetID: 12,
			OCRJobID:     int64PtrForSubjectProfileTest(302),
		},
		LegalPerson: MerchantSubjectLegalPerson{
			Name:                "李四",
			IDNumber:            "110101199001010099",
			IDCardAddress:       "北京市朝阳区证件路1号",
			IDCardValidDate:     "2020.01.01-2035.01.01",
			IDCardFrontMediaID:  13,
			IDCardBackMediaID:   14,
			IDCardFrontOCRJobID: int64PtrForSubjectProfileTest(303),
			IDCardBackOCRJobID:  int64PtrForSubjectProfileTest(304),
		},
	}

	reviewInput := profile.ReviewInput()
	require.Equal(t, "91330100MANUAL0001", reviewInput.BusinessLicense.CreditCode)
	require.Equal(t, "人工修正后的餐饮店", reviewInput.BusinessLicense.EnterpriseName)
	require.Equal(t, "李四", reviewInput.BusinessLicense.LegalRepresentative)
	require.Equal(t, "人工修正后的餐饮店", reviewInput.FoodPermit.CompanyName)
	require.Equal(t, "李四", reviewInput.FoodPermit.OperatorName)
	require.Equal(t, "李四", reviewInput.IDCardFront.Name)
	require.Equal(t, "110101199001010099", reviewInput.IDCardFront.IDNumber)

	baofuInput := profile.BaofuOpeningProfileDefaults()
	require.Equal(t, "人工修正后的餐饮店", baofuInput.LegalName)
	require.Equal(t, "91330100MANUAL0001", baofuInput.BusinessLicenseNo)
	require.Equal(t, "李四", baofuInput.LegalPersonName)
	require.Equal(t, "110101199001010099", baofuInput.LegalPersonIDNumber)
}

func TestMerchantSubjectProfile_ReviewInputPreservesOCRMetadata(t *testing.T) {
	profile := MerchantSubjectProfile{
		ApplicationID: 101,
		UserID:        202,
		BusinessLicense: MerchantSubjectBusinessLicense{
			Number:              "91330100MANUAL0001",
			Name:                "人工修正后的餐饮店",
			LegalRepresentative: "李四",
			Address:             "杭州市西湖区修正路1号",
			BusinessScope:       "餐饮服务",
			ValidPeriod:         "2020年01月01日至2040年01月01日",
		},
		FoodPermit: MerchantSubjectFoodPermit{
			PermitNo:     "JY11111111111111",
			CompanyName:  "人工修正后的餐饮店",
			OperatorName: "李四",
			ValidTo:      "2030年12月31日",
		},
		LegalPerson: MerchantSubjectLegalPerson{
			Name:            "李四",
			IDNumber:        "110101199001010099",
			IDCardValidDate: "2020.01.01-2035.01.01",
		},
		BusinessLicensePayload: []byte(`{
			"ocr_job_id":301,
			"readiness":{"state":"ready","reason_code":"ok"},
			"confirmation":{"confirmed_by":202,"source":"merchant","snapshot":{"enterprise_name":"旧OCR餐饮店","credit_code":"91330100OCRWRONG01"}},
			"enterprise_name":"旧OCR餐饮店",
			"credit_code":"91330100OCRWRONG01",
			"legal_representative":"张三",
			"address":"OCR旧地址",
			"business_scope":"旧经营范围",
			"valid_period":"2020年01月01日至2040年01月01日"
		}`),
		FoodPermitPayload: []byte(`{
			"ocr_job_id":302,
			"readiness":{"state":"ready","reason_code":"ok"},
			"confirmation":{"confirmed_by":202,"source":"merchant","snapshot":{"company_name":"旧OCR餐饮店","permit_no":"JY11111111111111"}},
			"permit_no":"JY11111111111111",
			"company_name":"旧OCR餐饮店",
			"operator_name":"张三",
			"valid_to":"2030年12月31日"
		}`),
		LegalPersonPayload: []byte(`{
			"name":"张三",
			"id_number":"110101199001010011",
			"id_card_valid_date":"2020.01.01-2035.01.01",
			"id_card_front_ocr_job_id":303,
			"id_card_back_ocr_job_id":304
		}`),
	}

	reviewInput := profile.ReviewInput()

	require.Equal(t, int64(301), *reviewInput.BusinessLicense.OCRJobID)
	require.Equal(t, "ready", reviewInput.BusinessLicense.Readiness.State)
	require.NotNil(t, reviewInput.BusinessLicense.Confirmation)
	require.Equal(t, int64(202), reviewInput.BusinessLicense.Confirmation.ConfirmedBy)
	require.Equal(t, "91330100MANUAL0001", reviewInput.BusinessLicense.CreditCode)
	require.Equal(t, "人工修正后的餐饮店", reviewInput.BusinessLicense.EnterpriseName)
	require.Equal(t, "李四", reviewInput.BusinessLicense.LegalRepresentative)
	require.Equal(t, "杭州市西湖区修正路1号", reviewInput.BusinessLicense.Address)
	require.Equal(t, "餐饮服务", reviewInput.BusinessLicense.BusinessScope)

	require.Equal(t, int64(302), *reviewInput.FoodPermit.OCRJobID)
	require.Equal(t, "ready", reviewInput.FoodPermit.Readiness.State)
	require.NotNil(t, reviewInput.FoodPermit.Confirmation)
	require.Equal(t, "人工修正后的餐饮店", reviewInput.FoodPermit.CompanyName)
	require.Equal(t, "李四", reviewInput.FoodPermit.OperatorName)
	require.Equal(t, int64(303), *reviewInput.IDCardFront.OCRJobID)
	require.Equal(t, int64(304), *reviewInput.IDCardBack.OCRJobID)
	require.Equal(t, "李四", reviewInput.IDCardFront.Name)
	require.Equal(t, "110101199001010099", reviewInput.IDCardFront.IDNumber)
}

func int64PtrForSubjectProfileTest(value int64) *int64 {
	return &value
}

func TestMerchantSubjectProfileService_SaveApplicationProfilePersistsVersionedAuthority(t *testing.T) {
	app := db.MerchantApplication{
		ID:                          101,
		UserID:                      202,
		MerchantName:                "人工修正后的餐饮店",
		BusinessLicenseNumber:       "91330100MANUAL0001",
		LegalPersonName:             "李四",
		LegalPersonIDNumber:         "110101199001010099",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 11, Valid: true},
		FoodPermitMediaAssetID:      pgtype.Int8{Int64: 12, Valid: true},
		IDCardFrontMediaAssetID:     pgtype.Int8{Int64: 13, Valid: true},
		IDCardBackMediaAssetID:      pgtype.Int8{Int64: 14, Valid: true},
		BusinessLicenseOcr:          []byte(`{"enterprise_name":"OCR错误餐饮店","credit_code":"91330100OCRWRONG01","legal_representative":"张三","address":"杭州市西湖区修正路1号","business_scope":"餐饮服务","valid_period":"2020年01月01日至2040年01月01日"}`),
		FoodPermitOcr:               []byte(`{"permit_no":"JY11111111111111","company_name":"OCR错误餐饮店","operator_name":"张三","valid_to":"2030年12月31日"}`),
		IDCardFrontOcr:              []byte(`{"name":"张三","id_number":"110101199001010011"}`),
		IDCardBackOcr:               []byte(`{"valid_date":"2020.01.01-2035.01.01"}`),
	}

	var upsertArg db.UpsertMerchantSubjectProfileParams
	var versionArg db.CreateMerchantSubjectProfileVersionParams
	service := NewMerchantSubjectProfileService(merchantSubjectProfileStoreStub{
		upsertFn: func(_ context.Context, arg db.UpsertMerchantSubjectProfileParams) (db.MerchantSubjectProfile, error) {
			upsertArg = arg
			return db.MerchantSubjectProfile{
				ID:                          501,
				MerchantApplicationID:       arg.MerchantApplicationID,
				UserID:                      arg.UserID,
				BusinessLicenseNumber:       arg.BusinessLicenseNumber,
				BusinessLicenseName:         arg.BusinessLicenseName,
				BusinessLicenseAddress:      arg.BusinessLicenseAddress,
				LegalPersonName:             arg.LegalPersonName,
				LegalPersonIDNumber:         arg.LegalPersonIDNumber,
				FoodPermitNumber:            arg.FoodPermitNumber,
				FoodPermitCompanyName:       arg.FoodPermitCompanyName,
				BusinessLicenseMediaAssetID: arg.BusinessLicenseMediaAssetID,
				FoodPermitMediaAssetID:      arg.FoodPermitMediaAssetID,
				IDCardFrontMediaAssetID:     arg.IDCardFrontMediaAssetID,
				IDCardBackMediaAssetID:      arg.IDCardBackMediaAssetID,
				BusinessLicensePayload:      arg.BusinessLicensePayload,
				FoodPermitPayload:           arg.FoodPermitPayload,
				LegalPersonPayload:          arg.LegalPersonPayload,
				SourceSnapshot:              arg.SourceSnapshot,
				Version:                     2,
				CreatedAt:                   time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
				UpdatedAt:                   time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			}, nil
		},
		createVersionFn: func(_ context.Context, arg db.CreateMerchantSubjectProfileVersionParams) (db.MerchantSubjectProfileVersion, error) {
			versionArg = arg
			return db.MerchantSubjectProfileVersion{ID: 601, ProfileID: arg.ProfileID, Version: arg.Version}, nil
		},
	})

	saved, err := service.SaveApplicationProfile(context.Background(), app)
	require.NoError(t, err)

	require.Equal(t, int64(501), saved.ID)
	require.Equal(t, int64(101), upsertArg.MerchantApplicationID)
	require.False(t, upsertArg.MerchantID.Valid)
	require.Equal(t, int64(202), upsertArg.UserID)
	require.Equal(t, "91330100MANUAL0001", upsertArg.BusinessLicenseNumber)
	require.Equal(t, "人工修正后的餐饮店", upsertArg.BusinessLicenseName)
	require.Equal(t, "李四", upsertArg.LegalPersonName)
	require.Equal(t, "110101199001010099", upsertArg.LegalPersonIDNumber)
	require.Equal(t, "JY11111111111111", upsertArg.FoodPermitNumber)
	require.Equal(t, "OCR错误餐饮店", upsertArg.FoodPermitCompanyName)
	require.Equal(t, int64(11), upsertArg.BusinessLicenseMediaAssetID.Int64)
	require.Equal(t, int64(501), versionArg.ProfileID)
	require.Equal(t, int64(101), versionArg.MerchantApplicationID)
	require.Equal(t, int32(2), versionArg.Version)
	var versionSnapshot struct {
		BusinessLicense MerchantSubjectBusinessLicense `json:"business_license"`
		FoodPermit      MerchantSubjectFoodPermit      `json:"food_permit"`
		LegalPerson     MerchantSubjectLegalPerson     `json:"legal_person"`
	}
	require.NoError(t, json.Unmarshal(versionArg.Snapshot, &versionSnapshot))
	require.Equal(t, "91330100MANUAL0001", versionSnapshot.BusinessLicense.Number)
	require.Equal(t, "人工修正后的餐饮店", versionSnapshot.BusinessLicense.Name)
	require.Equal(t, "李四", versionSnapshot.LegalPerson.Name)
	require.Equal(t, "110101199001010099", versionSnapshot.LegalPerson.IDNumber)
}

func TestMerchantSubjectProfileService_LoadsProfiles(t *testing.T) {
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	row := db.MerchantSubjectProfile{
		ID:                          501,
		MerchantApplicationID:       101,
		MerchantID:                  pgtype.Int8{Int64: 66, Valid: true},
		UserID:                      202,
		BusinessLicenseNumber:       "91330100MANUAL0001",
		BusinessLicenseName:         "人工修正后的餐饮店",
		BusinessLicenseAddress:      "杭州市西湖区修正路1号",
		LegalPersonName:             "李四",
		LegalPersonIDNumber:         "110101199001010099",
		FoodPermitNumber:            "JY11111111111111",
		FoodPermitCompanyName:       "人工修正后的餐饮店",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 11, Valid: true},
		FoodPermitMediaAssetID:      pgtype.Int8{Int64: 12, Valid: true},
		IDCardFrontMediaAssetID:     pgtype.Int8{Int64: 13, Valid: true},
		IDCardBackMediaAssetID:      pgtype.Int8{Int64: 14, Valid: true},
		BusinessLicensePayload:      []byte(`{"business_scope":"餐饮服务","valid_period":"2020年01月01日至2040年01月01日"}`),
		FoodPermitPayload:           []byte(`{"operator_name":"李四","valid_to":"2030年12月31日"}`),
		LegalPersonPayload:          []byte(`{"id_card_valid_date":"2020.01.01-2035.01.01"}`),
		SourceSnapshot:              []byte(`{"source":"test"}`),
		Version:                     2,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}
	service := NewMerchantSubjectProfileService(merchantSubjectProfileStoreStub{
		getByApplicationFn: func(_ context.Context, applicationID int64) (db.MerchantSubjectProfile, error) {
			require.Equal(t, int64(101), applicationID)
			return row, nil
		},
		getByMerchantFn: func(_ context.Context, merchantID pgtype.Int8) (db.MerchantSubjectProfile, error) {
			require.Equal(t, int64(66), merchantID.Int64)
			require.True(t, merchantID.Valid)
			return row, nil
		},
	})

	byApp, found, err := service.GetByApplication(context.Background(), 101)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "人工修正后的餐饮店", byApp.BusinessLicense.Name)

	byMerchant, found, err := service.GetByMerchant(context.Background(), 66)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "91330100MANUAL0001", byMerchant.BusinessLicense.Number)
}
