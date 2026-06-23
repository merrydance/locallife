package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 测试数据生成 ====================

func randomDishForMenu(categoryID int64) db.ListDishesForMenuRow {
	return db.ListDishesForMenuRow{
		ID:                util.RandomInt(1, 1000),
		CategoryID:        pgtype.Int8{Int64: categoryID, Valid: true},
		Name:              util.RandomString(10),
		Description:       pgtype.Text{String: util.RandomString(50), Valid: true},
		ImageMediaAssetID: pgtype.Int8{},
		Price:             util.RandomInt(1000, 10000),
		MemberPrice:       pgtype.Int8{Int64: util.RandomInt(800, 9000), Valid: true},
		IsAvailable:       true,
		SortOrder:         int16(util.RandomInt(1, 100)),
	}
}

func randomComboForMenu(merchantID int64) db.ListOnlineCombosByMerchantRow {
	return db.ListOnlineCombosByMerchantRow{
		ID:                util.RandomInt(1, 1000),
		MerchantID:        merchantID,
		Name:              util.RandomString(10),
		Description:       pgtype.Text{String: util.RandomString(50), Valid: true},
		ImageMediaAssetID: pgtype.Int8{},
		OriginalPrice:     util.RandomInt(5000, 10000),
		Price:             util.RandomInt(3000, 5000),
		IsOnline:          true,
	}
}

func randomDiscountRule(merchantID int64) db.DiscountRule {
	return db.DiscountRule{
		ID:             util.RandomInt(1, 1000),
		MerchantID:     merchantID,
		Name:           "满减规则",
		MinOrderAmount: 10000,
		DiscountAmount: 1000,
		IsActive:       true,
		ValidFrom:      time.Now().Add(-time.Hour * 24),
		ValidUntil:     time.Now().Add(time.Hour * 24 * 30),
		CreatedAt:      time.Now(),
	}
}

// ==================== scanTable 测试 ====================

func TestScanTableAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.IsOpen = true
	table := randomTable(merchant.ID)
	category := randomDishCategory()
	listCategory := db.ListDishCategoriesRow{
		ID:        category.ID,
		Name:      category.Name,
		CreatedAt: category.CreatedAt,
		DeletedAt: category.DeletedAt,
		SortOrder: 1,
	}
	category2 := randomDishCategory()
	listCategory2 := db.ListDishCategoriesRow{
		ID:        category2.ID,
		Name:      category2.Name,
		CreatedAt: category2.CreatedAt,
		DeletedAt: category2.DeletedAt,
		SortOrder: 2,
	}
	dish := randomDishForMenu(category.ID)
	combo := randomComboForMenu(merchant.ID)
	deliveryPromo := randomDeliveryPromotion(merchant.ID)
	discountRule := randomDiscountRule(merchant.ID)

	testCases := []struct {
		name          string
		merchantID    int64
		tableNo       string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Eq(db.GetTableByMerchantAndNoParams{
						MerchantID: merchant.ID,
						TableNo:    table.TableNo,
					})).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					ListDishCategories(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return([]db.ListDishCategoriesRow{listCategory}, nil)

				store.EXPECT().
					ListDishesForMenu(gomock.Any(), gomock.Eq(db.ListDishesForMenuParams{
						MerchantID:       merchant.ID,
						ExcludePackaging: false,
					})).
					Times(1).
					Return([]db.ListDishesForMenuRow{dish}, nil)

				store.EXPECT().
					ListOnlineCombosByMerchant(gomock.Any(), gomock.Eq(db.ListOnlineCombosByMerchantParams{
						MerchantID:       merchant.ID,
						ExcludePackaging: false,
					})).
					Times(1).
					Return([]db.ListOnlineCombosByMerchantRow{combo}, nil)

				store.EXPECT().
					ListActiveDeliveryPromotionsByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return([]db.MerchantDeliveryPromotion{deliveryPromo}, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return([]db.DiscountRule{discountRule}, nil)

				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(db.GetDishWithCustomizationsRow{ID: dish.ID}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response scanTableResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, merchant.ID, response.Merchant.ID)
				require.Equal(t, merchant.Name, response.Merchant.Name)
				require.Equal(t, table.ID, response.Table.ID)
				require.Equal(t, table.TableNo, response.Table.TableNo)
				require.Len(t, response.Categories, 1)
				require.Len(t, response.Combos, 1)
				require.Len(t, response.Promotions, 2) // delivery_return + discount
			},
		},
		{
			name:       "MerchantNotFound",
			merchantID: 9999,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(int64(9999))).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:       "InvalidDishTags",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Eq(db.GetTableByMerchantAndNoParams{
						MerchantID: merchant.ID,
						TableNo:    table.TableNo,
					})).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					ListDishCategories(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return([]db.ListDishCategoriesRow{listCategory}, nil)

				brokenDish := dish
				brokenDish.Tags = []byte(`not-json`)
				store.EXPECT().
					ListDishesForMenu(gomock.Any(), gomock.Eq(db.ListDishesForMenuParams{
						MerchantID:       merchant.ID,
						ExcludePackaging: false,
					})).
					Times(1).
					Return([]db.ListDishesForMenuRow{brokenDish}, nil)

				store.EXPECT().
					ListOnlineCombosByMerchant(gomock.Any(), gomock.Eq(db.ListOnlineCombosByMerchantParams{
						MerchantID:       merchant.ID,
						ExcludePackaging: false,
					})).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, "internal server error", resp.Message)
			},
		},
		{
			name:       "MultiCategories_DishInFirstCategory",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Eq(db.GetTableByMerchantAndNoParams{
						MerchantID: merchant.ID,
						TableNo:    table.TableNo,
					})).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					ListDishCategories(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return([]db.ListDishCategoriesRow{listCategory, listCategory2}, nil)

				store.EXPECT().
					ListDishesForMenu(gomock.Any(), gomock.Eq(db.ListDishesForMenuParams{
						MerchantID:       merchant.ID,
						ExcludePackaging: false,
					})).
					Times(1).
					Return([]db.ListDishesForMenuRow{dish}, nil)

				store.EXPECT().
					ListOnlineCombosByMerchant(gomock.Any(), gomock.Eq(db.ListOnlineCombosByMerchantParams{
						MerchantID:       merchant.ID,
						ExcludePackaging: false,
					})).
					Times(1).
					Return([]db.ListOnlineCombosByMerchantRow{}, nil)

				store.EXPECT().
					ListActiveDeliveryPromotionsByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return([]db.MerchantDeliveryPromotion{}, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(db.GetDishWithCustomizationsRow{ID: dish.ID}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response scanTableResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.Categories, 2)
				require.Len(t, response.Categories[0].Dishes, 1)
				require.Equal(t, dish.ID, response.Categories[0].Dishes[0].ID)
			},
		},
		{
			name:       "MerchantNotApproved",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				unapprovedMerchant := merchant
				unapprovedMerchant.Status = "pending"
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(unapprovedMerchant, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			},
		},
		{
			name:       "TableNotFound",
			merchantID: merchant.ID,
			tableNo:    "T999",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Eq(db.GetTableByMerchantAndNoParams{
						MerchantID: merchant.ID,
						TableNo:    "T999",
					})).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:       "TableDisabled",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				disabledTable := table
				disabledTable.Status = "disabled"
				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(disabledTable, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			},
		},
		{
			name:       "MissingMerchantID",
			merchantID: 0,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				// No calls expected due to validation failure
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "MissingTableNo",
			merchantID: merchant.ID,
			tableNo:    "",
			buildStubs: func(store *mockdb.MockStore) {
				// No calls expected due to validation failure
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "TableNoTooLong",
			merchantID: merchant.ID,
			tableNo:    util.RandomString(51), // max=50
			buildStubs: func(store *mockdb.MockStore) {
				// No calls expected due to validation failure
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "InvalidMerchantID_Negative",
			merchantID: -1,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				// No calls expected due to validation failure
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "GetMerchantInternalError",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Merchant{}, errors.New("internal error"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:       "GetTableInternalError",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Table{}, errors.New("internal error"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:       "ListCategoriesInternalError",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					ListDishCategories(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("internal error"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:       "ListDishesInternalError",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					ListDishCategories(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListDishCategoriesRow{listCategory}, nil)

				store.EXPECT().
					ListDishesForMenu(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("internal error"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:       "ListCombosInternalError",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					ListDishCategories(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListDishCategoriesRow{listCategory}, nil)

				store.EXPECT().
					ListDishesForMenu(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListDishesForMenuRow{dish}, nil)

				store.EXPECT().
					ListOnlineCombosByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("internal error"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:       "EmptyMenu",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					ListDishCategories(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListDishCategoriesRow{}, nil)

				store.EXPECT().
					ListDishesForMenu(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListDishesForMenuRow{}, nil)

				store.EXPECT().
					ListOnlineCombosByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListOnlineCombosByMerchantRow{}, nil)

				store.EXPECT().
					ListActiveDeliveryPromotionsByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.MerchantDeliveryPromotion{}, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.DiscountRule{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response scanTableResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Empty(t, response.Categories)
				require.Empty(t, response.Combos)
				require.Empty(t, response.Promotions)
			},
		},
		{
			name:       "PromotionsQueryError_Ignored",
			merchantID: merchant.ID,
			tableNo:    table.TableNo,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					ListDishCategories(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListDishCategoriesRow{listCategory}, nil)

				store.EXPECT().
					ListDishesForMenu(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListDishesForMenuRow{dish}, nil)

				store.EXPECT().
					ListOnlineCombosByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListOnlineCombosByMerchantRow{combo}, nil)

				// Promotion queries fail but are ignored
				store.EXPECT().
					ListActiveDeliveryPromotionsByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("db error"))

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("db error"))

				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), gomock.Eq(dish.ID)).
					Times(1).
					Return(db.GetDishWithCustomizationsRow{ID: dish.ID}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// Promotions errors are silently ignored, should still return OK
				require.Equal(t, http.StatusOK, recorder.Code)
				var response scanTableResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Empty(t, response.Promotions)
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

			url := fmt.Sprintf("/v1/scan/table?merchant_id=%d&table_no=%s", tc.merchantID, tc.tableNo)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestScanTableAPIExcludesLegacyPackagingWhenFreezeEnabled(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.IsOpen = true
	table := randomTable(merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetTableByMerchantAndNo(gomock.Any(), gomock.Eq(db.GetTableByMerchantAndNoParams{
			MerchantID: merchant.ID,
			TableNo:    table.TableNo,
		})).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		ListDishCategories(gomock.Any(), gomock.Eq(merchant.ID)).
		Times(1).
		Return([]db.ListDishCategoriesRow{}, nil)
	store.EXPECT().
		ListDishesForMenu(gomock.Any(), gomock.Eq(db.ListDishesForMenuParams{
			MerchantID:       merchant.ID,
			ExcludePackaging: true,
		})).
		Times(1).
		Return([]db.ListDishesForMenuRow{}, nil)
	store.EXPECT().
		ListOnlineCombosByMerchant(gomock.Any(), gomock.Eq(db.ListOnlineCombosByMerchantParams{
			MerchantID:       merchant.ID,
			ExcludePackaging: true,
		})).
		Times(1).
		Return([]db.ListOnlineCombosByMerchantRow{}, nil)
	store.EXPECT().
		ListActiveDeliveryPromotionsByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
		Times(1).
		Return([]db.MerchantDeliveryPromotion{}, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), gomock.Eq(merchant.ID)).
		Times(1).
		Return([]db.DiscountRule{}, nil)

	server := newTestServer(t, store)
	server.config.PackagingLegacyDishFreezeEnabled = true
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/scan/table?merchant_id=%d&table_no=%s", merchant.ID, table.TableNo), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
}

// ==================== generateTableQRCode 测试 ====================

func buildTestQRCodePNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.NRGBA{R: 0, G: 0, B: 0, A: 255}

	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, white)
		}
	}
	for y := 8; y < 56; y++ {
		for x := 8; x < 56; x++ {
			if (x+y)%3 == 0 {
				img.Set(x, y, black)
			}
		}
	}

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestStoreTableQRCodeReusesExistingMediaAsset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	qrCodeData := buildTestQRCodePNG(t)

	var storedObjectKey string
	store.EXPECT().
		CreateMediaAsset(gomock.Any(), gomock.AssignableToTypeOf(db.CreateMediaAssetParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateMediaAssetParams) (db.MediaAsset, error) {
			require.NotEmpty(t, arg.ObjectKey)
			require.Equal(t, "server", arg.SourceClient)
			storedObjectKey = arg.ObjectKey
			return db.MediaAsset{}, db.ErrUniqueViolation
		}).
		Times(1)

	store.EXPECT().
		GetMediaAssetByObjectKey(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, objectKey string) (db.MediaAsset, error) {
			require.Equal(t, storedObjectKey, objectKey)
			return db.MediaAsset{
				ID:               util.RandomInt(1, 1000),
				ObjectKey:        objectKey,
				ModerationStatus: "approved",
			}, nil
		}).
		Times(1)

	server, _ := newTestServerForMedia(t, store)

	qrCodeURL, err := server.storeTableQRCode(context.Background(), user.ID, merchant.ID, table.ID, qrCodeData)

	require.NoError(t, err)
	require.Contains(t, qrCodeURL, storedObjectKey)
}

func TestGenerateTableQRCodeAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	table.QrCodeUrl = pgtype.Text{}
	qrCodeData := buildTestQRCodePNG(t)

	testCases := []struct {
		name          string
		tableID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					Times(1).
					Return(qrCodeData, nil)

				store.EXPECT().
					CreateMediaAsset(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "pending"}, nil)

				store.EXPECT().
					SetMediaAssetModerationStatus(gomock.Any(), gomock.AssignableToTypeOf(db.SetMediaAssetModerationStatusParams{})).
					DoAndReturn(func(_ context.Context, arg db.SetMediaAssetModerationStatusParams) (db.MediaAsset, error) {
						require.Equal(t, "approved", arg.ModerationStatus)
						return db.MediaAsset{ID: arg.ID, ModerationStatus: arg.ModerationStatus}, nil
					}).
					Times(1)

				store.EXPECT().
					UpdateTable(gomock.Any(), gomock.Any()).
					Times(1).
					Return(table, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response generateTableQRCodeResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.NotEmpty(t, response.QrCodeUrl)
				require.Contains(t, response.QrCodeUrl, fmt.Sprintf("merchant/table/%d/qrcodes/", merchant.ID))
				require.Contains(t, response.QrCodeUrl, fmt.Sprintf("qrcode_m%d_t%d_", merchant.ID, table.ID))
				require.Contains(t, response.QrCodeUrl, currentTableQRCodeFilenameSuffix)
				require.Equal(t, table.TableNo, response.TableNo)
				require.Equal(t, merchant.ID, response.MerchantID)
			},
		},
		{
			name:    "ReuseCurrentVersionQRCode",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				tableWithQRCode := table
				tableWithQRCode.QrCodeUrl = pgtype.Text{String: fmt.Sprintf("https://cdn.example.com/qrcode%s", currentTableQRCodeFilenameSuffix), Valid: true}

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(tableWithQRCode, nil)

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response generateTableQRCodeResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Contains(t, response.QrCodeUrl, currentTableQRCodeFilenameSuffix)
			},
		},
		{
			name:    "HyphenTableNoUsesTableIDScene",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				hyphenTable := table
				hyphenTable.TableNo = "A-01"

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(hyphenTable.ID)).
					Times(1).
					Return(hyphenTable, nil)

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *wechat.WXACodeRequest) ([]byte, error) {
						require.Equal(t, fmt.Sprintf("tid_%d", hyphenTable.ID), req.Scene)
						return qrCodeData, nil
					}).
					Times(1)

				store.EXPECT().
					CreateMediaAsset(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "approved"}, nil)

				store.EXPECT().
					UpdateTable(gomock.Any(), gomock.Any()).
					Times(1).
					Return(hyphenTable, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response generateTableQRCodeResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "A-01", response.TableNo)
			},
		},
		{
			name:    "RegenerateOldVersionQRCode",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				tableWithLegacyQRCode := table
				tableWithLegacyQRCode.QrCodeUrl = pgtype.Text{String: "https://cdn.example.com/qrcode_labeled.png", Valid: true}

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(tableWithLegacyQRCode, nil)

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					Times(1).
					Return(qrCodeData, nil)

				store.EXPECT().
					CreateMediaAsset(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "approved"}, nil)

				store.EXPECT().
					UpdateTable(gomock.Any(), gomock.Any()).
					Times(1).
					Return(tableWithLegacyQRCode, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response generateTableQRCodeResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Contains(t, response.QrCodeUrl, currentTableQRCodeFilenameSuffix)
			},
		},
		{
			name:    "RetryReusesExistingQRCodeAsset",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					Times(1).
					Return(qrCodeData, nil)

				var storedObjectKey string
				store.EXPECT().
					CreateMediaAsset(gomock.Any(), gomock.AssignableToTypeOf(db.CreateMediaAssetParams{})).
					DoAndReturn(func(_ context.Context, arg db.CreateMediaAssetParams) (db.MediaAsset, error) {
						storedObjectKey = arg.ObjectKey
						return db.MediaAsset{}, db.ErrUniqueViolation
					}).
					Times(1)

				store.EXPECT().
					GetMediaAssetByObjectKey(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, objectKey string) (db.MediaAsset, error) {
						require.Equal(t, storedObjectKey, objectKey)
						return db.MediaAsset{
							ID:               util.RandomInt(1, 1000),
							ObjectKey:        objectKey,
							ModerationStatus: "approved",
						}, nil
					}).
					Times(1)

				store.EXPECT().
					UpdateTable(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.UpdateTableParams) (db.Table, error) {
						require.Equal(t, table.ID, arg.ID)
						require.True(t, arg.QrCodeUrl.Valid)
						require.Contains(t, arg.QrCodeUrl.String, storedObjectKey)
						updatedTable := table
						updatedTable.QrCodeUrl = arg.QrCodeUrl
						return updatedTable, nil
					}).
					Times(1)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response generateTableQRCodeResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Contains(t, response.QrCodeUrl, currentTableQRCodeFilenameSuffix)
				require.Equal(t, table.TableNo, response.TableNo)
			},
		},
		{
			name:    "NoAuthorization",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				// No calls expected
				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:    "TableNotFound",
			tableID: 9999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(int64(9999))).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "NotAMerchant",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				expectResolveNoAccessibleMerchants(store, user.ID)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "TableNotBelongToMerchant",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				// Return a different merchant
				otherMerchant := merchant
				otherMerchant.ID = merchant.ID + 1
				expectResolveSingleOwnedMerchant(store, user.ID, otherMerchant)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "InvalidTableID_NonNumeric",
			tableID: 0, // Will use "invalid" in URL
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "GetTableInternalError",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Table{}, errors.New("internal error"))

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:    "UpdateTableInternalError",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.Any()).
					Times(1).
					Return(qrCodeData, nil)

				store.EXPECT().
					CreateMediaAsset(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "pending"}, nil)

				store.EXPECT().
					SetMediaAssetModerationStatus(gomock.Any(), gomock.AssignableToTypeOf(db.SetMediaAssetModerationStatusParams{})).
					DoAndReturn(func(_ context.Context, arg db.SetMediaAssetModerationStatusParams) (db.MediaAsset, error) {
						require.Equal(t, "approved", arg.ModerationStatus)
						return db.MediaAsset{ID: arg.ID, ModerationStatus: arg.ModerationStatus}, nil
					}).
					Times(1)

				store.EXPECT().
					UpdateTable(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Table{}, errors.New("internal error"))
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
			wechatClient := mockwechat.NewMockWechatClient(ctrl)
			tc.buildStubs(store, wechatClient)

			server, _ := newTestServerForMediaWithWechat(t, store, wechatClient)

			var url string
			if tc.name == "InvalidTableID_NonNumeric" {
				url = "/v1/tables/invalid/qrcode"
			} else {
				url = fmt.Sprintf("/v1/tables/%d/qrcode", tc.tableID)
			}

			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 辅助函数测试 ====================

func TestFormatDeliveryReturnDesc(t *testing.T) {
	testCases := []struct {
		minAmount    int64
		returnAmount int64
		expected     string
	}{
		{5000, 500, "满50元返5元运费"},
		{10000, 1000, "满100元返10元运费"},
		{0, 0, "满0元返0元运费"},
		{2999, 299, "满30元返3元运费"}, // 整数舍入
	}

	for _, tc := range testCases {
		result := formatDeliveryReturnDesc(tc.minAmount, tc.returnAmount)
		require.Equal(t, tc.expected, result)
	}
}

func TestFormatDiscountDesc(t *testing.T) {
	testCases := []struct {
		minAmount     int64
		discountValue int64
		expected      string
	}{
		{5000, 500, "满50元减5元"},
		{10000, 2000, "满100元减20元"},
		{0, 0, "满0元减0元"},
	}

	for _, tc := range testCases {
		result := formatDiscountDesc(tc.minAmount, tc.discountValue)
		require.Equal(t, tc.expected, result)
	}
}
