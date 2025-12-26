package api

import (
	"bytes"
	"database/sql"
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
	app.BusinessLicenseUrl = pgtype.Text{String: "uploads/operators/1/license/test.jpg", Valid: true}
	app.IDCardFrontUrl = pgtype.Text{String: "uploads/operators/1/idcard/front.jpg", Valid: true}
	app.IDCardBackUrl = pgtype.Text{String: "uploads/operators/1/idcard/back.jpg", Valid: true}
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
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
				// 用户不是运营商
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(db.Operator{}, sql.ErrNoRows)
				// 区域存在
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
				// 区域没有运营商
				store.EXPECT().
					GetOperatorByRegion(gomock.Any(), region.ID).
					Return(db.Operator{}, sql.ErrNoRows)
				// 区域没有待审核申请
				store.EXPECT().
					GetPendingOperatorApplicationByRegion(gomock.Any(), region.ID).
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
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
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
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
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(db.Operator{}, sql.ErrNoRows)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
				// 区域已有运营商
				store.EXPECT().
					GetOperatorByRegion(gomock.Any(), region.ID).
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
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(db.Operator{}, sql.ErrNoRows)
				store.EXPECT().
					GetRegion(gomock.Any(), region.ID).
					Return(region, nil)
				store.EXPECT().
					GetOperatorByRegion(gomock.Any(), region.ID).
					Return(db.Operator{}, sql.ErrNoRows)
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
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(db.Operator{}, sql.ErrNoRows)
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
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
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
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
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
				completeApp.BusinessLicenseUrl = pgtype.Text{String: "uploads/test.jpg", Valid: true}
				completeApp.IDCardFrontUrl = pgtype.Text{String: "uploads/front.jpg", Valid: true}
				completeApp.IDCardBackUrl = pgtype.Text{String: "uploads/back.jpg", Valid: true}

				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(completeApp, nil)
				// 区域没有运营商
				store.EXPECT().
					GetOperatorByRegion(gomock.Any(), region.ID).
					Return(db.Operator{}, sql.ErrNoRows)
				// 没有其他待审核申请
				store.EXPECT().
					GetPendingOperatorApplicationByRegion(gomock.Any(), region.ID).
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
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
			name: "Conflict_RegionOccupied",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				completeApp := randomOperatorApplicationDraft(user.ID, region.ID)
				completeApp.Name = pgtype.Text{String: "测试运营商", Valid: true}
				completeApp.ContactName = pgtype.Text{String: "张三", Valid: true}
				completeApp.ContactPhone = pgtype.Text{String: "13800138000", Valid: true}
				completeApp.BusinessLicenseUrl = pgtype.Text{String: "uploads/test.jpg", Valid: true}
				completeApp.IDCardFrontUrl = pgtype.Text{String: "uploads/front.jpg", Valid: true}
				completeApp.IDCardBackUrl = pgtype.Text{String: "uploads/back.jpg", Valid: true}

				store.EXPECT().
					GetOperatorApplicationDraft(gomock.Any(), user.ID).
					Return(completeApp, nil)
				// 区域已被占用
				store.EXPECT().
					GetOperatorByRegion(gomock.Any(), region.ID).
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
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
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
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
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
					GetOperatorByRegion(gomock.Any(), newRegion.ID).
					Return(db.Operator{}, sql.ErrNoRows)
				store.EXPECT().
					GetPendingOperatorApplicationByRegion(gomock.Any(), newRegion.ID).
					Return(db.OperatorApplication{}, pgx.ErrNoRows)
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
					GetOperatorByRegion(gomock.Any(), newRegion.ID).
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
	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=10",
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

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}
