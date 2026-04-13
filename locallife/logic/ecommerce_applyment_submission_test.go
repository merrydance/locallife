package logic

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type testApplymentAssetDownloader struct {
	filename string
	fileData []byte
	err      error
	assetID  int64
}

func (t *testApplymentAssetDownloader) DownloadObject(_ context.Context, assetID int64) (string, []byte, error) {
	t.assetID = assetID
	if t.err != nil {
		return "", nil, t.err
	}
	return t.filename, t.fileData, nil
}

type testApplymentImageUploader struct {
	mediaID  string
	err      error
	filename string
	fileData []byte
}

func (t *testApplymentImageUploader) UploadImage(_ context.Context, filename string, fileData []byte) (*wechat.ImageUploadResponse, error) {
	t.filename = filename
	t.fileData = append([]byte(nil), fileData...)
	if t.err != nil {
		return nil, t.err
	}
	return &wechat.ImageUploadResponse{MediaID: t.mediaID}, nil
}

type testApplymentSensitiveEncryptor struct {
	outputs []string
	errAt   int
	err     error
	inputs  []string
}

func (t *testApplymentSensitiveEncryptor) EncryptSensitiveData(plaintext string) (string, error) {
	t.inputs = append(t.inputs, plaintext)
	callIndex := len(t.inputs)
	if t.errAt > 0 && callIndex == t.errAt {
		return "", t.err
	}
	if len(t.outputs) >= callIndex {
		return t.outputs[callIndex-1], nil
	}
	return fmt.Sprintf("enc:%s", plaintext), nil
}

func TestIsApplymentSubmissionInFlight(t *testing.T) {
	require.True(t, IsApplymentSubmissionInFlight("submitted", "active", ""))
	require.True(t, IsApplymentSubmissionInFlight("pending", "bindbank_submitted", "APPLYMENT_RECOVER_001"))
	require.False(t, IsApplymentSubmissionInFlight("pending", "active", "APPLYMENT_RECOVER_001"))
}

func TestValidateMerchantApplymentSubmissionState(t *testing.T) {
	existing := &db.EcommerceApplyment{Status: "submitted", OutRequestNo: "OUT_001"}
	require.NoError(t, ValidateMerchantApplymentSubmissionState("approved", nil))
	require.True(t, errors.Is(ValidateMerchantApplymentSubmissionState("disabled", nil), ErrMerchantApplymentSubmissionStatusInvalid))
	require.True(t, errors.Is(ValidateMerchantApplymentSubmissionState("pending_bindbank", existing), ErrApplymentSubmissionPending))
	require.True(t, errors.Is(ValidateMerchantApplymentSubmissionState("approved", &db.EcommerceApplyment{Status: "finish"}), ErrApplymentAlreadyRegistered))
}

func TestBuildCreateEcommerceApplymentParams(t *testing.T) {
	params := BuildCreateEcommerceApplymentParams(ApplymentLocalRecordInput{
		SubjectType:           "merchant",
		SubjectID:             42,
		OutRequestNo:          "OUT_003",
		OrganizationType:      "2",
		BusinessLicenseNumber: "BLN001",
		BusinessLicenseCopy:   "https://cdn.test/license.png",
		MerchantName:          "主体名称",
		LegalPerson:           "张三",
		IDCardNumber:          "encrypted-id",
		IDCardName:            "张三",
		IDCardValidTime:       "2030-01-01",
		IDCardFrontCopy:       "front.png",
		IDCardBackCopy:        "back.png",
		AccountType:           "ACCOUNT_TYPE_BUSINESS",
		AccountBank:           "招商银行",
		AccountBankCode:       1001,
		BankAlias:             "招商银行",
		BankAliasCode:         "1000001",
		BankAddressCode:       "440300",
		BankBranchID:          "4025",
		BankName:              "招商银行深圳分行",
		AccountNumber:         "encrypted-account",
		AccountName:           "张三",
		ContactName:           "李四",
		ContactIDCardNumber:   "encrypted-contact-id",
		MobilePhone:           "13800138000",
		MerchantShortname:     "主体简称",
	})

	require.Equal(t, int64(42), params.SubjectID)
	require.Equal(t, "OUT_003", params.OutRequestNo)
	require.True(t, params.BusinessLicenseNumber.Valid)
	require.Equal(t, int64(1001), params.AccountBankCode.Int64)
	require.True(t, params.ContactIDCardNumber.Valid)
	require.Equal(t, []byte("[]"), params.Qualifications)
	require.Empty(t, params.BusinessAdditionPics)
}

func TestBuildWechatApplymentRequest(t *testing.T) {
	request := BuildWechatApplymentRequest(ApplymentWechatRequestInput{
		OutRequestNo:      "OUT_004",
		OrganizationType:  "2",
		BusinessLicense:   &wechat.BusinessLicenseInfo{BusinessLicenseNumber: "BLN001"},
		MerchantShortname: "主体简称",
		IDCardInfo: &wechat.ApplymentIDCardInfo{
			IDCardCopy:           "front-media",
			IDCardNational:       "back-media",
			IDCardName:           "encrypted-name",
			IDCardNumber:         "encrypted-number",
			IDCardValidTimeBegin: "2020-01-01",
			IDCardValidTime:      "2030-01-01",
		},
		AccountInfo: ApplymentWechatAccountInput{
			AccountType:     "ACCOUNT_TYPE_BUSINESS",
			AccountBank:     "招商银行",
			AccountBankCode: 1001,
			AccountName:     "encrypted-account-name",
			BankAddressCode: "440300",
			BankBranchID:    "4025",
			BankName:        "招商银行深圳分行",
			AccountNumber:   "encrypted-account-number",
		},
		ContactInfo: ApplymentWechatContactInput{
			ContactType:             "SUPER",
			ContactName:             "encrypted-contact-name",
			ContactIDDocType:        "IDENTIFICATION_TYPE_IDCARD",
			ContactIDCardNumber:     "encrypted-contact-id",
			ContactIDDocPeriodBegin: "2020-01-01",
			ContactIDDocPeriodEnd:   "2030-01-01",
			ContactIDDocCopy:        "contact-front",
			ContactIDDocCopyBack:    "contact-back",
			MobilePhone:             "encrypted-phone",
		},
		StoreName:   "门店名",
		StoreQRCode: "store-qr-media",
	})

	require.Equal(t, "OUT_004", request.OutRequestNo)
	require.Equal(t, "2", request.OrganizationType)
	require.Equal(t, "招商银行", request.AccountInfo.AccountBank)
	require.Equal(t, "SUPER", request.ContactInfo.ContactType)
	require.Equal(t, "contact-front", request.ContactInfo.ContactIDDocCopy)
	require.NotNil(t, request.SalesSceneInfo)
	require.Equal(t, "门店名", request.SalesSceneInfo.StoreName)
	require.Equal(t, "store-qr-media", request.SalesSceneInfo.StoreQRCode)
}

func TestBuildApplymentBusinessTime(t *testing.T) {
	t.Parallel()

	require.Empty(t, BuildApplymentBusinessTime("长期"))
	require.Empty(t, BuildApplymentBusinessTime("永久有效"))
	require.Equal(t, `["2020-01-01","长期"]`, BuildApplymentBusinessTime("2020年01月01日至长期"))
}

func TestResolveApplymentOrganizationType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		businessLicenseNum string
		licenseType        string
		subjectName        string
		defaultType        string
		want               string
	}{
		{name: "NoBusinessLicenseFallsBackToMicro", businessLicenseNum: "", defaultType: "4", want: "2401"},
		{name: "IndividualLicenseUses4", businessLicenseNum: "91440300TEST123456", licenseType: "个体工商户", defaultType: "4", want: "4"},
		{name: "EnterpriseLicenseUses2", businessLicenseNum: "91440300TEST123456", licenseType: "有限责任公司", defaultType: "4", want: "2"},
		{name: "InstitutionUses3", businessLicenseNum: "91440300TEST123456", licenseType: "事业单位法人", defaultType: "4", want: "3"},
		{name: "GovernmentUses2502", businessLicenseNum: "91440300TEST123456", licenseType: "政府机关", defaultType: "4", want: "2502"},
		{name: "SocialOrganizationUses1708", businessLicenseNum: "91440300TEST123456", licenseType: "社会团体", defaultType: "4", want: "1708"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveApplymentOrganizationType(tc.businessLicenseNum, tc.licenseType, tc.subjectName, tc.defaultType)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseApplymentIDCardValidPeriod(t *testing.T) {
	begin, end := ParseApplymentIDCardValidPeriod("2020年01月01日至长期")
	require.Equal(t, "2020-01-01", begin)
	require.Equal(t, "长期", end)
}

func TestParseApplymentOperatorIDCardValidPeriod(t *testing.T) {
	begin, end := ParseApplymentOperatorIDCardValidPeriod("", "2008.09.29-2028.09.29")
	require.Equal(t, "2008-09-29", begin)
	require.Equal(t, "2028-09-29", end)
}

func TestBuildApplymentBusinessLicenseInfo(t *testing.T) {
	info := BuildApplymentBusinessLicenseInfo("license-media", "BLN001", "主体名称", "张三", "深圳市南山区", ApplymentBusinessLicenseOCRInput{
		Address:     "广州市天河区",
		ValidPeriod: "2020年01月01日至长期",
	})

	require.NotNil(t, info)
	require.Equal(t, "license-media", info.BusinessLicenseCopy)
	require.Equal(t, "广州市天河区", info.CompanyAddress)
	require.Equal(t, `["2020-01-01","长期"]`, info.BusinessTime)
}

func TestBuildApplymentStoreURL(t *testing.T) {
	require.Equal(t, "https://merchant.example.com", BuildApplymentStoreURL("https://merchant.example.com/", ""))
	require.Equal(t, "https://external.example.com", BuildApplymentStoreURL("", "https://external.example.com/"))
	require.Empty(t, BuildApplymentStoreURL("", ""))
}

func TestUploadApplymentAsset(t *testing.T) {
	downloader := &testApplymentAssetDownloader{
		filename: "id-card-front.png",
		fileData: []byte("png-data"),
	}
	uploader := &testApplymentImageUploader{mediaID: "media_123"}

	mediaID, err := UploadApplymentAsset(context.Background(), downloader, uploader, 99)

	require.NoError(t, err)
	require.Equal(t, int64(99), downloader.assetID)
	require.Equal(t, "id-card-front.png", uploader.filename)
	require.Equal(t, []byte("png-data"), uploader.fileData)
	require.Equal(t, "media_123", mediaID)
}

func TestUploadApplymentAssetEmptyMediaID(t *testing.T) {
	downloader := &testApplymentAssetDownloader{
		filename: "id-card-front.png",
		fileData: []byte("png-data"),
	}
	uploader := &testApplymentImageUploader{}

	_, err := UploadApplymentAsset(context.Background(), downloader, uploader, 99)

	require.Error(t, err)
	require.ErrorContains(t, err, "empty media id")
}

func TestEncryptApplymentWechatSensitiveFields(t *testing.T) {
	encryptor := &testApplymentSensitiveEncryptor{}

	output, err := EncryptApplymentWechatSensitiveFields(encryptor, ApplymentWechatSensitiveInput{
		IDCardName:          "张三",
		IDCardNumber:        "110101199001011234",
		ContactName:         "李四",
		ContactIDCardNumber: "110101199002021234",
		AccountName:         "张三",
		AccountNumber:       "6222000000000000",
		MobilePhone:         "13800138000",
	})

	require.NoError(t, err)
	require.Equal(t, []string{"张三", "110101199001011234", "李四", "110101199002021234", "张三", "6222000000000000", "13800138000"}, encryptor.inputs)
	require.Equal(t, "enc:张三", output.IDCardName)
	require.Equal(t, "enc:110101199001011234", output.IDCardNumber)
	require.Equal(t, "enc:李四", output.ContactName)
	require.Equal(t, "enc:110101199002021234", output.ContactIDCardNumber)
	require.Equal(t, "enc:张三", output.AccountName)
	require.Equal(t, "enc:6222000000000000", output.AccountNumber)
	require.Equal(t, "enc:13800138000", output.MobilePhone)
}

func TestEncryptApplymentWechatSensitiveFieldsFailure(t *testing.T) {
	encryptor := &testApplymentSensitiveEncryptor{errAt: 4, err: fmt.Errorf("encrypt failed")}

	_, err := EncryptApplymentWechatSensitiveFields(encryptor, ApplymentWechatSensitiveInput{
		IDCardName:          "张三",
		IDCardNumber:        "110101199001011234",
		ContactName:         "李四",
		ContactIDCardNumber: "110101199002021234",
		AccountName:         "张三",
		AccountNumber:       "6222000000000000",
		MobilePhone:         "13800138000",
	})

	require.Error(t, err)
	var encryptionErr *ApplymentSensitiveEncryptionError
	require.ErrorAs(t, err, &encryptionErr)
	require.Equal(t, "contact_id_card_number", encryptionErr.Field)
	require.ErrorContains(t, err, "encrypt failed")
}

func TestSubmitEcommerceApplymentWithoutEcommerceClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	applyment := db.EcommerceApplyment{ID: util.RandomInt(1, 1000)}
	updatedStatus := ""

	result, err := SubmitEcommerceApplyment(context.Background(), store, nil, func(_ context.Context, status string) error {
		updatedStatus = status
		return nil
	}, SubmitEcommerceApplymentInput{Applyment: applyment})

	require.NoError(t, err)
	require.Equal(t, ApplymentSubjectStatusBindbankSubmitted, updatedStatus)
	require.Equal(t, applyment.ID, result.ApplymentID)
	require.Equal(t, ApplymentSubmissionResultStatus, result.Status)
	require.Equal(t, ApplymentSubmissionFallbackMessage, result.Message)
}

func TestSubmitEcommerceApplymentSubmittedSyncFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	applyment := db.EcommerceApplyment{ID: util.RandomInt(1, 1000)}
	updatedStatus := ""

	ecommerceClient.EXPECT().
		CreateEcommerceApplyment(gomock.Any(), gomock.AssignableToTypeOf(&wechat.EcommerceApplymentRequest{})).
		Times(1).
		Return(&wechat.EcommerceApplymentResponse{ApplymentID: 123456789}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentToSubmitted(gomock.Any(), db.UpdateEcommerceApplymentToSubmittedParams{
			ID:          applyment.ID,
			ApplymentID: pgtype.Int8{Int64: 123456789, Valid: true},
		}).
		Times(1).
		Return(db.EcommerceApplyment{}, fmt.Errorf("sync failed"))

	_, err := SubmitEcommerceApplyment(context.Background(), store, ecommerceClient, func(_ context.Context, status string) error {
		updatedStatus = status
		return nil
	}, SubmitEcommerceApplymentInput{
		Applyment:     applyment,
		WechatRequest: &wechat.EcommerceApplymentRequest{OutRequestNo: "OUT_REQ_001"},
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "同步微信进件提交状态失败")
	require.Equal(t, ApplymentSubjectStatusBindbankSubmitted, updatedStatus)
}

func TestSubmitEcommerceApplymentReturnsInitialQueryStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	applyment := db.EcommerceApplyment{ID: util.RandomInt(1, 1000), OutRequestNo: "OUT_REQ_002"}
	updatedStatus := ""

	ecommerceClient.EXPECT().
		CreateEcommerceApplyment(gomock.Any(), gomock.AssignableToTypeOf(&wechat.EcommerceApplymentRequest{})).
		Return(&wechat.EcommerceApplymentResponse{ApplymentID: 22334455}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentToSubmitted(gomock.Any(), db.UpdateEcommerceApplymentToSubmittedParams{
			ID:          applyment.ID,
			ApplymentID: pgtype.Int8{Int64: 22334455, Valid: true},
		}).
		Return(db.EcommerceApplyment{}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentStatus(gomock.Any(), db.UpdateEcommerceApplymentStatusParams{
			ID:                 applyment.ID,
			ApplymentID:        pgtype.Int8{Int64: 22334455, Valid: true},
			Status:             "account_need_verify",
			RejectReason:       pgtype.Text{},
			SignUrl:            pgtype.Text{String: "https://wx.example.com/sign", Valid: true},
			SignState:          pgtype.Text{},
			LegalValidationUrl: pgtype.Text{},
			AccountValidation:  nil,
			SubMchID:           pgtype.Text{},
		}).
		Return(db.EcommerceApplyment{}, nil)

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByID(gomock.Any(), int64(22334455)).
		Return(&wechat.EcommerceApplymentQueryResponse{
			ApplymentID:    22334455,
			OutRequestNo:   "OUT_REQ_002",
			ApplymentState: "ACCOUNT_NEED_VERIFY",
			SignURL:        "https://wx.example.com/sign",
		}, nil)

	result, err := SubmitEcommerceApplyment(context.Background(), store, ecommerceClient, func(_ context.Context, status string) error {
		updatedStatus = status
		return nil
	}, SubmitEcommerceApplymentInput{
		Applyment:     applyment,
		WechatRequest: &wechat.EcommerceApplymentRequest{OutRequestNo: "OUT_REQ_002"},
	})

	require.NoError(t, err)
	require.Equal(t, ApplymentSubjectStatusBindbankSubmitted, updatedStatus)
	require.Equal(t, "account_need_verify", result.Status)
	require.Equal(t, "待账户验证", result.StatusDesc)
	require.Equal(t, "待账户验证", result.Message)
	require.NotNil(t, result.InitialQueryResponse)
	require.Equal(t, "https://wx.example.com/sign", result.InitialQueryResponse.SignURL)
}

func TestSubmitEcommerceApplymentFallsBackToOutRequestNoForInitialQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	applyment := db.EcommerceApplyment{ID: util.RandomInt(1, 1000), OutRequestNo: "OUT_REQ_003"}

	ecommerceClient.EXPECT().
		CreateEcommerceApplyment(gomock.Any(), gomock.AssignableToTypeOf(&wechat.EcommerceApplymentRequest{})).
		Return(&wechat.EcommerceApplymentResponse{ApplymentID: 99887766}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentToSubmitted(gomock.Any(), db.UpdateEcommerceApplymentToSubmittedParams{
			ID:          applyment.ID,
			ApplymentID: pgtype.Int8{Int64: 99887766, Valid: true},
		}).
		Return(db.EcommerceApplyment{}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentStatus(gomock.Any(), db.UpdateEcommerceApplymentStatusParams{
			ID:                 applyment.ID,
			ApplymentID:        pgtype.Int8{Int64: 99887766, Valid: true},
			Status:             "to_be_signed",
			RejectReason:       pgtype.Text{},
			SignUrl:            pgtype.Text{},
			SignState:          pgtype.Text{},
			LegalValidationUrl: pgtype.Text{},
			AccountValidation:  nil,
			SubMchID:           pgtype.Text{},
		}).
		Return(db.EcommerceApplyment{}, nil)

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByID(gomock.Any(), int64(99887766)).
		Return(nil, fmt.Errorf("query by id failed"))

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByOutRequestNo(gomock.Any(), "OUT_REQ_003").
		Return(&wechat.EcommerceApplymentQueryResponse{
			ApplymentID:        99887766,
			OutRequestNo:       "OUT_REQ_003",
			ApplymentState:     "NEED_SIGN",
			ApplymentStateDesc: "待签约",
		}, nil)

	result, err := SubmitEcommerceApplyment(context.Background(), store, ecommerceClient, nil, SubmitEcommerceApplymentInput{
		Applyment:     applyment,
		WechatRequest: &wechat.EcommerceApplymentRequest{OutRequestNo: "OUT_REQ_003"},
	})

	require.NoError(t, err)
	require.Equal(t, "to_be_signed", result.Status)
	require.Equal(t, "待签约，请点击签约链接完成签约", result.StatusDesc)
	require.NotNil(t, result.InitialQueryResponse)
}

func TestMapWechatApplymentStateToSubmissionStatus(t *testing.T) {
	require.Equal(t, "to_be_signed", mapWechatApplymentStateToSubmissionStatus("NEED_SIGN"))
	require.Equal(t, "", mapWechatApplymentStateToSubmissionStatus("NEW_UPSTREAM_STATE"))
}
