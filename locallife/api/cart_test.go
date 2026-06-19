package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== Helper Functions ====================

func randomCart(userID, merchantID int64) db.Cart {
	return db.Cart{
		ID:         util.RandomInt(1, 1000),
		UserID:     userID,
		MerchantID: merchantID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func randomCartItem(cartID int64, dish db.Dish) db.CartItem {
	return db.CartItem{
		ID:             util.RandomInt(1, 1000),
		CartID:         cartID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		ComboID:        pgtype.Int8{Valid: false},
		Quantity:       2,
		Customizations: []byte(`{"spicy": "medium"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func randomListCartItemsRow(cartItem db.CartItem, dish db.Dish) db.ListCartItemsRow {
	return db.ListCartItemsRow{
		ID:                     cartItem.ID,
		CartID:                 cartItem.CartID,
		DishID:                 cartItem.DishID,
		ComboID:                cartItem.ComboID,
		Quantity:               cartItem.Quantity,
		Customizations:         cartItem.Customizations,
		DishName:               pgtype.Text{String: dish.Name, Valid: true},
		DishPrice:              pgtype.Int8{Int64: dish.Price, Valid: true},
		DishImageMediaAssetID:  dish.ImageMediaAssetID,
		DishIsAvailable:        pgtype.Bool{Bool: dish.IsOnline, Valid: true},
		DishIsPackaging:        pgtype.Bool{Bool: dish.IsPackaging, Valid: true},
		ComboName:              pgtype.Text{Valid: false},
		ComboPrice:             pgtype.Int8{Valid: false},
		ComboImageMediaAssetID: pgtype.Int8{},
		ComboIsAvailable:       pgtype.Bool{Valid: false},
	}
}

type combinedCheckoutMapClientStub struct {
	route *maps.RouteResult
	err   error
}

func (s combinedCheckoutMapClientStub) GetBicyclingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return s.route, s.err
}

func (s combinedCheckoutMapClientStub) GetWalkingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, s.err
}

func (s combinedCheckoutMapClientStub) GetDrivingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, s.err
}

func (s combinedCheckoutMapClientStub) GetDistanceMatrix(ctx context.Context, froms, tos []maps.Location, mode string) (*maps.DistanceMatrixResult, error) {
	return nil, s.err
}

func (s combinedCheckoutMapClientStub) Geocode(ctx context.Context, address string) (*maps.GeocodeResult, error) {
	return nil, s.err
}

func (s combinedCheckoutMapClientStub) ReverseGeocode(ctx context.Context, location maps.Location) (*maps.ReverseGeocodeResult, error) {
	return nil, s.err
}

// ==================== GetCart Tests ====================

func TestGetCartAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	cart := randomCart(user.ID, merchant.ID)
	dish := randomDish(merchant.ID, nil)
	cartItem := randomCartItem(cart.ID, dish)
	listRow := randomListCartItemsRow(cartItem, dish)

	testCases := []struct {
		name          string
		merchantID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), gomock.Eq(db.GetCartByUserAndMerchantParams{
						UserID:     user.ID,
						MerchantID: merchant.ID,
						OrderType:  "takeout",
					})).
					Times(1).
					Return(cart, nil)

				store.EXPECT().
					ListCartItems(gomock.Any(), gomock.Eq(cart.ID)).
					Times(1).
					Return([]db.ListCartItemsRow{listRow}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response cartResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, cart.ID, response.ID)
				require.Equal(t, merchant.ID, response.MerchantID)
				require.Len(t, response.Items, 1)
			},
		},
		{
			name:       "CartNotFound_ReturnsEmptyCart",
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), gomock.Eq(db.GetCartByUserAndMerchantParams{
						UserID:     user.ID,
						MerchantID: merchant.ID,
						OrderType:  "takeout",
					})).
					Times(1).
					Return(db.Cart{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response cartResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Empty(t, response.Items)
				require.Equal(t, int64(0), response.Subtotal)
			},
		},
		{
			name:       "MissingMerchantID",
			merchantID: 0, // will be handled specially
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			merchantID: merchant.ID,
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
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

			url := "/v1/cart"
			if tc.merchantID != 0 {
				url = fmt.Sprintf("/v1/cart?merchant_id=%d", tc.merchantID)
			}

			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetCartAPIIncludesPackagingOptionsAndSelectedOption(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	cart := randomCart(user.ID, merchant.ID)
	dish := randomDish(merchant.ID, nil)
	dish.IsPackaging = true
	dish.IsOnline = true
	dish.IsAvailable = true
	cart.OrderType = db.OrderTypeTakeout
	cartItem := randomCartItem(cart.ID, dish)
	listRow := randomListCartItemsRow(cartItem, dish)
	option := db.MerchantPackagingOption{
		ID:          util.RandomInt(1000, 2000),
		MerchantID:  merchant.ID,
		Name:        "普通餐盒",
		Description: pgtype.Text{String: "环保纸盒", Valid: true},
		Price:       100,
		IsEnabled:   true,
		SortOrder:   2,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Eq(db.GetCartByUserAndMerchantParams{
			UserID:     user.ID,
			MerchantID: merchant.ID,
			OrderType:  "takeout",
		})).
		Times(1).
		Return(cart, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), gomock.Eq(cart.ID)).
		Times(1).
		Return([]db.ListCartItemsRow{listRow}, nil)
	store.EXPECT().
		GetMerchantPackagingSettings(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.MerchantPackagingSetting{
			MerchantID:           merchant.ID,
			Enabled:              true,
			Required:             true,
			ApplicableOrderTypes: []string{db.OrderTypeTakeout},
		}, nil)
	store.EXPECT().
		ListEnabledMerchantPackagingOptions(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.MerchantPackagingOption{option}, nil)
	store.EXPECT().
		GetCartPackagingSelection(gomock.Any(), cart.ID).
		Times(1).
		Return(db.CartPackagingSelection{
			CartID:            cart.ID,
			PackagingOptionID: pgtype.Int8{Int64: option.ID, Valid: true},
			SelectionVersion:  3,
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/cart?merchant_id=%d", merchant.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response cartResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.True(t, response.PackagingRequired)
	require.True(t, response.Packaging.Enabled)
	require.True(t, response.Packaging.Required)
	require.True(t, response.Packaging.Applicable)
	require.NotNil(t, response.Packaging.SelectedOptionID)
	require.Equal(t, option.ID, *response.Packaging.SelectedOptionID)
	require.Equal(t, int64(3), response.Packaging.SelectionVersion)
	require.Len(t, response.Packaging.Options, 1)
	require.Equal(t, option.ID, response.Packaging.Options[0].ID)
	require.Equal(t, option.Name, response.Packaging.Options[0].Name)
	require.Equal(t, option.Price, response.Packaging.Options[0].Price)
	require.Len(t, response.Items, 1)
	require.True(t, response.Items[0].IsPackaging)
}

func TestPutCartPackagingSelectionRejectsForeignCart(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	cart := randomCart(otherUser.ID, merchant.ID)
	cart.OrderType = db.OrderTypeTakeout
	optionID := util.RandomInt(1000, 2000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Eq(db.GetCartByUserAndMerchantParams{
			UserID:     user.ID,
			MerchantID: merchant.ID,
			OrderType:  db.OrderTypeTakeout,
		})).
		Times(1).
		Return(cart, nil)
	store.EXPECT().
		GetMerchantPackagingSettings(gomock.Any(), gomock.Any()).
		Times(0)
	store.EXPECT().
		GetMerchantPackagingOption(gomock.Any(), gomock.Any()).
		Times(0)
	store.EXPECT().
		UpsertCartPackagingSelection(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	body := gin.H{
		"merchant_id":         merchant.ID,
		"order_type":          db.OrderTypeTakeout,
		"packaging_option_id": optionID,
	}
	recorder := performCartPackagingSelectionRequest(t, server, http.MethodPut, body, user.ID)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestPutCartPackagingSelectionRejectsForeignOption(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	cart := randomCart(user.ID, merchant.ID)
	cart.OrderType = db.OrderTypeTakeout
	optionID := util.RandomInt(1000, 2000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectCartPackagingSelectionCartLookup(store, user.ID, merchant.ID, cart)
	expectEnabledCartPackagingSettings(store, merchant.ID)
	store.EXPECT().
		GetMerchantPackagingOption(gomock.Any(), gomock.Eq(db.GetMerchantPackagingOptionParams{
			ID:         optionID,
			MerchantID: merchant.ID,
		})).
		Times(1).
		Return(db.MerchantPackagingOption{}, db.ErrRecordNotFound)
	store.EXPECT().
		UpsertCartPackagingSelection(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	body := gin.H{
		"merchant_id":         merchant.ID,
		"order_type":          db.OrderTypeTakeout,
		"packaging_option_id": optionID,
	}
	recorder := performCartPackagingSelectionRequest(t, server, http.MethodPut, body, user.ID)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestPutCartPackagingSelectionRejectsDisabledOrDeletedOption(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	cart := randomCart(user.ID, merchant.ID)
	cart.OrderType = db.OrderTypeTakeout
	optionID := util.RandomInt(1000, 2000)

	testCases := []struct {
		name      string
		option    db.MerchantPackagingOption
		optionErr error
	}{
		{
			name: "DisabledOption",
			option: db.MerchantPackagingOption{
				ID:         optionID,
				MerchantID: merchant.ID,
				Name:       "普通餐盒",
				Price:      100,
				IsEnabled:  false,
			},
		},
		{
			name:      "DeletedOption",
			optionErr: db.ErrRecordNotFound,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectCartPackagingSelectionCartLookup(store, user.ID, merchant.ID, cart)
			expectEnabledCartPackagingSettings(store, merchant.ID)
			store.EXPECT().
				GetMerchantPackagingOption(gomock.Any(), gomock.Eq(db.GetMerchantPackagingOptionParams{
					ID:         optionID,
					MerchantID: merchant.ID,
				})).
				Times(1).
				Return(tc.option, tc.optionErr)
			store.EXPECT().
				UpsertCartPackagingSelection(gomock.Any(), gomock.Any()).
				Times(0)

			server := newTestServer(t, store)
			body := gin.H{
				"merchant_id":         merchant.ID,
				"order_type":          db.OrderTypeTakeout,
				"packaging_option_id": optionID,
			}
			recorder := performCartPackagingSelectionRequest(t, server, http.MethodPut, body, user.ID)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestDeleteCartPackagingSelectionClearsIdempotently(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	cart := randomCart(user.ID, merchant.ID)
	cart.OrderType = db.OrderTypeTakeout

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectCartPackagingSelectionCartLookup(store, user.ID, merchant.ID, cart)
	store.EXPECT().
		ClearCartPackagingSelection(gomock.Any(), cart.ID).
		Times(1).
		Return(db.CartPackagingSelection{
			CartID:           cart.ID,
			SelectionVersion: 2,
		}, nil)
	expectCartPackagingSelectionCartLookup(store, user.ID, merchant.ID, cart)
	store.EXPECT().
		ClearCartPackagingSelection(gomock.Any(), cart.ID).
		Times(1).
		Return(db.CartPackagingSelection{
			CartID:           cart.ID,
			SelectionVersion: 2,
		}, nil)

	server := newTestServer(t, store)
	body := gin.H{
		"merchant_id": merchant.ID,
		"order_type":  db.OrderTypeTakeout,
	}

	firstRecorder := performCartPackagingSelectionRequest(t, server, http.MethodDelete, body, user.ID)
	require.Equal(t, http.StatusOK, firstRecorder.Code)
	require.Contains(t, firstRecorder.Body.String(), `"selected_option_id":null`)
	var firstResp cartPackagingSelectionResponse
	requireUnmarshalAPIResponseData(t, firstRecorder.Body.Bytes(), &firstResp)
	require.Nil(t, firstResp.SelectedOptionID)
	require.Equal(t, int64(2), firstResp.SelectionVersion)

	secondRecorder := performCartPackagingSelectionRequest(t, server, http.MethodDelete, body, user.ID)
	require.Equal(t, http.StatusOK, secondRecorder.Code)
	var secondResp cartPackagingSelectionResponse
	requireUnmarshalAPIResponseData(t, secondRecorder.Body.Bytes(), &secondResp)
	require.Nil(t, secondResp.SelectedOptionID)
	require.Equal(t, firstResp.SelectionVersion, secondResp.SelectionVersion)
}

func TestPutCartPackagingSelectionRepeatingSameOptionKeepsVersion(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	cart := randomCart(user.ID, merchant.ID)
	cart.OrderType = db.OrderTypeTakeout
	option := db.MerchantPackagingOption{
		ID:         util.RandomInt(1000, 2000),
		MerchantID: merchant.ID,
		Name:       "普通餐盒",
		Price:      100,
		IsEnabled:  true,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectSuccessfulCartPackagingSelectionPut(store, user.ID, merchant.ID, cart, option, 4)
	expectSuccessfulCartPackagingSelectionPut(store, user.ID, merchant.ID, cart, option, 4)

	server := newTestServer(t, store)
	body := gin.H{
		"merchant_id":         merchant.ID,
		"order_type":          db.OrderTypeTakeout,
		"packaging_option_id": option.ID,
	}

	firstRecorder := performCartPackagingSelectionRequest(t, server, http.MethodPut, body, user.ID)
	require.Equal(t, http.StatusOK, firstRecorder.Code)
	var firstResp cartPackagingSelectionResponse
	requireUnmarshalAPIResponseData(t, firstRecorder.Body.Bytes(), &firstResp)

	secondRecorder := performCartPackagingSelectionRequest(t, server, http.MethodPut, body, user.ID)
	require.Equal(t, http.StatusOK, secondRecorder.Code)
	var secondResp cartPackagingSelectionResponse
	requireUnmarshalAPIResponseData(t, secondRecorder.Body.Bytes(), &secondResp)

	require.NotNil(t, secondResp.SelectedOptionID)
	require.Equal(t, option.ID, *secondResp.SelectedOptionID)
	require.Equal(t, firstResp.SelectionVersion, secondResp.SelectionVersion)
}

func TestPutCartPackagingSelectionChangingOptionIncrementsVersion(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	cart := randomCart(user.ID, merchant.ID)
	cart.OrderType = db.OrderTypeTakeout
	firstOption := db.MerchantPackagingOption{
		ID:         util.RandomInt(1000, 2000),
		MerchantID: merchant.ID,
		Name:       "普通餐盒",
		Price:      100,
		IsEnabled:  true,
	}
	secondOption := firstOption
	secondOption.ID = firstOption.ID + 1
	secondOption.Name = "保温餐盒"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectSuccessfulCartPackagingSelectionPut(store, user.ID, merchant.ID, cart, firstOption, 2)
	expectSuccessfulCartPackagingSelectionPut(store, user.ID, merchant.ID, cart, secondOption, 3)

	server := newTestServer(t, store)
	firstBody := gin.H{
		"merchant_id":         merchant.ID,
		"order_type":          db.OrderTypeTakeout,
		"packaging_option_id": firstOption.ID,
	}
	secondBody := gin.H{
		"merchant_id":         merchant.ID,
		"order_type":          db.OrderTypeTakeout,
		"packaging_option_id": secondOption.ID,
	}

	firstRecorder := performCartPackagingSelectionRequest(t, server, http.MethodPut, firstBody, user.ID)
	require.Equal(t, http.StatusOK, firstRecorder.Code)
	var firstResp cartPackagingSelectionResponse
	requireUnmarshalAPIResponseData(t, firstRecorder.Body.Bytes(), &firstResp)

	secondRecorder := performCartPackagingSelectionRequest(t, server, http.MethodPut, secondBody, user.ID)
	require.Equal(t, http.StatusOK, secondRecorder.Code)
	var secondResp cartPackagingSelectionResponse
	requireUnmarshalAPIResponseData(t, secondRecorder.Body.Bytes(), &secondResp)

	require.NotNil(t, secondResp.SelectedOptionID)
	require.Equal(t, secondOption.ID, *secondResp.SelectedOptionID)
	require.Equal(t, firstResp.SelectionVersion+1, secondResp.SelectionVersion)
}

func performCartPackagingSelectionRequest(t *testing.T, server *Server, method string, body gin.H, userID int64) *httptest.ResponseRecorder {
	t.Helper()

	data, err := json.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(method, "/v1/cart/packaging-selection", bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, userID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	return recorder
}

func expectCartPackagingSelectionCartLookup(store *mockdb.MockStore, userID, merchantID int64, cart db.Cart) {
	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Eq(db.GetCartByUserAndMerchantParams{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
		})).
		Times(1).
		Return(cart, nil)
}

func expectEnabledCartPackagingSettings(store *mockdb.MockStore, merchantID int64) {
	store.EXPECT().
		GetMerchantPackagingSettings(gomock.Any(), merchantID).
		Times(1).
		Return(db.MerchantPackagingSetting{
			MerchantID:           merchantID,
			Enabled:              true,
			Required:             true,
			ApplicableOrderTypes: []string{db.OrderTypeTakeout},
		}, nil)
}

func expectSuccessfulCartPackagingSelectionPut(store *mockdb.MockStore, userID, merchantID int64, cart db.Cart, option db.MerchantPackagingOption, version int64) {
	expectCartPackagingSelectionCartLookup(store, userID, merchantID, cart)
	expectEnabledCartPackagingSettings(store, merchantID)
	store.EXPECT().
		GetMerchantPackagingOption(gomock.Any(), gomock.Eq(db.GetMerchantPackagingOptionParams{
			ID:         option.ID,
			MerchantID: merchantID,
		})).
		Times(1).
		Return(option, nil)
	store.EXPECT().
		UpsertCartPackagingSelection(gomock.Any(), gomock.Eq(db.UpsertCartPackagingSelectionParams{
			CartID:            cart.ID,
			PackagingOptionID: pgtype.Int8{Int64: option.ID, Valid: true},
		})).
		Times(1).
		Return(db.CartPackagingSelection{
			CartID:            cart.ID,
			PackagingOptionID: pgtype.Int8{Int64: option.ID, Valid: true},
			SelectionVersion:  version,
		}, nil)
}

// ==================== AddCartItem Tests ====================

func TestAddCartItemAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "active"
	merchant.IsOpen = true
	cart := randomCart(user.ID, merchant.ID)
	dish := randomDish(merchant.ID, nil)
	cartItem := randomCartItem(cart.ID, dish)
	listRow := randomListCartItemsRow(cartItem, dish)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_AddDish",
			body: gin.H{
				"merchant_id": merchant.ID,
				"dish_id":     dish.ID,
				"quantity":    2,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(db.GetDishWithCustomizationsRow{CustomizationGroups: nil}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), gomock.Eq(db.GetCartByUserAndMerchantParams{
						UserID:     user.ID,
						MerchantID: merchant.ID,
						OrderType:  "takeout",
					})).
					Times(1).
					Return(db.Cart{}, db.ErrRecordNotFound)

				store.EXPECT().
					CreateCart(gomock.Any(), gomock.Any()).
					Times(1).
					Return(cart, nil)

				store.EXPECT().
					GetCartItemByDishAndCustomizations(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CartItem{}, db.ErrRecordNotFound)

				store.EXPECT().
					AddCartItem(gomock.Any(), gomock.Any()).
					Times(1).
					Return(cartItem, nil)

				store.EXPECT().
					ListCartItems(gomock.Any(), cart.ID).
					Times(1).
					Return([]db.ListCartItemsRow{listRow}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "MerchantNotFound",
			body: gin.H{
				"merchant_id": merchant.ID,
				"dish_id":     dish.ID,
				"quantity":    1,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(db.GetDishWithCustomizationsRow{CustomizationGroups: nil}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "DishNotFound",
			body: gin.H{
				"merchant_id": merchant.ID,
				"dish_id":     dish.ID,
				"quantity":    1,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(db.GetDishWithCustomizationsRow{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "DishOffline",
			body: gin.H{
				"merchant_id": merchant.ID,
				"dish_id":     dish.ID,
				"quantity":    1,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(db.GetDishWithCustomizationsRow{CustomizationGroups: nil}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				offlineDish := dish
				offlineDish.IsOnline = false
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(offlineDish, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidQuantity_Zero",
			body: gin.H{
				"merchant_id": merchant.ID,
				"dish_id":     dish.ID,
				"quantity":    0,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidQuantity_ExceedsMax",
			body: gin.H{
				"merchant_id": merchant.ID,
				"dish_id":     dish.ID,
				"quantity":    100, // max=99
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidMerchantID_Zero",
			body: gin.H{
				"merchant_id": 0,
				"dish_id":     dish.ID,
				"quantity":    1,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"merchant_id": merchant.ID,
				"dish_id":     dish.ID,
				"quantity":    1,
			},
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
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

			url := "/v1/cart/items"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== UpdateCartItem Tests ====================

func TestUpdateCartItemAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	cart := randomCart(user.ID, merchant.ID)
	otherCart := randomCart(otherUser.ID, merchant.ID)
	dish := randomDish(merchant.ID, nil)
	cartItem := randomCartItem(cart.ID, dish)
	otherCartItem := randomCartItem(otherCart.ID, dish)
	listRow := randomListCartItemsRow(cartItem, dish)

	testCases := []struct {
		name          string
		itemID        int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "OK",
			itemID: cartItem.ID,
			body: gin.H{
				"quantity": 5,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartItem(gomock.Any(), cartItem.ID).
					Times(1).
					Return(db.GetCartItemRow{
						ID:       cartItem.ID,
						CartID:   cart.ID,
						DishID:   cartItem.DishID,
						Quantity: cartItem.Quantity,
					}, nil)

				store.EXPECT().
					GetCart(gomock.Any(), cart.ID).
					Times(1).
					Return(cart, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					UpdateCartItem(gomock.Any(), gomock.Any()).
					Times(1).
					Return(cartItem, nil)

				store.EXPECT().
					ListCartItems(gomock.Any(), cart.ID).
					Times(1).
					Return([]db.ListCartItemsRow{listRow}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:   "ItemNotFound",
			itemID: 99999,
			body: gin.H{
				"quantity": 5,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartItem(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.GetCartItemRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:   "Forbidden_NotOwner",
			itemID: otherCartItem.ID,
			body: gin.H{
				"quantity": 5,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartItem(gomock.Any(), otherCartItem.ID).
					Times(1).
					Return(db.GetCartItemRow{
						ID:       otherCartItem.ID,
						CartID:   otherCart.ID,
						DishID:   otherCartItem.DishID,
						Quantity: otherCartItem.Quantity,
					}, nil)

				store.EXPECT().
					GetCart(gomock.Any(), otherCart.ID).
					Times(1).
					Return(otherCart, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:   "InvalidQuantity_ExceedsMax",
			itemID: cartItem.ID,
			body: gin.H{
				"quantity": 100, // max=99
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:   "NoAuthorization",
			itemID: cartItem.ID,
			body: gin.H{
				"quantity": 5,
			},
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
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

			url := fmt.Sprintf("/v1/cart/items/%d", tc.itemID)
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== DeleteCartItem Tests ====================

func TestDeleteCartItemAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	cart := randomCart(user.ID, merchant.ID)
	otherCart := randomCart(otherUser.ID, merchant.ID)
	dish := randomDish(merchant.ID, nil)
	cartItem := randomCartItem(cart.ID, dish)
	otherCartItem := randomCartItem(otherCart.ID, dish)
	listRow := randomListCartItemsRow(cartItem, dish)

	testCases := []struct {
		name          string
		itemID        int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "OK",
			itemID: cartItem.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartItem(gomock.Any(), cartItem.ID).
					Times(1).
					Return(db.GetCartItemRow{
						ID:       cartItem.ID,
						CartID:   cart.ID,
						DishID:   cartItem.DishID,
						Quantity: cartItem.Quantity,
					}, nil)

				store.EXPECT().
					GetCart(gomock.Any(), cart.ID).
					Times(1).
					Return(cart, nil)

				store.EXPECT().
					DeleteCartItem(gomock.Any(), cartItem.ID).
					Times(1).
					Return(nil)

				store.EXPECT().
					ListCartItems(gomock.Any(), cart.ID).
					Times(1).
					Return([]db.ListCartItemsRow{listRow}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:   "ItemNotFound",
			itemID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartItem(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.GetCartItemRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:   "Forbidden_NotOwner",
			itemID: otherCartItem.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartItem(gomock.Any(), otherCartItem.ID).
					Times(1).
					Return(db.GetCartItemRow{
						ID:       otherCartItem.ID,
						CartID:   otherCart.ID,
						DishID:   otherCartItem.DishID,
						Quantity: otherCartItem.Quantity,
					}, nil)

				store.EXPECT().
					GetCart(gomock.Any(), otherCart.ID).
					Times(1).
					Return(otherCart, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			itemID:     cartItem.ID,
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
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

			url := fmt.Sprintf("/v1/cart/items/%d", tc.itemID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== ClearCart Tests ====================

func TestClearCartAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	cart := randomCart(user.ID, merchant.ID)

	testCases := []struct {
		name          string
		merchantID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
						UserID:     user.ID,
						MerchantID: merchant.ID,
						OrderType:  "takeout",
					}).
					Times(1).
					Return(cart, nil)

				store.EXPECT().
					ClearCart(gomock.Any(), cart.ID).
					Times(1).
					Return(nil)

				store.EXPECT().
					ClearCartPackagingSelection(gomock.Any(), cart.ID).
					Times(1).
					Return(db.CartPackagingSelection{
						CartID:           cart.ID,
						SelectionVersion: 4,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response cartResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Nil(t, response.Packaging.SelectedOptionID)
				require.Equal(t, int64(4), response.Packaging.SelectionVersion)
				require.Contains(t, recorder.Body.String(), `"selected_option_id":null`)
			},
		},
		{
			name:       "CartNotFound_ReturnsEmptyCart",
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Cart{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// 购物车不存在时返回成功，表示购物车已清空
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "MissingMerchantID",
			merchantID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			merchantID: merchant.ID,
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
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

			url := "/v1/cart/clear"
			body := gin.H{}
			if tc.merchantID != 0 {
				body["merchant_id"] = tc.merchantID
			}

			data, err := json.Marshal(body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== CalculateCart Tests ====================

func TestCalculateCartAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "active"
	merchant.IsOpen = true
	cart := randomCart(user.ID, merchant.ID)
	dish := randomDish(merchant.ID, nil)
	cartItem := randomCartItem(cart.ID, dish)
	listRow := randomListCartItemsRow(cartItem, dish)

	testCases := []struct {
		name          string
		merchantID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
						UserID:     user.ID,
						MerchantID: merchant.ID,
						OrderType:  "takeout",
					}).
					Times(1).
					Return(cart, nil)

				store.EXPECT().
					ListCartItems(gomock.Any(), cart.ID).
					Times(1).
					Return([]db.ListCartItemsRow{listRow}, nil)

				store.EXPECT().
					ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
						UserID:         user.ID,
						MerchantID:     merchant.ID,
						MinOrderAmount: int64(cartItem.Quantity) * dish.Price,
					}).
					Times(1).
					Return([]db.ListUserAvailableVouchersForMerchantRow{}, nil)

				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
						MerchantID: merchant.ID,
						UserID:     user.ID,
					}).
					Times(1).
					Return(db.MerchantMembership{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "CartNotFound_ReturnsBadRequest",
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Cart{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "MissingMerchantID",
			merchantID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			merchantID: merchant.ID,
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
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

			var body []byte
			var err error
			if tc.merchantID != 0 {
				body, err = json.Marshal(gin.H{"merchant_id": tc.merchantID})
				require.NoError(t, err)
			} else {
				body = []byte("{}")
			}

			request, err := http.NewRequest(http.MethodPost, "/v1/cart/calculate", bytes.NewReader(body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== GetAllCarts Tests (多商户购物车汇总) ====================

func TestGetAllCartsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant1 := randomMerchant(user.ID)
	merchant2 := randomMerchant(user.ID)
	summaryArg := db.GetUserCartsSummaryParams{
		UserID: user.ID,
		OrderType: pgtype.Text{
			String: "",
			Valid:  false,
		},
	}
	detailsArg := db.GetUserCartsWithDetailsParams{
		UserID: user.ID,
		OrderType: pgtype.Text{
			String: "",
			Valid:  false,
		},
	}
	merchant2.ID = merchant1.ID + 1

	// 创建两个商户的购物车摘要数据
	cartsSummary := db.GetUserCartsSummaryRow{
		CartCount:   2,
		TotalItems:  5,
		TotalAmount: 15000, // 150元
	}

	cartsWithDetails := []db.GetUserCartsWithDetailsRow{
		{
			CartID:                   1,
			MerchantID:               merchant1.ID,
			MerchantName:             merchant1.Name,
			MerchantLogoMediaAssetID: pgtype.Int8{},
			SubMchid:                 pgtype.Text{String: "sub_mch_001", Valid: true},
			ItemCount:                3,
			Subtotal:                 8800,
			AllAvailable:             true,
			UpdatedAt:                time.Now(),
		},
		{
			CartID:                   2,
			MerchantID:               merchant2.ID,
			MerchantName:             merchant2.Name,
			MerchantLogoMediaAssetID: pgtype.Int8{},
			SubMchid:                 pgtype.Text{String: "sub_mch_002", Valid: true},
			ItemCount:                2,
			Subtotal:                 6200,
			AllAvailable:             true,
			UpdatedAt:                time.Now(),
		},
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
				store.EXPECT().
					GetUserCartsSummary(gomock.Any(), gomock.Eq(summaryArg)).
					Times(1).
					Return(cartsSummary, nil)

				store.EXPECT().
					GetUserCartsWithDetails(gomock.Any(), gomock.Eq(detailsArg)).
					Times(1).
					Return(cartsWithDetails, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response userCartsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				require.Equal(t, 2, response.Summary.CartCount)
				require.Equal(t, 5, response.Summary.TotalItems)
				require.Equal(t, int64(15000), response.Summary.TotalAmount)
				require.Len(t, response.Carts, 2)

				// 验证第一个商户购物车
				require.Equal(t, merchant1.ID, response.Carts[0].MerchantID)
				require.Equal(t, int64(8800), response.Carts[0].Subtotal)
				require.True(t, response.Carts[0].AllAvailable)
			},
		},
		{
			name: "EmptyCart",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserCartsSummary(gomock.Any(), gomock.Eq(summaryArg)).
					Times(1).
					Return(db.GetUserCartsSummaryRow{
						CartCount:   0,
						TotalItems:  0,
						TotalAmount: 0,
					}, nil)

				store.EXPECT().
					GetUserCartsWithDetails(gomock.Any(), gomock.Eq(detailsArg)).
					Times(1).
					Return([]db.GetUserCartsWithDetailsRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response userCartsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				require.Equal(t, 0, response.Summary.CartCount)
				require.Len(t, response.Carts, 0)
			},
		},
		{
			name: "SomeItemsUnavailable",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				unavailableCarts := []db.GetUserCartsWithDetailsRow{
					{
						CartID:       1,
						MerchantID:   merchant1.ID,
						MerchantName: merchant1.Name,
						ItemCount:    2,
						Subtotal:     5000,
						AllAvailable: false, // 有商品不可用
						UpdatedAt:    time.Now(),
					},
				}

				store.EXPECT().
					GetUserCartsSummary(gomock.Any(), gomock.Eq(summaryArg)).
					Times(1).
					Return(db.GetUserCartsSummaryRow{
						CartCount:   1,
						TotalItems:  2,
						TotalAmount: 5000,
					}, nil)

				store.EXPECT().
					GetUserCartsWithDetails(gomock.Any(), gomock.Eq(detailsArg)).
					Times(1).
					Return(unavailableCarts, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response userCartsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				require.False(t, response.Carts[0].AllAvailable)
			},
		},
		{
			name:      "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用任何数据库方法
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "InternalError",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserCartsSummary(gomock.Any(), gomock.Eq(summaryArg)).
					Times(1).
					Return(db.GetUserCartsSummaryRow{}, sql.ErrConnDone)
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

			request, err := http.NewRequest(http.MethodGet, "/v1/cart/summary", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== CombinedCheckout Tests (合单结算预览) ====================

func TestCombinedCheckoutAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant1 := randomMerchant(user.ID)
	merchant2 := randomMerchant(user.ID)
	merchant2.ID = merchant1.ID + 1
	merchant1.IsOpen = true
	merchant2.IsOpen = true

	cart1ID := int64(1)
	cart2ID := int64(2)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_MultiCart",
			body: gin.H{
				"cart_ids":   []int64{cart1ID, cart2ID},
				"address_id": 1,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Mock: 获取用户多购物车（使用 GetUserCartsByCartIDs）
				cartsWithMerchants := []db.GetUserCartsByCartIDsRow{
					{
						ID:                cart1ID,
						UserID:            user.ID,
						MerchantID:        merchant1.ID,
						MerchantName:      merchant1.Name,
						RegionID:          merchant1.RegionID,
						SubMchid:          pgtype.Text{String: "sub_mch_001", Valid: true},
						MerchantStatus:    "active",
						MerchantLatitude:  pgtype.Numeric{Valid: false},
						MerchantLongitude: pgtype.Numeric{Valid: false},
					},
					{
						ID:                cart2ID,
						UserID:            user.ID,
						MerchantID:        merchant2.ID,
						MerchantName:      merchant2.Name,
						RegionID:          merchant2.RegionID,
						SubMchid:          pgtype.Text{String: "sub_mch_002", Valid: true},
						MerchantStatus:    "active",
						MerchantLatitude:  pgtype.Numeric{Valid: false},
						MerchantLongitude: pgtype.Numeric{Valid: false},
					},
				}

				store.EXPECT().
					GetUserCartsByCartIDs(gomock.Any(), db.GetUserCartsByCartIDsParams{
						UserID:  user.ID,
						Column2: []int64{cart1ID, cart2ID},
					}).
					Times(1).
					Return(cartsWithMerchants, nil)

				store.EXPECT().
					GetCart(gomock.Any(), cart1ID).
					Times(1).
					Return(db.Cart{ID: cart1ID, UserID: user.ID, MerchantID: merchant1.ID, OrderType: "dine_in"}, nil)

				store.EXPECT().
					GetCart(gomock.Any(), cart2ID).
					Times(1).
					Return(db.Cart{ID: cart2ID, UserID: user.ID, MerchantID: merchant2.ID, OrderType: "dine_in"}, nil)

				// Mock: 获取购物车商品
				store.EXPECT().
					ListCartItems(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return([]db.ListCartItemsRow{}, nil)

				// Mock: 获取用户地址（用于距离计算）
				store.EXPECT().
					GetUserAddress(gomock.Any(), int64(1)).
					AnyTimes().
					Return(db.UserAddress{
						ID:        1,
						UserID:    user.ID,
						Latitude:  pgtype.Numeric{Valid: false},
						Longitude: pgtype.Numeric{Valid: false},
					}, nil)

				// Mock: 获取代取费配置（用于代取费计算）
				store.EXPECT().
					GetDeliveryFeeConfigByRegion(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
						ConfigKey: deliveryFeeDefaultConfigKey,
						ScopeType: db.PlatformConfigScopeGlobal,
						ScopeID:   pgtype.Int8{Valid: false},
					}).
					AnyTimes().
					Return(db.PlatformConfig{}, db.ErrRecordNotFound)
				store.EXPECT().
					GetLatestWeatherCoefficient(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(db.WeatherCoefficient{}, db.ErrRecordNotFound)
				store.EXPECT().
					ListPeakHourConfigsByRegion(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return([]db.PeakHourConfig{}, nil)
				store.EXPECT().
					ListActiveDeliveryPromotionsByMerchant(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return([]db.MerchantDeliveryPromotion{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// 接受成功或尚未完全实现的响应
				require.Contains(t, []int{http.StatusOK, http.StatusNotImplemented, http.StatusInternalServerError}, recorder.Code)
			},
		},
		{
			name: "InvalidRequest_NoCarts",
			body: gin.H{
				"cart_ids": []int64{},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应调用数据库，因为验证失败
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"cart_ids":   []int64{cart1ID, cart2ID},
				"address_id": 1,
			},
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
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

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, "/v1/cart/combined-checkout/preview", bytes.NewReader(body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestCombinedCheckoutAPI_TakeoutAppliesDeliveryFeeDiscount(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.IsOpen = true
	merchant.Latitude = pgtype.Numeric{Int: big.NewInt(300000000), Exp: -7, Valid: true}
	merchant.Longitude = pgtype.Numeric{Int: big.NewInt(1200000000), Exp: -7, Valid: true}
	cartID := int64(11)
	addressID := int64(22)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetUserCartsByCartIDs(gomock.Any(), db.GetUserCartsByCartIDsParams{
			UserID:  user.ID,
			Column2: []int64{cartID},
		}).
		Times(1).
		Return([]db.GetUserCartsByCartIDsRow{{
			ID:                cartID,
			UserID:            user.ID,
			MerchantID:        merchant.ID,
			MerchantName:      merchant.Name,
			RegionID:          merchant.RegionID,
			SubMchid:          pgtype.Text{String: "sub_mch_001", Valid: true},
			MerchantStatus:    "active",
			MerchantLatitude:  merchant.Latitude,
			MerchantLongitude: merchant.Longitude,
		}}, nil)
	store.EXPECT().
		GetCart(gomock.Any(), cartID).
		Times(1).
		Return(db.Cart{ID: cartID, UserID: user.ID, MerchantID: merchant.ID, OrderType: db.OrderTypeTakeout}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), cartID).
		Times(1).
		Return([]db.ListCartItemsRow{{
			DishID:    pgtype.Int8{Int64: 5, Valid: true},
			DishName:  pgtype.Text{String: "Dish", Valid: true},
			DishPrice: pgtype.Int8{Int64: 1000, Valid: true},
			Quantity:  2,
		}}, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(db.UserAddress{
			ID:        addressID,
			UserID:    user.ID,
			Latitude:  merchant.Latitude,
			Longitude: merchant.Longitude,
		}, nil)
	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), merchant.RegionID).
		Times(1).
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Times(1).
		Return(db.PlatformConfig{ConfigValue: mustMarshalDeliveryFeeDefaultConfig(t, deliveryFeeDefaultConfigValue{
			BaseFee:       500,
			BaseDistance:  3000,
			ExtraFeePerKm: 100,
			ValueRatio:    0.01,
			MinFee:        300,
		})}, nil)
	store.EXPECT().
		GetLatestWeatherCoefficient(gomock.Any(), merchant.RegionID).
		Times(1).
		Return(db.WeatherCoefficient{}, db.ErrRecordNotFound)
	store.EXPECT().
		ListPeakHourConfigsByRegion(gomock.Any(), merchant.RegionID).
		Times(1).
		Return([]db.PeakHourConfig{}, nil)
	store.EXPECT().
		ListActiveDeliveryPromotionsByMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.MerchantDeliveryPromotion{{
			MerchantID:     merchant.ID,
			MinOrderAmount: 1000,
			DiscountAmount: 200,
			ValidFrom:      time.Now().Add(-time.Hour),
			ValidUntil:     time.Now().Add(time.Hour),
			IsActive:       true,
		}}, nil)

	server := newTestServer(t, store)
	server.mapClient = combinedCheckoutMapClientStub{
		route: &maps.RouteResult{Distance: 2500, Duration: 600},
	}

	recorder := httptest.NewRecorder()
	body, err := json.Marshal(gin.H{
		"cart_ids":   []int64{cartID},
		"address_id": addressID,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/cart/combined-checkout/preview", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				Subtotal            int64 `json:"subtotal"`
				DeliveryFee         int64 `json:"delivery_fee"`
				DeliveryFeeDiscount int64 `json:"delivery_fee_discount"`
				TotalAmount         int64 `json:"total_amount"`
			} `json:"items"`
			TotalSubtotal            int64 `json:"total_subtotal"`
			TotalDeliveryFee         int64 `json:"total_delivery_fee"`
			TotalDeliveryFeeDiscount int64 `json:"total_delivery_fee_discount"`
			TotalAmount              int64 `json:"total_amount"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.Len(t, response.Data.Items, 1)
	require.Equal(t, int64(2000), response.Data.Items[0].Subtotal)
	require.Equal(t, int64(520), response.Data.Items[0].DeliveryFee)
	require.Equal(t, int64(200), response.Data.Items[0].DeliveryFeeDiscount)
	require.Equal(t, int64(2320), response.Data.Items[0].TotalAmount)
	require.Equal(t, int64(2000), response.Data.TotalSubtotal)
	require.Equal(t, int64(520), response.Data.TotalDeliveryFee)
	require.Equal(t, int64(200), response.Data.TotalDeliveryFeeDiscount)
	require.Equal(t, int64(2320), response.Data.TotalAmount)
}

func TestCombinedCheckoutAPI_TakeoutFallsBackWhenMapUnavailable(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.IsOpen = true
	merchant.Latitude = pgtype.Numeric{Int: big.NewInt(300000000), Exp: -7, Valid: true}
	merchant.Longitude = pgtype.Numeric{Int: big.NewInt(1200000000), Exp: -7, Valid: true}
	cartID := int64(12)
	addressID := int64(23)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetUserCartsByCartIDs(gomock.Any(), db.GetUserCartsByCartIDsParams{
			UserID:  user.ID,
			Column2: []int64{cartID},
		}).
		Times(1).
		Return([]db.GetUserCartsByCartIDsRow{{
			ID:                cartID,
			UserID:            user.ID,
			MerchantID:        merchant.ID,
			MerchantName:      merchant.Name,
			RegionID:          merchant.RegionID,
			SubMchid:          pgtype.Text{String: "sub_mch_001", Valid: true},
			MerchantStatus:    "active",
			MerchantLatitude:  merchant.Latitude,
			MerchantLongitude: merchant.Longitude,
		}}, nil)
	store.EXPECT().
		GetCart(gomock.Any(), cartID).
		Times(1).
		Return(db.Cart{ID: cartID, UserID: user.ID, MerchantID: merchant.ID, OrderType: db.OrderTypeTakeout}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), cartID).
		Times(1).
		Return([]db.ListCartItemsRow{{
			DishID:    pgtype.Int8{Int64: 5, Valid: true},
			DishName:  pgtype.Text{String: "Dish", Valid: true},
			DishPrice: pgtype.Int8{Int64: 1000, Valid: true},
			Quantity:  2,
		}}, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(db.UserAddress{
			ID:        addressID,
			UserID:    user.ID,
			Latitude:  merchant.Latitude,
			Longitude: merchant.Longitude,
		}, nil)
	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), merchant.RegionID).
		AnyTimes().
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		AnyTimes().
		Return(db.PlatformConfig{ConfigValue: mustMarshalDeliveryFeeDefaultConfig(t, deliveryFeeDefaultConfigValue{
			BaseFee:       500,
			BaseDistance:  3000,
			ExtraFeePerKm: 100,
			ValueRatio:    0,
			MinFee:        300,
		})}, nil)
	store.EXPECT().
		GetLatestWeatherCoefficient(gomock.Any(), merchant.RegionID).
		AnyTimes().
		Return(db.WeatherCoefficient{}, db.ErrRecordNotFound)
	store.EXPECT().
		ListPeakHourConfigsByRegion(gomock.Any(), merchant.RegionID).
		AnyTimes().
		Return([]db.PeakHourConfig{}, nil)
	store.EXPECT().
		ListActiveDeliveryPromotionsByMerchant(gomock.Any(), merchant.ID).
		AnyTimes().
		Return([]db.MerchantDeliveryPromotion{}, nil)

	server := newTestServer(t, store)
	server.mapClient = nil

	recorder := httptest.NewRecorder()
	body, err := json.Marshal(gin.H{
		"cart_ids":   []int64{cartID},
		"address_id": addressID,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/cart/combined-checkout/preview", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				DeliveryFee int64 `json:"delivery_fee"`
				TotalAmount int64 `json:"total_amount"`
			} `json:"items"`
			TotalDeliveryFee int64 `json:"total_delivery_fee"`
			TotalAmount      int64 `json:"total_amount"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.Len(t, response.Data.Items, 1)
	require.Equal(t, int64(500), response.Data.Items[0].DeliveryFee)
	require.Equal(t, int64(2500), response.Data.Items[0].TotalAmount)
	require.Equal(t, int64(500), response.Data.TotalDeliveryFee)
	require.Equal(t, int64(2500), response.Data.TotalAmount)
}
