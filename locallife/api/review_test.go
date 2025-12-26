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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateReviewAPI(t *testing.T) {
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
				"order_id": order.ID,
				"content":  "Great food and service!",
				"images":   []string{fmt.Sprintf("uploads/reviews/%d/image1.jpg", user.ID)},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock GetOrder
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)

				// Mock GetReviewByOrderID (check not exists)
				store.EXPECT().
					GetReviewByOrderID(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(db.Review{}, pgx.ErrNoRows)

				// Mock GetUserProfile (high trust score)
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
						TrustScore: 850,
					}, nil)

				// Mock GetUser (for wechat openid)
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)

				// Mock CreateReview
				store.EXPECT().
					CreateReview(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Review{
						ID:         1,
						OrderID:    order.ID,
						UserID:     user.ID,
						MerchantID: merchant.ID,
						Content:    "Great food and service!",
						Images:     []string{fmt.Sprintf("uploads/reviews/%d/image1.jpg", user.ID)},
						IsVisible:  true,
						CreatedAt:  time.Now(),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response reviewResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, order.ID, response.OrderID)
				require.Equal(t, user.ID, response.UserID)
				require.Equal(t, merchant.ID, response.MerchantID)
				require.Equal(t, "Great food and service!", response.Content)
				require.True(t, response.IsVisible)
			},
		},
		{
			name: "LowTrustScore_NotVisible",
			body: map[string]interface{}{
				"order_id": order.ID,
				"content":  "Bad review from low trust user",
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
					GetReviewByOrderID(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(db.Review{}, pgx.ErrNoRows)

				// Mock GetUserProfile (low trust score < 600)
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
						TrustScore: 500, // Low trust score
					}, nil)

				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)

				// Mock CreateReview with is_visible=false
				store.EXPECT().
					CreateReview(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.CreateReviewParams) (db.Review, error) {
						require.False(t, arg.IsVisible) // Verify is_visible is false
						return db.Review{
							ID:         1,
							OrderID:    order.ID,
							UserID:     user.ID,
							MerchantID: merchant.ID,
							Content:    "Bad review from low trust user",
							IsVisible:  false,
							CreatedAt:  time.Now(),
						}, nil
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response reviewResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.False(t, response.IsVisible) // Verify review is not visible
			},
		},
		{
			name: "OrderNotFound",
			body: map[string]interface{}{
				"order_id": 99999,
				"content":  "Great food!",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Order{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "OrderNotBelongToUser",
			body: map[string]interface{}{
				"order_id": order.ID,
				"content":  "Great food!",
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
				"order_id": order.ID,
				"content":  "Great food!",
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
			name: "AlreadyReviewed",
			body: map[string]interface{}{
				"order_id": order.ID,
				"content":  "Another review",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(order, nil)

				// Review already exists
				store.EXPECT().
					GetReviewByOrderID(gomock.Any(), gomock.Eq(order.ID)).
					Times(1).
					Return(db.Review{
						ID:      1,
						OrderID: order.ID,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"order_id": order.ID,
				"content":  "Great food!",
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
			name: "InvalidRequest_EmptyContent",
			body: map[string]interface{}{
				"order_id": order.ID,
				"content":  "",
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

			wechatClient := mockwechat.NewMockWechatClient(ctrl)
			// Wechat MsgSecCheck only runs for successful create cases
			switch tc.name {
			case "OK":
				wechatClient.EXPECT().
					MsgSecCheck(gomock.Any(), gomock.Eq(user.WechatOpenid), gomock.Eq(2), gomock.Eq("Great food and service!")).
					Times(1).
					Return(nil)
			case "LowTrustScore_NotVisible":
				wechatClient.EXPECT().
					MsgSecCheck(gomock.Any(), gomock.Eq(user.WechatOpenid), gomock.Eq(2), gomock.Eq("Bad review from low trust user")).
					Times(1).
					Return(nil)
			}

			server := newTestServerWithWechat(t, store, wechatClient)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/reviews"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantReviewsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	reviews := []db.Review{
		{
			ID:         1,
			OrderID:    1,
			UserID:     user.ID,
			MerchantID: merchant.ID,
			Content:    "Great food!",
			IsVisible:  true,
			CreatedAt:  time.Now(),
		},
		{
			ID:         2,
			OrderID:    2,
			UserID:     user.ID + 1,
			MerchantID: merchant.ID,
			Content:    "Good service!",
			IsVisible:  true,
			CreatedAt:  time.Now().Add(-time.Hour),
		},
	}

	testCases := []struct {
		name          string
		merchantID    int64
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			query:      "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListReviewsByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(reviews, nil)

				store.EXPECT().
					CountReviewsByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(int64(2), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, float64(2), response["total_count"])
				require.Equal(t, float64(1), response["page_id"])

				reviewsList := response["reviews"].([]interface{})
				require.Len(t, reviewsList, 2)
			},
		},
		{
			name:       "InvalidPageID",
			merchantID: merchant.ID,
			query:      "?page_id=0&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListReviewsByMerchant(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			merchantID: merchant.ID,
			query:      "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListReviewsByMerchant(gomock.Any(), gomock.Any()).
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

			wechatClient := mockwechat.NewMockWechatClient(ctrl)
			server := newTestServerWithWechat(t, store, wechatClient)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/reviews/merchants/%d%s", tc.merchantID, tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestReplyReviewAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	review := randomReview(user.ID+1, merchant.ID)

	testCases := []struct {
		name          string
		reviewID      int64
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			reviewID: review.ID,
			body: map[string]interface{}{
				"reply": "Thank you for your feedback!",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock GetReview
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)

				// Mock GetUserRoleByType (merchant owner)
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UserRole{
						ID:              1,
						UserID:          user.ID,
						Role:            "merchant_owner",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}, nil)

					// Mock GetUser (for wechat openid)
					store.EXPECT().
						GetUser(gomock.Any(), gomock.Eq(user.ID)).
						Times(1).
						Return(user, nil)

				// Mock UpdateMerchantReply
				store.EXPECT().
					UpdateMerchantReply(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Review{
						ID:            review.ID,
						OrderID:       review.OrderID,
						UserID:        review.UserID,
						MerchantID:    merchant.ID,
						Content:       review.Content,
						IsVisible:     review.IsVisible,
						MerchantReply: pgtype.Text{String: "Thank you for your feedback!", Valid: true},
						RepliedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
						CreatedAt:     review.CreatedAt,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response reviewResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.NotNil(t, response.MerchantReply)
				require.Equal(t, "Thank you for your feedback!", *response.MerchantReply)
			},
		},
		{
			name:     "ReviewNotFound",
			reviewID: 99999,
			body: map[string]interface{}{
				"reply": "Thank you!",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Review{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:     "NotMerchantOwner",
			reviewID: review.ID,
			body: map[string]interface{}{
				"reply": "Thank you!",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)

				// No merchant_owner role
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UserRole{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:     "WrongMerchant",
			reviewID: review.ID,
			body: map[string]interface{}{
				"reply": "Thank you!",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)

				// Different merchant
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UserRole{
						ID:              1,
						UserID:          user.ID,
						Role:            "merchant_owner",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID + 999, Valid: true},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
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

			wechatClient := mockwechat.NewMockWechatClient(ctrl)
			if tc.name == "OK" {
				wechatClient.EXPECT().
					MsgSecCheck(gomock.Any(), gomock.Eq(user.WechatOpenid), gomock.Eq(2), gomock.Eq("Thank you for your feedback!")).
					Times(1).
					Return(nil)
			}
			server := newTestServerWithWechat(t, store, wechatClient)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/reviews/%d/reply", tc.reviewID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDeleteReviewAPI(t *testing.T) {
	operatorUser, _ := randomUser(t)
	user, _ := randomUser(t)
	regionID := int64(1)
	
	// 创建带有 RegionID 的商户
	merchant := db.Merchant{
		ID:          util.RandomInt(1, 1000),
		OwnerUserID: operatorUser.ID,
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(50), Valid: true},
		Phone:       "13800138000",
		Address:     util.RandomString(30),
		Status:      "approved",
		RegionID:    regionID,
		Version:     1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	review := randomReview(user.ID, merchant.ID)
	
	operator := db.Operator{
		ID:       util.RandomInt(1, 100),
		UserID:   operatorUser.ID,
		RegionID: regionID,
		Status:   "active",
	}

	testCases := []struct {
		name          string
		reviewID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			reviewID: review.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, operatorUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), operatorUser.ID).
					Times(1).
					Return([]db.UserRole{{UserID: operatorUser.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: regionID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), gomock.Eq(operatorUser.ID)).
					Times(1).
					Return(operator, nil)

				// Mock GetReview
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)

				// Mock GetMerchant
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				// Mock checkOperatorManagesRegion in handler
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)

				// Mock DeleteReview
				store.EXPECT().
					DeleteReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "NotOperator",
			reviewID: review.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock for CasbinRoleMiddleware - 用户没有 operator 角色
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "customer", Status: "active"}}, nil)

				// 以下 mock 不应该被调用
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), gomock.Any()).
					Times(0)

				store.EXPECT().
					GetReview(gomock.Any(), gomock.Any()).
					Times(0)

				store.EXPECT().
					DeleteReview(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:     "OperatorNotManageRegion",
			reviewID: review.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, operatorUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), operatorUser.ID).
					Times(1).
					Return([]db.UserRole{{UserID: operatorUser.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: regionID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), gomock.Eq(operatorUser.ID)).
					Times(1).
					Return(operator, nil)

				// Mock GetReview
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)

				// Mock GetMerchant
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				// Mock checkOperatorManagesRegion - operator doesn't manage this region
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					DeleteReview(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:     "ReviewNotFound",
			reviewID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, operatorUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock for CasbinRoleMiddleware
				store.EXPECT().
					ListUserRoles(gomock.Any(), operatorUser.ID).
					Times(1).
					Return([]db.UserRole{{UserID: operatorUser.ID, Role: "operator", Status: "active", RelatedEntityID: pgtype.Int8{Int64: regionID, Valid: true}}}, nil)

				// Mock for LoadOperatorMiddleware
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), gomock.Eq(operatorUser.ID)).
					Times(1).
					Return(operator, nil)

				// Mock GetReview - not found
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(int64(99999))).
					Times(1).
					Return(db.Review{}, sql.ErrNoRows)

				store.EXPECT().
					DeleteReview(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/reviews/%d", tc.reviewID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// Helper functions

func randomReview(userID, merchantID int64) db.Review {
	return db.Review{
		ID:         1,
		OrderID:    1,
		UserID:     userID,
		MerchantID: merchantID,
		Content:    "Great food and service!",
		Images:     []string{"https://example.com/image1.jpg"},
		IsVisible:  true,
		CreatedAt:  time.Now(),
	}
}

func randomCompletedOrder(userID, merchantID int64) db.Order {
	return db.Order{
		ID:          1,
		OrderNo:     "20251130123456789012",
		UserID:      userID,
		MerchantID:  merchantID,
		OrderType:   "takeout",
		Status:      "completed",
		Subtotal:    10000,
		TotalAmount: 10000,
		CreatedAt:   time.Now(),
	}
}
