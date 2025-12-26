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
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ====================== Helper Functions ======================

func randomAppeal(claimID, appellantID, regionID int64, appellantType string) db.Appeal {
	return db.Appeal{
		ID:            util.RandomInt(1, 1000),
		ClaimID:       claimID,
		AppellantType: appellantType,
		AppellantID:   appellantID,
		Reason:        "订单包装完好，顾客收货时已当面核对",
		EvidenceUrls:  []string{"https://example.com/appeal_evidence.jpg"},
		Status:        "pending",
		RegionID:      regionID,
		CreatedAt:     time.Now(),
	}
}

// randomAppealRider 生成随机骑手（带区域ID）用于申诉测试
func randomAppealRider(userID, regionID int64) db.Rider {
	return db.Rider{
		ID:            util.RandomInt(1, 1000),
		UserID:        userID,
		RealName:      util.RandomString(6),
		Phone:         "139" + util.RandomString(8),
		IDCardNo:      util.RandomString(18),
		Status:        "active",
		DepositAmount: 30000,
		IsOnline:      true,
		RegionID:      pgtype.Int8{Int64: regionID, Valid: true},
		CreatedAt:     time.Now(),
	}
}

// randomAppealOperator 生成随机运营商（带区域ID）用于申诉测试
func randomAppealOperator(userID, regionID int64) db.Operator {
	return db.Operator{
		ID:           util.RandomInt(1, 1000),
		UserID:       userID,
		Name:         util.RandomString(10),
		ContactName:  util.RandomString(6),
		ContactPhone: util.RandomString(11),
		RegionID:     regionID,
		Status:       "active",
		CreatedAt:    time.Now(),
	}
}

// ====================== Merchant Claims Tests ======================

func TestListMerchantClaimsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	region := randomRegion()
	merchant.RegionID = region.ID

	claim := db.ListMerchantClaimsForMerchantRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "missing-item",
		Description: "缺少饮料",
		ClaimAmount: 500,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

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
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListMerchantClaimsForMerchant(gomock.Any(), db.ListMerchantClaimsForMerchantParams{
						MerchantID: merchant.ID,
						Limit:      10,
						Offset:     0,
					}).
					Times(1).
					Return([]db.ListMerchantClaimsForMerchantRow{claim}, nil)

				store.EXPECT().
					CountMerchantClaimsForMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				claims := response["claims"].([]interface{})
				require.Len(t, claims, 1)
				require.Equal(t, float64(1), response["total"])
			},
		},
		{
			name:  "NotMerchant",
			query: "?page_id=1&page_size=10",
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
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:  "InvalidPageID",
			query: "?page_id=0&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "Unauthorized",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No auth header
			},
			buildStubs: func(store *mockdb.MockStore) {},
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

			url := "/v1/merchant/claims" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Create Appeal Tests ======================

func TestCreateMerchantAppealAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	region := randomRegion()
	merchant.RegionID = region.ID

	claim := db.GetClaimForAppealRow{
		ID:          1,
		OrderID:     100,
		ClaimType:   "missing-item",
		ClaimAmount: 500,
		Status:      "approved",
		MerchantID:  merchant.ID,
		RegionID:    region.ID,
	}

	appeal := randomAppeal(claim.ID, merchant.ID, region.ID, "merchant")

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
				"claim_id":      claim.ID,
				"reason":        "订单包装完好，顾客收货时已当面核对",
				"evidence_urls": []string{"https://example.com/evidence.jpg"},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), claim.ID).
					Times(1).
					Return(claim, nil)

				store.EXPECT().
					CheckAppealExists(gomock.Any(), claim.ID).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					CreateAppeal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(appeal, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response appealResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, appeal.ID, response.ID)
				require.Equal(t, "pending", response.Status)
			},
		},
		{
			name: "ClaimNotFound",
			body: gin.H{
				"claim_id":      99999,
				"reason":        "这是一个充分的测试申诉理由",
				"evidence_urls": []string{"https://example.com/e.jpg"},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.GetClaimForAppealRow{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "AppealAlreadyExists",
			body: gin.H{
				"claim_id":      claim.ID,
				"reason":        "这是一个充分的测试申诉理由",
				"evidence_urls": []string{"https://example.com/e.jpg"},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), claim.ID).
					Times(1).
					Return(claim, nil)

				store.EXPECT().
					CheckAppealExists(gomock.Any(), claim.ID).
					Times(1).
					Return(true, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "ClaimNotBelongToMerchant",
			body: gin.H{
				"claim_id":      claim.ID,
				"reason":        "这是一个充分的测试申诉理由",
				"evidence_urls": []string{"https://example.com/e.jpg"},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				// 返回属于其他商户的索赔
				otherClaim := claim
				otherClaim.MerchantID = merchant.ID + 1
				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), claim.ID).
					Times(1).
					Return(otherClaim, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "ReasonTooShort",
			body: gin.H{
				"claim_id":      claim.ID,
				"reason":        "短",
				"evidence_urls": []string{"https://example.com/e.jpg"},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidEvidenceURL",
			body: gin.H{
				"claim_id":      claim.ID,
				"reason":        "这是一个有效的申诉理由",
				"evidence_urls": []string{"not-a-valid-url"},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
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

			url := "/v1/merchant/appeals"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== List Merchant Appeals Tests ======================

func TestListMerchantAppealsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	region := randomRegion()
	merchant.RegionID = region.ID

	appeal := db.ListMerchantAppealsForMerchantRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "merchant",
		AppellantID:      merchant.ID,
		Reason:           "测试申诉理由",
		Status:           "pending",
		RegionID:         region.ID,
		ClaimType:        "missing-item",
		ClaimAmount:      500,
		ClaimDescription: "缺少饮料",
		OrderNo:          "20240101120000123456",
		CreatedAt:        time.Now(),
	}

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
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListMerchantAppealsForMerchant(gomock.Any(), db.ListMerchantAppealsForMerchantParams{
						AppellantID: merchant.ID,
						Limit:       10,
						Offset:      0,
					}).
					Times(1).
					Return([]db.ListMerchantAppealsForMerchantRow{appeal}, nil)

				store.EXPECT().
					CountMerchantAppealsForMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				appeals := response["appeals"].([]interface{})
				require.Len(t, appeals, 1)
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

			url := "/v1/merchant/appeals" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Rider Appeals Tests ======================

func TestCreateRiderAppealAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomAppealRider(user.ID, region.ID)

	claim := db.GetClaimForAppealRow{
		ID:          1,
		OrderID:     100,
		ClaimType:   "delay",
		ClaimAmount: 300,
		Status:      "approved",
		MerchantID:  200,
		RegionID:    region.ID,
		RiderID:     pgtype.Int8{Int64: rider.ID, Valid: true},
	}

	appeal := randomAppeal(claim.ID, rider.ID, region.ID, "rider")

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
				"claim_id":      claim.ID,
				"reason":        "因恶劣天气导致配送延迟，非骑手原因",
				"evidence_urls": []string{"https://example.com/weather.jpg"},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), claim.ID).
					Times(1).
					Return(claim, nil)

				store.EXPECT().
					CheckAppealExists(gomock.Any(), claim.ID).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					CreateAppeal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(appeal, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "NotRider",
			body: gin.H{
				"claim_id":      claim.ID,
				"reason":        "因恶劣天气导致配送延迟，非骑手原因",
				"evidence_urls": []string{"https://example.com/weather.jpg"},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "ClaimNotRelatedToRider",
			body: gin.H{
				"claim_id":      claim.ID,
				"reason":        "因恶劣天气导致配送延迟，非骑手原因",
				"evidence_urls": []string{"https://example.com/weather.jpg"},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				// 返回属于其他骑手的索赔
				otherClaim := claim
				otherClaim.RiderID = pgtype.Int8{Int64: rider.ID + 1, Valid: true}
				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), claim.ID).
					Times(1).
					Return(otherClaim, nil)
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
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/rider/appeals"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Operator Review Appeal Tests ======================

func TestReviewAppealAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	operator := randomAppealOperator(user.ID, region.ID)

	appeal := db.Appeal{
		ID:            1,
		ClaimID:       100,
		AppellantType: "merchant",
		AppellantID:   200,
		Reason:        "测试申诉理由",
		Status:        "pending",
		RegionID:      region.ID,
		CreatedAt:     time.Now(),
	}

	reviewedAppeal := appeal
	reviewedAppeal.Status = "approved"
	reviewedAppeal.ReviewerID = pgtype.Int8{Int64: operator.ID, Valid: true}
	reviewedAppeal.ReviewNotes = pgtype.Text{String: "申诉理由充分，予以批准", Valid: true}
	reviewedAppeal.CompensationAmount = pgtype.Int8{Int64: 500, Valid: true}
	reviewedAppeal.ReviewedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	reviewedAppeal.CompensatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}

	appealForPostProcess := db.GetAppealForPostProcessRow{
		AppealID:       appeal.ID,
		ClaimID:        appeal.ClaimID,
		AppellantType:  "merchant",
		AppellantID:    200,
		ClaimantUserID: 300,
		ClaimType:      "missing-item",
		ClaimAmount:    500,
		OrderNo:        "20240101120000123456",
	}

	testCases := []struct {
		name          string
		appealID      int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "ApproveOK",
			appealID: appeal.ID,
			body: gin.H{
				"status":              "approved",
				"review_notes":        "申诉理由充分，予以批准",
				"compensation_amount": 500,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(operator, nil)

				store.EXPECT().
					GetAppeal(gomock.Any(), appeal.ID).
					Times(1).
					Return(appeal, nil)

				store.EXPECT().
					ReviewAppeal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(reviewedAppeal, nil)

				store.EXPECT().
					GetAppealForPostProcess(gomock.Any(), appeal.ID).
					Times(1).
					Return(appealForPostProcess, nil)

				taskDistributor.EXPECT().
					DistributeTaskProcessAppealResult(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response appealResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "approved", response.Status)
			},
		},
		{
			name:     "RejectOK",
			appealID: appeal.ID,
			body: gin.H{
				"status":       "rejected",
				"review_notes": "申诉证据不足，无法支持申诉理由",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(operator, nil)

				store.EXPECT().
					GetAppeal(gomock.Any(), appeal.ID).
					Times(1).
					Return(appeal, nil)

				rejectedAppeal := appeal
				rejectedAppeal.Status = "rejected"
				rejectedAppeal.ReviewerID = pgtype.Int8{Int64: operator.ID, Valid: true}
				rejectedAppeal.ReviewNotes = pgtype.Text{String: "申诉证据不足，无法支持申诉理由", Valid: true}
				rejectedAppeal.ReviewedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}

				store.EXPECT().
					ReviewAppeal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(rejectedAppeal, nil)

				store.EXPECT().
					GetAppealForPostProcess(gomock.Any(), appeal.ID).
					Times(1).
					Return(appealForPostProcess, nil)

				taskDistributor.EXPECT().
					DistributeTaskProcessAppealResult(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "AppealNotFound",
			appealID: 99999,
			body: gin.H{
				"status":       "rejected",
				"review_notes": "申诉证据不足",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(operator, nil)

				store.EXPECT().
					GetAppeal(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Appeal{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:     "AppealNotInOperatorRegion",
			appealID: appeal.ID,
			body: gin.H{
				"status":       "rejected",
				"review_notes": "申诉证据不足",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(operator, nil)

				// 返回属于其他区域的申诉
				otherRegionAppeal := appeal
				otherRegionAppeal.RegionID = region.ID + 1
				store.EXPECT().
					GetAppeal(gomock.Any(), appeal.ID).
					Times(1).
					Return(otherRegionAppeal, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:     "AppealAlreadyReviewed",
			appealID: appeal.ID,
			body: gin.H{
				"status":              "approved",
				"review_notes":        "申诉理由充分",
				"compensation_amount": 500,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(operator, nil)

				alreadyReviewedAppeal := appeal
				alreadyReviewedAppeal.Status = "approved"
				store.EXPECT().
					GetAppeal(gomock.Any(), appeal.ID).
					Times(1).
					Return(alreadyReviewedAppeal, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:     "ApproveWithoutCompensation",
			appealID: appeal.ID,
			body: gin.H{
				"status":       "approved",
				"review_notes": "申诉理由充分，予以批准",
				// missing compensation_amount
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(operator, nil)

				store.EXPECT().
					GetAppeal(gomock.Any(), appeal.ID).
					Times(1).
					Return(appeal, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:     "NotOperator",
			appealID: appeal.ID,
			body: gin.H{
				"status":       "rejected",
				"review_notes": "测试审核备注内容",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				// Mock for CasbinRoleMiddleware - 用户没有operator角色
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{}, nil) // 返回空角色列表
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:     "ReviewNotesTooShort",
			appealID: appeal.ID,
			body: gin.H{
				"status":       "rejected",
				"review_notes": "短",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)
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
			taskDistributor := mockwk.NewMockTaskDistributor(ctrl)
			tc.buildStubs(store, taskDistributor)

			server := newTestServerWithTaskDistributor(t, store, taskDistributor)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/operator/appeals/%d/review", tc.appealID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== List Operator Appeals Tests ======================

func TestListOperatorAppealsAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	operator := randomAppealOperator(user.ID, region.ID)

	appeal := db.ListOperatorAppealsRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "merchant",
		AppellantID:      200,
		Reason:           "测试申诉理由",
		Status:           "pending",
		RegionID:         region.ID,
		ClaimType:        "missing-item",
		ClaimAmount:      500,
		ClaimDescription: "缺少饮料",
		OrderNo:          "20240101120000123456",
		MerchantID:       200,
		MerchantName:     "测试商户",
		AppellantName:    "测试商户",
		CreatedAt:        time.Now(),
	}

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
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(operator, nil)

				store.EXPECT().
					ListOperatorAppeals(gomock.Any(), db.ListOperatorAppealsParams{
						RegionID: region.ID,
						Column2:  "",
						Limit:    10,
						Offset:   0,
					}).
					Times(1).
					Return([]db.ListOperatorAppealsRow{appeal}, nil)

				store.EXPECT().
					CountOperatorAppeals(gomock.Any(), db.CountOperatorAppealsParams{
						RegionID: region.ID,
						Column2:  "",
					}).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				appeals := response["appeals"].([]interface{})
				require.Len(t, appeals, 1)
			},
		},
		{
			name:  "FilterByStatus",
			query: "?page_id=1&page_size=10&status=pending",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(operator, nil)

				store.EXPECT().
					ListOperatorAppeals(gomock.Any(), db.ListOperatorAppealsParams{
						RegionID: region.ID,
						Column2:  "pending",
						Limit:    10,
						Offset:   0,
					}).
					Times(1).
					Return([]db.ListOperatorAppealsRow{appeal}, nil)

				store.EXPECT().
					CountOperatorAppeals(gomock.Any(), db.CountOperatorAppealsParams{
						RegionID: region.ID,
						Column2:  "pending",
					}).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidStatus",
			query: "?page_id=1&page_size=10&status=invalid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)
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

			url := "/v1/operator/appeals" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Get Merchant Claim Detail Tests ======================

func TestGetMerchantClaimDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	region := randomRegion()
	merchant.RegionID = region.ID

	claim := db.GetMerchantClaimDetailForMerchantRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "missing-item",
		Description: "缺少饮料",
		ClaimAmount: 500,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

	testCases := []struct {
		name          string
		claimID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantClaimDetailForMerchant(gomock.Any(), db.GetMerchantClaimDetailForMerchantParams{
						ID:         claim.ID,
						MerchantID: merchant.ID,
					}).
					Times(1).
					Return(claim, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "ClaimNotFound",
			claimID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantClaimDetailForMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantClaimDetailForMerchantRow{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "NotMerchant",
			claimID: claim.ID,
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
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchant/claims/%d", tc.claimID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Get Merchant Appeal Detail Tests ======================

func TestGetMerchantAppealDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	region := randomRegion()
	merchant.RegionID = region.ID

	appeal := db.GetMerchantAppealDetailRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "merchant",
		AppellantID:      merchant.ID,
		Reason:           "订单包装完好",
		Status:           "pending",
		RegionID:         region.ID,
		ClaimType:        "missing-item",
		ClaimAmount:      500,
		ClaimDescription: "缺少饮料",
		OrderNo:          "20240101120000123456",
		OrderAmount:      3000,
		UserPhone:        pgtype.Text{String: "13800138000", Valid: true},
		CreatedAt:        time.Now(),
	}

	testCases := []struct {
		name          string
		appealID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			appealID: appeal.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantAppealDetail(gomock.Any(), db.GetMerchantAppealDetailParams{
						ID:          appeal.ID,
						AppellantID: merchant.ID,
					}).
					Times(1).
					Return(appeal, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "AppealNotFound",
			appealID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), user.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetMerchantAppealDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantAppealDetailRow{}, pgx.ErrNoRows)
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

			url := fmt.Sprintf("/v1/merchant/appeals/%d", tc.appealID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== List Rider Claims Tests ======================

func TestListRiderClaimsAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomAppealRider(user.ID, region.ID)

	claim := db.ListRiderClaimsForRiderRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "delay",
		Description: "配送延迟",
		ClaimAmount: 300,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

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
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderClaimsForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListRiderClaimsForRiderRow{claim}, nil)

				store.EXPECT().
					CountRiderClaimsForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "NotRider",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)
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
			recorder := httptest.NewRecorder()

			url := "/v1/rider/claims" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== List Rider Appeals Tests ======================

func TestListRiderAppealsAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomAppealRider(user.ID, region.ID)

	appeal := db.ListRiderAppealsRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "rider",
		AppellantID:      rider.ID,
		Reason:           "恶劣天气导致延迟",
		Status:           "pending",
		RegionID:         region.ID,
		ClaimType:        "delay",
		ClaimAmount:      300,
		ClaimDescription: "配送延迟",
		OrderNo:          "20240101120000123456",
		CreatedAt:        time.Now(),
	}

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
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderAppeals(gomock.Any(), db.ListRiderAppealsParams{
						AppellantID: rider.ID,
						Limit:       10,
						Offset:      0,
					}).
					Times(1).
					Return([]db.ListRiderAppealsRow{appeal}, nil)

				store.EXPECT().
					CountRiderAppeals(gomock.Any(), rider.ID).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
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

			url := "/v1/rider/appeals" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Get Operator Appeal Detail Tests ======================

func TestGetOperatorAppealDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	operator := randomAppealOperator(user.ID, region.ID)

	appeal := db.GetOperatorAppealDetailRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "merchant",
		AppellantID:      200,
		Reason:           "订单包装完好",
		Status:           "pending",
		RegionID:         region.ID,
		ClaimType:        "missing-item",
		ClaimAmount:      500,
		ClaimDescription: "缺少饮料",
		OrderNo:          "20240101120000123456",
		OrderAmount:      3000,
		OrderStatus:      "completed",
		MerchantID:       200,
		MerchantName:     "测试商户",
		MerchantPhone:    "13900139000",
		UserPhone:        pgtype.Text{String: "13800138000", Valid: true},
		UserName:         "张三",
		CreatedAt:        time.Now(),
	}

	testCases := []struct {
		name          string
		appealID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			appealID: appeal.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(operator, nil)

				store.EXPECT().
					GetOperatorAppealDetail(gomock.Any(), db.GetOperatorAppealDetailParams{
						ID:       appeal.ID,
						RegionID: operator.RegionID,
					}).
					Times(1).
					Return(appeal, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "AppealNotFound",
			appealID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: region.ID, Valid: true}}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(operator, nil)

				store.EXPECT().
					GetOperatorAppealDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetOperatorAppealDetailRow{}, pgx.ErrNoRows)
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

			url := fmt.Sprintf("/v1/operator/appeals/%d", tc.appealID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Get Rider Claim Detail Tests ======================

func TestGetRiderClaimDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomAppealRider(user.ID, region.ID)

	claim := db.GetRiderClaimDetailForRiderRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "delay",
		Description: "配送延迟",
		ClaimAmount: 300,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

	testCases := []struct {
		name          string
		claimID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderClaimDetailForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(claim, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, float64(claim.ID), response["id"])
				require.Equal(t, claim.ClaimType, response["claim_type"])
			},
		},
		{
			name:    "ClaimNotFound",
			claimID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderClaimDetailForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetRiderClaimDetailForRiderRow{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "NotRider",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "InvalidClaimID",
			claimID: -1, // negative ID
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderClaimDetailForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetRiderClaimDetailForRiderRow{}, pgx.ErrNoRows)
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

			url := fmt.Sprintf("/v1/rider/claims/%d", tc.claimID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Get Rider Appeal Detail Tests ======================

func TestGetRiderAppealDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomAppealRider(user.ID, region.ID)

	appeal := db.GetRiderAppealDetailRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "rider",
		AppellantID:      rider.ID,
		Reason:           "恶劣天气导致延迟",
		Status:           "pending",
		RegionID:         region.ID,
		ClaimType:        "delay",
		ClaimAmount:      300,
		ClaimDescription: "配送延迟",
		OrderNo:          "20240101120000123456",
		OrderAmount:      3000,
		UserPhone:        pgtype.Text{String: "13800138000", Valid: true},
		CreatedAt:        time.Now(),
	}

	testCases := []struct {
		name          string
		appealID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			appealID: appeal.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderAppealDetail(gomock.Any(), db.GetRiderAppealDetailParams{
						ID:          appeal.ID,
						AppellantID: rider.ID,
					}).
					Times(1).
					Return(appeal, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, float64(appeal.ID), response["id"])
				require.Equal(t, "rider", response["appellant_type"])
			},
		},
		{
			name:     "AppealNotFound",
			appealID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderAppealDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetRiderAppealDetailRow{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:     "NotRider",
			appealID: appeal.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:     "InvalidAppealID",
			appealID: -1, // negative ID
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderAppealDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetRiderAppealDetailRow{}, pgx.ErrNoRows)
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

			url := fmt.Sprintf("/v1/rider/appeals/%d", tc.appealID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}
