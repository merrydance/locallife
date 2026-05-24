package api

import (
	"bytes"
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
					Return(db.Review{}, db.ErrRecordNotFound)

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

						IsVisible: true,
						CreatedAt: time.Now(),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response reviewResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, order.ID, response.OrderID)
				require.Equal(t, user.ID, response.UserID)
				require.Equal(t, merchant.ID, response.MerchantID)
				require.Equal(t, "Great food and service!", response.Content)
				require.True(t, response.IsVisible)
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
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "AddReviewImageFailure",
			body: map[string]interface{}{
				"order_id":        order.ID,
				"content":         "Great food with photos!",
				"media_asset_ids": []int64{101},
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
					Return(db.Review{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)

				store.EXPECT().
					ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{101})).
					Times(1).
					Return([]db.ListMediaAssetsByIDsRow{
						reviewImageAssetRow(101, user.ID),
					}, nil)

				store.EXPECT().
					CreateReview(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Review{
						ID:         1,
						OrderID:    order.ID,
						UserID:     user.ID,
						MerchantID: merchant.ID,
						Content:    "Great food with photos!",
						IsVisible:  true,
						CreatedAt:  time.Now(),
					}, nil)

				store.EXPECT().
					AddReviewImage(gomock.Any(), gomock.Eq(db.AddReviewImageParams{
						ReviewID:     1,
						MediaAssetID: 101,
						SortOrder:    0,
					})).
					Times(1).
					Return(db.ReviewImage{}, fmt.Errorf("insert review image failed"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, "internal server error", resp.Message)
			},
		},
		{
			name: "MediaAssetNotOwned",
			body: map[string]interface{}{
				"order_id":        order.ID,
				"content":         "Great food with someone else's photo!",
				"media_asset_ids": []int64{101},
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
					Return(db.Review{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)

				asset := reviewImageAssetRow(101, user.ID+999)
				store.EXPECT().
					ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{101})).
					Times(1).
					Return([]db.ListMediaAssetsByIDsRow{asset}, nil)

				store.EXPECT().
					CreateReview(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					AddReviewImage(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
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
			case "AddReviewImageFailure", "MediaAssetNotOwned":
				wechatClient.EXPECT().
					MsgSecCheck(gomock.Any(), gomock.Eq(user.WechatOpenid), gomock.Eq(2), gomock.Eq(tc.body["content"].(string))).
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

				store.EXPECT().
					ListReviewImagesByReviews(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ReviewImage{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, float64(2), response["total"])
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// Mock GetReview
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)

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

				store.EXPECT().
					ListReviewImages(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return([]db.ReviewImage{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response reviewResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetReview(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Review{}, db.ErrRecordNotFound)
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
				expectResolveNoAccessibleMerchants(store, user.ID)

				store.EXPECT().
					GetReview(gomock.Any(), gomock.Any()).
					Times(0)
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
				otherMerchant := merchant
				otherMerchant.ID = merchant.ID + 999
				expectResolveSingleOwnedMerchant(store, user.ID, otherMerchant)

				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)
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

func TestUpdateOwnReviewAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	review := randomReview(user.ID, merchant.ID)
	updatedReview := review
	updatedReview.Content = "Updated review with fresh photos"

	testCases := []struct {
		name          string
		authUserID    int64
		reviewID      int64
		body          map[string]interface{}
		buildStubs    func(store *mockdb.MockStore)
		buildWechat   func(ctrl *gomock.Controller) *mockwechat.MockWechatClient
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			authUserID: user.ID,
			reviewID:   review.ID,
			body: map[string]interface{}{
				"content":         "Updated review with fresh photos",
				"media_asset_ids": []int64{101, 102},
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)
				store.EXPECT().
					ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{101, 102})).
					Times(1).
					Return([]db.ListMediaAssetsByIDsRow{
						reviewImageAssetRow(101, user.ID),
						reviewImageAssetRow(102, user.ID),
					}, nil)
				store.EXPECT().
					UpdateReviewTx(gomock.Any(), gomock.Eq(db.UpdateReviewTxParams{
						ID:            review.ID,
						Content:       "Updated review with fresh photos",
						MediaAssetIDs: []int64{101, 102},
					})).
					Times(1).
					Return(db.UpdateReviewTxResult{
						Review: updatedReview,
						Images: []db.ReviewImage{
							{ReviewID: review.ID, MediaAssetID: 101, SortOrder: 0},
							{ReviewID: review.ID, MediaAssetID: 102, SortOrder: 1},
						},
					}, nil)
				store.EXPECT().
					ListMediaAssetsByIDs(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListMediaAssetsByIDsRow{
						{
							ID:               101,
							ObjectKey:        "reviews/101.jpg",
							Visibility:       "public",
							MediaCategory:    "review",
							ModerationStatus: "pending",
							UploadedBy:       user.ID,
						},
						{
							ID:               102,
							ObjectKey:        "reviews/102.jpg",
							Visibility:       "public",
							MediaCategory:    "review",
							ModerationStatus: "pending",
							UploadedBy:       user.ID,
						},
					}, nil)
			},
			buildWechat: func(ctrl *gomock.Controller) *mockwechat.MockWechatClient {
				wechatClient := mockwechat.NewMockWechatClient(ctrl)
				wechatClient.EXPECT().
					MsgSecCheck(gomock.Any(), gomock.Eq(user.WechatOpenid), gomock.Eq(2), gomock.Eq("Updated review with fresh photos")).
					Times(1).
					Return(nil)
				return wechatClient
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response reviewResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, updatedReview.Content, response.Content)
				require.Equal(t, []int64{101, 102}, response.ImageAssetIDs)
				require.Len(t, response.ImageURLs, 2)
			},
		},
		{
			name:       "MediaAssetWrongCategory",
			authUserID: user.ID,
			reviewID:   review.ID,
			body: map[string]interface{}{
				"content":         "Updated review with fresh photos",
				"media_asset_ids": []int64{101},
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)
				asset := reviewImageAssetRow(101, user.ID)
				asset.MediaCategory = "dish"
				store.EXPECT().
					ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{101})).
					Times(1).
					Return([]db.ListMediaAssetsByIDsRow{asset}, nil)
				store.EXPECT().
					UpdateReviewTx(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateReviewContent(gomock.Any(), gomock.Any()).
					Times(0)
			},
			buildWechat: func(ctrl *gomock.Controller) *mockwechat.MockWechatClient {
				wechatClient := mockwechat.NewMockWechatClient(ctrl)
				wechatClient.EXPECT().
					MsgSecCheck(gomock.Any(), gomock.Eq(user.WechatOpenid), gomock.Eq(2), gomock.Eq("Updated review with fresh photos")).
					Times(1).
					Return(nil)
				return wechatClient
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "NotOwner",
			authUserID: otherUser.ID,
			reviewID:   review.ID,
			body: map[string]interface{}{
				"content": "Trying to edit someone else's review",
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)
				store.EXPECT().
					UpdateReviewTx(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateReviewContent(gomock.Any(), gomock.Any()).
					Times(0)
			},
			buildWechat: func(ctrl *gomock.Controller) *mockwechat.MockWechatClient {
				return mockwechat.NewMockWechatClient(ctrl)
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
			wechatClient := tc.buildWechat(ctrl)
			server := newTestServerWithWechat(t, store, wechatClient)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/reviews/%d", tc.reviewID)
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, tc.authUserID, time.Minute)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDeleteReviewAPI(t *testing.T) {
	operatorUser, _ := randomUser(t)
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
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
			name:     "OperatorOK",
			reviewID: review.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, operatorUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, operatorUser.ID, operator)

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

				expectOperatorManagesRegion(store, operator, regionID, true)

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
			name:     "OwnerOK",
			reviewID: review.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)

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
			name:     "NotOwnerOrOperator",
			reviewID: review.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(review.ID)).
					Times(1).
					Return(review, nil)

				store.EXPECT().
					ListUserRoles(gomock.Any(), otherUser.ID).
					Times(1).
					Return([]db.UserRole{{UserID: otherUser.ID, Role: RoleCustomer, Status: "active"}}, nil)

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
				notManagedOperator := operator
				notManagedOperator.RegionID = regionID + 999
				expectActiveOperatorAuth(store, operatorUser.ID, notManagedOperator)

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

				expectOperatorManagesRegion(store, notManagedOperator, regionID, false)

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
				// Mock GetReview - not found
				store.EXPECT().
					GetReview(gomock.Any(), gomock.Eq(int64(99999))).
					Times(1).
					Return(db.Review{}, db.ErrRecordNotFound)

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
		IsVisible:  true,
		CreatedAt:  time.Now(),
	}
}

func reviewImageAssetRow(id int64, uploadedBy int64) db.ListMediaAssetsByIDsRow {
	return db.ListMediaAssetsByIDsRow{
		ID:               id,
		ObjectKey:        fmt.Sprintf("user/review/%d/photo.jpg", id),
		Visibility:       "public",
		MediaCategory:    "review",
		UploadStatus:     "confirmed",
		ModerationStatus: "pending",
		UploadedBy:       uploadedBy,
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

// TestListMerchantReviewsAPI_WithImages 回归测试（Phase 5.3）：
// 当评价存在关联图片时，GET /v1/reviews/merchants/{id} 响应中应包含 CDN image_urls。
func TestListMerchantReviewsAPI_WithImages(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	review := db.Review{
		ID: 1, OrderID: 1, UserID: user.ID, MerchantID: merchant.ID,
		Content: "Great!", IsVisible: true, CreatedAt: time.Now(),
	}

	const imageAssetID int64 = 99
	reviewImage := db.ReviewImage{ReviewID: review.ID, MediaAssetID: imageAssetID}
	imageAsset := db.ListMediaAssetsByIDsRow{
		ID:               imageAssetID,
		ObjectKey:        "review/image/99/photo.jpg",
		Visibility:       "public",
		ModerationStatus: "approved",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListReviewsByMerchant(gomock.Any(), gomock.Any()).
		Times(1).Return([]db.Review{review}, nil)
	store.EXPECT().
		CountReviewsByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
		Times(1).Return(int64(1), nil)
	store.EXPECT().
		ListReviewImagesByReviews(gomock.Any(), gomock.Any()).
		Times(1).Return([]db.ReviewImage{reviewImage}, nil)
	store.EXPECT().
		ListMediaAssetsByIDs(gomock.Any(), gomock.Any()).
		Times(1).Return([]db.ListMediaAssetsByIDsRow{imageAsset}, nil)

	server, _ := newTestServerForMedia(t, store)

	url := fmt.Sprintf("/v1/reviews/merchants/%d?page_id=1&page_size=10", merchant.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Reviews []reviewResponse `json:"reviews"`
	}
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Len(t, response.Reviews, 1)
	require.Len(t, response.Reviews[0].ImageURLs, 1, "review 应有 1 条 image_url")
	require.Contains(t, response.Reviews[0].ImageURLs[0], "https://cdn.test.example.com")
	require.Contains(t, response.Reviews[0].ImageURLs[0], imageAsset.ObjectKey)
}

func TestListMerchantReviewsAPI_HidesPendingReviewImages(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	review := db.Review{
		ID: 1, OrderID: 1, UserID: user.ID, MerchantID: merchant.ID,
		Content: "Great!", IsVisible: true, CreatedAt: time.Now(),
	}

	const imageAssetID int64 = 99
	reviewImage := db.ReviewImage{ReviewID: review.ID, MediaAssetID: imageAssetID}
	imageAsset := db.ListMediaAssetsByIDsRow{
		ID:               imageAssetID,
		ObjectKey:        "review/image/99/photo.jpg",
		Visibility:       "public",
		MediaCategory:    "review",
		ModerationStatus: "pending",
		UploadedBy:       user.ID,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListReviewsByMerchant(gomock.Any(), gomock.Any()).
		Times(1).Return([]db.Review{review}, nil)
	store.EXPECT().
		CountReviewsByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
		Times(1).Return(int64(1), nil)
	store.EXPECT().
		ListReviewImagesByReviews(gomock.Any(), gomock.Any()).
		Times(1).Return([]db.ReviewImage{reviewImage}, nil)
	store.EXPECT().
		ListMediaAssetsByIDs(gomock.Any(), gomock.Any()).
		Times(1).Return([]db.ListMediaAssetsByIDsRow{imageAsset}, nil)

	server, _ := newTestServerForMedia(t, store)

	url := fmt.Sprintf("/v1/reviews/merchants/%d?page_id=1&page_size=10", merchant.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Reviews []reviewResponse `json:"reviews"`
	}
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Len(t, response.Reviews, 1)
	require.Empty(t, response.Reviews[0].ImageURLs, "public merchant reviews must hide pending images")
}

// TestListUserReviewsAPI_WithOwnPendingReviewImages locks the owner-view media contract:
// users may see their own just-uploaded review images before async moderation finishes.
func TestListUserReviewsAPI_WithOwnPendingReviewImages(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	review := db.ListReviewsByUserRow{
		ID:         1,
		OrderID:    1,
		OrderNo:    "202605240729390001",
		UserID:     user.ID,
		MerchantID: merchant.ID,
		Content:    "Great with photos!",
		IsVisible:  true,
		CreatedAt:  time.Now(),
	}

	const imageAssetID int64 = 101
	reviewImage := db.ReviewImage{ReviewID: review.ID, MediaAssetID: imageAssetID}
	imageAsset := db.ListMediaAssetsByIDsRow{
		ID:               imageAssetID,
		ObjectKey:        "user/review/101/photo.jpg",
		Visibility:       "public",
		MediaCategory:    "review",
		ModerationStatus: "pending",
		UploadedBy:       user.ID,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListReviewsByUser(gomock.Any(), gomock.Eq(db.ListReviewsByUserParams{
			UserID: user.ID,
			Limit:  10,
			Offset: 0,
		})).
		Times(1).Return([]db.ListReviewsByUserRow{review}, nil)
	store.EXPECT().
		CountReviewsByUser(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).Return(int64(1), nil)
	store.EXPECT().
		ListReviewImagesByReviews(gomock.Any(), gomock.Any()).
		Times(1).Return([]db.ReviewImage{reviewImage}, nil)
	store.EXPECT().
		ListMediaAssetsByIDs(gomock.Any(), gomock.Any()).
		Times(1).Return([]db.ListMediaAssetsByIDsRow{imageAsset}, nil)

	server, _ := newTestServerForMedia(t, store)

	request, err := http.NewRequest(http.MethodGet, "/v1/reviews/me?page_id=1&page_size=10", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Reviews []reviewResponse `json:"reviews"`
	}
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Len(t, response.Reviews, 1)
	require.Equal(t, review.OrderNo, response.Reviews[0].OrderNo)
	require.Len(t, response.Reviews[0].ImageURLs, 1, "my reviews should include owner-visible pending review image URL")
	require.Contains(t, response.Reviews[0].ImageURLs[0], "https://cdn.test.example.com")
	require.Contains(t, response.Reviews[0].ImageURLs[0], imageAsset.ObjectKey)
}
