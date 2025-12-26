package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 菜品搜索测试 ====================

func TestSearchDishesAPI(t *testing.T) {
	merchant := randomMerchant(util.RandomInt(1, 100))

	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK_GlobalSearch",
			query: "?keyword=鸡&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				// 全局搜索
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Dish{}, nil)

				store.EXPECT().
					CountSearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_SearchInMerchant",
			query: fmt.Sprintf("?keyword=鸡&merchant_id=%d&page_id=1&page_size=10", merchant.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesByName(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Dish{}, nil)

				store.EXPECT().
					CountSearchDishesByName(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "BadRequest_MissingKeyword",
			query: "?page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_KeywordTooLong",
			query: "?keyword=" + util.RandomString(101) + "&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidPageID",
			query: "?keyword=鸡&page_id=0&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_PageSizeTooLarge",
			query: "?keyword=鸡&page_id=1&page_size=100",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidMerchantID",
			query: "?keyword=鸡&merchant_id=0&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesByName(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InternalError_GlobalSearch",
			query: "?keyword=鸡&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchDishesGlobal(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Dish{}, sql.ErrConnDone)
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

			url := "/v1/search/dishes" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 商户搜索测试 ====================

func TestSearchMerchantsAPI(t *testing.T) {
	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?keyword=火锅&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchMerchants(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Merchant{}, nil)

				store.EXPECT().
					CountSearchMerchants(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "BadRequest_MissingKeyword",
			query: "?page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchMerchants(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidPageSize",
			query: "?keyword=火锅&page_id=1&page_size=0",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchMerchants(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InternalError",
			query: "?keyword=火锅&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchMerchants(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Merchant{}, sql.ErrConnDone)
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

			url := "/v1/search/merchants" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 包间搜索测试 ====================

func TestSearchRoomsAPI(t *testing.T) {
	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRoomsWithImage(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchRoomsWithImageRow{}, nil)

				store.EXPECT().
					CountSearchRooms(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_WithFilters",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&min_capacity=4&max_capacity=10&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRoomsWithImage(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchRoomsWithImageRow{}, nil)

				store.EXPECT().
					CountSearchRooms(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_WithTagFilter",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&tag_id=1&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRoomsByMerchantTag(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchRoomsByMerchantTagRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "BadRequest_MissingReservationDate",
			query: "?reservation_time=18:00&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_MissingReservationTime",
			query: "?reservation_date=2025-12-15&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidDateFormat",
			query: "?reservation_date=2025/12/15&reservation_time=18:00&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidTimeFormat",
			query: "?reservation_date=2025-12-15&reservation_time=25:00&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidMinCapacity",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&min_capacity=0&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_CapacityTooLarge",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&max_capacity=101&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidRegionID",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&region_id=0&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRooms(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "BadRequest_InvalidTagID",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&tag_id=0&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRoomsByMerchantTag(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InternalError",
			query: "?reservation_date=2025-12-15&reservation_time=18:00&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRoomsWithImage(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.SearchRoomsWithImageRow{}, sql.ErrConnDone)
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

			url := "/v1/search/rooms" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
