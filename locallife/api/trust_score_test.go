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

	"github.com/jackc/pgx/v5/pgtype"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== SubmitClaim Tests ====================

func TestSubmitClaimAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomCompletedOrder(user.ID, merchant.ID)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"order_id":     order.ID,
				"claim_type":   "damage", // 使用新的索赔类型
				"claim_amount": 1000,
				"claim_reason": "餐品在配送过程中严重损坏",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock GetOrder - handler 和自动审批算法都会调用
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					AnyTimes().
					Return(order, nil)

				// Mock ListUserClaimsInPeriod (check no existing claim)
				store.EXPECT().
					ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Claim{}, nil)

				// Mock GetDeliveryByOrderID (获取配送费)
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), gomock.Eq(order.ID)).
					AnyTimes().
					Return(db.Delivery{
						ID:          1,
						OrderID:     order.ID,
						DeliveryFee: 500, // 5元配送费
					}, nil)

				// Mock GetUserBehaviorStats (行为回溯)
				store.EXPECT().
					GetUserBehaviorStats(gomock.Any(), gomock.Eq(user.ID)).
					AnyTimes().
					Return(db.GetUserBehaviorStatsRow{
						TakeoutOrders90d: 10,  // 近3月10单
						Claims90d:        1,   // 1次索赔，正常用户
						WarningCount:     0,
						RequiresEvidence: false,
						PlatformPayCount: 0,
					}, nil)

				// 以下是信用评分算法的复杂依赖链，使用 AnyTimes() 灵活处理
				store.EXPECT().
					GetUserProfile(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(db.UserProfile{
						ID:         1,
						UserID:     user.ID,
						Role:       "customer",
						TrustScore: 100, // 新体系：100分满分
					}, nil)

				store.EXPECT().
					GetUserRecentClaims(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return([]db.Claim{}, nil)

				store.EXPECT().
					IncrementUserClaimCount(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil)

				store.EXPECT().
					CreateClaim(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Claim{
						ID:          1,
						OrderID:     order.ID,
						UserID:      user.ID,
						ClaimType:   "damage",
						ClaimAmount: 1000,
						Status:      "auto-approved",
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				// 验证返回的数据结构
				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)

				// 验证关键字段存在（高信用分用户会触发instant approval）
				require.Contains(t, response, "status")
				status := response["status"].(string)
				// 高信用分用户可能是instant或auto-approved
				require.True(t, status == "instant" || status == "auto-approved" || status == "pending",
					"expected status to be instant/auto-approved/pending, got: %s", status)
			},
		},
		{
			name: "OrderNotFound",
			body: map[string]interface{}{
				"order_id":     99999,
				"claim_type":   "damage",
				"claim_amount": 1000,
				"claim_reason": "包装破损食物洒出",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(int64(99999))).
					Times(1).
					Return(db.Order{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "OrderNotBelongToUser",
			body: map[string]interface{}{
				"order_id":     order.ID,
				"claim_type":   "damage",
				"claim_amount": 1000,
				"claim_reason": "包装破损食物洒出",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+999, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "OrderNotCompleted",
			body: map[string]interface{}{
				"order_id":     order.ID,
				"claim_type":   "damage",
				"claim_amount": 1000,
				"claim_reason": "包装破损食物洒出",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				pendingOrder := order
				pendingOrder.Status = "paid"
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(pendingOrder, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ClaimAlreadyExists",
			body: map[string]interface{}{
				"order_id":     order.ID,
				"claim_type":   "damage",
				"claim_amount": 1000,
				"claim_reason": "包装破损食物洒出",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)

				// Return existing claim for this order
				store.EXPECT().
					ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Claim{
						{
							ID:      1,
							OrderID: order.ID,
							UserID:  user.ID,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "ClaimAmountExceedsOrderTotal",
			body: map[string]interface{}{
				"order_id":     order.ID,
				"claim_type":   "damage",
				"claim_amount": order.TotalAmount + 100,
				"claim_reason": "包装破损食物洒出",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Claim{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"order_id":     order.ID,
				"claim_type":   "damage",
				"claim_amount": 1000,
				"claim_reason": "包装破损食物洒出",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "InvalidClaimType",
			body: map[string]interface{}{
				"order_id":     order.ID,
				"claim_type":   "invalid-type",
				"claim_amount": 1000,
				"claim_reason": "食物质量有问题",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ClaimReasonTooShort",
			body: map[string]interface{}{
				"order_id":     order.ID,
				"claim_type":   "damage",
				"claim_amount": 1000,
				"claim_reason": "短",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ClaimAmountNegative",
			body: map[string]interface{}{
				"order_id":     order.ID,
				"claim_type":   "damage",
				"claim_amount": -100,
				"claim_reason": "包装破损食物洒出",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Any()).
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
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/trust-score/claims"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== ListUserClaims Tests ====================

func TestListUserClaimsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	claims := []db.Claim{
		{
			ID:          1,
			OrderID:     1,
			UserID:      user.ID,
			ClaimType:   "quality",
			Description: "食物有问题",
			ClaimAmount: 1000,
			Status:      "approved",
			IsMalicious: false,
			CreatedAt:   time.Now(),
		},
		{
			ID:          2,
			OrderID:     2,
			UserID:      user.ID,
			ClaimType:   "damage",
			Description: "餐品损坏",
			ClaimAmount: 500,
			Status:      "pending",
			IsMalicious: false,
			CreatedAt:   time.Now().Add(-time.Hour),
		},
	}
	_ = merchant

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page=1&page_size=20",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserClaims(gomock.Any(), db.ListUserClaimsParams{
						UserID: user.ID,
						Limit:  20,
						Offset: 0,
					}).
					Times(1).
					Return(claims, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, float64(1), response["page"])
				require.Equal(t, float64(20), response["page_size"])

				claimsList := response["claims"].([]interface{})
				require.Len(t, claimsList, 2)
			},
		},
		{
			name:  "OK_DefaultPagination",
			query: "",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserClaims(gomock.Any(), db.ListUserClaimsParams{
						UserID: user.ID,
						Limit:  20,
						Offset: 0,
					}).
					Times(1).
					Return(claims, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_EmptyList",
			query: "?page=1&page_size=20",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserClaims(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Claim{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)

				claimsList := response["claims"].([]interface{})
				require.Len(t, claimsList, 0)
			},
		},
		{
			name:  "NoAuthorization",
			query: "?page=1&page_size=20",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserClaims(gomock.Any(), gomock.Any()).
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
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/claims%s", tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== GetClaimDetail Tests ====================

func TestGetClaimDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	claim := db.Claim{
		ID:             1,
		OrderID:        1,
		UserID:         user.ID,
		ClaimType:      "quality",
		Description:    "食物有问题",
		ClaimAmount:    1000,
		ApprovedAmount: pgtype.Int8{Int64: 1000, Valid: true},
		Status:         "approved",
		IsMalicious:    false,
		CreatedAt:      time.Now(),
	}
	_ = merchant

	testCases := []struct {
		name          string
		claimID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Eq(claim.ID)).
					Times(1).
					Return(claim, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response claimResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, claim.ID, response.ID)
				require.Equal(t, claim.OrderID, response.OrderID)
				require.Equal(t, claim.UserID, response.UserID)
				require.Equal(t, claim.ClaimType, response.ClaimType)
				require.Equal(t, claim.Status, response.Status)
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
					GetClaim(gomock.Any(), gomock.Eq(int64(99999))).
					Times(1).
					Return(db.Claim{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "ClaimNotBelongToUser",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+999, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Eq(claim.ID)).
					Times(1).
					Return(claim, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "NoAuthorization",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Any()).
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
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/claims/%d", tc.claimID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== ReviewClaim Tests ====================

func TestReviewClaimAPI(t *testing.T) {
	user, _ := randomUser(t)
	reviewerUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	claim := db.Claim{
		ID:                 1,
		OrderID:            1,
		UserID:             user.ID,
		ClaimType:          "quality",
		Description:        "食物有问题",
		ClaimAmount:        1000,
		Status:             "pending",
		ApprovalType:       pgtype.Text{String: "manual", Valid: true},
		TrustScoreSnapshot: pgtype.Int2{Int16: 500, Valid: true},
		IsMalicious:        false,
		CreatedAt:          time.Now(),
	}
	_ = merchant

	testCases := []struct {
		name          string
		claimID       int64
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "ApproveOK",
			claimID: claim.ID,
			body: map[string]interface{}{
				"approved":        true,
				"approved_amount": 1000,
				"review_note":     "审核通过，符合索赔条件",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, reviewerUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Eq(claim.ID)).
					Times(1).
					Return(claim, nil)

				store.EXPECT().
					UpdateClaimStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// Return updated claim
				updatedClaim := claim
				updatedClaim.Status = "approved"
				updatedClaim.ApprovedAmount = pgtype.Int8{Int64: 1000, Valid: true}
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Eq(claim.ID)).
					Times(1).
					Return(updatedClaim, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "RejectOK",
			claimID: claim.ID,
			body: map[string]interface{}{
				"approved":    false,
				"review_note": "审核不通过，证据不足以支持索赔",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, reviewerUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 第一次调用：获取原始索赔记录
				firstCall := store.EXPECT().
					GetClaim(gomock.Any(), gomock.Eq(claim.ID)).
					Return(claim, nil)

				store.EXPECT().
					UpdateClaimStatus(gomock.Any(), gomock.Any()).
					After(firstCall).
					Return(nil)

				// 信用分计算的复杂依赖链，使用 AnyTimes() 灵活处理
				// TrustScore 设置为 100，扣 30 分后变成 70，刚好不触发 blacklist（<70 才触发）
				store.EXPECT().
					GetUserProfileForUpdate(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(db.UserProfile{
						ID:              1,
						UserID:          user.ID,
						Role:            "customer",
						TrustScore:      100,
						MaliciousClaims: 0,
					}, nil)

				store.EXPECT().
					GetUserProfile(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(db.UserProfile{
						ID:         1,
						UserID:     user.ID,
						Role:       "customer",
						TrustScore: 100,
					}, nil)

				store.EXPECT().
					UpdateUserProfile(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil)

				store.EXPECT().
					UpdateUserTrustScore(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil)

				store.EXPECT().
					CreateTrustScoreChange(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(db.TrustScoreChange{}, nil)

				// 第二次调用：获取更新后的索赔记录
				updatedClaim := claim
				updatedClaim.Status = "rejected"
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Eq(claim.ID)).
					Return(updatedClaim, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				// 验证返回的状态
				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "rejected", response["status"])
			},
		},
		{
			name:    "ClaimNotFound",
			claimID: 99999,
			body: map[string]interface{}{
				"approved":        true,
				"approved_amount": 1000,
				"review_note":     "审核通过，已核实",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, reviewerUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Eq(int64(99999))).
					Times(1).
					Return(db.Claim{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "ClaimAlreadyReviewed",
			claimID: claim.ID,
			body: map[string]interface{}{
				"approved":        true,
				"approved_amount": 1000,
				"review_note":     "审核通过，已核实",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, reviewerUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				reviewedClaim := claim
				reviewedClaim.Status = "approved"
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Eq(claim.ID)).
					Times(1).
					Return(reviewedClaim, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "ApproveWithoutAmount",
			claimID: claim.ID,
			body: map[string]interface{}{
				"approved":    true,
				"review_note": "审核通过，已核实",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, reviewerUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Eq(claim.ID)).
					Times(1).
					Return(claim, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "ApprovedAmountExceedsClaimAmount",
			claimID: claim.ID,
			body: map[string]interface{}{
				"approved":        true,
				"approved_amount": claim.ClaimAmount + 100,
				"review_note":     "审核通过，已核实",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, reviewerUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Eq(claim.ID)).
					Times(1).
					Return(claim, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "ReviewNoteTooShort",
			claimID: claim.ID,
			body: map[string]interface{}{
				"approved":        true,
				"approved_amount": 1000,
				"review_note":     "短",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, reviewerUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "NoAuthorization",
			claimID: claim.ID,
			body: map[string]interface{}{
				"approved":        true,
				"approved_amount": 1000,
				"review_note":     "审核通过",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), gomock.Any()).
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
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/trust-score/claims/%d/review", tc.claimID)
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
// ==================== GetTrustScoreProfile Tests ====================

func TestGetTrustScoreProfileAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		role          string
		entityID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK_Customer",
			role:     "customer",
			entityID: user.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserProfile(gomock.Any(), db.GetUserProfileParams{
						UserID: user.ID,
						Role:   "customer",
					}).
					Times(1).
					Return(db.UserProfile{
						ID:         1,
						UserID:     user.ID,
						Role:       "customer",
						TrustScore: 100,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "OK_Merchant",
			role:     "merchant",
			entityID: 1,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantProfile(gomock.Any(), int64(1)).
					Times(1).
					Return(db.MerchantProfile{
						ID:         1,
						MerchantID: 1,
						TrustScore: 100,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "InvalidRole",
			role:     "invalid",
			entityID: 1,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No store call expected
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:     "ProfileNotFound",
			role:     "customer",
			entityID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserProfile(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UserProfile{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:     "NoAuthorization",
			role:     "customer",
			entityID: 1,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No store call expected
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
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/trust-score/profiles/%s/%d", tc.role, tc.entityID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== GetTrustScoreHistory Tests ====================

func TestGetTrustScoreHistoryAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		role          string
		entityID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			role:     "customer",
			entityID: user.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListEntityTrustScoreChanges(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.TrustScoreChange{
						{
							ID:                1,
							EntityType:        "customer",
							EntityID:          user.ID,
							OldScore:          100,
							NewScore:          95,
							ScoreChange:       -5,
							ReasonType:        "claim-warning",
							ReasonDescription: "5单3索赔警告",
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "InvalidRole",
			role:     "invalid",
			entityID: 1,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No store call expected
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:     "NoAuthorization",
			role:     "customer",
			entityID: 1,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No store call expected
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
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/trust-score/history/%s/%d", tc.role, tc.entityID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== SubmitRecoveryRequest Tests ====================

func TestSubmitRecoveryRequestAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_Merchant",
			body: map[string]interface{}{
				"entity_type":        "merchant",
				"entity_id":          1,
				"commitment_message": "我保证今后严格遵守平台规则，提升食品安全标准",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock recovery history check
				store.EXPECT().
					ListEntityTrustScoreChanges(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.TrustScoreChange{}, nil)

				// Mock merchant profile update
				store.EXPECT().
					UpdateMerchantProfile(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				// Mock recovery log
				store.EXPECT().
					CreateTrustScoreChange(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.TrustScoreChange{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "InvalidEntityType",
			body: map[string]interface{}{
				"entity_type":        "customer",
				"entity_id":          1,
				"commitment_message": "我保证今后遵守平台规则",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No store call expected
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "CommitmentTooShort",
			body: map[string]interface{}{
				"entity_type":        "merchant",
				"entity_id":          1,
				"commitment_message": "短",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No store call expected
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"entity_type":        "merchant",
				"entity_id":          1,
				"commitment_message": "我保证今后严格遵守平台规则",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No store call expected
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
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/trust-score/recovery"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== SubmitAppeal Tests ====================

func TestSubmitAppealAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"entity_type":   "customer",
				"entity_id":     user.ID,
				"appeal_reason": "我认为扣分不合理，因为那次订单确实存在质量问题",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No store calls - appeal is just acknowledged
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "noted", response["status"])
			},
		},
		{
			name: "InvalidEntityType",
			body: map[string]interface{}{
				"entity_type":   "invalid",
				"entity_id":     1,
				"appeal_reason": "申诉原因说明文字内容",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No store call expected
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ReasonTooShort",
			body: map[string]interface{}{
				"entity_type":   "customer",
				"entity_id":     1,
				"appeal_reason": "短",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No store call expected
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"entity_type":   "customer",
				"entity_id":     1,
				"appeal_reason": "申诉原因说明文字内容",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No store call expected
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
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/trust-score/appeals"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}