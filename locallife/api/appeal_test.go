package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/merrydance/locallife/worker"
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
		ID:             1,
		OrderID:        100,
		UserID:         200,
		ClaimType:      "missing-item",
		Description:    "缺少饮料",
		ClaimAmount:    500,
		Status:         "approved",
		OrderNo:        "20240101120000123456",
		OrderAmount:    3000,
		UserPhone:      pgtype.Text{String: "13800138000", Valid: true},
		UserName:       "张三",
		RecoveryStatus: "pending",
		CreatedAt:      time.Now(),
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantClaimsForMerchant(gomock.Any(), db.ListMerchantClaimsForMerchantParams{
						MerchantID: merchant.ID,
						Bucket:     pgtype.Text{},
						Limit:      10,
						Offset:     0,
					}).
					Times(1).
					Return([]db.ListMerchantClaimsForMerchantRow{claim}, nil)

				store.EXPECT().
					CountMerchantClaimsForMerchant(gomock.Any(), db.CountMerchantClaimsForMerchantParams{
						MerchantID: merchant.ID,
						Bucket:     pgtype.Text{},
					}).
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
				require.Equal(t, false, response["has_more"])
			},
		},
		{
			name:  "OKWithBucketFilter",
			query: "?page_id=1&page_size=10&bucket=pending_action",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantClaimsForMerchant(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.ListMerchantClaimsForMerchantParams) ([]db.ListMerchantClaimsForMerchantRow, error) {
						require.Equal(t, merchant.ID, arg.MerchantID)
						require.True(t, arg.Bucket.Valid)
						require.Equal(t, "pending_action", arg.Bucket.String)
						require.Equal(t, int32(10), arg.Limit)
						require.Equal(t, int32(0), arg.Offset)
						return []db.ListMerchantClaimsForMerchantRow{claim}, nil
					})

				store.EXPECT().
					CountMerchantClaimsForMerchant(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.CountMerchantClaimsForMerchantParams) (int64, error) {
						require.Equal(t, merchant.ID, arg.MerchantID)
						require.True(t, arg.Bucket.Valid)
						require.Equal(t, "pending_action", arg.Bucket.String)
						return int64(1), nil
					})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				claims := response["claims"].([]interface{})
				require.Len(t, claims, 1)
			},
		},
		{
			name:  "NotMerchant",
			query: "?page_id=1&page_size=10",
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
		CreatedAt:   time.Now(),
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
				"claim_id": claim.ID,
				"reason":   "订单包装完好，顾客收货时已当面核对",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), claim.ID).
					Times(1).
					Return(claim, nil)

				store.EXPECT().
					CheckAppealExists(gomock.Any(), db.CheckAppealExistsParams{
						ClaimID:       claim.ID,
						AppellantType: "merchant",
					}).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					CreateAppeal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(appeal, nil)

				store.EXPECT().
					GetClaimRecoveryByClaimID(gomock.Any(), claim.ID).
					Times(1).
					Return(db.ClaimRecovery{}, db.ErrRecordNotFound)
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
				"claim_id": 99999,
				"reason":   "这是一个充分的测试申诉理由",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.GetClaimForAppealRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "AppealAlreadyExists",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "这是一个充分的测试申诉理由",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetClaimForAppeal(gomock.Any(), claim.ID).
					Times(1).
					Return(claim, nil)

				store.EXPECT().
					CheckAppealExists(gomock.Any(), db.CheckAppealExistsParams{
						ClaimID:       claim.ID,
						AppellantType: "merchant",
					}).
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
				"claim_id": claim.ID,
				"reason":   "这是一个充分的测试申诉理由",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				"claim_id": claim.ID,
				"reason":   "短",
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantAppealsForMerchant(gomock.Any(), db.ListMerchantAppealsForMerchantParams{
						AppellantID: merchant.ID,
						Limit:       10,
						Offset:      0,
					}).
					Times(1).
					Return([]db.ListMerchantAppealsForMerchantRow{appeal}, nil)

				store.EXPECT().
					CountMerchantAppealsForMerchant(gomock.Any(), db.CountMerchantAppealsForMerchantParams{
						AppellantID: merchant.ID,
						Status:      pgtype.Text{},
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
				require.Equal(t, false, response["has_more"])
			},
		},
		{
			name:  "OKWithStatusFilter",
			query: "?page_id=1&page_size=10&status=pending",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantAppealsForMerchant(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.ListMerchantAppealsForMerchantParams) ([]db.ListMerchantAppealsForMerchantRow, error) {
						require.Equal(t, merchant.ID, arg.AppellantID)
						require.True(t, arg.Status.Valid)
						require.Equal(t, "pending", arg.Status.String)
						require.Equal(t, int32(10), arg.Limit)
						require.Equal(t, int32(0), arg.Offset)
						return []db.ListMerchantAppealsForMerchantRow{appeal}, nil
					})

				store.EXPECT().
					CountMerchantAppealsForMerchant(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.CountMerchantAppealsForMerchantParams) (int64, error) {
						require.Equal(t, merchant.ID, arg.AppellantID)
						require.True(t, arg.Status.Valid)
						require.Equal(t, "pending", arg.Status.String)
						return int64(1), nil
					})
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
		CreatedAt:   time.Now(),
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
				"claim_id": claim.ID,
				"reason":   "因恶劣天气导致配送延迟，非骑手原因",
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
					CheckAppealExists(gomock.Any(), db.CheckAppealExistsParams{
						ClaimID:       claim.ID,
						AppellantType: "rider",
					}).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					CreateAppeal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(appeal, nil)

				store.EXPECT().
					GetClaimRecoveryByClaimID(gomock.Any(), claim.ID).
					Times(1).
					Return(db.ClaimRecovery{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "NotRider",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "因恶劣天气导致配送延迟，非骑手原因",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "ClaimNotRelatedToRider",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "因恶劣天气导致配送延迟，非骑手原因",
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
	reviewResult := db.ReviewAppealWithCompensationTxResult{
		Appeal:      reviewedAppeal,
		PostProcess: appealForPostProcess,
		CompensationAction: &db.BehaviorAction{
			ID: 88,
		},
	}
	approvedWithoutCompensation := reviewedAppeal
	approvedWithoutCompensation.CompensationAmount = pgtype.Int8{}
	approvedWithoutCompensationResult := db.ReviewAppealWithCompensationTxResult{
		Appeal:      approvedWithoutCompensation,
		PostProcess: appealForPostProcess,
	}

	testCases := []struct {
		name          string
		appealID      int64
		body          gin.H
		withTransfer  bool
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
			withTransfer: true,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				expectActiveOperatorAuth(store, user.ID, operator)

				store.EXPECT().
					GetAppeal(gomock.Any(), appeal.ID).
					Times(1).
					Return(appeal, nil)
				expectOperatorManagesRegion(store, operator, appeal.RegionID, true)

				store.EXPECT().
					ReviewAppealWithCompensationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(reviewResult, nil)

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
			name:     "ApproveWithoutCompensationOK",
			appealID: appeal.ID,
			body: gin.H{
				"status":       "approved",
				"review_notes": "申诉成立，撤销判责与追偿",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				expectActiveOperatorAuth(store, user.ID, operator)

				store.EXPECT().
					GetAppeal(gomock.Any(), appeal.ID).
					Times(1).
					Return(appeal, nil)
				expectOperatorManagesRegion(store, operator, appeal.RegionID, true)

				store.EXPECT().
					ReviewAppealWithCompensationTx(gomock.Any(), db.ReviewAppealWithCompensationTxParams{
						ID:                 appeal.ID,
						Status:             "approved",
						ReviewerID:         pgtype.Int8{Int64: operator.ID, Valid: true},
						ReviewNotes:        pgtype.Text{String: "申诉成立，撤销判责与追偿", Valid: true},
						CompensationAmount: pgtype.Int8{},
					}).
					Times(1).
					Return(approvedWithoutCompensationResult, nil)

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
				require.Nil(t, response.CompensationAmount)
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
				expectActiveOperatorAuth(store, user.ID, operator)

				store.EXPECT().
					GetAppeal(gomock.Any(), appeal.ID).
					Times(1).
					Return(appeal, nil)
				expectOperatorManagesRegion(store, operator, appeal.RegionID, true)

				rejectedAppeal := appeal
				rejectedAppeal.Status = "rejected"
				rejectedAppeal.ReviewerID = pgtype.Int8{Int64: operator.ID, Valid: true}
				rejectedAppeal.ReviewNotes = pgtype.Text{String: "申诉证据不足，无法支持申诉理由", Valid: true}
				rejectedAppeal.ReviewedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}

				store.EXPECT().
					ReviewAppealWithCompensationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ReviewAppealWithCompensationTxResult{Appeal: rejectedAppeal, PostProcess: appealForPostProcess}, nil)

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
				expectActiveOperatorAuth(store, user.ID, operator)

				store.EXPECT().
					GetAppeal(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Appeal{}, db.ErrRecordNotFound)
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
				expectActiveOperatorAuth(store, user.ID, operator)

				// 返回属于其他区域的申诉
				otherRegionAppeal := appeal
				otherRegionAppeal.RegionID = region.ID + 1
				store.EXPECT().
					GetAppeal(gomock.Any(), appeal.ID).
					Times(1).
					Return(otherRegionAppeal, nil)
				expectOperatorManagesRegion(store, operator, otherRegionAppeal.RegionID, false)
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
				expectActiveOperatorAuth(store, user.ID, operator)

				alreadyReviewedAppeal := appeal
				alreadyReviewedAppeal.Status = "approved"
				store.EXPECT().
					GetAppeal(gomock.Any(), appeal.ID).
					Times(1).
					Return(alreadyReviewedAppeal, nil)
				expectOperatorManagesRegion(store, operator, alreadyReviewedAppeal.RegionID, true)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:     "ApproveWithZeroCompensationInvalid",
			appealID: appeal.ID,
			body: gin.H{
				"status":              "approved",
				"review_notes":        "申诉理由充分，予以批准",
				"compensation_amount": 0,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				expectActiveOperatorAuth(store, user.ID, operator)
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
				expectActiveOperatorAuth(store, user.ID, operator)
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
			if tc.withTransfer {
				server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))
			}
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

func TestReviewAppealAPI_ApprovedCompensationRequiresTransferClient(t *testing.T) {
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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	store.EXPECT().GetAppeal(gomock.Any(), appeal.ID).
		Times(1).
		Return(appeal, nil)
	expectOperatorManagesRegion(store, operator, appeal.RegionID, true)

	server := newTestServer(t, store)

	body, err := json.Marshal(gin.H{
		"status":              "approved",
		"review_notes":        "申诉理由充分，予以批准",
		"compensation_amount": 500,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/operator/appeals/1/review", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrAppealCompensationUnavailable.Code, resp.Code)
	require.Equal(t, ErrAppealCompensationUnavailable.Message, resp.Error)
}

func TestReviewAppealAPI_NoopDistributorFallsBackInline(t *testing.T) {
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
	approvedAppeal := appeal
	approvedAppeal.Status = "approved"
	approvedAppeal.ReviewerID = pgtype.Int8{Int64: operator.ID, Valid: true}
	approvedAppeal.ReviewNotes = pgtype.Text{String: "申诉成立，撤销判责与追偿", Valid: true}
	approvedAppeal.ReviewedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	store.EXPECT().GetAppeal(gomock.Any(), appeal.ID).
		Times(1).
		Return(appeal, nil)
	expectOperatorManagesRegion(store, operator, appeal.RegionID, true)
	store.EXPECT().ReviewAppealWithCompensationTx(gomock.Any(), db.ReviewAppealWithCompensationTxParams{
		ID:                 appeal.ID,
		Status:             "approved",
		ReviewerID:         pgtype.Int8{Int64: operator.ID, Valid: true},
		ReviewNotes:        pgtype.Text{String: "申诉成立，撤销判责与追偿", Valid: true},
		CompensationAmount: pgtype.Int8{},
	}).
		Times(1).
		Return(db.ReviewAppealWithCompensationTxResult{
			Appeal: approvedAppeal,
			PostProcess: db.GetAppealForPostProcessRow{
				AppealID:       appeal.ID,
				ClaimID:        0,
				AppellantType:  "merchant",
				AppellantID:    appeal.AppellantID,
				ClaimantUserID: 300,
				ClaimType:      "missing-item",
				ClaimAmount:    500,
				OrderNo:        "20240101120000123456",
			},
		}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), appeal.AppellantID).
		Times(1).
		Return(db.Merchant{ID: appeal.AppellantID, OwnerUserID: 901}, nil)

	server := newTestServer(t, store)

	body, err := json.Marshal(gin.H{
		"status":       "approved",
		"review_notes": "申诉成立，撤销判责与追偿",
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/operator/appeals/1/review", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response appealResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "approved", response.Status)
	require.Nil(t, response.CompensationAmount)
}

func TestReviewAppealAPI_InlineCompensationFailureReturns500(t *testing.T) {
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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	store.EXPECT().GetAppeal(gomock.Any(), appeal.ID).
		Times(1).
		Return(appeal, nil)
	expectOperatorManagesRegion(store, operator, appeal.RegionID, true)
	store.EXPECT().ReviewAppealWithCompensationTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.ReviewAppealWithCompensationTxResult{
			Appeal: reviewedAppeal,
			PostProcess: db.GetAppealForPostProcessRow{
				AppealID:       appeal.ID,
				ClaimID:        0,
				AppellantType:  "merchant",
				AppellantID:    appeal.AppellantID,
				ClaimantUserID: 300,
				ClaimType:      "missing-item",
				ClaimAmount:    500,
				OrderNo:        "20240101120000123456",
			},
			CompensationAction: &db.BehaviorAction{ID: 88},
		}, nil)
	taskDistributor.EXPECT().DistributeTaskProcessAppealResult(gomock.Any(), gomock.Any()).
		Times(1).
		Return(errors.New("queue unavailable"))
	store.EXPECT().GetBehaviorAction(gomock.Any(), int64(88)).
		Times(1).
		Return(db.BehaviorAction{}, errors.New("load action failed"))

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))

	body, err := json.Marshal(gin.H{
		"status":              "approved",
		"review_notes":        "申诉理由充分，予以批准",
		"compensation_amount": 500,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/operator/appeals/1/review", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestReviewAppealAPI_InlineCompensationWechatErrorReturnsSemanticError(t *testing.T) {
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

	detailBytes, err := json.Marshal(workerClaimPayoutActionDetailForTest(appeal.ID, 300, 500))
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	taskDistributor := mockwk.NewMockTaskDistributor(ctrl)
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	store.EXPECT().GetAppeal(gomock.Any(), appeal.ID).
		Times(1).
		Return(appeal, nil)
	expectOperatorManagesRegion(store, operator, appeal.RegionID, true)
	store.EXPECT().ReviewAppealWithCompensationTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.ReviewAppealWithCompensationTxResult{
			Appeal: reviewedAppeal,
			PostProcess: db.GetAppealForPostProcessRow{
				AppealID:       appeal.ID,
				ClaimID:        0,
				AppellantType:  "merchant",
				AppellantID:    appeal.AppellantID,
				ClaimantUserID: 300,
				ClaimType:      "missing-item",
				ClaimAmount:    500,
				OrderNo:        "20240101120000123456",
			},
			CompensationAction: &db.BehaviorAction{ID: 88},
		}, nil)
	taskDistributor.EXPECT().DistributeTaskProcessAppealResult(gomock.Any(), gomock.Any()).
		Times(1).
		Return(errors.New("queue unavailable"))
	store.EXPECT().GetBehaviorAction(gomock.Any(), int64(88)).
		Times(1).
		Return(db.BehaviorAction{ID: 88, ActionType: "payout", TargetEntity: "user", Status: "created", Detail: detailBytes}, nil)
	store.EXPECT().GetUser(gomock.Any(), int64(300)).
		Times(1).
		Return(db.User{ID: 300, WechatOpenid: "openid-300", FullName: "张三"}, nil)
	transferClient.EXPECT().GetAppID().Times(1).Return("wx-mini-app")
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).Times(1).Return(nil)
	transferClient.EXPECT().CreateTransfer(gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil, &wechat.WechatPayError{Code: "NOT_ENOUGH", Message: "余额不足", StatusCode: http.StatusForbidden})
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).Times(1).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.SetTransferClientForTest(transferClient)

	body, err := json.Marshal(gin.H{
		"status":              "approved",
		"review_notes":        "申诉理由充分，予以批准",
		"compensation_amount": 500,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/operator/appeals/1/review", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)

	var resp apiTestEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "商户转账余额不足，当前无法完成企业赔付，请联系平台处理", resp.Message)
}

func workerClaimPayoutActionDetailForTest(appealID, userID, amount int64) map[string]any {
	return map[string]any{
		"appeal_id":      appealID,
		"user_id":        userID,
		"amount":         amount,
		"source_type":    "appeal",
		"source_id":      appealID,
		"remark":         "申诉补偿",
		"last_error":     "",
		"out_bill_no":    "",
		"transfer_state": "",
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
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagesRegion(store, operator, region.ID, true)

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
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagesRegion(store, operator, region.ID, true)

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
				expectActiveOperatorAuth(store, user.ID, operator)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMerchantClaimDetailForMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantClaimDetailForMerchantRow{}, db.ErrRecordNotFound)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMerchantAppealDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantAppealDetailRow{}, db.ErrRecordNotFound)
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
		ID:             1,
		OrderID:        100,
		UserID:         200,
		ClaimType:      "delay",
		Description:    "配送延迟",
		ClaimAmount:    300,
		Status:         "approved",
		RecoveryStatus: "pending",
		OrderNo:        "20240101120000123456",
		OrderAmount:    3000,
		UserPhone:      pgtype.Text{String: "13800138000", Valid: true},
		UserName:       "张三",
		CreatedAt:      time.Now(),
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

				var response merchantClaimsListResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.Claims, 1)
				require.NotNil(t, response.Claims[0].RecoveryStatus)
				require.Equal(t, claim.RecoveryStatus, *response.Claims[0].RecoveryStatus)
			},
		},
		{
			name:  "WithBucket",
			query: "?page_id=2&page_size=5&bucket=pending_action",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderClaimsForRider(gomock.Any(), gomock.Eq(db.ListRiderClaimsForRiderParams{
						RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
						Limit:   5,
						Offset:  5,
						Bucket:  pgtype.Text{String: "pending_action", Valid: true},
					})).
					Times(1).
					Return([]db.ListRiderClaimsForRiderRow{claim}, nil)

				store.EXPECT().
					CountRiderClaimsForRider(gomock.Any(), gomock.Eq(db.CountRiderClaimsForRiderParams{
						RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
						Bucket:  pgtype.Text{String: "pending_action", Valid: true},
					})).
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
					Return(db.Rider{}, db.ErrRecordNotFound)
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
				expectActiveOperatorAuth(store, user.ID, operator)
				store.EXPECT().
					GetAppeal(gomock.Any(), appeal.ID).
					Times(1).
					Return(db.Appeal{ID: appeal.ID, RegionID: operator.RegionID}, nil)
				expectOperatorManagesRegion(store, operator, operator.RegionID, true)
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
				expectActiveOperatorAuth(store, user.ID, operator)
				store.EXPECT().
					GetAppeal(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Appeal{}, db.ErrRecordNotFound)
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
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
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
					Return(db.GetRiderClaimDetailForRiderRow{}, db.ErrRecordNotFound)
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
					Return(db.Rider{}, db.ErrRecordNotFound)
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
					Return(db.GetRiderClaimDetailForRiderRow{}, db.ErrRecordNotFound)
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

func TestGetRiderClaimDecisionAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomAppealRider(user.ID, region.ID)

	claim := db.GetRiderClaimDetailForRiderRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "damage",
		Description: "餐损",
		ClaimAmount: 300,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

	decision := db.BehaviorDecision{
		ID:                 11,
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		DecisionStatus:     "decided",
		ReasonCodes:        []string{"instant", "normal"},
		TraceSummary:       pgtype.Text{String: "骑手责任，平台已先赔", Valid: true},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
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

				store.EXPECT().
					ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).
					Times(1).
					Return([]db.BehaviorDecision{decision}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response merchantClaimDecisionResult
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.NotNil(t, response.Decision)
				require.Equal(t, decision.ID, response.Decision.DecisionID)
				require.Equal(t, decision.ResponsibleParty, response.Decision.ResponsibleParty)
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
					Return(db.GetRiderClaimDetailForRiderRow{}, db.ErrRecordNotFound)
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
					Return(db.Rider{}, db.ErrRecordNotFound)
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

			url := fmt.Sprintf("/v1/rider/claims/%d/decision", tc.claimID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestGetRiderClaimDecisionAPI_ReadOnlyConsumerDoesNotCreateBehaviorAction(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomAppealRider(user.ID, region.ID)

	claim := db.GetRiderClaimDetailForRiderRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "damage",
		Description: "餐损",
		ClaimAmount: 300,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

	decision := db.BehaviorDecision{
		ID:                 11,
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		DecisionStatus:     "decided",
		ReasonCodes:        []string{"rider_recovery"},
		TraceSummary:       pgtype.Text{String: "骑手责任，平台已先赔", Valid: true},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetRiderByUserID(gomock.Any(), user.ID).Times(1).Return(rider, nil)
	store.EXPECT().GetRiderClaimDetailForRider(gomock.Any(), gomock.Any()).Times(1).Return(claim, nil)
	store.EXPECT().ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).Times(1).Return([]db.BehaviorDecision{decision}, nil)
	store.EXPECT().CreateBehaviorAction(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	url := fmt.Sprintf("/v1/rider/claims/%d/decision", claim.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response merchantClaimDecisionResult
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.NotNil(t, response.Decision)
	require.Equal(t, decision.ID, response.Decision.DecisionID)
}

func TestGetMerchantClaimDecisionAPI_ReadOnlyConsumerDoesNotCreateBehaviorAction(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	claim := db.GetMerchantClaimDetailForMerchantRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "foreign-object",
		Description: "异物",
		ClaimAmount: 300,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

	decision := db.BehaviorDecision{
		ID:                 21,
		ResponsibleParty:   "merchant",
		CompensationSource: "merchant",
		DecisionStatus:     "decided",
		ReasonCodes:        []string{"merchant_recovery"},
		TraceSummary:       pgtype.Text{String: "商户责任，平台已先赔", Valid: true},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantClaimDetailForMerchant(gomock.Any(), db.GetMerchantClaimDetailForMerchantParams{
		ID:         claim.ID,
		MerchantID: merchant.ID,
	}).Times(1).Return(claim, nil)
	store.EXPECT().ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).Times(1).Return([]db.BehaviorDecision{decision}, nil)
	store.EXPECT().CreateBehaviorAction(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	url := fmt.Sprintf("/v1/merchant/claims/%d/decision", claim.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response merchantClaimDecisionResult
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.NotNil(t, response.Decision)
	require.Equal(t, decision.ID, response.Decision.DecisionID)
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
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
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
					Return(db.GetRiderAppealDetailRow{}, db.ErrRecordNotFound)
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
					Return(db.Rider{}, db.ErrRecordNotFound)
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
					Return(db.GetRiderAppealDetailRow{}, db.ErrRecordNotFound)
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

func TestBuildAppealNotificationContent_ApprovedClaimantKeepsExistingPayout(t *testing.T) {
	title, content := buildAppealNotificationContent(&worker.ProcessAppealResultPayload{
		Status:    "approved",
		OrderNo:   "20240101120000123456",
		ClaimType: "damage",
	}, false)

	require.Equal(t, "索赔申诉结果通知", title)
	require.Contains(t, content, "已发放赔付不再向您追回")
	require.NotContains(t, content, "相关赔付已撤回")
}
