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
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateUserAddressAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	address := randomUserAddress(user.ID, region.ID)

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
				"region_id":      region.ID,
				"detail_address": address.DetailAddress,
				"contact_name":   address.ContactName,
				"contact_phone":  address.ContactPhone,
				"longitude":      "120.123456",
				"latitude":       "30.123456",
				"is_default":     false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(region, nil)

				store.EXPECT().
					CreateUserAddress(gomock.Any(), gomock.Any()).
					Times(1).
					Return(address, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "InvalidRegion",
			body: map[string]interface{}{
				"region_id":      region.ID,
				"detail_address": address.DetailAddress,
				"contact_name":   address.ContactName,
				"contact_phone":  address.ContactPhone,
				"longitude":      "120.123456",
				"latitude":       "30.123456",
				"is_default":     false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(db.Region{}, sql.ErrNoRows)

				store.EXPECT().
					CreateUserAddress(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "SetAsDefault",
			body: map[string]interface{}{
				"region_id":      region.ID,
				"detail_address": address.DetailAddress,
				"contact_name":   address.ContactName,
				"contact_phone":  address.ContactPhone,
				"longitude":      "120.123456",
				"latitude":       "30.123456",
				"is_default":     true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(region, nil)

				store.EXPECT().
					SetDefaultAddress(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(nil)

				store.EXPECT().
					CreateUserAddress(gomock.Any(), gomock.Any()).
					Times(1).
					Return(address, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"region_id":      region.ID,
				"detail_address": address.DetailAddress,
				"contact_name":   address.ContactName,
				"contact_phone":  address.ContactPhone,
				"longitude":      "120.123456",
				"latitude":       "30.123456",
				"is_default":     false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Any()).
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

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/addresses"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetUserAddressAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	address := randomUserAddress(user.ID, region.ID)

	testCases := []struct {
		name          string
		addressID     int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "OK",
			addressID: address.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserAddress(gomock.Any(), gomock.Eq(address.ID)).
					Times(1).
					Return(address, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:      "NotFound",
			addressID: address.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserAddress(gomock.Any(), gomock.Eq(address.ID)).
					Times(1).
					Return(db.UserAddress{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "Forbidden",
			addressID: address.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+1, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserAddress(gomock.Any(), gomock.Eq(address.ID)).
					Times(1).
					Return(address, nil)
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

			url := fmt.Sprintf("/v1/addresses/%d", tc.addressID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListUserAddressesAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()

	n := 5
	addresses := make([]db.UserAddress, n)
	for i := 0; i < n; i++ {
		addresses[i] = randomUserAddress(user.ID, region.ID)
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
					ListUserAddresses(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(addresses, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserAddresses(gomock.Any(), gomock.Any()).
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

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/addresses"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateUserAddressAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	address := randomUserAddress(user.ID, region.ID)

	testCases := []struct {
		name          string
		addressID     int64
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "OK",
			addressID: address.ID,
			body: map[string]interface{}{
				"detail_address": "New Address",
				"contact_name":   "New Name",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserAddress(gomock.Any(), gomock.Eq(address.ID)).
					Times(1).
					Return(address, nil)

				store.EXPECT().
					UpdateUserAddress(gomock.Any(), gomock.Any()).
					Times(1).
					Return(address, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:      "Forbidden",
			addressID: address.ID,
			body: map[string]interface{}{
				"detail_address": "New Address",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+1, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserAddress(gomock.Any(), gomock.Eq(address.ID)).
					Times(1).
					Return(address, nil)

				store.EXPECT().
					UpdateUserAddress(gomock.Any(), gomock.Any()).
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

			url := fmt.Sprintf("/v1/addresses/%d", tc.addressID)
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestSetDefaultAddressAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	address := randomUserAddress(user.ID, region.ID)

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
				"address_id": address.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserAddress(gomock.Any(), gomock.Eq(address.ID)).
					Times(1).
					Return(address, nil)

				store.EXPECT().
					SetDefaultAddress(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(nil)

				store.EXPECT().
					SetAddressAsDefault(gomock.Any(), gomock.Any()).
					Times(1).
					Return(address, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Forbidden",
			body: map[string]interface{}{
				"address_id": address.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+1, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserAddress(gomock.Any(), gomock.Eq(address.ID)).
					Times(1).
					Return(address, nil)

				store.EXPECT().
					SetDefaultAddress(gomock.Any(), gomock.Any()).
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

			url := fmt.Sprintf("/v1/addresses/%d/default", tc.body["address_id"])
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDeleteUserAddressAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	address := randomUserAddress(user.ID, region.ID)

	testCases := []struct {
		name          string
		addressID     int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "OK",
			addressID: address.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserAddress(gomock.Any(), gomock.Eq(address.ID)).
					Times(1).
					Return(address, nil)

				store.EXPECT().
					DeleteUserAddress(gomock.Any(), db.DeleteUserAddressParams{
						ID:     address.ID,
						UserID: user.ID,
					}).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNoContent, recorder.Code)
			},
		},
		{
			name:      "Forbidden",
			addressID: address.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+1, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserAddress(gomock.Any(), gomock.Eq(address.ID)).
					Times(1).
					Return(address, nil)

				store.EXPECT().
					DeleteUserAddress(gomock.Any(), gomock.Any()).
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

			url := fmt.Sprintf("/v1/addresses/%d", tc.addressID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func randomUserAddress(userID int64, regionID int64) db.UserAddress {
	return db.UserAddress{
		ID:            util.RandomInt(1, 1000),
		UserID:        userID,
		RegionID:      regionID,
		DetailAddress: util.RandomString(20),
		ContactName:   util.RandomString(10),
		ContactPhone:  "13800138000",
		Longitude: pgtype.Numeric{
			Valid: false,
		},
		Latitude: pgtype.Numeric{
			Valid: false,
		},
		IsDefault: false,
	}
}

func randomUser(_ *testing.T) (user db.User, password string) {
	password = util.RandomString(6)

	user = db.User{
		ID:            util.RandomInt(1, 1000),
		WechatOpenid:  util.RandomString(20),
		WechatUnionid: pgtype.Text{String: util.RandomString(20), Valid: true},
		FullName:      util.RandomString(10),
		Phone:         pgtype.Text{String: "13800138000", Valid: true},
		AvatarUrl:     pgtype.Text{String: "https://example.com/avatar.jpg", Valid: true},
	}
	return
}
