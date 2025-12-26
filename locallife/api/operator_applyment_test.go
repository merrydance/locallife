package api

import (
	"bytes"
	"encoding/json"
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
		Status:       "pending_bindbank",
		CreatedAt:    time.Now(),
	}
}

// randomOperatorApplicationForApplyment 创建随机运营商申请（进件测试专用）
func randomOperatorApplicationForApplyment(userID int64) db.OperatorApplication {
	return db.OperatorApplication{
		ID:                     util.RandomInt(1, 1000),
		UserID:                 userID,
		RegionID:               util.RandomInt(1, 100),
		Name:                   pgtype.Text{String: util.RandomString(10), Valid: true},
		ContactName:            pgtype.Text{String: util.RandomString(6), Valid: true},
		ContactPhone:           pgtype.Text{String: "13800138000", Valid: true},
		BusinessLicenseUrl:     pgtype.Text{String: "https://example.com/license.jpg", Valid: true},
		BusinessLicenseNumber:  pgtype.Text{String: util.RandomString(18), Valid: true},
		LegalPersonName:        pgtype.Text{String: util.RandomString(6), Valid: true},
		LegalPersonIDNumber:    pgtype.Text{String: "110101199001011234", Valid: true},
		IDCardFrontUrl:         pgtype.Text{String: "https://example.com/id_front.jpg", Valid: true},
		IDCardBackUrl:          pgtype.Text{String: "https://example.com/id_back.jpg", Valid: true},
		IDCardBackOcr:          []byte(`{"valid_date": "2020.01.01-2030.01.01"}`),
		RequestedContractYears: 3,
		Status:                 "approved",
	}
}

// ==================== 运营商开户测试 ====================

func TestOperatorBindBankAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperatorForApplyment(user.ID)
	application := randomOperatorApplicationForApplyment(user.ID)

	// 创建一个测试服务器用于模拟图片下载
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	applicationWithTestURL.IDCardFrontUrl = pgtype.Text{String: imageServer.URL + "/id_front.jpg", Valid: true}
	applicationWithTestURL.IDCardBackUrl = pgtype.Text{String: imageServer.URL + "/id_back.jpg", Valid: true}
	applicationWithTestURL.BusinessLicenseUrl = pgtype.Text{String: imageServer.URL + "/license.jpg", Valid: true}

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
				// 获取运营商
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				// 检查是否有进行中的申请
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, pgx.ErrNoRows)

				// 获取运营商申请信息
				store.EXPECT().
					GetApprovedOperatorApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(applicationWithTestURL, nil)

				// 创建进件记录
				store.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					Return(randomEcommerceApplymentForTest("operator", operator.ID), nil)

				// Mock 图片上传
				ecommerceClient.EXPECT().
					UploadImage(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(3).
					Return(&wechat.ImageUploadResponse{MediaID: "media_id_123"}, nil)

				// Mock 加密
				ecommerceClient.EXPECT().
					EncryptSensitiveData(gomock.Any()).
					Times(6).
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

				// 更新运营商状态
				store.EXPECT().
					UpdateOperatorStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Operator{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response operatorBindBankResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, int64(123456789), response.ApplymentID)
				require.Equal(t, "submitted", response.Status)
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
				"contact_phone":     "13800138000",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(db.Operator{}, pgx.ErrNoRows)
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
				"contact_phone":     "13800138000",
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
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

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
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "pending", response.Status)
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
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

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
			name: "OperatorNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(db.Operator{}, pgx.ErrNoRows)
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
