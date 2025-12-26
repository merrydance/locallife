package api

import (
	"bytes"
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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 行为埋点测试 ====================

func TestTrackBehaviorAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchantID := util.RandomInt(1, 100)
	dish := randomDish(merchantID, nil)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_TrackDishView",
			body: map[string]interface{}{
				"behavior_type": "view",
				"dish_id":       dish.ID,
				"duration":      30,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					TrackBehavior(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.TrackBehaviorParams) (db.UserBehavior, error) {
						require.Equal(t, user.ID, arg.UserID)
						require.Equal(t, "view", arg.BehaviorType)
						require.Equal(t, dish.ID, arg.DishID.Int64)
						require.True(t, arg.DishID.Valid)
						return db.UserBehavior{
							ID:           1,
							UserID:       user.ID,
							BehaviorType: "view",
							DishID:       pgtype.Int8{Int64: dish.ID, Valid: true},
							Duration:     pgtype.Int4{Int32: 30, Valid: true},
							CreatedAt:    time.Now(),
						}, nil
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "BadRequest_NoBehaviorType",
			body: map[string]interface{}{
				"dish_id": dish.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					TrackBehavior(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "BadRequest_NoRelatedEntity",
			body: map[string]interface{}{
				"behavior_type": "view",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					TrackBehavior(gomock.Any(), gomock.Any()).
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

			url := "/v1/behaviors/track"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 推荐菜品测试 ====================

func TestRecommendDishesAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK_GenerateRecommendations",
			query: "?limit=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock GetUserPreferences (called twice: once in API, once in algorithm)
				store.EXPECT().
					GetUserPreferences(gomock.Any(), gomock.Eq(user.ID)).
					Times(2).
					Return(db.UserPreference{
						ID:                1,
						UserID:            user.ID,
						PurchaseFrequency: 10,
					}, nil)

				// Mock algorithm queries
				store.EXPECT().
					GetDishIDsByCuisines(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return([]int64{1, 2, 3}, nil)

				store.EXPECT().
					GetExploreDishes(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return([]db.GetExploreDishesRow{{ID: 4}, {ID: 5}}, nil)

				store.EXPECT().
					GetRandomDishes(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return([]int64{6}, nil)

				store.EXPECT().
					GetPopularDishes(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return([]db.GetPopularDishesRow{}, nil)

				// Mock SaveRecommendations
				store.EXPECT().
					SaveRecommendations(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Recommendation{
						ID:        1,
						UserID:    user.ID,
						DishIds:   []int64{1, 2, 3, 4, 5, 6},
						Algorithm: "ee-algorithm",
						ExpiredAt: time.Now().Add(5 * time.Minute),
					}, nil)

				// Mock GetDishesWithMerchantByIDs
				store.EXPECT().
					GetDishesWithMerchantByIDs(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetDishesWithMerchantByIDsRow{
						{
							ID:               1,
							MerchantID:       100,
							Name:             "Dish 1",
							Price:            1000,
							IsAvailable:      true,
							MerchantName:     "Test Merchant",
							MerchantRegionID: 1,
							MonthlySales:     50,
						},
					}, nil)

				// Mock ListDishTags
				store.EXPECT().
					ListDishTags(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return([]db.Tag{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response struct {
					Code    int                     `json:"code"`
					Message string                  `json:"message"`
					Data    recommendDishesResponse `json:"data"`
				}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, 0, response.Code)
				require.Equal(t, "ok", response.Message)
				require.Equal(t, "ee-algorithm", response.Data.Algorithm)
			},
		},
		{
			name:  "BadRequest_InvalidLimit",
			query: "?limit=100", // 超过max=50
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 无需任何mock，参数验证失败
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

			url := "/v1/recommendations/dishes" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 推荐配置测试 ====================

func TestGetRecommendationConfigAPI(t *testing.T) {
	user, _ := randomUser(t)
	operatorUser, _ := randomUser(t)
	regionID := int64(1)
	operator := db.Operator{
		ID:       util.RandomInt(1, 100),
		UserID:   operatorUser.ID,
		RegionID: regionID,
		Status:   "active",
	}

	testCases := []struct {
		name          string
		regionID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK_ExistingConfig",
			regionID: regionID,
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
					GetOperatorByUser(gomock.Any(), operatorUser.ID).
					Times(1).
					Return(operator, nil)

				// Mock for ValidateOperatorRegionMiddleware
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)

				// Mock GetRecommendationConfig
				store.EXPECT().
					GetRecommendationConfig(gomock.Any(), gomock.Eq(regionID)).
					Times(1).
					Return(db.RecommendationConfig{
						ID:                1,
						RegionID:          regionID,
						ExploitationRatio: testNumericFromFloat(0.60),
						ExplorationRatio:  testNumericFromFloat(0.30),
						RandomRatio:       testNumericFromFloat(0.10),
						AutoAdjust:        false,
						UpdatedAt:         time.Now(),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response getRecommendationConfigResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, regionID, response.RegionID)
				// pgtype.Numeric转换可能需要更宽松的精度检查
				require.Greater(t, response.ExploitationRatio, 0.59)
				require.Less(t, response.ExploitationRatio, 0.61)
				require.Greater(t, response.ExplorationRatio, 0.29)
				require.Less(t, response.ExplorationRatio, 0.31)
				require.Greater(t, response.RandomRatio, 0.09)
				require.Less(t, response.RandomRatio, 0.11)
			},
		},
		{
			name:     "OK_DefaultConfig",
			regionID: regionID,
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
					GetOperatorByUser(gomock.Any(), operatorUser.ID).
					Times(1).
					Return(operator, nil)

				// Mock for ValidateOperatorRegionMiddleware
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)

				store.EXPECT().
					GetRecommendationConfig(gomock.Any(), gomock.Eq(regionID)).
					Times(1).
					Return(db.RecommendationConfig{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response getRecommendationConfigResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, regionID, response.RegionID)
				require.InDelta(t, 0.60, response.ExploitationRatio, 0.01) // default
			},
		},
		{
			name:     "Forbidden_NotOperator",
			regionID: regionID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock for CasbinRoleMiddleware - 用户没有 operator 角色
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{{UserID: user.ID, Role: "customer", Status: "active"}}, nil)

				// 以下 mock 不应该被调用，因为在 CasbinRoleMiddleware 就被拒绝了
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(0)

				store.EXPECT().
					GetRecommendationConfig(gomock.Any(), gomock.Any()).
					Times(0)
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

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/regions/%d/recommendation-config", tc.regionID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateRecommendationConfigAPI(t *testing.T) {
	operatorUser, _ := randomUser(t)
	user, _ := randomUser(t)
	regionID := int64(1)
	operator := randomOperatorForRecommendation(operatorUser.ID, regionID)

	testCases := []struct {
		name          string
		regionID      int64
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK_UpdateRatios",
			regionID: regionID,
			body: map[string]interface{}{
				"exploitation_ratio": 0.50,
				"exploration_ratio":  0.40,
				"random_ratio":       0.10,
			},
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

				// Mock for ValidateOperatorRegionMiddleware
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)

				store.EXPECT().
					GetRecommendationConfig(gomock.Any(), gomock.Eq(regionID)).
					Times(1).
					Return(db.RecommendationConfig{}, pgx.ErrNoRows)

				store.EXPECT().
					UpsertRecommendationConfig(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.RecommendationConfig{
						ID:                1,
						RegionID:          regionID,
						ExploitationRatio: testNumericFromFloat(0.50),
						ExplorationRatio:  testNumericFromFloat(0.40),
						RandomRatio:       testNumericFromFloat(0.10),
						AutoAdjust:        false,
						UpdatedAt:         time.Now(),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response getRecommendationConfigResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Greater(t, response.ExploitationRatio, 0.49)
				require.Less(t, response.ExploitationRatio, 0.51)
				require.Greater(t, response.ExplorationRatio, 0.39)
				require.Less(t, response.ExplorationRatio, 0.41)
			},
		},
		{
			name:     "BadRequest_InvalidRatioSum",
			regionID: regionID,
			body: map[string]interface{}{
				"exploitation_ratio": 0.50,
				"exploration_ratio":  0.40,
				"random_ratio":       0.20, // sum = 1.10
			},
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

				// Mock for ValidateOperatorRegionMiddleware
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)

				store.EXPECT().
					GetRecommendationConfig(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:     "Forbidden_NotOperator",
			regionID: regionID,
			body: map[string]interface{}{
				"exploitation_ratio": 0.50,
			},
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
					GetOperatorByUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(0)

				store.EXPECT().
					UpsertRecommendationConfig(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:     "Forbidden_OperatorNotManageRegion",
			regionID: int64(999), // Region not managed by operator
			body: map[string]interface{}{
				"exploitation_ratio": 0.50,
			},
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

				// Mock for ValidateOperatorRegionMiddleware - 不管理该区域
				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   int64(999),
					}).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					UpsertRecommendationConfig(gomock.Any(), gomock.Any()).
					Times(0)
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

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/regions/%d/recommendation-config", tc.regionID)
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 辅助函数 ====================
// randomDish already defined in dish_test.go

// randomOperatorForRecommendation creates a random operator for recommendation tests
func randomOperatorForRecommendation(userID int64, regionID int64) db.Operator {
	return db.Operator{
		ID:             util.RandomInt(1, 1000),
		UserID:         userID,
		Name:           util.RandomString(10),
		ContactName:    util.RandomString(6),
		ContactPhone:   util.RandomString(11),
		RegionID:       regionID,
		CommissionRate: pgtype.Numeric{},
		Status:         "active",
		CreatedAt:      time.Now(),
	}
}

// testNumericFromFloat 创建测试用的pgtype.Numeric（正确设置Valid标志）
func testNumericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	// Scan from string representation of the number
	err := n.Scan(fmt.Sprintf("%f", f))
	if err != nil {
		panic(err)
	}
	return n
}

// ==================== 探索包间测试 ====================

func TestExploreRoomsAPI(t *testing.T) {
	user, _ := randomUser(t)

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
				store.EXPECT().
					ExploreNearbyRooms(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ExploreNearbyRoomsRow{
						{
							ID:                  1,
							MerchantID:          100,
							TableNo:             "R01",
							TableType:           "room",
							Capacity:            8,
							Status:              "available",
							MerchantName:        "测试餐厅",
							MerchantAddress:     "测试地址",
							MonthlyReservations: 15,
						},
					}, nil)

				store.EXPECT().
					CountExploreNearbyRooms(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_WithFilters",
			query: "?page_id=1&page_size=10&region_id=1&min_capacity=4&max_capacity=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ExploreNearbyRooms(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ExploreNearbyRoomsRow{}, nil)

				store.EXPECT().
					CountExploreNearbyRooms(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "BadRequest_MissingPageID",
			query: "?page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_MissingPageSize",
			query: "?page_id=1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidCapacity",
			query: "?page_id=1&page_size=10&min_capacity=0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_CapacityTooLarge",
			query: "?page_id=1&page_size=10&max_capacity=101",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "Unauthorized",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No auth
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "InternalError",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ExploreNearbyRooms(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ExploreNearbyRoomsRow{}, pgx.ErrTxClosed)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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

			url := "/v1/recommendations/rooms" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
