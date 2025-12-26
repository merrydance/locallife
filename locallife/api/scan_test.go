package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 测试数据生成 ====================

func randomDishForMenu(categoryID int64) db.ListDishesForMenuRow {
	return db.ListDishesForMenuRow{
		ID:          util.RandomInt(1, 1000),
		CategoryID:  pgtype.Int8{Int64: categoryID, Valid: true},
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(50), Valid: true},
		ImageUrl:    pgtype.Text{String: "https://example.com/dish.jpg", Valid: true},
		Price:       util.RandomInt(1000, 10000),
		MemberPrice: pgtype.Int8{Int64: util.RandomInt(800, 9000), Valid: true},
		IsAvailable: true,
		SortOrder:   int16(util.RandomInt(1, 100)),
	}
}

func randomComboForMenu(merchantID int64) db.ListOnlineCombosByMerchantRow {
	return db.ListOnlineCombosByMerchantRow{
		ID:            util.RandomInt(1, 1000),
		MerchantID:    merchantID,
		Name:          util.RandomString(10),
		Description:   pgtype.Text{String: util.RandomString(50), Valid: true},
		ImageUrl:      pgtype.Text{String: "https://example.com/combo.jpg", Valid: true},
		OriginalPrice: util.RandomInt(5000, 10000),
		Price:         util.RandomInt(3000, 5000),
		IsOnline:      true,
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
	table := randomTable(merchant.ID)
	category := randomDishCategory()
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
					Return([]db.DishCategory{category}, nil)

				store.EXPECT().
					ListDishesForMenu(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return([]db.ListDishesForMenuRow{dish}, nil)

				store.EXPECT().
					ListOnlineCombosByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
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
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response scanTableResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
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
					Return(db.Merchant{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
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
					Return(db.Table{}, pgx.ErrNoRows)
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
					Return([]db.DishCategory{category}, nil)

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
					Return([]db.DishCategory{category}, nil)

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
					Return([]db.DishCategory{}, nil)

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
					Return([]db.DishCategory{category}, nil)

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

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== generateTableQRCode 测试 ====================

func TestGenerateTableQRCodeAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	testCases := []struct {
		name          string
		tableID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					UpdateTable(gomock.Any(), gomock.Any()).
					Times(1).
					Return(table, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response generateTableQRCodeResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.NotEmpty(t, response.QrCodeUrl)
				require.Contains(t, response.QrCodeUrl, fmt.Sprintf("merchant_id=%d", merchant.ID))
				require.Contains(t, response.QrCodeUrl, fmt.Sprintf("table_no=%s", table.TableNo))
				require.Equal(t, table.TableNo, response.TableNo)
				require.Equal(t, merchant.ID, response.MerchantID)
			},
		},
		{
			name:    "NoAuthorization",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				// No calls expected
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
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(int64(9999))).
					Times(1).
					Return(db.Table{}, pgx.ErrNoRows)
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
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, pgx.ErrNoRows)
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
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				// Return a different merchant
				otherMerchant := merchant
				otherMerchant.ID = merchant.ID + 1
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(otherMerchant, nil)
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
			buildStubs: func(store *mockdb.MockStore) {
				// No calls expected
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
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Table{}, errors.New("internal error"))
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
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

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
			tc.buildStubs(store)

			server := newTestServer(t, store)

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
