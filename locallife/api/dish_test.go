package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 测试数据生成 ====================

func randomDishCategory() db.DishCategory {
	return db.DishCategory{
		ID:        util.RandomInt(1, 1000),
		Name:      util.RandomString(6),
		CreatedAt: time.Now(),
	}
}

func randomDish(merchantID int64, categoryID *int64) db.Dish {
	var catID pgtype.Int8
	if categoryID != nil {
		catID = pgtype.Int8{Int64: *categoryID, Valid: true}
	}

	return db.Dish{
		ID:                util.RandomInt(1, 1000),
		MerchantID:        merchantID,
		CategoryID:        catID,
		Name:              util.RandomString(8),
		Description:       pgtype.Text{String: util.RandomString(20), Valid: true},
		ImageMediaAssetID: pgtype.Int8{},
		Price:             util.RandomInt(1000, 10000),
		MemberPrice:       pgtype.Int8{Int64: util.RandomInt(800, 9000), Valid: true},
		IsAvailable:       true,
		IsOnline:          true,
		SortOrder:         int16(util.RandomInt(0, 100)),
		CreatedAt:         time.Now(),
		UpdatedAt:         pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

// ==================== 菜品分类测试 ====================

func TestCreateDishCategoryAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	category := randomDishCategory()
	sortOrder := int16(util.RandomInt(0, 100))

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
				"name":       category.Name,
				"sort_order": sortOrder,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					CreateDishCategory(gomock.Any(), gomock.Eq(category.Name)).
					Times(1).
					Return(category, nil)

				store.EXPECT().
					LinkMerchantDishCategory(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantDishCategory{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				requireBodyMatchDishCategory(t, recorder.Body, category)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"name":       category.Name,
				"sort_order": sortOrder,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectNoMerchantAccessResolution(store)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "NotMerchant",
			body: gin.H{
				"name":       category.Name,
				"sort_order": sortOrder,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "InvalidRequest",
			body: gin.H{
				"name": "", // Empty name
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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

			url := "/v1/dishes/categories"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListDishCategoriesAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	n := 5
	categories := make([]db.ListDishCategoriesRow, n)
	for i := 0; i < n; i++ {
		cat := randomDishCategory()
		categories[i] = db.ListDishCategoriesRow{
			ID:        cat.ID,
			Name:      cat.Name,
			SortOrder: int16(util.RandomInt(0, 100)),
		}
	}

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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListDishCategories(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(categories, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var gotResponse listDishCategoriesResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &gotResponse)
				require.Len(t, gotResponse.Categories, len(categories))
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectNoMerchantAccessResolution(store)
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

			url := "/v1/dishes/categories"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 菜品管理测试 ====================

func TestCreateDishAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	category := randomDishCategory()
	dish := randomDish(merchant.ID, &category.ID)

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
				"category_id":    category.ID,
				"name":           dish.Name,
				"description":    dish.Description.String,
				"price":          dish.Price,
				"member_price":   dish.MemberPrice.Int64,
				"is_available":   dish.IsAvailable,
				"is_online":      dish.IsOnline,
				"sort_order":     dish.SortOrder,
				"ingredient_ids": []int64{},
				"tag_ids":        []int64{},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMerchantDishCategory(gomock.Any(), gomock.Eq(db.GetMerchantDishCategoryParams{
						MerchantID: merchant.ID,
						CategoryID: category.ID,
					})).
					Times(1).
					Return(db.MerchantDishCategory{}, nil)

				store.EXPECT().
					CreateDishTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateDishTxResult{
						Dish:        dish,
						Ingredients: []db.DishIngredient{},
						Tags:        []db.DishTag{},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "OKWithCustomizations",
			body: gin.H{
				"category_id":  category.ID,
				"name":         dish.Name,
				"price":        dish.Price,
				"is_available": dish.IsAvailable,
				"is_online":    dish.IsOnline,
				"customization_groups": []gin.H{
					{
						"name":        "辣度",
						"is_required": true,
						"sort_order":  1,
						"options": []gin.H{
							{
								"tag_id":      int64(101),
								"extra_price": int64(200),
								"sort_order":  1,
							},
						},
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMerchantDishCategory(gomock.Any(), gomock.Eq(db.GetMerchantDishCategoryParams{
						MerchantID: merchant.ID,
						CategoryID: category.ID,
					})).
					Times(1).
					Return(db.MerchantDishCategory{}, nil)

				store.EXPECT().
					GetTag(gomock.Any(), gomock.Eq(int64(101))).
					Times(1).
					Return(db.Tag{ID: 101, Name: "微辣"}, nil)

				store.EXPECT().
					CreateDishTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateDishTxResult{
						Dish: dish,
						CustomizationGroups: []db.DishCustomizationGroupWithOptions{
							{
								Group: db.DishCustomizationGroup{
									ID:         1,
									DishID:     dish.ID,
									Name:       "辣度",
									IsRequired: true,
									SortOrder:  1,
								},
								Options: []db.DishCustomizationOption{
									{
										ID:         11,
										GroupID:    1,
										TagID:      101,
										ExtraPrice: 200,
										SortOrder:  1,
									},
								},
							},
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var resp dishResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.CustomizationGroups, 1)
				require.Equal(t, "辣度", resp.CustomizationGroups[0].Name)
				require.Len(t, resp.CustomizationGroups[0].Options, 1)
				require.Equal(t, int64(101), resp.CustomizationGroups[0].Options[0].TagID)
				require.Equal(t, "微辣", resp.CustomizationGroups[0].Options[0].TagName)
				require.Equal(t, int64(200), resp.CustomizationGroups[0].Options[0].ExtraPrice)
			},
		},
		{
			name: "InvalidPrice",
			body: gin.H{
				"name":  dish.Name,
				"price": -100, // Invalid price
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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

			url := "/v1/dishes"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetDishAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	dish := randomDish(merchant.ID, nil)

	// Mock complete dish response with JSON fields
	completeDish := db.GetDishCompleteRow{
		ID:                  dish.ID,
		MerchantID:          dish.MerchantID,
		CategoryID:          dish.CategoryID,
		Name:                dish.Name,
		Description:         dish.Description,
		ImageMediaAssetID:   dish.ImageMediaAssetID,
		Price:               dish.Price,
		MemberPrice:         dish.MemberPrice,
		IsAvailable:         dish.IsAvailable,
		IsOnline:            dish.IsOnline,
		SortOrder:           dish.SortOrder,
		CreatedAt:           dish.CreatedAt,
		UpdatedAt:           dish.UpdatedAt,
		CategoryName:        pgtype.Text{String: "热菜", Valid: true},
		Ingredients:         []byte(`[]`),
		Tags:                []byte(`[]`),
		CustomizationGroups: []byte(`[]`),
	}

	testCases := []struct {
		name          string
		dishID        int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "OK",
			dishID: dish.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				completeDish.ImageMediaAssetID = pgtype.Int8{Int64: 321, Valid: true}
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), gomock.Eq(int64(321))).
					Times(1).
					Return(approvedAsset(321, "merchant/dish/1/detail.jpg"), nil)
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetDishComplete(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(completeDish, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp struct {
					Data dishResponse `json:"data"`
				}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.NotNil(t, resp.Data.ImageAssetID)
				require.Equal(t, int64(321), *resp.Data.ImageAssetID)
			},
		},
		{
			name:   "NotFound",
			dishID: dish.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetDishComplete(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(db.GetDishCompleteRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:   "Forbidden",
			dishID: dish.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// Return dish belonging to different merchant
				wrongDish := completeDish
				wrongDish.MerchantID = merchant.ID + 999
				store.EXPECT().
					GetDishComplete(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(wrongDish, nil)
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

			server, _ := newTestServerForMedia(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/dishes/%d", tc.dishID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListDishesByMerchantAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	n := 5
	dishes := make([]db.Dish, n)
	for i := 0; i < n; i++ {
		dishes[i] = randomDish(merchant.ID, nil)
	}

	type Query struct {
		PageID   int
		PageSize int
	}

	testCases := []struct {
		name          string
		query         Query
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			query: Query{
				PageID:   1,
				PageSize: n,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListDishesByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(dishes, nil)

				store.EXPECT().
					CountDishesByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(n), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "InvalidPageID",
			query: Query{
				PageID:   -1,
				PageSize: n,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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

			url := fmt.Sprintf("/v1/dishes?page_id=%d&page_size=%d", tc.query.PageID, tc.query.PageSize)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateDishAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	dish := randomDish(merchant.ID, nil)

	testCases := []struct {
		name          string
		dishID        int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "OK",
			dishID: dish.ID,
			body: gin.H{
				"name":           "新菜名",
				"price":          5000,
				"image_asset_id": int64(888),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				dish.ImageMediaAssetID = pgtype.Int8{Int64: 888, Valid: true}
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), gomock.Eq(int64(888))).
					Times(1).
					Return(approvedAsset(888, "merchant/dish/1/update.jpg"), nil)
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetDish(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					UpdateDishTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UpdateDishTxResult{
						Dish: dish,
						Tags: []db.DishTag{},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp struct {
					Data dishResponse `json:"data"`
				}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.NotNil(t, resp.Data.ImageAssetID)
				require.Equal(t, int64(888), *resp.Data.ImageAssetID)
			},
		},
		{
			name:   "NotFound",
			dishID: dish.ID,
			body: gin.H{
				"name": "新菜名",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetDish(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(db.Dish{}, db.ErrRecordNotFound)
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

			server, _ := newTestServerForMedia(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/dishes/%d", tc.dishID)
			request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDeleteDishAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	dish := randomDish(merchant.ID, nil)

	testCases := []struct {
		name          string
		dishID        int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "OK",
			dishID: dish.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetDish(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					DeleteDish(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(nil)

				store.EXPECT().
					RemoveDishFromAllCombos(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:   "NotFound",
			dishID: dish.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetDish(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(db.Dish{}, db.ErrRecordNotFound)
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

			url := fmt.Sprintf("/v1/dishes/%d", tc.dishID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 辅助函数 ====================

func requireBodyMatchDishCategory(t *testing.T, body *bytes.Buffer, category db.DishCategory) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var gotCategory dishCategoryResponse
	requireUnmarshalAPIResponseData(t, data, &gotCategory)
	require.Equal(t, category.ID, gotCategory.ID)
	require.Equal(t, category.Name, gotCategory.Name)
}
