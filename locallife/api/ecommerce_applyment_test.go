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
	"github.com/jackc/pgx/v5"
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
		ID:                      util.RandomInt(1, 1000),
		UserID:                  userID,
		MerchantName:            util.RandomString(10),
		BusinessLicenseNumber:   util.RandomString(18),
		BusinessLicenseImageUrl: "https://example.com/license.jpg",
		LegalPersonName:         util.RandomString(6),
		LegalPersonIDNumber:     "110101199001011234",
		LegalPersonIDFrontUrl:   "https://example.com/id_front.jpg",
		LegalPersonIDBackUrl:    "https://example.com/id_back.jpg",
		ContactPhone:            "13800138000",
		BusinessAddress:         util.RandomString(30),
		Status:                  "approved",
		IDCardBackOcr:           []byte(`{"valid_date": "2020.01.01-2030.01.01"}`),
	}
}

// randomRiderForApplyment 创建随机骑手
func randomRiderForApplyment(userID int64) db.Rider {
	return db.Rider{
		ID:        util.RandomInt(1, 1000),
		UserID:    userID,
		RealName:  util.RandomString(6),
		Phone:     "13800138000",
		Status:    "pending_bindbank",
		CreatedAt: time.Now(),
	}
}

// randomRiderApplicationForApplyment 创建随机骑手申请
func randomRiderApplicationForApplyment(userID int64) db.RiderApplication {
	return db.RiderApplication{
		ID:             util.RandomInt(1, 1000),
		UserID:         userID,
		RealName:       pgtype.Text{String: util.RandomString(6), Valid: true},
		Phone:          pgtype.Text{String: "13800138000", Valid: true},
		IDCardFrontUrl: pgtype.Text{String: "https://example.com/id_front.jpg", Valid: true},
		IDCardBackUrl:  pgtype.Text{String: "https://example.com/id_back.jpg", Valid: true},
		IDCardOcr:      []byte(`{"name": "张三", "id_number": "110101199001011234", "valid_date": "2020.01.01-2030.01.01"}`),
		Status:         "approved",
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

	// 创建一个测试服务器用于模拟图片下载
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回一个最小的有效 JPEG 图片
		// 最小 JPEG 文件头
		minJPEG := []byte{
			0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
			0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
			0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
			0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
			0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
			0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
			0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
			0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
			0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00,
			0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03,
			0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7D,
			0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00, 0x7F, 0xFF,
			0xD9,
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(minJPEG)
	}))
	defer imageServer.Close()

	// 更新申请记录使用测试服务器的 URL
	applicationWithTestURL := application
	applicationWithTestURL.LegalPersonIDFrontUrl = imageServer.URL + "/id_front.jpg"
	applicationWithTestURL.LegalPersonIDBackUrl = imageServer.URL + "/id_back.jpg"
	applicationWithTestURL.BusinessLicenseImageUrl = imageServer.URL + "/license.jpg"

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
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"bank_name":         "招商银行深圳分行",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
				"contact_phone":     "13800138000",
				"contact_email":     "test@example.com",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				// 获取商户
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				// 检查是否有进行中的申请
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, pgx.ErrNoRows)

				// 获取商户申请信息 - 使用带测试服务器 URL 的版本
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(applicationWithTestURL, nil)

				// 创建进件记录
				store.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					Return(randomEcommerceApplymentForTest("merchant", merchant.ID), nil)

				// Mock 图片上传
				ecommerceClient.EXPECT().
					UploadImage(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(3). // 身份证正面、背面、营业执照
					Return(&wechat.ImageUploadResponse{MediaID: "media_id_123"}, nil)

				// Mock 加密
				ecommerceClient.EXPECT().
					EncryptSensitiveData(gomock.Any()).
					Times(6). // 法人姓名、身份证号、账户名、账号、手机、邮箱（6次）
					Return("encrypted_data", nil)

				// Mock 提交进件
				ecommerceClient.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&wechat.EcommerceApplymentResponse{ApplymentID: 123456789}, nil)

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
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
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
				"contact_phone":     "13800138000",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(db.Merchant{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
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
				"contact_phone":     "13800138000",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				invalidMerchant := merchant
				invalidMerchant.Status = "pending" // 还未审核通过
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(invalidMerchant, nil)
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
				"contact_phone":     "13800138000",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

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
			name: "AlreadyFinished",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
				"contact_phone":     "13800138000",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

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
				"contact_phone":     "13800138000",
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
				// No stubs needed
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
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

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
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "pending", response.Status)
			},
		},
		{
			name: "OK_WithSignURL",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

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
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "to_be_signed", response.Status)
				require.NotNil(t, response.SignURL)
			},
		},
		{
			name: "OK_Finished",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

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
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "finish", response.Status)
				require.NotNil(t, response.SubMchID)
			},
		},
		{
			name: "NoApplyment",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "MerchantNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(db.Merchant{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
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

// ==================== 骑手开户测试 ====================

func TestRiderBindBankAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRiderForApplyment(user.ID)
	riderApplication := randomRiderApplicationForApplyment(user.ID)

	// 创建一个测试服务器用于模拟图片下载
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回一个最小的有效 JPEG 图片
		minJPEG := []byte{
			0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
			0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
			0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
			0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
			0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
			0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
			0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
			0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
			0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00,
			0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03,
			0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7D,
			0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00, 0x7F, 0xFF,
			0xD9,
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(minJPEG)
	}))
	defer imageServer.Close()

	// 更新申请记录使用测试服务器的 URL
	riderApplicationWithTestURL := riderApplication
	riderApplicationWithTestURL.IDCardFrontUrl = pgtype.Text{String: imageServer.URL + "/id_front.jpg", Valid: true}
	riderApplicationWithTestURL.IDCardBackUrl = pgtype.Text{String: imageServer.URL + "/id_back.jpg", Valid: true}

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"bank_name":         "招商银行深圳分行",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
				"contact_phone":     "13800138000",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, pgx.ErrNoRows)

				// 使用带测试服务器 URL 的版本
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(riderApplicationWithTestURL, nil)

				store.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					Return(randomEcommerceApplymentForTest("rider", rider.ID), nil)

				// Mock 图片上传
				ecommerceClient.EXPECT().
					UploadImage(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2). // 身份证正面、背面
					Return(&wechat.ImageUploadResponse{MediaID: "media_id_123"}, nil)

				// Mock 加密
				ecommerceClient.EXPECT().
					EncryptSensitiveData(gomock.Any()).
					Times(5). // 姓名、身份证号、账户名、账号、手机
					Return("encrypted_data", nil)

				// Mock 提交进件
				ecommerceClient.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&wechat.EcommerceApplymentResponse{ApplymentID: 123456789}, nil)

				store.EXPECT().
					UpdateEcommerceApplymentToSubmitted(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, nil)

				store.EXPECT().
					UpdateRiderStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Rider{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response riderBindBankResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, int64(123456789), response.ApplymentID)
			},
		},
		{
			name: "RiderNotFound",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
				"contact_phone":     "13800138000",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "InvalidRiderStatus",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
				"contact_phone":     "13800138000",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				invalidRider := rider
				invalidRider.Status = "pending"
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(invalidRider, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidAccountType",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_BUSINESS", // 骑手不支持对公账户
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
				"contact_phone":     "13800138000",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				// No stubs needed - validation fails first
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

			url := "/v1/rider/applyment/bindbank"
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

// ==================== 骑手开户状态查询测试 ====================

func TestGetRiderApplymentStatusAPI(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRiderForApplyment(user.ID)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				applyment := randomEcommerceApplymentForTest("rider", rider.ID)
				applyment.Status = "auditing"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NoApplyment",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
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

			url := "/v1/rider/applyment/status"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
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

// ==================== 无 ecommerceClient 时的降级测试 ====================

func TestMerchantBindBankWithoutEcommerceClient(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	// 获取商户
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), user.ID).
		Times(1).
		Return(merchant, nil)

	// 检查是否有进行中的申请
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, pgx.ErrNoRows)

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
		"contact_phone":     "13800138000",
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
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "submitted", response.Status)
	require.Contains(t, response.Message, "待人工处理")
}

// ==================== 图片上传失败测试 ====================

func TestMerchantBindBankUploadImageFailed(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)

	// 创建一个测试服务器用于模拟图片下载
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回一个最小的有效 JPEG 图片
		minJPEG := []byte{
			0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
			0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
			0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
			0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
			0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
			0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
			0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
			0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
			0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00,
			0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03,
			0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7D,
			0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00, 0x7F, 0xFF,
			0xD9,
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(minJPEG)
	}))
	defer imageServer.Close()

	// 更新申请记录使用测试服务器的 URL
	applicationWithTestURL := application
	applicationWithTestURL.LegalPersonIDFrontUrl = imageServer.URL + "/id_front.jpg"
	applicationWithTestURL.LegalPersonIDBackUrl = imageServer.URL + "/id_back.jpg"
	applicationWithTestURL.BusinessLicenseImageUrl = imageServer.URL + "/license.jpg"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	// 获取商户
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), user.ID).
		Times(1).
		Return(merchant, nil)

	// 检查是否有进行中的申请
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, pgx.ErrNoRows)

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

	// Mock 图片上传失败
	ecommerceClient.EXPECT().
		UploadImage(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil, fmt.Errorf("upload failed"))

	server := newTestServerWithEcommerce(t, store, ecommerceClient)

	body := gin.H{
		"account_type":      "ACCOUNT_TYPE_PRIVATE",
		"account_bank":      "招商银行",
		"bank_address_code": "440300",
		"account_number":    "6214830012345678",
		"account_name":      "张三",
		"contact_phone":     "13800138000",
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

// ==================== 加密失败测试 ====================

func TestMerchantBindBankEncryptFailed(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)

	// 创建一个测试服务器用于模拟图片下载
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回一个最小的有效 JPEG 图片
		minJPEG := []byte{
			0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
			0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
			0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
			0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
			0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
			0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
			0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
			0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
			0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00,
			0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03,
			0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7D,
			0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00, 0x7F, 0xFF,
			0xD9,
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(minJPEG)
	}))
	defer imageServer.Close()

	// 更新申请记录使用测试服务器的 URL
	applicationWithTestURL := application
	applicationWithTestURL.LegalPersonIDFrontUrl = imageServer.URL + "/id_front.jpg"
	applicationWithTestURL.LegalPersonIDBackUrl = imageServer.URL + "/id_back.jpg"
	applicationWithTestURL.BusinessLicenseImageUrl = imageServer.URL + "/license.jpg"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	// 获取商户
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), user.ID).
		Times(1).
		Return(merchant, nil)

	// 检查是否有进行中的申请
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, pgx.ErrNoRows)

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

	// Mock 图片上传成功
	ecommerceClient.EXPECT().
		UploadImage(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(3).
		Return(&wechat.ImageUploadResponse{MediaID: "media_id_123"}, nil)

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
		"contact_phone":     "13800138000",
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
