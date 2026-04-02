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

// ==================== 套餐创建测试 ====================

func TestCreateComboSetAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	combo := randomComboSet(merchant.ID)

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
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				var response comboSetResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, combo.Name, response.Name)
				require.Equal(t, combo.ComboPrice, response.ComboPrice)
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
	dishesJSON := []byte(`[{"id":1,"name":"菜品1"},{"id":2,"name":"菜品2"}]`)
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
					GetComboMemberImagesByCombos(gomock.Any(), gomock.Eq([]int64{combo.ID})).
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListComboSetsByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(combos, nil)

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

// ==================== 套餐更新测试 ====================

func TestUpdateComboSetAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	combo := randomComboSet(merchant.ID)

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
					UpdateComboSetTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UpdateComboSetTxResult{ComboSet: updatedCombo}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response comboSetResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, newName, response.Name)
				require.Equal(t, newPrice, response.ComboPrice)
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

// ==================== 套餐添加菜品测试 ====================

func TestAddComboDishAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	combo := randomComboSet(merchant.ID)
	dishID := util.RandomInt(1, 100)

	// 创建一个属于该商户的菜品
	dish := db.Dish{
		ID:         dishID,
		MerchantID: merchant.ID,
		Name:       util.RandomString(8),
		Price:      util.RandomInt(1000, 10000),
	}

	comboDish := db.ComboDish{
		ID:       util.RandomInt(1, 1000),
		ComboID:  combo.ID,
		DishID:   dishID,
		Quantity: 1,
	}

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
				"dish_id": dishID,
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

				// P1修复：添加菜品所有权验证
				store.EXPECT().
					GetDish(gomock.Any(), gomock.Eq(dishID)).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					AddComboDish(gomock.Any(), gomock.Any()).
					Times(1).
					Return(comboDish, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "DishNotBelongToMerchant",
			comboID: combo.ID,
			body: gin.H{
				"dish_id": dishID,
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

				// 返回一个属于其他商户的菜品
				otherDish := db.Dish{
					ID:         dishID,
					MerchantID: merchant.ID + 1, // 不同的商户
					Name:       util.RandomString(8),
				}
				store.EXPECT().
					GetDish(gomock.Any(), gomock.Eq(dishID)).
					Times(1).
					Return(otherDish, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "NoAuthorization",
			comboID: combo.ID,
			body: gin.H{
				"dish_id": dishID,
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
			name:    "InvalidDishID",
			comboID: combo.ID,
			body: gin.H{
				"dish_id": 0,
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

			url := fmt.Sprintf("/v1/combos/%d/dishes", tc.comboID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 套餐上下线测试 ====================

func TestToggleComboOnlineAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	combo := randomComboSet(merchant.ID)
	combo.IsOnline = false

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
					UpdateComboSetOnlineStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
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

// ==================== 套餐移除菜品测试 ====================

func TestRemoveComboDishAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	combo := randomComboSet(merchant.ID)
	dishID := util.RandomInt(1, 100)

	testCases := []struct {
		name          string
		comboID       int64
		dishID        int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			comboID: combo.ID,
			dishID:  dishID,
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
					RemoveComboDish(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "NoAuthorization",
			comboID: combo.ID,
			dishID:  dishID,
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
			name:    "InvalidDishID",
			comboID: combo.ID,
			dishID:  0,
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

			url := fmt.Sprintf("/v1/combos/%d/dishes/%d", tc.comboID, tc.dishID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
