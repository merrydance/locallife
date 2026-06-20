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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 测试数据生成 ====================

func randomComboSet(merchantID int64) db.ComboSet {
	return db.ComboSet{
		ID:                util.RandomInt(1, 1000),
		MerchantID:        merchantID,
		Name:              util.RandomString(8),
		Description:       pgtype.Text{String: util.RandomString(20), Valid: true},
		ImageMediaAssetID: pgtype.Int8{},
		OriginalPrice:     util.RandomInt(5000, 15000),
		ComboPrice:        util.RandomInt(4000, 12000),
		IsOnline:          true,
		CreatedAt:         time.Now(),
		UpdatedAt:         pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

func comboDetailsRow(combo db.ComboSet, dishesJSON, tagsJSON []byte) db.GetComboSetWithDetailsRow {
	return db.GetComboSetWithDetailsRow{
		ID:                combo.ID,
		MerchantID:        combo.MerchantID,
		Name:              combo.Name,
		Description:       combo.Description,
		ImageMediaAssetID: combo.ImageMediaAssetID,
		OriginalPrice:     combo.OriginalPrice,
		ComboPrice:        combo.ComboPrice,
		IsOnline:          combo.IsOnline,
		CreatedAt:         combo.CreatedAt,
		UpdatedAt:         combo.UpdatedAt,
		Dishes:            dishesJSON,
		Tags:              tagsJSON,
	}
}

func comboDishOrderabilityRow(dishID int64, name string, dishExists bool, isOnline bool, isAvailable bool, isPackaging ...bool) db.ListComboDishOrderabilityRow {
	packaging := false
	if len(isPackaging) > 0 {
		packaging = isPackaging[0]
	}
	return db.ListComboDishOrderabilityRow{
		DishID:      dishID,
		DishName:    name,
		DishExists:  pgtype.Bool{Bool: dishExists, Valid: true},
		IsOnline:    isOnline,
		IsAvailable: isAvailable,
		IsPackaging: packaging,
	}
}

func expectComboSummaryReload(store *mockdb.MockStore, combo db.ComboSet, dishesJSON, tagsJSON []byte) {
	store.EXPECT().
		GetComboSetWithDetails(gomock.Any(), gomock.Eq(combo.ID)).
		Times(1).
		Return(comboDetailsRow(combo, dishesJSON, tagsJSON), nil)

	store.EXPECT().
		GetComboMemberImagesByCombos(gomock.Any(), gomock.Eq(db.GetComboMemberImagesByCombosParams{
			Column1:          []int64{combo.ID},
			ExcludePackaging: false,
		})).
		Times(1).
		Return(nil, nil)
}

func comboCustomizationGroupsFixture() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":          int64(501),
			"name":        "杯型",
			"sort_order":  int32(1),
			"is_required": true,
			"options": []map[string]interface{}{
				{
					"id":          int64(601),
					"tag_id":      int64(701),
					"tag_name":    "大杯",
					"extra_price": int64(300),
					"sort_order":  int32(1),
				},
			},
		},
		{
			"id":          int64(502),
			"name":        "冰量",
			"sort_order":  int32(2),
			"is_required": true,
			"options": []map[string]interface{}{
				{
					"id":          int64(602),
					"tag_id":      int64(702),
					"tag_name":    "少冰",
					"extra_price": int64(0),
					"sort_order":  int32(1),
				},
			},
		},
	}
}

// ==================== 套餐创建测试 ====================

func TestCreateComboSetAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	combo := randomComboSet(merchant.ID)
	responseDishesJSON := []byte(`[{"dish_id":101,"dish_name":"菜品1","dish_price":1200,"quantity":2}]`)
	responseTagsJSON := []byte(`[{"id":11,"name":"招牌"}]`)
	normalizedDishID := int64(101)
	normalizedDish := db.Dish{ID: normalizedDishID, MerchantID: merchant.ID, Name: "菜品1", Price: 1200, IsAvailable: true, IsOnline: true}
	customizationGroups := comboCustomizationGroupsFixture()

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
				"name":        combo.Name,
				"description": combo.Description.String,
				"combo_price": combo.ComboPrice,
				"is_active":   true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					CreateComboSetTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateComboSetTxResult{ComboSet: combo}, nil)

				expectComboSummaryReload(store, combo, responseDishesJSON, responseTagsJSON)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				var response comboSetResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, combo.Name, response.Name)
				require.Equal(t, combo.ComboPrice, response.ComboPrice)
				require.Equal(t, int64(1), response.DishCount)
				require.Equal(t, int64(2), response.DishTotalQuantity)
				require.Len(t, response.Tags, 1)
			},
		},
		{
			name: "NormalizeFixedCustomizations",
			body: gin.H{
				"name":        combo.Name,
				"combo_price": combo.ComboPrice,
				"dishes": []gin.H{{
					"dish_id":  normalizedDishID,
					"quantity": 2,
					"customizations": gin.H{
						"501":        601,
						"502":        602,
						"meta_specs": "客户端伪造摘要",
					},
				}},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), gomock.Eq(normalizedDishID)).
					Times(1).
					Return(db.GetDishWithCustomizationsRow{
						ID:                  normalizedDish.ID,
						MerchantID:          merchant.ID,
						Price:               normalizedDish.Price,
						CustomizationGroups: customizationGroups,
					}, nil)

				store.EXPECT().
					GetDish(gomock.Any(), gomock.Eq(normalizedDishID)).
					Times(1).
					Return(normalizedDish, nil)

				store.EXPECT().
					CreateComboSetTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CreateComboSetTxParams) (db.CreateComboSetTxResult, error) {
						require.Len(t, arg.Dishes, 1)
						require.Equal(t, normalizedDish.Price, arg.Dishes[0].DishBasePriceSnapshot)
						require.Equal(t, int64(300), arg.Dishes[0].CustomizationExtraPrice)
						require.JSONEq(t, `{"501":601,"502":602,"meta_specs":"大杯 / 少冰"}`, string(arg.Dishes[0].Customizations))
						return db.CreateComboSetTxResult{ComboSet: combo}, nil
					})

				expectComboSummaryReload(store, combo, responseDishesJSON, responseTagsJSON)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "RejectMetaOnlyCustomizations",
			body: gin.H{
				"name":        combo.Name,
				"combo_price": combo.ComboPrice,
				"dishes": []gin.H{{
					"dish_id":  normalizedDishID,
					"quantity": 1,
					"customizations": gin.H{
						"meta_specs": "伪造摘要",
					},
				}},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), gomock.Eq(normalizedDishID)).
					Times(1).
					Return(db.GetDishWithCustomizationsRow{
						ID:                  normalizedDish.ID,
						MerchantID:          merchant.ID,
						Price:               normalizedDish.Price,
						CustomizationGroups: customizationGroups,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "RejectOnlineComboWithUnavailableDish",
			body: gin.H{
				"name":        combo.Name,
				"combo_price": combo.ComboPrice,
				"is_online":   true,
				"dishes": []gin.H{{
					"dish_id":  normalizedDishID,
					"quantity": 1,
				}},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				unavailableDish := normalizedDish
				unavailableDish.IsAvailable = false
				store.EXPECT().
					GetDish(gomock.Any(), gomock.Eq(normalizedDishID)).
					Times(1).
					Return(unavailableDish, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "DuplicateDishes",
			body: gin.H{
				"name":        combo.Name,
				"combo_price": combo.ComboPrice,
				"dishes": []gin.H{
					{"dish_id": 88, "quantity": 1},
					{"dish_id": 88, "quantity": 2},
				},
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
		{
			name: "NoAuthorization",
			body: gin.H{
				"name":        combo.Name,
				"combo_price": combo.ComboPrice,
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
			body: gin.H{
				"name":        combo.Name,
				"combo_price": combo.ComboPrice,
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
			name: "InvalidPrice",
			body: gin.H{
				"name":        combo.Name,
				"combo_price": -100, // Invalid price
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

			url := "/v1/combos"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 套餐详情测试 ====================

func TestGetComboSetAPI(t *testing.T) {
	user, _ := randomUser(t)
	combo := randomComboSet(util.RandomInt(1, 100))

	// 模拟GetComboSetWithDetails返回的数据
	dishesJSON := []byte(`[
		{"dish_id":1,"dish_name":"菜品1","dish_price":1800,"quantity":1,"customizations":{"12":34,"meta_specs":"大杯 / 少冰"},"customization_extra_price":300,"customization_summary":"大杯 / 少冰"},
		{"dish_id":2,"dish_name":"菜品2","dish_price":2200,"quantity":2}
	]`)
	tagsJSON := []byte(`[{"id":1,"name":"标签1"}]`)

	// 创建一个商户用于测试
	merchant := randomMerchant(user.ID)
	// 将套餐设置为属于该商户
	combo.MerchantID = merchant.ID

	testCases := []struct {
		name          string
		comboID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			comboID: combo.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				const comboDishAssetID int64 = 31
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				detailsRow := db.GetComboSetWithDetailsRow{
					ID:                combo.ID,
					MerchantID:        combo.MerchantID,
					Name:              combo.Name,
					Description:       combo.Description,
					ImageMediaAssetID: combo.ImageMediaAssetID,
					OriginalPrice:     combo.OriginalPrice,
					ComboPrice:        combo.ComboPrice,
					IsOnline:          combo.IsOnline,
					CreatedAt:         combo.CreatedAt,
					UpdatedAt:         combo.UpdatedAt,
					Dishes:            dishesJSON,
					Tags:              tagsJSON,
				}

				store.EXPECT().
					GetComboSetWithDetails(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(detailsRow, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), combo.MerchantID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetComboMemberImagesByCombos(gomock.Any(), gomock.Eq(db.GetComboMemberImagesByCombosParams{
						Column1:          []int64{combo.ID},
						ExcludePackaging: false,
					})).
					Times(1).
					Return([]db.GetComboMemberImagesByCombosRow{{
						ComboID:           combo.ID,
						ImageMediaAssetID: pgtype.Int8{Int64: comboDishAssetID, Valid: true},
					}}, nil)

				store.EXPECT().
					ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{comboDishAssetID})).
					Times(1).
					Return([]db.ListMediaAssetsByIDsRow{{
						ID:               comboDishAssetID,
						ObjectKey:        "merchant/combo/31/member.jpg",
						Visibility:       "public",
						ModerationStatus: "approved",
					}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response comboSetWithDetailsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, combo.ID, response.ID)
				require.Equal(t, combo.Name, response.Name)
				require.Len(t, response.Dishes, 2)
				require.Equal(t, "大杯 / 少冰", response.Dishes[0].CustomizationSummary)
				require.Equal(t, int64(300), response.Dishes[0].CustomizationExtraPrice)
				require.Equal(t, float64(34), response.Dishes[0].Customizations["12"])
				require.Len(t, response.Tags, 1)
				require.Len(t, response.DishImageURLs, 1)
				require.Contains(t, response.DishImageURLs[0], "merchant/combo/31/member.jpg")
			},
		},
		{
			name:    "NoAuthorization",
			comboID: combo.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectNoMerchantAccessResolution(store)
				store.EXPECT().
					GetComboSetWithDetails(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:    "NotFound",
			comboID: combo.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSetWithDetails(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(db.GetComboSetWithDetailsRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "NotMerchant",
			comboID: combo.ID,
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
			name:    "ComboNotBelongToMerchant",
			comboID: combo.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 返回一个属于其他商户的套餐
				detailsRow := db.GetComboSetWithDetailsRow{
					ID:         combo.ID,
					MerchantID: merchant.ID + 1, // 不同的商户ID
					Name:       combo.Name,
				}

				store.EXPECT().
					GetComboSetWithDetails(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(detailsRow, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "InvalidID",
			comboID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
				store.EXPECT().
					GetComboSetWithDetails(gomock.Any(), gomock.Any()).
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

			server, _ := newTestServerForMedia(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/combos/%d", tc.comboID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 套餐列表测试 ====================

func TestListComboSetsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	n := 5
	combos := make([]db.ComboSet, n)
	for i := 0; i < n; i++ {
		combos[i] = randomComboSet(merchant.ID)
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=5",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				const comboImageAssetID int64 = 41
				listRows := make([]db.ListComboSetsByMerchantRow, 0, len(combos))
				for index, combo := range combos {
					row := db.ListComboSetsByMerchantRow{
						ID:                combo.ID,
						Name:              combo.Name,
						Description:       combo.Description,
						OriginalPrice:     combo.OriginalPrice,
						ComboPrice:        combo.ComboPrice,
						IsOnline:          combo.IsOnline,
						DishCount:         2,
						DishTotalQuantity: 3,
					}
					if index == 0 {
						row.Tags = []byte(`[{"id":11,"name":"招牌"}]`)
					}
					listRows = append(listRows, row)
				}

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListComboSetsByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(listRows, nil)

				store.EXPECT().
					CountComboSetsByMerchant(gomock.Any(), gomock.Eq(db.CountComboSetsByMerchantParams{
						MerchantID: merchant.ID,
						IsOnline:   pgtype.Bool{},
					})).
					Times(1).
					Return(int64(n), nil)

				store.EXPECT().
					GetComboMemberImagesByCombos(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetComboMemberImagesByCombosRow{{
						ComboID:           combos[0].ID,
						ImageMediaAssetID: pgtype.Int8{Int64: comboImageAssetID, Valid: true},
					}}, nil)

				store.EXPECT().
					ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{comboImageAssetID})).
					Times(1).
					Return([]db.ListMediaAssetsByIDsRow{{
						ID:               comboImageAssetID,
						ObjectKey:        "merchant/combo/41/list.jpg",
						Visibility:       "public",
						ModerationStatus: "approved",
					}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response listComboSetsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.ComboSets, n)
				require.Equal(t, combos[0].OriginalPrice, response.ComboSets[0].OriginalPrice)
				require.Equal(t, int64(2), response.ComboSets[0].DishCount)
				require.Equal(t, int64(3), response.ComboSets[0].DishTotalQuantity)
				require.Len(t, response.ComboSets[0].Tags, 1)
				require.Len(t, response.ComboSets[0].DishImageURLs, 1)
				require.Contains(t, response.ComboSets[0].DishImageURLs[0], "merchant/combo/41/list.jpg")
			},
		},
		{
			name:  "NoAuthorization",
			query: "?page_id=1&page_size=5",
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
			name:  "InvalidPageID",
			query: "?page_id=0&page_size=5",
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

			server, _ := newTestServerForMedia(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/combos" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetPublicComboDetailAPI_ApprovedMerchantKeepsOpenState(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.IsOpen = true
	combo := randomComboSet(merchant.ID)
	combo.IsOnline = true

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	detailsRow := db.GetComboSetWithDetailsRow{
		ID:                combo.ID,
		MerchantID:        combo.MerchantID,
		Name:              combo.Name,
		Description:       combo.Description,
		ImageMediaAssetID: combo.ImageMediaAssetID,
		OriginalPrice:     combo.OriginalPrice,
		ComboPrice:        combo.ComboPrice,
		IsOnline:          combo.IsOnline,
		CreatedAt:         combo.CreatedAt,
		UpdatedAt:         combo.UpdatedAt,
	}

	store.EXPECT().
		GetComboSetWithDetails(gomock.Any(), gomock.Eq(combo.ID)).
		Times(1).
		Return(detailsRow, nil)

	store.EXPECT().
		ListComboDishOrderability(gomock.Any(), gomock.Eq(combo.ID)).
		Times(1).
		Return([]db.ListComboDishOrderabilityRow{
			comboDishOrderabilityRow(201, "可售菜品", true, true, true),
		}, nil)

	store.EXPECT().
		GetMerchant(gomock.Any(), combo.MerchantID).
		Times(1).
		Return(merchant, nil)

	store.EXPECT().
		GetComboMemberImagesByCombos(gomock.Any(), gomock.Eq(db.GetComboMemberImagesByCombosParams{
			Column1:          []int64{combo.ID},
			ExcludePackaging: false,
		})).
		Times(1).
		Return(nil, nil)

	server, _ := newTestServerForMedia(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/public/combos/%d", combo.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response comboSetWithDetailsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.True(t, response.IsOpen)
}

func TestGetPublicComboDetailAPIExcludesLegacyPackagingImagesWhenFreezeEnabled(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.IsOpen = true
	combo := randomComboSet(merchant.ID)
	combo.IsOnline = true

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetComboSetWithDetails(gomock.Any(), gomock.Eq(combo.ID)).
		Times(1).
		Return(comboDetailsRow(combo, nil, nil), nil)
	store.EXPECT().
		ListComboDishOrderability(gomock.Any(), gomock.Eq(combo.ID)).
		Times(1).
		Return([]db.ListComboDishOrderabilityRow{
			comboDishOrderabilityRow(201, "可售菜品", true, true, true),
		}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), combo.MerchantID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetComboMemberImagesByCombos(gomock.Any(), gomock.Eq(db.GetComboMemberImagesByCombosParams{
			Column1:          []int64{combo.ID},
			ExcludePackaging: true,
		})).
		Times(1).
		Return(nil, nil)

	server, _ := newTestServerForMedia(t, store)
	server.config.PackagingLegacyDishFreezeEnabled = true
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/public/combos/%d", combo.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestGetPublicComboDetailAPI_RejectsUnavailableChildDish(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	combo := randomComboSet(merchant.ID)
	combo.IsOnline = true

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetComboSetWithDetails(gomock.Any(), gomock.Eq(combo.ID)).
		Times(1).
		Return(comboDetailsRow(combo, nil, nil), nil)
	store.EXPECT().
		ListComboDishOrderability(gomock.Any(), gomock.Eq(combo.ID)).
		Times(1).
		Return([]db.ListComboDishOrderabilityRow{
			comboDishOrderabilityRow(201, "暂不可售菜品", true, true, false),
		}, nil)

	server, _ := newTestServerForMedia(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/public/combos/%d", combo.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "combo set not found")
}

func TestGetPublicComboDetailAPIRejectsLegacyPackagingChildWhenFreezeEnabled(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	combo := randomComboSet(merchant.ID)
	combo.IsOnline = true

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetComboSetWithDetails(gomock.Any(), gomock.Eq(combo.ID)).
		Times(1).
		Return(comboDetailsRow(combo, nil, nil), nil)
	store.EXPECT().
		ListComboDishOrderability(gomock.Any(), gomock.Eq(combo.ID)).
		Times(1).
		Return([]db.ListComboDishOrderabilityRow{
			comboDishOrderabilityRow(201, "旧餐盒", true, true, true, true),
		}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), gomock.Any()).
		Times(0)
	store.EXPECT().
		GetComboMemberImagesByCombos(gomock.Any(), gomock.Any()).
		Times(0)

	server, _ := newTestServerForMedia(t, store)
	server.config.PackagingLegacyDishFreezeEnabled = true
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/public/combos/%d", combo.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "combo set not found")
}

// ==================== 套餐更新测试 ====================

func TestUpdateComboSetAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	combo := randomComboSet(merchant.ID)
	responseDishesJSON := []byte(`[{"dish_id":201,"dish_name":"菜品A","dish_price":1800,"quantity":1},{"dish_id":202,"dish_name":"菜品B","dish_price":2200,"quantity":3}]`)
	responseTagsJSON := []byte(`[{"id":21,"name":"午市推荐"}]`)
	normalizedDishID := int64(201)
	normalizedDish := db.Dish{ID: normalizedDishID, MerchantID: merchant.ID, Name: "菜品A", Price: 1500, IsAvailable: true, IsOnline: true}
	customizationGroups := comboCustomizationGroupsFixture()

	newName := "Updated Combo"
	newPrice := int64(8888)

	updatedCombo := combo
	updatedCombo.Name = newName
	updatedCombo.ComboPrice = newPrice

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
				"id":          combo.ID,
				"name":        newName,
				"combo_price": newPrice,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(combo, nil)

				store.EXPECT().
					ListComboDishOrderability(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return([]db.ListComboDishOrderabilityRow{
						comboDishOrderabilityRow(normalizedDishID, normalizedDish.Name, true, true, true),
					}, nil)

				store.EXPECT().
					UpdateComboSetTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UpdateComboSetTxResult{ComboSet: updatedCombo}, nil)

				expectComboSummaryReload(store, updatedCombo, responseDishesJSON, responseTagsJSON)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response comboSetResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, newName, response.Name)
				require.Equal(t, newPrice, response.ComboPrice)
				require.Equal(t, int64(2), response.DishCount)
				require.Equal(t, int64(4), response.DishTotalQuantity)
				require.Len(t, response.Tags, 1)
			},
		},
		{
			name: "NormalizeFixedCustomizations",
			body: gin.H{
				"id": combo.ID,
				"dishes": []gin.H{{
					"dish_id":  normalizedDishID,
					"quantity": 2,
					"customizations": gin.H{
						"501":        601,
						"502":        602,
						"meta_specs": "伪造摘要",
					},
				}},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(combo, nil)

				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), gomock.Eq(normalizedDishID)).
					Times(1).
					Return(db.GetDishWithCustomizationsRow{
						ID:                  normalizedDish.ID,
						MerchantID:          merchant.ID,
						Price:               normalizedDish.Price,
						CustomizationGroups: customizationGroups,
					}, nil)

				store.EXPECT().
					GetDish(gomock.Any(), gomock.Eq(normalizedDishID)).
					Times(1).
					Return(normalizedDish, nil)

				store.EXPECT().
					UpdateComboSetTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateComboSetTxParams) (db.UpdateComboSetTxResult, error) {
						require.NotNil(t, arg.Dishes)
						require.Len(t, *arg.Dishes, 1)
						require.Equal(t, normalizedDish.Price, (*arg.Dishes)[0].DishBasePriceSnapshot)
						require.Equal(t, int64(300), (*arg.Dishes)[0].CustomizationExtraPrice)
						require.JSONEq(t, `{"501":601,"502":602,"meta_specs":"大杯 / 少冰"}`, string((*arg.Dishes)[0].Customizations))
						return db.UpdateComboSetTxResult{ComboSet: updatedCombo}, nil
					})

				expectComboSummaryReload(store, updatedCombo, responseDishesJSON, responseTagsJSON)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "RejectMetaOnlyCustomizations",
			body: gin.H{
				"id": combo.ID,
				"dishes": []gin.H{{
					"dish_id": normalizedDishID,
					"customizations": gin.H{
						"meta_specs": "伪造摘要",
					},
				}},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(combo, nil)

				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), gomock.Eq(normalizedDishID)).
					Times(1).
					Return(db.GetDishWithCustomizationsRow{
						ID:                  normalizedDish.ID,
						MerchantID:          merchant.ID,
						Price:               normalizedDish.Price,
						CustomizationGroups: customizationGroups,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "RejectOnlineComboWithOfflineDish",
			body: gin.H{
				"id":        combo.ID,
				"is_online": true,
				"dishes": []gin.H{{
					"dish_id":  normalizedDishID,
					"quantity": 1,
				}},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(combo, nil)

				offlineDish := normalizedDish
				offlineDish.IsOnline = false
				store.EXPECT().
					GetDish(gomock.Any(), gomock.Eq(normalizedDishID)).
					Times(1).
					Return(offlineDish, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "RejectOnlineComboWithExistingUnavailableDish",
			body: gin.H{
				"id":        combo.ID,
				"is_online": true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(combo, nil)

				store.EXPECT().
					ListComboDishOrderability(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return([]db.ListComboDishOrderabilityRow{
						comboDishOrderabilityRow(normalizedDishID, normalizedDish.Name, true, true, false),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ListExistingDishesError",
			body: gin.H{
				"id":        combo.ID,
				"is_online": true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(combo, nil)

				store.EXPECT().
					ListComboDishOrderability(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(nil, errors.New("list combo dishes failed"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "DuplicateDishes",
			body: gin.H{
				"id": combo.ID,
				"dishes": []gin.H{
					{"dish_id": 99, "quantity": 1},
					{"dish_id": 99, "quantity": 2},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(combo, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"id":   combo.ID,
				"name": newName,
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
			name: "NotFound",
			body: gin.H{
				"id":   combo.ID,
				"name": newName,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(db.ComboSet{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "UnauthorizedMerchant",
			body: gin.H{
				"id":   combo.ID,
				"name": newName,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 套餐属于不同的商户
				otherCombo := combo
				otherCombo.MerchantID = merchant.ID + 999
				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(otherCombo, nil)
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

			url := fmt.Sprintf("/v1/combos/%d", combo.ID)
			request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 套餐删除测试 ====================

func TestDeleteComboSetAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	combo := randomComboSet(merchant.ID)

	testCases := []struct {
		name          string
		comboID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			comboID: combo.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(combo, nil)

				store.EXPECT().
					DeleteComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response MessageResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "combo set deleted", response.Message)
			},
		},
		{
			name:    "NoAuthorization",
			comboID: combo.ID,
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
			name:    "NotFound",
			comboID: combo.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(db.ComboSet{}, db.ErrRecordNotFound)
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

			url := fmt.Sprintf("/v1/combos/%d", tc.comboID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 旧套餐-菜品直连路由退役测试 ====================

func TestDirectComboDishRoutesRetiredAPI(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	testCases := []struct {
		name   string
		method string
		url    string
		body   *bytes.Reader
	}{
		{
			name:   "AddDishRouteRetired",
			method: http.MethodPost,
			url:    "/v1/combos/123/dishes",
			body:   bytes.NewReader([]byte(`{"dish_id":456}`)),
		},
		{
			name:   "RemoveDishRouteRetired",
			method: http.MethodDelete,
			url:    "/v1/combos/123/dishes/456",
			body:   bytes.NewReader(nil),
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(tc.method, tc.url, tc.body)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

// ==================== 套餐上下线测试 ====================

func TestToggleComboOnlineAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	combo := randomComboSet(merchant.ID)
	combo.IsOnline = false
	updatedCombo := combo
	updatedCombo.IsOnline = true
	responseDishesJSON := []byte(`[{"dish_id":301,"dish_name":"热销菜","dish_price":3200,"quantity":2}]`)
	responseTagsJSON := []byte(`[{"id":31,"name":"热卖"}]`)

	testCases := []struct {
		name          string
		comboID       int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			comboID: combo.ID,
			body: gin.H{
				"is_online": true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(combo, nil)

				store.EXPECT().
					ListComboDishOrderability(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return([]db.ListComboDishOrderabilityRow{
						comboDishOrderabilityRow(301, "热销菜", true, true, true),
					}, nil)

				store.EXPECT().
					UpdateComboSetOnlineStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				expectComboSummaryReload(store, updatedCombo, responseDishesJSON, responseTagsJSON)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response comboSetResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.True(t, response.IsOnline)
				require.Equal(t, int64(1), response.DishCount)
				require.Equal(t, int64(2), response.DishTotalQuantity)
				require.Len(t, response.Tags, 1)
			},
		},
		{
			name:    "NoAuthorization",
			comboID: combo.ID,
			body: gin.H{
				"is_online": true,
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
			name:    "NotFound",
			comboID: combo.ID,
			body: gin.H{
				"is_online": true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(db.ComboSet{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "RejectOnlineWithUnavailableChildDish",
			comboID: combo.ID,
			body: gin.H{
				"is_online": true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(combo, nil)

				store.EXPECT().
					ListComboDishOrderability(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return([]db.ListComboDishOrderabilityRow{
						comboDishOrderabilityRow(301, "热销菜", true, true, false),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "ListExistingDishesError",
			comboID: combo.ID,
			body: gin.H{
				"is_online": true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetComboSet(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(combo, nil)

				store.EXPECT().
					ListComboDishOrderability(gomock.Any(), gomock.Eq(combo.ID)).
					Times(1).
					Return(nil, errors.New("list combo dishes failed"))
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/combos/%d/online", tc.comboID)
			request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
