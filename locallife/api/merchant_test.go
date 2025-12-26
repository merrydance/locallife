package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	mockwechat "github.com/merrydance/locallife/wechat/mock"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUploadMerchantImageAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		category      string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			category: "business_license",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response uploadImageResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.NotEmpty(t, response.ImageURL)
				require.Contains(t, response.ImageURL, "merchants")
				require.Contains(t, response.ImageURL, "business_license")
			},
		},
		{
			name:     "InvalidCategory",
			category: "invalid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:     "NoAuthorization",
			category: "logo",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
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
			wechatClient := mockwechat.NewMockWechatClient(ctrl)
			if tc.name == "OK" {
				// business_license no longer goes through ImgSecCheck
			}
			server := newTestServerWithWechat(t, store, wechatClient)

			// Create multipart form
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			writer.WriteField("category", tc.category)

			// Add fake image file
			part, err := writer.CreateFormFile("image", "test.jpg")
			require.NoError(t, err)
			part.Write([]byte("fake image data"))
			writer.Close()

			url := "/v1/merchants/images/upload"
			request, err := http.NewRequest(http.MethodPost, url, body)
			require.NoError(t, err)
			request.Header.Set("Content-Type", writer.FormDataContentType())

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestCreateMerchantApplicationAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"merchant_name":              "Test Restaurant",
				"business_license_number":    "123456789",
				"business_license_image_url": "uploads/merchants/1/business_license/test.jpg",
				"legal_person_name":          "Test Person",
				"legal_person_id_number":     "110101199001011234",
				"legal_person_id_front_url":  "uploads/merchants/1/id_front/test.jpg",
				"legal_person_id_back_url":   "uploads/merchants/1/id_back/test.jpg",
				"contact_phone":              "13800138000",
				"business_address":           "Test Address",
				"longitude":                  "116.404",
				"latitude":                   "39.915",
				"business_scope":             "餐饮服务",
				"region_id":                  1, // 前端上报的区域ID
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Check existing application
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.MerchantApplication{}, sql.ErrNoRows)

				// Create application
				store.EXPECT().
					CreateMerchantApplication(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantApplication{
						ID:                      1,
						UserID:                  user.ID,
						MerchantName:            "Test Restaurant",
						BusinessLicenseNumber:   "123456789",
						BusinessLicenseImageUrl: "uploads/merchants/1/business_license/test.jpg",
						LegalPersonName:         "Test Person",
						LegalPersonIDNumber:     "110101199001011234",
						LegalPersonIDFrontUrl:   "uploads/merchants/1/id_front/test.jpg",
						LegalPersonIDBackUrl:    "uploads/merchants/1/id_back/test.jpg",
						ContactPhone:            "13800138000",
						BusinessAddress:         "Test Address",
						Status:                  "pending",
						CreatedAt:               time.Now(),
						UpdatedAt:               time.Now(),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplicationResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, user.ID, response.UserID)
				require.Equal(t, "Test Restaurant", response.MerchantName)
				require.Equal(t, "pending", response.Status)
			},
		},
		{
			name: "AlreadyHasPendingApplication",
			body: gin.H{
				"merchant_name":              "Test Restaurant",
				"business_license_number":    "123456789",
				"business_license_image_url": "test.jpg",
				"legal_person_name":          "Test Person",
				"legal_person_id_number":     "110101199001011234",
				"legal_person_id_front_url":  "test.jpg",
				"legal_person_id_back_url":   "test.jpg",
				"contact_phone":              "13800138000",
				"business_address":           "Test Address",
				"longitude":                  "116.404",
				"latitude":                   "39.915",
				"region_id":                  1,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.MerchantApplication{
						ID:     1,
						Status: "pending",
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"merchant_name":              "Test Restaurant",
				"business_license_number":    "123456789",
				"business_license_image_url": "test.jpg",
				"legal_person_name":          "Test Person",
				"legal_person_id_number":     "110101199001011234",
				"legal_person_id_front_url":  "test.jpg",
				"legal_person_id_back_url":   "test.jpg",
				"contact_phone":              "13800138000",
				"business_address":           "Test Address",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No calls expected
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/merchants/applications"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetCurrentMerchantAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, merchant.ID, response.ID)
				require.Equal(t, merchant.Name, response.Name)
			},
		},
		{
			name: "NotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No calls expected
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
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

			url := "/v1/merchants/me"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateCurrentMerchantAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"name":        "Updated Name",
				"description": "Updated Description",
				"version":     1, // ✅ 添加version字段
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					UpdateMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Merchant{
						ID:          merchant.ID,
						OwnerUserID: merchant.OwnerUserID,
						Name:        "Updated Name",
						Description: pgtype.Text{String: "Updated Description", Valid: true},
						Phone:       merchant.Phone,
						Address:     merchant.Address,
						Status:      merchant.Status,
						Version:     2, // 版本号递增
						CreatedAt:   merchant.CreatedAt,
						UpdatedAt:   time.Now(),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "Updated Name", response.Name)
			},
		},
		{
			name: "MerchantNotFound",
			body: gin.H{
				"name":    "Updated Name",
				"version": 1,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "VersionConflict",
			body: gin.H{
				"name":    "Updated Name",
				"version": 1, // 旧版本号
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				merchantWithNewVersion := merchant
				merchantWithNewVersion.Version = 2 // 数据库中已是版本2

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchantWithNewVersion, nil)

				// UpdateMerchant不会被调用，因为version检查在之前
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/merchants/me"
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// Helper functions

func randomMerchant(ownerID int64) db.Merchant {
	return db.Merchant{
		ID:          util.RandomInt(1, 1000),
		OwnerUserID: ownerID,
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(50), Valid: true},
		Phone:       "13800138000",
		Address:     util.RandomString(30),
		Status:      "approved",
		Version:     1, // 初始版本号
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func randomMerchantApplication(userID int64) db.MerchantApplication {
	return db.MerchantApplication{
		ID:                      util.RandomInt(1, 1000),
		UserID:                  userID,
		MerchantName:            util.RandomString(10),
		BusinessLicenseNumber:   util.RandomString(18),
		BusinessLicenseImageUrl: "uploads/merchants/test/license.jpg",
		LegalPersonName:         util.RandomString(6),
		LegalPersonIDNumber:     "110101199001011234",
		LegalPersonIDFrontUrl:   "uploads/merchants/test/id_front.jpg",
		LegalPersonIDBackUrl:    "uploads/merchants/test/id_back.jpg",
		ContactPhone:            "13800138000",
		BusinessAddress:         util.RandomString(30),
		BusinessScope:           pgtype.Text{String: "餐饮服务", Valid: true},
		Status:                  "pending",
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}
}

// ==================== 获取用户入驻申请测试 ====================

func TestGetUserMerchantApplicationAPI(t *testing.T) {
	user, _ := randomUser(t)
	application := randomMerchantApplication(user.ID)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(application, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplicationResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, application.ID, response.ID)
				require.Equal(t, application.MerchantName, response.MerchantName)
			},
		},
		{
			name: "NotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.MerchantApplication{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
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

			url := "/v1/merchants/applications/me"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 商户申请列表测试 ====================

func TestListMerchantApplicationsAPI(t *testing.T) {
	user, _ := randomUser(t)
	applications := make([]db.MerchantApplication, 3)
	for i := 0; i < 3; i++ {
		applications[i] = randomMerchantApplication(user.ID)
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Admin role check
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "admin",
					}).
					Times(1).
					Return(db.UserRole{ID: 1, UserID: user.ID, Role: "admin"}, nil)

				arg := db.ListAllMerchantApplicationsParams{
					Limit:  10,
					Offset: 0,
				}
				store.EXPECT().
					ListAllMerchantApplications(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(applications, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response []merchantApplicationResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Len(t, response, 3)
			},
		},
		{
			name:  "WithStatusFilter",
			query: "?page_id=1&page_size=10&status=pending",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Admin role check
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "admin",
					}).
					Times(1).
					Return(db.UserRole{ID: 1, UserID: user.ID, Role: "admin"}, nil)

				arg := db.ListMerchantApplicationsParams{
					Status: "pending",
					Limit:  10,
					Offset: 0,
				}
				store.EXPECT().
					ListMerchantApplications(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(applications, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "NoAuthorization",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					ListAllMerchantApplications(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "InvalidPageID",
			query: "?page_id=0&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					ListAllMerchantApplications(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
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
			tc.buildStubs(store)

			server := newTestServer(t, store)

			url := "/v1/admin/merchants/applications" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 审核商户申请测试 ====================

func TestReviewMerchantApplicationAPI(t *testing.T) {
	user, _ := randomUser(t)
	application := randomMerchantApplication(user.ID)
	application.Status = "pending"

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "ApproveOK",
			body: gin.H{
				"application_id": application.ID,
				"approve":        true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Admin role check
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "admin",
					}).
					Times(1).
					Return(db.UserRole{ID: 1, UserID: user.ID, Role: "admin"}, nil)

				store.EXPECT().
					GetMerchantApplication(gomock.Any(), gomock.Eq(application.ID)).
					Times(1).
					Return(application, nil)

				store.EXPECT().
					UpdateMerchantApplicationStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantApplication{
						ID:           application.ID,
						UserID:       application.UserID,
						MerchantName: application.MerchantName,
						Status:       "approved",
					}, nil)

				store.EXPECT().
					CreateMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Merchant{
						ID:          1,
						OwnerUserID: application.UserID,
						Name:        application.MerchantName,
						Status:      "approved",
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "RejectOK",
			body: gin.H{
				"application_id": application.ID,
				"approve":        false,
				"reject_reason":  "资料不完整",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Admin role check
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "admin",
					}).
					Times(1).
					Return(db.UserRole{ID: 1, UserID: user.ID, Role: "admin"}, nil)

				store.EXPECT().
					GetMerchantApplication(gomock.Any(), gomock.Eq(application.ID)).
					Times(1).
					Return(application, nil)

				store.EXPECT().
					UpdateMerchantApplicationStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantApplication{
						ID:           application.ID,
						UserID:       application.UserID,
						MerchantName: application.MerchantName,
						Status:       "rejected",
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NotFound",
			body: gin.H{
				"application_id": 99999,
				"approve":        true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Admin role check
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "admin",
					}).
					Times(1).
					Return(db.UserRole{ID: 1, UserID: user.ID, Role: "admin"}, nil)

				store.EXPECT().
					GetMerchantApplication(gomock.Any(), gomock.Eq(int64(99999))).
					Times(1).
					Return(db.MerchantApplication{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"application_id": application.ID,
				"approve":        true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					GetMerchantApplication(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "AlreadyReviewed",
			body: gin.H{
				"application_id": application.ID,
				"approve":        true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Admin role check
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   "admin",
					}).
					Times(1).
					Return(db.UserRole{ID: 1, UserID: user.ID, Role: "admin"}, nil)

				approvedApp := application
				approvedApp.Status = "approved"
				store.EXPECT().
					GetMerchantApplication(gomock.Any(), gomock.Eq(application.ID)).
					Times(1).
					Return(approvedApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
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
			tc.buildStubs(store)

			server := newTestServer(t, store)

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/admin/merchants/applications/review"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
