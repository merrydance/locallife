package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 运营商开户测试辅助函数 ====================

// randomOperatorForApplyment 创建随机运营商（进件测试专用）
func randomOperatorForApplyment(userID int64) db.Operator {
	return db.Operator{
		ID:           util.RandomInt(1, 1000),
		UserID:       userID,
		RegionID:     util.RandomInt(1, 100),
		Name:         util.RandomString(10),
		ContactName:  util.RandomString(6),
		ContactPhone: "13800138000",
		Status:       "active", // 入驻审批通过后直接 active，绑卡为可选后置步骤
		CreatedAt:    time.Now(),
	}
}

// randomOperatorApplicationForApplyment 创建随机运营商申请（进件测试专用）
func randomOperatorApplicationForApplyment(userID int64) db.OperatorApplication {
	return db.OperatorApplication{
		ID:                          util.RandomInt(1, 1000),
		UserID:                      userID,
		RegionID:                    util.RandomInt(1, 100),
		Name:                        pgtype.Text{String: util.RandomString(10), Valid: true},
		ContactName:                 pgtype.Text{String: util.RandomString(6), Valid: true},
		ContactPhone:                pgtype.Text{String: "13800138000", Valid: true},
		BusinessLicenseMediaAssetID: pgtype.Int8{},
		BusinessLicenseNumber:       pgtype.Text{String: util.RandomString(18), Valid: true},
		BusinessLicenseOcr:          []byte(`{"type_of_enterprise":"有限责任公司","address":"广州市天河区","valid_period":"2020年01月01日至2040年01月01日"}`),
		LegalPersonName:             pgtype.Text{String: util.RandomString(6), Valid: true},
		LegalPersonIDNumber:         pgtype.Text{String: "110101199001011234", Valid: true},
		IDCardFrontMediaAssetID:     pgtype.Int8{},
		IDCardBackMediaAssetID:      pgtype.Int8{},
		IDCardBackOcr:               []byte(`{"valid_start": "20200101", "valid_end": "20300101"}`),
		RequestedContractYears:      3,
		Status:                      "approved",
	}
}

// ==================== 运营商开户测试 ====================

func TestOperatorBindBankAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperatorForApplyment(user.ID)
	application := randomOperatorApplicationForApplyment(user.ID)

	applicationWithTestURL := application

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface)
		prepareServer func(t *testing.T, server *Server)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_WithEcommerceClient",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_BUSINESS",
				"account_bank":      "其他银行",
				"account_bank_code": 1099,
				"bank_alias":        "深圳前海微众银行",
				"bank_alias_code":   "1000009561",
				"need_bank_branch":  true,
				"bank_address_code": "440300",
				"bank_branch_id":    "402584040001",
				"bank_name":         "深圳前海微众银行深圳南山支行",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				// 获取运营商
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				// 检查是否有进行中的申请
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				// 获取运营商申请信息
				store.EXPECT().
					GetApprovedOperatorApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(applicationWithTestURL, nil)

				// 创建进件记录
				store.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.CreateEcommerceApplymentParams) (db.EcommerceApplyment, error) {
						require.Equal(t, "2", arg.OrganizationType)
						require.Equal(t, "其他银行", arg.AccountBank)
						require.True(t, arg.AccountBankCode.Valid)
						require.Equal(t, int64(1099), arg.AccountBankCode.Int64)
						require.True(t, arg.BankAlias.Valid)
						require.Equal(t, "深圳前海微众银行", arg.BankAlias.String)
						require.True(t, arg.BankAliasCode.Valid)
						require.Equal(t, "1000009561", arg.BankAliasCode.String)
						require.True(t, arg.BankBranchID.Valid)
						require.Equal(t, "402584040001", arg.BankBranchID.String)
						require.Equal(t, applicationWithTestURL.ContactPhone.String, arg.MobilePhone)
						require.False(t, arg.ContactEmail.Valid)
						return randomEcommerceApplymentForTest("operator", operator.ID), nil
					})

				// Mock 加密
				ecommerceClient.EXPECT().
					EncryptSensitiveData(gomock.Any()).
					Times(7).
					Return("encrypted_data", nil)

				// Mock 提交进件
				ecommerceClient.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, req *wechat.EcommerceApplymentRequest) (*wechat.EcommerceApplymentResponse, error) {
						require.Equal(t, "2", req.OrganizationType)
						require.NotNil(t, req.AccountInfo)
						require.Equal(t, "其他银行", req.AccountInfo.AccountBank)
						require.Equal(t, int64(1099), req.AccountInfo.AccountBankCode)
						require.Equal(t, "402584040001", req.AccountInfo.BankBranchID)
						require.NotNil(t, req.IDCardInfo)
						require.Equal(t, "2020-01-01", req.IDCardInfo.IDCardValidTimeBegin)
						require.Equal(t, "2030-01-01", req.IDCardInfo.IDCardValidTime)
						require.NotNil(t, req.ContactInfo)
						require.Equal(t, "encrypted_data", req.ContactInfo.MobilePhone)
						require.NotNil(t, req.SalesSceneInfo)
						require.Equal(t, "https://merchant.example.com", req.SalesSceneInfo.StoreURL)
						return &wechat.EcommerceApplymentResponse{ApplymentID: 123456789}, nil
					})

				// 更新进件状态
				store.EXPECT().
					UpdateEcommerceApplymentToSubmitted(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, nil)

				// 更新运营商状态
				store.EXPECT().
					UpdateOperatorStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Operator{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response operatorBindBankResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, int64(123456789), response.ApplymentID)
				require.Equal(t, "submitted", response.Status)
			},
		},
		{
			name: "OK_WithSuperAdministratorContact",
			body: gin.H{
				"account_type":                      "ACCOUNT_TYPE_BUSINESS",
				"account_bank":                      "其他银行",
				"account_bank_code":                 1099,
				"bank_alias":                        "深圳前海微众银行",
				"bank_alias_code":                   "1000009561",
				"need_bank_branch":                  true,
				"bank_address_code":                 "440300",
				"bank_branch_id":                    "402584040001",
				"bank_name":                         "深圳前海微众银行深圳南山支行",
				"account_number":                    "6214830012345678",
				"account_name":                      "张三",
				"contact_type":                      "SUPER",
				"contact_name":                      "王五",
				"contact_id_doc_type":               "IDENTIFICATION_TYPE_IDCARD",
				"contact_id_card_number":            "110101199303034567",
				"contact_id_doc_copy_asset_id":      601,
				"contact_id_doc_copy_back_asset_id": 602,
				"contact_id_doc_period_begin":       "2020-01-01",
				"contact_id_doc_period_end":         "2030-01-01",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(601)).
					AnyTimes().
					Return(db.MediaAsset{
						ID:               601,
						ObjectKey:        "id_card/front/1/20240101/operator-contact-front.png",
						Visibility:       string(media.VisibilityPrivate),
						MediaCategory:    string(media.CategoryIDCardFront),
						MimeType:         "image/png",
						UploadedBy:       user.ID,
						UploadStatus:     "confirmed",
						ModerationStatus: "approved",
					}, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(602)).
					AnyTimes().
					Return(db.MediaAsset{
						ID:               602,
						ObjectKey:        "id_card/back/1/20240101/operator-contact-back.png",
						Visibility:       string(media.VisibilityPrivate),
						MediaCategory:    string(media.CategoryIDCardBack),
						MimeType:         "image/png",
						UploadedBy:       user.ID,
						UploadStatus:     "confirmed",
						ModerationStatus: "approved",
					}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetApprovedOperatorApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(applicationWithTestURL, nil)

				store.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.CreateEcommerceApplymentParams) (db.EcommerceApplyment, error) {
						require.Equal(t, "王五", arg.ContactName)
						require.True(t, arg.ContactIDCardNumber.Valid)
						require.NotEmpty(t, arg.ContactIDCardNumber.String)
						return randomEcommerceApplymentForTest("operator", operator.ID), nil
					})

				ecommerceClient.EXPECT().
					EncryptSensitiveData(gomock.Any()).
					Times(7).
					Return("encrypted_data", nil)

				ecommerceClient.EXPECT().
					UploadImage(gomock.Any(), "operator-contact-front.png", gomock.Any()).
					Times(1).
					Return(&wechat.ImageUploadResponse{MediaID: "wx_operator_super_contact_front_media_id"}, nil)
				ecommerceClient.EXPECT().
					UploadImage(gomock.Any(), "operator-contact-back.png", gomock.Any()).
					Times(1).
					Return(&wechat.ImageUploadResponse{MediaID: "wx_operator_super_contact_back_media_id"}, nil)

				ecommerceClient.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, req *wechat.EcommerceApplymentRequest) (*wechat.EcommerceApplymentResponse, error) {
						require.NotNil(t, req.ContactInfo)
						require.Equal(t, "SUPER", req.ContactInfo.ContactType)
						require.Equal(t, "encrypted_data", req.ContactInfo.ContactName)
						require.Equal(t, "IDENTIFICATION_TYPE_IDCARD", req.ContactInfo.ContactIDDocType)
						require.Equal(t, "encrypted_data", req.ContactInfo.ContactIDCardNumber)
						require.Equal(t, "wx_operator_super_contact_front_media_id", req.ContactInfo.ContactIDDocCopy)
						require.Equal(t, "wx_operator_super_contact_back_media_id", req.ContactInfo.ContactIDDocCopyBack)
						require.Equal(t, "2020-01-01", req.ContactInfo.ContactIDDocPeriodBegin)
						require.Equal(t, "2030-01-01", req.ContactInfo.ContactIDDocPeriodEnd)
						return &wechat.EcommerceApplymentResponse{ApplymentID: 99887766}, nil
					})

				store.EXPECT().
					UpdateEcommerceApplymentToSubmitted(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, nil)

				store.EXPECT().
					UpdateOperatorStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Operator{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response operatorBindBankResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, int64(99887766), response.ApplymentID)
				require.Equal(t, "submitted", response.Status)
			},
			prepareServer: func(t *testing.T, server *Server) {
				content := buildTestQRCodePNG(t)
				seedPrivateContactDocumentAsset(t, server, "id_card/front/1/20240101/operator-contact-front.png", content)
				seedPrivateContactDocumentAsset(t, server, "id_card/back/1/20240101/operator-contact-back.png", content)
			},
		},
		{
			name: "OK_WithRangeStoredInValidEnd",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_BUSINESS",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"bank_name":         "招商银行深圳分行",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				rangeInEndApplication := applicationWithTestURL
				rangeInEndApplication.IDCardBackOcr = []byte(`{"valid_start":"","valid_end":"2008.09.29-2028.09.29"}`)
				store.EXPECT().
					GetApprovedOperatorApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rangeInEndApplication, nil)

				store.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					Return(randomEcommerceApplymentForTest("operator", operator.ID), nil)

				ecommerceClient.EXPECT().
					EncryptSensitiveData(gomock.Any()).
					Times(7).
					Return("encrypted_data", nil)

				ecommerceClient.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, req *wechat.EcommerceApplymentRequest) (*wechat.EcommerceApplymentResponse, error) {
						require.Equal(t, "2008-09-29", req.IDCardInfo.IDCardValidTimeBegin)
						require.Equal(t, "2028-09-29", req.IDCardInfo.IDCardValidTime)
						require.NotNil(t, req.ContactInfo)
						require.Equal(t, "encrypted_data", req.ContactInfo.MobilePhone)
						return &wechat.EcommerceApplymentResponse{ApplymentID: 123456789}, nil
					})

				store.EXPECT().
					UpdateEcommerceApplymentToSubmitted(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, nil)

				store.EXPECT().
					UpdateOperatorStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Operator{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response operatorBindBankResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, int64(123456789), response.ApplymentID)
				require.Equal(t, "submitted", response.Status)
			},
		},
		{
			name: "PersonalOperatorDoesNotNeedApplyment",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				personalApplication := applicationWithTestURL
				personalApplication.BusinessLicenseNumber = pgtype.Text{}
				personalApplication.BusinessLicenseOcr = nil
				store.EXPECT().
					GetApprovedOperatorApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(personalApplication, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrOperatorPersonalApplymentUnsupported.Code, response.Code)
				require.Equal(t, ErrOperatorPersonalApplymentUnsupported.Message, response.Error)
			},
		},
		{
			name: "EnterpriseOperatorRequiresBusinessAccount",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetApprovedOperatorApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(applicationWithTestURL, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrApplymentEnterprisePublicAccountRequired.Code, response.Code)
				require.Equal(t, ErrApplymentEnterprisePublicAccountRequired.Message, response.Error)
			},
		},
		{
			name: "OperatorNotFound",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(db.Operator{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "InvalidOperatorStatus",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				invalidOperator := operator
				invalidOperator.Status = "pending"
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(invalidOperator, nil)
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "AlreadyHasPendingApplyment",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				existingApplyment := randomEcommerceApplymentForTest("operator", operator.ID)
				existingApplyment.Status = "auditing"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existingApplyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "AlreadyHasPendingApplyment_AccountNeedVerify",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				existingApplyment := randomEcommerceApplymentForTest("operator", operator.ID)
				existingApplyment.Status = "account_need_verify"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existingApplyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "AlreadyHasPendingApplyment_Checking",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				existingApplyment := randomEcommerceApplymentForTest("operator", operator.ID)
				existingApplyment.Status = "checking"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existingApplyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "AlreadyHasPendingApplyment_ToBeConfirmed",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				existingApplyment := randomEcommerceApplymentForTest("operator", operator.ID)
				existingApplyment.Status = "to_be_confirmed"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existingApplyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "AlreadyFinished",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				existingApplyment := randomEcommerceApplymentForTest("operator", operator.ID)
				existingApplyment.Status = "finish"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existingApplyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidIDCardValidityPeriod",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_BUSINESS",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				invalidApplication := applicationWithTestURL
				invalidApplication.IDCardBackOcr = []byte(`{"valid_end": "20300101"}`)
				store.EXPECT().
					GetApprovedOperatorApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(invalidApplication, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrApplymentIDCardValidityInvalid.Code, response.Code)
				require.Equal(t, ErrApplymentIDCardValidityInvalid.Message, response.Error)
			},
		},
		{
			name: "InvalidBusinessLicenseValidityPeriod",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_BUSINESS",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				invalidApplication := applicationWithTestURL
				invalidApplication.BusinessLicenseOcr = []byte(`{"type_of_enterprise":"有限责任公司","address":"广州市天河区","valid_period":"无效文本"}`)
				store.EXPECT().
					GetApprovedOperatorApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(invalidApplication, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrApplymentBusinessLicenseValidityInvalid.Code, response.Code)
				require.Equal(t, ErrApplymentBusinessLicenseValidityInvalid.Message, response.Error)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "InvalidRequestBody",
			body: gin.H{
				"account_type": "INVALID_TYPE",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, ecommerceClient)

			server := newTestServerWithEcommerce(t, store, ecommerceClient)
			if tc.prepareServer != nil {
				tc.prepareServer(t, server)
			}

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/operator/applyment/bindbank"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ==================== 运营商开户状态查询测试 ====================

func TestGetOperatorApplymentStatusAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperatorForApplyment(user.ID)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_Pending",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				testOperator := operator
				testOperator.Status = "active"

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(testOperator, nil)

				applyment := randomEcommerceApplymentForTest("operator", operator.ID)
				applyment.Status = "pending"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response operatorApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "pending", response.Status)
				require.Equal(t, "待提交", response.StatusDesc)
				require.True(t, response.CanSubmit)
			},
		},
		{
			name: "OK_Finished",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				applyment := randomEcommerceApplymentForTest("operator", operator.ID)
				applyment.Status = "finish"
				applyment.SubMchID = pgtype.Text{String: "1234567890", Valid: true}
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response operatorApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "finish", response.Status)
				require.NotNil(t, response.SubMchID)
				require.False(t, response.CanSubmit)
			},
		},
		{
			name: "FinishWithoutSubMch_ShouldFallbackToSubmitted",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				applyment := randomEcommerceApplymentForTest("operator", operator.ID)
				applyment.Status = "finish"
				applyment.SubMchID = pgtype.Text{Valid: false}
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response operatorApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "submitted", response.Status)
				require.Empty(t, response.SubMchID)
				require.False(t, response.CanSubmit)
			},
		},
		{
			name: "FinishWithoutApplymentSubMch_UsesOperatorSubMch",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				testOperator := operator
				testOperator.SubMchID = pgtype.Text{String: "1900001234", Valid: true}

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(testOperator, nil)

				applyment := randomEcommerceApplymentForTest("operator", operator.ID)
				applyment.Status = "finish"
				applyment.SubMchID = pgtype.Text{Valid: false}
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response operatorApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "finish", response.Status)
				require.Equal(t, "1900001234", response.SubMchID)
				require.False(t, response.CanSubmit)
			},
		},
		{
			name: "NoApplyment",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetApprovedOperatorApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(randomOperatorApplicationForApplyment(user.ID), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response operatorApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "active", response.Status)
				require.Equal(t, "可提交开户信息", response.StatusDesc)
				require.True(t, response.CanSubmit)
			},
		},
		{
			name: "NoApplyment_PersonalOperator",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				personalApplication := randomOperatorApplicationForApplyment(user.ID)
				personalApplication.BusinessLicenseNumber = pgtype.Text{}
				personalApplication.BusinessLicenseOcr = nil
				store.EXPECT().
					GetApprovedOperatorApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(personalApplication, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response operatorApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "active", response.Status)
				require.Equal(t, "当前无需提交开户信息", response.StatusDesc)
				require.False(t, response.CanSubmit)
				require.Equal(t, getPersonalOperatorApplymentBlockReason(), response.BlockReason)
			},
		},
		{
			name: "NoApplyment_SuspendedOperator",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				testOperator := operator
				testOperator.Status = "suspended"

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(testOperator, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response operatorApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "frozen", response.Status)
				require.Equal(t, "当前账号状态不可用", response.StatusDesc)
				require.False(t, response.CanSubmit)
				require.Equal(t, "当前运营商状态不可用，暂不支持提交微信支付开户。", response.BlockReason)
			},
		},
		{
			name: "OperatorNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(db.Operator{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)

			url := "/v1/operator/applyment/status"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestGetOperatorApplymentStatusAPI_QueryBackfillsSubMchIDWhenStatusUnchanged(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperatorForApplyment(user.ID)
	applyment := randomEcommerceApplymentForTest("operator", operator.ID)
	applyment.Status = "auditing"
	applyment.ApplymentID = pgtype.Int8{Int64: 123456789, Valid: true}
	applyment.SubMchID = pgtype.Text{}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(1).
		Return(operator, nil)

	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(applyment, nil)

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByID(gomock.Any(), applyment.ApplymentID.Int64).
		Times(1).
		Return(&wechat.EcommerceApplymentQueryResponse{
			ApplymentID:    applyment.ApplymentID.Int64,
			ApplymentState: "AUDITING",
			SubMchID:       "1900005678",
		}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentStatus(gomock.Any(), db.UpdateEcommerceApplymentStatusParams{
			ID:           applyment.ID,
			Status:       "auditing",
			RejectReason: pgtype.Text{},
			SignUrl:      pgtype.Text{},
			SignState:    pgtype.Text{},
			SubMchID:     pgtype.Text{String: "1900005678", Valid: true},
		}).
		Times(1).
		Return(applyment, nil)

	server := newTestServerWithEcommerce(t, store, ecommerceClient)

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/applyment/status", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response operatorApplymentStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "auditing", response.Status)
	require.Equal(t, "1900005678", response.SubMchID)
	require.False(t, response.CanSubmit)
}

func TestGetOperatorApplymentStatusAPI_RequeriesLocalFinishStatus(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperatorForApplyment(user.ID)
	applyment := randomEcommerceApplymentForTest("operator", operator.ID)
	applyment.Status = "finish"
	applyment.ApplymentID = pgtype.Int8{Int64: 22334455, Valid: true}
	applyment.SubMchID = pgtype.Text{String: "1900007788", Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(1).
		Return(operator, nil)

	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(applyment, nil)

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByID(gomock.Any(), applyment.ApplymentID.Int64).
		Times(1).
		Return(&wechat.EcommerceApplymentQueryResponse{
			ApplymentID:    applyment.ApplymentID.Int64,
			ApplymentState: "ACCOUNT_NEED_VERIFY",
			SubMchID:       "1900007788",
		}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentStatus(gomock.Any(), db.UpdateEcommerceApplymentStatusParams{
			ID:           applyment.ID,
			Status:       "account_need_verify",
			RejectReason: pgtype.Text{},
			SignUrl:      pgtype.Text{},
			SignState:    pgtype.Text{},
			SubMchID:     pgtype.Text{String: "1900007788", Valid: true},
		}).
		Times(1).
		Return(applyment, nil)

	server := newTestServerWithEcommerce(t, store, ecommerceClient)

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/applyment/status", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response operatorApplymentStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "account_need_verify", response.Status)
	require.Equal(t, "1900007788", response.SubMchID)
	require.False(t, response.CanSubmit)
}

func TestGetOperatorApplymentStatusAPI_QueryByOutRequestNoWhenApplymentIDMissing(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperatorForApplyment(user.ID)
	operator.Status = "bindbank_submitted"
	applyment := randomEcommerceApplymentForTest("operator", operator.ID)
	applyment.Status = "pending"
	applyment.ApplymentID = pgtype.Int8{}
	applyment.OutRequestNo = "OP_APPLYMENT_001"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(1).
		Return(operator, nil)

	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(applyment, nil)

	ecommerceClient.EXPECT().
		QueryEcommerceApplymentByOutRequestNo(gomock.Any(), applyment.OutRequestNo).
		Times(1).
		Return(&wechat.EcommerceApplymentQueryResponse{
			ApplymentID:    987654321,
			OutRequestNo:   applyment.OutRequestNo,
			ApplymentState: "AUDITING",
		}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentStatus(gomock.Any(), db.UpdateEcommerceApplymentStatusParams{
			ID:           applyment.ID,
			Status:       "auditing",
			RejectReason: pgtype.Text{},
			SignUrl:      pgtype.Text{},
			SignState:    pgtype.Text{},
			SubMchID:     pgtype.Text{},
		}).
		Times(1).
		Return(applyment, nil)

	server := newTestServerWithEcommerce(t, store, ecommerceClient)

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/applyment/status", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response operatorApplymentStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "auditing", response.Status)
	require.NotNil(t, response.ApplymentID)
	require.EqualValues(t, 987654321, *response.ApplymentID)
	require.False(t, response.CanSubmit)
}

func TestOperatorBindBankSubmittedStateSyncFailed(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperatorForApplyment(user.ID)
	application := randomOperatorApplicationForApplyment(user.ID)
	applyment := randomEcommerceApplymentForTest("operator", operator.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(1).
		Return(operator, nil)

	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

	store.EXPECT().
		GetApprovedOperatorApplicationByUserID(gomock.Any(), user.ID).
		Times(1).
		Return(application, nil)

	store.EXPECT().
		CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
		Times(1).
		Return(applyment, nil)

	ecommerceClient.EXPECT().
		EncryptSensitiveData(gomock.Any()).
		Times(7).
		Return("encrypted_data", nil)

	ecommerceClient.EXPECT().
		CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&wechat.EcommerceApplymentResponse{ApplymentID: 123456789}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentToSubmitted(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, fmt.Errorf("update submitted failed"))

	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{
			ID:     operator.ID,
			Status: "bindbank_submitted",
		}).
		Times(1).
		Return(db.Operator{}, nil)

	server := newTestServerWithEcommerce(t, store, ecommerceClient)

	body := gin.H{
		"account_type":      "ACCOUNT_TYPE_BUSINESS",
		"account_bank":      "招商银行",
		"bank_address_code": "440300",
		"account_number":    "6214830012345678",
		"account_name":      "张三",
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/operator/applyment/bindbank", bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestOperatorBindBankRejectsRecoveringPendingApplyment(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperatorForApplyment(user.ID)
	operator.Status = "bindbank_submitted"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(1).
		Return(operator, nil)

	existingApplyment := randomEcommerceApplymentForTest("operator", operator.ID)
	existingApplyment.Status = "pending"
	existingApplyment.OutRequestNo = "OPERATOR_APPLYMENT_RECOVER_001"
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(existingApplyment, nil)

	server := newTestServer(t, store)

	body := gin.H{
		"account_type":      "ACCOUNT_TYPE_BUSINESS",
		"account_bank":      "招商银行",
		"bank_address_code": "440300",
		"account_number":    "6214830012345678",
		"account_name":      "张三",
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/operator/applyment/bindbank", bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
