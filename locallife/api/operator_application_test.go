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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// randomOperatorApplicationDraft 创建随机运营商申请草稿
func randomOperatorApplicationDraft(userID, regionID int64) db.OperatorApplication {
	return db.OperatorApplication{
		ID:                     1,
		UserID:                 userID,
		RegionID:               regionID,
		RequestedContractYears: 1,
		Status:                 "draft",
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}
}

// randomOperatorApplicationSubmitted 创建已提交的运营商申请
func randomOperatorApplicationSubmitted(userID, regionID int64) db.OperatorApplication {
	app := randomOperatorApplicationDraft(userID, regionID)
	app.Status = "submitted"
	app.Name = pgtype.Text{String: "测试运营商", Valid: true}
	app.ContactName = pgtype.Text{String: "张三", Valid: true}
	app.ContactPhone = pgtype.Text{String: "13800138000", Valid: true}
	app.BusinessLicenseMediaAssetID = pgtype.Int8{}
	app.IDCardFrontMediaAssetID = pgtype.Int8{}
	app.IDCardBackMediaAssetID = pgtype.Int8{}
	app.SubmittedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return app
}

func TestGetOrCreateOperatorApplicationDraftAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_CreateNew",
			body: gin.H{
				"region_id": region.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 没有现有申请
				store.EXPECT().
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
				// 用户不是运营商
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(db.Operator{}, db.ErrRecordNotFound)
				// 区域存在
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
				// 区域没有运营商
				store.EXPECT().
					GetActiveOperatorByRegion(gomock.Any(), region.ID).
					Return(db.Operator{}, db.ErrRecordNotFound)
				// 区域没有待审核申请
				store.EXPECT().
					GetPendingOperatorApplicationByRegion(gomock.Any(), region.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
				// 创建草稿
				store.EXPECT().
					CreateOperatorApplicationDraft(gomock.Any(), db.CreateOperatorApplicationDraftParams{
						UserID:   user.ID,
						RegionID: region.ID,
					}).
					Return(randomOperatorApplicationDraft(user.ID, region.ID), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "OK_ExistingDraft",
			body: gin.H{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 有现有草稿
				store.EXPECT().
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(randomOperatorApplicationDraft(user.ID, region.ID), nil)
				// 获取区域名称
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Conflict_AlreadyOperator",
			body: gin.H{
				"region_id": region.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 没有申请
				store.EXPECT().
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
				// 用户已是运营商
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(db.Operator{ID: 1}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "Conflict_ApprovedApplication",
			body: gin.H{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				approvedApp := randomOperatorApplicationDraft(user.ID, region.ID)
				approvedApp.Status = "approved"
				store.EXPECT().
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(approvedApp, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "Conflict_RegionOccupied",
			body: gin.H{
				"region_id": region.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(db.Operator{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
				// 区域已有运营商
				store.EXPECT().
					GetActiveOperatorByRegion(gomock.Any(), region.ID).
					Return(db.Operator{ID: 999}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "Conflict_PendingApplication",
			body: gin.H{
				"region_id": region.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(db.Operator{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
				store.EXPECT().
					GetActiveOperatorByRegion(gomock.Any(), region.ID).
					Return(db.Operator{}, db.ErrRecordNotFound)
				// 区域有其他人的待审核申请
				otherApp := randomOperatorApplicationSubmitted(user.ID+1, region.ID)
				store.EXPECT().
					GetPendingOperatorApplicationByRegion(gomock.Any(), region.ID).
					Return(otherApp, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "BadRequest_MissingRegionID",
			body: gin.H{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(db.Operator{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Unauthorized",
			body: gin.H{
				"region_id": region.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加认证
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
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/operator/application"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			request.Header.Set("Content-Type", "application/json")

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestGetOperatorApplicationAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()

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
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(randomOperatorApplicationDraft(user.ID, region.ID), nil)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Unauthorized",
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
			recorder := httptest.NewRecorder()

			url := "/v1/operator/application"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestGetOperatorApplicationAPI_IncludesBaofuSettlementReadiness(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	app := randomOperatorApplicationDraft(user.ID, region.ID)
	app.Status = "approved"
	operator := db.Operator{ID: 3101, UserID: user.ID, RegionID: region.ID}
	binding := db.BaofuAccountBinding{
		ID:           4201,
		OwnerType:    db.BaofuAccountOwnerTypeOperator,
		OwnerID:      operator.ID,
		AccountType:  db.BaofuAccountTypeBusiness,
		ContractNo:   pgtype.Text{String: "CO3101", Valid: true},
		SharingMerID: pgtype.Text{String: "SH3101", Valid: true},
		OpenState:    db.BaofuAccountOpenStateProcessing,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetOperatorApplicationByUserID(gomock.Any(), user.ID).
		Times(1).
		Return(app, nil)
	store.EXPECT().
		GetRegion(gomock.Any(), region.ID).
		Times(1).
		Return(region, nil)
	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(1).
		Return(operator, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeOperator,
			OwnerID:   operator.ID,
		}).
		Times(1).
		Return(binding, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "CO3101")
	require.NotContains(t, recorder.Body.String(), "SH3101")
	require.NotContains(t, recorder.Body.String(), "contractNo")
	require.NotContains(t, recorder.Body.String(), "sharingMerId")
	require.NotContains(t, recorder.Body.String(), "contract_no")
	require.NotContains(t, recorder.Body.String(), "sharing_mer_id")

	var resp operatorApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.True(t, resp.IsOperator)
	require.NotNil(t, resp.SettlementAccount)
	require.Equal(t, "baofu_opening_processing", resp.SettlementAccount.State)
	require.Equal(t, "宝付开户处理中", resp.SettlementAccount.Label)
	require.False(t, resp.SettlementAccount.PaymentReady)
}

func TestUpdateOperatorApplicationBasicInfoAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"name":                     "新运营商名称",
				"contact_name":             "李四",
				"contact_phone":            "13900139000",
				"requested_contract_years": 2,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(randomOperatorApplicationDraft(user.ID, region.ID), nil)
				store.EXPECT().
					UpdateOperatorApplicationBasicInfo(gomock.Any(), gomock.Any()).
					Return(randomOperatorApplicationDraft(user.ID, region.ID), nil)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "BadRequest_NotDraft",
			body: gin.H{
				"name": "新运营商名称",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				submittedApp := randomOperatorApplicationSubmitted(user.ID, region.ID)
				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(submittedApp, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "BadRequest_InvalidPhone",
			body: gin.H{
				"contact_phone": "123",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NotFound",
			body: gin.H{
				"name": "测试",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
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
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/operator/application/basic"
			request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			request.Header.Set("Content-Type", "application/json")

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestSubmitOperatorApplicationAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()

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
				// 获取草稿（已填写完整）
				completeApp := randomOperatorApplicationDraft(user.ID, region.ID)
				completeApp.Name = pgtype.Text{String: "测试运营商", Valid: true}
				completeApp.ContactName = pgtype.Text{String: "张三", Valid: true}
				completeApp.ContactPhone = pgtype.Text{String: "13800138000", Valid: true}
				completeApp.IDCardFrontMediaAssetID = pgtype.Int8{Int64: 1, Valid: true}
				completeApp.IDCardBackMediaAssetID = pgtype.Int8{Int64: 2, Valid: true}

				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(completeApp, nil)
				// 区域没有运营商
				store.EXPECT().
					GetActiveOperatorByRegion(gomock.Any(), region.ID).
					Return(db.Operator{}, db.ErrRecordNotFound)
				// 没有其他待审核申请
				store.EXPECT().
					GetPendingOperatorApplicationByRegion(gomock.Any(), region.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
				// 提交申请
				submittedApp := completeApp
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitOperatorApplication(gomock.Any(), completeApp.ID).
					Return(submittedApp, nil)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "BadRequest_Incomplete",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 获取不完整的草稿
				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(randomOperatorApplicationDraft(user.ID, region.ID), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "OK_AutoFillNameFromLegalPerson",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomOperatorApplicationDraft(user.ID, region.ID)
				app.Name = pgtype.Text{}
				app.LegalPersonName = pgtype.Text{String: "李四", Valid: true}
				app.ContactName = pgtype.Text{String: "李四", Valid: true}
				app.ContactPhone = pgtype.Text{String: "13800138000", Valid: true}
				app.IDCardFrontMediaAssetID = pgtype.Int8{Int64: 1, Valid: true}
				app.IDCardBackMediaAssetID = pgtype.Int8{Int64: 2, Valid: true}

				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(app, nil)

				appWithName := app
				appWithName.Name = pgtype.Text{String: "李四", Valid: true}
				store.EXPECT().
					UpdateOperatorApplicationBasicInfo(gomock.Any(), gomock.Any()).
					Return(appWithName, nil)

				store.EXPECT().
					GetActiveOperatorByRegion(gomock.Any(), region.ID).
					Return(db.Operator{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetPendingOperatorApplicationByRegion(gomock.Any(), region.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)

				submittedApp := appWithName
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitOperatorApplication(gomock.Any(), app.ID).
					Return(submittedApp, nil)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Conflict_RegionOccupied",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				completeApp := randomOperatorApplicationDraft(user.ID, region.ID)
				completeApp.Name = pgtype.Text{String: "测试运营商", Valid: true}
				completeApp.ContactName = pgtype.Text{String: "张三", Valid: true}
				completeApp.ContactPhone = pgtype.Text{String: "13800138000", Valid: true}
				completeApp.IDCardFrontMediaAssetID = pgtype.Int8{Int64: 1, Valid: true}
				completeApp.IDCardBackMediaAssetID = pgtype.Int8{Int64: 2, Valid: true}

				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(completeApp, nil)
				// 区域已被占用
				store.EXPECT().
					GetActiveOperatorByRegion(gomock.Any(), region.ID).
					Return(db.Operator{ID: 999}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "NotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
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
			recorder := httptest.NewRecorder()

			url := "/v1/operator/application/submit"
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestResetOperatorApplicationToDraftAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()

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
				rejectedApp := randomOperatorApplicationDraft(user.ID, region.ID)
				rejectedApp.Status = "rejected"
				rejectedApp.RejectReason = pgtype.Text{String: "资料不完整", Valid: true}

				store.EXPECT().
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(rejectedApp, nil)

				draftApp := rejectedApp
				draftApp.Status = "draft"
				draftApp.RejectReason = pgtype.Text{Valid: false}
				store.EXPECT().
					ResetOperatorApplicationToDraft(gomock.Any(), rejectedApp.ID).
					Return(draftApp, nil)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "BadRequest_NotRejected",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 申请是草稿状态，不能重置
				store.EXPECT().
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(randomOperatorApplicationDraft(user.ID, region.ID), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorApplicationByUserID(gomock.Any(), user.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
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
			recorder := httptest.NewRecorder()

			url := "/v1/operator/application/reset"
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestUpdateOperatorApplicationRegionAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	newRegion := randomRegion()
	newRegion.ID = region.ID + 1

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"region_id": newRegion.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(randomOperatorApplicationDraft(user.ID, region.ID), nil)
				store.EXPECT().
					GetRegion(gomock.Any(), newRegion.ID).
					Return(newRegion, nil)
				store.EXPECT().
					GetActiveOperatorByRegion(gomock.Any(), newRegion.ID).
					Return(db.Operator{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetPendingOperatorApplicationByRegion(gomock.Any(), newRegion.ID).
					Return(db.OperatorApplication{}, db.ErrRecordNotFound)
				store.EXPECT().
					UpdateOperatorApplicationRegion(gomock.Any(), gomock.Any()).
					Return(randomOperatorApplicationDraft(user.ID, newRegion.ID), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Conflict_NewRegionOccupied",
			body: gin.H{
				"region_id": newRegion.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(randomOperatorApplicationDraft(user.ID, region.ID), nil)
				store.EXPECT().
					GetRegion(gomock.Any(), newRegion.ID).
					Return(newRegion, nil)
				store.EXPECT().
					GetActiveOperatorByRegion(gomock.Any(), newRegion.ID).
					Return(db.Operator{ID: 999}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "BadRequest_InvalidRegionID",
			body: gin.H{
				"region_id": 0,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
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
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/operator/application/region"
			request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			request.Header.Set("Content-Type", "application/json")

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// 添加区域可用性检查测试
func TestListAvailableRegionsForOperatorAPI(t *testing.T) {
	user, _ := randomUser(t)
	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				region := randomRegion()
				store.EXPECT().
					ListAvailableRegions(gomock.Any(), gomock.Any()).
					Return([]db.ListAvailableRegionsRow{
						{
							ID:    region.ID,
							Code:  region.Code,
							Name:  region.Name,
							Level: region.Level,
						},
					}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "BadRequest_MissingPageID",
			query: "?page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
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
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/regions/available%s", tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// TestGetOperatorApplicationAPI_WithMediaAssetIDs — Phase 5.8
// 当运营商申请草稿中已绑定证照媒体资产 ID 时，GET /v1/operator/application
// 响应中应包含 business_license_asset_id 和 id_card_front_asset_id
func TestGetOperatorApplicationAPI_WithMediaAssetIDs(t *testing.T) {
	user, _ := randomUser(t)

	app := randomOperatorApplicationDraft(user.ID, 1)
	app.Status = "draft"
	app.BusinessLicenseMediaAssetID = pgtype.Int8{Int64: 10, Valid: true}
	app.IDCardFrontMediaAssetID = pgtype.Int8{Int64: 11, Valid: true}
	app.IDCardBackMediaAssetID = pgtype.Int8{Int64: 12, Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetOperatorApplicationByUserID(gomock.Any(), user.ID).
		Times(1).
		Return(app, nil)
	// getRegionName 内部调用 GetRegion：返回错误使其回退为空字符串
	store.EXPECT().
		GetRegion(gomock.Any(), gomock.Eq(int64(1))).
		Times(1).
		Return(db.Region{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp operatorApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.BusinessLicenseAssetID)
	require.Equal(t, int64(10), *resp.BusinessLicenseAssetID)
	require.NotNil(t, resp.IDCardFrontAssetID)
	require.Equal(t, int64(11), *resp.IDCardFrontAssetID)
	require.NotNil(t, resp.IDCardBackAssetID)
	require.Equal(t, int64(12), *resp.IDCardBackAssetID)
}

func TestGetOperatorApplicationAPI_ReturnsAsyncOCRFields(t *testing.T) {
	user, _ := randomUser(t)
	ocrJobID := int64(9101)

	businessLicenseOCR, err := json.Marshal(BusinessLicenseOCRData{
		Status:         "done",
		QueuedAt:       "2026-03-25T16:10:00Z",
		StartedAt:      "2026-03-25T16:10:02Z",
		OCRJobID:       &ocrJobID,
		AlertEmittedAt: "2026-03-25T16:10:30Z",
	})
	require.NoError(t, err)
	idCardFrontOCR, err := json.Marshal(OperatorIDCardOCRData{
		Status:         "processing",
		ErrorCode:      "ocr_retryable_error",
		AlertEmittedAt: "2026-03-25T16:12:00Z",
		QueuedAt:       "2026-03-25T16:11:00Z",
		StartedAt:      "2026-03-25T16:11:03Z",
		OCRJobID:       &ocrJobID,
	})
	require.NoError(t, err)
	idCardBackOCR, err := json.Marshal(OperatorIDCardBackOCR{
		Status:    "done",
		QueuedAt:  "2026-03-25T16:13:00Z",
		StartedAt: "2026-03-25T16:13:04Z",
		OCRJobID:  &ocrJobID,
	})
	require.NoError(t, err)

	app := randomOperatorApplicationDraft(user.ID, 1)
	app.BusinessLicenseOcr = businessLicenseOCR
	app.IDCardFrontOcr = idCardFrontOCR
	app.IDCardBackOcr = idCardBackOCR

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetOperatorApplicationByUserID(gomock.Any(), user.ID).
		Times(1).
		Return(app, nil)
	store.EXPECT().
		GetRegion(gomock.Any(), gomock.Eq(int64(1))).
		Times(1).
		Return(db.Region{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp operatorApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.BusinessLicenseOCR)
	require.Equal(t, "2026-03-25T16:10:00Z", resp.BusinessLicenseOCR.QueuedAt)
	require.Equal(t, "2026-03-25T16:10:02Z", resp.BusinessLicenseOCR.StartedAt)
	require.NotNil(t, resp.BusinessLicenseOCR.OCRJobID)
	require.Equal(t, ocrJobID, *resp.BusinessLicenseOCR.OCRJobID)
	require.NotNil(t, resp.IDCardFrontOCR)
	require.Equal(t, "processing", resp.IDCardFrontOCR.Status)
	require.Equal(t, "ocr_retryable_error", resp.IDCardFrontOCR.ErrorCode)
	require.Equal(t, "2026-03-25T16:12:00Z", resp.IDCardFrontOCR.AlertEmittedAt)
	require.NotNil(t, resp.IDCardFrontOCR.OCRJobID)
	require.Equal(t, ocrJobID, *resp.IDCardFrontOCR.OCRJobID)
	require.NotNil(t, resp.IDCardBackOCR)
	require.Equal(t, "2026-03-25T16:13:00Z", resp.IDCardBackOCR.QueuedAt)
	require.Equal(t, "2026-03-25T16:13:04Z", resp.IDCardBackOCR.StartedAt)
	require.NotNil(t, resp.IDCardBackOCR.OCRJobID)
	require.Equal(t, ocrJobID, *resp.IDCardBackOCR.OCRJobID)
}

func TestGetOperatorApplicationAPI_ReturnsInternalServerErrorOnInvalidBusinessLicenseOCR(t *testing.T) {
	user, _ := randomUser(t)
	app := randomOperatorApplicationDraft(user.ID, 1)
	app.BusinessLicenseOcr = []byte(`not-json`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetOperatorApplicationByUserID(gomock.Any(), user.ID).
		Times(1).
		Return(app, nil)
	store.EXPECT().
		GetRegion(gomock.Any(), gomock.Eq(int64(1))).
		Times(1).
		Return(db.Region{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "internal server error", resp.Message)
}

func TestDeleteOperatorApplicationDocumentAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	region.ID = 1

	testCases := []struct {
		name          string
		documentType  string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:         "OKBusinessLicense",
			documentType: "business_license",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomOperatorApplicationDraft(user.ID, region.ID)
				app.BusinessLicenseMediaAssetID = pgtype.Int8{Int64: 10, Valid: true}
				app.BusinessLicenseNumber = pgtype.Text{String: "91310000123456789A", Valid: true}
				app.BusinessLicenseOcr = []byte(`{"status":"done"}`)
				updated := app
				updated.BusinessLicenseMediaAssetID = pgtype.Int8{}
				updated.BusinessLicenseNumber = pgtype.Text{}
				updated.BusinessLicenseOcr = nil

				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					ClearOperatorApplicationBusinessLicense(gomock.Any(), app.ID).
					Times(1).
					Return(updated, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(10)).
					Times(1).
					Return(db.MediaAsset{ID: 10, UploadedBy: user.ID}, nil)
				store.EXPECT().
					SoftDeleteMediaAsset(gomock.Any(), int64(10)).
					Times(1).
					Return(db.MediaAsset{ID: 10, UploadedBy: user.ID}, nil)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Times(1).
					Return(region, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp operatorApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Nil(t, resp.BusinessLicenseAssetID)
				require.Empty(t, resp.BusinessLicenseNumber)
				require.Nil(t, resp.BusinessLicenseOCR)
			},
		},
		{
			name:         "OKIDCardFront",
			documentType: "id_card_front",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomOperatorApplicationDraft(user.ID, region.ID)
				app.IDCardFrontMediaAssetID = pgtype.Int8{Int64: 11, Valid: true}
				app.LegalPersonName = pgtype.Text{String: "张三", Valid: true}
				app.LegalPersonIDNumber = pgtype.Text{String: "110101199001011234", Valid: true}
				app.IDCardFrontOcr = []byte(`{"status":"done","name":"张三"}`)
				updated := app
				updated.IDCardFrontMediaAssetID = pgtype.Int8{}
				updated.LegalPersonName = pgtype.Text{}
				updated.LegalPersonIDNumber = pgtype.Text{}
				updated.IDCardFrontOcr = nil

				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					ClearOperatorApplicationIDCardFront(gomock.Any(), app.ID).
					Times(1).
					Return(updated, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(11)).
					Times(1).
					Return(db.MediaAsset{ID: 11, UploadedBy: user.ID}, nil)
				store.EXPECT().
					SoftDeleteMediaAsset(gomock.Any(), int64(11)).
					Times(1).
					Return(db.MediaAsset{ID: 11, UploadedBy: user.ID}, nil)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Times(1).
					Return(region, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp operatorApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Nil(t, resp.IDCardFrontAssetID)
				require.Empty(t, resp.LegalPersonName)
				require.Empty(t, resp.LegalPersonIDNumber)
				require.Nil(t, resp.IDCardFrontOCR)
			},
		},
		{
			name:         "OKIDCardBack",
			documentType: "id_card_back",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomOperatorApplicationDraft(user.ID, region.ID)
				app.IDCardBackMediaAssetID = pgtype.Int8{Int64: 12, Valid: true}
				app.IDCardBackOcr = []byte(`{"status":"done","valid_end":"2035-01-01"}`)
				updated := app
				updated.IDCardBackMediaAssetID = pgtype.Int8{}
				updated.IDCardBackOcr = nil

				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					ClearOperatorApplicationIDCardBack(gomock.Any(), app.ID).
					Times(1).
					Return(updated, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(12)).
					Times(1).
					Return(db.MediaAsset{ID: 12, UploadedBy: user.ID}, nil)
				store.EXPECT().
					SoftDeleteMediaAsset(gomock.Any(), int64(12)).
					Times(1).
					Return(db.MediaAsset{ID: 12, UploadedBy: user.ID}, nil)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Times(1).
					Return(region, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp operatorApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Nil(t, resp.IDCardBackAssetID)
				require.Nil(t, resp.IDCardBackOCR)
			},
		},
		{
			name:         "InvalidDocumentType",
			documentType: "invalid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
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
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodDelete, "/v1/operator/application/documents/"+tc.documentType, nil)
			require.NoError(t, err)
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
