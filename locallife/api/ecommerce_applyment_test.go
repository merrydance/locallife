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
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 测试辅助函数 ====================

// newTestServerWithEcommerce 创建带 ecommerceClient 的测试服务器
func newTestServerWithEcommerce(t *testing.T, store db.Store, ecommerceClient wechat.EcommerceClientInterface) *Server {
	config := util.Config{
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
		WebBaseURL:          "https://merchant.example.com",
	}

	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	require.NoError(t, err)

	server := &Server{
		config:          config,
		store:           store,
		tokenMaker:      tokenMaker,
		ecommerceClient: ecommerceClient,
	}

	server.setupRouter()
	return server
}

// randomMerchantForApplyment 创建随机商户（进件测试专用）
func randomMerchantForApplyment(ownerID int64) db.Merchant {
	return db.Merchant{
		ID:          util.RandomInt(1, 1000),
		OwnerUserID: ownerID,
		Name:        util.RandomString(10),
		Phone:       "13800138000",
		Address:     util.RandomString(30),
		Status:      "pending_bindbank",
		CreatedAt:   time.Now(),
	}
}

// randomMerchantApplicationForApplyment 创建随机商户申请（进件测试专用）
func randomMerchantApplicationForApplyment(userID int64) db.MerchantApplication {
	return db.MerchantApplication{
		ID:                    util.RandomInt(1, 1000),
		UserID:                userID,
		MerchantName:          util.RandomString(10),
		BusinessLicenseNumber: util.RandomString(18),
		LegalPersonName:       util.RandomString(6),
		LegalPersonIDNumber:   "110101199001011234",
		ContactPhone:          "13800138000",
		BusinessAddress:       util.RandomString(30),
		Status:                "approved",
		BusinessLicenseOcr:    []byte(`{"type_of_enterprise":"个体工商户","address":"深圳市南山区","valid_period":"2020年01月01日至长期"}`),
		IDCardBackOcr:         []byte(`{"valid_date": "2020-01-01-2030-01-01"}`),
	}
}

// randomEcommerceApplymentForTest 创建随机进件记录（测试专用）
func randomEcommerceApplymentForTest(subjectType string, subjectID int64) db.EcommerceApplyment {
	return db.EcommerceApplyment{
		ID:               util.RandomInt(1, 1000),
		SubjectType:      subjectType,
		SubjectID:        subjectID,
		OutRequestNo:     util.RandomString(20),
		OrganizationType: "2401",
		Status:           "pending",
		CreatedAt:        time.Now(),
	}
}

// ==================== 商户开户测试 ====================

func TestMerchantBindBankAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)

	applicationWithTestURL := application

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_WithEcommerceClient",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
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
				// 获取商户
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 检查是否有进行中的申请
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				// 获取商户申请信息 - 使用带测试服务器 URL 的版本
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(applicationWithTestURL, nil)

				// 创建进件记录
				store.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.CreateEcommerceApplymentParams) (db.EcommerceApplyment, error) {
						require.Equal(t, "4", arg.OrganizationType)
						require.Equal(t, "其他银行", arg.AccountBank)
						require.True(t, arg.AccountBankCode.Valid)
						require.Equal(t, int64(1099), arg.AccountBankCode.Int64)
						require.True(t, arg.BankAlias.Valid)
						require.Equal(t, "深圳前海微众银行", arg.BankAlias.String)
						require.True(t, arg.BankAliasCode.Valid)
						require.Equal(t, "1000009561", arg.BankAliasCode.String)
						require.True(t, arg.BankBranchID.Valid)
						require.Equal(t, "402584040001", arg.BankBranchID.String)
						require.Equal(t, applicationWithTestURL.ContactPhone, arg.MobilePhone)
						require.False(t, arg.ContactEmail.Valid)
						return randomEcommerceApplymentForTest("merchant", merchant.ID), nil
					})

				// Mock 加密
				ecommerceClient.EXPECT().
					EncryptSensitiveData(gomock.Any()).
					Times(5). // 法人姓名、身份证号、账户名、账号、手机
					Return("encrypted_data", nil)

				// Mock 提交进件
				ecommerceClient.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, req *wechat.EcommerceApplymentRequest) (*wechat.EcommerceApplymentResponse, error) {
						require.Equal(t, "4", req.OrganizationType)
						require.NotNil(t, req.AccountInfo)
						require.Equal(t, "其他银行", req.AccountInfo.AccountBank)
						require.Equal(t, int64(1099), req.AccountInfo.AccountBankCode)
						require.Equal(t, "402584040001", req.AccountInfo.BankBranchID)
						require.NotNil(t, req.IDCardInfo)
						require.Equal(t, "2020-01-01", req.IDCardInfo.IDCardValidTimeBegin)
						require.Equal(t, "2030-01-01", req.IDCardInfo.IDCardValidTime)
						require.NotNil(t, req.ContactInfo)
						require.Equal(t, "encrypted_data", req.ContactInfo.MobilePhone)
						require.Empty(t, req.ContactInfo.ContactEmail)
						require.NotNil(t, req.SalesSceneInfo)
						require.Equal(t, applicationWithTestURL.MerchantName, req.SalesSceneInfo.StoreName)
						require.Equal(t, "https://merchant.example.com", req.SalesSceneInfo.StoreURL)
						return &wechat.EcommerceApplymentResponse{ApplymentID: 123456789}, nil
					})

				// 更新进件状态
				store.EXPECT().
					UpdateEcommerceApplymentToSubmitted(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, nil)

				// 更新商户状态
				store.EXPECT().
					UpdateMerchantStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Merchant{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantBindBankResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, int64(123456789), response.ApplymentID)
				require.Equal(t, "submitted", response.Status)
			},
		},
		{
			name: "MerchantNotFound",
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
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "StaffForbidden",
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
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "InvalidMerchantStatus",
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
				invalidMerchant := merchant
				invalidMerchant.Status = "pending" // 还未审核通过
				expectResolveSingleOwnedMerchant(store, user.ID, invalidMerchant)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 已有进行中的申请
				existingApplyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
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
			name: "InvalidIDCardValidityPeriod",
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				invalidApplication := applicationWithTestURL
				invalidApplication.IDCardBackOcr = []byte(`{"valid_date": "长期"}`)
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				invalidApplication := applicationWithTestURL
				invalidApplication.BusinessLicenseOcr = []byte(`{"type_of_enterprise":"个体工商户","address":"深圳市南山区","valid_period":"无效文本"}`)
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 已完成
				existingApplyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
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
			name: "NoAuthorization",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No auth
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				// No stubs needed
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "InvalidRequestBody",
			body: gin.H{
				"account_type": "INVALID_TYPE", // 无效的账户类型
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/merchant/applyment/bindbank"
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

func TestValidateApplymentBusinessLicenseValidity(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		validPeriod string
		wantErr     bool
	}{
		{
			name:        "EmptyIsAllowed",
			validPeriod: "",
			wantErr:     false,
		},
		{
			name:        "LongTermOnlyIsAllowed",
			validPeriod: "长期",
			wantErr:     false,
		},
		{
			name:        "PermanentKeywordIsAllowed",
			validPeriod: "永久有效",
			wantErr:     false,
		},
		{
			name:        "RangedLongTermIsAllowed",
			validPeriod: "2020年01月01日至长期",
			wantErr:     false,
		},
		{
			name:        "InvalidTextRejected",
			validPeriod: "无效文本",
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateApplymentBusinessLicenseValidity(tc.validPeriod)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrApplymentBusinessLicenseValidityInvalid)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestBuildApplymentBusinessTime(t *testing.T) {
	t.Parallel()

	require.Empty(t, buildApplymentBusinessTime("长期"))
	require.Empty(t, buildApplymentBusinessTime("永久有效"))
	require.Equal(t, `["2020-01-01","长期"]`, buildApplymentBusinessTime("2020年01月01日至长期"))
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
		{
			name:               "NoBusinessLicenseFallsBackToMicro",
			businessLicenseNum: "",
			defaultType:        "4",
			want:               "2401",
		},
		{
			name:               "IndividualLicenseUses4",
			businessLicenseNum: "91440300TEST123456",
			licenseType:        "个体工商户",
			defaultType:        "4",
			want:               "4",
		},
		{
			name:               "EnterpriseLicenseUses2",
			businessLicenseNum: "91440300TEST123456",
			licenseType:        "有限责任公司",
			defaultType:        "4",
			want:               "2",
		},
		{
			name:               "InstitutionUses3",
			businessLicenseNum: "91440300TEST123456",
			licenseType:        "事业单位法人",
			defaultType:        "4",
			want:               "3",
		},
		{
			name:               "GovernmentUses2502",
			businessLicenseNum: "91440300TEST123456",
			licenseType:        "政府机关",
			defaultType:        "4",
			want:               "2502",
		},
		{
			name:               "SocialOrganizationUses1708",
			businessLicenseNum: "91440300TEST123456",
			licenseType:        "社会团体",
			defaultType:        "4",
			want:               "1708",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveApplymentOrganizationType(tc.businessLicenseNum, tc.licenseType, tc.subjectName, tc.defaultType)
			require.Equal(t, tc.want, got)
		})
	}
}

// ==================== 商户开户状态查询测试 ====================

func TestGetMerchantApplymentStatusAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)

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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				applyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				applyment.Status = "pending"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "pending", response.Status)
				require.True(t, response.CanSubmit)
			},
		},
		{
			name: "OK_WithSignURL",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				applyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				applyment.Status = "to_be_signed"
				applyment.SignUrl = pgtype.Text{String: "https://pay.weixin.qq.com/sign/xxx", Valid: true}
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "to_be_signed", response.Status)
				require.NotNil(t, response.SignURL)
				require.False(t, response.CanSubmit)
			},
		},
		{
			name: "OK_Finished",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				applyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				applyment.Status = "finish"
				applyment.SubMchID = pgtype.Text{String: "1234567890", Valid: true}
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "finish", response.Status)
				require.NotNil(t, response.SubMchID)
				require.False(t, response.CanSubmit)
			},
		},
		{
			name: "NoApplyment",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "not_applied", response.Status)
				require.True(t, response.CanSubmit)
			},
		},
		{
			name: "NoApplyment_SuspendedMerchant",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				testMerchant := merchant
				testMerchant.Status = "suspended"
				expectResolveSingleOwnedMerchant(store, user.ID, testMerchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "not_applied", response.Status)
				require.False(t, response.CanSubmit)
				require.Equal(t, "当前商户状态不可用，暂不支持提交收付通进件。", response.BlockReason)
			},
		},
		{
			name: "MerchantNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
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

			url := "/v1/merchant/applyment/status"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestMapWechatApplymentStatus(t *testing.T) {
	testCases := []struct {
		name     string
		wxStatus string
		expected string
	}{
		{
			name:     "LegacyAuditing",
			wxStatus: "APPLYMENT_STATE_AUDITING",
			expected: "auditing",
		},
		{
			name:     "LatestChecking",
			wxStatus: "CHECKING",
			expected: "auditing",
		},
		{
			name:     "LatestAccountNeedVerify",
			wxStatus: "ACCOUNT_NEED_VERIFY",
			expected: "auditing",
		},
		{
			name:     "LatestNeedSign",
			wxStatus: "NEED_SIGN",
			expected: "to_be_signed",
		},
		{
			name:     "LatestFinish",
			wxStatus: "FINISH",
			expected: "finish",
		},
		{
			name:     "LatestCanceled",
			wxStatus: "CANCELED",
			expected: "rejected",
		},
		{
			name:     "LatestFrozen",
			wxStatus: "FROZEN",
			expected: "frozen",
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, mapWechatApplymentStatus(tc.wxStatus))
		})
	}
}

// ==================== 进件回调测试 ====================

func TestHandleApplymentStateNotifyAPI(t *testing.T) {
	testCases := []struct {
		name          string
		body          string
		headers       map[string]string
		buildStubs    func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface, ecommerceClient *mockwechat.MockEcommerceClientInterface)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "NoEcommerceClient",
			body: `{"event_type": "APPLYMENT_STATE.CHANGE"}`,
			headers: map[string]string{
				"Wechatpay-Signature": "test_signature",
				"Wechatpay-Timestamp": "1234567890",
				"Wechatpay-Nonce":     "test_nonce",
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockPaymentClientInterface, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				// ecommerceClient is nil
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, paymentClient, ecommerceClient)

			// 不传 ecommerceClient 来测试错误情况
			server := newTestServer(t, store)

			url := "/v1/webhooks/wechat-ecommerce/applyment-notify"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(tc.body)))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			for k, v := range tc.headers {
				request.Header.Set(k, v)
			}

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestListMerchantApplymentBanksAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	ecommerceClient.EXPECT().
		ListPersonalBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).
		Times(1).
		Return(&wechat.CapitalBankListResponse{
			TotalCount: 2,
			Count:      2,
			Data: []wechat.CapitalBank{{
				BankAlias:       "其他银行",
				BankAliasCode:   "1099",
				AccountBank:     "其他银行",
				AccountBankCode: 1099,
				NeedBankBranch:  true,
			}, {
				BankAlias:       "招商银行",
				BankAliasCode:   "1000009561",
				AccountBank:     "招商银行",
				AccountBankCode: 1001,
				NeedBankBranch:  false,
			}},
		}, nil)

	server := newTestServerWithEcommerce(t, store, ecommerceClient)
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/applyment/banks?account_type=ACCOUNT_TYPE_PRIVATE", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response applymentBankListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Len(t, response.Banks, 1)
	require.Equal(t, "招商银行", response.Banks[0].BankAlias)
	require.Equal(t, "招商银行", response.Banks[0].AccountBank)
	require.Equal(t, int64(1001), response.Banks[0].AccountBankCode)
}

// ==================== 无 ecommerceClient 时的降级测试 ====================

func TestMerchantBindBankWithoutEcommerceClient(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	// 获取商户
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	// 检查是否有进行中的申请
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

	// 获取商户申请信息
	store.EXPECT().
		GetUserMerchantApplication(gomock.Any(), user.ID).
		Times(1).
		Return(application, nil)

	// 创建进件记录
	store.EXPECT().
		CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
		Times(1).
		Return(randomEcommerceApplymentForTest("merchant", merchant.ID), nil)

	// 更新商户状态（降级处理）
	store.EXPECT().
		UpdateMerchantStatus(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Merchant{}, nil)

	// 创建不带 ecommerceClient 的服务器
	server := newTestServer(t, store)

	body := gin.H{
		"account_type":      "ACCOUNT_TYPE_PRIVATE",
		"account_bank":      "招商银行",
		"bank_address_code": "440300",
		"account_number":    "6214830012345678",
		"account_name":      "张三",
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := "/v1/merchant/applyment/bindbank"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	// 应该成功但返回降级消息
	require.Equal(t, http.StatusOK, recorder.Code)
	var response merchantBindBankResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "submitted", response.Status)
	require.Contains(t, response.Message, "待人工处理")
}

// ==================== 加密失败测试 ====================

func TestMerchantBindBankEncryptFailed(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)

	applicationWithTestURL := application

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	// 获取商户
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	// 检查是否有进行中的申请
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

	// 获取商户申请信息
	store.EXPECT().
		GetUserMerchantApplication(gomock.Any(), user.ID).
		Times(1).
		Return(applicationWithTestURL, nil)

	// 创建进件记录
	store.EXPECT().
		CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
		Times(1).
		Return(randomEcommerceApplymentForTest("merchant", merchant.ID), nil)

	// Mock 加密失败
	ecommerceClient.EXPECT().
		EncryptSensitiveData(gomock.Any()).
		Times(1).
		Return("", fmt.Errorf("encrypt failed"))

	server := newTestServerWithEcommerce(t, store, ecommerceClient)

	body := gin.H{
		"account_type":      "ACCOUNT_TYPE_PRIVATE",
		"account_bank":      "招商银行",
		"bank_address_code": "440300",
		"account_number":    "6214830012345678",
		"account_name":      "张三",
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := "/v1/merchant/applyment/bindbank"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
