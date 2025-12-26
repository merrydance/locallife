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

func TestCreateDiscountRuleAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)

	regularUser, _ := randomUser(t)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		merchantID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"name":             "满100减20",
				"min_order_amount": 10000, // 100元
				"discount_amount":  2000,  // 20元
				"valid_from":       time.Now().Format(time.RFC3339),
				"valid_until":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
			},
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock GetUserRoleByType
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UserRole{
						UserID:          merchantOwner.ID,
						Role:            "merchant",
						RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
					}, nil)

				// Mock CreateDiscountRule
				store.EXPECT().
					CreateDiscountRule(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.DiscountRule{
						ID:             1,
						MerchantID:     merchant.ID,
						Name:           "满100减20",
						MinOrderAmount: 10000,
						DiscountAmount: 2000,
						IsActive:       true,
						ValidFrom:      time.Now(),
						ValidUntil:     time.Now().Add(30 * 24 * time.Hour),
						CreatedAt:      time.Now(),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "BadRequest_DiscountExceedsMinAmount",
			body: map[string]interface{}{
				"name":             "满50减100",
				"min_order_amount": 5000,  // 50元
				"discount_amount":  10000, // 100元 (超过最低消费)
				"valid_from":       time.Now().Format(time.RFC3339),
				"valid_until":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
			},
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					CreateDiscountRule(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Forbidden_NotMerchant",
			body: map[string]interface{}{
				"name":             "满100减20",
				"min_order_amount": 10000,
				"discount_amount":  2000,
				"valid_from":       time.Now().Format(time.RFC3339),
				"valid_until":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
			},
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// Regular user trying to create discount rule
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, regularUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UserRole{}, sql.ErrNoRows)

				store.EXPECT().
					CreateDiscountRule(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "BadRequest_NegativeDiscount",
			body: map[string]interface{}{
				"name":             "负折扣",
				"min_order_amount": 10000,
				"discount_amount":  -1000, // 负数
				"valid_from":       time.Now().Format(time.RFC3339),
				"valid_until":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
			},
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserRoleByType(gomock.Any(), gomock.Any()).
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

			url := fmt.Sprintf("/v1/merchants/%d/discounts", tc.merchantID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetApplicableDiscountRulesAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(1)

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
			query:      "?order_amount=10000", // 100元订单
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.GetApplicableDiscountRulesParams{
					MerchantID:     merchant.ID,
					MinOrderAmount: 10000,
				}
				store.EXPECT().
					GetApplicableDiscountRules(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return([]db.DiscountRule{
						{
							ID:             1,
							MerchantID:     merchant.ID,
							Name:           "满100减20",
							MinOrderAmount: 10000,
							DiscountAmount: 2000,
							IsActive:       true,
							ValidFrom:      time.Now(),
							ValidUntil:     time.Now().Add(30 * 24 * time.Hour),
						},
						{
							ID:             2,
							MerchantID:     merchant.ID,
							Name:           "满100减10",
							MinOrderAmount: 10000,
							DiscountAmount: 1000,
							IsActive:       true,
							ValidFrom:      time.Now(),
							ValidUntil:     time.Now().Add(30 * 24 * time.Hour),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response []db.DiscountRule
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, 2, len(response))
			},
		},
		{
			name:       "NoRules",
			merchantID: merchant.ID,
			query:      "?order_amount=1000", // 10元订单（低于最低消费）
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetApplicableDiscountRules(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.DiscountRule{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response []db.DiscountRule
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, 0, len(response))
			},
		},
		{
			name:       "BadRequest_MissingOrderAmount",
			merchantID: merchant.ID,
			query:      "", // Missing order_amount
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetApplicableDiscountRules(gomock.Any(), gomock.Any()).
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

			url := fmt.Sprintf("/v1/merchants/%d/discounts/applicable%s", tc.merchantID, tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetBestDiscountRuleAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(1)

	testCases := []struct {
		name          string
		merchantID    int64
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK_BestDiscount",
			merchantID: merchant.ID,
			query:      "?order_amount=10000",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.GetBestDiscountRuleParams{
					MerchantID:     merchant.ID,
					MinOrderAmount: 10000,
				}
				// 满100减20
				store.EXPECT().
					GetBestDiscountRule(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(db.DiscountRule{
						ID:             1,
						MerchantID:     merchant.ID,
						Name:           "满100减20",
						MinOrderAmount: 10000,
						DiscountAmount: 2000,
						IsActive:       true,
						ValidFrom:      time.Now(),
						ValidUntil:     time.Now().Add(30 * 24 * time.Hour),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response db.DiscountRule
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, "满100减20", response.Name)
			},
		},
		{
			name:       "NoRules",
			merchantID: merchant.ID,
			query:      "?order_amount=1000",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetBestDiscountRule(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.DiscountRule{}, sql.ErrNoRows)
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

			url := fmt.Sprintf("/v1/merchants/%d/discounts/best%s", tc.merchantID, tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
